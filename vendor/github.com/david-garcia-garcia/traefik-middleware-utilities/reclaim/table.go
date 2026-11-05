package reclaim

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const (
	DefaultGrace = 10 * time.Second

	MsgPut       = "reclaim_put"
	MsgBind      = "reclaim_bind"
	MsgOrphan    = "reclaim_orphan"
	MsgReclaim   = "reclaim_reclaim"
	MsgDispose   = "reclaim_dispose"
	MsgHookPanic = "reclaim_hook_panic"
)

// Hooks are the optional sleep, wake, and close funcs for one incarnation, plus whether that
// incarnation keeps the key mapped until Close returns. A nil func skips that event. The table
// stores this value at put and ignores it on a later Open for the same key.
type Hooks struct {
	Sleep func()
	Wake  func()
	Close func()
	// EnforceCloseBeforeOpen keeps this incarnation mapped slotBusy until Close returns, so a
	// later Open of the same key waits and then creates. The zero value (false) unmaps first: a
	// concurrent Open may create while Close is still in flight. Set this when the value owns
	// something exclusive that cannot be held twice. The ending path reads the stored hooks, not
	// a later Open's argument.
	EnforceCloseBeforeOpen bool
}

// Table stores one value per key and drives it through create, sleep, wake, and close.
//
//	       Open, key absent
//	              |
//	          create()                 Wake()
//	              v                       |
//	Open ------> AWAKE                    |
//	              |                       |
//	 last holder Done                     |
//	              v                       |
//	          Sleep()                     |
//	              v                       |
//	           ASLEEP ---- Open before ---+
//	              |         grace ends
//	 grace elapsed / Reset / grace == 0
//	              v
//	           Close()  key deleted
//
// Every state change happens under t.mu; create and the stored hooks (Wake, Sleep, Close)
// run outside it. Wake and Sleep park the slot in slotBusy. Close does too when the stored
// EnforceCloseBeforeOpen is set; otherwise the key is unmapped first, so a concurrent Open
// may create during Close. An Open or a drop that meets a busy slot waits on slot.ready and
// looks again.
//
// The last-holder drop sleeps the value and writes reclaim_orphan, then either expires at zero
// grace or arms a grace AfterFunc. Close and reclaim_dispose stay after orphan, in that order.
// A Sleep panic aborts instead: Close and unmap (order follows stored EnforceCloseBeforeOpen), no orphan. Those lines cannot be reordered,
// because one goroutine writes them in that order.
type Table struct {
	mu      sync.Mutex
	grace   time.Duration
	items   map[string]*slot
	aliases map[string]*aliasEntry
}

// slotState is what the table may do with a slot right now.
type slotState int

const (
	// slotBusy means create, Wake, Sleep, or (when EnforceCloseBeforeOpen) Close is in flight.
	// Wait on slot.ready, then look again.
	slotBusy slotState = iota
	// slotAwake means the value is usable and Open may bind a holder to it.
	slotAwake
	// slotAsleep means the value has been slept and is kept until grace ends.
	slotAsleep
	// slotGone means this incarnation has been claimed for close, or create failed. The key is
	// already unmapped, so nothing can reach the slot except a watcher that predates the claim.
	slotGone
)

// slot is one incarnation: the value, what may be done with it, how many holders need it, and
// the create failure it is permanently stuck with if it never got a value.
type slot struct {
	value any
	// hooks are the Sleep, Wake, and Close funcs stored at put. Bind and reclaim do not replace them.
	hooks Hooks
	// createErr is what create returned. Every Open parked on ready replays it, and the slot is
	// gone: a later Open creates a new incarnation rather than retrying this one.
	createErr error
	state     slotState
	holders   int
	// ready is closed when the in-flight transition ends. Waiters re-read state afterwards.
	ready chan struct{}
	// graceTimer expires a sleeping incarnation. An Open that reclaims stops it.
	graceTimer *time.Timer
	// finished is closed on every path that ends this incarnation, so a nil-Done watcher
	// can stop polling without drop. Paths: endBusySlot (create fail; Sleep/Wake panic
	// when EnforceCloseBeforeOpen is unset), unmapAfterClose on the real incarnation
	// (enforce Close: zero-grace drop, expire), expire when EnforceCloseBeforeOpen is
	// unset, Reset for awake/asleep, and endBusyAfterPanic when EnforceCloseBeforeOpen
	// is set (Sleep-panic drop and Wake-panic wakeAndBind). A closer placeholder
	// never has a finished channel: no holder binds to it.
	finished chan struct{}
	logger   *slog.Logger
	// aliases are the public names currently bound to this incarnation.
	// Table.aliases is the forward map; this slice is only the reverse link for Close.
	aliases []*aliasEntry
}

// Config is the freeze-at-New settings for a Table. New copies Grace onto the table.
// Later writes to this struct do not change a table that already ran New.
type Config struct {
	// Grace is how long a sleeping value is kept before it is disposed. Zero keeps nothing.
	// A negative Grace becomes DefaultGrace.
	Grace time.Duration
}

// New builds an empty table. Grace is copied from cfg and MUST NOT change on that table afterwards.
func New(cfg Config) *Table {
	grace := cfg.Grace
	if grace < 0 {
		grace = DefaultGrace
	}
	return &Table{
		grace:   grace,
		items:   map[string]*slot{},
		aliases: map[string]*aliasEntry{},
	}
}

// requireContext panics if ctx is missing. Traefik New gets a WithCancel ctx; Background is still accepted.
func requireContext(ctx context.Context) {
	if ctx == nil {
		panic("reclaim: Open requires a context")
	}
}

// waitCtx returns when ctx is done or this incarnation has ended. Only a holder whose Done is nil
// (Background, the Yaegi shape) gets here, because dropWhenDone hands every holder that has a Done
// channel to context.AfterFunc, so this polls Err. incarnationEnded is true when finished closed
// first: the caller must not drop, because the slot is already gone.
func waitCtx(ctx context.Context, finished <-chan struct{}) (incarnationEnded bool) {
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		// finished is read before Err, and again below against the tick, so an ended incarnation
		// always wins over a done ctx. A holder that is canceled at the same time its slot ends
		// must report ended: reporting done would send watch to drop a slot nobody owns.
		select {
		case <-finished:
			return true
		default:
		}
		if ctx.Err() != nil {
			return false
		}
		select {
		case <-finished:
			return true
		case <-tick.C:
		}
	}
}

// runHook calls one hook and returns what it panicked with, or nil when it is unset or returned.
// The table finishes the slotBusy protocol either way: a hook runs on an AfterFunc goroutine, so
// letting the panic out would kill the process. Returning it keeps the caller in charge of the
// failure — no caller may treat nil-or-not as success. Runs outside t.mu.
func runHook(hook func()) (recovered any) {
	if hook == nil {
		return nil
	}
	defer func() { recovered = recover() }()
	hook()
	return nil
}

// closeFinished closes the per-incarnation finished channel exactly once. The caller holds t.mu.
// Every ending path must call this on the real incarnation (see finished). A closer whose
// finished is nil is a no-op, which is deliberate: no watcher can reference that slot.
func closeFinished(incarnation *slot) {
	if incarnation.finished == nil {
		return
	}
	close(incarnation.finished)
	incarnation.finished = nil
}

// dispose runs the Close hook and then reports the end. reclaim_dispose means Close has returned;
// a Close panic is reported on its own line, because nothing can retry. Close is invoked only
// here, through runHook, so an AfterFunc panic cannot kill the process on either ending path.
func dispose(key string, hooks Hooks, logger *slog.Logger) {
	if recovered := runHook(hooks.Close); recovered != nil {
		logger.Error(MsgHookPanic, "key", key, "hook", "close", "panic", recovered)
	}
	logger.Debug(MsgDispose, "key", key)
}

// endBusySlot records createErr on a busy slot (nil means waiters create), unmaps it, and closes ready.
func (t *Table) endBusySlot(key string, incarnation *slot, createErr error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	incarnation.createErr = createErr
	incarnation.state = slotGone
	closeFinished(incarnation)
	t.unbindIncarnationLocked(incarnation)
	if t.items[key] == incarnation {
		delete(t.items, key)
	}
	close(incarnation.ready)
}

// unmapAfterClose unmaps a still-mapped busy incarnation after Close has returned (or its panic
// was recovered) and closes ready so waiters create. The caller already ran dispose outside t.mu.
func (t *Table) unmapAfterClose(key string, incarnation *slot) {
	ready := t.unmapLocked(key, incarnation)
	close(ready)
}

// unmapLocked unmaps a still-mapped busy incarnation and returns ready so the caller can close
// it outside t.mu.
func (t *Table) unmapLocked(key string, incarnation *slot) chan struct{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.items[key] == incarnation {
		delete(t.items, key)
	}
	incarnation.state = slotGone
	closeFinished(incarnation)
	t.unbindIncarnationLocked(incarnation)
	return incarnation.ready
}

// endMappedClose runs dispose while the slot is still mapped slotBusy, then unmaps. Used when
// the stored EnforceCloseBeforeOpen is set. Close never runs under t.mu.
func (t *Table) endMappedClose(key string, incarnation *slot, storedHooks Hooks, logger *slog.Logger) {
	dispose(key, storedHooks, logger)
	t.unmapAfterClose(key, incarnation)
}

// closerSwap is the closer slot and the old ready channel installCloser returns.
type closerSwap struct {
	closer   *slot
	oldReady chan struct{}
}

// installCloser records createErr, ends the real incarnation, and occupies the key with a closer
// for the Close window. finished stays nil on the closer: a holder never binds to it.
func (t *Table) installCloser(key string, incarnation *slot, logger *slog.Logger, createErr error) closerSwap {
	t.mu.Lock()
	defer t.mu.Unlock()
	incarnation.createErr = createErr
	incarnation.state = slotGone
	closeFinished(incarnation)
	t.unbindIncarnationLocked(incarnation)
	oldReady := incarnation.ready
	// Occupy the key for the Close window. Waiters already parked on oldReady replay
	// createErr from this incarnation; a later Open finds closer and creates after Close.
	// finished stays nil: a holder never binds to this closer, so no watcher can
	// reference it. Do not add a channel here.
	closer := &slot{state: slotBusy, ready: make(chan struct{}), logger: logger}
	t.items[key] = closer
	return closerSwap{closer: closer, oldReady: oldReady}
}

// endBusyAfterPanic ends a busy slot after a recovered Sleep or Wake panic. createErr is what
// waiters already parked on this transition replay after ready closes (nil means they create).
// When the stored EnforceCloseBeforeOpen is set, the key stays mapped slotBusy across Close so a
// later Open waits, then creates. Wake waiters still hold this incarnation and replay createErr;
// a later Open parks on a closer that occupies the key until Close returns. Otherwise dest order:
// unmap first, then Close. Close never runs under t.mu.
func (t *Table) endBusyAfterPanic(key string, incarnation *slot, storedHooks Hooks, logger *slog.Logger, createErr error) {
	if storedHooks.EnforceCloseBeforeOpen {
		swap := t.installCloser(key, incarnation, logger, createErr)
		close(swap.oldReady)
		dispose(key, storedHooks, logger)
		t.unmapAfterClose(key, swap.closer)
		return
	}
	t.endBusySlot(key, incarnation, createErr)
	dispose(key, storedHooks, logger)
}

// openAction is what Open does after lookupOpen releases t.mu.
type openAction int

const (
	openCreate openAction = iota
	openBind
	openWake
	openPark
	openRetry
)

// openStep is the snapshot lookupOpen returns after releasing t.mu.
type openStep struct {
	action      openAction
	incarnation *slot
	value       any
	ready       <-chan struct{}
	storedHooks Hooks
}

// lookupOpen inspects key under t.mu and returns the next Open step. Register, bind, and wake
// mutations happen here so the caller can run create, logs, and Wake outside the lock.
func (t *Table) lookupOpen(key string, logger *slog.Logger) (openStep, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.items == nil {
		return openStep{}, fmt.Errorf("reclaim: open %q: uninitialized table", key)
	}
	incarnation, mapped := t.items[key]
	if !mapped {
		// Register the key before create runs, so a second first Open waits for this result
		// instead of creating a value that would be thrown away.
		incarnation = &slot{state: slotBusy, ready: make(chan struct{}), finished: make(chan struct{}), logger: logger}
		t.items[key] = incarnation
		return openStep{action: openCreate, incarnation: incarnation}, nil
	}
	switch incarnation.state {
	case slotAwake:
		// Only an Open that binds takes the slot's logger: orphan and dispose belong to the
		// last Open that actually held this incarnation, not to one that merely looked.
		incarnation.logger = logger
		incarnation.holders++
		return openStep{action: openBind, incarnation: incarnation, value: incarnation.value}, nil
	case slotAsleep:
		incarnation.logger = logger
		incarnation.state = slotBusy
		incarnation.ready = make(chan struct{})
		if incarnation.graceTimer != nil {
			// Cancel expiry: this incarnation is not being disposed after all.
			incarnation.graceTimer.Stop()
			incarnation.graceTimer = nil
		}
		incarnation.holders++
		return openStep{
			action:      openWake,
			incarnation: incarnation,
			value:       incarnation.value,
			storedHooks: incarnation.hooks,
		}, nil
	case slotBusy:
		// A create, wake, sleep, or (when EnforceCloseBeforeOpen) close owns the slot.
		return openStep{action: openPark, incarnation: incarnation, ready: incarnation.ready}, nil
	case slotGone:
		// This incarnation has ended. Take the key back and create a fresh one.
		if t.items[key] == incarnation {
			delete(t.items, key)
		}
	}
	return openStep{action: openRetry}, nil
}

// createErrOf returns the stored create failure for a slot that just left slotBusy.
func (t *Table) createErrOf(incarnation *slot) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return incarnation.createErr
}

// Open returns the stored value for key, creating it once, and tracks ctx until it is done.
// create takes no arguments: Yaegi cannot call func(context.Context) (any, error).
// logger is required; it is the only logger for this Open and is stored on the slot for orphan
// and dispose. hooks are stored on the incarnation at put; a later Open (bind or reclaim)
// ignores this argument. A sleeping value is woken before Open returns, so a caller never
// receives one asleep. The Close hook, when set, runs when this incarnation ends, after Sleep.
// (value, nil) means this call bound a holder that was still live at return. If ctx.Err() is
// set at bind, Open returns that error and not the pointer, and drops the holder on this call.
func (t *Table) Open(ctx context.Context, key string, logger *slog.Logger, create func() (any, error), hooks Hooks) (any, error) {
	if create == nil {
		// Guarded before wrapping: the closure below is never nil, so OpenWithHooks could not
		// report a nil create. The table and logger keep their precedence over this error.
		if t == nil {
			return nil, fmt.Errorf("reclaim: open %q: nil table", key)
		}
		if logger == nil {
			return nil, fmt.Errorf("reclaim: open %q: nil logger", key)
		}
		return nil, fmt.Errorf("reclaim: create %q: nil create", key)
	}
	return t.OpenWithHooks(ctx, key, logger, func() (any, Hooks, error) {
		value, err := create()
		return value, hooks, err
	})
}

// OpenWithHooks is Open with the hooks coming back from create instead of alongside it, so the
// caller does not need a variable declared before Open that the hooks close over. create returns
// the value and the hooks for that one incarnation; the table stores both at put, and a later
// Open (bind or reclaim) never runs create and keeps the stored hooks. Everything else, including
// EnforceCloseBeforeOpen, behaves exactly as documented on Open.
func (t *Table) OpenWithHooks(ctx context.Context, key string, logger *slog.Logger, create func() (any, Hooks, error)) (any, error) {
	if t == nil {
		return nil, fmt.Errorf("reclaim: open %q: nil table", key)
	}
	if logger == nil {
		return nil, fmt.Errorf("reclaim: open %q: nil logger", key)
	}
	if create == nil {
		return nil, fmt.Errorf("reclaim: create %q: nil create", key)
	}
	requireContext(ctx)

	for {
		step, err := t.lookupOpen(key, logger)
		if err != nil {
			return nil, err
		}
		switch step.action {
		case openCreate:
			return t.put(ctx, key, step.incarnation, logger, create)
		case openBind:
			logger.Debug(MsgBind, "key", key)
			return t.finishBind(ctx, key, step.incarnation, step.value)
		case openWake:
			return t.wakeAndBind(ctx, key, step.incarnation, logger, step.value, step.storedHooks)
		case openPark:
			<-step.ready
			if err := t.createErrOf(step.incarnation); err != nil {
				return nil, err
			}
		case openRetry:
			// Gone incarnation was unmapped; look up again and create.
		}
	}
}

// State is whether a Peek'd slot is usable now (Awake) or kept for grace (Asleep).
type State int

const (
	// Awake means the value is bound to at least one live context.
	Awake State = iota
	// Asleep means the last holder is gone and grace has not ended.
	Asleep
)

// Peek returns the stored value without binding a holder. ok is false when the key is missing,
// the slot is gone, or the slot is busy (create/Wake/Sleep/Close in flight). Peek does not wait,
// increment holders, call Wake, or stop grace. Busy is never returned as a State.
func (t *Table) Peek(key string) (value any, state State, ok bool) {
	if t == nil {
		return nil, 0, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.items == nil {
		return nil, 0, false
	}
	incarnation, mapped := t.items[key]
	if !mapped {
		return nil, 0, false
	}
	switch incarnation.state { //nolint:exhaustive // busy and gone are not peekable states
	case slotAwake:
		return incarnation.value, Awake, true
	case slotAsleep:
		return incarnation.value, Asleep, true
	default:
		return nil, 0, false
	}
}

// put runs create for a slot this Open registered, then publishes the value or the failure to
// every caller waiting on that slot. The hooks create returns are the ones stored on this
// incarnation, so a Sleep or Close hook may close over what create just built.
func (t *Table) put(ctx context.Context, key string, incarnation *slot, logger *slog.Logger, create func() (any, Hooks, error)) (any, error) {
	var value any
	var hooks Hooks
	var err error
	if recovered := runHook(func() { value, hooks, err = create() }); recovered != nil {
		err = fmt.Errorf("reclaim: create %q: panic: %v", key, recovered)
	}
	if err != nil {
		t.endBusySlot(key, incarnation, err)
		return nil, err
	}

	t.publishPut(key, incarnation, value, hooks)

	logger.Debug(MsgPut, "key", key)
	logger.Debug(MsgBind, "key", key)
	return t.finishBind(ctx, key, incarnation, value)
}

// publishPut stores a successful create and closes ready so waiters bind.
func (t *Table) publishPut(key string, incarnation *slot, value any, hooks Hooks) {
	t.mu.Lock()
	defer t.mu.Unlock()
	incarnation.value = value
	incarnation.hooks = hooks
	incarnation.state = slotAwake
	incarnation.holders++
	// Reset is tests only and must not race Open on a key, but if it did it dropped this slot
	// while create ran. Take the key back rather than strand a value nobody can close.
	t.items[key] = incarnation
	close(incarnation.ready)
}

// wakeAndBind wakes a sleeping slot for this Open and binds ctx. Wake must not run under the
// table mutex. If ctx is already done after wake, it drops this holder and returns that error
// without the pointer.
func (t *Table) wakeAndBind(ctx context.Context, key string, incarnation *slot, logger *slog.Logger, value any, storedHooks Hooks) (any, error) {
	if recovered := runHook(storedHooks.Wake); recovered != nil {
		// This Open has a caller to answer, so it reports the panic as its error. A waiter parked
		// on ready replays it from createErr rather than resuming a value Wake left half-done.
		err := fmt.Errorf("reclaim: wake %q: panic: %v", key, recovered)
		t.endBusyAfterPanic(key, incarnation, storedHooks, logger, err)
		return nil, err
	}

	t.publishWake(incarnation)

	logger.Debug(MsgReclaim, "key", key)
	logger.Debug(MsgBind, "key", key)
	return t.finishBind(ctx, key, incarnation, value)
}

// publishWake marks a woken slot awake and closes ready so waiters bind.
func (t *Table) publishWake(incarnation *slot) {
	t.mu.Lock()
	defer t.mu.Unlock()
	incarnation.state = slotAwake
	close(incarnation.ready)
}

// finishBind returns the bound value if ctx is still live, and watches it until Done. If ctx is
// already done, it drops this holder now (no AfterFunc) and returns that error without the pointer.
func (t *Table) finishBind(ctx context.Context, key string, incarnation *slot, value any) (any, error) {
	if err := ctx.Err(); err != nil {
		t.drop(key, incarnation)
		return nil, err
	}
	t.dropWhenDone(ctx, key, incarnation)
	return value, nil
}

// finishedAtBind copies incarnation.finished under t.mu so dropWhenDone can start watch without
// racing closeFinished. Nil means the incarnation already ended between bind and here.
func (t *Table) finishedAtBind(incarnation *slot) <-chan struct{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	return incarnation.finished
}

// dropWhenDone runs drop when ctx is done. A holder with a Done channel uses AfterFunc so the
// hold does not park a waiter. A holder whose Done is nil still needs watch to poll Err.
func (t *Table) dropWhenDone(ctx context.Context, key string, incarnation *slot) {
	if ctx.Done() != nil {
		context.AfterFunc(ctx, func() { t.drop(key, incarnation) })
		return
	}
	finished := t.finishedAtBind(incarnation)
	if finished == nil {
		// This incarnation ended between the bind and here, so there is no holder left to drop
		// and no channel left to wake a watcher. Starting one would poll ctx.Err forever.
		return
	}
	go t.watch(ctx, key, incarnation, finished)
}

// watch waits until ctx is done or this incarnation has ended. It drops the holder only when ctx
// is done while the incarnation is still live. finished is the channel from bind time, so a
// later closeFinished nil does not hide the close from this goroutine.
func (t *Table) watch(ctx context.Context, key string, incarnation *slot, finished <-chan struct{}) {
	if waitCtx(ctx, finished) {
		return
	}
	t.drop(key, incarnation)
}

// releaseHolder decrements the incarnation's holder count.
func (t *Table) releaseHolder(incarnation *slot) {
	t.mu.Lock()
	defer t.mu.Unlock()
	incarnation.holders--
}

// dropAction is what drop does after claimDrop releases t.mu.
type dropAction int

const (
	dropPark dropAction = iota
	dropStop
	dropClaim
)

// dropStep is what claimDrop returns after releasing t.mu.
type dropStep struct {
	action dropAction
	ready  <-chan struct{}
	logger *slog.Logger
	hooks  Hooks
	grace  time.Duration
}

// claimDrop either parks the caller on a busy ready channel, stops because this is not the last
// awake holder, or claims the last-holder Sleep transition.
func (t *Table) claimDrop(incarnation *slot) dropStep {
	t.mu.Lock()
	defer t.mu.Unlock()
	if incarnation.state == slotBusy {
		// A create, wake, sleep, or (when EnforceCloseBeforeOpen) close owns the slot. No schedule
		// the current state machine produces gets here — a busy slot only ever has holders that
		// have not bound yet — so only TestTable_DropWaitsForAnInFlightTransition reaches it. The
		// guard stays because the alternative is sleeping a value this goroutine does not own.
		return dropStep{action: dropPark, ready: incarnation.ready}
	}
	if incarnation.holders == 0 && incarnation.state == slotAwake {
		incarnation.state = slotBusy
		incarnation.ready = make(chan struct{})
		return dropStep{
			action: dropClaim,
			logger: incarnation.logger,
			hooks:  incarnation.hooks,
			grace:  t.grace,
		}
	}
	return dropStep{action: dropStop}
}

// parkAsleep marks the incarnation asleep after Sleep returned, closes ready, and arms grace
// when the key is still mapped with a positive grace. mapped is false when this goroutine must
// expire now (zero grace, or Reset already dropped the slot).
func (t *Table) parkAsleep(key string, incarnation *slot, grace time.Duration) (mapped bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	incarnation.state = slotAsleep
	close(incarnation.ready)
	mapped = t.items[key] == incarnation
	if mapped && grace <= 0 {
		// Zero grace keeps nothing: unmap now, so no Open can ever see this value asleep.
		delete(t.items, key)
		mapped = false
	}
	if mapped {
		// Why AfterFunc, not go + select on timer.C and woken: this package is interpreted under
		// Yaegi (Traefik plugins and TestYaegi_*). Yaegi v0.16.1's interp._select can miss a timer
		// wake when interpreted code selects on a channel from a goroutine started as go method(...).
		// Concurrent expire on Go 1.21.13 left expire uncalled and a goroutine in interp._select.func4
		// (run.go:3815) created by go callf(in) (run.go:1322). time.AfterFunc is the real stdlib
		// timer. The wait is not on the drop caller: a canceled bind must not stall Open for grace.
		incarnation.graceTimer = time.AfterFunc(grace, func() { t.expire(key, incarnation) })
	}
	return mapped
}

// drop removes one holder. When it was the last one, this goroutine sleeps the value and writes
// reclaim_orphan. Zero grace expires on this stack. Positive grace arms a grace AfterFunc.
// Orphan still precedes dispose. A Sleep panic aborts: Close, unmap (order follows stored
// EnforceCloseBeforeOpen), no orphan. When the stored
// EnforceCloseBeforeOpen is set, Close runs while the key is still mapped slotBusy. A watcher
// whose incarnation is already gone finds slotGone and returns.
func (t *Table) drop(key string, incarnation *slot) {
	t.releaseHolder(incarnation)
	var logger *slog.Logger
	var storedHooks Hooks
	var grace time.Duration
	for {
		step := t.claimDrop(incarnation)
		switch step.action {
		case dropPark:
			<-step.ready
			continue
		case dropStop:
			return
		case dropClaim:
			logger = step.logger
			storedHooks = step.hooks
			grace = step.grace
		}
		break
	}

	if recovered := runHook(storedHooks.Sleep); recovered != nil {
		// Nobody is waiting on a return value here, so createErr stays nil: a later Open creates a
		// fresh incarnation instead of inheriting one whose Sleep never finished. Stored
		// EnforceCloseBeforeOpen still keeps the key mapped across Close.
		logger.Error(MsgHookPanic, "key", key, "hook", "sleep", "panic", recovered)
		t.endBusyAfterPanic(key, incarnation, storedHooks, logger, nil)
		return
	}
	// Orphan is written while the slot is still busy. Reset leaves a busy slot to the goroutine
	// that owns the transition, so nothing else can write this incarnation's dispose line first.
	logger.Debug(MsgOrphan, "key", key)

	if storedHooks.EnforceCloseBeforeOpen && grace <= 0 {
		// Zero grace keeps nothing: Close while still slotBusy, then unmap so a racing Open
		// waits instead of creating during Close.
		t.endMappedClose(key, incarnation, storedHooks, logger)
		return
	}

	mapped := t.parkAsleep(key, incarnation, grace)

	// Not mapped means zero grace, or Reset dropped this slot: either way it is ours to close.
	if !mapped {
		t.expire(key, incarnation)
	}
}

// expireAction is what expire does after claimExpire releases t.mu.
type expireAction int

const (
	expireSkip expireAction = iota
	expireEnforce
	expireDispose
)

// expireStep is what claimExpire returns after releasing t.mu.
type expireStep struct {
	action expireAction
	hooks  Hooks
	logger *slog.Logger
}

// claimExpire takes a sleeping incarnation for close, or returns skip when an Open woke it.
func (t *Table) claimExpire(key string, incarnation *slot) expireStep {
	t.mu.Lock()
	defer t.mu.Unlock()
	if incarnation.state == slotAsleep && incarnation.holders == 0 {
		step := expireStep{hooks: incarnation.hooks, logger: incarnation.logger}
		if incarnation.hooks.EnforceCloseBeforeOpen {
			incarnation.state = slotBusy
			incarnation.ready = make(chan struct{})
			step.action = expireEnforce
			return step
		}
		incarnation.state = slotGone
		closeFinished(incarnation)
		if t.items[key] == incarnation {
			delete(t.items, key)
		}
		step.action = expireDispose
		return step
	}
	return expireStep{action: expireSkip}
}

// expire ends a sleeping incarnation, unless an Open woke it or something else already claimed
// it. When the stored EnforceCloseBeforeOpen is set, Close runs as slotBusy so a racing Open
// waits; otherwise the key is unmapped first (dest) so a concurrent Open may create during Close.
func (t *Table) expire(key string, incarnation *slot) {
	step := t.claimExpire(key, incarnation)
	switch step.action {
	case expireSkip:
		return
	case expireEnforce:
		t.endMappedClose(key, incarnation, step.hooks, step.logger)
	case expireDispose:
		dispose(key, step.hooks, step.logger)
		// claimExpire already unmapped the key. Aliases still pointing at this
		// incarnation are cleared after Close, same as the enforce path.
		t.mu.Lock()
		t.unbindIncarnationLocked(incarnation)
		t.mu.Unlock()
	}
}

// takeAll swaps out every mapped incarnation and stops their grace timers. Tests-only Reset.
func (t *Table) takeAll() map[string]*slot {
	t.mu.Lock()
	defer t.mu.Unlock()
	items := t.items
	t.items = map[string]*slot{}
	for _, incarnation := range items {
		t.unbindIncarnationLocked(incarnation)
	}
	t.aliases = map[string]*aliasEntry{}
	for _, incarnation := range items {
		if incarnation.graceTimer != nil {
			incarnation.graceTimer.Stop()
			incarnation.graceTimer = nil
		}
	}
	return items
}

// endResetSlot snapshots an incarnation for Reset and closeFinished when it was awake or asleep.
func (t *Table) endResetSlot(incarnation *slot) (state slotState, storedHooks Hooks, logger *slog.Logger) {
	t.mu.Lock()
	defer t.mu.Unlock()
	state, storedHooks, logger = incarnation.state, incarnation.hooks, incarnation.logger
	if state == slotAwake || state == slotAsleep {
		incarnation.state = slotGone
		closeFinished(incarnation)
	}
	return
}

// Reset ends every incarnation on this table. An awake value is slept first, so Close never sees
// a live value and orphan still precedes dispose when Sleep returns. A Sleep panic skips orphan
// and still disposes. Tests only: it must not race an Open on the same key. A slot that is
// mid-transition is ended by the goroutine that owns that transition. Reset unmaps first
// regardless of EnforceCloseBeforeOpen.
func (t *Table) Reset() {
	if t == nil {
		return
	}
	items := t.takeAll()
	if items == nil {
		return
	}

	for key, incarnation := range items {
		state, storedHooks, logger := t.endResetSlot(incarnation)

		switch state {
		case slotAwake:
			if recovered := runHook(storedHooks.Sleep); recovered != nil {
				logger.Error(MsgHookPanic, "key", key, "hook", "sleep", "panic", recovered)
			} else {
				logger.Debug(MsgOrphan, "key", key)
			}
			dispose(key, storedHooks, logger)
		case slotAsleep:
			dispose(key, storedHooks, logger)
		case slotBusy, slotGone:
			// The goroutine that owns this transition ends the incarnation itself: it finds the
			// slot unmapped, or its grace wait already released by the loop above.
		}
	}
}

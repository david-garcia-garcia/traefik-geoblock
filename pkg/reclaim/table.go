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

	MsgPut     = "reclaim_put"
	MsgBind    = "reclaim_bind"
	MsgOrphan  = "reclaim_orphan"
	MsgReclaim = "reclaim_reclaim"
	MsgDispose = "reclaim_dispose"
)

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
// Every state change happens under t.mu; every call into the value (create, Wake, Sleep, Close)
// happens outside it with the slot parked in slotBusy. One key's transitions are therefore
// sequential, and a value that is slow to create or slow to sleep never blocks another key. An
// Open or a drop that meets a busy slot waits on slot.ready and looks again.
//
// The goroutine that sees the last holder go Done owns the rest of that incarnation: it sleeps
// the value, writes reclaim_orphan, waits out grace, closes the value, and writes
// reclaim_dispose. Those lines cannot be reordered, because one goroutine writes them in that
// order.
type Table struct {
	mu    sync.Mutex
	grace time.Duration
	items map[string]*slot
}

// slotState is what the table may do with a slot right now.
type slotState int

const (
	// slotBusy means create, Wake, or Sleep is in flight. Wait on slot.ready, then look again.
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
	// createErr is what create returned. Every Open parked on ready replays it, and the slot is
	// gone: a later Open creates a new incarnation rather than retrying this one.
	createErr error
	state     slotState
	holders   int
	// ready is closed when the in-flight transition ends. Waiters re-read state afterwards.
	ready chan struct{}
	// woken is closed by the Open that reclaims a sleeping value, to end its grace wait.
	woken  chan struct{}
	logger *slog.Logger
}

// NewTable builds an empty table. Grace is how long a sleeping value is kept before it is
// disposed. Zero grace keeps nothing. A negative grace becomes DefaultGrace.
func NewTable(grace time.Duration) *Table {
	if grace < 0 {
		grace = DefaultGrace
	}
	return &Table{
		grace: grace,
		items: map[string]*slot{},
	}
}

// requireContext panics if ctx is missing. Traefik New gets a WithCancel ctx; Background is still accepted.
func requireContext(ctx context.Context) {
	if ctx == nil {
		panic("reclaim: Open requires a context")
	}
}

// waitCtx returns when ctx is done. Prefer Done(); if it is nil (Background), poll Err.
func waitCtx(ctx context.Context) {
	if done := ctx.Done(); done != nil {
		<-done
		return
	}
	for ctx.Err() == nil {
		time.Sleep(20 * time.Millisecond)
	}
}

// closer is an optional Close on a stored value, called once when the incarnation ends.
// Sleep has always run first, so Close never has to handle the live state.
type closer interface {
	Close()
}

// sleeper is an optional Sleep on a stored value, called when its last holder is gone. The value
// stays stored and keeps its identity; it releases what is expensive to hold idle.
type sleeper interface {
	Sleep()
}

// waker is an optional Wake on a stored value, called before Open hands a sleeping value back.
// It cannot fail: a value that cannot guarantee resume does not implement Sleep and Wake.
type waker interface {
	Wake()
}

// The three lookups below are type switches rather than `value.(sleeper)` because a comma-ok
// assertion to an interface panics under Yaegi ("reflect.Set: value of type interface {} is not
// assignable to type interp.valueInterface") for a value that reached `any` by being passed in,
// and a panic here runs on a background goroutine and would end the Traefik process. The type
// switch reports no match instead.
//
// It reports no match a lot: Yaegi hands back a value returned through an interpreted
// `func() (any, error)` as a synthesized struct type with no methods, so under the interpreter
// none of the three ever matches and the optional lifecycle is inert. That is upstream and
// predates this change (Close has never run interpreted either); see
// knowledge/debt/2026-09-08-yaegi-drops-methods-on-any.md. Compiled callers get all four events.

// sleepValue puts value to sleep if it has Sleep. Runs outside t.mu.
func sleepValue(value any) {
	switch typed := value.(type) {
	case sleeper:
		typed.Sleep()
	}
}

// wakeValue wakes value if it has Wake. Runs outside t.mu, before Open returns.
func wakeValue(value any) {
	switch typed := value.(type) {
	case waker:
		typed.Wake()
	}
}

// closeValue closes value if it has Close. Runs outside t.mu, always after sleepValue.
func closeValue(value any) {
	switch typed := value.(type) {
	case closer:
		typed.Close()
	}
}

// dispose closes the value and then reports the end, so reclaim_dispose means Close has returned.
func dispose(key string, value any, logger *slog.Logger) {
	closeValue(value)
	logger.Debug(MsgDispose, "key", key)
}

// Open returns the stored value for key, creating it once, and tracks ctx until it is done.
// create takes no arguments: Yaegi cannot call func(context.Context) (any, error).
// logger is required; it is the only logger for this Open and is stored on the slot for orphan
// and dispose. A sleeping value is woken before Open returns, so a caller never receives one
// asleep. If the value has Close(), the table calls it when this incarnation ends, after Sleep.
func (t *Table) Open(ctx context.Context, key string, logger *slog.Logger, create func() (any, error)) (any, error) {
	if t == nil {
		return nil, fmt.Errorf("reclaim: open %q: nil table", key)
	}
	if logger == nil {
		return nil, fmt.Errorf("reclaim: open %q: nil logger", key)
	}
	requireContext(ctx)

	for {
		t.mu.Lock()
		incarnation, mapped := t.items[key]
		if !mapped {
			// Register the key before create runs, so a second first Open waits for this result
			// instead of creating a value that would be thrown away.
			incarnation = &slot{state: slotBusy, ready: make(chan struct{}), logger: logger}
			t.items[key] = incarnation
			t.mu.Unlock()
			return t.put(ctx, key, incarnation, logger, create)
		}
		switch incarnation.state {
		case slotAwake:
			// Only an Open that binds takes the slot's logger: orphan and dispose belong to the
			// last Open that actually held this incarnation, not to one that merely looked.
			incarnation.logger = logger
			incarnation.holders++
			value := incarnation.value
			t.mu.Unlock()
			logger.Debug(MsgBind, "key", key)
			go t.watch(key, incarnation, ctx)
			return value, nil
		case slotAsleep:
			return t.reclaimLocked(ctx, key, incarnation, logger), nil
		case slotBusy:
			// A create, wake, or sleep owns the slot. Wait for it, then look again.
			ready := incarnation.ready
			t.mu.Unlock()
			<-ready
			t.mu.Lock()
			createErr := incarnation.createErr
			t.mu.Unlock()
			if createErr != nil {
				return nil, createErr
			}
		case slotGone:
			// This incarnation has ended. Take the key back and create a fresh one.
			if t.items[key] == incarnation {
				delete(t.items, key)
			}
			t.mu.Unlock()
		}
	}
}

// put runs create for a slot this Open registered, then publishes the value or the failure to
// every caller waiting on that slot.
func (t *Table) put(ctx context.Context, key string, incarnation *slot, logger *slog.Logger, create func() (any, error)) (any, error) {
	value, err := create()

	t.mu.Lock()
	if err != nil {
		incarnation.createErr = err
		incarnation.state = slotGone
		if t.items[key] == incarnation {
			delete(t.items, key)
		}
		close(incarnation.ready)
		t.mu.Unlock()
		return nil, err
	}
	incarnation.value = value
	incarnation.state = slotAwake
	incarnation.holders++
	// Reset is tests only and must not race Open on a key, but if it did it dropped this slot
	// while create ran. Take the key back rather than strand a value nobody can close.
	t.items[key] = incarnation
	close(incarnation.ready)
	t.mu.Unlock()

	logger.Debug(MsgPut, "key", key)
	logger.Debug(MsgBind, "key", key)
	go t.watch(key, incarnation, ctx)
	return value, nil
}

// reclaimLocked wakes a sleeping slot for this Open and binds ctx. The caller holds t.mu and has
// seen slotAsleep; this releases it, because Wake must not run under the table mutex.
func (t *Table) reclaimLocked(ctx context.Context, key string, incarnation *slot, logger *slog.Logger) any {
	incarnation.logger = logger
	incarnation.state = slotBusy
	incarnation.ready = make(chan struct{})
	if incarnation.woken != nil {
		// End the grace wait: this incarnation is not being disposed after all.
		close(incarnation.woken)
		incarnation.woken = nil
	}
	incarnation.holders++
	value := incarnation.value
	t.mu.Unlock()

	wakeValue(value)

	t.mu.Lock()
	incarnation.state = slotAwake
	close(incarnation.ready)
	t.mu.Unlock()

	logger.Debug(MsgReclaim, "key", key)
	logger.Debug(MsgBind, "key", key)
	go t.watch(key, incarnation, ctx)
	return value
}

// watch waits until ctx is done, then drops that holder from this slot.
func (t *Table) watch(key string, incarnation *slot, ctx context.Context) {
	waitCtx(ctx)
	t.drop(key, incarnation)
}

// drop removes one holder. When it was the last one, this goroutine ends the incarnation: sleep,
// orphan, grace, close, dispose — in that order, so those lines cannot be reordered. A watcher
// whose incarnation is already gone finds slotGone and returns.
func (t *Table) drop(key string, incarnation *slot) {
	t.mu.Lock()
	incarnation.holders--
	for incarnation.state == slotBusy {
		// A create, wake, or sleep owns the slot. Wait for it before deciding to sleep.
		ready := incarnation.ready
		t.mu.Unlock()
		<-ready
		t.mu.Lock()
	}
	if incarnation.holders > 0 || incarnation.state != slotAwake {
		t.mu.Unlock()
		return
	}

	incarnation.state = slotBusy
	incarnation.ready = make(chan struct{})
	incarnation.woken = make(chan struct{})
	woken := incarnation.woken
	value, logger := incarnation.value, incarnation.logger
	grace := t.grace
	t.mu.Unlock()

	sleepValue(value)
	// Orphan is written while the slot is still busy. Reset leaves a busy slot to the goroutine
	// that owns the transition, so nothing else can write this incarnation's dispose line first.
	logger.Debug(MsgOrphan, "key", key)

	t.mu.Lock()
	incarnation.state = slotAsleep
	close(incarnation.ready)
	mapped := t.items[key] == incarnation
	if mapped && grace <= 0 {
		// Zero grace keeps nothing: unmap now, so no Open can ever see this value asleep.
		delete(t.items, key)
		mapped = false
	}
	t.mu.Unlock()

	// Not mapped means zero grace, or Reset dropped this slot: either way it is ours to close.
	if !mapped {
		t.expire(key, incarnation)
		return
	}

	wait := time.NewTimer(grace)
	defer wait.Stop()
	select {
	case <-wait.C:
		t.expire(key, incarnation)
	case <-woken:
	}
}

// expire ends a sleeping incarnation, unless an Open woke it or something else already claimed
// it. It is the only place that closes a value the table still had mapped.
func (t *Table) expire(key string, incarnation *slot) {
	t.mu.Lock()
	if incarnation.state != slotAsleep || incarnation.holders > 0 {
		t.mu.Unlock()
		return
	}
	incarnation.state = slotGone
	if t.items[key] == incarnation {
		delete(t.items, key)
	}
	value, logger := incarnation.value, incarnation.logger
	t.mu.Unlock()
	dispose(key, value, logger)
}

// Reset ends every incarnation on this table. An awake value is slept first, so Close never sees
// a live value and orphan still precedes dispose. Tests only: it must not race an Open on the
// same key. A slot that is mid-transition is ended by the goroutine that owns that transition.
func (t *Table) Reset() {
	if t == nil {
		return
	}
	t.mu.Lock()
	items := t.items
	t.items = map[string]*slot{}
	for _, incarnation := range items {
		if incarnation.woken != nil {
			close(incarnation.woken)
			incarnation.woken = nil
		}
	}
	t.mu.Unlock()

	for key, incarnation := range items {
		t.mu.Lock()
		state, value, logger := incarnation.state, incarnation.value, incarnation.logger
		if state == slotAwake || state == slotAsleep {
			incarnation.state = slotGone
		}
		t.mu.Unlock()

		switch state {
		case slotAwake:
			sleepValue(value)
			logger.Debug(MsgOrphan, "key", key)
			dispose(key, value, logger)
		case slotAsleep:
			dispose(key, value, logger)
		case slotBusy, slotGone:
			// The goroutine that owns this transition ends the incarnation itself: it finds the
			// slot unmapped, or its grace wait already released by the loop above.
		}
	}
}

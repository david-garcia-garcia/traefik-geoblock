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

	// TableGrace is the grace an Open passes to take the table's grace instead of naming its own.
	// Any negative duration means the same; this is the spelling to use.
	TableGrace time.Duration = -1

	MsgPut     = "reclaim_put"
	MsgBind    = "reclaim_bind"
	MsgOrphan  = "reclaim_orphan"
	MsgReclaim = "reclaim_reclaim"
	MsgDispose = "reclaim_dispose"
)

// Table stores one value per key and keeps it while any bound context is live or grace has not elapsed.
//
//	              Open (first create)
//	                     |
//	                     v
//	Open (bind) -----> LIVE <---- Open same key before timer fires
//	                     |              ^
//	         last holder Done           | stop timer, keep value
//	                     v              |
//	                  ORPHAN -----------+
//	                     |
//	        grace elapsed / Reset / grace==0
//	                     v
//	                   GONE    cancel(life), Close value, log dispose
//
// drop and fire take the *slot pointer. If items[key] is a different slot
// (Reset, or a later incarnation), they no-op. fire also no-ops unless
// graceGen still matches the armed generation (a reclaim or a later orphan
// bumped it). That stops a queued AfterFunc from disposing a live slot.
type Table struct {
	mu    sync.Mutex
	grace time.Duration
	items map[string]*slot
}

// slot is one incarnation: the value, the cancel for its lifetime, and the holders that still need it.
type slot struct {
	value   any
	cancel  context.CancelFunc
	holders map[uint64]struct{}
	nextID  uint64
	// grace is this incarnation's wait after its last holder goes, fixed by the Open that created
	// it. The table's grace is what an Open takes when it names none. A later Open never changes it.
	grace time.Duration
	// valueClosed is closed by the lifetime goroutine once it has run Close on the value.
	// fire and Reset receive on it before they log dispose, so every slot must have one.
	valueClosed chan struct{}
	// graceTimer is the armed AfterFunc; arming is the window between the orphan
	// log and that arming, so grace never starts before the orphan line lands.
	graceTimer *time.Timer
	arming     bool
	graceGen   uint64
	logger     *slog.Logger
}

// NewTable builds an empty table. Zero grace means no wait after the last holder. A negative grace becomes DefaultGrace.
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

// closer is an optional Close on a stored value, called when the incarnation ends.
type closer interface {
	Close()
}

// stopValue calls Close if value has it. Used for a lost create and when an incarnation ends.
func stopValue(value any) {
	if c, ok := value.(closer); ok {
		c.Close()
	}
}

// Open returns the stored value for key, creating it once, and tracks ctx until it is done.
// create takes no arguments: Yaegi cannot call func(context.Context) (any, error) (it assigns life onto the value).
// logger is required; it is the only logger for this Open and is stored on the slot for orphan and dispose.
// grace is this incarnation's wait after its last holder goes; pass TableGrace to take the table's.
// It applies only when this Open creates the value — an Open that finds one keeps that incarnation's grace.
// If the value has Close(), the table calls it when this incarnation ends, before it logs dispose.
func (t *Table) Open(ctx context.Context, key string, logger *slog.Logger, grace time.Duration, create func() (any, error)) (any, error) {
	if t == nil {
		return nil, fmt.Errorf("reclaim: open %q: nil table", key)
	}
	if logger == nil {
		return nil, fmt.Errorf("reclaim: open %q: nil logger", key)
	}
	requireContext(ctx)

	// Reuse a live or in-grace incarnation.
	t.mu.Lock()
	if e, ok := t.items[key]; ok {
		id, reclaimed := t.bindLocked(e)
		e.logger = logger
		stored := e.value
		t.mu.Unlock()
		t.logBind(logger, key, reclaimed)
		go t.watch(key, id, e, ctx)
		return stored, nil
	}
	t.mu.Unlock()

	// Create outside the lock so two first Opens can race.
	life, cancel := context.WithCancel(context.Background())
	created, err := create()
	if err != nil {
		cancel()
		return nil, err
	}

	t.mu.Lock()
	// Another Open won: keep the stored value and drop this extra create.
	if e, ok := t.items[key]; ok {
		id, reclaimed := t.bindLocked(e)
		e.logger = logger
		stored := e.value
		t.mu.Unlock()
		cancel()
		stopValue(created)
		t.logBind(logger, key, reclaimed)
		go t.watch(key, id, e, ctx)
		return stored, nil
	}

	// First put for this key. Close the value when life is canceled (fire / Reset).
	if grace < 0 {
		grace = t.grace
	}
	e := &slot{
		value:       created,
		cancel:      cancel,
		holders:     map[uint64]struct{}{},
		grace:       grace,
		valueClosed: make(chan struct{}),
		logger:      logger,
	}
	t.items[key] = e
	id, _ := t.bindLocked(e)
	t.mu.Unlock()
	go func() {
		waitCtx(life)
		stopValue(created)
		close(e.valueClosed)
	}()
	logger.Debug(MsgPut, "key", key)
	t.logBind(logger, key, false)
	go t.watch(key, id, e, ctx)
	return created, nil
}

// gracePending reports whether this slot is already on its way out: grace armed, or still
// in the window between the orphan line and the arm. Caller holds t.mu.
func (e *slot) gracePending() bool {
	return e.graceTimer != nil || e.arming
}

// bindLocked attaches a holder and invalidates grace if this Open reclaimed the key. Caller holds t.mu.
// Grace pending means the slot was orphaned, so binding into it is a reclaim: the generation bump
// tells a queued fire, and a drop that is still arming, to leave this slot alone.
func (t *Table) bindLocked(e *slot) (id uint64, reclaimed bool) {
	if e.gracePending() {
		if e.graceTimer != nil {
			e.graceTimer.Stop()
			e.graceTimer = nil
		}
		e.graceGen++
		reclaimed = true
	}
	e.nextID++
	e.holders[e.nextID] = struct{}{}
	return e.nextID, reclaimed
}

// logBind emits reclaim (if this Open stopped grace) and bind. Caller must not hold t.mu.
func (t *Table) logBind(logger *slog.Logger, key string, reclaimed bool) {
	if reclaimed {
		logger.Debug(MsgReclaim, "key", key)
	}
	logger.Debug(MsgBind, "key", key)
}

// watch waits until ctx is done, then drops that holder on this slot only.
func (t *Table) watch(key string, id uint64, e *slot, ctx context.Context) {
	waitCtx(ctx)
	t.drop(key, id, e)
}

// drop removes one holder from e and starts grace when none remain. A stale watcher (Reset or a newer slot) is ignored.
// The orphan line is logged outside the mutex and before grace starts, so dispose can never precede it.
func (t *Table) drop(key string, id uint64, e *slot) {
	t.mu.Lock()
	cur, ok := t.items[key]
	if !ok || cur != e {
		t.mu.Unlock()
		return
	}
	delete(e.holders, id)
	if len(e.holders) > 0 || e.gracePending() {
		t.mu.Unlock()
		return
	}

	// Last holder gone: claim the orphan window, then log with the mutex released.
	e.graceGen++
	e.arming = true
	orphanLog := e.logger
	t.mu.Unlock()

	orphanLog.Debug(MsgOrphan, "key", key)

	t.mu.Lock()
	e.arming = false
	if t.items[key] != e || len(e.holders) > 0 {
		t.mu.Unlock()
		return
	}
	// Still orphaned, so this drop owns the arm. Read the generation now rather than trusting
	// the one logged above: an Open can have reclaimed and released the slot during that line,
	// and its own drop returned early on the arming window, leaving the arm to this one.
	gen := e.graceGen
	// This incarnation's own grace, not the table's: the Open that created it may have named one.
	// Zero grace ends the incarnation right here, on this watcher goroutine: fire runs the value's
	// Close and blocks until it returns, so a slow Close holds up this drop rather than a timer.
	if e.grace == 0 {
		t.mu.Unlock()
		t.fire(key, e, gen)
		return
	}
	// Grace is armed for this generation only. A later reclaim bumps the generation, which is what
	// makes the queued fire a no-op even though the timer still runs.
	e.graceTimer = time.AfterFunc(e.grace, func() { t.fire(key, e, gen) })
	t.mu.Unlock()
}

// fire cancels the incarnation lifetime if e is still the mapped slot, this grace generation is current, and no holders remain.
// It waits for the lifetime goroutine to close the value, so the dispose line means the value is already stopped.
func (t *Table) fire(key string, e *slot, gen uint64) {
	t.mu.Lock()
	cur, ok := t.items[key]
	if !ok || cur != e || e.graceGen != gen || len(e.holders) > 0 {
		t.mu.Unlock()
		return
	}
	cancel := e.cancel
	disposeLog := e.logger
	delete(t.items, key)
	t.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	<-e.valueClosed
	disposeLog.Debug(MsgDispose, "key", key)
}

// Reset stops grace timers and cancels every incarnation lifetime, then logs dispose once each value is closed. Tests only.
func (t *Table) Reset() {
	if t == nil {
		return
	}
	t.mu.Lock()
	items := t.items
	t.items = map[string]*slot{}
	// Invalidate the slots we just took off the map: a queued fire, and a drop
	// that is still arming, both compare against these fields under the mutex.
	for _, e := range items {
		if e.graceTimer != nil {
			e.graceTimer.Stop()
			e.graceTimer = nil
		}
		e.arming = false
		e.graceGen++
	}
	t.mu.Unlock()
	for key, e := range items {
		if e.cancel != nil {
			e.cancel()
		}
		<-e.valueClosed
		e.logger.Debug(MsgDispose, "key", key)
	}
}

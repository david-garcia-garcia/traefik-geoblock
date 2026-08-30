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

// Table stores one value per key and keeps it while any bound context is live or grace has not elapsed.
//
//	                   Open (first create)
//	                          |
//	                          v
//	     Open (bind) -----> LIVE <---- Open same key before timer fires
//	                          |              ^
//	              last holder Done           | stop timer, keep value
//	                          v              |
//	                       ORPHAN -----------+
//	                          |
//	             grace elapsed / Reset / grace==0
//	                          v
//	                        GONE    cancel(life), delete key
//
// drop and fire take the *slot pointer. If items[key] is a different slot
// (Reset, or a later incarnation), they no-op. fire also no-ops unless
// graceGen still matches the armed generation (a reclaim or a later orphan
// bumped it). That stops a queued AfterFunc from disposing a live slot.
type Table struct {
	mu     sync.Mutex
	grace  time.Duration
	logger *slog.Logger
	items  map[string]*slot
}

// slot is one incarnation: the value, the cancel for its lifetime, and the holders that still need it.
type slot struct {
	value      any
	cancel     context.CancelFunc
	holders    map[uint64]struct{}
	nextID     uint64
	graceTimer *time.Timer
	graceGen   uint64
}

// NewTable builds an empty table. Zero grace means no wait after the last holder. A negative grace becomes DefaultGrace. A nil logger becomes slog.Default.
func NewTable(grace time.Duration, logger *slog.Logger) *Table {
	if grace < 0 {
		grace = DefaultGrace
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Table{
		grace:  grace,
		logger: logger,
		items:  map[string]*slot{},
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

// stopValue calls Close if v has it. Used for a lost create and when life ends.
func stopValue(v any) {
	if c, ok := v.(closer); ok {
		c.Close()
	}
}

// Open returns the stored value for key, creating it once, and tracks ctx until it is done.
// create takes no arguments: Yaegi cannot call func(context.Context) (any, error) (it assigns life onto the value).
// If the value has Close(), the table calls it when this incarnation ends.
func (t *Table) Open(ctx context.Context, key string, create func() (any, error)) (any, error) {
	if t == nil {
		return nil, fmt.Errorf("reclaim: open %q: nil table", key)
	}
	requireContext(ctx)

	// Reuse a live or in-grace incarnation.
	t.mu.Lock()
	if e, ok := t.items[key]; ok {
		id, reclaimed := t.bindLocked(e)
		v := e.value
		t.mu.Unlock()
		t.logBind(key, reclaimed)
		go t.watch(key, id, e, ctx)
		return v, nil
	}
	t.mu.Unlock()

	// Create outside the lock so two first Opens can race.
	life, cancel := context.WithCancel(context.Background())
	v, err := create()
	if err != nil {
		cancel()
		return nil, err
	}

	t.mu.Lock()
	// Another Open won: keep the stored value and drop this extra create.
	if e, ok := t.items[key]; ok {
		id, reclaimed := t.bindLocked(e)
		exist := e.value
		t.mu.Unlock()
		cancel()
		stopValue(v)
		t.logBind(key, reclaimed)
		go t.watch(key, id, e, ctx)
		return exist, nil
	}

	// First put for this key. Close the value when life is canceled (fire / Reset).
	e := &slot{
		value:   v,
		cancel:  cancel,
		holders: map[uint64]struct{}{},
	}
	t.items[key] = e
	id, _ := t.bindLocked(e)
	t.mu.Unlock()
	go func() {
		waitCtx(life)
		stopValue(v)
	}()
	t.logger.Info(MsgPut, "key", key)
	t.logBind(key, false)
	go t.watch(key, id, e, ctx)
	return v, nil
}

// bindLocked attaches a holder and invalidates grace if this Open reclaimed the key. Caller holds t.mu.
func (t *Table) bindLocked(e *slot) (id uint64, reclaimed bool) {
	if e.graceTimer != nil {
		e.graceTimer.Stop()
		e.graceTimer = nil
		e.graceGen++
		reclaimed = true
	}
	e.nextID++
	e.holders[e.nextID] = struct{}{}
	return e.nextID, reclaimed
}

// logBind emits reclaim (if this Open stopped grace) and bind. Caller must not hold t.mu.
func (t *Table) logBind(key string, reclaimed bool) {
	if reclaimed {
		t.logger.Debug(MsgReclaim, "key", key)
	}
	t.logger.Debug(MsgBind, "key", key)
}

// watch waits until ctx is done, then drops that holder on this slot only.
func (t *Table) watch(key string, id uint64, e *slot, ctx context.Context) {
	waitCtx(ctx)
	t.drop(key, id, e)
}

// drop removes one holder from e and starts grace when none remain. A stale watcher (Reset or a newer slot) is ignored.
func (t *Table) drop(key string, id uint64, e *slot) {
	t.mu.Lock()
	cur, ok := t.items[key]
	if !ok || cur != e {
		t.mu.Unlock()
		return
	}
	delete(e.holders, id)
	if len(e.holders) > 0 || e.graceTimer != nil {
		t.mu.Unlock()
		return
	}

	// Last holder gone: arm grace or end immediately when grace is zero.
	e.graceGen++
	gen := e.graceGen
	if t.grace > 0 {
		e.graceTimer = time.AfterFunc(t.grace, func() { t.fire(key, e, gen) })
		t.mu.Unlock()
		t.logger.Debug(MsgOrphan, "key", key)
		return
	}
	t.mu.Unlock()
	t.logger.Debug(MsgOrphan, "key", key)
	t.fire(key, e, gen)
}

// fire cancels the incarnation lifetime if e is still the mapped slot, this grace generation is current, and no holders remain.
func (t *Table) fire(key string, e *slot, gen uint64) {
	t.mu.Lock()
	cur, ok := t.items[key]
	if !ok || cur != e || e.graceGen != gen || len(e.holders) > 0 {
		t.mu.Unlock()
		return
	}
	cancel := e.cancel
	delete(t.items, key)
	t.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	t.logger.Info(MsgDispose, "key", key)
}

// Reset stops grace timers and cancels every incarnation lifetime. Tests only.
func (t *Table) Reset() {
	if t == nil {
		return
	}
	t.mu.Lock()
	items := t.items
	t.items = map[string]*slot{}
	t.mu.Unlock()
	for key, e := range items {
		if e.graceTimer != nil {
			e.graceTimer.Stop()
			e.graceTimer = nil
		}
		e.graceGen++
		if e.cancel != nil {
			e.cancel()
		}
		t.logger.Info(MsgDispose, "key", key)
	}
}

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
}

// NewTable builds an empty table. Non-positive grace becomes DefaultGrace; a nil logger becomes slog.Default.
func NewTable(grace time.Duration, logger *slog.Logger) *Table {
	if grace <= 0 {
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

// Open returns the stored value for key, creating it once, and tracks ctx until it is Done.
// create receives a lifetime context that is canceled when this incarnation ends.
func (t *Table) Open(ctx context.Context, key string, create func(life context.Context) (any, error)) (any, error) {
	if t == nil {
		return nil, fmt.Errorf("reclaim: open %q: nil table", key)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// Reuse a live or in-grace incarnation.
	t.mu.Lock()
	if e, ok := t.items[key]; ok {
		id := t.bindLocked(key, e)
		v := e.value
		t.mu.Unlock()
		go t.watch(key, id, ctx)
		return v, nil
	}
	t.mu.Unlock()

	// Create outside the lock so two first Opens can race.
	life, cancel := context.WithCancel(context.Background())
	v, err := create(life)
	if err != nil {
		cancel()
		return nil, err
	}

	t.mu.Lock()
	// Another Open won: keep the stored value and cancel this extra create.
	if e, ok := t.items[key]; ok {
		id := t.bindLocked(key, e)
		exist := e.value
		t.mu.Unlock()
		cancel()
		go t.watch(key, id, ctx)
		return exist, nil
	}

	// First put for this key.
	e := &slot{
		value:   v,
		cancel:  cancel,
		holders: map[uint64]struct{}{},
	}
	t.items[key] = e
	t.logger.Info(MsgPut, "key", key)
	id := t.bindLocked(key, e)
	t.mu.Unlock()
	go t.watch(key, id, ctx)
	return v, nil
}

// bindLocked attaches a holder and cancels grace if this Open reclaimed the key. Caller holds t.mu.
func (t *Table) bindLocked(key string, e *slot) uint64 {
	reclaimed := false
	if e.graceTimer != nil {
		e.graceTimer.Stop()
		e.graceTimer = nil
		reclaimed = true
	}
	e.nextID++
	e.holders[e.nextID] = struct{}{}
	if reclaimed {
		t.logger.Debug(MsgReclaim, "key", key)
	}
	t.logger.Debug(MsgBind, "key", key)
	return e.nextID
}

// watch waits until ctx is Done, then drops that holder.
func (t *Table) watch(key string, id uint64, ctx context.Context) {
	<-ctx.Done()
	t.drop(key, id)
}

// drop removes one holder and starts grace when none remain.
func (t *Table) drop(key string, id uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	e, ok := t.items[key]
	if !ok {
		return
	}
	delete(e.holders, id)
	if len(e.holders) > 0 || e.graceTimer != nil {
		return
	}
	t.logger.Debug(MsgOrphan, "key", key)
	e.graceTimer = time.AfterFunc(t.grace, func() { t.fire(key) })
}

// fire cancels the incarnation lifetime if it is still orphaned when grace ends.
func (t *Table) fire(key string) {
	t.mu.Lock()
	e, ok := t.items[key]
	if !ok || len(e.holders) > 0 {
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
	for _, e := range items {
		if e.graceTimer != nil {
			e.graceTimer.Stop()
		}
		if e.cancel != nil {
			e.cancel()
		}
	}
}

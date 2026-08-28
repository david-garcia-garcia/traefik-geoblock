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

// Table is keyed lease+storage. Values are any; the caller type-asserts.
type Table struct {
	mu     sync.Mutex
	grace  time.Duration
	logger *slog.Logger
	items  map[string]*slot
}

type slot struct {
	value      any
	dispose    func()
	holders    map[uint64]struct{}
	nextID     uint64
	graceTimer *time.Timer
}

// NewTable returns a table. grace <= 0 uses DefaultGrace. A nil logger uses slog.Default.
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

// Open returns the value for key, creating it once, and binds ctx as a holder.
func (t *Table) Open(ctx context.Context, key string, create func() (any, error), dispose func(any)) (any, error) {
	if t == nil {
		return nil, fmt.Errorf("reclaim: open %q: nil table", key)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	t.mu.Lock()
	if e, ok := t.items[key]; ok {
		id := t.bindLocked(key, e)
		v := e.value
		t.mu.Unlock()
		go t.watch(key, id, ctx)
		return v, nil
	}
	t.mu.Unlock()

	v, err := create()
	if err != nil {
		return nil, err
	}

	t.mu.Lock()
	if e, ok := t.items[key]; ok {
		id := t.bindLocked(key, e)
		exist := e.value
		t.mu.Unlock()
		if dispose != nil {
			dispose(v)
		}
		go t.watch(key, id, ctx)
		return exist, nil
	}
	e := &slot{
		value:   v,
		holders: map[uint64]struct{}{},
	}
	if dispose != nil {
		held := v
		d := dispose
		e.dispose = func() { d(held) }
	}
	t.items[key] = e
	t.logger.Info(MsgPut, "key", key)
	id := t.bindLocked(key, e)
	t.mu.Unlock()
	go t.watch(key, id, ctx)
	return v, nil
}

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

func (t *Table) watch(key string, id uint64, ctx context.Context) {
	<-ctx.Done()
	t.drop(key, id)
}

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

func (t *Table) fire(key string) {
	t.mu.Lock()
	e, ok := t.items[key]
	if !ok || len(e.holders) > 0 {
		t.mu.Unlock()
		return
	}
	disp := e.dispose
	delete(t.items, key)
	t.mu.Unlock()
	if disp != nil {
		disp()
	}
	t.logger.Info(MsgDispose, "key", key)
}

// Reset disposes every incarnation. Tests only.
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
		if e.dispose != nil {
			e.dispose()
		}
	}
}

package dbwrappers

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

var tablesMu sync.Mutex

const (
	DefaultGrace = 10 * time.Second

	MsgPut     = "reclaim_put"
	MsgBind    = "reclaim_bind"
	MsgOrphan  = "reclaim_orphan"
	MsgReclaim = "reclaim_reclaim"
	MsgDispose = "reclaim_dispose"
)

// Table is keyed lease+storage. Define it in the same package as T (Yaegi).
type Table[T any] struct {
	mu     sync.Mutex
	grace  time.Duration
	logger *slog.Logger
	items  map[string]*slot[T]
}

type slot[T any] struct {
	value      T
	dispose    func()
	holders    map[uint64]struct{}
	nextID     uint64
	graceTimer *time.Timer
}

// NewTable returns a table. grace <= 0 uses DefaultGrace. A nil logger uses slog.Default.
func NewTable[T any](grace time.Duration, logger *slog.Logger) *Table[T] {
	if grace <= 0 {
		grace = DefaultGrace
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Table[T]{
		grace:  grace,
		logger: logger,
		items:  map[string]*slot[T]{},
	}
}

// Open returns the value for key, creating it once, and binds ctx as a holder.
func (t *Table[T]) Open(ctx context.Context, key string, create func() (T, error), dispose func(T)) (T, error) {
	var zero T
	if t == nil {
		return zero, fmt.Errorf("reclaim: open %q: nil table", key)
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
		return zero, err
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
	e := &slot[T]{
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

func (t *Table[T]) bindLocked(key string, e *slot[T]) uint64 {
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

func (t *Table[T]) watch(key string, id uint64, ctx context.Context) {
	<-ctx.Done()
	t.drop(key, id)
}

func (t *Table[T]) drop(key string, id uint64) {
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

func (t *Table[T]) fire(key string) {
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
func (t *Table[T]) Reset() {
	if t == nil {
		return
	}
	t.mu.Lock()
	items := t.items
	t.items = map[string]*slot[T]{}
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

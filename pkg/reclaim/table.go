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
//	         Open, key absent
//	                |
//	            create()                 Wake()
//	                v                       |
//	  Open ------> AWAKE                    |
//	                |                       |
//	   last holder Done                     |
//	                v                       |
//	            Sleep()                     |
//	                v                       |
//	             ASLEEP ---- Open before ---+
//	                |         grace ends
//	   grace elapsed / Reset / grace == 0
//	                v
//	             Close()  key deleted
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

// slot is one incarnation: the value, what may be done with it, and how many holders need it.
type slot struct {
	value   any
	err     error
	state   slotState
	holders int
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

// sleepValue puts value to sleep if it has Sleep. Runs outside t.mu.
func sleepValue(value any) {
	if s, ok := value.(sleeper); ok {
		s.Sleep()
	}
}

// wakeValue wakes value if it has Wake. Runs outside t.mu, before Open returns.
func wakeValue(value any) {
	if w, ok := value.(waker); ok {
		w.Wake()
	}
}

// closeValue closes value if it has Close. Runs outside t.mu, always after sleepValue.
func closeValue(value any) {
	if c, ok := value.(closer); ok {
		c.Close()
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
		s, mapped := t.items[key]
		if !mapped {
			// Register the key before create runs, so a second first Open waits for this result
			// instead of creating a value that would be thrown away.
			s = &slot{state: slotBusy, ready: make(chan struct{}), logger: logger}
			t.items[key] = s
			t.mu.Unlock()
			return t.put(ctx, key, s, logger, create)
		}
		s.logger = logger

		switch s.state {
		case slotAwake:
			s.holders++
			value := s.value
			t.mu.Unlock()
			logger.Debug(MsgBind, "key", key)
			go t.watch(key, s, ctx)
			return value, nil
		case slotAsleep:
			return t.reclaim(ctx, key, s, logger), nil
		case slotBusy:
			// A create, wake, or sleep owns the slot. Wait for it, then look again.
			ready := s.ready
			t.mu.Unlock()
			<-ready
			t.mu.Lock()
			err := s.err
			t.mu.Unlock()
			if err != nil {
				return nil, err
			}
		}
	}
}

// put runs create for a slot this Open registered, then publishes the value or the failure to
// every caller waiting on that slot.
func (t *Table) put(ctx context.Context, key string, s *slot, logger *slog.Logger, create func() (any, error)) (any, error) {
	value, err := create()

	t.mu.Lock()
	if err != nil {
		s.err = err
		s.state = slotGone
		if t.items[key] == s {
			delete(t.items, key)
		}
		close(s.ready)
		t.mu.Unlock()
		return nil, err
	}
	s.value = value
	s.state = slotAwake
	s.holders++
	// Reset is tests only and must not race Open on a key, but if it did it dropped this slot
	// while create ran. Take the key back rather than strand a value nobody can close.
	t.items[key] = s
	close(s.ready)
	t.mu.Unlock()

	logger.Debug(MsgPut, "key", key)
	logger.Debug(MsgBind, "key", key)
	go t.watch(key, s, ctx)
	return value, nil
}

// reclaim wakes a sleeping slot for this Open and binds ctx. The caller holds t.mu and has seen
// slotAsleep; reclaim releases it, because Wake must not run under the table mutex.
func (t *Table) reclaim(ctx context.Context, key string, s *slot, logger *slog.Logger) any {
	s.state = slotBusy
	s.ready = make(chan struct{})
	if s.woken != nil {
		// End the grace wait: this incarnation is not being disposed after all.
		close(s.woken)
		s.woken = nil
	}
	s.holders++
	value := s.value
	t.mu.Unlock()

	wakeValue(value)

	t.mu.Lock()
	s.state = slotAwake
	close(s.ready)
	t.mu.Unlock()

	logger.Debug(MsgReclaim, "key", key)
	logger.Debug(MsgBind, "key", key)
	go t.watch(key, s, ctx)
	return value
}

// watch waits until ctx is done, then drops that holder from this slot.
func (t *Table) watch(key string, s *slot, ctx context.Context) {
	waitCtx(ctx)
	t.drop(key, s)
}

// drop removes one holder. When it was the last one, this goroutine ends the incarnation: sleep,
// orphan, grace, close, dispose — in that order, so those lines cannot be reordered. A watcher
// whose incarnation is already gone finds slotGone and returns.
func (t *Table) drop(key string, s *slot) {
	t.mu.Lock()
	s.holders--
	for s.state == slotBusy {
		// A create, wake, or sleep owns the slot. Wait for it before deciding to sleep.
		ready := s.ready
		t.mu.Unlock()
		<-ready
		t.mu.Lock()
	}
	if s.holders > 0 || s.state != slotAwake {
		t.mu.Unlock()
		return
	}

	s.state = slotBusy
	s.ready = make(chan struct{})
	s.woken = make(chan struct{})
	woken := s.woken
	value, logger := s.value, s.logger
	grace := t.grace
	t.mu.Unlock()

	sleepValue(value)

	t.mu.Lock()
	s.state = slotAsleep
	close(s.ready)
	mapped := t.items[key] == s
	if mapped && grace <= 0 {
		// Zero grace keeps nothing: unmap now, so no Open can ever see this value asleep.
		delete(t.items, key)
		mapped = false
	}
	t.mu.Unlock()
	logger.Debug(MsgOrphan, "key", key)

	// Not mapped means zero grace, or Reset dropped this slot: either way it is ours to close.
	if !mapped {
		t.expire(key, s)
		return
	}

	wait := time.NewTimer(grace)
	defer wait.Stop()
	select {
	case <-wait.C:
		t.expire(key, s)
	case <-woken:
	}
}

// expire ends a sleeping incarnation, unless an Open woke it or something else already claimed
// it. It is the only place that closes a value the table still had mapped.
func (t *Table) expire(key string, s *slot) {
	t.mu.Lock()
	if s.state != slotAsleep || s.holders > 0 {
		t.mu.Unlock()
		return
	}
	s.state = slotGone
	if t.items[key] == s {
		delete(t.items, key)
	}
	value, logger := s.value, s.logger
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
	for _, s := range items {
		if s.woken != nil {
			close(s.woken)
			s.woken = nil
		}
	}
	t.mu.Unlock()

	for key, s := range items {
		t.mu.Lock()
		state, value, logger := s.state, s.value, s.logger
		if state == slotAwake || state == slotAsleep {
			s.state = slotGone
		}
		t.mu.Unlock()

		switch state {
		case slotAwake:
			sleepValue(value)
			logger.Debug(MsgOrphan, "key", key)
			dispose(key, value, logger)
		case slotAsleep:
			dispose(key, value, logger)
		}
	}
}

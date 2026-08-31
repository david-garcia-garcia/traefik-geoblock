package reclaim

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const waitBudget = 2 * time.Second

// box is a disposable stand-in stored on the table in tests.
type box struct {
	n     int
	ended *atomic.Bool
}

// Close marks ended when the table stops this incarnation.
func (b *box) Close() {
	if b.ended != nil {
		b.ended.Store(true)
	}
}

// ending is a box that sets done when Close runs.
func ending(n int, done *atomic.Bool) *box {
	return &box{n: n, ended: done}
}

// namedEnd appends name to ended when the table stops that incarnation.
type namedEnd struct {
	name  string
	mu    *sync.Mutex
	ended *[]string
}

// Close records this name as stopped.
func (n *namedEnd) Close() {
	n.mu.Lock()
	*n.ended = append(*n.ended, n.name)
	n.mu.Unlock()
}

// recHandler records slog lines so tests can assert reclaim msg + key + level.
type recHandler struct {
	mu   sync.Mutex
	recs []slog.Record
}

// Enabled keeps every level so debug reclaim lines are captured.
func (h *recHandler) Enabled(context.Context, slog.Level) bool { return true }

// Handle stores a clone of the record.
func (h *recHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	h.recs = append(h.recs, r.Clone())
	h.mu.Unlock()
	return nil
}

// WithAttrs returns the same handler; tests do not use slog attributes on the handler itself.
func (h *recHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

// WithGroup returns the same handler; tests do not use slog groups.
func (h *recHandler) WithGroup(string) slog.Handler { return h }

// events is msg + key for each recorded line, in order.
func (h *recHandler) events() [][2]string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([][2]string, 0, len(h.recs))
	for _, r := range h.recs {
		var key string
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "key" {
				key = a.Value.String()
			}
			return true
		})
		out = append(out, [2]string{r.Message, key})
	}
	return out
}

// keySeq is the message sequence for one key.
func keySeq(ev [][2]string, key string) []string {
	var out []string
	for _, e := range ev {
		if e[1] == key {
			out = append(out, e[0])
		}
	}
	return out
}

// countKeyMsg counts one message for one key.
func countKeyMsg(ev [][2]string, msg, key string) int {
	n := 0
	for _, e := range ev {
		if e[0] == msg && e[1] == key {
			n++
		}
	}
	return n
}

// waitUntil fails if cond is still false after waitBudget.
func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(waitBudget)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timeout waiting for condition")
}

// mustSlotA returns the mapped slot for key a or fails.
func mustSlotA(t *testing.T, tab *Table) *slot {
	t.Helper()
	tab.mu.Lock()
	defer tab.mu.Unlock()
	e := tab.items["a"]
	if e == nil {
		t.Fatal("missing slot a")
	}
	return e
}

// waitKeyMsg waits until msg is logged for key.
func waitKeyMsg(t *testing.T, h *recHandler, msg, key string) {
	t.Helper()
	waitUntil(t, func() bool { return countKeyMsg(h.events(), msg, key) > 0 })
}

// requireLevels checks spec log levels on every recorded reclaim line.
func (h *recHandler) requireLevels(t *testing.T) {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, r := range h.recs {
		switch r.Message {
		case MsgPut, MsgDispose, MsgBind, MsgOrphan, MsgReclaim:
			if r.Level != slog.LevelDebug {
				t.Fatalf("%s level %v want debug", r.Message, r.Level)
			}
		}
	}
}

// TestTable_OpenCancelDispose checks that cancel plus grace cancels the lifetime once and logs the exact sequence.
func TestTable_OpenCancelDispose(t *testing.T) {
	h := &recHandler{}
	grace := 20 * time.Millisecond
	tab := NewTable(grace)
	var ended atomic.Bool
	ctx, cancel := context.WithCancel(context.Background())

	if _, err := tab.Open(ctx, "a", slog.New(h), func() (any, error) {
		return ending(1, &ended), nil
	}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	start := time.Now()
	cancel()
	waitUntil(t, ended.Load)
	if time.Since(start) < grace*3/4 {
		t.Fatalf("canceled too early: %v", time.Since(start))
	}
	if !reflect.DeepEqual(keySeq(h.events(), "a"), []string{MsgPut, MsgBind, MsgOrphan, MsgDispose}) {
		t.Fatalf("events: %+v", h.events())
	}
	if countKeyMsg(h.events(), MsgDispose, "a") != 1 {
		t.Fatalf("dispose count: %+v", h.events())
	}
	h.requireLevels(t)
}

// TestTable_OpenDuringGraceReclaims checks that Open before grace keeps the incarnation and returns it.
func TestTable_OpenDuringGraceReclaims(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(80 * time.Millisecond)
	var ended atomic.Bool
	ctx1, cancel1 := context.WithCancel(context.Background())
	first, err := tab.Open(ctx1, "a", slog.New(h), func() (any, error) {
		return ending(1, &ended), nil
	})
	if err != nil {
		t.Fatalf("Open 1: %v", err)
	}

	cancel1()
	waitKeyMsg(t, h, MsgOrphan, "a")
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	second, err := tab.Open(ctx2, "a", slog.New(h), func() (any, error) {
		return &box{n: 2}, nil
	})
	if err != nil {
		t.Fatalf("Open 2: %v", err)
	}
	if first != second {
		t.Fatal("reclaim must return the stored value")
	}

	waitUntil(t, func() bool { return countKeyMsg(h.events(), MsgReclaim, "a") == 1 })
	deadline := time.Now().Add(160 * time.Millisecond)
	for time.Now().Before(deadline) {
		if ended.Load() {
			t.Fatal("reclaim must not cancel the lifetime")
		}
		time.Sleep(time.Millisecond)
	}
	if !reflect.DeepEqual(keySeq(h.events(), "a"), []string{MsgPut, MsgBind, MsgOrphan, MsgReclaim, MsgBind}) {
		t.Fatalf("events: %+v", h.events())
	}
	h.requireLevels(t)
}

// TestTable_SecondCreateDisposeIgnored checks that a later Open does not run create or replace the lifetime.
func TestTable_SecondCreateDisposeIgnored(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(20 * time.Millisecond)
	var created atomic.Int32
	var ended atomic.Bool
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx1, "a", slog.New(h), func() (any, error) {
		created.Add(1)
		return ending(1, &ended), nil
	}); err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	if _, err := tab.Open(ctx2, "a", slog.New(h), func() (any, error) {
		created.Add(1)
		return ending(2, &ended), nil
	}); err != nil {
		t.Fatalf("Open 2: %v", err)
	}

	cancel1()
	cancel2()
	waitUntil(t, ended.Load)
	if created.Load() != 1 {
		t.Fatalf("second Open must not run create, created=%d", created.Load())
	}
	h.requireLevels(t)
}

// TestTable_TwoOpensOneDispose checks that one live holder blocks lifetime cancel until the last ctx is Done.
func TestTable_TwoOpensOneDispose(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(20 * time.Millisecond)
	var ended atomic.Bool
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx1, "a", slog.New(h), func() (any, error) {
		return ending(1, &ended), nil
	}); err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	if _, err := tab.Open(ctx2, "a", slog.New(h), func() (any, error) {
		return &box{n: 2}, nil
	}); err != nil {
		t.Fatalf("Open 2: %v", err)
	}

	cancel1()
	deadline := time.Now().Add(80 * time.Millisecond)
	for time.Now().Before(deadline) {
		if ended.Load() {
			t.Fatal("one live open must keep the incarnation")
		}
		time.Sleep(time.Millisecond)
	}

	cancel2()
	waitUntil(t, ended.Load)
	h.requireLevels(t)
}

// TestTable_NegativeGraceUsesDefault checks that a negative grace becomes DefaultGrace.
func TestTable_NegativeGraceUsesDefault(t *testing.T) {
	tab := NewTable(-1)
	if tab.grace != DefaultGrace {
		t.Fatalf("grace: %v", tab.grace)
	}
}

// TestTable_ZeroGraceEndsImmediately checks that zero grace cancels as soon as the last holder is gone.
func TestTable_ZeroGraceEndsImmediately(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(0)
	if tab.grace != 0 {
		t.Fatalf("grace: %v", tab.grace)
	}
	var ended atomic.Bool
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx, "a", slog.New(h), func() (any, error) {
		return ending(1, &ended), nil
	}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	cancel()
	waitUntil(t, ended.Load)
	if !reflect.DeepEqual(keySeq(h.events(), "a"), []string{MsgPut, MsgBind, MsgOrphan, MsgDispose}) {
		t.Fatalf("events: %+v", h.events())
	}
	h.requireLevels(t)
}

// TestTable_StdlibImports checks that table.go and default.go import only the standard library.
func TestTable_StdlibImports(t *testing.T) {
	for _, name := range []string{"table.go", "default.go"} {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(path, ".") {
				t.Fatalf("%s: non-stdlib import %s", name, path)
			}
		}
	}
}

// TestTable_HashChangeProof checks that canceling key A’s lifetime does not cancel a live key B.
func TestTable_HashChangeProof(t *testing.T) {
	h := &recHandler{}
	grace := 20 * time.Millisecond
	tab := NewTable(grace)
	var ended []string
	var mu sync.Mutex

	ctxA, cancelA := context.WithCancel(context.Background())
	if _, err := tab.Open(ctxA, "A", slog.New(h), func() (any, error) {
		return &namedEnd{name: "A", mu: &mu, ended: &ended}, nil
	}); err != nil {
		t.Fatalf("Open A: %v", err)
	}

	start := time.Now()
	cancelA()
	ctxB, cancelB := context.WithCancel(context.Background())
	defer cancelB()
	if _, err := tab.Open(ctxB, "B", slog.New(h), func() (any, error) {
		return &namedEnd{name: "B", mu: &mu, ended: &ended}, nil
	}); err != nil {
		t.Fatalf("Open B: %v", err)
	}

	waitKeyMsg(t, h, MsgDispose, "A")
	if time.Since(start) < grace*3/4 {
		t.Fatalf("A disposed before grace: %v", time.Since(start))
	}
	mu.Lock()
	got := append([]string(nil), ended...)
	mu.Unlock()
	if len(got) != 1 || got[0] != "A" {
		t.Fatalf("ended: %v", got)
	}
	if !reflect.DeepEqual(keySeq(h.events(), "A"), []string{MsgPut, MsgBind, MsgOrphan, MsgDispose}) {
		t.Fatalf("A events: %+v", h.events())
	}
	if countKeyMsg(h.events(), MsgDispose, "B") != 0 {
		t.Fatalf("B must not dispose: %+v", h.events())
	}
	if countKeyMsg(h.events(), MsgPut, "B") != 1 {
		t.Fatalf("missing put B: %+v", h.events())
	}
	h.requireLevels(t)
}

// TestDefault_OpenSharesIncarnation checks that package Open and Default().Open are the same table.
func TestDefault_OpenSharesIncarnation(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, err := Open(ctx, "k", slog.Default(), func() (any, error) { return &box{n: 7}, nil })
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	b, err := Default().Open(ctx, "k", slog.Default(), func() (any, error) { return &box{n: 8}, nil })
	if err != nil {
		t.Fatalf("Default.Open: %v", err)
	}
	if a != b {
		t.Fatal("expected the process table to return the same value")
	}
	if a.(*box).n != 7 {
		t.Fatalf("create ran twice or wrong value: %+v", a)
	}
}

// TestTable_OpenNilContextPanics checks that a missing holder context is rejected.
func TestTable_OpenNilContextPanics(t *testing.T) {
	tab := NewTable(time.Millisecond)
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	_, _ = tab.Open(nil, "a", slog.Default(), func() (any, error) { return &box{n: 1}, nil }) //nolint:staticcheck // Open must panic on nil
}

// TestTable_OpenBackgroundDoesNotPanic checks that Background is accepted (Yaegi Done is often nil).
func TestTable_OpenBackgroundDoesNotPanic(t *testing.T) {
	tab := NewTable(time.Millisecond)
	ctx := context.Background()
	if _, err := tab.Open(ctx, "a", slog.Default(), func() (any, error) { return &box{n: 1}, nil }); err != nil {
		t.Fatalf("Open: %v", err)
	}
}

// TestTable_CreateErrorCancelsLife checks that a failed create does not store a slot and cancels life.
func TestTable_CreateErrorCancelsLife(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(20 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := tab.Open(ctx, "a", slog.New(h), func() (any, error) {
		return nil, errors.New("boom")
	})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("err: %v", err)
	}
	if countKeyMsg(h.events(), MsgPut, "a") != 0 {
		t.Fatalf("failed create must not put: %+v", h.events())
	}

	v, err := tab.Open(ctx, "a", slog.New(h), func() (any, error) { return &box{n: 2}, nil })
	if err != nil {
		t.Fatalf("retry Open: %v", err)
	}
	if v.(*box).n != 2 {
		t.Fatalf("retry value: %+v", v)
	}
}

// TestTable_LostCreateRaceCancelsLoser checks that a racing extra create is canceled and not stored.
func TestTable_LostCreateRaceCancelsLoser(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(20 * time.Millisecond)
	var started sync.WaitGroup
	started.Add(2)
	gate := make(chan struct{})

	var closed [2]atomic.Bool
	create := func(i int) func() (any, error) {
		return func() (any, error) {
			started.Done()
			<-gate
			return ending(i, &closed[i]), nil
		}
	}

	ctx0, cancel0 := context.WithCancel(context.Background())
	defer cancel0()
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()

	var got [2]any
	var errs [2]error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		got[0], errs[0] = tab.Open(ctx0, "a", slog.New(h), create(0))
	}()
	go func() {
		defer wg.Done()
		got[1], errs[1] = tab.Open(ctx1, "a", slog.New(h), create(1))
	}()
	started.Wait()
	close(gate)
	wg.Wait()

	if errs[0] != nil || errs[1] != nil {
		t.Fatalf("errs: %v %v", errs[0], errs[1])
	}
	if got[0] != got[1] {
		t.Fatal("both Opens must return the stored value")
	}
	if countKeyMsg(h.events(), MsgPut, "a") != 1 {
		t.Fatalf("one put: %+v", h.events())
	}

	if closed[0].Load() == closed[1].Load() {
		t.Fatalf("exactly one create must be discarded: %v %v", closed[0].Load(), closed[1].Load())
	}
	h.requireLevels(t)
}

// TestTable_ResetLogsDisposeAndKeepsNextIncarnation checks Reset logs dispose and stale watchers cannot drop a later Open.
func TestTable_ResetLogsDisposeAndKeepsNextIncarnation(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(20 * time.Millisecond)
	var firstEnded, secondEnded atomic.Bool
	ctx1, cancel1 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx1, "a", slog.New(h), func() (any, error) {
		return ending(1, &firstEnded), nil
	}); err != nil {
		t.Fatalf("Open 1: %v", err)
	}

	tab.Reset()
	waitUntil(t, firstEnded.Load)
	if countKeyMsg(h.events(), MsgDispose, "a") != 1 {
		t.Fatalf("Reset must log dispose: %+v", h.events())
	}

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	v, err := tab.Open(ctx2, "a", slog.New(h), func() (any, error) {
		return ending(2, &secondEnded), nil
	})
	if err != nil {
		t.Fatalf("Open 2: %v", err)
	}
	if v.(*box).n != 2 {
		t.Fatalf("value: %+v", v)
	}

	// Stale watcher from ctx1 must not orphan the new slot.
	cancel1()
	deadline := time.Now().Add(80 * time.Millisecond)
	for time.Now().Before(deadline) {
		if secondEnded.Load() {
			t.Fatal("stale drop must not cancel the new incarnation")
		}
		time.Sleep(time.Millisecond)
	}
	h.requireLevels(t)
}

// TestTable_ResetStopsArmedTimer checks that Reset during grace stops the timer and still logs dispose.
func TestTable_ResetStopsArmedTimer(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(time.Second)
	var ended atomic.Bool
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx, "a", slog.New(h), func() (any, error) {
		return ending(1, &ended), nil
	}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	cancel()
	waitKeyMsg(t, h, MsgOrphan, "a")
	tab.Reset()
	waitUntil(t, ended.Load)
	if countKeyMsg(h.events(), MsgDispose, "a") != 1 {
		t.Fatalf("events: %+v", h.events())
	}
	h.requireLevels(t)
}

// TestTable_ConcurrentOpenSameKeySharesOneIncarnation checks parallel Open on one key.
func TestTable_ConcurrentOpenSameKeySharesOneIncarnation(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(20 * time.Millisecond)
	const n = 8
	var created atomic.Int32
	ctxs := make([]context.Context, n)
	cancels := make([]context.CancelFunc, n)
	vals := make([]any, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		ctxs[i], cancels[i] = context.WithCancel(context.Background())
		go func(i int) {
			defer wg.Done()
			var err error
			vals[i], err = tab.Open(ctxs[i], "a", slog.New(h), func() (any, error) {
				created.Add(1)
				return &box{n: 1}, nil
			})
			if err != nil {
				t.Errorf("Open %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	if created.Load() < 1 {
		t.Fatal("create never ran")
	}
	for i := 1; i < n; i++ {
		if vals[i] != vals[0] {
			t.Fatalf("value %d differs", i)
		}
	}
	for _, c := range cancels {
		c()
	}
	waitKeyMsg(t, h, MsgDispose, "a")
	if countKeyMsg(h.events(), MsgDispose, "a") != 1 {
		t.Fatalf("one dispose: %+v", h.events())
	}
	h.requireLevels(t)
}

// TestTable_NilTableOpenErrors checks the nil-table guard.
func TestTable_NilTableOpenErrors(t *testing.T) {
	var tab *Table
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := tab.Open(ctx, "a", slog.Default(), func() (any, error) { return &box{n: 1}, nil }); err == nil {
		t.Fatal("expected error")
	}
}

// TestTable_NilOpenLoggerRejected checks that Open requires a logger.
func TestTable_NilOpenLoggerRejected(t *testing.T) {
	tab := NewTable(time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := tab.Open(ctx, "a", nil, func() (any, error) { return &box{n: 1}, nil }); err == nil {
		t.Fatal("expected nil logger error")
	}
}

// TestTable_StaleFireAfterReclaimNoops checks that fire with the orphan generation cannot dispose after reclaim.
func TestTable_StaleFireAfterReclaimNoops(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(time.Second)
	var ended atomic.Bool
	ctx1, cancel1 := context.WithCancel(context.Background())
	first, err := tab.Open(ctx1, "a", slog.New(h), func() (any, error) {
		return ending(1, &ended), nil
	})
	if err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	cancel1()
	waitKeyMsg(t, h, MsgOrphan, "a")
	e := mustSlotA(t, tab)
	gen := e.graceGen

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	second, err := tab.Open(ctx2, "a", slog.New(h), func() (any, error) { return &box{n: 2}, nil })
	if err != nil {
		t.Fatalf("Open 2: %v", err)
	}
	if first != second {
		t.Fatal("reclaim must return the stored value")
	}

	tab.fire("a", e, gen)
	if ended.Load() {
		t.Fatal("stale fire must not cancel the lifetime")
	}
	if countKeyMsg(h.events(), MsgDispose, "a") != 0 {
		t.Fatalf("stale fire must not dispose: %+v", h.events())
	}
	if mustSlotA(t, tab).value != first {
		t.Fatal("slot replaced")
	}
}

// TestTable_StaleFireWhileHeldNoops checks that fire is ignored while a holder is still live.
func TestTable_StaleFireWhileHeldNoops(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(20 * time.Millisecond)
	var ended atomic.Bool
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := tab.Open(ctx, "a", slog.New(h), func() (any, error) {
		return ending(1, &ended), nil
	}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	e := mustSlotA(t, tab)
	tab.fire("a", e, e.graceGen)
	if ended.Load() {
		t.Fatal("fire must not run while a holder is live")
	}
	if countKeyMsg(h.events(), MsgDispose, "a") != 0 {
		t.Fatalf("events: %+v", h.events())
	}
}

// TestTable_StaleFireAfterResetNoops checks that an old slot’s fire cannot drop a later incarnation.
func TestTable_StaleFireAfterResetNoops(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(20 * time.Millisecond)
	var firstEnded, secondEnded atomic.Bool
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	if _, err := tab.Open(ctx1, "a", slog.New(h), func() (any, error) {
		return ending(1, &firstEnded), nil
	}); err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	old := mustSlotA(t, tab)
	oldGen := old.graceGen
	tab.Reset()
	waitUntil(t, firstEnded.Load)

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	if _, err := tab.Open(ctx2, "a", slog.New(h), func() (any, error) {
		return ending(2, &secondEnded), nil
	}); err != nil {
		t.Fatalf("Open 2: %v", err)
	}

	tab.fire("a", old, oldGen)
	if secondEnded.Load() {
		t.Fatal("old fire must not cancel the new incarnation")
	}
	if countKeyMsg(h.events(), MsgDispose, "a") != 1 {
		t.Fatalf("only Reset dispose: %+v", h.events())
	}
}

// TestTable_ConcurrentCancelLastHolders checks that many holders ending together orphan and dispose once.
func TestTable_ConcurrentCancelLastHolders(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(15 * time.Millisecond)
	const n = 8
	var ended atomic.Bool
	cancels := make([]context.CancelFunc, n)
	for i := 0; i < n; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancels[i] = cancel
		if _, err := tab.Open(ctx, "a", slog.New(h), func() (any, error) {
			if i == 0 {
				return ending(1, &ended), nil
			}
			return &box{n: 1}, nil
		}); err != nil {
			t.Fatalf("Open %d: %v", i, err)
		}
	}

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			cancels[i]()
		}(i)
	}
	wg.Wait()
	waitUntil(t, ended.Load)
	if countKeyMsg(h.events(), MsgOrphan, "a") != 1 {
		t.Fatalf("one orphan: %+v", h.events())
	}
	if countKeyMsg(h.events(), MsgDispose, "a") != 1 {
		t.Fatalf("one dispose: %+v", h.events())
	}
	h.requireLevels(t)
}

// TestTable_ReclaimRacesFire keeps a holder across the grace edge so a late AfterFunc cannot dispose.
func TestTable_ReclaimRacesFire(t *testing.T) {
	const rounds = 40
	grace := 3 * time.Millisecond
	for i := 0; i < rounds; i++ {
		h := &recHandler{}
		tab := NewTable(grace)
		var ended atomic.Bool
		ctx1, cancel1 := context.WithCancel(context.Background())
		first, err := tab.Open(ctx1, "a", slog.New(h), func() (any, error) {
			return ending(1, &ended), nil
		})
		if err != nil {
			t.Fatalf("round %d Open 1: %v", i, err)
		}
		cancel1()
		waitKeyMsg(t, h, MsgOrphan, "a")

		ctx2, cancel2 := context.WithCancel(context.Background())
		second, err := tab.Open(ctx2, "a", slog.New(h), func() (any, error) { return &box{n: 2}, nil })
		if err != nil {
			cancel2()
			t.Fatalf("round %d Open 2: %v", i, err)
		}
		if first != second {
			cancel2()
			t.Fatalf("round %d reclaim lost the value", i)
		}

		deadline := time.Now().Add(grace * 6)
		for time.Now().Before(deadline) {
			if ended.Load() {
				cancel2()
				t.Fatalf("round %d late fire disposed a live incarnation", i)
			}
			time.Sleep(time.Millisecond)
		}
		cancel2()
		waitUntil(t, ended.Load)
	}
}

// TestTable_ZeroGraceOpenRacesCancel checks Open vs last-holder fire at grace 0 never drops a live holder.
func TestTable_ZeroGraceOpenRacesCancel(t *testing.T) {
	const rounds = 40
	for i := 0; i < rounds; i++ {
		h := &recHandler{}
		tab := NewTable(0)
		var ended atomic.Bool
		ctx1, cancel1 := context.WithCancel(context.Background())
		first, err := tab.Open(ctx1, "a", slog.New(h), func() (any, error) {
			return ending(1, &ended), nil
		})
		if err != nil {
			t.Fatalf("round %d Open 1: %v", i, err)
		}

		ctx2, cancel2 := context.WithCancel(context.Background())
		var second any
		var openErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			cancel1()
		}()
		go func() {
			defer wg.Done()
			second, openErr = tab.Open(ctx2, "a", slog.New(h), func() (any, error) { return &box{n: 2}, nil })
		}()
		wg.Wait()
		if openErr != nil {
			cancel2()
			t.Fatalf("round %d Open 2: %v", i, openErr)
		}

		// Same pointer: still the first incarnation. New pointer: fire won and create ran again.
		if second == first {
			if ended.Load() {
				cancel2()
				t.Fatalf("round %d shared value but life already canceled", i)
			}
		} else {
			waitUntil(t, ended.Load)
		}
		cancel2()
	}
}

// TestTable_ResetNil is a no-op on a nil table.
func TestTable_ResetNil(t *testing.T) {
	var tab *Table
	tab.Reset()
}

// levelGate records only lines at or above min.
type levelGate struct {
	min slog.Level
	recHandler
}

// Enabled is true when l is at or above min.
func (h *levelGate) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.min
}

// TestTable_OpenLoggerLevelGatesPutDispose checks that an info Open logger hides put/dispose and a debug logger shows them.
func TestTable_OpenLoggerLevelGatesPutDispose(t *testing.T) {
	infoH := &levelGate{min: slog.LevelInfo}
	tab := NewTable(time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx, "a", slog.New(infoH), func() (any, error) { return &box{n: 1}, nil }); err != nil {
		t.Fatalf("info Open: %v", err)
	}
	cancel()
	time.Sleep(30 * time.Millisecond)
	if countKeyMsg(infoH.events(), MsgPut, "a") != 0 || countKeyMsg(infoH.events(), MsgDispose, "a") != 0 {
		t.Fatalf("info logger leaked reclaim lines: %+v", infoH.events())
	}

	debugH := &levelGate{min: slog.LevelDebug}
	ctx2, cancel2 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx2, "b", slog.New(debugH), func() (any, error) { return &box{n: 2}, nil }); err != nil {
		t.Fatalf("debug Open: %v", err)
	}
	cancel2()
	waitKeyMsg(t, &debugH.recHandler, MsgPut, "b")
	waitKeyMsg(t, &debugH.recHandler, MsgDispose, "b")
}

package reclaim

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"log/slog"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// waitBudget guards a condition that should already be true; a loaded runner must not exhaust it.
const waitBudget = 10 * time.Second

// graceNoRace is long enough that a test asserting the reclaim branch cannot lose the timer race.
const graceNoRace = 5 * time.Second

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

// requireDisposeAfterClose waits for the dispose line on key and then checks the contract this
// change exists for: by the time dispose is logged, the value's Close has already returned.
// Wait for the line rather than the flag — the flag is set first, so it proves nothing on its own.
func requireDisposeAfterClose(t *testing.T, h *recHandler, key string, closed *atomic.Bool) {
	t.Helper()
	waitKeyMsg(t, h, MsgDispose, key)
	if !closed.Load() {
		t.Fatalf("dispose for %s must not be logged before Close ran", key)
	}
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

	if _, err := tab.Open(ctx, "a", slog.New(h), TableGrace, func() (any, error) {
		return ending(1, &ended), nil
	}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	start := time.Now()
	cancel()
	requireDisposeAfterClose(t, h, "a", &ended)
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
	tab := NewTable(graceNoRace)
	var ended atomic.Bool
	ctx1, cancel1 := context.WithCancel(context.Background())
	first, err := tab.Open(ctx1, "a", slog.New(h), TableGrace, func() (any, error) {
		return ending(1, &ended), nil
	})
	if err != nil {
		t.Fatalf("Open 1: %v", err)
	}

	cancel1()
	waitKeyMsg(t, h, MsgOrphan, "a")
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	second, err := tab.Open(ctx2, "a", slog.New(h), TableGrace, func() (any, error) {
		return &box{n: 2}, nil
	})
	if err != nil {
		t.Fatalf("Open 2: %v", err)
	}
	if first != second {
		t.Fatal("reclaim must return the stored value")
	}

	waitUntil(t, func() bool { return countKeyMsg(h.events(), MsgReclaim, "a") == 1 })
	// The armed timer must be gone, not merely slow: with a long grace, watching ended for a
	// window proves nothing, so read the state the reclaim was supposed to clear.
	e := mustSlotA(t, tab)
	tab.mu.Lock()
	armed := e.graceTimer != nil
	tab.mu.Unlock()
	if armed {
		t.Fatal("reclaim must invalidate the armed grace timer")
	}
	if ended.Load() {
		t.Fatal("reclaim must not end the incarnation")
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
	if _, err := tab.Open(ctx1, "a", slog.New(h), TableGrace, func() (any, error) {
		created.Add(1)
		return ending(1, &ended), nil
	}); err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	if _, err := tab.Open(ctx2, "a", slog.New(h), TableGrace, func() (any, error) {
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
	if _, err := tab.Open(ctx1, "a", slog.New(h), TableGrace, func() (any, error) {
		return ending(1, &ended), nil
	}); err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	if _, err := tab.Open(ctx2, "a", slog.New(h), TableGrace, func() (any, error) {
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

// TestTable_GraceIsPerIncarnation checks that the grace an Open names governs that key alone, and
// that a key opened with TableGrace on the same table still waits the table's grace.
func TestTable_GraceIsPerIncarnation(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(graceNoRace)
	defer tab.Reset()
	var fastEnded, slowEnded atomic.Bool

	fastCtx, cancelFast := context.WithCancel(context.Background())
	if _, err := tab.Open(fastCtx, "fast", slog.New(h), 0, func() (any, error) {
		return ending(1, &fastEnded), nil
	}); err != nil {
		cancelFast()
		t.Fatalf("Open fast: %v", err)
	}
	slowCtx, cancelSlow := context.WithCancel(context.Background())
	if _, err := tab.Open(slowCtx, "slow", slog.New(h), TableGrace, func() (any, error) {
		return ending(2, &slowEnded), nil
	}); err != nil {
		cancelFast()
		cancelSlow()
		t.Fatalf("Open slow: %v", err)
	}

	// The zero-grace key ends the moment its holder goes, on a table whose own grace is 5 s.
	cancelFast()
	requireDisposeAfterClose(t, h, "fast", &fastEnded)

	// The key that named no grace takes the table's, so it is still alive well past the other's end.
	cancelSlow()
	waitKeyMsg(t, h, MsgOrphan, "slow")
	time.Sleep(50 * time.Millisecond)
	if slowEnded.Load() {
		t.Fatal("a key opened with TableGrace must wait the table's grace, not another key's")
	}
	if got := countKeyMsg(h.events(), MsgDispose, "slow"); got != 0 {
		t.Fatalf("dispose slow = %d, want 0 while its own grace runs", got)
	}
}

// TestTable_OpenGraceBelowZeroTakesTheTable checks that TableGrace is a spelling, not a magic number:
// any negative grace means the table's.
func TestTable_OpenGraceBelowZeroTakesTheTable(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(graceNoRace)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := tab.Open(ctx, "a", slog.New(h), -time.Hour, func() (any, error) {
		return &box{n: 1}, nil
	}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	e := mustSlotA(t, tab)
	tab.mu.Lock()
	got := e.grace
	tab.mu.Unlock()
	if got != graceNoRace {
		t.Fatalf("grace = %v, want the table's %v", got, graceNoRace)
	}
}

// TestTable_ReclaimDoesNotChangeTheGrace checks that grace belongs to the incarnation: it is fixed by
// the Open that created the value, and an Open that reclaims it cannot shorten or extend it.
func TestTable_ReclaimDoesNotChangeTheGrace(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(time.Millisecond)
	defer tab.Reset()
	var ended atomic.Bool

	ctx1, cancel1 := context.WithCancel(context.Background())
	first, err := tab.Open(ctx1, "a", slog.New(h), graceNoRace, func() (any, error) {
		return ending(1, &ended), nil
	})
	if err != nil {
		cancel1()
		t.Fatalf("Open 1: %v", err)
	}
	cancel1()
	waitKeyMsg(t, h, MsgOrphan, "a")

	ctx2, cancel2 := context.WithCancel(context.Background())
	second, err := tab.Open(ctx2, "a", slog.New(h), 0, func() (any, error) {
		t.Error("an Open inside grace must reclaim, not create")
		return &box{n: 2}, nil
	})
	if err != nil {
		cancel2()
		t.Fatalf("Open 2: %v", err)
	}
	if second != first {
		cancel2()
		t.Fatal("expected a reclaim of the stored incarnation")
	}
	e := mustSlotA(t, tab)
	tab.mu.Lock()
	got := e.grace
	tab.mu.Unlock()
	if got != graceNoRace {
		t.Fatalf("grace = %v, want the creating Open's %v", got, graceNoRace)
	}

	// The reclaiming Open asked for zero grace. If that had taken, this second orphan would
	// dispose on the spot.
	cancel2()
	waitUntil(t, func() bool { return countKeyMsg(h.events(), MsgOrphan, "a") == 2 })
	time.Sleep(50 * time.Millisecond)
	if ended.Load() {
		t.Fatal("a reclaiming Open must not change the incarnation's grace")
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
	if _, err := tab.Open(ctx, "a", slog.New(h), TableGrace, func() (any, error) {
		return ending(1, &ended), nil
	}); err != nil {
		t.Fatalf("Open: %v", err)
	}
	cancel()
	requireDisposeAfterClose(t, h, "a", &ended)
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
	if _, err := tab.Open(ctxA, "A", slog.New(h), TableGrace, func() (any, error) {
		return &namedEnd{name: "A", mu: &mu, ended: &ended}, nil
	}); err != nil {
		t.Fatalf("Open A: %v", err)
	}

	start := time.Now()
	cancelA()
	ctxB, cancelB := context.WithCancel(context.Background())
	defer cancelB()
	if _, err := tab.Open(ctxB, "B", slog.New(h), TableGrace, func() (any, error) {
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
	a, err := Open(ctx, "k", slog.Default(), TableGrace, func() (any, error) { return &box{n: 7}, nil })
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	b, err := Default().Open(ctx, "k", slog.Default(), TableGrace, func() (any, error) { return &box{n: 8}, nil })
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
	_, _ = tab.Open(nil, "a", slog.Default(), TableGrace, func() (any, error) { return &box{n: 1}, nil }) //nolint:staticcheck // Open must panic on nil
}

// TestTable_OpenBackgroundDoesNotPanic checks that Background is accepted (Yaegi Done is often nil).
func TestTable_OpenBackgroundDoesNotPanic(t *testing.T) {
	tab := NewTable(time.Millisecond)
	ctx := context.Background()
	if _, err := tab.Open(ctx, "a", slog.Default(), TableGrace, func() (any, error) { return &box{n: 1}, nil }); err != nil {
		t.Fatalf("Open: %v", err)
	}
}

// TestTable_CreateErrorCancelsLife checks that a failed create does not store a slot and cancels life.
func TestTable_CreateErrorCancelsLife(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(20 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := tab.Open(ctx, "a", slog.New(h), TableGrace, func() (any, error) {
		return nil, errors.New("boom")
	})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("err: %v", err)
	}
	if countKeyMsg(h.events(), MsgPut, "a") != 0 {
		t.Fatalf("failed create must not put: %+v", h.events())
	}

	v, err := tab.Open(ctx, "a", slog.New(h), TableGrace, func() (any, error) { return &box{n: 2}, nil })
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
		got[0], errs[0] = tab.Open(ctx0, "a", slog.New(h), TableGrace, create(0))
	}()
	go func() {
		defer wg.Done()
		got[1], errs[1] = tab.Open(ctx1, "a", slog.New(h), TableGrace, create(1))
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
	if _, err := tab.Open(ctx1, "a", slog.New(h), TableGrace, func() (any, error) {
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
	v, err := tab.Open(ctx2, "a", slog.New(h), TableGrace, func() (any, error) {
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
	if _, err := tab.Open(ctx, "a", slog.New(h), TableGrace, func() (any, error) {
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
			vals[i], err = tab.Open(ctxs[i], "a", slog.New(h), TableGrace, func() (any, error) {
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
	if _, err := tab.Open(ctx, "a", slog.Default(), TableGrace, func() (any, error) { return &box{n: 1}, nil }); err == nil {
		t.Fatal("expected error")
	}
}

// TestTable_NilOpenLoggerRejected checks that Open requires a logger.
func TestTable_NilOpenLoggerRejected(t *testing.T) {
	tab := NewTable(time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := tab.Open(ctx, "a", nil, TableGrace, func() (any, error) { return &box{n: 1}, nil }); err == nil {
		t.Fatal("expected nil logger error")
	}
}

// TestTable_StaleFireAfterReclaimNoops checks that fire with the orphan generation cannot dispose after reclaim.
func TestTable_StaleFireAfterReclaimNoops(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(time.Second)
	var ended atomic.Bool
	ctx1, cancel1 := context.WithCancel(context.Background())
	first, err := tab.Open(ctx1, "a", slog.New(h), TableGrace, func() (any, error) {
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
	second, err := tab.Open(ctx2, "a", slog.New(h), TableGrace, func() (any, error) { return &box{n: 2}, nil })
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
	if _, err := tab.Open(ctx, "a", slog.New(h), TableGrace, func() (any, error) {
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
	if _, err := tab.Open(ctx1, "a", slog.New(h), TableGrace, func() (any, error) {
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
	if _, err := tab.Open(ctx2, "a", slog.New(h), TableGrace, func() (any, error) {
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
		if _, err := tab.Open(ctx, "a", slog.New(h), TableGrace, func() (any, error) {
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
	requireDisposeAfterClose(t, h, "a", &ended)
	if countKeyMsg(h.events(), MsgOrphan, "a") != 1 {
		t.Fatalf("one orphan: %+v", h.events())
	}
	if countKeyMsg(h.events(), MsgDispose, "a") != 1 {
		t.Fatalf("one dispose: %+v", h.events())
	}
	h.requireLevels(t)
}

// TestTable_ReclaimRacesFire opens a key while grace is expiring. Either side of that edge is
// correct, so this asserts what must hold for the side that happened: a reclaimed incarnation
// survives the queued AfterFunc, and a replaced one had its value closed.
func TestTable_ReclaimRacesFire(t *testing.T) {
	const rounds = 40
	grace := 3 * time.Millisecond
	for i := 0; i < rounds; i++ {
		h := &recHandler{}
		tab := NewTable(grace)
		var firstEnded, secondEnded atomic.Bool
		var creates atomic.Int32
		ctx1, cancel1 := context.WithCancel(context.Background())
		first, err := tab.Open(ctx1, "a", slog.New(h), TableGrace, func() (any, error) {
			creates.Add(1)
			return ending(1, &firstEnded), nil
		})
		if err != nil {
			t.Fatalf("round %d Open 1: %v", i, err)
		}
		cancel1()
		waitKeyMsg(t, h, MsgOrphan, "a")

		ctx2, cancel2 := context.WithCancel(context.Background())
		second, err := tab.Open(ctx2, "a", slog.New(h), TableGrace, func() (any, error) {
			creates.Add(1)
			return ending(2, &secondEnded), nil
		})
		if err != nil {
			cancel2()
			t.Fatalf("round %d Open 2: %v", i, err)
		}

		// Whichever side won, the incarnation the caller now holds must be the stored one and
		// must stay alive while ctx2 is live. Both flags are monotonic, so one wait past the
		// grace edge catches a wrong dispose as well as polling would.
		reclaimed := second == first
		if reclaimed {
			time.Sleep(grace * 2)
			if firstEnded.Load() {
				cancel2()
				t.Fatalf("round %d late fire disposed a live incarnation", i)
			}
			if n := creates.Load(); n != 1 {
				cancel2()
				t.Fatalf("round %d reclaim must not run create: %d creates", i, n)
			}
		} else {
			// Grace won: the first incarnation is closed and the replacement holds the key.
			waitUntil(t, firstEnded.Load)
			time.Sleep(grace * 2)
			if secondEnded.Load() {
				cancel2()
				t.Fatalf("round %d the replacement was disposed while its holder was live", i)
			}
		}
		tab.mu.Lock()
		cur, mapped := tab.items["a"]
		tab.mu.Unlock()
		if !mapped || cur.value != second {
			cancel2()
			t.Fatalf("round %d the returned value must be the stored incarnation (reclaimed=%v)", i, reclaimed)
		}

		cancel2()
		waitUntil(t, func() bool {
			events := h.events()
			return countKeyMsg(events, MsgDispose, "a") == countKeyMsg(events, MsgPut, "a")
		})
		if !firstEnded.Load() {
			t.Fatalf("round %d the first value was never closed", i)
		}
		if !reclaimed && !secondEnded.Load() {
			t.Fatalf("round %d the replacement value was never closed", i)
		}
	}
}

// TestTable_ZeroGraceOpenRacesCancel checks Open vs last-holder fire at grace 0 never drops a live holder.
func TestTable_ZeroGraceOpenRacesCancel(t *testing.T) {
	const rounds = 40
	for i := 0; i < rounds; i++ {
		h := &recHandler{}
		tab := NewTable(0)
		var ended, secondEnded atomic.Bool
		ctx1, cancel1 := context.WithCancel(context.Background())
		first, err := tab.Open(ctx1, "a", slog.New(h), TableGrace, func() (any, error) {
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
			second, openErr = tab.Open(ctx2, "a", slog.New(h), TableGrace, func() (any, error) {
				return ending(2, &secondEnded), nil
			})
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
			if secondEnded.Load() {
				cancel2()
				t.Fatalf("round %d the replacement was disposed while its holder was live", i)
			}
		}
		// Either way the value the caller holds is the one the table has stored.
		tab.mu.Lock()
		cur, mapped := tab.items["a"]
		tab.mu.Unlock()
		if !mapped || cur.value != second {
			cancel2()
			t.Fatalf("round %d the returned value must be the stored incarnation", i)
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
	if _, err := tab.Open(ctx, "a", slog.New(infoH), TableGrace, func() (any, error) { return &box{n: 1}, nil }); err != nil {
		t.Fatalf("info Open: %v", err)
	}
	cancel()
	time.Sleep(30 * time.Millisecond)
	if countKeyMsg(infoH.events(), MsgPut, "a") != 0 || countKeyMsg(infoH.events(), MsgDispose, "a") != 0 {
		t.Fatalf("info logger leaked reclaim lines: %+v", infoH.events())
	}

	debugH := &levelGate{min: slog.LevelDebug}
	ctx2, cancel2 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx2, "b", slog.New(debugH), TableGrace, func() (any, error) { return &box{n: 2}, nil }); err != nil {
		t.Fatalf("debug Open: %v", err)
	}
	cancel2()
	waitKeyMsg(t, &debugH.recHandler, MsgPut, "b")
	waitKeyMsg(t, &debugH.recHandler, MsgDispose, "b")
}

// closeProbe is a stored value that records whether the dispose line for its key
// had already been logged at the moment the table called Close.
type closeProbe struct {
	handler    *recHandler
	key        string
	closed     atomic.Bool
	sawDispose atomic.Bool
}

// Close records the dispose state seen from inside Close.
func (p *closeProbe) Close() {
	p.sawDispose.Store(countKeyMsg(p.handler.events(), MsgDispose, p.key) > 0)
	p.closed.Store(true)
}

// TestTable_DisposeLogFollowsClose checks that grace-path dispose is emitted only after Close returned.
func TestTable_DisposeLogFollowsClose(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(5 * time.Millisecond)
	probe := &closeProbe{handler: h, key: "a"}
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx, "a", slog.New(h), TableGrace, func() (any, error) { return probe, nil }); err != nil {
		t.Fatalf("Open: %v", err)
	}

	cancel()
	waitKeyMsg(t, h, MsgDispose, "a")
	if !probe.closed.Load() {
		t.Fatal("dispose was logged before Close ran")
	}
	if probe.sawDispose.Load() {
		t.Fatal("Close ran after the dispose line, so dispose is not a completion signal")
	}
}

// TestTable_ResetDisposeLogFollowsClose checks the same order on the Reset path, synchronously.
func TestTable_ResetDisposeLogFollowsClose(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(time.Minute)
	probe := &closeProbe{handler: h, key: "a"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := tab.Open(ctx, "a", slog.New(h), TableGrace, func() (any, error) { return probe, nil }); err != nil {
		t.Fatalf("Open: %v", err)
	}

	tab.Reset()
	if !probe.closed.Load() {
		t.Fatal("Reset returned before Close ran")
	}
	if probe.sawDispose.Load() {
		t.Fatal("Close ran after the dispose line on the Reset path")
	}
	if countKeyMsg(h.events(), MsgDispose, "a") != 1 {
		t.Fatalf("Reset must log one dispose: %+v", h.events())
	}
}

// TestTable_OrphanPrecedesDisposeAtTinyGrace checks the log order when the grace timer fires
// as soon as it is armed, which is where an orphan line logged after arming would lose.
func TestTable_OrphanPrecedesDisposeAtTinyGrace(t *testing.T) {
	const rounds = 200
	for i := 0; i < rounds; i++ {
		h := &recHandler{}
		tab := NewTable(time.Nanosecond)
		ctx, cancel := context.WithCancel(context.Background())
		if _, err := tab.Open(ctx, "a", slog.New(h), TableGrace, func() (any, error) { return &box{n: 1}, nil }); err != nil {
			cancel()
			t.Fatalf("round %d Open: %v", i, err)
		}
		cancel()
		waitKeyMsg(t, h, MsgDispose, "a")
		if got := keySeq(h.events(), "a"); !reflect.DeepEqual(got, []string{MsgPut, MsgBind, MsgOrphan, MsgDispose}) {
			t.Fatalf("round %d events: %v", i, got)
		}
	}
}

// TestTable_BindWhileArmingReclaims checks that an Open landing in the window between the
// orphan line and the armed timer is a reclaim, and stops that arming from happening.
func TestTable_BindWhileArmingReclaims(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := tab.Open(ctx, "a", slog.New(h), TableGrace, func() (any, error) { return &box{n: 1}, nil }); err != nil {
		t.Fatalf("Open: %v", err)
	}
	e := mustSlotA(t, tab)

	tab.mu.Lock()
	e.arming = true
	genBeforeBind := e.graceGen
	_, reclaimed := tab.bindLocked(e)
	genAfterBind := e.graceGen
	tab.mu.Unlock()

	if !reclaimed {
		t.Fatal("a bind inside the arming window must report a reclaim")
	}
	if genAfterBind == genBeforeBind {
		t.Fatal("a bind inside the arming window must bump the grace generation so the arm is skipped")
	}
}

// gateHandler records like recHandler and holds the first line whose message is gate,
// so a test can stop the table inside the window between the orphan line and the arm.
type gateHandler struct {
	recHandler
	gate    string
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

// Handle records the line, then blocks the first gate line until the test releases it.
func (h *gateHandler) Handle(ctx context.Context, r slog.Record) error {
	err := h.recHandler.Handle(ctx, r)
	if r.Message == h.gate {
		h.once.Do(func() {
			close(h.entered)
			<-h.release
		})
	}
	return err
}

// TestTable_ReclaimAndReleaseInsideOrphanWindowStillArms pins the interleaving that can strand an
// incarnation: a drop claims the orphan window, an Open reclaims and its already-Done holder drops
// again inside that window, so that second drop returns early. The first drop still owns the arm,
// and without it the slot would stay mapped forever with no holders, no timer, and Close never run.
func TestTable_ReclaimAndReleaseInsideOrphanWindowStillArms(t *testing.T) {
	h := &gateHandler{gate: MsgOrphan, entered: make(chan struct{}), release: make(chan struct{})}
	tab := NewTable(5 * time.Millisecond)
	var ended atomic.Bool

	ctx1, cancel1 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx1, "a", slog.New(h), TableGrace, func() (any, error) {
		return ending(1, &ended), nil
	}); err != nil {
		cancel1()
		t.Fatalf("Open 1: %v", err)
	}
	e := mustSlotA(t, tab)

	// Hold the first drop between the orphan line and the arm.
	cancel1()
	<-h.entered

	// Reclaim inside that window with a holder that is already Done, so its own drop runs
	// while arming is still set and returns early.
	doneCtx, cancelDone := context.WithCancel(context.Background())
	cancelDone()
	if _, err := tab.Open(doneCtx, "a", slog.New(h), TableGrace, func() (any, error) { return &box{n: 2}, nil }); err != nil {
		close(h.release)
		t.Fatalf("Open 2: %v", err)
	}
	waitUntil(t, func() bool {
		tab.mu.Lock()
		defer tab.mu.Unlock()
		return len(e.holders) == 0
	})

	close(h.release)
	waitKeyMsg(t, &h.recHandler, MsgDispose, "a")
	if !ended.Load() {
		t.Fatal("the stranded incarnation must still be closed")
	}
	tab.mu.Lock()
	_, mapped := tab.items["a"]
	tab.mu.Unlock()
	if mapped {
		t.Fatal("the key must be evicted, not left mapped with no holders and no timer")
	}
}

// TestTable_OpenInsideOrphanWindowKeepsIncarnationLive drives the arming window through drop
// rather than reaching into bindLocked: the drop is held between the orphan line and the arm,
// an Open binds a live holder there, and the drop then finds the slot held again. The incarnation
// must survive with no timer armed, because a timer armed here would dispose a value in use.
func TestTable_OpenInsideOrphanWindowKeepsIncarnationLive(t *testing.T) {
	const grace = 5 * time.Millisecond
	h := &gateHandler{gate: MsgOrphan, entered: make(chan struct{}), release: make(chan struct{})}
	tab := NewTable(grace)
	var ended atomic.Bool

	ctx1, cancel1 := context.WithCancel(context.Background())
	first, err := tab.Open(ctx1, "a", slog.New(h), TableGrace, func() (any, error) { return ending(1, &ended), nil })
	if err != nil {
		cancel1()
		t.Fatalf("Open 1: %v", err)
	}
	e := mustSlotA(t, tab)

	// Hold the drop inside the orphan window, then bind a live holder there.
	cancel1()
	<-h.entered
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	second, err := tab.Open(ctx2, "a", slog.New(h), TableGrace, func() (any, error) {
		t.Error("an Open inside the orphan window must reclaim, not create")
		return &box{n: 2}, nil
	})
	if err != nil {
		close(h.release)
		t.Fatalf("Open 2: %v", err)
	}
	if second != first {
		close(h.release)
		t.Fatal("an Open inside the orphan window must return the same incarnation")
	}
	close(h.release)

	// arming clears when the held drop re-locks, which is where it decides whether to arm.
	waitUntil(t, func() bool {
		tab.mu.Lock()
		defer tab.mu.Unlock()
		return !e.arming
	})
	tab.mu.Lock()
	cur, mapped := tab.items["a"]
	armed := e.graceTimer != nil
	holders := len(e.holders)
	tab.mu.Unlock()
	if !mapped || cur != e {
		t.Fatal("the reclaimed incarnation must stay mapped")
	}
	if armed {
		t.Fatal("a drop that finds the slot held again must not arm grace")
	}
	if holders != 1 {
		t.Fatalf("holders = %d, want the reclaiming Open's holder", holders)
	}

	// Nothing is scheduled, so give a would-be timer several grace periods to prove itself.
	time.Sleep(20 * grace)
	if ended.Load() {
		t.Fatal("the value must not be closed while a holder is live")
	}
	ev := h.events()
	if countKeyMsg(ev, MsgDispose, "a") != 0 {
		t.Fatalf("no dispose is due while a holder is live: %+v", ev)
	}
	// The gate is what pins this order: it holds the orphan line until Open 2 has bound.
	if got := strings.Join(keySeq(ev, "a"), ","); got != "reclaim_put,reclaim_bind,reclaim_orphan,reclaim_reclaim,reclaim_bind" {
		t.Fatalf("key sequence = %s", got)
	}
}

// TestTable_GoroutinesReturnToBaseline checks that the watcher and lifetime goroutines a key
// costs are all gone once its holders are Done and the incarnation has ended.
func TestTable_GoroutinesReturnToBaseline(t *testing.T) {
	const keys = 50
	// The two goroutines per key are joined deterministically now, so the only slack needed is for
	// runtime goroutines that come and go around the sample, not for a share of the ~100 started.
	const slack = 2
	tab := NewTable(time.Millisecond)
	h := &recHandler{}
	base := settledGoroutines()

	ctx, cancel := context.WithCancel(context.Background())
	for i := 0; i < keys; i++ {
		key := "k" + strconv.Itoa(i)
		if _, err := tab.Open(ctx, key, slog.New(h), TableGrace, func() (any, error) { return &box{n: i}, nil }); err != nil {
			cancel()
			t.Fatalf("Open %s: %v", key, err)
		}
	}

	cancel()
	waitUntil(t, func() bool { return countMsg(h.events(), MsgDispose) == keys })
	waitUntil(t, func() bool { return runtime.NumGoroutine() <= base+slack })
}

// settledGoroutines is the goroutine count after the runtime has had a moment to retire finished ones.
func settledGoroutines() int {
	time.Sleep(20 * time.Millisecond)
	return runtime.NumGoroutine()
}

// countMsg counts one message across every key.
func countMsg(ev [][2]string, msg string) int {
	n := 0
	for _, e := range ev {
		if e[0] == msg {
			n++
		}
	}
	return n
}

// nilDoneCtx is a holder context with no Done channel, the shape Yaegi hands a plugin.
// It reports an error only after cancel is called.
type nilDoneCtx struct {
	canceled atomic.Bool
}

// Deadline reports no deadline.
func (c *nilDoneCtx) Deadline() (time.Time, bool) { return time.Time{}, false }

// Done is nil, so the table must poll Err instead of selecting.
func (c *nilDoneCtx) Done() <-chan struct{} { return nil }

// Err reports canceled once cancel has been called.
func (c *nilDoneCtx) Err() error {
	if c.canceled.Load() {
		return context.Canceled
	}
	return nil
}

// Value carries nothing.
func (c *nilDoneCtx) Value(any) any { return nil }

// cancel makes this holder look Done to the polling watcher.
func (c *nilDoneCtx) cancel() { c.canceled.Store(true) }

// TestTable_HolderWithoutDoneChannelIsPolled checks the polling branch for a holder whose
// Done() is nil: the incarnation lives until that context reports an error.
func TestTable_HolderWithoutDoneChannelIsPolled(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(time.Millisecond)
	var ended atomic.Bool
	holder := &nilDoneCtx{}
	if _, err := tab.Open(holder, "a", slog.New(h), TableGrace, func() (any, error) {
		return ending(1, &ended), nil
	}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	if ended.Load() {
		t.Fatal("a holder that reports no error must keep the incarnation")
	}

	holder.cancel()
	waitUntil(t, ended.Load)
	waitKeyMsg(t, h, MsgDispose, "a")
}

// TestTable_ValueWithoutCloseStillDisposes checks that a stored value with no Close method
// still ends its incarnation and logs dispose.
func TestTable_ValueWithoutCloseStillDisposes(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	stored, err := tab.Open(ctx, "a", slog.New(h), TableGrace, func() (any, error) { return "no closer here", nil })
	if err != nil {
		cancel()
		t.Fatalf("Open: %v", err)
	}
	if stored != "no closer here" {
		cancel()
		t.Fatalf("value: %v", stored)
	}

	cancel()
	waitKeyMsg(t, h, MsgDispose, "a")
}

// TestDefault_ResetWithAppliesGrace checks that ResetWith installs the grace it was given and Reset restores the product default.
func TestDefault_ResetWithAppliesGrace(t *testing.T) {
	t.Cleanup(Reset)
	ResetWith(37 * time.Millisecond)
	if got := Default().grace; got != 37*time.Millisecond {
		t.Fatalf("ResetWith grace: %v", got)
	}
	Reset()
	if got := Default().grace; got != DefaultGrace {
		t.Fatalf("Reset grace: %v want %v", got, DefaultGrace)
	}
}

// TestDefault_ConcurrentFirstUseReturnsOneTable checks the process table is created once under a race.
func TestDefault_ConcurrentFirstUseReturnsOneTable(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	const callers = 16
	tables := make([]*Table, callers)
	var wg sync.WaitGroup
	wg.Add(callers)
	for i := 0; i < callers; i++ {
		go func(i int) {
			defer wg.Done()
			tables[i] = Default()
		}(i)
	}
	wg.Wait()
	for i := 1; i < callers; i++ {
		if tables[i] != tables[0] {
			t.Fatalf("Default returned a second table at caller %d", i)
		}
	}
}

// TestTable_RepeatedReclaimCyclesKeepOneIncarnation checks that orphan then reclaim can repeat
// without ever running create again or ending the incarnation early.
func TestTable_RepeatedReclaimCyclesKeepOneIncarnation(t *testing.T) {
	const cycles = 5
	h := &recHandler{}
	tab := NewTable(graceNoRace)
	var ended atomic.Bool
	var creates atomic.Int32

	create := func() (any, error) {
		creates.Add(1)
		return ending(1, &ended), nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	first, err := tab.Open(ctx, "a", slog.New(h), TableGrace, create)
	if err != nil {
		cancel()
		t.Fatalf("Open: %v", err)
	}

	holderCancel := cancel
	for cycle := 0; cycle < cycles; cycle++ {
		holderCancel()
		waitUntil(t, func() bool { return countKeyMsg(h.events(), MsgOrphan, "a") == cycle+1 })

		next, nextCancel := context.WithCancel(context.Background())
		stored, err := tab.Open(next, "a", slog.New(h), TableGrace, create)
		if err != nil {
			nextCancel()
			t.Fatalf("cycle %d Open: %v", cycle, err)
		}
		if stored != first {
			nextCancel()
			t.Fatalf("cycle %d lost the stored value", cycle)
		}
		holderCancel = nextCancel
	}

	if ended.Load() {
		holderCancel()
		t.Fatal("repeated reclaim must not end the incarnation")
	}

	// Reset ends the incarnation synchronously, so the counts below need no timing.
	holderCancel()
	tab.Reset()
	if !ended.Load() {
		t.Fatal("Reset must close the stored value")
	}

	events := h.events()
	if creates.Load() != 1 {
		t.Fatalf("create ran %d times", creates.Load())
	}
	if countKeyMsg(events, MsgPut, "a") != 1 || countKeyMsg(events, MsgDispose, "a") != 1 {
		t.Fatalf("one put and one dispose expected: %+v", events)
	}
	if got := countKeyMsg(events, MsgReclaim, "a"); got != cycles {
		t.Fatalf("reclaim count %d want %d", got, cycles)
	}
}

// TestTable_HolderIdsAreReleased checks that holders whose contexts are Done leave no entry behind.
func TestTable_HolderIdsAreReleased(t *testing.T) {
	const holders = 20
	h := &recHandler{}
	tab := NewTable(graceNoRace)
	keepAlive, keepAliveCancel := context.WithCancel(context.Background())
	defer keepAliveCancel()
	if _, err := tab.Open(keepAlive, "a", slog.New(h), TableGrace, func() (any, error) { return &box{n: 1}, nil }); err != nil {
		t.Fatalf("Open: %v", err)
	}

	for i := 0; i < holders; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		if _, err := tab.Open(ctx, "a", slog.New(h), TableGrace, func() (any, error) { return &box{n: 2}, nil }); err != nil {
			cancel()
			t.Fatalf("Open %d: %v", i, err)
		}
		cancel()
	}

	waitUntil(t, func() bool {
		tab.mu.Lock()
		defer tab.mu.Unlock()
		e := tab.items["a"]
		return e != nil && len(e.holders) == 1
	})
}

// TestTable_OrphanAndDisposeUseTheLastBindingLogger checks the logger ownership rule for a key
// that was opened twice with different loggers.
func TestTable_OrphanAndDisposeUseTheLastBindingLogger(t *testing.T) {
	firstLog := &recHandler{}
	secondLog := &recHandler{}
	tab := NewTable(time.Millisecond)
	ctx1, cancel1 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx1, "a", slog.New(firstLog), TableGrace, func() (any, error) { return &box{n: 1}, nil }); err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx2, "a", slog.New(secondLog), TableGrace, func() (any, error) { return &box{n: 2}, nil }); err != nil {
		t.Fatalf("Open 2: %v", err)
	}

	cancel1()
	cancel2()
	waitKeyMsg(t, secondLog, MsgDispose, "a")

	if countKeyMsg(secondLog.events(), MsgOrphan, "a") != 1 {
		t.Fatalf("last logger must record orphan: %+v", secondLog.events())
	}
	if countKeyMsg(firstLog.events(), MsgOrphan, "a") != 0 || countKeyMsg(firstLog.events(), MsgDispose, "a") != 0 {
		t.Fatalf("first logger must not record orphan or dispose: %+v", firstLog.events())
	}
	if countKeyMsg(firstLog.events(), MsgPut, "a") != 1 || countKeyMsg(firstLog.events(), MsgBind, "a") != 1 {
		t.Fatalf("first logger owns its own put and bind: %+v", firstLog.events())
	}
}

// TestTable_ManyKeysDisposeIndependently checks that ending one key leaves every other key stored.
func TestTable_ManyKeysDisposeIndependently(t *testing.T) {
	const keys = 20
	h := &recHandler{}
	tab := NewTable(time.Millisecond)
	keep, keepCancel := context.WithCancel(context.Background())
	defer keepCancel()

	for i := 1; i < keys; i++ {
		key := "k" + strconv.Itoa(i)
		if _, err := tab.Open(keep, key, slog.New(h), TableGrace, func() (any, error) { return &box{n: i}, nil }); err != nil {
			t.Fatalf("Open %s: %v", key, err)
		}
	}
	goneCtx, goneCancel := context.WithCancel(context.Background())
	if _, err := tab.Open(goneCtx, "k0", slog.New(h), TableGrace, func() (any, error) { return &box{n: 0}, nil }); err != nil {
		goneCancel()
		t.Fatalf("Open k0: %v", err)
	}

	goneCancel()
	waitKeyMsg(t, h, MsgDispose, "k0")

	if got := countMsg(h.events(), MsgDispose); got != 1 {
		t.Fatalf("only k0 may dispose, saw %d: %+v", got, h.events())
	}
	tab.mu.Lock()
	defer tab.mu.Unlock()
	if len(tab.items) != keys-1 {
		t.Fatalf("stored keys %d want %d", len(tab.items), keys-1)
	}
}

// TestTable_ResetRacingOpenClosesEveryValue checks that Reset against in-flight Opens never
// leaves a created value stored on a dead table or unclosed.
func TestTable_ResetRacingOpenClosesEveryValue(t *testing.T) {
	const rounds = 50
	h := &recHandler{}
	tab := NewTable(time.Millisecond)
	var mu sync.Mutex
	var values []*counterClose

	for i := 0; i < rounds; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, err := tab.Open(ctx, "a", slog.New(h), TableGrace, func() (any, error) {
				v := &counterClose{}
				mu.Lock()
				values = append(values, v)
				mu.Unlock()
				return v, nil
			})
			if err != nil {
				t.Errorf("round %d Open: %v", i, err)
			}
		}()
		go func() {
			defer wg.Done()
			tab.Reset()
		}()
		wg.Wait()
		cancel()
	}

	tab.Reset()

	// Per value, not in aggregate: a total would also be satisfied by one value closed twice
	// and another never closed, which is exactly the pair of bugs this test is looking for.
	mu.Lock()
	created := append([]*counterClose(nil), values...)
	mu.Unlock()
	waitUntil(t, func() bool {
		for _, v := range created {
			if v.closes.Load() == 0 {
				return false
			}
		}
		return true
	})
	for i, v := range created {
		if got := v.closes.Load(); got != 1 {
			t.Fatalf("value %d closed %d times, want exactly 1", i, got)
		}
	}
	tab.mu.Lock()
	defer tab.mu.Unlock()
	if len(tab.items) != 0 {
		t.Fatalf("a reset table must store nothing, has %d", len(tab.items))
	}
}

// counterClose counts its own Close so a test can prove every created value was closed exactly once.
type counterClose struct {
	closes atomic.Int32
}

// Close records that this value was stopped.
func (c *counterClose) Close() {
	c.closes.Add(1)
}

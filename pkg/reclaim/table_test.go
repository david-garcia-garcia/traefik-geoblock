package reclaim

import (
	"context"
	"go/parser"
	"go/token"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// endedOnLife sets done when life is canceled.
func endedOnLife(life context.Context, done *atomic.Bool) {
	go func() {
		<-life.Done()
		done.Store(true)
	}()
}

// box is a disposable stand-in stored on the table in tests.
type box struct {
	n int
}

// recHandler records slog lines so tests can assert reclaim msg + key order.
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

// hasSubseq reports whether want appears in order inside got (other lines may sit between).
func hasSubseq(got [][2]string, want [][2]string) bool {
	i := 0
	for _, g := range got {
		if i < len(want) && g == want[i] {
			i++
		}
	}
	return i == len(want)
}

// TestTable_OpenCancelDispose checks that cancel plus grace cancels the lifetime once and logs the full sequence.
func TestTable_OpenCancelDispose(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(20*time.Millisecond, slog.New(h))
	var ended atomic.Bool
	ctx, cancel := context.WithCancel(context.Background())

	if _, err := tab.Open(ctx, "a", func(life context.Context) (any, error) {
		endedOnLife(life, &ended)
		return &box{1}, nil
	}); err != nil {
		t.Fatalf("Open: %v", err)
	}

	// Last holder gone; wait past grace.
	cancel()
	time.Sleep(80 * time.Millisecond)
	if !ended.Load() {
		t.Fatal("expected lifetime cancel after grace")
	}
	if !hasSubseq(h.events(), [][2]string{
		{MsgPut, "a"},
		{MsgBind, "a"},
		{MsgOrphan, "a"},
		{MsgDispose, "a"},
	}) {
		t.Fatalf("events: %+v", h.events())
	}
}

// TestTable_OpenDuringGraceReclaims checks that Open before grace keeps the incarnation.
func TestTable_OpenDuringGraceReclaims(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(80*time.Millisecond, slog.New(h))
	var ended atomic.Bool
	ctx1, cancel1 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx1, "a", func(life context.Context) (any, error) {
		endedOnLife(life, &ended)
		return &box{1}, nil
	}); err != nil {
		t.Fatalf("Open 1: %v", err)
	}

	// Orphan, then rebind while grace is still running.
	cancel1()
	time.Sleep(15 * time.Millisecond)
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	if _, err := tab.Open(ctx2, "a", func(life context.Context) (any, error) {
		return &box{2}, nil
	}); err != nil {
		t.Fatalf("Open 2: %v", err)
	}

	time.Sleep(120 * time.Millisecond)
	if ended.Load() {
		t.Fatal("reclaim must not cancel the lifetime")
	}
	if !hasSubseq(h.events(), [][2]string{
		{MsgPut, "a"},
		{MsgOrphan, "a"},
		{MsgReclaim, "a"},
		{MsgBind, "a"},
	}) {
		t.Fatalf("events: %+v", h.events())
	}
}

// TestTable_SecondCreateDisposeIgnored checks that a later Open does not run create or replace the lifetime.
func TestTable_SecondCreateDisposeIgnored(t *testing.T) {
	tab := NewTable(20*time.Millisecond, slog.New(&recHandler{}))
	var created, ended atomic.Int32
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx1, "a", func(life context.Context) (any, error) {
		created.Add(1)
		go func() {
			<-life.Done()
			ended.Add(1)
		}()
		return &box{1}, nil
	}); err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	if _, err := tab.Open(ctx2, "a", func(life context.Context) (any, error) {
		created.Add(1)
		go func() {
			<-life.Done()
			ended.Add(1)
		}()
		return &box{2}, nil
	}); err != nil {
		t.Fatalf("Open 2: %v", err)
	}

	cancel1()
	cancel2()
	time.Sleep(80 * time.Millisecond)
	if created.Load() != 1 {
		t.Fatalf("second Open must not run create, created=%d", created.Load())
	}
	if ended.Load() != 1 {
		t.Fatalf("expected one lifetime cancel, ended=%d", ended.Load())
	}
}

// TestTable_TwoOpensOneDispose checks that one live holder blocks lifetime cancel until the last ctx is Done.
func TestTable_TwoOpensOneDispose(t *testing.T) {
	tab := NewTable(20*time.Millisecond, slog.New(&recHandler{}))
	var ended atomic.Bool
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx1, "a", func(life context.Context) (any, error) {
		endedOnLife(life, &ended)
		return &box{1}, nil
	}); err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	if _, err := tab.Open(ctx2, "a", func(context.Context) (any, error) {
		return &box{2}, nil
	}); err != nil {
		t.Fatalf("Open 2: %v", err)
	}

	cancel1()
	time.Sleep(80 * time.Millisecond)
	if ended.Load() {
		t.Fatal("one live open must keep the incarnation")
	}

	cancel2()
	time.Sleep(80 * time.Millisecond)
	if !ended.Load() {
		t.Fatal("expected lifetime cancel after last holder")
	}
}

// TestTable_DefaultGrace checks that a non-positive grace becomes DefaultGrace.
func TestTable_DefaultGrace(t *testing.T) {
	tab := NewTable(0, slog.Default())
	if tab.grace != DefaultGrace {
		t.Fatalf("grace: %v", tab.grace)
	}
}

// TestTable_StdlibImports checks that table.go imports only the standard library.
func TestTable_StdlibImports(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "table.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if strings.Contains(path, ".") {
			t.Fatalf("non-stdlib import %s", path)
		}
	}
}

// TestTable_HashChangeProof checks that canceling key A’s lifetime does not cancel a live key B.
func TestTable_HashChangeProof(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(20*time.Millisecond, slog.New(h))
	var ended []string
	var mu sync.Mutex

	ctxA, cancelA := context.WithCancel(context.Background())
	if _, err := tab.Open(ctxA, "A", func(life context.Context) (any, error) {
		go func() {
			<-life.Done()
			mu.Lock()
			ended = append(ended, "A")
			mu.Unlock()
		}()
		return &box{1}, nil
	}); err != nil {
		t.Fatalf("Open A: %v", err)
	}

	// A orphans; B is a new incarnation on the same table.
	cancelA()
	ctxB, cancelB := context.WithCancel(context.Background())
	defer cancelB()
	if _, err := tab.Open(ctxB, "B", func(life context.Context) (any, error) {
		go func() {
			<-life.Done()
			mu.Lock()
			ended = append(ended, "B")
			mu.Unlock()
		}()
		return &box{2}, nil
	}); err != nil {
		t.Fatalf("Open B: %v", err)
	}

	time.Sleep(80 * time.Millisecond)
	mu.Lock()
	got := append([]string(nil), ended...)
	mu.Unlock()
	if len(got) != 1 || got[0] != "A" {
		t.Fatalf("ended: %v", got)
	}
	if !hasSubseq(h.events(), [][2]string{
		{MsgPut, "A"},
		{MsgOrphan, "A"},
		{MsgDispose, "A"},
	}) {
		t.Fatalf("events: %+v", h.events())
	}

	foundB := false
	for _, e := range h.events() {
		if e[0] == MsgPut && e[1] == "B" {
			foundB = true
		}
		if e[0] == MsgDispose && e[1] == "B" {
			t.Fatal("B must not cancel")
		}
	}
	if !foundB {
		t.Fatalf("missing put B: %+v", h.events())
	}
}

// TestDefault_OpenSharesIncarnation checks that package Open and Default().Open are the same table.
func TestDefault_OpenSharesIncarnation(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	ctx := context.Background()
	a, err := Open(ctx, "k", func(context.Context) (any, error) { return &box{7}, nil })
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	b, err := Default().Open(ctx, "k", func(context.Context) (any, error) { return &box{8}, nil })
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

package reclaim

import (
	"context"
	"go/parser"
	"go/token"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

type box struct {
	n int
}

type recHandler struct {
	mu   sync.Mutex
	recs []slog.Record
}

func (h *recHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	h.recs = append(h.recs, r.Clone())
	h.mu.Unlock()
	return nil
}

func (h *recHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recHandler) WithGroup(string) slog.Handler      { return h }

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

func hasSubseq(got [][2]string, want [][2]string) bool {
	i := 0
	for _, g := range got {
		if i < len(want) && g == want[i] {
			i++
		}
	}
	return i == len(want)
}

func TestTable_OpenCancelDispose(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(20*time.Millisecond, slog.New(h))
	disposed := false
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx, "a", func() (any, error) { return &box{1}, nil }, func(any) { disposed = true }); err != nil {
		t.Fatalf("Open: %v", err)
	}
	cancel()
	time.Sleep(80 * time.Millisecond)
	if !disposed {
		t.Fatal("expected dispose after grace")
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

func TestTable_OpenDuringGraceReclaims(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(80*time.Millisecond, slog.New(h))
	disposed := false
	ctx1, cancel1 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx1, "a", func() (any, error) { return &box{1}, nil }, func(any) { disposed = true }); err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	cancel1()
	time.Sleep(15 * time.Millisecond)
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	if _, err := tab.Open(ctx2, "a", func() (any, error) { return &box{2}, nil }, func(any) { disposed = true }); err != nil {
		t.Fatalf("Open 2: %v", err)
	}
	time.Sleep(120 * time.Millisecond)
	if disposed {
		t.Fatal("reclaim must not dispose")
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

func TestTable_SecondCreateDisposeIgnored(t *testing.T) {
	tab := NewTable(20*time.Millisecond, slog.New(&recHandler{}))
	first, second := false, false
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx1, "a", func() (any, error) { return &box{1}, nil }, func(any) { first = true }); err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	if _, err := tab.Open(ctx2, "a", func() (any, error) { return &box{2}, nil }, func(any) { second = true }); err != nil {
		t.Fatalf("Open 2: %v", err)
	}
	cancel1()
	cancel2()
	time.Sleep(80 * time.Millisecond)
	if !first {
		t.Fatal("expected first dispose")
	}
	if second {
		t.Fatal("second Open must not replace dispose")
	}
}

func TestTable_TwoOpensOneDispose(t *testing.T) {
	tab := NewTable(20*time.Millisecond, slog.New(&recHandler{}))
	disposed := false
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx1, "a", func() (any, error) { return &box{1}, nil }, func(any) { disposed = true }); err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	if _, err := tab.Open(ctx2, "a", func() (any, error) { return &box{2}, nil }, func(any) { disposed = true }); err != nil {
		t.Fatalf("Open 2: %v", err)
	}
	cancel1()
	time.Sleep(80 * time.Millisecond)
	if disposed {
		t.Fatal("one live open must keep the incarnation")
	}
	cancel2()
	time.Sleep(80 * time.Millisecond)
	if !disposed {
		t.Fatal("expected dispose after last holder")
	}
}

func TestTable_DefaultGrace(t *testing.T) {
	tab := NewTable(0, slog.Default())
	if tab.grace != DefaultGrace {
		t.Fatalf("grace: %v", tab.grace)
	}
}

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

func TestTable_HashChangeProof(t *testing.T) {
	h := &recHandler{}
	tab := NewTable(20*time.Millisecond, slog.New(h))
	var disposed []string
	var mu sync.Mutex
	ctxA, cancelA := context.WithCancel(context.Background())
	if _, err := tab.Open(ctxA, "A", func() (any, error) { return &box{1}, nil }, func(any) {
		mu.Lock()
		disposed = append(disposed, "A")
		mu.Unlock()
	}); err != nil {
		t.Fatalf("Open A: %v", err)
	}
	cancelA()
	ctxB, cancelB := context.WithCancel(context.Background())
	defer cancelB()
	if _, err := tab.Open(ctxB, "B", func() (any, error) { return &box{2}, nil }, func(any) {
		mu.Lock()
		disposed = append(disposed, "B")
		mu.Unlock()
	}); err != nil {
		t.Fatalf("Open B: %v", err)
	}
	time.Sleep(80 * time.Millisecond)
	mu.Lock()
	got := append([]string(nil), disposed...)
	mu.Unlock()
	if len(got) != 1 || got[0] != "A" {
		t.Fatalf("disposed: %v", got)
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
			t.Fatal("B must not dispose")
		}
	}
	if !foundB {
		t.Fatalf("missing put B: %+v", h.events())
	}
}

func TestDefault_OpenSharesIncarnation(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	ctx := context.Background()
	a, err := Open(ctx, "k", func() (any, error) { return &box{7}, nil }, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	b, err := Default().Open(ctx, "k", func() (any, error) { return &box{8}, nil }, nil)
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

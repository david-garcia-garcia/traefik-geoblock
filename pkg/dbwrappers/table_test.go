package dbwrappers

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

func TestTable_OpenCancelDispose(t *testing.T) {
	h := &recHandler{}
	tab := NewTable[*box](20*time.Millisecond, slog.New(h))
	disposed := false
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx, "a", func() (*box, error) { return &box{1}, nil }, func(*box) { disposed = true }); err != nil {
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
	tab := NewTable[*box](80*time.Millisecond, slog.New(h))
	disposed := false
	ctx1, cancel1 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx1, "a", func() (*box, error) { return &box{1}, nil }, func(*box) { disposed = true }); err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	cancel1()
	time.Sleep(15 * time.Millisecond)
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	if _, err := tab.Open(ctx2, "a", func() (*box, error) { return &box{2}, nil }, func(*box) { disposed = true }); err != nil {
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
	tab := NewTable[*box](20*time.Millisecond, slog.New(&recHandler{}))
	first, second := false, false
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx1, "a", func() (*box, error) { return &box{1}, nil }, func(*box) { first = true }); err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	if _, err := tab.Open(ctx2, "a", func() (*box, error) { return &box{2}, nil }, func(*box) { second = true }); err != nil {
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
	tab := NewTable[*box](20*time.Millisecond, slog.New(&recHandler{}))
	disposed := false
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	if _, err := tab.Open(ctx1, "a", func() (*box, error) { return &box{1}, nil }, func(*box) { disposed = true }); err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	if _, err := tab.Open(ctx2, "a", func() (*box, error) { return &box{2}, nil }, func(*box) { disposed = true }); err != nil {
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
	tab := NewTable[*box](0, slog.Default())
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
	tab := NewTable[*box](20*time.Millisecond, slog.New(h))
	var disposed []string
	var mu sync.Mutex
	ctxA, cancelA := context.WithCancel(context.Background())
	if _, err := tab.Open(ctxA, "A", func() (*box, error) { return &box{1}, nil }, func(*box) {
		mu.Lock()
		disposed = append(disposed, "A")
		mu.Unlock()
	}); err != nil {
		t.Fatalf("Open A: %v", err)
	}
	cancelA()
	ctxB, cancelB := context.WithCancel(context.Background())
	defer cancelB()
	if _, err := tab.Open(ctxB, "B", func() (*box, error) { return &box{2}, nil }, func(*box) {
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

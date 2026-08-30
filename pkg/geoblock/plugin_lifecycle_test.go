package geoblock

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbwrappers"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/reclaim"
)

type lifecycleLog struct {
	mu   sync.Mutex
	recs []slog.Record
}

func (h *lifecycleLog) Enabled(context.Context, slog.Level) bool { return true }

func (h *lifecycleLog) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	h.recs = append(h.recs, r.Clone())
	h.mu.Unlock()
	return nil
}

func (h *lifecycleLog) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *lifecycleLog) WithGroup(string) slog.Handler      { return h }

func (h *lifecycleLog) events() [][2]string {
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

func countMsg(ev [][2]string, msg string) int {
	n := 0
	for _, e := range ev {
		if e[0] == msg {
			n++
		}
	}
	return n
}

func putKeys(ev [][2]string) []string {
	var keys []string
	seen := map[string]bool{}
	for _, e := range ev {
		if e[0] == reclaim.MsgPut && !seen[e[1]] {
			seen[e[1]] = true
			keys = append(keys, e[1])
		}
	}
	return keys
}

func hasEvent(ev [][2]string, msg, key string) bool {
	for _, e := range ev {
		if e[0] == msg && e[1] == key {
			return true
		}
	}
	return false
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

func shortLeases(t *testing.T) *lifecycleLog {
	t.Helper()
	dbwrappers.Reset()
	t.Cleanup(dbwrappers.Reset)
	h := &lifecycleLog{}
	dbwrappers.ResetWith(25*time.Millisecond, slog.New(h))
	return h
}

func tickerURL(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/db.bin"
}

func lifecycleBIN(path, url, dir string) *Config {
	return &Config{
		Mode:                  ModeEnrichAndBlock,
		DatabaseSources:       map[string]DatabaseSource{"seed": {Path: path, URL: url, DatabaseType: dbsource.TypeBIN}},
		Ip2locationSourceGeo:  "seed",
		DatabaseAutoUpdateDir: dir,
		AllowedCountries:      []string{"US"},
		DisallowedStatusCode:  http.StatusForbidden,
		IPHeaders:             []string{"x-real-ip"},
		IPHeaderStrategy:      IPHeaderStrategyCheckAll,
		BanIfError:            true,
	}
}

func lifecycleIPinfo(path, url, dir string) *Config {
	return &Config{
		Mode:                  ModeEnrichAndBlock,
		DatabaseProvider:      DatabaseProviderIPinfo,
		DatabaseSources:       map[string]DatabaseSource{"seed": {Path: path, URL: url, DatabaseType: dbsource.TypeMMDB}},
		IpinfoSource:          "seed",
		DatabaseAutoUpdateDir: dir,
		AllowedCountries:      []string{"US"},
		DisallowedStatusCode:  http.StatusForbidden,
		IPHeaders:             []string{"x-real-ip"},
		IPHeaderStrategy:      IPHeaderStrategyCheckAll,
		BanIfError:            true,
	}
}

func mustPlugin(t *testing.T, ctx context.Context, cfg *Config) *Route {
	t.Helper()
	h, err := newRoute(ctx, &noopHandler{}, cfg, pluginName)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	p, ok := h.(*Route)
	if !ok {
		t.Fatalf("handler type %T", h)
	}
	return p
}

func requireLookupUS(t *testing.T, p *Route) {
	t.Helper()
	rec, err := p.Lookup("8.8.8.8")
	if err != nil || rec.Country != "US" {
		t.Fatalf("lookup: %+v %v", rec, err)
	}
}

func requireLookupClosed(t *testing.T, p *Route) {
	t.Helper()
	if _, err := p.Lookup("8.8.8.8"); err == nil {
		t.Fatal("expected lookup to fail after dispose")
	}
}

func TestNew_ContextBindsWrapper(t *testing.T) {
	h := shortLeases(t)
	ctx, cancel := context.WithCancel(context.Background())
	p := mustPlugin(t, ctx, lifecycleBIN(dbFilePath, tickerURL(t), t.TempDir()))
	requireLookupUS(t, p)
	ev := h.events()
	keys := putKeys(ev)
	if len(keys) == 0 {
		t.Fatalf("expected wrapper stored, got %+v", ev)
	}
	if !hasSubseq(ev, [][2]string{{reclaim.MsgPut, keys[0]}, {reclaim.MsgBind, keys[0]}}) {
		t.Fatalf("expected put+bind, got %+v", ev)
	}
	cancel()
	time.Sleep(80 * time.Millisecond)
	requireLookupClosed(t, p)
	if countMsg(h.events(), reclaim.MsgDispose) == 0 {
		t.Fatalf("expected dispose after grace, got %+v", h.events())
	}
}

func TestNew_SameHashReclaimAfterGenerationCancel(t *testing.T) {
	h := shortLeases(t)
	dir := t.TempDir()
	url := tickerURL(t)
	cfg := lifecycleBIN(dbFilePath, url, dir)
	gen, cancel := context.WithCancel(context.Background())
	a := mustPlugin(t, gen, cfg)
	b := mustPlugin(t, gen, cfg)
	requireLookupUS(t, a)
	requireLookupUS(t, b)
	puts := countMsg(h.events(), reclaim.MsgPut)
	cancel()
	time.Sleep(15 * time.Millisecond)
	next, cancelNext := context.WithCancel(context.Background())
	defer cancelNext()
	c := mustPlugin(t, next, cfg)
	requireLookupUS(t, c)
	time.Sleep(80 * time.Millisecond)
	requireLookupUS(t, c)
	ev := h.events()
	if countMsg(ev, reclaim.MsgPut) != puts {
		t.Fatalf("second keep-current must not create, puts before=%d after=%d ev=%+v", puts, countMsg(ev, reclaim.MsgPut), ev)
	}
	if countMsg(ev, reclaim.MsgReclaim) == 0 {
		t.Fatalf("expected reclaim, got %+v", ev)
	}
	if countMsg(ev, reclaim.MsgDispose) != 0 {
		t.Fatalf("reclaim must not dispose, got %+v", ev)
	}
}

func TestNew_UnreclaimedHashDisposesAfterGrace(t *testing.T) {
	h := shortLeases(t)
	dir := t.TempDir()
	url := tickerURL(t)
	cfg := lifecycleBIN(dbFilePath, url, dir)
	ctx, cancel := context.WithCancel(context.Background())
	p := mustPlugin(t, ctx, cfg)
	requireLookupUS(t, p)
	cancel()
	time.Sleep(80 * time.Millisecond)
	requireLookupClosed(t, p)
	if countMsg(h.events(), reclaim.MsgDispose) == 0 {
		t.Fatalf("expected dispose, got %+v", h.events())
	}
	puts := countMsg(h.events(), reclaim.MsgPut)
	next, cancelNext := context.WithCancel(context.Background())
	defer cancelNext()
	again := mustPlugin(t, next, cfg)
	requireLookupUS(t, again)
	if countMsg(h.events(), reclaim.MsgPut) <= puts {
		t.Fatalf("later open must create a new wrapper, ev=%+v", h.events())
	}
}

func TestNew_ProviderCloseDoesNotDispose(t *testing.T) {
	shortLeases(t)
	cfg := lifecycleBIN(dbFilePath, "", "")
	ctxA, cancelA := context.WithCancel(context.Background())
	defer cancelA()
	ctxB, cancelB := context.WithCancel(context.Background())
	defer cancelB()
	a := mustPlugin(t, ctxA, cfg)
	b := mustPlugin(t, ctxB, cfg)
	if err := a.db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	requireLookupUS(t, b)
}

func TestNew_HashChangeDisposesOld(t *testing.T) {
	h := shortLeases(t)
	dir := t.TempDir()
	url := tickerURL(t)
	copyPath := filepath.Join(t.TempDir(), "copy.BIN")
	src, err := os.ReadFile(dbFilePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copyPath, src, 0600); err != nil {
		t.Fatal(err)
	}
	ctx1, cancel1 := context.WithCancel(context.Background())
	h1 := mustPlugin(t, ctx1, lifecycleBIN(dbFilePath, url, dir))
	requireLookupUS(t, h1)
	first := putKeys(h.events())
	cancel1()
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	h2 := mustPlugin(t, ctx2, lifecycleBIN(copyPath, url, dir))
	time.Sleep(80 * time.Millisecond)
	requireLookupClosed(t, h1)
	requireLookupUS(t, h2)
	ev := h.events()
	var disposedOld bool
	for _, key := range first {
		if hasEvent(ev, reclaim.MsgDispose, key) {
			disposedOld = true
			if !hasSubseq(ev, [][2]string{
				{reclaim.MsgPut, key},
				{reclaim.MsgOrphan, key},
				{reclaim.MsgDispose, key},
			}) {
				t.Fatalf("H1 sequence: %+v key %s", ev, key)
			}
		}
	}
	if !disposedOld {
		t.Fatalf("expected dispose of an H1 key, got %+v first=%v", ev, first)
	}
	after := putKeys(ev)
	if len(after) <= len(first) {
		t.Fatalf("expected H2 create, first=%v after=%v ev=%+v", first, after, ev)
	}
	for _, key := range after {
		seen := false
		for _, k := range first {
			if k == key {
				seen = true
				break
			}
		}
		if !seen && hasEvent(ev, reclaim.MsgDispose, key) {
			t.Fatalf("H2 must not dispose, key %s ev=%+v", key, ev)
		}
	}
}

func TestNew_IPinfoSameHashReclaimAfterGenerationCancel(t *testing.T) {
	h := shortLeases(t)
	cfg := lifecycleIPinfo(ipinfoFilePath, tickerURL(t), t.TempDir())
	gen, cancel := context.WithCancel(context.Background())
	_ = mustPlugin(t, gen, cfg)
	puts := countMsg(h.events(), reclaim.MsgPut)
	if puts != 1 {
		t.Fatalf("expected one MMDB put, got %d ev=%+v", puts, h.events())
	}
	cancel()
	time.Sleep(15 * time.Millisecond)
	next, cancelNext := context.WithCancel(context.Background())
	defer cancelNext()
	p := mustPlugin(t, next, cfg)
	requireLookupUS(t, p)
	time.Sleep(80 * time.Millisecond)
	requireLookupUS(t, p)
	ev := h.events()
	if countMsg(ev, reclaim.MsgPut) != 1 {
		t.Fatalf("second keep-current must not create, ev=%+v", ev)
	}
	if countMsg(ev, reclaim.MsgReclaim) == 0 || countMsg(ev, reclaim.MsgDispose) != 0 {
		t.Fatalf("expected reclaim and no dispose, ev=%+v", ev)
	}
}

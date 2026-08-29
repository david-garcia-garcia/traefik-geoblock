package traefik_geoblock

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
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/geoblock"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/reclaim"
)

type instanceLog struct {
	mu   sync.Mutex
	recs []slog.Record
}

func (h *instanceLog) Enabled(context.Context, slog.Level) bool { return true }

func (h *instanceLog) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	h.recs = append(h.recs, r.Clone())
	h.mu.Unlock()
	return nil
}

func (h *instanceLog) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *instanceLog) WithGroup(string) slog.Handler      { return h }

func (h *instanceLog) events() [][2]string {
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

func countPutsPrefix(ev [][2]string, prefix string) int {
	n := 0
	for _, e := range ev {
		if e[0] == reclaim.MsgPut && len(e[1]) >= len(prefix) && e[1][:len(prefix)] == prefix {
			n++
		}
	}
	return n
}

func countInstanceMsg(ev [][2]string, msg string) int {
	n := 0
	for _, e := range ev {
		if e[0] == msg {
			n++
		}
	}
	return n
}

func shortInstanceLeases(t *testing.T) *instanceLog {
	t.Helper()
	dbwrappers.Reset()
	t.Cleanup(dbwrappers.Reset)
	h := &instanceLog{}
	dbwrappers.ResetWith(25*time.Millisecond, slog.New(h))
	return h
}

func instanceModuleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

func instanceBIN(path, url, dir string) *Config {
	return &Config{
		Enabled: true,
		DatabaseSources: map[string]geoblock.DatabaseSource{
			"seed": {Path: path, URL: url, DatabaseType: dbsource.TypeBIN},
		},
		Ip2locationSourceGeo:  "seed",
		DatabaseAutoUpdateDir: dir,
		AllowedCountries:      []string{"US"},
		DisallowedStatusCode:  http.StatusForbidden,
		IPHeaders:             []string{"x-real-ip"},
		IPHeaderStrategy:      geoblock.IPHeaderStrategyCheckAll,
		BanIfError:            true,
	}
}

func mustRootPlugin(t *testing.T, ctx context.Context, cfg *Config, name string) *geoblock.Plugin {
	t.Helper()
	h, err := New(ctx, noopNext{}, cfg, name)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	p, ok := h.(*geoblock.Plugin)
	if !ok {
		t.Fatalf("handler type %T", h)
	}
	return p
}

type noopNext struct{}

func (noopNext) ServeHTTP(http.ResponseWriter, *http.Request) {}

type countHandler struct{ n *int }

func (c countHandler) ServeHTTP(http.ResponseWriter, *http.Request) { *c.n++ }

func usPassRequest() *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "8.8.8.8")
	return req
}

func requireLookupUS(t *testing.T, p *geoblock.Plugin) {
	t.Helper()
	rec, err := p.Lookup("8.8.8.8")
	if err != nil || rec.Country != "US" {
		t.Fatalf("lookup: %+v %v", rec, err)
	}
}

func TestNew_SameNameConfigSharesIncarnation(t *testing.T) {
	shortInstanceLeases(t)
	cfg := instanceBIN(filepath.Join(instanceModuleRoot(), "seeds", "IP2LOCATION-LITE-DB1.IPV6.BIN"), "", "")
	ctx := context.Background()
	var n1, n2 int
	a, err := New(ctx, countHandler{&n1}, cfg, "geoblock")
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(ctx, countHandler{&n2}, cfg, "geoblock")
	if err != nil {
		t.Fatal(err)
	}
	pa, pb := a.(*geoblock.Plugin), b.(*geoblock.Plugin)
	if !pa.SameCore(pb) {
		t.Fatal("expected shared incarnation")
	}
	if pa.Next() == pb.Next() {
		t.Fatal("next must be per New")
	}
	a.ServeHTTP(httptest.NewRecorder(), usPassRequest())
	b.ServeHTTP(httptest.NewRecorder(), usPassRequest())
	if n1 != 1 || n2 != 1 {
		t.Fatalf("next hits n1=%d n2=%d", n1, n2)
	}
}

func TestNew_NameMissSeparateIncarnation(t *testing.T) {
	shortInstanceLeases(t)
	cfg := instanceBIN(filepath.Join(instanceModuleRoot(), "seeds", "IP2LOCATION-LITE-DB1.IPV6.BIN"), "", "")
	ctx := context.Background()
	a := mustRootPlugin(t, ctx, cfg, "geoblock")
	b := mustRootPlugin(t, ctx, cfg, "geoblock-b")
	if a.SameCore(b) {
		t.Fatal("different names must not share incarnation")
	}
}

func TestNew_ConfigMissSeparateIncarnation(t *testing.T) {
	shortInstanceLeases(t)
	ctx := context.Background()
	seed := filepath.Join(instanceModuleRoot(), "seeds", "IP2LOCATION-LITE-DB1.IPV6.BIN")
	a := mustRootPlugin(t, ctx, instanceBIN(seed, "", ""), "geoblock")
	cfgB := instanceBIN(seed, "", "")
	cfgB.AllowedCountries = []string{"DE"}
	b := mustRootPlugin(t, ctx, cfgB, "geoblock")
	if a.SameCore(b) {
		t.Fatal("different config must not share incarnation")
	}
}

func TestNew_PluginReclaimAfterGenerationCancel(t *testing.T) {
	h := shortInstanceLeases(t)
	cfg := instanceBIN(filepath.Join(instanceModuleRoot(), "seeds", "IP2LOCATION-LITE-DB1.IPV6.BIN"), "", "")
	gen, cancel := context.WithCancel(context.Background())
	_ = mustRootPlugin(t, gen, cfg, "geoblock")
	puts := countPutsPrefix(h.events(), keyPrefixPlugin)
	if puts != 1 {
		t.Fatalf("expected one plugin put, got %d ev=%+v", puts, h.events())
	}
	cancel()
	time.Sleep(15 * time.Millisecond)
	next, cancelNext := context.WithCancel(context.Background())
	defer cancelNext()
	p := mustRootPlugin(t, next, cfg, "geoblock")
	requireLookupUS(t, p)
	ev := h.events()
	if countPutsPrefix(ev, keyPrefixPlugin) != 1 {
		t.Fatalf("reclaim must not create plugin, ev=%+v", ev)
	}
	if countInstanceMsg(ev, reclaim.MsgReclaim) == 0 {
		t.Fatalf("expected reclaim, ev=%+v", ev)
	}
}

func TestNew_PluginDisposeAfterGrace(t *testing.T) {
	h := shortInstanceLeases(t)
	cfg := instanceBIN(filepath.Join(instanceModuleRoot(), "seeds", "IP2LOCATION-LITE-DB1.IPV6.BIN"), "", "")
	ctx, cancel := context.WithCancel(context.Background())
	_ = mustRootPlugin(t, ctx, cfg, "geoblock")
	cancel()
	time.Sleep(80 * time.Millisecond)
	puts := countPutsPrefix(h.events(), keyPrefixPlugin)
	next, cancelNext := context.WithCancel(context.Background())
	defer cancelNext()
	again := mustRootPlugin(t, next, cfg, "geoblock")
	requireLookupUS(t, again)
	if countPutsPrefix(h.events(), keyPrefixPlugin) <= puts {
		t.Fatalf("later New must create a new plugin incarnation, ev=%+v", h.events())
	}
}

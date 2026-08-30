package traefik_geoblock

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbwrappers"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/geoblock"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/reclaim"
)

// instanceLog captures reclaim slog records for instance-reuse tests.
type instanceLog struct {
	mu   sync.Mutex
	recs []slog.Record
}

func (h *instanceLog) Enabled(context.Context, slog.Level) bool { return true } // always capture

// Handle stores a clone of each record.
func (h *instanceLog) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	h.recs = append(h.recs, r.Clone())
	h.mu.Unlock()
	return nil
}

func (h *instanceLog) WithAttrs([]slog.Attr) slog.Handler { return h } // same sink
func (h *instanceLog) WithGroup(string) slog.Handler      { return h } // same sink

// events is each record’s message plus its key attr.
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

// countPluginPuts counts reclaim_put events for plugin: keys.
func countPluginPuts(ev [][2]string) int {
	return countPrefixedMsg(ev, reclaim.MsgPut, keyPrefixPlugin)
}

// countPrefixedMsg counts events with msg whose key starts with prefix.
func countPrefixedMsg(ev [][2]string, msg, prefix string) int {
	n := 0
	for _, e := range ev {
		if e[0] == msg && strings.HasPrefix(e[1], prefix) {
			n++
		}
	}
	return n
}

// countInstanceMsg counts events with the given reclaim message.
func countInstanceMsg(ev [][2]string, msg string) int {
	n := 0
	for _, e := range ev {
		if e[0] == msg {
			n++
		}
	}
	return n
}

// shortInstanceLeases resets the process table to a 25ms grace and captures logs.
func shortInstanceLeases(t *testing.T) *instanceLog {
	t.Helper()
	dbwrappers.Reset()
	t.Cleanup(dbwrappers.Reset)
	h := &instanceLog{}
	dbwrappers.ResetWith(100*time.Millisecond, slog.New(h))
	return h
}

// instanceModuleRoot walks parents until go.mod so seed paths work from any test cwd.
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

// instanceBIN is an enabled IP2Location config that allows US.
func instanceBIN(path string) *Config {
	return &Config{
		Enabled: true,
		DatabaseSources: map[string]geoblock.DatabaseSource{
			"seed": {Path: path, DatabaseType: dbsource.TypeBIN},
		},
		Ip2locationSourceGeo: "seed",
		AllowedCountries:     []string{"US"},
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-real-ip"},
		IPHeaderStrategy:     geoblock.IPHeaderStrategyCheckAll,
		BanIfError:           true,
	}
}

// instanceMaxMind is an enabled MaxMind config that points at a seed MMDB.
func instanceMaxMind(path string) *Config {
	return &Config{
		Enabled:          true,
		DatabaseProvider: geoblock.DatabaseProviderMaxMind,
		DatabaseSources: map[string]geoblock.DatabaseSource{
			"seed": {Path: path, DatabaseType: dbsource.TypeMMDB},
		},
		MaxmindSource:        "seed",
		AllowedCountries:     []string{"GB"},
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-real-ip"},
		IPHeaderStrategy:     geoblock.IPHeaderStrategyCheckAll,
		BanIfError:           true,
	}
}

// waitReclaimEvents waits until pred is true or timeout.
func waitReclaimEvents(t *testing.T, h *instanceLog, timeout time.Duration, pred func([][2]string) bool) [][2]string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		ev := h.events()
		if pred(ev) {
			return ev
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for reclaim events, ev=%+v", ev)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// mustRootPlugin calls root New and type-asserts *geoblock.Route.
func mustRootPlugin(t *testing.T, ctx context.Context, cfg *Config, name string) *geoblock.Route {
	t.Helper()
	h, err := New(ctx, noopNext{}, cfg, name)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	p, ok := h.(*geoblock.Route)
	if !ok {
		t.Fatalf("handler type %T", h)
	}
	return p
}

// noopNext is a next handler that does nothing.
type noopNext struct{}

func (noopNext) ServeHTTP(http.ResponseWriter, *http.Request) {}

// countHandler increments n on each ServeHTTP.
type countHandler struct{ n *int }

func (c countHandler) ServeHTTP(http.ResponseWriter, *http.Request) { *c.n++ }

// usPassRequest is a GET with X-Real-IP 8.8.8.8 (US in the LITE seed).
func usPassRequest() *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "8.8.8.8")
	return req
}

// requireLookupUS fails unless Lookup(8.8.8.8) returns country US.
func requireLookupUS(t *testing.T, p *geoblock.Route) {
	t.Helper()
	rec, err := p.Lookup("8.8.8.8")
	if err != nil || rec.Country != "US" {
		t.Fatalf("lookup: %+v %v", rec, err)
	}
}

func TestNew_SameNameConfigSharesIncarnation(t *testing.T) {
	shortInstanceLeases(t)
	cfg := instanceBIN(filepath.Join(instanceModuleRoot(), "seeds", "IP2LOCATION-LITE-DB1.IPV6.BIN"))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	var n1, n2 int
	a, err := New(ctx, countHandler{&n1}, cfg, "geoblock")
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(ctx, countHandler{&n2}, cfg, "geoblock")
	if err != nil {
		t.Fatal(err)
	}
	pa, pb := a.(*geoblock.Route), b.(*geoblock.Route)
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
	cfg := instanceBIN(filepath.Join(instanceModuleRoot(), "seeds", "IP2LOCATION-LITE-DB1.IPV6.BIN"))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	a := mustRootPlugin(t, ctx, cfg, "geoblock")
	b := mustRootPlugin(t, ctx, cfg, "geoblock-b")
	if a.SameCore(b) {
		t.Fatal("different names must not share incarnation")
	}
}

func TestNew_ConfigMissSeparateIncarnation(t *testing.T) {
	shortInstanceLeases(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	seed := filepath.Join(instanceModuleRoot(), "seeds", "IP2LOCATION-LITE-DB1.IPV6.BIN")
	a := mustRootPlugin(t, ctx, instanceBIN(seed), "geoblock")
	cfgB := instanceBIN(seed)
	cfgB.AllowedCountries = []string{"DE"}
	b := mustRootPlugin(t, ctx, cfgB, "geoblock")
	if a.SameCore(b) {
		t.Fatal("different config must not share incarnation")
	}
}

func TestNew_PluginReclaimAfterGenerationCancel(t *testing.T) {
	h := shortInstanceLeases(t)
	cfg := instanceBIN(filepath.Join(instanceModuleRoot(), "seeds", "IP2LOCATION-LITE-DB1.IPV6.BIN"))
	gen, cancel := context.WithCancel(context.Background())
	_ = mustRootPlugin(t, gen, cfg, "geoblock")
	puts := countPluginPuts(h.events())
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
	if countPluginPuts(ev) != 1 {
		t.Fatalf("reclaim must not create plugin, ev=%+v", ev)
	}
	if countInstanceMsg(ev, reclaim.MsgReclaim) == 0 {
		t.Fatalf("expected reclaim, ev=%+v", ev)
	}
}

func TestNew_PluginDisposeAfterGrace(t *testing.T) {
	h := shortInstanceLeases(t)
	cfg := instanceBIN(filepath.Join(instanceModuleRoot(), "seeds", "IP2LOCATION-LITE-DB1.IPV6.BIN"))
	ctx, cancel := context.WithCancel(context.Background())
	_ = mustRootPlugin(t, ctx, cfg, "geoblock")
	cancel()
	time.Sleep(250 * time.Millisecond)
	puts := countPluginPuts(h.events())
	next, cancelNext := context.WithCancel(context.Background())
	defer cancelNext()
	again := mustRootPlugin(t, next, cfg, "geoblock")
	requireLookupUS(t, again)
	if countPluginPuts(h.events()) <= puts {
		t.Fatalf("later New must create a new plugin incarnation, ev=%+v", h.events())
	}
}

const (
	testPrefixBIN  = "bin:"
	testPrefixMMDB = "mmdb:"
)

// TestNew_DifferentConfigSharesBINWrappers checks two policy configs share one geo BIN and one ASN wrapper.
func TestNew_DifferentConfigSharesBINWrappers(t *testing.T) {
	h := shortInstanceLeases(t)
	seed := filepath.Join(instanceModuleRoot(), "seeds", "IP2LOCATION-LITE-DB1.IPV6.BIN")
	ctx, cancel := context.WithCancel(context.Background())
	a := mustRootPlugin(t, ctx, instanceBIN(seed), "geoblock")
	cfgB := instanceBIN(seed)
	cfgB.AllowedCountries = []string{"DE"}
	b := mustRootPlugin(t, ctx, cfgB, "geoblock")
	if a.SameCore(b) {
		t.Fatal("different config must not share plugin incarnation")
	}
	ev := h.events()
	if got := countPluginPuts(ev); got != 2 {
		t.Fatalf("plugin puts: got %d want 2 ev=%+v", got, ev)
	}
	// IP2Location always opens geo plus allow-missing ASN. Two plugins must not double those.
	if got := countPrefixedMsg(ev, reclaim.MsgPut, testPrefixBIN); got != 2 {
		t.Fatalf("bin puts: got %d want 2 (geo+asn) ev=%+v", got, ev)
	}

	cancel()
	ev = waitReclaimEvents(t, h, time.Second, func(ev [][2]string) bool {
		return countPrefixedMsg(ev, reclaim.MsgDispose, keyPrefixPlugin) == 2 &&
			countPrefixedMsg(ev, reclaim.MsgDispose, testPrefixBIN) == 2
	})
	if got := countPrefixedMsg(ev, reclaim.MsgPut, testPrefixBIN); got != 2 {
		t.Fatalf("bin must not put again during dispose ev=%+v", ev)
	}

	again, cancelAgain := context.WithCancel(context.Background())
	defer cancelAgain()
	_ = mustRootPlugin(t, again, instanceBIN(seed), "geoblock")
	ev = h.events()
	if got := countPluginPuts(ev); got != 3 {
		t.Fatalf("later New must put a new plugin, got %d ev=%+v", got, ev)
	}
	if got := countPrefixedMsg(ev, reclaim.MsgPut, testPrefixBIN); got != 4 {
		t.Fatalf("later New must put new BIN wrappers, got %d ev=%+v", got, ev)
	}
}

// TestNew_DifferentConfigSharesMMDBWrapper checks two policy configs share one MMDB wrapper.
func TestNew_DifferentConfigSharesMMDBWrapper(t *testing.T) {
	h := shortInstanceLeases(t)
	seed := filepath.Join(instanceModuleRoot(), "seeds", "GeoIP2-Country-Test.mmdb")
	ctx, cancel := context.WithCancel(context.Background())
	cfgA := instanceMaxMind(seed)
	a := mustRootPlugin(t, ctx, cfgA, "geoblock")
	cfgB := instanceMaxMind(seed)
	cfgB.AllowedCountries = []string{"DE"}
	b := mustRootPlugin(t, ctx, cfgB, "geoblock")
	if a.SameCore(b) {
		t.Fatal("different config must not share plugin incarnation")
	}
	ev := h.events()
	if got := countPluginPuts(ev); got != 2 {
		t.Fatalf("plugin puts: got %d want 2 ev=%+v", got, ev)
	}
	if got := countPrefixedMsg(ev, reclaim.MsgPut, testPrefixMMDB); got != 1 {
		t.Fatalf("mmdb puts: got %d want 1 ev=%+v", got, ev)
	}

	cancel()
	ev = waitReclaimEvents(t, h, time.Second, func(ev [][2]string) bool {
		return countPrefixedMsg(ev, reclaim.MsgDispose, keyPrefixPlugin) == 2 &&
			countPrefixedMsg(ev, reclaim.MsgDispose, testPrefixMMDB) == 1
	})
	if got := countPrefixedMsg(ev, reclaim.MsgPut, testPrefixMMDB); got != 1 {
		t.Fatalf("mmdb must not put again during dispose ev=%+v", ev)
	}

	again, cancelAgain := context.WithCancel(context.Background())
	defer cancelAgain()
	_ = mustRootPlugin(t, again, instanceMaxMind(seed), "geoblock")
	ev = h.events()
	if got := countPluginPuts(ev); got != 3 {
		t.Fatalf("later New must put a new plugin, got %d ev=%+v", got, ev)
	}
	if got := countPrefixedMsg(ev, reclaim.MsgPut, testPrefixMMDB); got != 2 {
		t.Fatalf("later New must put a new MMDB wrapper, got %d ev=%+v", got, ev)
	}
}

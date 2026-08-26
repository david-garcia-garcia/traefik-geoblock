package traefik_geoblock

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// CI floors. Set from local/CI measurements with headroom for shared runners.
// Raise them after you have a few CI runs; keep them well below observed ops/s.
const (
	minLookupOpsPerSec  = 20000
	minRequestOpsPerSec = 20000
	throughputWindow    = 400 * time.Millisecond
	throughputWarmup    = 2000
)

func newThroughputPlugin(tb testing.TB) *Plugin {
	tb.Helper()
	handler, err := New(context.TODO(), &noopHandler{}, &Config{
		Enabled:                     true,
		Ip2locationDatabaseFilePath: dbFilePath,
		AllowedCountries:            []string{"US", "AU"},
		DefaultAllow:                false,
		AllowPrivate:                false,
		BanIfError:                  true,
		DisallowedStatusCode:        http.StatusForbidden,
		IPHeaders:                   []string{"x-real-ip"},
		IPHeaderStrategy:            IPHeaderStrategyCheckFirst,
		LogLevel:                    "error",
		LogFormat:                   "text",
	}, pluginName)
	if err != nil {
		tb.Fatalf("failed to create plugin: %v", err)
	}
	plugin, ok := handler.(*Plugin)
	if !ok {
		tb.Fatalf("expected *Plugin, got %T", handler)
	}
	return plugin
}

func requireMinThroughput(t *testing.T, name string, minOpsPerSec float64, op func()) {
	t.Helper()
	if testing.Short() {
		t.Skip("throughput gates skipped in short mode")
	}

	for i := 0; i < throughputWarmup; i++ {
		op()
	}

	start := time.Now()
	n := 0
	for time.Since(start) < throughputWindow {
		op()
		n++
	}
	elapsed := time.Since(start).Seconds()
	ops := float64(n) / elapsed
	t.Logf("%s: %.0f ops/s (%d ops in %.3fs; min %.0f)", name, ops, n, elapsed, minOpsPerSec)
	if ops < minOpsPerSec {
		t.Fatalf("%s throughput %.0f ops/s is below the CI floor of %.0f ops/s", name, ops, minOpsPerSec)
	}
}

func TestThroughput_IPLookup(t *testing.T) {
	plugin := newThroughputPlugin(t)
	ips := []string{"1.1.1.1", "8.8.8.8", "9.9.9.9"}
	i := 0
	requireMinThroughput(t, "Lookup", minLookupOpsPerSec, func() {
		_, err := plugin.Lookup(ips[i%len(ips)])
		if err != nil {
			t.Fatalf("Lookup failed: %v", err)
		}
		i++
	})
}

func TestThroughput_ServeHTTP(t *testing.T) {
	plugin := newThroughputPlugin(t)
	requireMinThroughput(t, "ServeHTTP", minRequestOpsPerSec, func() {
		req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		req.Header.Set("X-Real-IP", "8.8.8.8")
		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)
		if rr.Code != http.StatusTeapot {
			t.Fatalf("unexpected status %d", rr.Code)
		}
	})
}

func BenchmarkPlugin_Lookup(b *testing.B) {
	plugin := newThroughputPlugin(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := plugin.Lookup("8.8.8.8"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPlugin_ServeHTTP(b *testing.B) {
	plugin := newThroughputPlugin(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		req.Header.Set("X-Real-IP", "8.8.8.8")
		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)
	}
}

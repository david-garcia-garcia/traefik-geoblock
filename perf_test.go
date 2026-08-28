package traefik_geoblock

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// CI floors. golang:1.21 docker measured ~2.1k LITE Lookup/ServeHTTP and ~0.7k DB8.
// 20k was a desktop-class floor and flakes on shared GitHub runners. Catch collapses, not noise.
const (
	minThroughputOpsPerSec = 300
	throughputWindow       = 400 * time.Millisecond
	throughputWarmup       = 2000
)

func newThroughputPlugin(tb testing.TB) *Plugin {
	tb.Helper()
	handler, err := New(context.TODO(), &noopHandler{}, &Config{
		Enabled:                     true,
		DatabaseDownloads: seedCatalog(dbFilePath), Ip2locationDownloadGeo: "seed",
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

func requireMinThroughput(t *testing.T, name string, op func()) {
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
	t.Logf("%s: %.0f ops/s (%d ops in %.3fs; min %.0f)", name, ops, n, elapsed, float64(minThroughputOpsPerSec))
	if ops < minThroughputOpsPerSec {
		t.Fatalf("%s throughput %.0f ops/s is below the CI floor of %.0f ops/s", name, ops, float64(minThroughputOpsPerSec))
	}
}

func TestThroughput_IPLookup(t *testing.T) {
	plugin := newThroughputPlugin(t)
	ips := []string{"1.1.1.1", "8.8.8.8", "9.9.9.9"}
	i := 0
	requireMinThroughput(t, "Lookup", func() {
		_, err := plugin.Lookup(ips[i%len(ips)])
		if err != nil {
			t.Fatalf("Lookup failed: %v", err)
		}
		i++
	})
}

func TestThroughput_ServeHTTP(t *testing.T) {
	plugin := newThroughputPlugin(t)
	requireMinThroughput(t, "ServeHTTP", func() {
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

// Plugin-only allocs: request and recorder are reused (Traefik does this).
func BenchmarkPlugin_ServeHTTP_Enrich(b *testing.B) {
	handler, err := New(context.TODO(), &noopHandler{}, &Config{
		Enabled:                     true,
		DatabaseDownloads: seedCatalog(dbFilePath), Ip2locationDownloadGeo: "seed",
		AllowedCountries:            []string{"US", "AU"},
		DefaultAllow:                false,
		AllowPrivate:                false,
		BanIfError:                  true,
		DisallowedStatusCode:        http.StatusForbidden,
		IPHeaders:                   []string{"x-real-ip"},
		IPHeaderStrategy:            IPHeaderStrategyCheckFirst,
		LogLevel:                    "error",
		LogFormat:                   "text",
		RequestHeaderEnrich: map[string]string{
			"X-Geo-Country": "country",
			"X-Geo-Region":  "region",
			"X-Geo-City":    "city",
		},
	}, pluginName)
	if err != nil {
		b.Fatal(err)
	}
	plugin := handler.(*Plugin)
	req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
	req.Header.Set("X-Real-IP", "8.8.8.8")
	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin.ServeHTTP(rr, req)
	}
}

func newLITEASNPlugin(tb testing.TB) *Plugin {
	tb.Helper()
	asn := requireASN(tb)
	handler, err := New(context.TODO(), &noopHandler{}, &Config{
		Enabled:                        true,
		DatabaseDownloads:              seedCatalogPair(dbFilePath, asn),
		Ip2locationDownloadGeo:         "geo",
		Ip2locationDownloadAsn:         "asn",
		AllowedCountries:               []string{"US", "AU"},
		DefaultAllow:                   false,
		AllowPrivate:                   false,
		BanIfError:                     true,
		DisallowedStatusCode:           http.StatusForbidden,
		IPHeaders:                      []string{"x-real-ip"},
		IPHeaderStrategy:               IPHeaderStrategyCheckFirst,
		LogLevel:                       "error",
		LogFormat:                      "text",
	}, pluginName)
	if err != nil {
		tb.Fatalf("failed to create LITE+ASN plugin: %v", err)
	}
	return handler.(*Plugin)
}

func newDB8ASNPlugin(tb testing.TB) *Plugin {
	tb.Helper()
	path := requireDB8(tb)
	asn := requireASN(tb)
	handler, err := New(context.TODO(), &noopHandler{}, &Config{
		Enabled:                        true,
		DatabaseDownloads:              seedCatalogPair(path, asn),
		Ip2locationDownloadGeo:         "geo",
		Ip2locationDownloadAsn:         "asn",
		AllowedCountries:               []string{"US", "AU"},
		DefaultAllow:                   false,
		AllowPrivate:                   false,
		BanIfError:                     true,
		DisallowedStatusCode:           http.StatusForbidden,
		IPHeaders:                      []string{"x-real-ip"},
		IPHeaderStrategy:               IPHeaderStrategyCheckFirst,
		LogLevel:                       "error",
		LogFormat:                      "text",
		RequestHeaderEnrich:            fullEnrichHeaders,
	}, pluginName)
	if err != nil {
		tb.Fatalf("failed to create DB8+ASN plugin: %v", err)
	}
	return handler.(*Plugin)
}

func newDB8CountryPlugin(tb testing.TB) *Plugin {
	tb.Helper()
	path := requireDB8(tb)
	handler, err := New(context.TODO(), &noopHandler{}, &Config{
		Enabled:                     true,
		DatabaseDownloads: seedCatalog(path), Ip2locationDownloadGeo: "seed",
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
		tb.Fatalf("failed to create DB8 country plugin: %v", err)
	}
	plugin, ok := handler.(*Plugin)
	if !ok {
		tb.Fatalf("expected *Plugin, got %T", handler)
	}
	return plugin
}

func newDB8FullEnrichPlugin(tb testing.TB) *Plugin {
	tb.Helper()
	path := requireDB8(tb)
	handler, err := New(context.TODO(), &noopHandler{}, &Config{
		Enabled:                     true,
		DatabaseDownloads: seedCatalog(path), Ip2locationDownloadGeo: "seed",
		AllowedCountries:            []string{"US", "AU"},
		DefaultAllow:                false,
		AllowPrivate:                false,
		BanIfError:                  true,
		DisallowedStatusCode:        http.StatusForbidden,
		IPHeaders:                   []string{"x-real-ip"},
		IPHeaderStrategy:            IPHeaderStrategyCheckFirst,
		LogLevel:                    "error",
		LogFormat:                   "text",
		RequestHeaderEnrich:         fullEnrichHeaders,
	}, pluginName)
	if err != nil {
		tb.Fatalf("failed to create DB8 enrich plugin: %v", err)
	}
	plugin, ok := handler.(*Plugin)
	if !ok {
		tb.Fatalf("expected *Plugin, got %T", handler)
	}
	return plugin
}

func TestThroughput_ServeHTTP_DB8Country(t *testing.T) {
	plugin := newDB8CountryPlugin(t)
	req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
	req.Header.Set("X-Real-IP", "8.8.8.8")
	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)
	requireMinThroughput(t, "ServeHTTP_DB8Country", func() {
		plugin.ServeHTTP(rr, req)
		if rr.Code != http.StatusTeapot {
			t.Fatalf("unexpected status %d", rr.Code)
		}
	})
}

func TestThroughput_ServeHTTP_DB8FullEnrich(t *testing.T) {
	plugin := newDB8FullEnrichPlugin(t)
	req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
	req.Header.Set("X-Real-IP", "8.8.8.8")
	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)
	if req.Header.Get("X-Geo-Isp") == "" || req.Header.Get("X-Geo-Domain") == "" {
		t.Fatalf("expected DB8 to write isp/domain, isp=%q domain=%q", req.Header.Get("X-Geo-Isp"), req.Header.Get("X-Geo-Domain"))
	}
	requireMinThroughput(t, "ServeHTTP_DB8FullEnrich", func() {
		plugin.ServeHTTP(rr, req)
		if rr.Code != http.StatusTeapot {
			t.Fatalf("unexpected status %d", rr.Code)
		}
	})
}

func BenchmarkPlugin_Lookup_LITEASN(b *testing.B) {
	plugin := newLITEASNPlugin(b)
	rec, err := plugin.Lookup("8.8.8.8")
	if err != nil {
		b.Fatal(err)
	}
	if rec.Asn == "" {
		b.Fatal("expected ASN from ASN BIN")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := plugin.Lookup("8.8.8.8"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPlugin_ServeHTTP_LITEASN(b *testing.B) {
	plugin := newLITEASNPlugin(b)
	req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
	req.Header.Set("X-Real-IP", "8.8.8.8")
	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin.ServeHTTP(rr, req)
	}
}

func BenchmarkPlugin_Lookup_DB8ASN(b *testing.B) {
	plugin := newDB8ASNPlugin(b)
	rec, err := plugin.Lookup("8.8.8.8")
	if err != nil {
		b.Fatal(err)
	}
	if rec.Asn == "" {
		b.Fatal("expected ASN from ASN BIN")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := plugin.Lookup("8.8.8.8"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPlugin_ServeHTTP_DB8ASN(b *testing.B) {
	plugin := newDB8ASNPlugin(b)
	req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
	req.Header.Set("X-Real-IP", "8.8.8.8")
	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)
	if req.Header.Get("X-Geo-Asn") == "" {
		b.Fatal("expected X-Geo-Asn from ASN BIN")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin.ServeHTTP(rr, req)
	}
}

func BenchmarkPlugin_Lookup_DB8Country(b *testing.B) {
	plugin := newDB8CountryPlugin(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := plugin.Lookup("8.8.8.8"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPlugin_ServeHTTP_DB8Country(b *testing.B) {
	plugin := newDB8CountryPlugin(b)
	req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
	req.Header.Set("X-Real-IP", "8.8.8.8")
	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin.ServeHTTP(rr, req)
	}
}

func BenchmarkPlugin_Lookup_DB8(b *testing.B) {
	plugin := newDB8FullEnrichPlugin(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := plugin.Lookup("8.8.8.8"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPlugin_ServeHTTP_DB8FullEnrich(b *testing.B) {
	plugin := newDB8FullEnrichPlugin(b)
	req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
	req.Header.Set("X-Real-IP", "8.8.8.8")
	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin.ServeHTTP(rr, req)
	}
}

func BenchmarkPlugin_ServeHTTP_Reuse(b *testing.B) {
	plugin := newThroughputPlugin(b)
	req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
	req.Header.Set("X-Real-IP", "8.8.8.8")
	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin.ServeHTTP(rr, req)
	}
}

const maxmindDummyIP = "81.2.69.142"

func newMaxMindPlugin(tb testing.TB) *Plugin {
	tb.Helper()
	handler, err := New(context.TODO(), &noopHandler{}, &Config{
		Enabled:                 true,
		DatabaseProvider:        DatabaseProviderMaxMind,
		DatabaseDownloads: seedCatalog(maxmindFilePath), MaxmindDownload: "seed",
		AllowedCountries:        []string{"GB"},
		DefaultAllow:            false,
		AllowPrivate:            false,
		BanIfError:              true,
		DisallowedStatusCode:    http.StatusForbidden,
		IPHeaders:               []string{"x-real-ip"},
		IPHeaderStrategy:        IPHeaderStrategyCheckFirst,
		LogLevel:                "error",
		LogFormat:               "text",
	}, pluginName)
	if err != nil {
		tb.Fatalf("failed to create MaxMind plugin: %v", err)
	}
	plugin, ok := handler.(*Plugin)
	if !ok {
		tb.Fatalf("expected *Plugin, got %T", handler)
	}
	return plugin
}

func TestThroughput_ServeHTTP_MaxMind(t *testing.T) {
	plugin := newMaxMindPlugin(t)
	req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
	req.Header.Set("X-Real-IP", maxmindDummyIP)
	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)
	requireMinThroughput(t, "ServeHTTP_MaxMind", func() {
		plugin.ServeHTTP(rr, req)
		if rr.Code != http.StatusTeapot {
			t.Fatalf("unexpected status %d", rr.Code)
		}
	})
}

func BenchmarkPlugin_Lookup_MaxMind(b *testing.B) {
	plugin := newMaxMindPlugin(b)
	rec, err := plugin.Lookup(maxmindDummyIP)
	if err != nil {
		b.Fatal(err)
	}
	if rec.Country != "GB" {
		b.Fatalf("expected GB from dummy fixture, got %q", rec.Country)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := plugin.Lookup(maxmindDummyIP); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPlugin_ServeHTTP_MaxMind(b *testing.B) {
	plugin := newMaxMindPlugin(b)
	req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
	req.Header.Set("X-Real-IP", maxmindDummyIP)
	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)
	if rr.Code != http.StatusTeapot {
		b.Fatalf("unexpected status %d", rr.Code)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin.ServeHTTP(rr, req)
	}
}

func BenchmarkPlugin_ServeHTTP_MaxMindEnrich(b *testing.B) {
	handler, err := New(context.TODO(), &noopHandler{}, &Config{
		Enabled:                 true,
		DatabaseProvider:        DatabaseProviderMaxMind,
		DatabaseDownloads: seedCatalog(maxmindFilePath), MaxmindDownload: "seed",
		AllowedCountries:        []string{"GB"},
		DefaultAllow:            false,
		AllowPrivate:            false,
		BanIfError:              true,
		DisallowedStatusCode:    http.StatusForbidden,
		IPHeaders:               []string{"x-real-ip"},
		IPHeaderStrategy:        IPHeaderStrategyCheckFirst,
		LogLevel:                "error",
		LogFormat:               "text",
		RequestHeaderEnrich: map[string]string{
			"X-Geo-Country": "country",
			"X-Geo-Region":  "region",
			"X-Geo-City":    "city",
		},
	}, pluginName)
	if err != nil {
		b.Fatal(err)
	}
	plugin := handler.(*Plugin)
	req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
	req.Header.Set("X-Real-IP", maxmindDummyIP)
	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plugin.ServeHTTP(rr, req)
	}
}

package geoblock

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/logging"
)

const (
	testUSIP        = "8.8.8.8"
	testAUIP        = "1.1.1.1"
	testTrustedCIDR = "10.10.60.0/24"
	testTrustedPeer = "10.10.60.5:443"
)

// rightmostCfg is a RightmostUntrusted plugin config with blocked US and the seed catalog.
func rightmostCfg(headers []string, trusted []string) *Config {
	return &Config{
		Mode:                 ModeEnrichAndBlock,
		DatabaseSources:      seedCatalog(dbFilePath),
		AllowPrivate:         true,
		DefaultAllow:         true,
		BlockedCountries:     []string{"US"},
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            headers,
		IPHeaderStrategy:     IPHeaderStrategyRightmostUntrusted,
		TrustedProxies:       trusted,
		CountryHeader:        "x-country-code",
		BanIfError:           true,
	}
}

// serveRightmost runs one GET through a RightmostUntrusted plugin.
func serveRightmost(t *testing.T, cfg *Config, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
	if err != nil {
		t.Fatalf("create plugin: %v", err)
	}
	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)
	return rr
}

// newRightmostPlugin is a RightmostUntrusted Plugin for hop-list asserts.
func newRightmostPlugin(t *testing.T, headers []string, trusted []string) *Plugin {
	t.Helper()
	plugin, err := newTestPlugin(holdCtx(t), rightmostCfg(headers, trusted), pluginName)
	if err != nil {
		t.Fatalf("create plugin: %v", err)
	}
	return plugin
}

// evaluatedHop is the RightmostUntrusted selected hop, or empty.
func evaluatedHop(t *testing.T, plugin *Plugin, req *http.Request) string {
	t.Helper()
	hops := plugin.rightmostUntrusted(plugin.rightmostChain(req))
	if len(hops) == 0 {
		return ""
	}
	return hops[0]
}

func TestRightmostUntrusted_DedupBypass(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "8.8.8.8, 1.1.1.1, 8.8.8.8")
	rr := serveRightmost(t, rightmostCfg([]string{"x-forwarded-for"}, []string{testTrustedCIDR}), req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("status %d, want %d", rr.Code, http.StatusForbidden)
	}
	if got := req.Header.Get("x-country-code"); got != "US" {
		t.Errorf("country %q, want US", got)
	}
}

func TestRightmostUntrusted_SecondFieldLine(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Add("X-Forwarded-For", testAUIP)
	req.Header.Add("X-Forwarded-For", testUSIP)
	rr := serveRightmost(t, rightmostCfg([]string{"x-forwarded-for"}, nil), req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("status %d, want %d", rr.Code, http.StatusForbidden)
	}
	if got := req.Header.Get("x-country-code"); got != "US" {
		t.Errorf("country %q, want US", got)
	}
}

func TestRightmostUntrusted_MixedFieldLines(t *testing.T) {
	plugin := newRightmostPlugin(t, []string{"x-forwarded-for"}, []string{testTrustedCIDR})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Add("X-Forwarded-For", "1.1.1.1, 2.2.2.2")
	req.Header.Add("X-Forwarded-For", "8.8.8.8, 10.10.60.5")
	if got := evaluatedHop(t, plugin, req); got != testUSIP {
		t.Errorf("evaluated hop %q, want 8.8.8.8", got)
	}
}

func TestRightmostUntrusted_TrustedRemoteAddrUsesHeader(t *testing.T) {
	plugin := newRightmostPlugin(t, []string{"x-forwarded-for", "remoteAddress"}, []string{testTrustedCIDR})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", testUSIP)
	req.RemoteAddr = testTrustedPeer
	if got := evaluatedHop(t, plugin, req); got != testUSIP {
		t.Errorf("evaluated hop %q, want 8.8.8.8", got)
	}
}

func TestRightmostUntrusted_RepeatedHeaderAndRemoteAddr(t *testing.T) {
	plugin := newRightmostPlugin(t, []string{"x-forwarded-for", "remoteAddress"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", testAUIP)
	req.RemoteAddr = "1.1.1.1:5000"
	if got := evaluatedHop(t, plugin, req); got != testAUIP {
		t.Errorf("evaluated hop %q, want 1.1.1.1", got)
	}
}

func TestRightmostUntrusted_FirstPresentHeaderWins(t *testing.T) {
	plugin := newRightmostPlugin(t, []string{"cf-connecting-ip", "x-forwarded-for", "remoteAddress"}, []string{testTrustedCIDR})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("CF-Connecting-IP", testAUIP)
	req.Header.Set("X-Forwarded-For", testUSIP)
	req.RemoteAddr = testTrustedPeer
	if got := evaluatedHop(t, plugin, req); got != testAUIP {
		t.Errorf("evaluated hop %q, want 1.1.1.1", got)
	}
}

func TestRightmostUntrusted_LaterHeaderWhenEarlierAbsent(t *testing.T) {
	plugin := newRightmostPlugin(t, []string{"cf-connecting-ip", "x-forwarded-for", "remoteAddress"}, []string{testTrustedCIDR})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", testUSIP)
	req.RemoteAddr = testTrustedPeer
	if got := evaluatedHop(t, plugin, req); got != testUSIP {
		t.Errorf("evaluated hop %q, want 8.8.8.8", got)
	}
}

func TestRightmostUntrusted_OnlyCfConnectingIP(t *testing.T) {
	plugin := newRightmostPlugin(t, []string{"cf-connecting-ip", "x-forwarded-for", "remoteAddress"}, []string{testTrustedCIDR})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("CF-Connecting-IP", testUSIP)
	req.RemoteAddr = testTrustedPeer
	if got := evaluatedHop(t, plugin, req); got != testUSIP {
		t.Errorf("evaluated hop %q, want 8.8.8.8", got)
	}
}

func TestRightmostUntrusted_UntrustedPeerIgnoresHeaders(t *testing.T) {
	plugin := newRightmostPlugin(t, []string{"cf-connecting-ip", "x-forwarded-for", "remoteAddress"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", testAUIP)
	req.RemoteAddr = "8.8.8.8:5000"
	if got := evaluatedHop(t, plugin, req); got != testUSIP {
		t.Errorf("evaluated hop %q, want 8.8.8.8", got)
	}
}

func TestRightmostUntrusted_DefaultHeadersFirstPresent(t *testing.T) {
	plugin := newRightmostPlugin(t, []string{"x-forwarded-for", "x-real-ip"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", testUSIP)
	req.Header.Set("X-Real-IP", testAUIP)
	if got := evaluatedHop(t, plugin, req); got != testUSIP {
		t.Errorf("evaluated hop %q, want 8.8.8.8", got)
	}
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", testUSIP)
	req.Header.Set("X-Real-IP", testAUIP)
	rr := serveRightmost(t, rightmostCfg([]string{"x-forwarded-for", "x-real-ip"}, nil), req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("status %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func TestRightmostUntrusted_DefaultHeadersAccepted(t *testing.T) {
	_, err := newRoute(holdCtx(t), &noopHandler{}, rightmostCfg([]string{"x-forwarded-for", "x-real-ip"}, nil), pluginName)
	if err != nil {
		t.Fatalf("default ipHeaders should load: %v", err)
	}
}

func TestRightmostUntrusted_RemoteAddressNotLastFails(t *testing.T) {
	_, err := newRoute(holdCtx(t), &noopHandler{}, rightmostCfg([]string{"remoteAddress", "x-forwarded-for"}, nil), pluginName)
	if err == nil {
		t.Fatal("expected creation error when remoteAddress is not last")
	}
	if !strings.Contains(err.Error(), "remoteAddress") {
		t.Errorf("error should name remoteAddress, got %v", err)
	}
}

func TestRightmostUntrusted_MissingRemoteAddressWarns(t *testing.T) {
	var buf bytes.Buffer
	SetTestPluginLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { SetTestPluginLogger(nil) })
	_, err := newRoute(holdCtx(t), &noopHandler{}, rightmostCfg([]string{"x-forwarded-for"}, nil), pluginName)
	if err != nil {
		t.Fatalf("missing remoteAddress should load: %v", err)
	}
	if !strings.Contains(buf.String(), "without checking the direct peer") {
		t.Fatalf("want missing-remoteAddress warning, got %q", buf.String())
	}
}

func TestRightmostUntrusted_NoForwardingHeaderUsesRemoteAddr(t *testing.T) {
	plugin := newRightmostPlugin(t, []string{"x-forwarded-for", "remoteAddress"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "8.8.8.8:5000"
	if got := evaluatedHop(t, plugin, req); got != testUSIP {
		t.Errorf("evaluated hop %q, want 8.8.8.8", got)
	}
}

func TestRightmostUntrusted_TrustedProxiesRejectedWithCheckAll(t *testing.T) {
	cfg := rightmostCfg([]string{"x-forwarded-for"}, []string{"10.0.0.0/8"})
	cfg.IPHeaderStrategy = IPHeaderStrategyCheckAll
	_, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
	if err == nil {
		t.Fatal("expected creation error for trustedProxies with CheckAll")
	}
	if !strings.Contains(err.Error(), "trustedProxies") {
		t.Errorf("error should name trustedProxies, got %v", err)
	}
}

func TestRightmostUntrusted_IPv4ZeroDoesNotTrustIPv6(t *testing.T) {
	plugin := newRightmostPlugin(t, []string{"x-forwarded-for"}, []string{"0.0.0.0/0"})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "2001:db8::2")
	if got := evaluatedHop(t, plugin, req); got != "2001:db8::2" {
		t.Errorf("evaluated hop %q, want 2001:db8::2", got)
	}
}

func TestRightmostUntrusted_BareIPv6MatchesOnlyThatAddress(t *testing.T) {
	plugin := newRightmostPlugin(t, []string{"x-forwarded-for"}, []string{"2001:db8::1"})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "2001:db8::1, 2001:db8::2")
	if got := evaluatedHop(t, plugin, req); got != "2001:db8::2" {
		t.Errorf("evaluated hop %q, want 2001:db8::2", got)
	}
}

func TestRightmostUntrusted_InvalidTrustedProxiesEntry(t *testing.T) {
	_, err := newRoute(holdCtx(t), &noopHandler{}, rightmostCfg([]string{"x-forwarded-for"}, []string{"not-an-ip"}), pluginName)
	if err == nil {
		t.Fatal("expected creation error for not-an-ip")
	}
	if !strings.Contains(err.Error(), "not-an-ip") {
		t.Errorf("error should name not-an-ip, got %v", err)
	}
}

func TestRightmostUntrusted_InvalidTrustedProxiesPrefix(t *testing.T) {
	_, err := newRoute(holdCtx(t), &noopHandler{}, rightmostCfg([]string{"x-forwarded-for"}, []string{"10.0.0.0/33"}), pluginName)
	if err == nil {
		t.Fatal("expected creation error for 10.0.0.0/33")
	}
	if !strings.Contains(err.Error(), "10.0.0.0/33") {
		t.Errorf("error should name 10.0.0.0/33, got %v", err)
	}
}

func TestRightmostUntrusted_AllTrustedBanIfError(t *testing.T) {
	t.Run("ban", func(t *testing.T) {
		cfg := rightmostCfg([]string{"x-forwarded-for"}, []string{testTrustedCIDR})
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "10.10.60.5")
		rr := serveRightmost(t, cfg, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("status %d, want %d", rr.Code, http.StatusForbidden)
		}
	})
	t.Run("pass", func(t *testing.T) {
		cfg := rightmostCfg([]string{"x-forwarded-for"}, []string{testTrustedCIDR})
		cfg.BanIfError = false
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "10.10.60.5")
		rr := serveRightmost(t, cfg, req)
		if rr.Code != http.StatusTeapot {
			t.Errorf("status %d, want %d", rr.Code, http.StatusTeapot)
		}
	})
}

func TestRightmostUntrusted_TraceKeepsFullChain(t *testing.T) {
	var buf bytes.Buffer
	SetTestPluginLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: logging.LevelTrace})))
	t.Cleanup(func() { SetTestPluginLogger(nil) })
	cfg := rightmostCfg([]string{"x-forwarded-for"}, []string{testTrustedCIDR})
	cfg.LogLevel = "trace"
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "8.8.8.8, 1.1.1.1, 8.8.8.8")
	_ = serveRightmost(t, cfg, req)
	got := buf.String()
	if !strings.Contains(got, "8.8.8.8, 1.1.1.1, 8.8.8.8") {
		t.Errorf("trace should keep the full chain, got %q", got)
	}
	if !strings.Contains(got, "ip=8.8.8.8") {
		t.Errorf("trace should name the selected hop, got %q", got)
	}
}

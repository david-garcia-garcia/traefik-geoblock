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

func TestRightmostUntrusted_UnparsableHopIsEvaluated(t *testing.T) {
	plugin := newRightmostPlugin(t, []string{"x-forwarded-for"}, []string{testTrustedCIDR})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "not-an-ip, 10.10.60.5")
	if got := evaluatedHop(t, plugin, req); got != "not-an-ip" {
		t.Errorf("evaluated hop %q, want not-an-ip", got)
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

func TestRightmostUntrusted_AllTrustedEvaluatesOrigin(t *testing.T) {
	plugin := newRightmostPlugin(t, []string{"x-forwarded-for", "remoteAddress"}, []string{testTrustedCIDR})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "10.10.60.7, 10.10.60.6")
	req.RemoteAddr = testTrustedPeer
	if got := evaluatedHop(t, plugin, req); got != "10.10.60.7" {
		t.Errorf("evaluated hop %q, want the leftmost (origin) hop 10.10.60.7", got)
	}
}

// A load balancer health check reaches the plugin straight from a trusted proxy with no forwarding header.
func TestRightmostUntrusted_HealthCheckFromTrustedProxy(t *testing.T) {
	healthCheck := func() *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req.RemoteAddr = testTrustedPeer
		return req
	}
	t.Run("allowPrivate passes without a warning", func(t *testing.T) {
		var buf bytes.Buffer
		SetTestPluginLogger(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
		t.Cleanup(func() { SetTestPluginLogger(nil) })
		cfg := rightmostCfg([]string{"x-forwarded-for", "remoteAddress"}, []string{testTrustedCIDR})
		cfg.LogStatusDetailHeader = "X-Geoblock-Decision"
		req := healthCheck()
		rr := serveRightmost(t, cfg, req)
		if rr.Code != http.StatusTeapot {
			t.Errorf("status %d, want %d", rr.Code, http.StatusTeapot)
		}
		if got := req.Header.Get("X-Geoblock-Decision"); got != LogStatusPass+":"+PhaseAllowPrivate {
			t.Errorf("decision %q, want pass:%s", got, PhaseAllowPrivate)
		}
		if strings.Contains(buf.String(), "no client IP extracted") {
			t.Errorf("health check must not log the empty-hop warning, got %q", buf.String())
		}
	})
	t.Run("allowPrivate false blocks", func(t *testing.T) {
		cfg := rightmostCfg([]string{"x-forwarded-for", "remoteAddress"}, []string{testTrustedCIDR})
		cfg.AllowPrivate = false
		rr := serveRightmost(t, cfg, healthCheck())
		if rr.Code != http.StatusForbidden {
			t.Errorf("status %d, want %d", rr.Code, http.StatusForbidden)
		}
	})
}

func TestRightmostUntrusted_AllTrustedPublicOriginUsesCountry(t *testing.T) {
	cfg := rightmostCfg([]string{"x-forwarded-for", "remoteAddress"}, []string{"8.8.8.0/24", testTrustedCIDR})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", testUSIP)
	req.RemoteAddr = testTrustedPeer
	rr := serveRightmost(t, cfg, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("status %d, want %d", rr.Code, http.StatusForbidden)
	}
	if got := req.Header.Get("x-country-code"); got != "US" {
		t.Errorf("country %q, want US", got)
	}
}

func TestRightmostUntrusted_EmptyChainUsesBanIfError(t *testing.T) {
	for _, tc := range []struct {
		banIfError bool
		want       int
	}{{true, http.StatusForbidden}, {false, http.StatusTeapot}} {
		cfg := rightmostCfg([]string{"x-forwarded-for"}, []string{testTrustedCIDR})
		cfg.BanIfError = tc.banIfError
		rr := serveRightmost(t, cfg, httptest.NewRequest(http.MethodGet, "/test", nil))
		if rr.Code != tc.want {
			t.Errorf("banIfError=%v: status %d, want %d", tc.banIfError, rr.Code, tc.want)
		}
	}
}

// An unparsable evaluated hop has no country or CIDR decision; with banIfError off, defaultAllow decides it.
func TestRightmostUntrusted_UnparsableHopUsesDefaultAllow(t *testing.T) {
	for _, tc := range []struct {
		defaultAllow bool
		wantCode     int
		wantDecision string
	}{
		{false, http.StatusForbidden, LogStatusBlock + ":" + PhaseDefaultAllow},
		{true, http.StatusTeapot, LogStatusPass + ":" + PhaseDefaultAllow},
	} {
		cfg := rightmostCfg([]string{"cf-connecting-ip", "x-forwarded-for", "remoteAddress"}, []string{testTrustedCIDR})
		cfg.BanIfError = false
		cfg.DefaultAllow = tc.defaultAllow
		cfg.LogStatusDetailHeader = "X-Geoblock-Decision"
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("CF-Connecting-IP", "garbage")
		req.RemoteAddr = testTrustedPeer
		rr := serveRightmost(t, cfg, req)
		if rr.Code != tc.wantCode {
			t.Errorf("defaultAllow=%v: status %d, want %d", tc.defaultAllow, rr.Code, tc.wantCode)
		}
		if got := req.Header.Get("X-Geoblock-Decision"); got != tc.wantDecision {
			t.Errorf("defaultAllow=%v: decision %q, want %q", tc.defaultAllow, got, tc.wantDecision)
		}
	}
}

func TestRightmostUntrusted_UnparsableHopBanIfErrorBlocks(t *testing.T) {
	cfg := rightmostCfg([]string{"x-forwarded-for"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "garbage")
	rr := serveRightmost(t, cfg, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("status %d, want %d", rr.Code, http.StatusForbidden)
	}
}

// Forwarding headers are tried in ipHeaders order; the first one with a hop is used, empty ones are skipped.
func TestRightmostUntrusted_FirstNonEmptyHeaderWins(t *testing.T) {
	headers := []string{"cf-connecting-ip", "x-real-ip", "x-forwarded-for", "remoteAddress"}
	plugin := newRightmostPlugin(t, headers, []string{testTrustedCIDR})
	tests := []struct {
		name  string
		setup func(h http.Header)
		want  string
	}{
		{"present but empty is skipped", func(h http.Header) {
			h["Cf-Connecting-Ip"] = []string{""}
			h.Set("X-Forwarded-For", testUSIP)
		}, testUSIP},
		{"whitespace and commas only is skipped", func(h http.Header) {
			h.Set("CF-Connecting-IP", " , ,")
			h.Set("X-Real-IP", testAUIP)
			h.Set("X-Forwarded-For", testUSIP)
		}, testAUIP},
		{"earliest non-empty wins over later headers", func(h http.Header) {
			h.Set("X-Real-IP", testAUIP)
			h.Set("X-Forwarded-For", testUSIP)
		}, testAUIP},
		{"empty field line of the selected header is ignored", func(h http.Header) {
			h["X-Forwarded-For"] = []string{"", testUSIP}
		}, testUSIP},
		{"all empty falls back to RemoteAddr", func(h http.Header) {
			h["Cf-Connecting-Ip"] = []string{""}
			h["X-Forwarded-For"] = []string{" "}
		}, "10.10.60.5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			tt.setup(req.Header)
			req.RemoteAddr = testTrustedPeer
			if got := evaluatedHop(t, plugin, req); got != tt.want {
				t.Errorf("evaluated hop %q, want %q", got, tt.want)
			}
		})
	}
}

// trustedProxies accepts IPv4-mapped CIDRs without panicking (iplookup remaps them to the IPv4 tree).
func TestRightmostUntrusted_IPv4MappedCIDRTrusted(t *testing.T) {
	for _, entry := range []string{"::ffff:10.10.60.0/120", "::ffff:10.0.0.0/104", "::ffff:10.10.60.5/128"} {
		plugin := newRightmostPlugin(t, []string{"x-forwarded-for"}, []string{entry})
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "8.8.8.8, 10.10.60.5")
		if got := evaluatedHop(t, plugin, req); got != testUSIP {
			t.Errorf("%s: evaluated hop %q, want 8.8.8.8", entry, got)
		}
	}
}

// A bare IPv4-mapped literal is the IPv4 host, not the IPv6 range ::/32.
func TestRightmostUntrusted_BareIPv4MappedIsIPv4Host(t *testing.T) {
	plugin := newRightmostPlugin(t, []string{"x-forwarded-for"}, []string{"::ffff:10.10.60.5"})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "8.8.8.8, 10.10.60.5")
	if got := evaluatedHop(t, plugin, req); got != testUSIP {
		t.Errorf("evaluated hop %q, want 8.8.8.8 (10.10.60.5 is trusted)", got)
	}
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "8.8.8.8, ::5")
	if got := evaluatedHop(t, plugin, req); got != "::5" {
		t.Errorf("evaluated hop %q, want ::5 (not in ::/32)", got)
	}
}

func TestIPBlocks_IPv4MappedCIDRLoads(t *testing.T) {
	cfg := rightmostCfg([]string{"x-forwarded-for"}, nil)
	cfg.IPHeaderStrategy = IPHeaderStrategyCheckAll
	cfg.BlockedCountries = nil
	cfg.BlockedIPBlocks = []string{"::ffff:1.1.1.0/120"}
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", testAUIP)
	rr := serveRightmost(t, cfg, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("status %d, want %d (1.1.1.1 is in ::ffff:1.1.1.0/120)", rr.Code, http.StatusForbidden)
	}
}

func TestRightmostUntrusted_EmptyTrustedProxiesIsNil(t *testing.T) {
	for _, trusted := range [][]string{nil, {}, {" ", ""}} {
		plugin := newRightmostPlugin(t, []string{"x-forwarded-for"}, trusted)
		if plugin.trustedProxies != nil {
			t.Errorf("trustedProxies %q: helper should be nil", trusted)
		}
	}
	plugin := newRightmostPlugin(t, []string{"x-forwarded-for"}, []string{testTrustedCIDR})
	if plugin.trustedProxies == nil {
		t.Error("configured trustedProxies should build a helper")
	}
}

func TestRightmostUntrusted_ChainJoinedOnlyAtTrace(t *testing.T) {
	plugin := newRightmostPlugin(t, []string{"x-forwarded-for"}, []string{testTrustedCIDR})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "8.8.8.8, 10.10.60.5")
	remoteIPs, ipChain := plugin.clientHops(req)
	if len(remoteIPs) != 1 || remoteIPs[0] != testUSIP {
		t.Errorf("remoteIPs %v, want [8.8.8.8]", remoteIPs)
	}
	if ipChain != "" {
		t.Errorf("ipChain %q, want empty below trace", ipChain)
	}
}

func TestGetRemoteIPs_DedupeAndAllocs(t *testing.T) {
	plugin := newRightmostPlugin(t, []string{"x-forwarded-for", "x-real-ip", "remoteAddress"}, nil)
	plugin.ipHeaderStrategy = IPHeaderStrategyCheckAll
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "8.8.8.8, 1.1.1.1, 8.8.8.8")
	req.Header.Set("X-Real-IP", testAUIP)
	req.RemoteAddr = "2.2.2.2:443"
	if got := strings.Join(plugin.GetRemoteIPs(req), ","); got != "8.8.8.8,1.1.1.1,2.2.2.2" {
		t.Errorf("GetRemoteIPs %q, want 8.8.8.8,1.1.1.1,2.2.2.2", got)
	}

	single := httptest.NewRequest(http.MethodGet, "/test", nil)
	single.Header.Set("X-Forwarded-For", testUSIP)
	single.RemoteAddr = ""
	if allocs := testing.AllocsPerRun(100, func() { _ = plugin.GetRemoteIPs(single) }); allocs > 1 {
		t.Errorf("GetRemoteIPs with one hop: %.0f allocs, want at most 1 (no dedupe map)", allocs)
	}
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

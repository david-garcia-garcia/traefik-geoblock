package geoblock

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestIPHeaderStrategy_RightmostUntrusted proves that the client is the rightmost hop not in
// TrustedProxies: hops a client writes itself (left) never decide the country, hops of the trusted
// load balancer or CDN (right) are skipped.
func TestIPHeaderStrategy_RightmostUntrusted(t *testing.T) {
	cfg := &Config{
		Mode:                 ModeEnrichAndBlock,
		DatabaseSources:      seedCatalog(dbFilePath),
		AllowPrivate:         false,
		DefaultAllow:         true,
		BlockedCountries:     []string{"US"},
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-forwarded-for"},
		CountryHeader:        "x-country-code",
		IPHeaderStrategy:     IPHeaderStrategyRightmostUntrusted,
		TrustedProxies:       []string{"10.10.60.0/24", "212.227.172.0/24", "203.0.113.9"},
	}

	tests := []struct {
		name            string
		headerValue     string
		expectedStatus  int
		expectedCountry string
	}{
		{
			name:            "SpoofedLeftHopIsIgnored",
			headerValue:     "1.1.1.1, 8.8.8.8", // forged AU, real US client appended by the load balancer
			expectedStatus:  http.StatusForbidden,
			expectedCountry: "US",
		},
		{
			name:            "TrustedLoadBalancerHopIsSkipped",
			headerValue:     "8.8.8.8, 1.1.1.1, 10.10.60.12", // forged US, AU client, load balancer
			expectedStatus:  http.StatusTeapot,
			expectedCountry: "AU",
		},
		{
			name:            "CdnThenLoadBalancerPath",
			headerValue:     "8.8.8.8, 212.227.172.40", // US client behind a CDN edge
			expectedStatus:  http.StatusForbidden,
			expectedCountry: "US",
		},
		{
			name:            "SingleHostTrustedEntry",
			headerValue:     "1.1.1.1, 203.0.113.9",
			expectedStatus:  http.StatusTeapot,
			expectedCountry: "AU",
		},
	}

	plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-Forwarded-For", tt.headerValue)
			rr := httptest.NewRecorder()
			plugin.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
			if got := req.Header.Get("x-country-code"); got != tt.expectedCountry {
				t.Errorf("expected country %q, got %q", tt.expectedCountry, got)
			}
		})
	}
}

// TestIPHeaderStrategy_RightmostUntrusted_AllTrusted proves that a chain of trusted proxies only has
// no client hop: nothing is looked up and the request follows BanIfError.
func TestIPHeaderStrategy_RightmostUntrusted_AllTrusted(t *testing.T) {
	for _, banIfError := range []bool{false, true} {
		cfg := &Config{
			Mode:                 ModeEnrichAndBlock,
			DatabaseSources:      seedCatalog(dbFilePath),
			DefaultAllow:         true,
			BanIfError:           banIfError,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-forwarded-for"},
			CountryHeader:        "x-country-code",
			IPHeaderStrategy:     IPHeaderStrategyRightmostUntrusted,
			TrustedProxies:       []string{"10.0.0.0/8"},
		}
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
		if err != nil {
			t.Fatalf("Failed to create plugin: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "10.1.2.3, 10.10.60.12")
		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)

		want := http.StatusTeapot
		if banIfError {
			want = http.StatusForbidden
		}
		if rr.Code != want {
			t.Errorf("banIfError=%v: expected status %d, got %d", banIfError, want, rr.Code)
		}
	}
}

// TestIPHeaderStrategy_RightmostUntrusted_InvalidProxy proves that a wrong TrustedProxies entry
// stops the plugin at load instead of silently trusting nothing.
func TestIPHeaderStrategy_RightmostUntrusted_InvalidProxy(t *testing.T) {
	cfg := &Config{
		Mode:                 ModeEnrichAndBlock,
		DatabaseSources:      seedCatalog(dbFilePath),
		DefaultAllow:         true,
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-forwarded-for"},
		CountryHeader:        "x-country-code",
		IPHeaderStrategy:     IPHeaderStrategyRightmostUntrusted,
		TrustedProxies:       []string{"10.0.0.0/33"},
	}
	if _, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName); err == nil {
		t.Fatal("expected an error for an invalid TrustedProxies entry")
	}
}

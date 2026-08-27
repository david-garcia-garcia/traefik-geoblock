package traefik_geoblock

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/iplookup"
)

func TestLogHeader_ShouldSetDecisionOnRequest(t *testing.T) {
	// Track the log header values that were set on the request
	var capturedLogStatus string
	var capturedLogStatusDetail string
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedLogStatus = r.Header.Get("X-Geoblock-Status")
		capturedLogStatusDetail = r.Header.Get("X-Geoblock-Decision")
		w.WriteHeader(http.StatusTeapot)
	})

	cfg := &Config{
		Enabled:                     true,
		Ip2locationDatabaseFilePath: dbFilePath,
		AllowedCountries:            []string{"AU"},
		BlockedCountries:            []string{"US"},
		DefaultAllow:                false,
		AllowPrivate:                true,
		DisallowedStatusCode:        http.StatusForbidden,
		IPHeaders:                   []string{"x-forwarded-for"},
		IPHeaderStrategy:            IPHeaderStrategyCheckAll,
		CountryHeader:               "X-Country",
		LogStatusDetailHeader:       "X-Geoblock-Decision",
	}

	plugin, err := New(context.TODO(), captureHandler, cfg, pluginName)
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	tests := []struct {
		name              string
		ip                string
		expectedStatus    int
		expectedLogStatus string
		expectedLogDetail string
		description       string
	}{
		{
			name:              "Allowed_AU_IP_should_have_pass_header",
			ip:                "1.1.1.1", // AU IP
			expectedStatus:    http.StatusTeapot,
			expectedLogStatus: LogStatusPass,
			expectedLogDetail: LogStatusPass + ":" + PhaseAllowedCountry,
			description:       "AU IP should set headers to pass and pass:allowed_country",
		},
		{
			name:              "Blocked_US_IP_should_have_block_header",
			ip:                "8.8.8.8", // US IP
			expectedStatus:    http.StatusForbidden,
			expectedLogStatus: LogStatusBlock,
			expectedLogDetail: LogStatusBlock + ":" + PhaseBlockedCountry,
			description:       "US IP should set headers to block and block:blocked_country",
		},
		{
			name:              "Allowed_private_IP_should_have_pass_header",
			ip:                "192.168.1.1", // Private IP
			expectedStatus:    http.StatusTeapot,
			expectedLogStatus: LogStatusPass,
			expectedLogDetail: LogStatusPass + ":" + PhaseAllowPrivate,
			description:       "Private IP should set headers to pass and pass:allow_private",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capturedLogStatus = ""       // Reset
			capturedLogStatusDetail = "" // Reset

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-Forwarded-For", tt.ip)

			rr := httptest.NewRecorder()
			plugin.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// For allowed requests, check the headers on the forwarded request
			if tt.expectedStatus == http.StatusTeapot {
				if capturedLogStatus != "" {
					t.Errorf("expected no logStatusHeader, got '%s'", capturedLogStatus)
				}
				if capturedLogStatusDetail != tt.expectedLogDetail {
					t.Errorf("Expected logStatusDetailHeader '%s', got '%s'", tt.expectedLogDetail, capturedLogStatusDetail)
				}
			} else {
				if req.Header.Get("X-Geoblock-Status") != "" {
					t.Errorf("expected no logStatusHeader, got '%s'", req.Header.Get("X-Geoblock-Status"))
				}
				if req.Header.Get("X-Geoblock-Decision") != tt.expectedLogDetail {
					t.Errorf("Expected logStatusDetailHeader '%s', got '%s'", tt.expectedLogDetail, req.Header.Get("X-Geoblock-Decision"))
				}
			}

			t.Logf("SUCCESS: %s - IP: %s -> Status: %d, LogStatus: %s, LogDetail: %s",
				tt.description, tt.ip, rr.Code, tt.expectedLogStatus, tt.expectedLogDetail)
		})
	}
}

func TestLogHeader_SkipReasons(t *testing.T) {
	// Track the log header values that were set on the request
	var capturedLogStatus string
	var capturedLogStatusDetail string
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedLogStatus = r.Header.Get("X-Geoblock-Status")
		capturedLogStatusDetail = r.Header.Get("X-Geoblock-Decision")
		w.WriteHeader(http.StatusTeapot)
	})

	t.Run("IgnoreVerb_should_set_pass_ignore_verb", func(t *testing.T) {
		cfg := &Config{
			Enabled:                     true,
			Ip2locationDatabaseFilePath: dbFilePath,
			BlockedCountries:            []string{"US"},
			DefaultAllow:                false,
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"x-forwarded-for"},
			IPHeaderStrategy:            IPHeaderStrategyCheckAll,
			LogStatusDetailHeader:       "X-Geoblock-Decision",
			IgnoreVerbs:                 []string{"OPTIONS"},
		}

		plugin, err := New(context.TODO(), captureHandler, cfg, pluginName)
		if err != nil {
			t.Fatalf("Failed to create plugin: %v", err)
		}

		capturedLogStatus = ""
		capturedLogStatusDetail = ""
		req := httptest.NewRequest(http.MethodOptions, "/test", nil)
		req.Header.Set("X-Forwarded-For", "8.8.8.8") // US IP (normally blocked)

		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)

		if rr.Code != http.StatusTeapot {
			t.Errorf("Expected status %d, got %d", http.StatusTeapot, rr.Code)
		}
		if capturedLogStatus != "" {
			t.Errorf("expected no logStatusHeader, got '%s'", capturedLogStatus)
		}
		expectedDetail := LogStatusPass + ":" + PhaseIgnoreVerb
		if capturedLogStatusDetail != expectedDetail {
			t.Errorf("Expected logStatusDetailHeader '%s', got '%s'", expectedDetail, capturedLogStatusDetail)
		}
		t.Logf("SUCCESS: OPTIONS verb sets headers to %s and %s", LogStatusPass, expectedDetail)
	})

	t.Run("ExcludedRegex_should_set_pass_excluded_regex", func(t *testing.T) {
		cfg := &Config{
			Enabled:                     true,
			Ip2locationDatabaseFilePath: dbFilePath,
			BlockedCountries:            []string{"US"},
			DefaultAllow:                false,
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"x-forwarded-for"},
			IPHeaderStrategy:            IPHeaderStrategyCheckAll,
			LogStatusDetailHeader:       "X-Geoblock-Decision",
			ExcludedPathsRegex:          "^[^/]*/api/.*",
		}

		plugin, err := New(context.TODO(), captureHandler, cfg, pluginName)
		if err != nil {
			t.Fatalf("Failed to create plugin: %v", err)
		}

		capturedLogStatus = ""
		capturedLogStatusDetail = ""
		req := httptest.NewRequest(http.MethodGet, "http://example.com/api/users", nil)
		req.Host = "example.com"
		req.Header.Set("X-Forwarded-For", "8.8.8.8") // US IP (normally blocked)

		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)

		if rr.Code != http.StatusTeapot {
			t.Errorf("Expected status %d, got %d", http.StatusTeapot, rr.Code)
		}
		if capturedLogStatus != "" {
			t.Errorf("expected no logStatusHeader, got '%s'", capturedLogStatus)
		}
		expectedDetail := LogStatusPass + ":" + PhaseExcludedRegex
		if capturedLogStatusDetail != expectedDetail {
			t.Errorf("Expected logStatusDetailHeader '%s', got '%s'", expectedDetail, capturedLogStatusDetail)
		}
		t.Logf("SUCCESS: Excluded regex path sets headers to %s and %s", LogStatusPass, expectedDetail)
	})

	t.Run("NotIncludedRegex_should_set_pass_not_included_regex", func(t *testing.T) {
		handler, err := New(context.TODO(), captureHandler, &Config{
			Enabled:                     true,
			Ip2locationDatabaseFilePath: dbFilePath,
			BlockedCountries:            []string{"US"},
			DefaultAllow:                false,
			AllowPrivate:                false,
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"x-real-ip"},
			IPHeaderStrategy:            IPHeaderStrategyCheckAll,
			LogStatusDetailHeader:       "X-Geoblock-Decision",
			IncludedPathsRegex:          "^[^/]*/secure/.*",
		}, pluginName)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		capturedLogStatus = ""
		capturedLogStatusDetail = ""
		req := httptest.NewRequest(http.MethodGet, "/public", nil)
		req.Header.Set("X-Real-IP", "8.8.8.8")
		handler.ServeHTTP(httptest.NewRecorder(), req)
		expectedDetail := LogStatusPass + ":" + PhaseNotIncludedRegex
		if capturedLogStatusDetail != expectedDetail {
			t.Errorf("Expected logStatusDetailHeader '%s', got '%s'", expectedDetail, capturedLogStatusDetail)
		}
	})

	t.Run("BypassHeader_should_set_pass_bypass_header", func(t *testing.T) {
		cfg := &Config{
			Enabled:                     true,
			Ip2locationDatabaseFilePath: dbFilePath,
			BlockedCountries:            []string{"US"},
			DefaultAllow:                false,
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"x-forwarded-for"},
			IPHeaderStrategy:            IPHeaderStrategyCheckAll,
			LogStatusDetailHeader:       "X-Geoblock-Decision",
			BypassHeaders:               map[string]string{"X-Bypass": "secret123"},
		}

		plugin, err := New(context.TODO(), captureHandler, cfg, pluginName)
		if err != nil {
			t.Fatalf("Failed to create plugin: %v", err)
		}

		capturedLogStatus = ""
		capturedLogStatusDetail = ""
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "8.8.8.8") // US IP (normally blocked)
		req.Header.Set("X-Bypass", "secret123")

		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)

		if rr.Code != http.StatusTeapot {
			t.Errorf("Expected status %d, got %d", http.StatusTeapot, rr.Code)
		}
		if capturedLogStatus != "" {
			t.Errorf("expected no logStatusHeader, got '%s'", capturedLogStatus)
		}
		expectedDetail := LogStatusPass + ":" + PhaseBypassHeader
		if capturedLogStatusDetail != expectedDetail {
			t.Errorf("Expected logStatusDetailHeader '%s', got '%s'", expectedDetail, capturedLogStatusDetail)
		}
		t.Logf("SUCCESS: Bypass header sets headers to %s and %s", LogStatusPass, expectedDetail)
	})
}

func TestLogHeader_GeoRuleReasons(t *testing.T) {
	// Track the log header values that were set on the request
	var capturedLogStatus string
	var capturedLogStatusDetail string
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedLogStatus = r.Header.Get("X-Geoblock-Status")
		capturedLogStatusDetail = r.Header.Get("X-Geoblock-Decision")
		w.WriteHeader(http.StatusTeapot)
	})

	t.Run("DefaultAllow_true_should_set_pass_default_allow", func(t *testing.T) {
		cfg := &Config{
			Enabled:                     true,
			Ip2locationDatabaseFilePath: dbFilePath,
			AllowedCountries:            []string{}, // No countries explicitly allowed
			BlockedCountries:            []string{}, // No countries blocked
			DefaultAllow:                true,       // Allow by default
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"x-forwarded-for"},
			IPHeaderStrategy:            IPHeaderStrategyCheckAll,
			LogStatusDetailHeader:       "X-Geoblock-Decision",
		}

		plugin, err := New(context.TODO(), captureHandler, cfg, pluginName)
		if err != nil {
			t.Fatalf("Failed to create plugin: %v", err)
		}

		capturedLogStatus = ""
		capturedLogStatusDetail = ""
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "1.1.1.1") // AU IP - not in any list

		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)

		if rr.Code != http.StatusTeapot {
			t.Errorf("Expected status %d, got %d", http.StatusTeapot, rr.Code)
		}
		if capturedLogStatus != "" {
			t.Errorf("expected no logStatusHeader, got '%s'", capturedLogStatus)
		}
		expectedDetail := LogStatusPass + ":" + PhaseDefaultAllow
		if capturedLogStatusDetail != expectedDetail {
			t.Errorf("Expected logStatusDetailHeader '%s', got '%s'", expectedDetail, capturedLogStatusDetail)
		}
		t.Logf("SUCCESS: DefaultAllow=true sets headers to %s and %s", LogStatusPass, expectedDetail)
	})

	t.Run("DefaultAllow_false_should_set_block_default_allow", func(t *testing.T) {
		cfg := &Config{
			Enabled:                     true,
			Ip2locationDatabaseFilePath: dbFilePath,
			AllowedCountries:            []string{}, // No countries explicitly allowed
			BlockedCountries:            []string{}, // No countries blocked
			DefaultAllow:                false,      // Block by default
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"x-forwarded-for"},
			IPHeaderStrategy:            IPHeaderStrategyCheckAll,
			LogStatusDetailHeader:       "X-Geoblock-Decision",
		}

		plugin, err := New(context.TODO(), captureHandler, cfg, pluginName)
		if err != nil {
			t.Fatalf("Failed to create plugin: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "1.1.1.1") // AU IP - not in any list

		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("Expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
		expectedDetail := LogStatusBlock + ":" + PhaseDefaultAllow
		if req.Header.Get("X-Geoblock-Status") != "" {
			t.Errorf("expected no logStatusHeader, got '%s'", req.Header.Get("X-Geoblock-Status"))
		}
		if req.Header.Get("X-Geoblock-Decision") != expectedDetail {
			t.Errorf("Expected logStatusDetailHeader '%s', got '%s'", expectedDetail, req.Header.Get("X-Geoblock-Decision"))
		}
		t.Logf("SUCCESS: DefaultAllow=false sets headers to %s and %s", LogStatusBlock, expectedDetail)
	})

	t.Run("AllowedIPBlock_should_set_pass_allowed_ip_block", func(t *testing.T) {
		cfg := &Config{
			Enabled:                     true,
			Ip2locationDatabaseFilePath: dbFilePath,
			BlockedCountries:            []string{"US"},         // Block US
			AllowedIPBlocks:             []string{"8.8.8.0/24"}, // But allow this Google range
			DefaultAllow:                false,
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"x-forwarded-for"},
			IPHeaderStrategy:            IPHeaderStrategyCheckAll,
			LogStatusDetailHeader:       "X-Geoblock-Decision",
		}

		plugin, err := New(context.TODO(), captureHandler, cfg, pluginName)
		if err != nil {
			t.Fatalf("Failed to create plugin: %v", err)
		}

		capturedLogStatus = ""
		capturedLogStatusDetail = ""
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "8.8.8.8") // US IP but in allowed block

		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)

		if rr.Code != http.StatusTeapot {
			t.Errorf("Expected status %d, got %d", http.StatusTeapot, rr.Code)
		}
		if capturedLogStatus != "" {
			t.Errorf("expected no logStatusHeader, got '%s'", capturedLogStatus)
		}
		expectedDetail := LogStatusPass + ":" + PhaseAllowedIPBlock
		if capturedLogStatusDetail != expectedDetail {
			t.Errorf("Expected logStatusDetailHeader '%s', got '%s'", expectedDetail, capturedLogStatusDetail)
		}
		t.Logf("SUCCESS: AllowedIPBlock sets headers to %s and %s", LogStatusPass, expectedDetail)
	})

	t.Run("BlockedIPBlock_should_set_block_blocked_ip_block", func(t *testing.T) {
		cfg := &Config{
			Enabled:                     true,
			Ip2locationDatabaseFilePath: dbFilePath,
			AllowedCountries:            []string{"AU"},         // Allow AU
			BlockedIPBlocks:             []string{"1.1.1.0/24"}, // But block this AU range
			DefaultAllow:                true,
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"x-forwarded-for"},
			IPHeaderStrategy:            IPHeaderStrategyCheckAll,
			LogStatusDetailHeader:       "X-Geoblock-Decision",
		}

		plugin, err := New(context.TODO(), captureHandler, cfg, pluginName)
		if err != nil {
			t.Fatalf("Failed to create plugin: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "1.1.1.1") // AU IP but in blocked range

		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("Expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
		expectedDetail := LogStatusBlock + ":" + PhaseBlockedIPBlock
		if req.Header.Get("X-Geoblock-Status") != "" {
			t.Errorf("expected no logStatusHeader, got '%s'", req.Header.Get("X-Geoblock-Status"))
		}
		if req.Header.Get("X-Geoblock-Decision") != expectedDetail {
			t.Errorf("Expected logStatusDetailHeader '%s', got '%s'", expectedDetail, req.Header.Get("X-Geoblock-Decision"))
		}
		t.Logf("SUCCESS: BlockedIPBlock sets headers to %s and %s", LogStatusBlock, expectedDetail)
	})

	t.Run("NoIPsFound_should_set_pass_none", func(t *testing.T) {
		cfg := &Config{
			Enabled:                     true,
			Ip2locationDatabaseFilePath: dbFilePath,
			DefaultAllow:                true,
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"x-custom-ip"}, // Header that won't be set
			IPHeaderStrategy:            IPHeaderStrategyCheckAll,
			LogStatusDetailHeader:       "X-Geoblock-Decision",
		}

		plugin, err := New(context.TODO(), captureHandler, cfg, pluginName)
		if err != nil {
			t.Fatalf("Failed to create plugin: %v", err)
		}

		capturedLogStatus = ""
		capturedLogStatusDetail = ""
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		// Don't set any IP headers - no IPs to evaluate

		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)

		if rr.Code != http.StatusTeapot {
			t.Errorf("Expected status %d, got %d", http.StatusTeapot, rr.Code)
		}
		if capturedLogStatus != "" {
			t.Errorf("expected no logStatusHeader, got '%s'", capturedLogStatus)
		}
		expectedDetail := LogStatusPass + ":" + PhaseNone
		if capturedLogStatusDetail != expectedDetail {
			t.Errorf("Expected logStatusDetailHeader '%s', got '%s'", expectedDetail, capturedLogStatusDetail)
		}
		t.Logf("SUCCESS: No IPs found sets headers to %s and %s", LogStatusPass, expectedDetail)
	})
}

type stubGeoProvider struct {
	rec dbprovider.Record
	err error
}

func (s stubGeoProvider) Lookup(string) (dbprovider.Record, error) {
	return s.rec, s.err
}

func (s stubGeoProvider) Close() error { return nil }

func TestRequestHeaderEnrich(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	emptyBlocks, err := iplookup.NewIpLookupFileMonitor(nil, "", logger)
	if err != nil {
		t.Fatalf("cidr monitor: %v", err)
	}

	t.Run("writes country region city", func(t *testing.T) {
		plugin := &Plugin{
			next:    &noopHandler{},
			enabled: true,
			db: stubGeoProvider{rec: dbprovider.Record{
				Country: "US", Region: "California", City: "Mountain View",
				Isp: "Google LLC", Domain: "google.com", Asn: "15169",
			}},
			defaultAllow:     true,
			allowPrivate:     false,
			ipHeaders:        []string{"X-Real-Ip"},
			ipHeaderStrategy: IPHeaderStrategyCheckFirst,
			allowedIPBlocks:  emptyBlocks,
			blockedIPBlocks:  emptyBlocks,
			logger:           logger,
			requestHeaderEnrich: map[string]string{
				"X-Geo-Country": dbprovider.MetaCountry,
				"X-Geo-Region":  dbprovider.MetaRegion,
				"X-Geo-City":    dbprovider.MetaCity,
				"X-Geo-Isp":     dbprovider.MetaIsp,
				"X-Geo-Domain":  dbprovider.MetaDomain,
				"X-Geo-Asn":     dbprovider.MetaAsn,
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		req.Header.Set("X-Real-IP", "8.8.8.8")
		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)
		if rr.Code != http.StatusTeapot {
			t.Fatalf("status %d", rr.Code)
		}
		if got := req.Header.Get("X-Geo-Country"); got != "US" {
			t.Errorf("country: got %q", got)
		}
		if got := req.Header.Get("X-Geo-Region"); got != "California" {
			t.Errorf("region: got %q", got)
		}
		if got := req.Header.Get("X-Geo-City"); got != "Mountain View" {
			t.Errorf("city: got %q", got)
		}
		if got := req.Header.Get("X-Geo-Isp"); got != "Google LLC" {
			t.Errorf("isp: got %q", got)
		}
		if got := req.Header.Get("X-Geo-Domain"); got != "google.com" {
			t.Errorf("domain: got %q", got)
		}
		if got := req.Header.Get("X-Geo-Asn"); got != "15169" {
			t.Errorf("asn: got %q", got)
		}
	})

	t.Run("real BIN writes country and skips missing region city", func(t *testing.T) {
		handler, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:                     true,
			Ip2locationDatabaseFilePath: dbFilePath,
			DefaultAllow:                true,
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"x-real-ip"},
			IPHeaderStrategy:            IPHeaderStrategyCheckFirst,
			RequestHeaderEnrich: map[string]string{
				"X-Geo-Country": "country",
				"X-Geo-Region":  "region",
				"X-Geo-City":    "city",
			},
		}, pluginName)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		req.Header.Set("X-Real-IP", "8.8.8.8")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if req.Header.Get("X-Geo-Country") != "US" {
			t.Errorf("country: got %q", req.Header.Get("X-Geo-Country"))
		}
		if req.Header.Get("X-Geo-Region") != "" {
			t.Errorf("DB1 should not set region, got %q", req.Header.Get("X-Geo-Region"))
		}
		if req.Header.Get("X-Geo-City") != "" {
			t.Errorf("DB1 should not set city, got %q", req.Header.Get("X-Geo-City"))
		}
	})

	t.Run("DB8 writes region city isp domain", func(t *testing.T) {
		path := requireDB8(t)
		handler, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:                     true,
			Ip2locationDatabaseFilePath: path,
			DefaultAllow:                true,
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"x-real-ip"},
			IPHeaderStrategy:            IPHeaderStrategyCheckFirst,
			RequestHeaderEnrich:         fullEnrichHeaders,
		}, pluginName)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		req.Header.Set("X-Real-IP", "8.8.8.8")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if req.Header.Get("X-Geo-Country") != "US" {
			t.Errorf("country: got %q", req.Header.Get("X-Geo-Country"))
		}
		if req.Header.Get("X-Geo-Region") != "California" {
			t.Errorf("region: got %q", req.Header.Get("X-Geo-Region"))
		}
		if req.Header.Get("X-Geo-City") != "Mountain View" {
			t.Errorf("city: got %q", req.Header.Get("X-Geo-City"))
		}
		if req.Header.Get("X-Geo-Isp") != "Google LLC" {
			t.Errorf("isp: got %q", req.Header.Get("X-Geo-Isp"))
		}
		if req.Header.Get("X-Geo-Domain") != "google.com" {
			t.Errorf("domain: got %q", req.Header.Get("X-Geo-Domain"))
		}
		if req.Header.Get("X-Geo-Asn") != "" {
			t.Errorf("DB8 has no ASN column, got %q", req.Header.Get("X-Geo-Asn"))
		}
	})
}

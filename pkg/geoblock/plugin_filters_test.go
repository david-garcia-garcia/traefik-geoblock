package geoblock

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBypassHeaders_ShouldStillEnrichWithGeoIP(t *testing.T) {
	// Test that bypass headers skip blocking but still enrich with country information
	cfg := &Config{
		Mode: ModeEnrichAndBlock,
		DatabaseSources:      seedCatalog(dbFilePath),
		Ip2locationSourceGeo: "seed",
		AllowPrivate:         false,          // Block private IPs
		DefaultAllow:         false,          // Block by default
		BlockedCountries:     []string{"US"}, // Block US
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-forwarded-for"},
		IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		CountryHeader:        "x-country-code",
		BypassHeaders: map[string]string{
			"x-bypass-token": "secret123",
		},
	}

	plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	tests := []struct {
		name              string
		ip                string
		bypassToken       string
		expectedStatus    int
		expectedCountry   string
		shouldHaveCountry bool
		description       string
	}{
		{
			name:              "Bypass_US_IP_Should_Be_Enriched",
			ip:                "8.8.8.8",         // US IP (would normally be blocked)
			bypassToken:       "secret123",       // Valid bypass token
			expectedStatus:    http.StatusTeapot, // Should be allowed (noopHandler)
			expectedCountry:   "US",
			shouldHaveCountry: true,
			description:       "Bypassed US IP should still get country header enrichment",
		},
		{
			name:              "Bypass_AU_IP_Should_Be_Enriched",
			ip:                "1.1.1.1",         // AU IP (would be allowed anyway)
			bypassToken:       "secret123",       // Valid bypass token
			expectedStatus:    http.StatusTeapot, // Should be allowed
			expectedCountry:   "AU",
			shouldHaveCountry: true,
			description:       "Bypassed AU IP should still get country header enrichment",
		},
		{
			name:              "Bypass_Private_IP_Should_Be_Enriched",
			ip:                "192.168.1.1",     // Private IP (would normally be blocked)
			bypassToken:       "secret123",       // Valid bypass token
			expectedStatus:    http.StatusTeapot, // Should be allowed
			expectedCountry:   "PRIVATE",
			shouldHaveCountry: true,
			description:       "Bypassed private IP should still get PRIVATE country header",
		},
		{
			name:              "No_Bypass_US_IP_Should_Be_Blocked",
			ip:                "8.8.8.8",            // US IP (blocked)
			bypassToken:       "",                   // No bypass token
			expectedStatus:    http.StatusForbidden, // Should be blocked
			expectedCountry:   "US",
			shouldHaveCountry: false, // Blocked requests don't get forwarded, so header won't be visible
			description:       "Non-bypassed US IP should be blocked but still processed for country",
		},
		{
			name:              "Invalid_Bypass_US_IP_Should_Be_Blocked",
			ip:                "8.8.8.8",            // US IP (blocked)
			bypassToken:       "wrong-token",        // Invalid bypass token
			expectedStatus:    http.StatusForbidden, // Should be blocked
			expectedCountry:   "US",
			shouldHaveCountry: false, // Blocked requests don't get forwarded
			description:       "Invalid bypass token should not bypass blocking",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-Forwarded-For", tt.ip)

			if tt.bypassToken != "" {
				req.Header.Set("X-Bypass-Token", tt.bypassToken)
			}

			rr := httptest.NewRecorder()
			plugin.ServeHTTP(rr, req)

			// Check response status
			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Check country header enrichment
			countryHeader := req.Header.Get("x-country-code")

			if tt.shouldHaveCountry {
				if countryHeader == "" {
					t.Errorf("Expected country header to be set, but it was empty")
				} else if countryHeader != tt.expectedCountry {
					t.Errorf("Expected country header '%s', got '%s'", tt.expectedCountry, countryHeader)
				}
			}

			t.Logf("SUCCESS: %s - IP: %s, Bypass: %s -> Status: %d, Country: %s",
				tt.description, tt.ip, tt.bypassToken, rr.Code, countryHeader)
		})
	}
}

func TestIgnoreVerbs_ShouldSkipBlockingButStillEnrich(t *testing.T) {
	// Test that ignored HTTP verbs skip blocking but still get GeoIP enrichment
	cfg := &Config{
		Mode: ModeEnrichAndBlock,
		DatabaseSources:      seedCatalog(dbFilePath),
		Ip2locationSourceGeo: "seed",
		AllowPrivate:         false,
		DefaultAllow:         false,
		BlockedCountries:     []string{"US"}, // Block US
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-forwarded-for"},
		IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		CountryHeader:        "x-country-code",
		IgnoreVerbs:          []string{"OPTIONS", "HEAD"}, // Ignore these verbs
	}

	plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	tests := []struct {
		name            string
		method          string
		ip              string
		expectedStatus  int
		expectedCountry string
		description     string
	}{
		{
			name:            "OPTIONS_US_IP_Should_Be_Allowed_And_Enriched",
			method:          "OPTIONS",
			ip:              "8.8.8.8", // US IP (normally blocked)
			expectedStatus:  http.StatusTeapot,
			expectedCountry: "US",
			description:     "OPTIONS requests should skip blocking but still get country enrichment",
		},
		{
			name:            "HEAD_US_IP_Should_Be_Allowed_And_Enriched",
			method:          "HEAD",
			ip:              "8.8.8.8", // US IP (normally blocked)
			expectedStatus:  http.StatusTeapot,
			expectedCountry: "US",
			description:     "HEAD requests should skip blocking but still get country enrichment",
		},
		{
			name:            "GET_US_IP_Should_Be_Blocked",
			method:          "GET",
			ip:              "8.8.8.8", // US IP (blocked)
			expectedStatus:  http.StatusForbidden,
			expectedCountry: "US", // Still enriched but request blocked
			description:     "GET requests should still be blocked for blocked countries",
		},
		{
			name:            "POST_US_IP_Should_Be_Blocked",
			method:          "POST",
			ip:              "8.8.8.8", // US IP (blocked)
			expectedStatus:  http.StatusForbidden,
			expectedCountry: "US", // Still enriched but request blocked
			description:     "POST requests should still be blocked for blocked countries",
		},
		{
			name:            "OPTIONS_AU_IP_Should_Be_Allowed",
			method:          "OPTIONS",
			ip:              "1.1.1.1", // AU IP (normally allowed)
			expectedStatus:  http.StatusTeapot,
			expectedCountry: "AU",
			description:     "OPTIONS requests from allowed countries should work normally",
		},
		{
			name:            "OPTIONS_Private_IP_Should_Be_Allowed",
			method:          "OPTIONS",
			ip:              "192.168.1.1", // Private IP (normally blocked)
			expectedStatus:  http.StatusTeapot,
			expectedCountry: "PRIVATE",
			description:     "OPTIONS requests from private IPs should skip blocking",
		},
		{
			name:            "GET_Private_IP_Should_Be_Blocked",
			method:          "GET",
			ip:              "192.168.1.1", // Private IP (blocked)
			expectedStatus:  http.StatusForbidden,
			expectedCountry: "PRIVATE",
			description:     "GET requests from private IPs should still be blocked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/test", nil)
			req.Header.Set("X-Forwarded-For", tt.ip)

			rr := httptest.NewRecorder()
			plugin.ServeHTTP(rr, req)

			// Check response status
			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Check country header enrichment (should always be set)
			countryHeader := req.Header.Get("x-country-code")
			if countryHeader != tt.expectedCountry {
				t.Errorf("Expected country header '%s', got '%s'", tt.expectedCountry, countryHeader)
			}

			t.Logf("SUCCESS: %s - Method: %s, IP: %s -> Status: %d, Country: %s",
				tt.description, tt.method, tt.ip, rr.Code, countryHeader)
		})
	}
}

func TestExcludedPathsRegex_ShouldSkipBlockingButStillEnrich(t *testing.T) {
	cfg := &Config{
		Mode: ModeEnrichAndBlock,
		DatabaseSources:      seedCatalog(dbFilePath),
		Ip2locationSourceGeo: "seed",
		AllowedCountries:     []string{"AU"}, // Only AU allowed
		BlockedCountries:     []string{"US"}, // US blocked
		DefaultAllow:         false,          // Block by default
		AllowPrivate:         false,          // Block private IPs to test regex properly
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-forwarded-for"},
		IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		CountryHeader:        "x-country-code",
		// Regex matches against "{host}{path}" - httptest uses "example.com" as default host
		// Pattern matches: example.com/api/*, example.com/health, example.com/metrics
		ExcludedPathsRegex: "^[^/]*/(api/.*|health|metrics)$",
	}

	plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	tests := []struct {
		name            string
		path            string
		ip              string
		expectedStatus  int
		expectedCountry string
		description     string
	}{
		{
			name:            "Excluded_api_path_US_IP_Should_Be_Allowed",
			path:            "/api/users",
			ip:              "8.8.8.8", // US IP (normally blocked)
			expectedStatus:  http.StatusTeapot,
			expectedCountry: "US", // Still enriched
			description:     "US IP should be allowed on excluded /api/* path",
		},
		{
			name:            "Excluded_api_nested_path_US_IP_Should_Be_Allowed",
			path:            "/api/v1/users/123",
			ip:              "8.8.8.8", // US IP (normally blocked)
			expectedStatus:  http.StatusTeapot,
			expectedCountry: "US",
			description:     "US IP should be allowed on excluded nested /api/* path",
		},
		{
			name:            "Excluded_health_path_US_IP_Should_Be_Allowed",
			path:            "/health",
			ip:              "8.8.8.8", // US IP (normally blocked)
			expectedStatus:  http.StatusTeapot,
			expectedCountry: "US",
			description:     "US IP should be allowed on excluded /health path",
		},
		{
			name:            "Excluded_metrics_path_US_IP_Should_Be_Allowed",
			path:            "/metrics",
			ip:              "8.8.8.8", // US IP (normally blocked)
			expectedStatus:  http.StatusTeapot,
			expectedCountry: "US",
			description:     "US IP should be allowed on excluded /metrics path",
		},
		{
			name:            "NonExcluded_path_US_IP_Should_Be_Blocked",
			path:            "/other",
			ip:              "8.8.8.8", // US IP (blocked)
			expectedStatus:  http.StatusForbidden,
			expectedCountry: "US",
			description:     "US IP should be blocked on non-excluded path",
		},
		{
			name:            "Similar_but_not_matching_apiversion_Should_Be_Blocked",
			path:            "/apiversion",
			ip:              "8.8.8.8", // US IP (blocked)
			expectedStatus:  http.StatusForbidden,
			expectedCountry: "US",
			description:     "US IP should be blocked on /apiversion (doesn't match /api/*)",
		},
		{
			name:            "Similar_but_not_matching_healthcheck_Should_Be_Blocked",
			path:            "/healthcheck",
			ip:              "8.8.8.8", // US IP (blocked)
			expectedStatus:  http.StatusForbidden,
			expectedCountry: "US",
			description:     "US IP should be blocked on /healthcheck (doesn't match /health exactly)",
		},
		{
			name:            "Excluded_path_AU_IP_Should_Be_Allowed",
			path:            "/api/test",
			ip:              "1.1.1.1", // AU IP (allowed)
			expectedStatus:  http.StatusTeapot,
			expectedCountry: "AU",
			description:     "AU IP should be allowed on excluded path (would be allowed anyway)",
		},
		{
			name:            "NonExcluded_path_AU_IP_Should_Be_Allowed",
			path:            "/other",
			ip:              "1.1.1.1", // AU IP (allowed)
			expectedStatus:  http.StatusTeapot,
			expectedCountry: "AU",
			description:     "AU IP should be allowed on non-excluded path (allowed country)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("X-Forwarded-For", tt.ip)

			rr := httptest.NewRecorder()
			plugin.ServeHTTP(rr, req)

			// Check response status
			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Check country header enrichment (should always be set)
			countryHeader := req.Header.Get("x-country-code")
			if countryHeader != tt.expectedCountry {
				t.Errorf("Expected country header '%s', got '%s'", tt.expectedCountry, countryHeader)
			}

			t.Logf("SUCCESS: %s - Path: %s, IP: %s -> Status: %d, Country: %s",
				tt.description, tt.path, tt.ip, rr.Code, countryHeader)
		})
	}
}

func TestIncludedPathsRegex_OnlyMatchingPathsAreBlocked(t *testing.T) {
	cfg := &Config{
		Mode: ModeEnrichAndBlock,
		DatabaseSources:      seedCatalog(dbFilePath),
		Ip2locationSourceGeo: "seed",
		AllowedCountries:     []string{"AU"},
		BlockedCountries:     []string{"US"},
		DefaultAllow:         false,
		AllowPrivate:         false,
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-forwarded-for"},
		IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		CountryHeader:        "x-country-code",
		IncludedPathsRegex:   "^[^/]*/secure/.*",
	}

	plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	tests := []struct {
		name            string
		path            string
		ip              string
		expectedStatus  int
		expectedCountry string
	}{
		{name: "not_included_US_allowed", path: "/public", ip: "8.8.8.8", expectedStatus: http.StatusTeapot, expectedCountry: "US"},
		{name: "included_US_blocked", path: "/secure/page", ip: "8.8.8.8", expectedStatus: http.StatusForbidden, expectedCountry: "US"},
		{name: "included_AU_allowed", path: "/secure/page", ip: "1.1.1.1", expectedStatus: http.StatusTeapot, expectedCountry: "AU"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("X-Forwarded-For", tt.ip)
			rr := httptest.NewRecorder()
			plugin.ServeHTTP(rr, req)
			if rr.Code != tt.expectedStatus {
				t.Errorf("status: got %d want %d", rr.Code, tt.expectedStatus)
			}
			if got := req.Header.Get("x-country-code"); got != tt.expectedCountry {
				t.Errorf("country: got %q want %q", got, tt.expectedCountry)
			}
		})
	}
}

func TestIncludedPathsRegex_ExcludeStillWinsAfterInclude(t *testing.T) {
	cfg := &Config{
		Mode: ModeEnrichAndBlock,
		DatabaseSources:      seedCatalog(dbFilePath),
		Ip2locationSourceGeo: "seed",
		BlockedCountries:     []string{"US"},
		DefaultAllow:         false,
		AllowPrivate:         false,
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-forwarded-for"},
		IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		CountryHeader:        "x-country-code",
		IncludedPathsRegex:   "^[^/]*/secure/.*",
		ExcludedPathsRegex:   "^[^/]*/secure/health$",
	}

	plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	t.Run("included_and_excluded_US_allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/secure/health", nil)
		req.Header.Set("X-Forwarded-For", "8.8.8.8")
		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)
		if rr.Code != http.StatusTeapot {
			t.Errorf("exclude after include should skip block, got %d", rr.Code)
		}
		if got := req.Header.Get("x-country-code"); got != "US" {
			t.Errorf("country: got %q", got)
		}
	})

	t.Run("included_not_excluded_US_blocked", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/secure/page", nil)
		req.Header.Set("X-Forwarded-For", "8.8.8.8")
		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("included path should still block, got %d", rr.Code)
		}
	})
}

func TestIncludedPathsRegex_InvalidRegex(t *testing.T) {
	_, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
		Mode: ModeEnrichAndBlock,
		DatabaseSources:      seedCatalog(dbFilePath),
		Ip2locationSourceGeo: "seed",
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-forwarded-for"},
		IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		IncludedPathsRegex:   "[invalid(regex",
	}, pluginName)
	if err == nil {
		t.Fatal("expected error for invalid includedPathsRegex")
	}
	if !strings.Contains(err.Error(), "invalid includedPathsRegex pattern") {
		t.Errorf("got %v", err)
	}
}

func TestIncludedPathsRegex_EmptyRegex(t *testing.T) {
	plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
		Mode: ModeEnrichAndBlock,
		DatabaseSources:      seedCatalog(dbFilePath),
		Ip2locationSourceGeo: "seed",
		BlockedCountries:     []string{"US"},
		DefaultAllow:         false,
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-forwarded-for"},
		IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		IncludedPathsRegex:   "",
	}, pluginName)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/public", nil)
	req.Header.Set("X-Forwarded-For", "8.8.8.8")
	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("empty include should not skip blocking, got %d", rr.Code)
	}
}

func TestExcludedPathsRegex_InvalidRegex(t *testing.T) {
	cfg := &Config{
		Mode: ModeEnrichAndBlock,
		DatabaseSources:      seedCatalog(dbFilePath),
		Ip2locationSourceGeo: "seed",
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-forwarded-for"},
		IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		ExcludedPathsRegex:   "[invalid(regex", // Invalid regex pattern
	}

	_, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
	if err == nil {
		t.Error("Expected error for invalid regex, but got none")
	}
	if !strings.Contains(err.Error(), "invalid excludedPathsRegex pattern") {
		t.Errorf("Expected error message to mention invalid regex, got: %v", err)
	}
}

func TestExcludedPathsRegex_EmptyRegex(t *testing.T) {
	cfg := &Config{
		Mode: ModeEnrichAndBlock,
		DatabaseSources:      seedCatalog(dbFilePath),
		Ip2locationSourceGeo: "seed",
		BlockedCountries:     []string{"US"},
		DefaultAllow:         false,
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-forwarded-for"},
		IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		ExcludedPathsRegex:   "", // Empty regex - should not affect blocking
	}

	plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	// US IP should still be blocked when regex is empty
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("X-Forwarded-For", "8.8.8.8")

	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status %d with empty regex, got %d", http.StatusForbidden, rr.Code)
	}
}

func TestExcludedPathsRegex_DomainBasedMatching(t *testing.T) {
	// Test that regex can match specific domains
	// Pattern: only exclude paths on "api.example.com" domain
	cfg := &Config{
		Mode: ModeEnrichAndBlock,
		DatabaseSources:      seedCatalog(dbFilePath),
		Ip2locationSourceGeo: "seed",
		BlockedCountries:     []string{"US"},
		DefaultAllow:         false,
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-forwarded-for"},
		IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		// Only exclude /api/* on api.example.com, not on other domains
		ExcludedPathsRegex: "^api\\.example\\.com/api/.*",
	}

	plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	tests := []struct {
		name           string
		host           string
		path           string
		expectedStatus int
		description    string
	}{
		{
			name:           "Matching_domain_should_be_excluded",
			host:           "api.example.com",
			path:           "/api/users",
			expectedStatus: http.StatusTeapot, // Excluded, passes through
			description:    "api.example.com/api/* should be excluded",
		},
		{
			name:           "Different_domain_should_be_blocked",
			host:           "www.example.com",
			path:           "/api/users",
			expectedStatus: http.StatusForbidden, // Not excluded, blocked
			description:    "www.example.com/api/* should NOT be excluded",
		},
		{
			name:           "Default_host_should_be_blocked",
			host:           "example.com",
			path:           "/api/users",
			expectedStatus: http.StatusForbidden, // Not excluded, blocked
			description:    "example.com/api/* should NOT be excluded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://"+tt.host+tt.path, nil)
			req.Host = tt.host                           // Explicitly set Host header
			req.Header.Set("X-Forwarded-For", "8.8.8.8") // US IP (blocked)

			rr := httptest.NewRecorder()
			plugin.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("%s: Expected status %d, got %d", tt.description, tt.expectedStatus, rr.Code)
			}
			t.Logf("SUCCESS: %s - Host: %s, Path: %s -> Status: %d", tt.description, tt.host, tt.path, rr.Code)
		})
	}
}

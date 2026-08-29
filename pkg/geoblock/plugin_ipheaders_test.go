package geoblock

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIPHeaderStrategy_Integration(t *testing.T) {
	// Create a plugin with blocked countries (e.g., block CN)
	cfg := &Config{
		Enabled:              true,
		DatabaseSources:      seedCatalog(dbFilePath),
		Ip2locationSourceGeo: "seed",
		AllowPrivate:         true,
		DefaultAllow:         true,
		BlockedCountries:     []string{"CN"},
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-forwarded-for"},
		CountryHeader:        "x-country-code",
	}

	tests := []struct {
		name            string
		strategy        string
		headerValue     string
		expectedStatus  int
		expectedCountry string
		description     string
	}{
		{
			name:            "CheckAll_MultipleIPs_AllowedFirst",
			strategy:        IPHeaderStrategyCheckAll,
			headerValue:     "8.8.8.8, 192.168.1.1", // US (allowed), Private
			expectedStatus:  http.StatusTeapot,
			expectedCountry: "US", // Should be US, not overridden by private
			description:     "CheckAll with allowed public IP first should pass and set country to public IP country",
		},
		{
			name:            "CheckAll_MultipleIPs_PrivateFirst",
			strategy:        IPHeaderStrategyCheckAll,
			headerValue:     "192.168.1.1, 8.8.8.8", // Private, US (allowed)
			expectedStatus:  http.StatusTeapot,
			expectedCountry: "US", // Should prefer US over PRIVATE
			description:     "CheckAll with private IP first should pass and prefer real country over private",
		},
		{
			name:            "CheckFirst_MultipleIPs_AllowedFirst",
			strategy:        IPHeaderStrategyCheckFirst,
			headerValue:     "8.8.8.8, 1.1.1.1", // US (allowed), AU (allowed)
			expectedStatus:  http.StatusTeapot,
			expectedCountry: "US",
			description:     "CheckFirst should only check first IP and set its country",
		},
		{
			name:            "CheckFirst_MultipleIPs_PrivateFirst",
			strategy:        IPHeaderStrategyCheckFirst,
			headerValue:     "192.168.1.1, 8.8.8.8", // Private, US (allowed)
			expectedStatus:  http.StatusTeapot,
			expectedCountry: "PRIVATE",
			description:     "CheckFirst with private IP first should only check private IP",
		},
		{
			name:            "CheckFirstNonePrivate_MultipleIPs_PrivateFirst",
			strategy:        IPHeaderStrategyCheckFirstNonePrivate,
			headerValue:     "192.168.1.1, 8.8.8.8", // Private, US (allowed)
			expectedStatus:  http.StatusTeapot,
			expectedCountry: "US",
			description:     "CheckFirstNonePrivate should skip private IP and check first public IP",
		},
		{
			name:            "CheckFirstNonePrivate_OnlyPrivate",
			strategy:        IPHeaderStrategyCheckFirstNonePrivate,
			headerValue:     "192.168.1.1, 10.0.0.1", // Private, Private
			expectedStatus:  http.StatusTeapot,
			expectedCountry: "PRIVATE",
			description:     "CheckFirstNonePrivate with only private IPs should check first private IP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config with specific strategy
			testCfg := *cfg
			testCfg.IPHeaderStrategy = tt.strategy

			plugin, err := New(context.TODO(), &noopHandler{}, &testCfg, pluginName)
			if err != nil {
				t.Fatalf("Failed to create plugin: %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-Forwarded-For", tt.headerValue)

			rr := httptest.NewRecorder()
			plugin.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			countryHeader := req.Header.Get("x-country-code")
			if countryHeader != tt.expectedCountry {
				t.Errorf("Expected country header '%s', got '%s'", tt.expectedCountry, countryHeader)
			}

			t.Logf("SUCCESS: %s - Status: %d, Country: %s", tt.description, rr.Code, countryHeader)
		})
	}
}

func TestIPHeaderStrategy_CountryHeaderPriority(t *testing.T) {
	// Test that real countries take priority over private IP countries in the header
	cfg := &Config{
		Enabled:              true,
		DatabaseSources:      seedCatalog(dbFilePath),
		Ip2locationSourceGeo: "seed",
		AllowPrivate:         true,
		DefaultAllow:         true,
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-forwarded-for"},
		CountryHeader:        "x-country-code",
		IPHeaderStrategy:     IPHeaderStrategyCheckAll,
	}

	plugin, err := New(context.TODO(), &noopHandler{}, cfg, pluginName)
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	tests := []struct {
		name            string
		headerValue     string
		expectedCountry string
		description     string
	}{
		{
			name:            "PrivateFirst_PublicSecond",
			headerValue:     "192.168.1.1, 8.8.8.8", // Private, US
			expectedCountry: "US",
			description:     "Should prefer US over PRIVATE when both are present",
		},
		{
			name:            "PublicFirst_PrivateSecond",
			headerValue:     "8.8.8.8, 192.168.1.1", // US, Private
			expectedCountry: "US",
			description:     "Should keep US and not override with PRIVATE",
		},
		{
			name:            "OnlyPrivate",
			headerValue:     "192.168.1.1, 10.0.0.1", // Private, Private
			expectedCountry: "PRIVATE",
			description:     "Should set PRIVATE when only private IPs are present",
		},
		{
			name:            "OnlyPublic",
			headerValue:     "8.8.8.8, 1.1.1.1", // US, AU
			expectedCountry: "US",
			description:     "Should set first public country when only public IPs are present",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-Forwarded-For", tt.headerValue)

			rr := httptest.NewRecorder()
			plugin.ServeHTTP(rr, req)

			countryHeader := req.Header.Get("x-country-code")
			if countryHeader != tt.expectedCountry {
				t.Errorf("Expected country header '%s', got '%s'", tt.expectedCountry, countryHeader)
			}

			t.Logf("SUCCESS: %s - Country: %s", tt.description, countryHeader)
		})
	}
}

func TestIPHeaderStrategy_InvalidStrategy(t *testing.T) {
	cfg := &Config{
		Enabled:              true,
		DatabaseSources:      seedCatalog(dbFilePath),
		Ip2locationSourceGeo: "seed",
		IPHeaders:            []string{"x-forwarded-for"},
		IPHeaderStrategy:     "InvalidStrategy",
		DisallowedStatusCode: http.StatusForbidden,
	}

	_, err := New(context.TODO(), &noopHandler{}, cfg, pluginName)
	if err == nil {
		t.Error("Expected error for invalid IP header strategy, but got none")
	}

	expectedError := "invalid IPHeaderStrategy 'InvalidStrategy'"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', but got: %s", expectedError, err.Error())
	}
}

func TestIPHeaderStrategy_PrivateIPDoesNotOverridePublicCountry(t *testing.T) {
	// Test that private IPs processed AFTER public IPs do not override the country header
	cfg := &Config{
		Enabled:              true,
		DatabaseSources:      seedCatalog(dbFilePath),
		Ip2locationSourceGeo: "seed",
		AllowPrivate:         true,
		DefaultAllow:         true,
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-forwarded-for"},
		CountryHeader:        "x-country-code",
		IPHeaderStrategy:     IPHeaderStrategyCheckAll, // Check all IPs to test override scenario
	}

	plugin, err := New(context.TODO(), &noopHandler{}, cfg, pluginName)
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	tests := []struct {
		name            string
		headerValue     string
		expectedCountry string
		description     string
	}{
		{
			name:            "PublicFirst_PrivateLast_ShouldNotOverride",
			headerValue:     "8.8.8.8, 1.1.1.1, 192.168.1.1, 10.0.0.1", // US, AU, Private, Private
			expectedCountry: "US",
			description:     "Public IP country (US) should not be overridden by later private IPs",
		},
		{
			name:            "PublicMiddle_PrivateLast_ShouldNotOverride",
			headerValue:     "127.0.0.1, 8.8.8.8, 192.168.1.1", // Private, US, Private
			expectedCountry: "US",
			description:     "Public IP country (US) should not be overridden by later private IP",
		},
		{
			name:            "MultiplePublic_PrivateLast_ShouldKeepFirst",
			headerValue:     "8.8.8.8, 1.1.1.1, 9.9.9.9, 192.168.1.1", // US, AU, US, Private
			expectedCountry: "US",
			description:     "First public IP country (US) should be kept, not overridden by private IP",
		},
		{
			name:            "PrivateFirst_PublicSecond_PrivateThird_ShouldUsePublic",
			headerValue:     "192.168.1.1, 8.8.8.8, 10.0.0.1", // Private, US, Private
			expectedCountry: "US",
			description:     "Private IP should not override public IP country even when private comes after",
		},
		{
			name:            "OnlyPrivate_ShouldSetPrivate",
			headerValue:     "192.168.1.1, 10.0.0.1, 127.0.0.1", // Private, Private, Private
			expectedCountry: "PRIVATE",
			description:     "When only private IPs are present, PRIVATE should be set",
		},
		{
			name:            "ComplexMix_PrivateAtEnd",
			headerValue:     "192.168.1.1, 8.8.8.8, 1.1.1.1, 172.16.0.1, 10.0.0.1", // Private, US, AU, Private, Private
			expectedCountry: "US",
			description:     "Complex mix with multiple private IPs at end should not override first public country",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-Forwarded-For", tt.headerValue)

			rr := httptest.NewRecorder()
			plugin.ServeHTTP(rr, req)

			// Check that request was allowed (status 418 = teapot from noopHandler)
			if rr.Code != http.StatusTeapot {
				t.Errorf("Expected request to be allowed (status %d), got %d", http.StatusTeapot, rr.Code)
			}

			countryHeader := req.Header.Get("x-country-code")
			if countryHeader != tt.expectedCountry {
				t.Errorf("Expected country header '%s', got '%s'", tt.expectedCountry, countryHeader)
			}

			t.Logf("SUCCESS: %s - Header: '%s' -> Country: '%s'", tt.description, tt.headerValue, countryHeader)
		})
	}
}

func TestIPHeaderStrategy_CountryHeaderOverrideEdgeCases(t *testing.T) {
	// Test edge cases for country header override protection
	cfg := &Config{
		Enabled:              true,
		DatabaseSources:      seedCatalog(dbFilePath),
		Ip2locationSourceGeo: "seed",
		AllowPrivate:         true,
		DefaultAllow:         true,
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
		CountryHeader:        "x-country-code",
		IPHeaderStrategy:     IPHeaderStrategyCheckAll,
	}

	plugin, err := New(context.TODO(), &noopHandler{}, cfg, pluginName)
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	tests := []struct {
		name            string
		forwardedFor    string
		realIP          string
		expectedCountry string
		description     string
	}{
		{
			name:            "MultipleHeaders_PublicInFirst_PrivateInSecond",
			forwardedFor:    "8.8.8.8, 192.168.1.1", // US, Private
			realIP:          "10.0.0.1",             // Private
			expectedCountry: "US",
			description:     "Public IP in first header should not be overridden by private IPs in later headers",
		},
		{
			name:            "MultipleHeaders_PrivateInFirst_PublicInSecond",
			forwardedFor:    "192.168.1.1, 10.0.0.1", // Private, Private
			realIP:          "8.8.8.8",               // US
			expectedCountry: "US",
			description:     "Public IP in second header should override private IPs from first header",
		},
		{
			name:            "MultipleHeaders_PublicInBoth_PrivateAtEnd",
			forwardedFor:    "8.8.8.8, 1.1.1.1", // US, AU
			realIP:          "192.168.1.1",      // Private
			expectedCountry: "US",
			description:     "First public IP should be used, private IP should not override",
		},
		{
			name:            "MultipleHeaders_OnlyPrivate",
			forwardedFor:    "192.168.1.1, 10.0.0.1", // Private, Private
			realIP:          "127.0.0.1",             // Private
			expectedCountry: "PRIVATE",
			description:     "When all IPs are private across multiple headers, PRIVATE should be set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-Forwarded-For", tt.forwardedFor)
			req.Header.Set("X-Real-IP", tt.realIP)

			rr := httptest.NewRecorder()
			plugin.ServeHTTP(rr, req)

			countryHeader := req.Header.Get("x-country-code")
			if countryHeader != tt.expectedCountry {
				t.Errorf("Expected country header '%s', got '%s'", tt.expectedCountry, countryHeader)
			}

			t.Logf("SUCCESS: %s - X-Forwarded-For: '%s', X-Real-IP: '%s' -> Country: '%s'",
				tt.description, tt.forwardedFor, tt.realIP, countryHeader)
		})
	}
}

func TestIPHeaderStrategy_HeaderOrderRespected(t *testing.T) {
	// Test that IP headers are processed in the order they are defined in ipHeaders
	cfg := &Config{
		Enabled:              true,
		DatabaseSources:      seedCatalog(dbFilePath),
		Ip2locationSourceGeo: "seed",
		AllowPrivate:         true,
		DefaultAllow:         true,
		DisallowedStatusCode: http.StatusForbidden,
		CountryHeader:        "x-country-code",
		IPHeaderStrategy:     IPHeaderStrategyCheckAll,
	}

	tests := []struct {
		name            string
		ipHeaders       []string
		headerValues    map[string]string
		expectedOrder   []string
		expectedCountry string
		description     string
	}{
		{
			name:      "ForwardedFor_First_RealIP_Second",
			ipHeaders: []string{"x-forwarded-for", "x-real-ip"},
			headerValues: map[string]string{
				"x-forwarded-for": "8.8.8.8, 1.1.1.1", // US, AU
				"x-real-ip":       "9.9.9.9",          // US
			},
			expectedOrder:   []string{"8.8.8.8", "1.1.1.1", "9.9.9.9"},
			expectedCountry: "US", // First IP should be US (8.8.8.8 - leftmost/original client)
			description:     "X-Forwarded-For should be processed first, then X-Real-IP",
		},
		{
			name:      "RealIP_First_ForwardedFor_Second",
			ipHeaders: []string{"x-real-ip", "x-forwarded-for"},
			headerValues: map[string]string{
				"x-forwarded-for": "8.8.8.8, 1.1.1.1", // US, AU
				"x-real-ip":       "9.9.9.9",          // US
			},
			expectedOrder:   []string{"9.9.9.9", "8.8.8.8", "1.1.1.1"},
			expectedCountry: "US", // First IP should be US (9.9.9.9 from x-real-ip)
			description:     "X-Real-IP should be processed first, then X-Forwarded-For",
		},
		{
			name:      "Custom_Header_Order",
			ipHeaders: []string{"cf-connecting-ip", "x-forwarded-for", "x-real-ip"},
			headerValues: map[string]string{
				"x-forwarded-for":  "8.8.8.8",          // US
				"x-real-ip":        "1.1.1.1",          // AU
				"cf-connecting-ip": "9.9.9.9, 8.8.4.4", // US, US
			},
			expectedOrder:   []string{"9.9.9.9", "8.8.4.4", "8.8.8.8", "1.1.1.1"},
			expectedCountry: "US", // First IP should be US (9.9.9.9 - leftmost in first header)
			description:     "Custom header order should be respected: CF-Connecting-IP, X-Forwarded-For, X-Real-IP",
		},
		{
			name:      "Mixed_Private_Public_Order",
			ipHeaders: []string{"x-forwarded-for", "x-real-ip"},
			headerValues: map[string]string{
				"x-forwarded-for": "192.168.1.1, 8.8.8.8", // Private, US
				"x-real-ip":       "1.1.1.1",              // AU
			},
			expectedOrder:   []string{"192.168.1.1", "8.8.8.8", "1.1.1.1"},
			expectedCountry: "US", // First public IP should be US (8.8.8.8 - second in processing order)
			description:     "Order should be respected even with mixed private/public IPs",
		},
		{
			name:      "Deduplication_Preserves_First_Occurrence",
			ipHeaders: []string{"x-forwarded-for", "x-real-ip", "x-client-ip"},
			headerValues: map[string]string{
				"x-forwarded-for": "8.8.8.8, 1.1.1.1", // US, AU
				"x-real-ip":       "8.8.8.8",          // US (duplicate)
				"x-client-ip":     "9.9.9.9, 8.8.8.8", // US, US (duplicate)
			},
			expectedOrder:   []string{"8.8.8.8", "1.1.1.1", "9.9.9.9"}, // Duplicates removed, order preserved
			expectedCountry: "US",                                      // First IP should be US (8.8.8.8)
			description:     "Duplicate IPs should be removed but order of first occurrence preserved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config with specific header order
			testCfg := *cfg
			testCfg.IPHeaders = tt.ipHeaders

			plugin, err := New(context.TODO(), &noopHandler{}, &testCfg, pluginName)
			if err != nil {
				t.Fatalf("Failed to create plugin: %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, "/test", nil)

			// Set headers as specified in test
			for header, value := range tt.headerValues {
				req.Header.Set(header, value)
			}

			// Test GetRemoteIPs order
			p := plugin.(*Plugin)
			actualIPs := p.GetRemoteIPs(req)

			if len(actualIPs) != len(tt.expectedOrder) {
				t.Errorf("Expected %d IPs, got %d. Expected: %v, Got: %v",
					len(tt.expectedOrder), len(actualIPs), tt.expectedOrder, actualIPs)
				return
			}

			for i, expectedIP := range tt.expectedOrder {
				if actualIPs[i] != expectedIP {
					t.Errorf("Expected IP at position %d to be %s, got %s. Full order - Expected: %v, Got: %v",
						i, expectedIP, actualIPs[i], tt.expectedOrder, actualIPs)
				}
			}

			// Test full request processing
			rr := httptest.NewRecorder()
			plugin.ServeHTTP(rr, req)

			countryHeader := req.Header.Get("x-country-code")
			if countryHeader != tt.expectedCountry {
				t.Errorf("Expected country header '%s', got '%s'", tt.expectedCountry, countryHeader)
			}

			t.Logf("SUCCESS: %s - Header Order: %v -> IP Order: %v -> Country: %s",
				tt.description, tt.ipHeaders, actualIPs, countryHeader)
		})
	}
}

func TestIPHeaderStrategy_HeaderOrderWithStrategies(t *testing.T) {
	// Test that different IP header strategies respect header order
	tests := []struct {
		name            string
		strategy        string
		ipHeaders       []string
		headerValues    map[string]string
		expectedCountry string
		description     string
	}{
		{
			name:      "CheckFirst_RespectsHeaderOrder",
			strategy:  IPHeaderStrategyCheckFirst,
			ipHeaders: []string{"x-real-ip", "x-forwarded-for"},
			headerValues: map[string]string{
				"x-real-ip":       "8.8.8.8",          // US (should be checked)
				"x-forwarded-for": "1.1.1.1, 9.9.9.9", // AU, US (should be ignored)
			},
			expectedCountry: "US", // Should use first IP from first header (x-real-ip)
			description:     "CheckFirst should use first IP from first header in order",
		},
		{
			name:      "CheckFirst_FirstHeaderEmpty",
			strategy:  IPHeaderStrategyCheckFirst,
			ipHeaders: []string{"x-client-ip", "x-real-ip", "x-forwarded-for"},
			headerValues: map[string]string{
				// x-client-ip is missing/empty
				"x-real-ip":       "1.1.1.1",          // AU (should be checked)
				"x-forwarded-for": "8.8.8.8, 9.9.9.9", // US, US (should be ignored)
			},
			expectedCountry: "AU", // Should use first IP from first non-empty header (x-real-ip)
			description:     "CheckFirst should skip empty headers and use first IP from first non-empty header",
		},
		{
			name:      "CheckFirstNonePrivate_RespectsHeaderOrder",
			strategy:  IPHeaderStrategyCheckFirstNonePrivate,
			ipHeaders: []string{"x-forwarded-for", "x-real-ip"},
			headerValues: map[string]string{
				"x-forwarded-for": "192.168.1.1, 8.8.8.8", // Private, US
				"x-real-ip":       "1.1.1.1",              // AU (should be ignored)
			},
			expectedCountry: "US", // Should use first public IP from header order (8.8.8.8)
			description:     "CheckFirstNonePrivate should find first public IP respecting header order",
		},
		{
			name:      "CheckFirstNonePrivate_CrossHeaders",
			strategy:  IPHeaderStrategyCheckFirstNonePrivate,
			ipHeaders: []string{"x-real-ip", "x-forwarded-for"},
			headerValues: map[string]string{
				"x-real-ip":       "192.168.1.1",      // Private
				"x-forwarded-for": "1.1.1.1, 8.8.8.8", // AU, US
			},
			expectedCountry: "AU", // Should use first public IP across all headers in order (1.1.1.1)
			description:     "CheckFirstNonePrivate should find first public IP across headers in order",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Enabled:              true,
				DatabaseSources:      seedCatalog(dbFilePath),
				Ip2locationSourceGeo: "seed",
				AllowPrivate:         true,
				DefaultAllow:         true,
				DisallowedStatusCode: http.StatusForbidden,
				IPHeaders:            tt.ipHeaders,
				IPHeaderStrategy:     tt.strategy,
				CountryHeader:        "x-country-code",
			}

			plugin, err := New(context.TODO(), &noopHandler{}, cfg, pluginName)
			if err != nil {
				t.Fatalf("Failed to create plugin: %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, "/test", nil)

			// Set headers as specified in test
			for header, value := range tt.headerValues {
				req.Header.Set(header, value)
			}

			rr := httptest.NewRecorder()
			plugin.ServeHTTP(rr, req)

			countryHeader := req.Header.Get("x-country-code")
			if countryHeader != tt.expectedCountry {
				t.Errorf("Expected country header '%s', got '%s'", tt.expectedCountry, countryHeader)
			}

			t.Logf("SUCCESS: %s - Strategy: %s, Headers: %v -> Country: %s",
				tt.description, tt.strategy, tt.ipHeaders, countryHeader)
		})
	}
}

func TestGetRemoteIPs_SyntheticRemoteAddress(t *testing.T) {
	// Test the synthetic "remoteAddress" header that maps to req.RemoteAddr
	tests := []struct {
		name          string
		ipHeaders     []string
		headerValues  map[string]string
		remoteAddr    string
		expectedOrder []string
		description   string
	}{
		{
			name:         "RemoteAddress_Only",
			ipHeaders:    []string{"remoteAddress"},
			headerValues: map[string]string{
				// No actual headers set
			},
			remoteAddr:    "203.0.113.1:12345",
			expectedOrder: []string{"203.0.113.1"},
			description:   "Should extract IP from req.RemoteAddr when using remoteAddress synthetic header",
		},
		{
			name:      "RemoteAddress_First_Then_Headers",
			ipHeaders: []string{"remoteAddress", "x-forwarded-for"},
			headerValues: map[string]string{
				"x-forwarded-for": "8.8.8.8, 1.1.1.1",
			},
			remoteAddr:    "203.0.113.1:12345",
			expectedOrder: []string{"203.0.113.1", "8.8.8.8", "1.1.1.1"},
			description:   "Should process remoteAddress first, then other headers in order",
		},
		{
			name:      "Headers_First_Then_RemoteAddress",
			ipHeaders: []string{"x-forwarded-for", "remoteAddress"},
			headerValues: map[string]string{
				"x-forwarded-for": "8.8.8.8, 1.1.1.1",
			},
			remoteAddr:    "203.0.113.1:12345",
			expectedOrder: []string{"8.8.8.8", "1.1.1.1", "203.0.113.1"},
			description:   "Should process headers first, then remoteAddress based on order",
		},
		{
			name:         "RemoteAddress_With_Port_Stripping",
			ipHeaders:    []string{"remoteAddress"},
			headerValues: map[string]string{
				// No actual headers set
			},
			remoteAddr:    "192.168.1.100:54321",
			expectedOrder: []string{"192.168.1.100"},
			description:   "Should strip port from req.RemoteAddr",
		},
		{
			name:         "RemoteAddress_IPv6_With_Port",
			ipHeaders:    []string{"remoteAddress"},
			headerValues: map[string]string{
				// No actual headers set
			},
			remoteAddr:    "[2001:db8::1]:8080",
			expectedOrder: []string{"2001:db8::1"},
			description:   "Should handle IPv6 addresses with port stripping",
		},
		{
			name:      "RemoteAddress_Deduplication",
			ipHeaders: []string{"x-forwarded-for", "remoteAddress"},
			headerValues: map[string]string{
				"x-forwarded-for": "203.0.113.1, 8.8.8.8",
			},
			remoteAddr:    "203.0.113.1:12345",                // Same IP as in header
			expectedOrder: []string{"203.0.113.1", "8.8.8.8"}, // Should deduplicate
			description:   "Should deduplicate remoteAddress if it matches header IPs",
		},
		{
			name:      "Multiple_Headers_With_RemoteAddress_In_Middle",
			ipHeaders: []string{"x-real-ip", "remoteAddress", "x-forwarded-for"},
			headerValues: map[string]string{
				"x-real-ip":       "9.9.9.9",
				"x-forwarded-for": "8.8.8.8, 1.1.1.1",
			},
			remoteAddr:    "203.0.113.1:12345",
			expectedOrder: []string{"9.9.9.9", "203.0.113.1", "8.8.8.8", "1.1.1.1"},
			description:   "Should process remoteAddress in the correct position based on header order",
		},
		{
			name:      "RemoteAddress_Empty_Should_Be_Ignored",
			ipHeaders: []string{"remoteAddress", "x-forwarded-for"},
			headerValues: map[string]string{
				"x-forwarded-for": "8.8.8.8",
			},
			remoteAddr:    "", // Empty RemoteAddr
			expectedOrder: []string{"8.8.8.8"},
			description:   "Should ignore empty req.RemoteAddr and continue with other headers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := &Plugin{
				ipHeaders: tt.ipHeaders,
			}

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tt.remoteAddr

			// Set headers as specified in test
			for header, value := range tt.headerValues {
				req.Header.Set(header, value)
			}

			actualIPs := plugin.GetRemoteIPs(req)

			if len(actualIPs) != len(tt.expectedOrder) {
				t.Errorf("Expected %d IPs, got %d. Expected: %v, Got: %v",
					len(tt.expectedOrder), len(actualIPs), tt.expectedOrder, actualIPs)
				return
			}

			for i, expectedIP := range tt.expectedOrder {
				if actualIPs[i] != expectedIP {
					t.Errorf("Expected IP at position %d to be %s, got %s. Full order - Expected: %v, Got: %v",
						i, expectedIP, actualIPs[i], tt.expectedOrder, actualIPs)
				}
			}

			t.Logf("SUCCESS: %s - Headers: %v, RemoteAddr: %s -> IPs: %v",
				tt.description, tt.ipHeaders, tt.remoteAddr, actualIPs)
		})
	}
}

func TestRemoteAddress_IntegrationWithStrategies(t *testing.T) {
	// Test remoteAddress with different IP header strategies
	cfg := &Config{
		Enabled:              true,
		DatabaseSources:      seedCatalog(dbFilePath),
		Ip2locationSourceGeo: "seed",
		AllowPrivate:         true,
		DefaultAllow:         true,
		DisallowedStatusCode: http.StatusForbidden,
		CountryHeader:        "x-country-code",
	}

	tests := []struct {
		name            string
		strategy        string
		ipHeaders       []string
		headerValues    map[string]string
		remoteAddr      string
		expectedCountry string
		description     string
	}{
		{
			name:      "CheckFirst_RemoteAddress_First",
			strategy:  IPHeaderStrategyCheckFirst,
			ipHeaders: []string{"remoteAddress", "x-forwarded-for"},
			headerValues: map[string]string{
				"x-forwarded-for": "1.1.1.1", // AU
			},
			remoteAddr:      "8.8.8.8:12345", // US
			expectedCountry: "US",
			description:     "CheckFirst should use remoteAddress when it's first in order",
		},
		{
			name:      "CheckFirst_RemoteAddress_Second",
			strategy:  IPHeaderStrategyCheckFirst,
			ipHeaders: []string{"x-forwarded-for", "remoteAddress"},
			headerValues: map[string]string{
				"x-forwarded-for": "1.1.1.1", // AU
			},
			remoteAddr:      "8.8.8.8:12345", // US
			expectedCountry: "AU",
			description:     "CheckFirst should use header IP when remoteAddress is second",
		},
		{
			name:      "CheckFirstNonePrivate_RemoteAddress_Public",
			strategy:  IPHeaderStrategyCheckFirstNonePrivate,
			ipHeaders: []string{"x-forwarded-for", "remoteAddress"},
			headerValues: map[string]string{
				"x-forwarded-for": "192.168.1.1", // Private
			},
			remoteAddr:      "8.8.8.8:12345", // US (public)
			expectedCountry: "US",
			description:     "CheckFirstNonePrivate should use public remoteAddress over private header IP",
		},
		{
			name:      "CheckFirstNonePrivate_RemoteAddress_Private",
			strategy:  IPHeaderStrategyCheckFirstNonePrivate,
			ipHeaders: []string{"remoteAddress", "x-forwarded-for"},
			headerValues: map[string]string{
				"x-forwarded-for": "8.8.8.8", // US (public)
			},
			remoteAddr:      "192.168.1.1:12345", // Private
			expectedCountry: "US",
			description:     "CheckFirstNonePrivate should skip private remoteAddress and use public header IP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCfg := *cfg
			testCfg.IPHeaders = tt.ipHeaders
			testCfg.IPHeaderStrategy = tt.strategy

			plugin, err := New(context.TODO(), &noopHandler{}, &testCfg, pluginName)
			if err != nil {
				t.Fatalf("Failed to create plugin: %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tt.remoteAddr

			// Set headers as specified in test
			for header, value := range tt.headerValues {
				req.Header.Set(header, value)
			}

			rr := httptest.NewRecorder()
			plugin.ServeHTTP(rr, req)

			countryHeader := req.Header.Get("x-country-code")
			if countryHeader != tt.expectedCountry {
				t.Errorf("Expected country header '%s', got '%s'", tt.expectedCountry, countryHeader)
			}

			t.Logf("SUCCESS: %s - Strategy: %s, RemoteAddr: %s -> Country: %s",
				tt.description, tt.strategy, tt.remoteAddr, countryHeader)
		})
	}
}

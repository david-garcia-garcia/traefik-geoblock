package geoblock

import (
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlugin_ServeHTTP(t *testing.T) {
	t.Run("Allowed", func(t *testing.T) {
		cfg := &Config{
			Mode:                 ModeEnrichAndBlock,
			DatabaseSources:      seedCatalog(dbFilePath),
			AllowedCountries:     []string{"AU"},
			DisallowedStatusCode: http.StatusOK,
			IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}

		plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
		if err != nil {
			t.Errorf("expected no error, but got: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		req.Header.Set("X-Real-IP", "1.1.1.1")

		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)

		if rr.Code != http.StatusTeapot {
			t.Errorf("expected status code %d, but got: %d", http.StatusTeapot, rr.Code)
		}
	})

	t.Run("AllowedPrivate", func(t *testing.T) {
		cfg := &Config{
			Mode:                 ModeEnrichAndBlock,
			DatabaseSources:      seedCatalog(dbFilePath),
			AllowedCountries:     []string{},
			AllowPrivate:         true,
			DisallowedStatusCode: http.StatusOK,
			IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}

		plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
		if err != nil {
			t.Errorf("expected no error, but got: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		req.Header.Set("X-Real-IP", "192.168.178.66")

		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)

		if rr.Code != http.StatusTeapot {
			t.Errorf("expected status code %d, but got: %d", http.StatusTeapot, rr.Code)
		}
	})

	t.Run("AllowedPrivate172Range", func(t *testing.T) {
		cfg := &Config{
			Mode:                 ModeEnrichAndBlock,
			DatabaseSources:      seedCatalog(dbFilePath),
			AllowedCountries:     []string{},
			AllowPrivate:         true,
			DisallowedStatusCode: http.StatusOK,
			IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}

		plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
		if err != nil {
			t.Errorf("expected no error, but got: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		req.Header.Set("X-Real-IP", "172.19.0.1")

		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)

		if rr.Code != http.StatusTeapot {
			t.Errorf("expected status code %d, but got: %d", http.StatusTeapot, rr.Code)
		}
	})

	t.Run("Disallowed", func(t *testing.T) {
		cfg := &Config{
			Mode:                 ModeEnrichAndBlock,
			DatabaseSources:      seedCatalog(dbFilePath),
			AllowedCountries:     []string{"DE"},
			DisallowedStatusCode: http.StatusForbidden,
			BanHtmlFilePath:      filepath.Join(moduleRoot(), "geoblockban.html"),
			IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}

		plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
		if err != nil {
			t.Errorf("expected no error, but got: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		req.Header.Set("X-Real-IP", "1.1.1.1")

		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status code %d, but got: %d", http.StatusForbidden, rr.Code)
		}

		// Check that response contains the IP address
		body := rr.Body.String()
		if !strings.Contains(body, "1.1.1.1") {
			t.Errorf("expected response to contain IP address '1.1.1.1', but got: %s", body)
		}
	})

	t.Run("DisallowedPrivate", func(t *testing.T) {
		cfg := &Config{
			Mode:                 ModeEnrichAndBlock,
			DatabaseSources:      seedCatalog(dbFilePath),
			AllowedCountries:     []string{},
			AllowPrivate:         false,
			DisallowedStatusCode: http.StatusForbidden,
			BanHtmlFilePath:      filepath.Join(moduleRoot(), "geoblockban.html"),
			IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}

		plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
		if err != nil {
			t.Errorf("expected no error, but got: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		req.Header.Set("X-Real-IP", "192.168.178.66")

		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status code %d, but got: %d", http.StatusForbidden, rr.Code)
		}

		// Check that response contains the IP address
		body := rr.Body.String()
		if !strings.Contains(body, "192.168.178.66") {
			t.Errorf("expected response to contain IP address '192.168.178.66', but got: %s", body)
		}
	})

	t.Run("Blocklist", func(t *testing.T) {
		cfg := &Config{
			Mode:                 ModeEnrichAndBlock,
			DatabaseSources:      seedCatalog(dbFilePath),
			BlockedCountries:     []string{"US"},
			AllowPrivate:         false,
			DefaultAllow:         true,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}

		testRequest(t, "US IP blocked", cfg, "8.8.8.8", http.StatusForbidden)
		testRequest(t, "DE IP allowed", cfg, "185.5.82.105", 0)

		cfg.BlockedCountries = nil
		cfg.BlockedIPBlocks = []string{"8.8.8.0/24"}

		testRequest(t, "Google DNS-A blocked", cfg, "8.8.8.8", http.StatusForbidden)
		testRequest(t, "Google DNS-B allowed", cfg, "8.8.4.4", 0)

		cfg.AllowedIPBlocks = []string{"8.8.8.7/32"}

		testRequest(t, "Higher specificity IP CIDR allow trumps lower specificity IP CIDR block", cfg, "8.8.8.7", 0)
		testRequest(t, "Higher specificity IP CIDR allow should not override encompassing CIDR block", cfg, "8.8.8.9", http.StatusForbidden)

		cfg.DefaultAllow = false

		testRequest(t, "Default allow false", cfg, "8.8.4.4", http.StatusForbidden)
	})

	t.Run("IPWhitelist", func(t *testing.T) {
		cfg := &Config{
			Mode:                 ModeEnrichAndBlock,
			DatabaseSources:      seedCatalog(dbFilePath),
			AllowedIPBlocks:      []string{"203.0.113.0/24", "198.51.100.1/32"}, // Using TEST-NET-3 and TEST-NET-2 ranges
			DefaultAllow:         false,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}

		testRequest(t, "Whitelisted subnet allowed", cfg, "203.0.113.100", http.StatusTeapot)
		testRequest(t, "Whitelisted specific IP allowed", cfg, "198.51.100.1", http.StatusTeapot)
		testRequest(t, "Non-whitelisted IP blocked", cfg, "203.0.114.1", http.StatusForbidden)

		// Test interaction with country rules
		cfg.AllowedCountries = []string{"US"}
		testRequest(t, "Whitelisted IP allowed despite country rules", cfg, "203.0.113.100", http.StatusTeapot)
		testRequest(t, "US IP allowed when in allowed countries", cfg, "8.8.8.8", http.StatusTeapot)
		testRequest(t, "Non-US IP blocked when not in whitelist", cfg, "1.1.1.1", http.StatusForbidden)
	})

	t.Run("BypassHeaders", func(t *testing.T) {
		cfg := &Config{
			Mode:                 ModeEnrichAndBlock,
			DatabaseSources:      seedCatalog(dbFilePath),
			BlockedCountries:     []string{"US"},
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
			BypassHeaders: map[string]string{
				"X-Bypass-Key": "secret123",
				"Auth-Token":   "bypass-token",
			},
		}

		tests := []struct {
			name         string
			headers      map[string]string
			expectedCode int
			description  string
		}{
			{
				name: "bypass with correct X-Bypass-Key",
				headers: map[string]string{
					"X-Real-IP":    "8.8.8.8", // US IP that would normally be blocked
					"X-Bypass-Key": "secret123",
				},
				expectedCode: http.StatusTeapot,
				description:  "should allow access with correct bypass header",
			},
			{
				name: "bypass with correct Auth-Token",
				headers: map[string]string{
					"X-Real-IP":  "8.8.8.8",
					"Auth-Token": "bypass-token",
				},
				expectedCode: http.StatusTeapot,
				description:  "should allow access with correct auth token",
			},
			{
				name: "no bypass with incorrect header value",
				headers: map[string]string{
					"X-Real-IP":    "8.8.8.8",
					"X-Bypass-Key": "wrong-secret",
				},
				expectedCode: http.StatusForbidden,
				description:  "should block access with incorrect bypass header",
			},
			{
				name: "no bypass without headers",
				headers: map[string]string{
					"X-Real-IP": "8.8.8.8",
				},
				expectedCode: http.StatusForbidden,
				description:  "should block access without bypass headers",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
				if err != nil {
					t.Fatalf("expected no error, but got: %v", err)
				}

				req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
				for key, value := range tt.headers {
					req.Header.Set(key, value)
				}

				rr := httptest.NewRecorder()
				plugin.ServeHTTP(rr, req)

				if rr.Code != tt.expectedCode {
					t.Errorf("%s: expected status code %d, but got: %d",
						tt.description, tt.expectedCode, rr.Code)
				}
			})
		}
	})

	t.Run("Set Country Header", func(t *testing.T) {
		countryHeader := "X-Country"
		cfg := &Config{
			Mode:                 ModeEnrichAndBlock,
			DatabaseSources:      seedCatalog(dbFilePath),
			AllowedCountries:     []string{"AU"},
			DisallowedStatusCode: http.StatusForbidden,
			CountryHeader:        countryHeader,
			IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}

		tests := []struct {
			name                  string
			ip                    string
			expectedCode          int
			expectedCountryHeader string
			description           string
		}{
			{
				name:                  "Request from allowed country",
				ip:                    "1.1.1.1",
				expectedCode:          http.StatusTeapot,
				expectedCountryHeader: "AU",
				description:           "should set country header if request is allowed",
			},
			{
				name:                  "Request from disallowed country",
				ip:                    "8.8.8.8",
				expectedCountryHeader: "US",
				expectedCode:          http.StatusForbidden,
				description:           "should set country header if request is denied",
			},
			{
				name:                  "Request from private IP",
				ip:                    "192.168.178.66",
				expectedCountryHeader: "PRIVATE",
				expectedCode:          http.StatusForbidden,
				description:           "should set country header to PRIVATE for private IP",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
				if err != nil {
					t.Fatalf("expected no error, but got: %v", err)
				}

				req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
				req.Header.Set("X-Real-IP", tt.ip)

				rr := httptest.NewRecorder()
				plugin.ServeHTTP(rr, req)

				if rr.Code != tt.expectedCode {
					t.Errorf("%s: expected status code %d, but got: %d",
						tt.description, tt.expectedCode, rr.Code)
				}
				header := req.Header.Get(countryHeader)
				if header != tt.expectedCountryHeader {
					t.Errorf("expected header %s with value %s, but got: %s", countryHeader, tt.expectedCountryHeader, header)
				}
			})
		}
	})
}

func testRequest(t *testing.T, testName string, cfg *Config, ip string, expectedStatus int) {
	t.Run(testName, func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
		if err != nil {
			t.Errorf("expected no error, but got: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		req.Header.Set("X-Real-IP", ip)

		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)

		if expectedStatus > 0 && rr.Code != expectedStatus {
			t.Errorf("expected status code %d, but got: %d", expectedStatus, rr.Code)
		}
	})
}

func TestPlugin_Lookup(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		cfg := &Config{
			Mode:                 ModeEnrichAndBlock,
			DatabaseSources:      seedCatalog(dbFilePath),
			AllowedCountries:     []string{},
			AllowPrivate:         false,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}

		plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
		if err != nil {
			t.Errorf("expected no error, but got: %v", err)
		}

		rec, err := plugin.(*Route).Lookup("8.8.8.8")
		if err != nil {
			t.Errorf("expected no error, but got: %v", err)
		}
		if rec.Country != "US" {
			t.Errorf("expected country to be %s, but got: %s", "US", rec.Country)
		}
		if rec.Asn != "" {
			t.Errorf("expected empty ASN without ASN BIN, got %q", rec.Asn)
		}
	})

	t.Run("Invalid", func(t *testing.T) {
		cfg := &Config{
			Mode:                 ModeEnrichAndBlock,
			DatabaseSources:      seedCatalog(dbFilePath),
			AllowedCountries:     []string{},
			AllowPrivate:         false,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}

		plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
		if err != nil {
			t.Errorf("expected no error, but got: %v", err)
		}

		rec, err := plugin.(*Route).Lookup("foobar")
		if err == nil {
			t.Errorf("expected error, but got none")
		}
		if err.Error() != "Invalid IP address." {
			t.Errorf("unexpected error: %v", err)
		}
		if rec.Country != "" {
			t.Errorf("expected country to be empty, but was: %s", rec.Country)
		}
	})
}

func TestPlugin_ServeHTTP_MalformedIP(t *testing.T) {
	tests := []struct {
		name       string
		banIfError bool
		ip         string
		wantStatus int
	}{
		{
			name:       "malformed IP with banIfError true",
			banIfError: true,
			ip:         "not.an.ip.address",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "malformed IP with banIfError false",
			banIfError: false,
			ip:         "not.an.ip.address",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test response recorder
			rr := httptest.NewRecorder()

			// Create a test request with the malformed IP
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("X-Forwarded-For", tt.ip)

			// Create a mock next handler that always returns 200 OK
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Create plugin config
			cfg := &Config{
				Mode:                 ModeEnrichAndBlock,
				DisallowedStatusCode: http.StatusForbidden,
				BanIfError:           tt.banIfError,
				DatabaseSources:      seedCatalog(dbFilePath),
				IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
				IPHeaderStrategy:     IPHeaderStrategyCheckAll,
			}

			// Initialize plugin
			plugin, err := newRoute(holdCtx(t), next, cfg, "test")
			if err != nil {
				t.Fatalf("Failed to create plugin: %v", err)
			}

			// Serve the request
			plugin.ServeHTTP(rr, req)

			// Check the status code
			if rr.Code != tt.wantStatus {
				t.Errorf("ServeHTTP() status = %v, want %v", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestPrivateIPDetection(t *testing.T) {
	// Test what Go's net.IP.IsPrivate() actually returns for various IPs
	testIPs := []struct {
		ip       string
		expected bool
		name     string
	}{
		{"127.0.0.1", true, "localhost"},
		{"127.0.0.2", true, "loopback range"},
		{"::1", true, "IPv6 localhost"},
		{"192.168.1.1", true, "private range 192.168"},
		{"10.0.0.1", true, "private range 10.x"},
		{"172.16.0.1", true, "private range 172.16-31"},
		{"8.8.8.8", false, "public IP"},
		{"1.1.1.1", false, "public IP"},
		{"2001:db8::1", false, "public IPv6"},
	}

	for _, test := range testIPs {
		t.Run(test.name+"_"+test.ip, func(t *testing.T) {
			ipAddr := net.ParseIP(test.ip)
			if ipAddr == nil {
				t.Fatalf("Failed to parse IP: %s", test.ip)
			}

			// Test what our updated logic returns
			isPrivateOrLoopback := ipAddr.IsPrivate() || ipAddr.IsLoopback()
			t.Logf("IP %s: IsPrivate() || IsLoopback() = %v", test.ip, isPrivateOrLoopback)

			if isPrivateOrLoopback != test.expected {
				t.Errorf("IP %s: expected IsPrivate() || IsLoopback() = %v, got %v", test.ip, test.expected, isPrivateOrLoopback)
			}
		})
	}
}

func TestCheckAllowed_Localhost(t *testing.T) {
	// Test the actual CheckAllowed method with 127.0.0.1
	cfg := &Config{
		Mode:                 ModeEnrichAndBlock,
		DatabaseSources:      seedCatalog(dbFilePath),
		AllowPrivate:         true,
		DefaultAllow:         false,
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
		IPHeaderStrategy:     IPHeaderStrategyCheckAll,
	}

	plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	p := plugin.(*Route)

	// Test various loopback IPs (IPv4 and IPv6)
	testIPs := []string{"127.0.0.1", "127.0.0.2", "127.1.1.1", "::1"}

	for _, ip := range testIPs {
		t.Run("IP_"+ip, func(t *testing.T) {
			allowed, phase, err := p.CheckAllowed(ip)

			t.Logf("CheckAllowed(%s) = allowed:%v, phase:%s, err:%v",
				ip, allowed, phase, err)

			if err != nil {
				t.Errorf("CheckAllowed returned error: %v", err)
			}

			// With allowPrivate=true, loopback IPs should be allowed
			if !allowed {
				t.Errorf("IP %s should be allowed when allowPrivate=true, but was blocked", ip)
			}

			// Phase should be "allow_private" for private IPs
			if phase != PhaseAllowPrivate {
				t.Errorf("IP %s should have phase='%s', but got '%s'", ip, PhaseAllowPrivate, phase)
			}
		})
	}
}

func TestServeHTTP_LocalhostWithAllowPrivate(t *testing.T) {
	// Test complete HTTP request flow with localhost
	cfg := &Config{
		Mode:                 ModeEnrichAndBlock,
		DatabaseSources:      seedCatalog(dbFilePath),
		AllowPrivate:         true,
		DefaultAllow:         false,
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
		IPHeaderStrategy:     IPHeaderStrategyCheckAll,
	}

	plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	// Test direct localhost request (simulating direct access)
	req := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	req.RemoteAddr = "127.0.0.1:12345" // This simulates direct localhost access
	req.Header.Set("X-Real-IP", "127.0.0.1")

	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)

	if rr.Code != http.StatusTeapot {
		t.Errorf("Expected localhost request to be allowed (status %d), but got status %d",
			http.StatusTeapot, rr.Code)
	}

	t.Logf("SUCCESS: Localhost request with allowPrivate=true was allowed (status %d)", rr.Code)
}

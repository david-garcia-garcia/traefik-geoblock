package traefik_geoblock

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	pluginName = "geoblock"
	dbFilePath = "./IP2LOCATION-LITE-DB1.IPV6.BIN"
)

type noopHandler struct{}

func (n noopHandler) ServeHTTP(rw http.ResponseWriter, _ *http.Request) {
	rw.WriteHeader(http.StatusTeapot)
}

func TestNew(t *testing.T) {
	t.Run("Disabled", func(t *testing.T) {
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{Enabled: false, IPHeaders: []string{"x-real-ip"}, IPHeaderStrategy: IPHeaderStrategyCheckAll}, pluginName)
		if err != nil {
			t.Errorf("expected no error, but got: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/foobar", nil)

		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)

		if rr.Code != http.StatusTeapot {
			t.Errorf("expected status code %d, but got: %d", http.StatusTeapot, rr.Code)
		}
	})

	t.Run("NoNextHandler", func(t *testing.T) {
		plugin, err := New(context.TODO(), nil, &Config{Enabled: true, IPHeaders: []string{"x-real-ip"}, IPHeaderStrategy: IPHeaderStrategyCheckAll}, pluginName)
		if err == nil {
			t.Errorf("expected error, but got none")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil, but is not")
		}
	})

	t.Run("Nogeoblock.Config", func(t *testing.T) {
		plugin, err := New(context.TODO(), &noopHandler{}, nil, pluginName)
		if err == nil {
			t.Errorf("expected error, but got none")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil, but is not")
		}
	})

	t.Run("InvalidDisallowedStatusCode", func(t *testing.T) {
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{Enabled: true, DisallowedStatusCode: -1, IPHeaders: []string{"x-real-ip"}, IPHeaderStrategy: IPHeaderStrategyCheckAll}, pluginName)
		if err == nil {
			t.Errorf("expected error, but got none")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil, but is not")
		}
	})

	t.Run("UnableToFindDatabase", func(t *testing.T) {
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{Enabled: true, DisallowedStatusCode: http.StatusForbidden, DatabaseFilePath: "bad-database", IPHeaders: []string{"x-real-ip"}}, pluginName)
		if err == nil {
			t.Errorf("expected error, but got none")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil, but is not")
		}
	})

	t.Run("InvalidDatabaseVersion", func(t *testing.T) {
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:          true,
			DatabaseFilePath: "./testdata/invalid.bin",
			IPHeaders:        []string{"x-real-ip"},
		}, pluginName)
		if err == nil {
			t.Errorf("expected error about invalid database version, but got none")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil, but is not")
		}
	})

	t.Run("EmptyIPHeaders", func(t *testing.T) {
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:              true,
			DatabaseFilePath:     dbFilePath,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{}, // Empty slice should be rejected
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}, pluginName)
		if err == nil {
			t.Errorf("expected error about empty IPHeaders, but got none")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil, but is not")
		}
	})

	t.Run("CustomIPHeaders", func(t *testing.T) {
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:              true,
			DatabaseFilePath:     dbFilePath,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"custom-ip-header", "another-ip-header"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
			AllowedCountries:     []string{"AU"},
		}, pluginName)
		if err != nil {
			t.Errorf("expected no error, but got: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin to not be nil")
		}

		// Test that custom headers are used for IP extraction
		req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		req.Header.Set("custom-ip-header", "1.1.1.1") // Cloudflare DNS (AU)

		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)

		if rr.Code != http.StatusTeapot {
			t.Errorf("expected status code %d for allowed AU IP, but got: %d", http.StatusTeapot, rr.Code)
		}

		// Test that default headers are NOT used when custom headers are configured
		req2 := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		req2.Header.Set("x-real-ip", "1.1.1.1")       // This should be ignored
		req2.Header.Set("x-forwarded-for", "1.1.1.1") // This should be ignored

		rr2 := httptest.NewRecorder()
		plugin.ServeHTTP(rr2, req2)

		// Should get localhost behavior (allowed due to private IP) since custom headers are not set
		if rr2.Code != http.StatusTeapot {
			t.Errorf("expected status code %d when custom headers not present, but got: %d", http.StatusTeapot, rr2.Code)
		}
	})

	// NEW: Test environment variable fallback when no filepath is provided
	t.Run("EnvironmentVariableFallback_NoFilePath", func(t *testing.T) {
		// Cleanup any existing factories to avoid conflicts
		CleanupFactories()
		defer CleanupFactories()

		// Create a temporary directory and database file for the environment variable
		envDir := t.TempDir()
		envDBPath := filepath.Join(envDir, "IP2LOCATION-LITE-DB1.IPV6.BIN")

		// Copy the existing database to the environment directory
		dbContent, err := os.ReadFile(dbFilePath)
		if err != nil {
			t.Fatalf("failed to read source database: %v", err)
		}
		if err := os.WriteFile(envDBPath, dbContent, 0600); err != nil {
			t.Fatalf("failed to create env database: %v", err)
		}

		// Set the environment variable
		originalEnv := os.Getenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH")
		os.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", envDir)
		defer func() {
			if originalEnv != "" {
				os.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", originalEnv)
			} else {
				os.Unsetenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH")
			}
		}()

		// Try to create plugin with empty DatabaseFilePath - should use environment variable
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:              true,
			DatabaseFilePath:     "", // Empty - should fallback to environment variable
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}, pluginName)

		if err != nil {
			t.Errorf("expected no error when using environment variable fallback, but got: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin not to be nil when using environment variable fallback")
		}

		// Verify the plugin works by testing a lookup
		if plugin != nil {
			country, err := plugin.(*Plugin).Lookup("8.8.8.8")
			if err != nil {
				t.Errorf("expected successful lookup with env var database, but got: %v", err)
			}
			if country != "US" {
				t.Errorf("expected country US for 8.8.8.8, got %s", country)
			}
		}
	})

	// NEW: Test environment variable fallback when bad filepath is provided
	t.Run("BadFilePath_FallbackToEnvironmentVariable", func(t *testing.T) {
		// Cleanup any existing factories to avoid conflicts
		CleanupFactories()
		defer CleanupFactories()

		// Create a temporary directory and database file for the environment variable
		envDir := t.TempDir()
		envDBPath := filepath.Join(envDir, "IP2LOCATION-LITE-DB1.IPV6.BIN")

		// Copy the existing database to the environment directory
		dbContent, err := os.ReadFile(dbFilePath)
		if err != nil {
			t.Fatalf("failed to read source database: %v", err)
		}
		if err := os.WriteFile(envDBPath, dbContent, 0600); err != nil {
			t.Fatalf("failed to create env database: %v", err)
		}

		// Set the environment variable
		originalEnv := os.Getenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH")
		os.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", envDir)
		defer func() {
			if originalEnv != "" {
				os.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", originalEnv)
			} else {
				os.Unsetenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH")
			}
		}()

		// Try to create plugin with bad DatabaseFilePath but valid environment variable
		badDBPath := "/nonexistent/path/bad-database.bin"
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:              true,
			DatabaseFilePath:     badDBPath, // Bad path
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}, pluginName)

		if err != nil {
			t.Errorf("expected no error when environment variable is valid, but got: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin not to be nil when environment variable is valid")
		}

		// Verify the plugin is using the database from the environment variable
		if plugin != nil {
			factory, err := GetDatabaseFactory(&DatabaseConfig{
				DatabaseFilePath: badDBPath,
			}, plugin.(*Plugin).logger)
			if err != nil {
				t.Errorf("failed to get database factory: %v", err)
			} else {
				actualPath := factory.GetWrapper().GetPath()
				if !filepath.IsAbs(actualPath) || !strings.Contains(actualPath, envDir) {
					t.Errorf("expected database path to be from environment directory, got: %s", actualPath)
				}
			}
		}
	})

	// NEW: Test error when both filepath and environment variable are bad
	t.Run("BadFilePath_BadEnvironmentVariable_ShouldError", func(t *testing.T) {
		// Cleanup any existing factories to avoid conflicts
		CleanupFactories()
		defer CleanupFactories()

		// Set a bad environment variable
		originalEnv := os.Getenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH")
		os.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", "/nonexistent/env/path")
		defer func() {
			if originalEnv != "" {
				os.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", originalEnv)
			} else {
				os.Unsetenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH")
			}
		}()

		// Try to create plugin with bad DatabaseFilePath and bad environment variable
		badDBPath := "/nonexistent/path/bad-database.bin"
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:              true,
			DatabaseFilePath:     badDBPath, // Bad path
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}, pluginName)

		if err == nil {
			t.Error("expected error when both filepath and environment variable are bad, but got none")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil when both filepath and environment variable are bad")
		}
	})

	// NEW: Test error when no filepath and no environment variable are provided
	t.Run("NoFilePath_NoEnvironmentVariable_ShouldError", func(t *testing.T) {
		// Cleanup any existing factories to avoid conflicts
		CleanupFactories()
		defer CleanupFactories()

		// Ensure no environment variable is set
		originalEnv := os.Getenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH")
		os.Unsetenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH")
		defer func() {
			if originalEnv != "" {
				os.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", originalEnv)
			}
		}()

		// Try to create plugin with no DatabaseFilePath and no environment variable
		// This should fail since the Search function doesn't automatically search current directory
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:              true,
			DatabaseFilePath:     "", // Empty - should fail without environment variable
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}, pluginName)

		if err == nil {
			t.Error("expected error when no filepath and no environment variable are provided, but got none")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil when no filepath and no environment variable are provided")
		}
	})
}

func TestNew_AutoUpdate(t *testing.T) {
	// Create a temporary directory for test databases
	tmpDir, err := os.MkdirTemp("", "geoblock-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// This is the default location for the internal temp copy
	tmpFile := filepath.Join(os.TempDir(), "IP2LOCATION-LITE-DB1.IPV6.BIN")
	_ = os.Remove(tmpFile)

	// Copy the test database to the temp directory with a versioned name
	versionedDbPath := filepath.Join(tmpDir, "20240301_IP2LOCATION-LITE-DB1.IPV6.BIN")
	if err := copyFile(dbFilePath, versionedDbPath, true); err != nil {
		t.Fatalf("Failed to copy test database: %v", err)
	}

	t.Run("AutoUpdateEnabled", func(t *testing.T) {
		cfg := &Config{
			Enabled:                true,
			DatabaseFilePath:       dbFilePath, // Add fallback database path
			DatabaseAutoUpdate:     true,
			DatabaseAutoUpdateDir:  tmpDir,
			DatabaseAutoUpdateCode: "DB1",
			IPHeaderStrategy:       IPHeaderStrategyCheckAll,
			DisallowedStatusCode:   http.StatusForbidden,
			IPHeaders:              []string{"x-forwarded-for", "x-real-ip"},
		}

		plugin, err := New(context.TODO(), &noopHandler{}, cfg, pluginName)
		if err != nil {
			t.Errorf("expected no error, but got: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin to not be nil")
		}

		// Verify that the database is working by testing a lookup
		p := plugin.(*Plugin)
		country, err := p.Lookup("8.8.8.8")
		if err != nil {
			t.Errorf("expected database to be initialized and working, but lookup failed: %v", err)
		}
		if country != "US" {
			t.Errorf("expected lookup to return US for 8.8.8.8, but got: %s", country)
		}
	})

	t.Run("AutoUpdateEnabledNoDir", func(t *testing.T) {
		cfg := &Config{
			Enabled:              true,
			DatabaseAutoUpdate:   true,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
			// Deliberately omit DatabaseAutoUpdateDir AND DatabaseFilePath
		}

		plugin, err := New(context.TODO(), &noopHandler{}, cfg, pluginName)
		if err == nil {
			t.Error("expected error about missing database path, but got none")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil")
		}
	})

	t.Run("AutoUpdateEnabledEmptyDir", func(t *testing.T) {
		emptyDir, err := os.MkdirTemp("", "geoblock-empty-*")
		if err != nil {
			t.Fatalf("Failed to create empty temp dir: %v", err)
		}
		defer os.RemoveAll(emptyDir)

		cfg := &Config{
			Enabled:               true,
			DatabaseAutoUpdate:    true,
			DatabaseAutoUpdateDir: emptyDir,
			DisallowedStatusCode:  http.StatusForbidden,
			DatabaseFilePath:      dbFilePath, // Fall back to default database
			IPHeaders:             []string{"x-forwarded-for", "x-real-ip"},
			IPHeaderStrategy:      IPHeaderStrategyCheckAll,
		}

		plugin, err := New(context.TODO(), &noopHandler{}, cfg, pluginName)
		if err != nil {
			t.Errorf("expected no error when falling back to default database, but got: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin to not be nil when falling back to default database")
		}
	})
}

func TestPlugin_ServeHTTP(t *testing.T) {
	t.Run("Allowed", func(t *testing.T) {
		cfg := &Config{
			Enabled:              true,
			DatabaseFilePath:     dbFilePath,
			AllowedCountries:     []string{"AU"},
			DisallowedStatusCode: http.StatusOK,
			IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}

		plugin, err := New(context.TODO(), &noopHandler{}, cfg, pluginName)
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
			Enabled:              true,
			DatabaseFilePath:     dbFilePath,
			AllowedCountries:     []string{},
			AllowPrivate:         true,
			DisallowedStatusCode: http.StatusOK,
			IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}

		plugin, err := New(context.TODO(), &noopHandler{}, cfg, pluginName)
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
			Enabled:              true,
			DatabaseFilePath:     dbFilePath,
			AllowedCountries:     []string{},
			AllowPrivate:         true,
			DisallowedStatusCode: http.StatusOK,
			IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}

		plugin, err := New(context.TODO(), &noopHandler{}, cfg, pluginName)
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
			Enabled:              true,
			DatabaseFilePath:     dbFilePath,
			AllowedCountries:     []string{"DE"},
			DisallowedStatusCode: http.StatusForbidden,
			BanHtmlFilePath:      "geoblockban.html",
			IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}

		plugin, err := New(context.TODO(), &noopHandler{}, cfg, pluginName)
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
			Enabled:              true,
			DatabaseFilePath:     dbFilePath,
			AllowedCountries:     []string{},
			AllowPrivate:         false,
			DisallowedStatusCode: http.StatusForbidden,
			BanHtmlFilePath:      "geoblockban.html",
			IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}

		plugin, err := New(context.TODO(), &noopHandler{}, cfg, pluginName)
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
			Enabled:              true,
			DatabaseFilePath:     dbFilePath,
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
			Enabled:              true,
			DatabaseFilePath:     dbFilePath,
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
			Enabled:              true,
			DatabaseFilePath:     dbFilePath,
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
				plugin, err := New(context.TODO(), &noopHandler{}, cfg, pluginName)
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
			Enabled:              true,
			DatabaseFilePath:     dbFilePath,
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
				plugin, err := New(context.TODO(), &noopHandler{}, cfg, pluginName)
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
		plugin, err := New(context.TODO(), &noopHandler{}, cfg, pluginName)
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
			Enabled:              true,
			DatabaseFilePath:     dbFilePath,
			AllowedCountries:     []string{},
			AllowPrivate:         false,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}

		plugin, err := New(context.TODO(), &noopHandler{}, cfg, pluginName)
		if err != nil {
			t.Errorf("expected no error, but got: %v", err)
		}

		country, err := plugin.(*Plugin).Lookup("8.8.8.8")
		if err != nil {
			t.Errorf("expected no error, but got: %v", err)
		}
		if country != "US" {
			t.Errorf("expected country to be %s, but got: %s", "US", country)
		}
	})

	t.Run("Invalid", func(t *testing.T) {
		cfg := &Config{
			Enabled:              true,
			DatabaseFilePath:     dbFilePath,
			AllowedCountries:     []string{},
			AllowPrivate:         false,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}

		plugin, err := New(context.TODO(), &noopHandler{}, cfg, pluginName)
		if err != nil {
			t.Errorf("expected no error, but got: %v", err)
		}

		country, err := plugin.(*Plugin).Lookup("foobar")
		if err == nil {
			t.Errorf("expected error, but got none")
		}
		if err.Error() != "Invalid IP address." {
			t.Errorf("unexpected error: %v", err)
		}
		if country != "" {
			t.Errorf("expected country to be empty, but was: %s", country)
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
				Enabled:              true,
				DisallowedStatusCode: http.StatusForbidden,
				BanIfError:           tt.banIfError,
				DatabaseFilePath:     dbFilePath,
				IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
				IPHeaderStrategy:     IPHeaderStrategyCheckAll,
			}

			// Initialize plugin
			plugin, err := New(context.Background(), next, cfg, "test")
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
		Enabled:              true,
		DatabaseFilePath:     dbFilePath,
		AllowPrivate:         true,
		DefaultAllow:         false,
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
		IPHeaderStrategy:     IPHeaderStrategyCheckAll,
	}

	plugin, err := New(context.TODO(), &noopHandler{}, cfg, pluginName)
	if err != nil {
		t.Fatalf("Failed to create plugin: %v", err)
	}

	p := plugin.(*Plugin)

	// Test various loopback IPs (IPv4 and IPv6)
	testIPs := []string{"127.0.0.1", "127.0.0.2", "127.1.1.1", "::1"}

	for _, ip := range testIPs {
		t.Run("IP_"+ip, func(t *testing.T) {
			allowed, country, phase, err := p.CheckAllowed(ip)

			t.Logf("CheckAllowed(%s) = allowed:%v, country:%s, phase:%s, err:%v",
				ip, allowed, country, phase, err)

			if err != nil {
				t.Errorf("CheckAllowed returned error: %v", err)
			}

			// With allowPrivate=true, loopback IPs should be allowed
			if !allowed {
				t.Errorf("IP %s should be allowed when allowPrivate=true, but was blocked", ip)
			}

			// Country should be "PRIVATE" for private IPs
			if country != "PRIVATE" {
				t.Errorf("IP %s should have country='PRIVATE', but got '%s'", ip, country)
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
		Enabled:              true,
		DatabaseFilePath:     dbFilePath,
		AllowPrivate:         true,
		DefaultAllow:         false,
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
		IPHeaderStrategy:     IPHeaderStrategyCheckAll,
	}

	plugin, err := New(context.TODO(), &noopHandler{}, cfg, pluginName)
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

func TestIPHeaderStrategy_Integration(t *testing.T) {
	// Create a plugin with blocked countries (e.g., block CN)
	cfg := &Config{
		Enabled:              true,
		DatabaseFilePath:     dbFilePath,
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
		DatabaseFilePath:     dbFilePath,
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
		DatabaseFilePath:     dbFilePath,
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
		DatabaseFilePath:     dbFilePath,
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
		DatabaseFilePath:     dbFilePath,
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
		DatabaseFilePath:     dbFilePath,
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
				DatabaseFilePath:     dbFilePath,
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

func TestBypassHeaders_ShouldStillEnrichWithGeoIP(t *testing.T) {
	// Test that bypass headers skip blocking but still enrich with country information
	cfg := &Config{
		Enabled:              true,
		DatabaseFilePath:     dbFilePath,
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

	plugin, err := New(context.TODO(), &noopHandler{}, cfg, pluginName)
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
		DatabaseFilePath:     dbFilePath,
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

func TestIgnoreVerbs_ShouldSkipBlockingButStillEnrich(t *testing.T) {
	// Test that ignored HTTP verbs skip blocking but still get GeoIP enrichment
	cfg := &Config{
		Enabled:              true,
		DatabaseFilePath:     dbFilePath,
		AllowPrivate:         false,
		DefaultAllow:         false,
		BlockedCountries:     []string{"US"}, // Block US
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-forwarded-for"},
		IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		CountryHeader:        "x-country-code",
		IgnoreVerbs:          []string{"OPTIONS", "HEAD"}, // Ignore these verbs
	}

	plugin, err := New(context.TODO(), &noopHandler{}, cfg, pluginName)
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

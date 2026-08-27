package traefik_geoblock

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/ip2location"
)

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
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{Enabled: true, DisallowedStatusCode: http.StatusForbidden, Ip2locationDatabaseFilePath: "bad-database", IPHeaders: []string{"x-real-ip"}}, pluginName)
		if err == nil {
			t.Errorf("expected error, but got none")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil, but is not")
		}
	})

	t.Run("InvalidDatabaseVersion", func(t *testing.T) {
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:                     true,
			Ip2locationDatabaseFilePath: "./testdata/invalid.bin",
			IPHeaders:                   []string{"x-real-ip"},
		}, pluginName)
		if err == nil {
			t.Errorf("expected error about invalid database version, but got none")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil, but is not")
		}
	})

	t.Run("UnsupportedDatabaseProvider", func(t *testing.T) {
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:                     true,
			DatabaseProvider:            "maxmind",
			Ip2locationDatabaseFilePath: dbFilePath,
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"x-real-ip"},
			IPHeaderStrategy:            IPHeaderStrategyCheckAll,
		}, pluginName)
		if err == nil {
			t.Errorf("expected error about unsupported database provider, but got none")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil, but is not")
		}
		if err != nil && !strings.Contains(err.Error(), "unsupported database provider") {
			t.Errorf("expected unsupported database provider error, got: %v", err)
		}
	})

	t.Run("DeprecatedDatabaseFilePath", func(t *testing.T) {
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:              true,
			DatabaseFilePath:     dbFilePath,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}, pluginName)
		if err != nil {
			t.Errorf("expected deprecated databaseFilePath to map to ip2location_databaseFilePath, but got: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin not to be nil")
		}
	})

	t.Run("PrefixedDatabaseFilePathWinsOverDeprecated", func(t *testing.T) {
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:                     true,
			DatabaseFilePath:            "bad-database",
			Ip2locationDatabaseFilePath: dbFilePath,
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"x-real-ip"},
			IPHeaderStrategy:            IPHeaderStrategyCheckAll,
		}, pluginName)
		if err != nil {
			t.Errorf("expected prefixed path to win over deprecated databaseFilePath, but got: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin not to be nil")
		}
	})

	t.Run("ExplicitIPinfoProvider", func(t *testing.T) {
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:                true,
			DatabaseProvider:       DatabaseProviderIPinfo,
			IpinfoDatabaseFilePath: ipinfoFilePath,
			DisallowedStatusCode:   http.StatusForbidden,
			IPHeaders:              []string{"x-real-ip"},
			IPHeaderStrategy:       IPHeaderStrategyCheckAll,
		}, pluginName)
		if err != nil {
			t.Errorf("expected no error with databaseProvider ipinfo, but got: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin not to be nil")
		}
	})

	t.Run("IPinfoEmptyPathUsesBundledMMDB", func(t *testing.T) {
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:              true,
			DatabaseProvider:     DatabaseProviderIPinfo,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
			CountryHeader:        "x-country-code",
		}, pluginName)
		if err != nil {
			t.Fatalf("empty ipinfo path should use bundled MMDB: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		req.Header.Set("X-Real-IP", "8.8.8.8")
		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)
		if got := req.Header.Get("x-country-code"); got != "US" {
			t.Errorf("country: got %q want US", got)
		}
	})

	t.Run("IPinfoAutoUpdateWithoutDirFails", func(t *testing.T) {
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:                  true,
			DatabaseProvider:         DatabaseProviderIPinfo,
			IpinfoDatabaseFilePath:   ipinfoFilePath,
			IpinfoDatabaseAutoUpdate: true,
			DisallowedStatusCode:     http.StatusForbidden,
			IPHeaders:                []string{"x-real-ip"},
			IPHeaderStrategy:         IPHeaderStrategyCheckAll,
		}, pluginName)
		if err == nil {
			t.Fatal("expected New to fail when IPinfo auto-update is on without a dir")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil")
		}
	})

	t.Run("IPinfoAutoUpdateWithoutTokenDoesNotFail", func(t *testing.T) {
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:                     true,
			DatabaseProvider:            DatabaseProviderIPinfo,
			IpinfoDatabaseFilePath:      ipinfoFilePath,
			IpinfoDatabaseAutoUpdate:    true,
			IpinfoDatabaseAutoUpdateDir: t.TempDir(),
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"x-real-ip"},
			IPHeaderStrategy:            IPHeaderStrategyCheckAll,
		}, pluginName)
		if err != nil {
			t.Errorf("IPinfo auto-update without token must not fail New, got: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin not to be nil")
		}
	})

	t.Run("ExplicitIP2LocationProvider", func(t *testing.T) {
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:                     true,
			DatabaseProvider:            DatabaseProviderIP2Location,
			Ip2locationDatabaseFilePath: dbFilePath,
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"x-real-ip"},
			IPHeaderStrategy:            IPHeaderStrategyCheckAll,
		}, pluginName)
		if err != nil {
			t.Errorf("expected no error with databaseProvider ip2location, but got: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin not to be nil")
		}
	})

	t.Run("AsnAutoUpdateWithoutTokenDoesNotFail", func(t *testing.T) {
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:                          true,
			Ip2locationDatabaseFilePath:      dbFilePath,
			Ip2locationAsnDatabaseAutoUpdate: true,
			DisallowedStatusCode:             http.StatusForbidden,
			IPHeaders:                        []string{"x-real-ip"},
			IPHeaderStrategy:                 IPHeaderStrategyCheckAll,
		}, pluginName)
		if err != nil {
			t.Errorf("ASN auto-update without token must not fail New, got: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin not to be nil")
		}
		rec, err := plugin.(*Plugin).Lookup("8.8.8.8")
		if err != nil {
			t.Errorf("Lookup: %v", err)
		}
		if rec.Asn != "" {
			t.Errorf("expected empty ASN without token download or path, got %q", rec.Asn)
		}
	})

	t.Run("RequestHeaderEnrichIspDomainKeysAllowed", func(t *testing.T) {
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:                     true,
			Ip2locationDatabaseFilePath: dbFilePath,
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"x-real-ip"},
			IPHeaderStrategy:            IPHeaderStrategyCheckAll,
			RequestHeaderEnrich: map[string]string{
				"X-Geo-Isp":    "isp",
				"X-Geo-Domain": "domain",
			},
		}, pluginName)
		if err != nil {
			t.Errorf("expected isp/domain enrich keys to be allowed, got: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin not to be nil")
		}
	})

	t.Run("RequestHeaderEnrichAsnKeyAllowed", func(t *testing.T) {
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:                     true,
			Ip2locationDatabaseFilePath: dbFilePath,
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"x-real-ip"},
			IPHeaderStrategy:            IPHeaderStrategyCheckAll,
			RequestHeaderEnrich:         map[string]string{"X-Geo-Asn": "asn"},
		}, pluginName)
		if err != nil {
			t.Errorf("expected asn enrich key to be allowed, got: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin not to be nil")
		}
	})

	t.Run("CountryHeaderFoldsIntoRequestHeaderEnrich", func(t *testing.T) {
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:                     true,
			Ip2locationDatabaseFilePath: dbFilePath,
			CountryHeader:               "X-IPCountry",
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"x-real-ip"},
			IPHeaderStrategy:            IPHeaderStrategyCheckAll,
		}, pluginName)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		p := plugin.(*Plugin)
		if got := p.requestHeaderEnrich[http.CanonicalHeaderKey("X-IPCountry")]; got != dbprovider.MetaCountry {
			t.Errorf("folded countryHeader: got %q want %s", got, dbprovider.MetaCountry)
		}
	})

	t.Run("RequestHeaderEnrichWinsOverCountryHeader", func(t *testing.T) {
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:                     true,
			Ip2locationDatabaseFilePath: dbFilePath,
			CountryHeader:               "X-Geo-Country",
			RequestHeaderEnrich:         map[string]string{"X-Geo-Country": "city"},
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"x-real-ip"},
			IPHeaderStrategy:            IPHeaderStrategyCheckAll,
		}, pluginName)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		p := plugin.(*Plugin)
		if got := p.requestHeaderEnrich[http.CanonicalHeaderKey("X-Geo-Country")]; got != dbprovider.MetaCity {
			t.Errorf("explicit enrich should win: got %q want %s", got, dbprovider.MetaCity)
		}
	})

	t.Run("UnknownRequestHeaderEnrichKey", func(t *testing.T) {
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:                     true,
			Ip2locationDatabaseFilePath: dbFilePath,
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"x-real-ip"},
			IPHeaderStrategy:            IPHeaderStrategyCheckAll,
			RequestHeaderEnrich:         map[string]string{"X-Geo-Asn": "not-a-key"},
		}, pluginName)
		if err == nil {
			t.Error("expected error for unknown requestHeaderEnrich key")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil")
		}
		if err != nil && !strings.Contains(err.Error(), "unknown requestHeaderEnrich") {
			t.Errorf("expected unknown key error, got: %v", err)
		}
	})

	t.Run("EmptyIPHeaders", func(t *testing.T) {
		plugin, err := New(context.TODO(), &noopHandler{}, &Config{
			Enabled:                     true,
			Ip2locationDatabaseFilePath: dbFilePath,
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{}, // Empty slice should be rejected
			IPHeaderStrategy:            IPHeaderStrategyCheckAll,
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
			Enabled:                     true,
			Ip2locationDatabaseFilePath: dbFilePath,
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"custom-ip-header", "another-ip-header"},
			IPHeaderStrategy:            IPHeaderStrategyCheckAll,
			AllowedCountries:            []string{"AU"},
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
		ip2location.CleanupFactories()
		defer ip2location.CleanupFactories()

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
			Enabled:                     true,
			Ip2locationDatabaseFilePath: "", // Empty - should fallback to environment variable
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"x-real-ip"},
			IPHeaderStrategy:            IPHeaderStrategyCheckAll,
		}, pluginName)

		if err != nil {
			t.Errorf("expected no error when using environment variable fallback, but got: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin not to be nil when using environment variable fallback")
		}

		// Verify the plugin works by testing a lookup
		if plugin != nil {
			rec, err := plugin.(*Plugin).Lookup("8.8.8.8")
			if err != nil {
				t.Errorf("expected successful lookup with env var database, but got: %v", err)
			}
			if rec.Country != "US" {
				t.Errorf("expected country US for 8.8.8.8, got %s", rec.Country)
			}
		}
	})

	// NEW: Test environment variable fallback when bad filepath is provided
	t.Run("BadFilePath_FallbackToEnvironmentVariable", func(t *testing.T) {
		// Cleanup any existing factories to avoid conflicts
		ip2location.CleanupFactories()
		defer ip2location.CleanupFactories()

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
			Enabled:                     true,
			Ip2locationDatabaseFilePath: badDBPath, // Bad path
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"x-real-ip"},
			IPHeaderStrategy:            IPHeaderStrategyCheckAll,
		}, pluginName)

		if err != nil {
			t.Errorf("expected no error when environment variable is valid, but got: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin not to be nil when environment variable is valid")
		}

		// Verify the plugin is using the database from the environment variable
		if plugin != nil {
			factory, err := ip2location.GetDatabaseFactory(&ip2location.DatabaseConfig{
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
		ip2location.CleanupFactories()
		defer ip2location.CleanupFactories()

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
			Enabled:                     true,
			Ip2locationDatabaseFilePath: badDBPath, // Bad path
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"x-real-ip"},
			IPHeaderStrategy:            IPHeaderStrategyCheckAll,
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
		ip2location.CleanupFactories()
		defer ip2location.CleanupFactories()

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
			Enabled:                     true,
			Ip2locationDatabaseFilePath: "", // Empty - should fail without environment variable
			DisallowedStatusCode:        http.StatusForbidden,
			IPHeaders:                   []string{"x-real-ip"},
			IPHeaderStrategy:            IPHeaderStrategyCheckAll,
		}, pluginName)

		if err == nil {
			t.Error("expected error when no filepath and no environment variable are provided, but got none")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil when no filepath and no environment variable are provided")
		}
	})
}

func TestApplyDeprecatedIP2LocationSettings(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("maps old when new empty", func(t *testing.T) {
		cfg := &Config{
			DatabaseFilePath:        "./old.bin",
			DatabaseAutoUpdate:      true,
			DatabaseAutoUpdateDir:   "/old-dir",
			DatabaseAutoUpdateToken: "tok",
			DatabaseAutoUpdateCode:  "DB8BINIPV6",
		}
		applyDeprecatedIP2LocationSettings(cfg, logger)
		if cfg.Ip2locationDatabaseFilePath != "./old.bin" {
			t.Errorf("path: got %q", cfg.Ip2locationDatabaseFilePath)
		}
		if !cfg.Ip2locationDatabaseAutoUpdate {
			t.Error("expected auto-update to map from deprecated field")
		}
		if cfg.Ip2locationDatabaseAutoUpdateDir != "/old-dir" {
			t.Errorf("dir: got %q", cfg.Ip2locationDatabaseAutoUpdateDir)
		}
		if cfg.Ip2locationDatabaseAutoUpdateToken != "tok" {
			t.Errorf("token: got %q", cfg.Ip2locationDatabaseAutoUpdateToken)
		}
		if cfg.Ip2locationDatabaseAutoUpdateCode != "DB8BINIPV6" {
			t.Errorf("code: got %q", cfg.Ip2locationDatabaseAutoUpdateCode)
		}
	})

	t.Run("prefixed wins", func(t *testing.T) {
		cfg := &Config{
			DatabaseFilePath:                   "./old.bin",
			Ip2locationDatabaseFilePath:        "./new.bin",
			DatabaseAutoUpdate:                 false,
			Ip2locationDatabaseAutoUpdate:      true,
			DatabaseAutoUpdateDir:              "/old-dir",
			Ip2locationDatabaseAutoUpdateDir:   "/new-dir",
			DatabaseAutoUpdateToken:            "old-tok",
			Ip2locationDatabaseAutoUpdateToken: "new-tok",
			DatabaseAutoUpdateCode:             "DB1",
			Ip2locationDatabaseAutoUpdateCode:  "DB8BINIPV6",
		}
		applyDeprecatedIP2LocationSettings(cfg, logger)
		if cfg.Ip2locationDatabaseFilePath != "./new.bin" {
			t.Errorf("path: got %q", cfg.Ip2locationDatabaseFilePath)
		}
		if cfg.Ip2locationDatabaseAutoUpdateDir != "/new-dir" {
			t.Errorf("dir: got %q", cfg.Ip2locationDatabaseAutoUpdateDir)
		}
		if cfg.Ip2locationDatabaseAutoUpdateToken != "new-tok" {
			t.Errorf("token: got %q", cfg.Ip2locationDatabaseAutoUpdateToken)
		}
		if cfg.Ip2locationDatabaseAutoUpdateCode != "DB8BINIPV6" {
			t.Errorf("code: got %q", cfg.Ip2locationDatabaseAutoUpdateCode)
		}
		if !cfg.Ip2locationDatabaseAutoUpdate {
			t.Error("expected prefixed auto-update to stay true")
		}
	})

	t.Run("defaults code to DB1", func(t *testing.T) {
		cfg := &Config{}
		applyDeprecatedIP2LocationSettings(cfg, logger)
		if cfg.Ip2locationDatabaseAutoUpdateCode != "DB1" {
			t.Errorf("code: got %q", cfg.Ip2locationDatabaseAutoUpdateCode)
		}
		if cfg.Ip2locationAsnDatabaseAutoUpdateCode != ip2location.DefaultASNDatabaseCode {
			t.Errorf("asn code: got %q", cfg.Ip2locationAsnDatabaseAutoUpdateCode)
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
	if err := fileutils.Copy(dbFilePath, versionedDbPath, true); err != nil {
		t.Fatalf("Failed to copy test database: %v", err)
	}

	t.Run("AutoUpdateEnabled", func(t *testing.T) {
		cfg := &Config{
			Enabled:                           true,
			Ip2locationDatabaseFilePath:       dbFilePath, // Add fallback database path
			Ip2locationDatabaseAutoUpdate:     true,
			Ip2locationDatabaseAutoUpdateDir:  tmpDir,
			Ip2locationDatabaseAutoUpdateCode: "DB1",
			IPHeaderStrategy:                  IPHeaderStrategyCheckAll,
			DisallowedStatusCode:              http.StatusForbidden,
			IPHeaders:                         []string{"x-forwarded-for", "x-real-ip"},
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
		rec, err := p.Lookup("8.8.8.8")
		if err != nil {
			t.Errorf("expected database to be initialized and working, but lookup failed: %v", err)
		}
		if rec.Country != "US" {
			t.Errorf("expected lookup to return US for 8.8.8.8, but got: %s", rec.Country)
		}
	})

	t.Run("AutoUpdateEnabledNoDir", func(t *testing.T) {
		cfg := &Config{
			Enabled:                       true,
			Ip2locationDatabaseAutoUpdate: true,
			DisallowedStatusCode:          http.StatusForbidden,
			IPHeaders:                     []string{"x-forwarded-for", "x-real-ip"},
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
			Enabled:                          true,
			Ip2locationDatabaseAutoUpdate:    true,
			Ip2locationDatabaseAutoUpdateDir: emptyDir,
			DisallowedStatusCode:             http.StatusForbidden,
			Ip2locationDatabaseFilePath:      dbFilePath, // Fall back to default database
			IPHeaders:                        []string{"x-forwarded-for", "x-real-ip"},
			IPHeaderStrategy:                 IPHeaderStrategyCheckAll,
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

package geoblock

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbwrappers"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
)

// pluginRootDir is the module root so New tests can set PluginPathEnv for seed lookup.
func pluginRootDir(t *testing.T) string {
	t.Helper()
	root := moduleRoot()
	if root == "." {
		t.Fatal("go.mod not found")
	}
	return root
}

func TestNew(t *testing.T) {
	t.Setenv(fileutils.PluginPathEnv, pluginRootDir(t))
	t.Run("Disabled", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{Mode: ModeDisabled, IPHeaders: []string{"x-real-ip"}, IPHeaderStrategy: IPHeaderStrategyCheckAll}, pluginName)
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
		plugin, err := newRoute(holdCtx(t), nil, &Config{Mode: ModeEnrichAndBlock, IPHeaders: []string{"x-real-ip"}, IPHeaderStrategy: IPHeaderStrategyCheckAll}, pluginName)
		if err == nil {
			t.Errorf("expected error, but got none")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil, but is not")
		}
	})

	t.Run("Nogeoblock.Config", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, nil, pluginName)
		if err == nil {
			t.Errorf("expected error, but got none")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil, but is not")
		}
	})

	t.Run("InvalidDisallowedStatusCode", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{Mode: ModeEnrichAndBlock, DisallowedStatusCode: -1, IPHeaders: []string{"x-real-ip"}, IPHeaderStrategy: IPHeaderStrategyCheckAll}, pluginName)
		if err == nil {
			t.Errorf("expected error, but got none")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil, but is not")
		}
	})

	t.Run("UnableToFindDatabase", func(t *testing.T) {
		wd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(t.TempDir()); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chdir(wd) })
		t.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", "")
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{Mode: ModeEnrichAndBlock, DisallowedStatusCode: http.StatusForbidden, DatabaseSources: seedCatalog(""), IPHeaders: []string{"x-real-ip"}}, pluginName)
		if err == nil {
			t.Errorf("expected error, but got none")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil, but is not")
		}
	})

	t.Run("InvalidDatabaseVersion", func(t *testing.T) {
		bad := filepath.Join(t.TempDir(), "not-a-bin.BIN")
		if err := os.WriteFile(bad, []byte("not a bin"), 0600); err != nil {
			t.Fatal(err)
		}
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:            ModeEnrichAndBlock,
			DatabaseSources: seedCatalog(bad),
			IPHeaders:       []string{"x-real-ip"},
		}, pluginName)
		if err == nil {
			t.Errorf("expected error about invalid database version, but got none")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil, but is not")
		}
	})

	t.Run("UnknownVendorFails", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode: ModeEnrichAndBlock,
			DatabaseSources: map[string]DatabaseSource{
				DefaultIP2LocationCatalogKey: {Enabled: boolPtr(false), Vendor: VendorIP2Location, DatabaseType: "bin"},
				"seed":                       {Path: dbFilePath, Vendor: "no-such-vendor", DatabaseType: "bin"},
			},
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}, pluginName)
		if err == nil {
			t.Errorf("expected error about unknown vendor, but got none")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil, but is not")
		}
		if err != nil && !strings.Contains(err.Error(), "vendor") {
			t.Errorf("expected vendor error, got: %v", err)
		}
	})

	t.Run("ExplicitMaxMindCatalog", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                 ModeEnrichAndBlock,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
			DatabaseSources:      shippedMaxMindOnly(maxmindFilePath),
		}, pluginName)
		if err != nil {
			t.Errorf("expected no error with maxmind catalog row, but got: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin not to be nil")
		}
	})

	t.Run("ExplicitIPinfoCatalog", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                 ModeEnrichAndBlock,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
			DatabaseSources:      shippedIPinfoOnly(),
		}, pluginName)
		if err != nil {
			t.Errorf("expected no error with ipinfo catalog row, but got: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin not to be nil")
		}
	})

	t.Run("IPinfoEmptyPathUsesBundledMMDB", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                 ModeEnrichAndBlock,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
			CountryHeader:        "x-country-code",
			DatabaseSources:      shippedIPinfoOnly(),
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

	t.Run("IPinfoURLWithoutDirUsesTemp", func(t *testing.T) {
		cfg := &Config{
			Mode: ModeEnrichAndBlock,
			DatabaseSources: map[string]DatabaseSource{
				DefaultIP2LocationCatalogKey: {Enabled: boolPtr(false), Vendor: VendorIP2Location, DatabaseType: "bin"},
				"lite":                       {URL: "https://example.com/geo.mmdb", DatabaseType: "mmdb", Archive: "none", Vendor: VendorIPinfo, DefaultFile: "ipinfo_lite.mmdb"},
			},
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
		if err != nil {
			t.Fatalf("empty dir should WARN and use temp: %v", err)
		}
		if plugin == nil {
			t.Fatal("expected plugin")
		}
		if !strings.Contains(cfg.DatabaseAutoUpdateDir, defaultAutoUpdateDirName) {
			t.Errorf("temp dir: got %q", cfg.DatabaseAutoUpdateDir)
		}
	})

	t.Run("IPinfoEmptyMapUsesSeed", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                 ModeEnrichAndBlock,
			DatabaseSources:      shippedIPinfoOnly(),
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}, pluginName)
		if err != nil {
			t.Errorf("empty download map must use seed, got: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin not to be nil")
		}
	})

	t.Run("FreeCatalogKeySucceeds", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode: ModeEnrichAndBlock,
			DatabaseSources: map[string]DatabaseSource{
				"city":                       {URL: "https://example.com/city.BIN", DatabaseType: "bin", Archive: "none", Vendor: VendorIP2Location, Enabled: boolPtr(false)},
				DefaultIP2LocationCatalogKey: {Path: dbFilePath, DatabaseType: "bin", Vendor: VendorIP2Location},
			},
			DatabaseAutoUpdateDir: t.TempDir(),
			DisallowedStatusCode:  http.StatusForbidden,
			IPHeaders:             []string{"x-real-ip"},
			IPHeaderStrategy:      IPHeaderStrategyCheckAll,
		}, pluginName)
		if err != nil {
			t.Fatalf("unused catalog key must not fail New: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin not to be nil")
		}
	})

	t.Run("DefaultCatalogInserted", func(t *testing.T) {
		cfg := &Config{DatabaseSources: map[string]DatabaseSource{}}
		insertReservedCatalog(cfg)
		row, ok := cfg.DatabaseSources[DefaultIP2LocationCatalogKey]
		if !ok {
			t.Fatal("expected default_ip2location catalog row")
		}
		if row.URL != DefaultIP2LocationLiteURL {
			t.Errorf("default URL: got %q", row.URL)
		}
		if row.DatabaseType != "bin" || row.Archive != "zip" {
			t.Errorf("default type/archive: got %q %q", row.DatabaseType, row.Archive)
		}
		if row.Vendor != VendorIP2Location || row.DefaultFile != DefaultIP2LocationGeoFile {
			t.Errorf("vendor/defaultFile: got %q %q", row.Vendor, row.DefaultFile)
		}
		if _, ok := cfg.DatabaseSources[DefaultIPinfoCatalogKey]; !ok {
			t.Error("expected default_ipinfo")
		}
		if _, ok := cfg.DatabaseSources[DefaultMaxmindCatalogKey]; !ok {
			t.Error("expected default_maxmind")
		}
	})

	t.Run("OperatorDefaultCatalogKept", func(t *testing.T) {
		custom := "https://example.com/custom.BIN.ZIP"
		cfg := &Config{
			Mode:                 ModeEnrichAndBlock,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
			DatabaseSources: map[string]DatabaseSource{
				DefaultIP2LocationCatalogKey: {URL: custom, DatabaseType: "bin", Archive: "zip", Path: dbFilePath, Vendor: VendorIP2Location},
			},
		}
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if plugin == nil {
			t.Fatal("expected plugin")
		}
		if got := cfg.DatabaseSources[DefaultIP2LocationCatalogKey].URL; got != custom {
			t.Errorf("operator row replaced: got %q", got)
		}
	})

	t.Run("DefaultGeoliteCatalogInserted", func(t *testing.T) {
		cfg := &Config{DatabaseSources: map[string]DatabaseSource{}}
		insertReservedCatalog(cfg)
		row, ok := cfg.DatabaseSources[DefaultGeoliteCatalogKey]
		if !ok {
			t.Fatal("expected default_geolite catalog row")
		}
		if row.URL != DefaultGeoliteURL {
			t.Errorf("default URL: got %q", row.URL)
		}
		if row.DatabaseType != "mmdb" || row.Archive != "none" {
			t.Errorf("default type/archive: got %q %q", row.DatabaseType, row.Archive)
		}
		if sourceEnabled(row) {
			t.Error("default_geolite must be disabled")
		}
	})

	t.Run("OperatorDefaultGeoliteKept", func(t *testing.T) {
		custom := "https://example.com/custom.mmdb"
		cfg := &Config{
			Mode:                 ModeEnrichAndBlock,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
			DatabaseSources: map[string]DatabaseSource{
				DefaultIP2LocationCatalogKey: {Enabled: boolPtr(false), Vendor: VendorIP2Location, DatabaseType: "bin"},
				DefaultGeoliteCatalogKey:     {URL: custom, DatabaseType: "mmdb", Archive: "none", Path: maxmindFilePath, Vendor: VendorMaxMind},
			},
		}
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if plugin == nil {
			t.Fatal("expected plugin")
		}
		if got := cfg.DatabaseSources[DefaultGeoliteCatalogKey].URL; got != custom {
			t.Errorf("operator row replaced: got %q", got)
		}
	})

	t.Run("PointerTypeMismatchFails", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                  ModeEnrichAndBlock,
			DisallowedStatusCode:  http.StatusForbidden,
			IPHeaders:             []string{"x-real-ip"},
			IPHeaderStrategy:      IPHeaderStrategyCheckAll,
			DatabaseAutoUpdateDir: t.TempDir(),
			DatabaseSources: map[string]DatabaseSource{
				DefaultIP2LocationCatalogKey: {Enabled: boolPtr(false), Vendor: VendorIP2Location, DatabaseType: "bin"},
				"lite":                       {URL: "https://example.com/geo.mmdb", DatabaseType: "mmdb", Archive: "none", Vendor: VendorIP2Location},
			},
		}, pluginName)
		if err == nil {
			t.Fatal("expected New to fail when ip2location vendor is pointed at mmdb")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil")
		}
	})

	t.Run("UnknownDatabaseTypeFails", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode: ModeEnrichAndBlock,
			DatabaseSources: map[string]DatabaseSource{
				"litezip": {URL: "https://example.com/db.zip", DatabaseType: "csv", Archive: "zip"},
			},
			DatabaseAutoUpdateDir: t.TempDir(),
			DisallowedStatusCode:  http.StatusForbidden,
			IPHeaders:             []string{"x-real-ip"},
			IPHeaderStrategy:      IPHeaderStrategyCheckAll,
		}, pluginName)
		if err == nil {
			t.Fatal("expected New to fail for unknown databaseType")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil")
		}
	})

	t.Run("UnknownArchiveFails", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode: ModeEnrichAndBlock,
			DatabaseSources: map[string]DatabaseSource{
				"litezip": {URL: "https://example.com/db.zip", DatabaseType: "bin", Archive: "rar"},
			},
			DatabaseAutoUpdateDir: t.TempDir(),
			DisallowedStatusCode:  http.StatusForbidden,
			IPHeaders:             []string{"x-real-ip"},
			IPHeaderStrategy:      IPHeaderStrategyCheckAll,
		}, pluginName)
		if err == nil {
			t.Fatal("expected New to fail for unknown archive")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil")
		}
	})

	t.Run("ExtensionlessURLWithoutArchiveFails", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode: ModeEnrichAndBlock,
			DatabaseSources: map[string]DatabaseSource{
				"tok": {URL: "https://example.com/download", DatabaseType: "bin"},
			},
			DatabaseAutoUpdateDir: t.TempDir(),
			DisallowedStatusCode:  http.StatusForbidden,
			IPHeaders:             []string{"x-real-ip"},
			IPHeaderStrategy:      IPHeaderStrategyCheckAll,
		}, pluginName)
		if err == nil {
			t.Fatal("expected New to fail when archive is empty and the URL has no extension")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil")
		}
		if strings.Contains(err.Error(), "example.com") {
			t.Errorf("error leaked URL: %v", err)
		}
	})

	t.Run("GeoNamedKeyIsOrdinary", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode: ModeEnrichAndBlock,
			DatabaseSources: map[string]DatabaseSource{
				// Path only: a URL would GET into t.TempDir() and fail Linux cleanup.
				DefaultIP2LocationCatalogKey: {Enabled: boolPtr(false), Vendor: VendorIP2Location, DatabaseType: "bin"},
				"geo":                        {Path: dbFilePath, DatabaseType: "bin", Vendor: VendorIP2Location},
			},
			DatabaseAutoUpdateDir: t.TempDir(),
			DisallowedStatusCode:  http.StatusForbidden,
			IPHeaders:             []string{"x-real-ip"},
			IPHeaderStrategy:      IPHeaderStrategyCheckAll,
		}, pluginName)
		if err != nil {
			t.Fatalf("geo catalog key is ordinary: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin not to be nil")
		}
	})

	t.Run("DefaultIP2LocationCatalog", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                 ModeEnrichAndBlock,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}, pluginName)
		if err != nil {
			t.Errorf("expected no error with default catalog, but got: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin not to be nil")
		}
	})

	t.Run("EmptyMapLeavesASNEmpty", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                 ModeEnrichAndBlock,
			DatabaseSources:      map[string]DatabaseSource{},
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}, pluginName)
		if err != nil {
			t.Errorf("empty download map must not fail New, got: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin not to be nil")
		}
		rec, err := plugin.(*Route).Lookup("8.8.8.8")
		if err != nil {
			t.Errorf("Lookup: %v", err)
		}
		if rec.Asn != "" {
			t.Errorf("expected empty ASN without asn URL or path, got %q", rec.Asn)
		}
	})

	t.Run("RequestHeaderEnrichIspDomainKeysAllowed", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                 ModeEnrichAndBlock,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
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
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                 ModeEnrichAndBlock,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
			RequestHeaderEnrich:  map[string]string{"X-Geo-Asn": "asn"},
		}, pluginName)
		if err != nil {
			t.Errorf("expected asn enrich key to be allowed, got: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin not to be nil")
		}
	})

	t.Run("CountryHeaderFoldsIntoRequestHeaderEnrich", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                 ModeEnrichAndBlock,
			CountryHeader:        "X-IPCountry",
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}, pluginName)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		p := plugin.(*Route)
		if got := p.requestHeaderEnrich[http.CanonicalHeaderKey("X-IPCountry")]; got != dbprovider.MetaCountry {
			t.Errorf("folded countryHeader: got %q want %s", got, dbprovider.MetaCountry)
		}
	})

	t.Run("RequestHeaderEnrichWinsOverCountryHeader", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                 ModeEnrichAndBlock,
			CountryHeader:        "X-Geo-Country",
			RequestHeaderEnrich:  map[string]string{"X-Geo-Country": "city"},
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}, pluginName)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		p := plugin.(*Route)
		if got := p.requestHeaderEnrich[http.CanonicalHeaderKey("X-Geo-Country")]; got != dbprovider.MetaCity {
			t.Errorf("explicit enrich should win: got %q want %s", got, dbprovider.MetaCity)
		}
	})

	t.Run("UnknownRequestHeaderEnrichKey", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                 ModeEnrichAndBlock,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
			RequestHeaderEnrich:  map[string]string{"X-Geo-Asn": "not-a-key"},
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
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                 ModeEnrichAndBlock,
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
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                 ModeEnrichAndBlock,
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
		dbwrappers.Reset()
		defer dbwrappers.Reset()

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
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                 ModeEnrichAndBlock,
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
			rec, err := plugin.(*Route).Lookup("8.8.8.8")
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
		dbwrappers.Reset()
		defer dbwrappers.Reset()

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

		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                 ModeEnrichAndBlock,
			DatabaseSources:      seedCatalog("/nonexistent/path/bad-database.bin"),
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

		if plugin != nil {
			rec, err := plugin.(*Route).Lookup("8.8.8.8")
			if err != nil {
				t.Errorf("expected successful lookup with env var database, but got: %v", err)
			}
			if rec.Country != "US" {
				t.Errorf("expected country US for 8.8.8.8, got %s", rec.Country)
			}
		}
	})

	// NEW: Test error when both filepath and environment variable are bad
	t.Run("BadFilePath_BadEnvironmentVariable_ShouldError", func(t *testing.T) {
		// Cleanup any existing factories to avoid conflicts
		dbwrappers.Reset()
		defer dbwrappers.Reset()

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

		bad := filepath.Join(t.TempDir(), "not-a-bin.BIN")
		if err := os.WriteFile(bad, []byte("not a bin"), 0600); err != nil {
			t.Fatal(err)
		}
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                 ModeEnrichAndBlock,
			DatabaseSources:      seedCatalog(bad),
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}, pluginName)

		if err == nil {
			t.Error("expected error when catalog path is an invalid BIN, but got none")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil when catalog path is an invalid BIN")
		}
	})

	// NEW: Test error when no filepath and no environment variable are provided
	t.Run("NoFilePath_NoEnvironmentVariable_ShouldError", func(t *testing.T) {
		// Cleanup any existing factories to avoid conflicts
		dbwrappers.Reset()
		defer dbwrappers.Reset()

		// Ensure no environment variable is set
		originalEnv := os.Getenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH")
		os.Unsetenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH")
		defer func() {
			if originalEnv != "" {
				os.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", originalEnv)
			}
		}()

		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                  ModeEnrichAndBlock,
			DatabaseAutoUpdateDir: t.TempDir(),
			DisallowedStatusCode:  http.StatusForbidden,
			IPHeaders:             []string{"x-real-ip"},
			IPHeaderStrategy:      IPHeaderStrategyCheckAll,
		}, pluginName)

		if err == nil {
			t.Error("expected error when TRAEFIK_PLUGIN_GEOBLOCK_PATH is unset and no catalog path")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil without plugin-root env or catalog path")
		}
	})
}

func TestCreateConfig_DatabaseSources(t *testing.T) {
	cfg := CreateConfig()
	if cfg.DatabaseSources == nil {
		t.Fatal("CreateConfig must initialize DatabaseSources")
	}
	if len(cfg.DatabaseSources) != 0 {
		t.Errorf("expected empty map, got %d entries", len(cfg.DatabaseSources))
	}
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
	versionedDbPath := filepath.Join(tmpDir, "20240301_geo.BIN")
	if err := fileutils.Copy(dbFilePath, versionedDbPath, true); err != nil {
		t.Fatalf("Failed to copy test database: %v", err)
	}

	t.Run("AutoUpdateEnabled", func(t *testing.T) {
		cfg := &Config{
			Mode: ModeEnrichAndBlock,
			DatabaseSources: map[string]DatabaseSource{
				DefaultIP2LocationCatalogKey: {Enabled: boolPtr(false), Vendor: VendorIP2Location, DatabaseType: "bin"},
				"geo":                        {Vendor: VendorIP2Location, DatabaseType: "bin"},
			},
			DatabaseAutoUpdateDir: tmpDir,
			IPHeaderStrategy:      IPHeaderStrategyCheckAll,
			DisallowedStatusCode:  http.StatusForbidden,
			IPHeaders:             []string{"x-forwarded-for", "x-real-ip"},
		}

		plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
		if err != nil {
			t.Errorf("expected no error, but got: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin to not be nil")
		}

		// Verify that the database is working by testing a lookup
		p := plugin.(*Route)
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
			Mode: ModeEnrichAndBlock,
			DatabaseSources: map[string]DatabaseSource{
				DefaultIP2LocationCatalogKey: {Enabled: boolPtr(false), Vendor: VendorIP2Location, DatabaseType: "bin"},
				"litezip":                    {URL: "https://example.com/geo.ZIP", DatabaseType: "bin", Archive: "zip", Vendor: VendorIP2Location},
			},
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-forwarded-for", "x-real-ip"},
			// Deliberately omit DatabaseAutoUpdateDir AND DatabaseFilePath
		}

		plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
		if err == nil {
			t.Error("expected error about missing database path, but got none")
		}
		if plugin != nil {
			t.Error("expected plugin to be nil")
		}
	})

	t.Run("AutoUpdateEnabledEmptyDir", func(t *testing.T) {
		t.Setenv(fileutils.PluginPathEnv, pluginRootDir(t))
		emptyDir, err := os.MkdirTemp("", "geoblock-empty-*")
		if err != nil {
			t.Fatalf("Failed to create empty temp dir: %v", err)
		}
		defer os.RemoveAll(emptyDir)

		cfg := &Config{
			Mode:                  ModeEnrichAndBlock,
			DatabaseAutoUpdateDir: emptyDir,
			DisallowedStatusCode:  http.StatusForbidden,
			IPHeaders:             []string{"x-forwarded-for", "x-real-ip"},
			IPHeaderStrategy:      IPHeaderStrategyCheckAll,
		}

		plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
		if err != nil {
			t.Errorf("expected no error when falling back to default database, but got: %v", err)
		}
		if plugin == nil {
			t.Error("expected plugin to not be nil when falling back to default database")
		}
	})
}

package geoblock

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPrepare_ModeAndCountryHeader(t *testing.T) {
	t.Run("unknown mode fails", func(t *testing.T) {
		err := Prepare(&Config{
			Mode:                 "full",
			CountryHeader:        "X-IPCountry",
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}, pluginName)
		if err == nil {
			t.Fatal("expected invalid mode to fail")
		}
	})

	t.Run("empty countryHeader defaults to X-IPCountry", func(t *testing.T) {
		cfg := &Config{
			Mode:                 ModeBlock,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}
		if err := Prepare(cfg, pluginName); err != nil {
			t.Fatalf("Prepare: %v", err)
		}
		if cfg.CountryHeader != DefaultCountryHeader {
			t.Errorf("CountryHeader %q want %q", cfg.CountryHeader, DefaultCountryHeader)
		}
	})

	t.Run("second country enrich header fails", func(t *testing.T) {
		err := Prepare(&Config{
			Mode:                 ModeEnrich,
			CountryHeader:        "X-IPCountry",
			RequestHeaderEnrich:  map[string]string{"X-Other": "country"},
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}, pluginName)
		if err == nil {
			t.Fatal("expected conflicting country enrich header to fail")
		}
	})
}

func TestMode_BlockDoesNotOpenDatabase(t *testing.T) {
	plugin, err := newTestPlugin(holdCtx(t), &Config{
		Mode:                 ModeBlock,
		CountryHeader:        "X-IPCountry",
		BlockedCountries:     []string{"US"},
		DefaultAllow:         true,
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-real-ip"},
		IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		BanIfError:           true,
	}, pluginName)
	if err != nil {
		t.Fatalf("NewCore: %v", err)
	}
	if plugin.db != nil {
		t.Fatal("block mode opened a DatabaseProvider")
	}

	req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
	req.Header.Set("X-Real-IP", "8.8.8.8")
	req.Header.Set("X-IPCountry", "US")
	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req, &noopHandler{})
	if rr.Code != http.StatusForbidden {
		t.Errorf("status %d want %d", rr.Code, http.StatusForbidden)
	}
	if req.Header.Get("X-IPCountry") != "US" {
		t.Errorf("block overwrote inbound country: %q", req.Header.Get("X-IPCountry"))
	}
}

func TestMode_EnrichDoesNotBlock(t *testing.T) {
	plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
		Mode:                 ModeEnrich,
		CountryHeader:        "X-IPCountry",
		DatabaseSources:      seedCatalog(dbFilePath),
		BlockedCountries:     []string{"US"},
		DefaultAllow:         false,
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-real-ip"},
		IPHeaderStrategy:     IPHeaderStrategyCheckAll,
	}, pluginName)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
	req.Header.Set("X-Real-IP", "8.8.8.8")
	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)
	if rr.Code != http.StatusTeapot {
		t.Errorf("enrich blocked: status %d", rr.Code)
	}
	if req.Header.Get("X-IPCountry") != "US" {
		t.Errorf("country header: got %q", req.Header.Get("X-IPCountry"))
	}
}

func TestMode_BlockCIDRWithoutDatabase(t *testing.T) {
	plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
		Mode:                 ModeBlock,
		CountryHeader:        "X-IPCountry",
		BlockedIPBlocks:      []string{"8.8.8.8/32"},
		DefaultAllow:         true,
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-real-ip"},
		IPHeaderStrategy:     IPHeaderStrategyCheckAll,
	}, pluginName)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
	req.Header.Set("X-Real-IP", "8.8.8.8")
	req.Header.Set("X-IPCountry", "DE")
	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("CIDR block status %d", rr.Code)
	}
}

func TestMode_BlockPrivateHeaderFollowsAllowPrivate(t *testing.T) {
	t.Run("allowPrivate true", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                 ModeBlock,
			CountryHeader:        "X-IPCountry",
			AllowPrivate:         true,
			DefaultAllow:         false,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}, pluginName)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		req.Header.Set("X-Real-IP", "8.8.8.8")
		req.Header.Set("X-IPCountry", PrivateIpCountryAlias)
		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)
		if rr.Code != http.StatusTeapot {
			t.Errorf("PRIVATE + allowPrivate status %d want pass", rr.Code)
		}
	})

	t.Run("allowPrivate false", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                 ModeBlock,
			CountryHeader:        "X-IPCountry",
			AllowPrivate:         false,
			DefaultAllow:         true,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}, pluginName)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		req.Header.Set("X-Real-IP", "8.8.8.8")
		req.Header.Set("X-IPCountry", PrivateIpCountryAlias)
		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("PRIVATE without allowPrivate status %d want block", rr.Code)
		}
	})

	t.Run("CIDR still wins", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                 ModeBlock,
			CountryHeader:        "X-IPCountry",
			AllowPrivate:         true,
			BlockedIPBlocks:      []string{"8.8.8.8/32"},
			DefaultAllow:         true,
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}, pluginName)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		req.Header.Set("X-Real-IP", "8.8.8.8")
		req.Header.Set("X-IPCountry", PrivateIpCountryAlias)
		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("CIDR vs PRIVATE status %d want block", rr.Code)
		}
	})
}

func TestMode_BlockMissingCountryUsesBanIfError(t *testing.T) {
	plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
		Mode:                 ModeBlock,
		CountryHeader:        "X-IPCountry",
		DefaultAllow:         true,
		BanIfError:           true,
		DisallowedStatusCode: http.StatusForbidden,
		IPHeaders:            []string{"x-real-ip"},
		IPHeaderStrategy:     IPHeaderStrategyCheckAll,
	}, pluginName)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
	req.Header.Set("X-Real-IP", "8.8.8.8")
	rr := httptest.NewRecorder()
	plugin.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("missing country status %d", rr.Code)
	}
}

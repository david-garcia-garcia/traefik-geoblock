package geoblock

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
)

// TestNormalizeMode_EmptyIsEnrichAndBlock checks empty and whitespace mode become enrichandblock.
func TestNormalizeMode_EmptyIsEnrichAndBlock(t *testing.T) {
	if got := NormalizeMode(""); got != ModeEnrichAndBlock {
		t.Errorf("empty: %q want %q", got, ModeEnrichAndBlock)
	}
	if got := NormalizeMode("  "); got != ModeEnrichAndBlock {
		t.Errorf("whitespace: %q want %q", got, ModeEnrichAndBlock)
	}
	if got := NormalizeMode(ModeDisabled); got != ModeDisabled {
		t.Errorf("disabled: %q want %q", got, ModeDisabled)
	}
}

func TestPrepare_ModeAndCountryHeader(t *testing.T) {
	t.Run("empty mode is enrichandblock", func(t *testing.T) {
		for _, mode := range []string{"", "  "} {
			cfg := &Config{
				Mode:                 mode,
				DisallowedStatusCode: http.StatusForbidden,
				IPHeaders:            []string{"x-real-ip"},
				IPHeaderStrategy:     IPHeaderStrategyCheckAll,
			}
			if err := Prepare(cfg, pluginName); err != nil {
				t.Fatalf("Prepare(%q): %v", mode, err)
			}
			if cfg.Mode != ModeEnrichAndBlock {
				t.Errorf("Mode %q want %q", cfg.Mode, ModeEnrichAndBlock)
			}
		}
	})

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

	t.Run("extra country enrich header is allowed", func(t *testing.T) {
		err := Prepare(&Config{
			Mode:                 ModeEnrich,
			CountryHeader:        "X-IPCountry",
			RequestHeaderEnrich:  map[string]string{"X-Geo-Country": "country"},
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-real-ip"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}, pluginName)
		if err != nil {
			t.Fatalf("Prepare: %v", err)
		}
	})
}

// TestMode_EmptyOpensCatalog checks omitted mode opens catalog sources as enrichandblock.
func TestMode_EmptyOpensCatalog(t *testing.T) {
	plugin, err := newTestPlugin(holdCtx(t), &Config{
		CountryHeader:        "X-IPCountry",
		DatabaseSources:      seedCatalog(dbFilePath),
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
	if plugin.db == nil {
		t.Fatal("empty mode did not open catalog sources")
	}
	if plugin.mode != ModeEnrichAndBlock {
		t.Errorf("mode %q want %q", plugin.mode, ModeEnrichAndBlock)
	}
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
		t.Fatal("block mode opened catalog sources")
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

func TestMode_ExtraCountryEnrichHeaderWritesBoth(t *testing.T) {
	plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
		Mode:                 ModeEnrich,
		CountryHeader:        "X-IPCountry",
		RequestHeaderEnrich:  map[string]string{"X-Geo-Country": "country"},
		DatabaseSources:      seedCatalog(dbFilePath),
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
		t.Errorf("status %d", rr.Code)
	}
	if req.Header.Get("X-IPCountry") != "US" {
		t.Errorf("countryHeader: got %q", req.Header.Get("X-IPCountry"))
	}
	if req.Header.Get("X-Geo-Country") != "US" {
		t.Errorf("extra enrich country: got %q", req.Header.Get("X-Geo-Country"))
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

// TestMode_UnresolvedPublicIPFollowsDefaultAllow covers a public address that the
// enabled sources ran against and did not resolve. That is not a private hop, so it
// must not inherit the PRIVATE enrich default and must fall through to the country
// rules, where defaultAllow decides. The shipped MaxMind test database resolves
// 81.2.69.142 (GB) and 89.160.20.112 (SE) and nothing else used here.
func TestMode_UnresolvedPublicIPFollowsDefaultAllow(t *testing.T) {
	// banIfError is true here because CreateConfig ships it true (config.go) while
	// Prepare leaves it alone, so a Config literal would otherwise test the opposite
	// of what operators run -- and banIfError is exactly why the sentinel is XX and
	// not null or "": those route to the banIfError branch in blockFromHeader.
	// Memoised: every construction opens the seed database, and `go test ./...` runs
	// packages in parallel, so building one per subtest adds enough CPU pressure to
	// upset the grace-window tests in pkg/reclaim on a two-core runner.
	built := map[string]http.Handler{}
	newPlugin := func(t *testing.T, defaultAllow bool, strategy string, allowed, blocked []string) http.Handler {
		t.Helper()
		key := fmt.Sprintf("%v|%s|%v|%v", defaultAllow, strategy, allowed, blocked)
		if p, ok := built[key]; ok {
			return p
		}
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                  ModeEnrichAndBlock,
			CountryHeader:         "X-Ipcountry",
			LogStatusDetailHeader: "X-Geoblock-Decision",
			DatabaseSources:       seedCatalog(maxmindFilePath),
			AllowPrivate:          true,
			BanIfError:            true,
			DefaultAllow:          defaultAllow,
			AllowedCountries:      allowed,
			BlockedCountries:      blocked,
			DisallowedStatusCode:  http.StatusForbidden,
			IPHeaders:             []string{"x-forwarded-for"},
			IPHeaderStrategy:      strategy,
		}, pluginName)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		built[key] = plugin
		return plugin
	}
	serve := func(plugin http.Handler, chain string) (int, string, string) {
		req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		req.Header.Set("X-Forwarded-For", chain)
		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)
		return rr.Code, req.Header.Get("X-Ipcountry"), req.Header.Get("X-Geoblock-Decision")
	}

	t.Run("defaultAllow false blocks an unresolved public IP", func(t *testing.T) {
		plugin := newPlugin(t, false, IPHeaderStrategyCheckAll, []string{"GB"}, nil)
		for _, ip := range []string{"203.0.113.7", "198.51.100.1"} {
			code, country, _ := serve(plugin, ip)
			if code != http.StatusForbidden {
				t.Errorf("%s: status %d want %d (country %q)", ip, code, http.StatusForbidden, country)
			}
			if country != UnknownCountryAlias {
				t.Errorf("%s: country %q want %q", ip, country, UnknownCountryAlias)
			}
		}
	})

	t.Run("defaultAllow true still allows an unresolved public IP", func(t *testing.T) {
		plugin := newPlugin(t, true, IPHeaderStrategyCheckAll, nil, nil)
		code, country, _ := serve(plugin, "203.0.113.7")
		if code != http.StatusTeapot {
			t.Errorf("status %d want pass", code)
		}
		if country != UnknownCountryAlias {
			t.Errorf("country %q want %q", country, UnknownCountryAlias)
		}
	})

	t.Run("defaultAllow true with banIfError true still passes, not block:error", func(t *testing.T) {
		// This is why the sentinel is XX. null or "" would reach the banIfError
		// branch in blockFromHeader and ban these instead.
		plugin := newPlugin(t, true, IPHeaderStrategyCheckAll, nil, nil)
		code, country, decision := serve(plugin, "203.0.113.7")
		if code != http.StatusTeapot {
			t.Errorf("status %d want pass (decision %q)", code, decision)
		}
		if country != UnknownCountryAlias {
			t.Errorf("country %q want %q", country, UnknownCountryAlias)
		}
		if decision != LogStatusPass+":"+PhaseDefaultAllow {
			t.Errorf("decision %q want %q", decision, LogStatusPass+":"+PhaseDefaultAllow)
		}
	})

	t.Run("XX can be listed in allowedCountries", func(t *testing.T) {
		plugin := newPlugin(t, false, IPHeaderStrategyCheckAll, []string{"GB", UnknownCountryAlias}, nil)
		code, _, decision := serve(plugin, "203.0.113.7")
		if code != http.StatusTeapot || decision != LogStatusPass+":"+PhaseAllowedCountry {
			t.Errorf("status %d decision %q want pass/%s", code, decision, PhaseAllowedCountry)
		}
	})

	t.Run("XX can be listed in blockedCountries", func(t *testing.T) {
		plugin := newPlugin(t, true, IPHeaderStrategyCheckAll, nil, []string{UnknownCountryAlias})
		code, _, decision := serve(plugin, "203.0.113.7")
		if code != http.StatusForbidden || decision != LogStatusBlock+":"+PhaseBlockedCountry {
			t.Errorf("status %d decision %q want block/%s", code, decision, PhaseBlockedCountry)
		}
	})

	t.Run("a private hop keeps PRIVATE and follows allowPrivate", func(t *testing.T) {
		plugin := newPlugin(t, false, IPHeaderStrategyCheckAll, []string{"GB"}, nil)
		code, country, _ := serve(plugin, "192.168.1.50")
		if code != http.StatusTeapot {
			t.Errorf("status %d want pass", code)
		}
		if country != PrivateIpCountryAlias {
			t.Errorf("country %q want %q", country, PrivateIpCountryAlias)
		}
	})

	t.Run("a later hop that resolves still wins", func(t *testing.T) {
		plugin := newPlugin(t, false, IPHeaderStrategyCheckAll, []string{"GB"}, nil)
		code, country, _ := serve(plugin, "203.0.113.7, 81.2.69.142")
		if country != "GB" {
			t.Errorf("country %q want GB", country)
		}
		if code != http.StatusTeapot {
			t.Errorf("status %d want pass", code)
		}
	})

	t.Run("an unresolved hop before a private hop still blocks", func(t *testing.T) {
		plugin := newPlugin(t, false, IPHeaderStrategyCheckAll, []string{"GB"}, nil)
		code, country, _ := serve(plugin, "203.0.113.7, 10.0.0.1")
		if code != http.StatusForbidden {
			t.Errorf("status %d want %d (country %q)", code, http.StatusForbidden, country)
		}
		if country != UnknownCountryAlias {
			t.Errorf("country %q want %q", country, UnknownCountryAlias)
		}
	})

	t.Run("CGNAT and link-local are public, so they resolve to XX", func(t *testing.T) {
		// privateOrLoopback uses net.IP.IsPrivate, which is RFC 1918 only. These
		// ranges were already outside allowPrivate; before this change they kept the
		// PRIVATE default and were allowed anyway.
		plugin := newPlugin(t, false, IPHeaderStrategyCheckAll, []string{"GB"}, nil)
		for _, ip := range []string{"100.64.0.1", "169.254.1.1", "198.18.0.1"} {
			code, country, _ := serve(plugin, ip)
			if country != UnknownCountryAlias {
				t.Errorf("%s: country %q want %q", ip, country, UnknownCountryAlias)
			}
			if code != http.StatusForbidden {
				t.Errorf("%s: status %d want %d", ip, code, http.StatusForbidden)
			}
		}
	})

	t.Run("a bin source miss writes XX, not a vendor dash", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                  ModeEnrichAndBlock,
			CountryHeader:         "X-Ipcountry",
			LogStatusDetailHeader: "X-Geoblock-Decision",
			DatabaseSources:       seedCatalog(dbFilePath),
			AllowPrivate:          true,
			DefaultAllow:          false,
			AllowedCountries:      []string{"GB"},
			DisallowedStatusCode:  http.StatusForbidden,
			IPHeaders:             []string{"x-forwarded-for"},
			IPHeaderStrategy:      IPHeaderStrategyCheckAll,
		}, pluginName)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.7")
		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)
		if got := req.Header.Get("X-Ipcountry"); got != UnknownCountryAlias {
			t.Errorf("country %q want %q", got, UnknownCountryAlias)
		}
		if rr.Code != http.StatusForbidden {
			t.Errorf("status %d want %d", rr.Code, http.StatusForbidden)
		}
	})

	t.Run("XX in allowedCountries covers a BIN miss", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                  ModeEnrichAndBlock,
			CountryHeader:         "X-Ipcountry",
			LogStatusDetailHeader: "X-Geoblock-Decision",
			DatabaseSources:       seedCatalog(dbFilePath),
			AllowPrivate:          true,
			DefaultAllow:          false,
			AllowedCountries:      []string{UnknownCountryAlias},
			DisallowedStatusCode:  http.StatusForbidden,
			IPHeaders:             []string{"x-forwarded-for"},
			IPHeaderStrategy:      IPHeaderStrategyCheckAll,
		}, pluginName)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.7")
		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)
		if rr.Code != http.StatusTeapot {
			t.Errorf("status %d want pass", rr.Code)
		}
		if got := req.Header.Get("X-Geoblock-Decision"); got != LogStatusPass+":"+PhaseAllowedCountry {
			t.Errorf("decision %q want %q", got, LogStatusPass+":"+PhaseAllowedCountry)
		}
	})

	t.Run("a BIN miss does not lock the country for a later hop", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                 ModeEnrichAndBlock,
			CountryHeader:        "X-Ipcountry",
			DatabaseSources:      seedCatalog(dbFilePath),
			AllowPrivate:         true,
			DefaultAllow:         false,
			AllowedCountries:     []string{"US"},
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-forwarded-for"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}, pluginName)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.7, 8.8.8.8")
		rr := httptest.NewRecorder()
		plugin.ServeHTTP(rr, req)
		if got := req.Header.Get("X-Ipcountry"); got != "US" {
			t.Errorf("country %q want US", got)
		}
		if rr.Code != http.StatusTeapot {
			t.Errorf("status %d want pass", rr.Code)
		}
	})

	t.Run("every country enrich header gets XX", func(t *testing.T) {
		plugin, err := newRoute(holdCtx(t), &noopHandler{}, &Config{
			Mode:                 ModeEnrichAndBlock,
			CountryHeader:        "X-Ipcountry",
			RequestHeaderEnrich:  map[string]string{"X-Geo-Country": dbprovider.MetaCountry},
			DatabaseSources:      seedCatalog(maxmindFilePath),
			AllowPrivate:         true,
			DefaultAllow:         false,
			AllowedCountries:     []string{"GB"},
			DisallowedStatusCode: http.StatusForbidden,
			IPHeaders:            []string{"x-forwarded-for"},
			IPHeaderStrategy:     IPHeaderStrategyCheckAll,
		}, pluginName)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/foobar", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.7")
		plugin.ServeHTTP(httptest.NewRecorder(), req)
		for _, header := range []string{"X-Ipcountry", "X-Geo-Country"} {
			if got := req.Header.Get(header); got != UnknownCountryAlias {
				t.Errorf("%s: got %q want %q", header, got, UnknownCountryAlias)
			}
		}
	})

	t.Run("CheckFirstNonePrivate blocks an unresolved public hop after a private one", func(t *testing.T) {
		plugin := newPlugin(t, false, IPHeaderStrategyCheckFirstNonePrivate, []string{"GB"}, nil)
		code, country, _ := serve(plugin, "10.0.0.1, 203.0.113.7")
		if code != http.StatusForbidden {
			t.Errorf("status %d want %d (country %q)", code, http.StatusForbidden, country)
		}
		if country != UnknownCountryAlias {
			t.Errorf("country %q want %q", country, UnknownCountryAlias)
		}
	})
}

// TestMode_UnparseableHopDoesNotBecomeCountry pins that an IP header value the
// plugin cannot parse never reaches countryHeader. Only the ISO country, PRIVATE
// and XX are legal there, so an unresolvable hop is XX like any other, and a
// later hop that does resolve still wins.
func TestMode_UnparseableHopDoesNotBecomeCountry(t *testing.T) {
	tests := []struct {
		name            string
		mode            string
		headerValue     string
		expectedCountry string
	}{
		{
			name:            "a country code in the IP header is not a country",
			mode:            ModeEnrich,
			headerValue:     "DE",
			expectedCountry: UnknownCountryAlias,
		},
		{
			name:            "arbitrary text in the IP header is not a country",
			mode:            ModeEnrich,
			headerValue:     "Norway",
			expectedCountry: UnknownCountryAlias,
		},
		{
			name:            "a later hop wins over an unparseable one",
			mode:            ModeEnrich,
			headerValue:     "DE, 8.8.8.8",
			expectedCountry: "US",
		},
		{
			name:            "enrichandblock behaves the same",
			mode:            ModeEnrichAndBlock,
			headerValue:     "DE, 8.8.8.8",
			expectedCountry: "US",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Mode:                 tt.mode,
				DatabaseSources:      seedCatalog(dbFilePath),
				DefaultAllow:         true,
				AllowPrivate:         true,
				BanIfError:           false,
				DisallowedStatusCode: http.StatusForbidden,
				IPHeaders:            []string{"x-forwarded-for"},
				IPHeaderStrategy:     IPHeaderStrategyCheckAll,
				CountryHeader:        "X-IPCountry",
			}

			plugin, err := newRoute(holdCtx(t), &noopHandler{}, cfg, pluginName)
			if err != nil {
				t.Fatalf("Failed to create plugin: %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-Forwarded-For", tt.headerValue)

			rr := httptest.NewRecorder()
			plugin.ServeHTTP(rr, req)

			if got := req.Header.Get("X-IPCountry"); got != tt.expectedCountry {
				t.Errorf("X-Forwarded-For %q: X-IPCountry %q, want %q", tt.headerValue, got, tt.expectedCountry)
			}
		})
	}
}

// TestMode_LookupErrorCarriesNoCountry pins the second error path: a parseable
// address whose lookup fails must not put the address itself on countryHeader.
func TestMode_LookupErrorCarriesNoCountry(t *testing.T) {
	p := Plugin{db: dbprovider.Bind(func(ip string) (dbprovider.Record, error) {
		return dbprovider.Record{}, fmt.Errorf("source unavailable")
	})}

	rec, err := p.recordForLookup("8.8.8.8")
	if err == nil {
		t.Fatal("expected a lookup error")
	}
	if rec.Country != "" {
		t.Errorf("country %q, want empty: the record is written to countryHeader before err is checked", rec.Country)
	}
}

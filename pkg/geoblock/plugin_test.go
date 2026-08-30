package geoblock

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
)

// newTestPlugin is NewCore plus Close when ctx is done. Tests that do not use the reclaim table.
func newTestPlugin(ctx context.Context, cfg *Config, name string) (*Plugin, error) {
	if cfg == nil {
		return nil, fmt.Errorf("%s: no config provided", name)
	}
	if NormalizeMode(cfg.Mode) != ModeDisabled && strings.TrimSpace(cfg.CountryHeader) == "" {
		cfg.CountryHeader = countryHeaderFromEnrich(cfg.RequestHeaderEnrich)
		if cfg.CountryHeader == "" {
			cfg.CountryHeader = "x-country-code"
		}
	}
	if err := Prepare(cfg, name); err != nil {
		return nil, err
	}
	pluginInstance, err := NewCore(name, cfg)
	if err != nil {
		return nil, err
	}
	context.AfterFunc(ctx, pluginInstance.Close)
	return pluginInstance, nil
}

// newRoute creates one test incarnation and attaches next.
func newRoute(ctx context.Context, next http.Handler, cfg *Config, name string) (http.Handler, error) {
	pluginInstance, err := newTestPlugin(ctx, cfg, name)
	if err != nil {
		return nil, err
	}
	return pluginInstance.ForRoute(next)
}

// holdCtx is a cancelable New context for tests. Background and TODO panic in reclaim.Open.
func holdCtx(tb testing.TB) context.Context {
	tb.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	tb.Cleanup(cancel)
	return ctx
}

func moduleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

var (
	pluginName      = "geoblock"
	dbFilePath      = filepath.Join(moduleRoot(), "seeds", "IP2LOCATION-LITE-DB1.IPV6.BIN")
	db8FilePath     = filepath.Join(moduleRoot(), "testdata", "IP2LOCATION-DB8.BIN")
	ipinfoFilePath  = filepath.Join(moduleRoot(), "seeds", "ipinfo_lite.mmdb")
	maxmindFilePath = filepath.Join(moduleRoot(), "seeds", "GeoIP2-Country-Test.mmdb")
)

// countryHeaderFromEnrich is the requestHeaderEnrich header mapped to country, if any.
func countryHeaderFromEnrich(enrich map[string]string) string {
	for header, key := range enrich {
		if strings.ToLower(strings.TrimSpace(key)) == dbprovider.MetaCountry {
			return header
		}
	}
	return ""
}

func seedCatalog(path string) map[string]DatabaseSource {
	return map[string]DatabaseSource{"seed": {Path: path}}
}

func seedCatalogPair(geo, asn string) map[string]DatabaseSource {
	return map[string]DatabaseSource{
		"geo": {Path: geo},
		"asn": {Path: asn},
	}
}

var fullEnrichHeaders = map[string]string{
	"X-Geo-Country": dbprovider.MetaCountry,
	"X-Geo-Region":  dbprovider.MetaRegion,
	"X-Geo-City":    dbprovider.MetaCity,
	"X-Geo-Isp":     dbprovider.MetaIsp,
	"X-Geo-Domain":  dbprovider.MetaDomain,
	"X-Geo-Asn":     dbprovider.MetaAsn,
}

func requireDB8(tb testing.TB) string {
	tb.Helper()
	if _, err := os.Stat(db8FilePath); err != nil {
		tb.Skip("paid DB8 BIN not present; place testdata/IP2LOCATION-DB8.BIN")
	}
	return db8FilePath
}

func requireASN(tb testing.TB) string {
	tb.Helper()
	if p := os.Getenv("IP2LOCATION_ASN_BIN"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	for _, candidate := range []string{
		filepath.Join(moduleRoot(), "IP2LOCATION-LITE-ASN.IPV6.BIN"),
		filepath.Join(moduleRoot(), "testdata", "IP2LOCATION-LITE-ASN.IPV6.BIN"),
		`D:\IP2LOCATION-LITE-ASN.IPV6.BIN`,
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	tb.Skip("ASN LITE BIN not present; set IP2LOCATION_ASN_BIN or place IP2LOCATION-LITE-ASN.IPV6.BIN")
	return ""
}

type noopHandler struct{}

func (n noopHandler) ServeHTTP(rw http.ResponseWriter, _ *http.Request) {
	rw.WriteHeader(http.StatusTeapot)
}

package ip2location

import (
	"context"
	"log/slog"
	"time"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbwrappers"
)

// DownloadMinAge is how long a dated BIN stays current before GET.
const DownloadMinAge = 30 * 24 * time.Hour

// DatabaseConfig is IP2Location geo and ASN sources plus shared auto-update dir.
type DatabaseConfig struct {
	DatabaseAutoUpdateDir string
	Source                dbsource.Config
	AsnSource             dbsource.Config
}

type provider struct {
	geo *dbwrappers.BIN
	asn *dbwrappers.BIN
}

// New constructs the IP2Location DatabaseProvider (geo factory plus optional ASN).
func New(ctx context.Context, config DatabaseConfig, logger *slog.Logger) (dbprovider.Provider, error) {
	geo, err := dbwrappers.OpenBIN(ctx, geoBINConfig(config), logger)
	if err != nil {
		return nil, err
	}
	asn, err := dbwrappers.OpenBIN(ctx, asnBINConfig(config), logger)
	if err != nil {
		return nil, err
	}
	return &provider{geo: geo, asn: asn}, nil
}

func geoBINConfig(cfg DatabaseConfig) dbwrappers.BINConfig {
	return dbwrappers.BINConfig{
		Dir:             cfg.DatabaseAutoUpdateDir,
		Source:          cfg.Source,
		DefaultFileName: defaultGeoFileName,
		MinAge:          DownloadMinAge,
	}
}

func asnBINConfig(cfg DatabaseConfig) dbwrappers.BINConfig {
	return dbwrappers.BINConfig{
		Dir:             cfg.DatabaseAutoUpdateDir,
		Source:          cfg.AsnSource,
		AllowMissing:    cfg.AsnSource.Path == "",
		DefaultFileName: defaultASNFileName,
		MinAge:          DownloadMinAge,
	}
}

func (p *provider) Lookup(ip string) (dbprovider.Record, error) {
	rec, err := p.geo.Lookup(ip)
	if err != nil {
		return rec, err
	}
	rec.Asn = p.asn.LookupASN(ip)
	return rec, nil
}

func (p *provider) Close() error {
	return nil
}

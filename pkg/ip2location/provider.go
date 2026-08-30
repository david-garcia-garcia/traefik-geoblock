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

// asnOnly is an IP2Location ASN BIN that fills Record.Asn only.
type asnOnly struct {
	asn *dbwrappers.BIN
}

// New constructs the IP2Location geo Lookup. Optional AsnSource is still merged when set (package tests).
func New(ctx context.Context, config DatabaseConfig, logger *slog.Logger) (dbprovider.Provider, error) {
	geo, err := dbwrappers.OpenBIN(ctx, geoBINConfig(config), logger)
	if err != nil {
		return nil, err
	}
	if config.AsnSource.Path == "" && config.AsnSource.URL == "" && config.AsnSource.Key == "" {
		return &provider{geo: geo}, nil
	}
	asn, err := dbwrappers.OpenBIN(ctx, asnBINConfig(config), logger)
	if err != nil {
		return nil, err
	}
	return &provider{geo: geo, asn: asn}, nil
}

// NewASN constructs the IP2Location ASN Lookup (Record.Asn only).
func NewASN(ctx context.Context, config DatabaseConfig, logger *slog.Logger) (dbprovider.Provider, error) {
	asn, err := dbwrappers.OpenBIN(ctx, asnBINConfig(config), logger)
	if err != nil {
		return nil, err
	}
	return &asnOnly{asn: asn}, nil
}

func geoBINConfig(cfg DatabaseConfig) dbwrappers.BINConfig {
	return dbwrappers.BINConfig{
		Dir:             cfg.DatabaseAutoUpdateDir,
		Source:          cfg.Source,
		DefaultFileName: cfg.Source.DefaultFileName,
		MinAge:          DownloadMinAge,
	}
}

func asnBINConfig(cfg DatabaseConfig) dbwrappers.BINConfig {
	return dbwrappers.BINConfig{
		Dir:          cfg.DatabaseAutoUpdateDir,
		Source:       cfg.AsnSource,
		AllowMissing: cfg.AsnSource.Path == "",
		MinAge:       DownloadMinAge,
	}
}

func (p *provider) Lookup(ip string) (dbprovider.Record, error) {
	rec, err := p.geo.Lookup(ip)
	if err != nil {
		return rec, err
	}
	if p.asn != nil {
		rec.Asn = p.asn.LookupASN(ip)
	}
	return rec, nil
}

func (p *provider) Close() error {
	return nil
}

func (p *asnOnly) Lookup(ip string) (dbprovider.Record, error) {
	if p == nil || p.asn == nil {
		return dbprovider.Record{}, nil
	}
	return dbprovider.Record{Asn: p.asn.LookupASN(ip)}, nil
}

func (p *asnOnly) Close() error {
	return nil
}

package ip2location

import (
	"log/slog"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbutils"
)

// provider looks up country/region/city on the geo BIN and ASN on the ASN LITE BIN.
type provider struct {
	geo *DatabaseWrapper
	asn *DatabaseWrapper
}

// New constructs the IP2Location DatabaseProvider (geo factory plus optional ASN).
func New(config DatabaseConfig, logger *slog.Logger) (dbprovider.Provider, error) {
	geoFactory, err := GetDatabaseFactory(geoFactoryConfig(config), logger)
	if err != nil {
		return nil, err
	}

	p := &provider{geo: geoFactory.GetWrapper()}

	asnFactory, err := GetDatabaseFactory(asnFactoryConfig(config), logger)
	if err != nil {
		return nil, err
	}
	p.asn = asnFactory.GetWrapper()
	return p, nil
}

func geoFactoryConfig(cfg DatabaseConfig) *DatabaseConfig {
	return &DatabaseConfig{
		DatabaseFilePath:      cfg.DatabaseFilePath,
		DatabaseAutoUpdateDir: cfg.DatabaseAutoUpdateDir,
		Download:              cfg.Download,
		BinRole:               dbutils.SlotGeo,
	}
}

func asnFactoryConfig(geo DatabaseConfig) *DatabaseConfig {
	return &DatabaseConfig{
		DatabaseFilePath:      geo.AsnDatabaseFilePath,
		DatabaseAutoUpdateDir: geo.DatabaseAutoUpdateDir,
		Download:              geo.AsnDownload,
		BinRole:               dbutils.SlotASN,
		AllowMissing:          geo.AsnDatabaseFilePath == "",
	}
}

func (p *provider) Lookup(ip string) (dbprovider.Record, error) {
	rec, err := p.geo.Lookup(ip)
	if err != nil {
		return rec, err
	}
	rec.Asn = p.asn.lookupASN(ip)
	return rec, nil
}

func (p *provider) Close() error {
	if p.geo != nil {
		_ = p.geo.Close()
	}
	if p.asn != nil {
		_ = p.asn.Close()
	}
	return nil
}

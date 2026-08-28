package ip2location

import (
	"log/slog"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
)

// provider looks up country/region/city on the geo BIN and ASN on the ASN LITE BIN.
type provider struct {
	geo *DatabaseWrapper
	asn *DatabaseWrapper
}

// New constructs the IP2Location DatabaseProvider (geo factory plus optional ASN).
// middleware is the Traefik plugin instance name (logged on factory create).
func New(config DatabaseConfig, logger *slog.Logger, middleware string) (dbprovider.Provider, error) {
	geoCfg := geoFactoryConfig(config)
	geoFactory, err := GetDatabaseFactory(geoCfg, logger, middleware)
	if err != nil {
		return nil, err
	}

	p := &provider{geo: geoFactory.GetWrapper()}

	if config.AsnDatabaseAutoUpdate && config.DatabaseAutoUpdateToken == "" {
		logger.Error("ip2location_asnDatabaseAutoUpdate is true but ip2location_databaseAutoUpdateToken is empty; ASN download skipped (LITE ASN is not on the public CDN)")
	}

	asnFactory, err := GetDatabaseFactory(asnFactoryConfig(config), logger, middleware)
	if err != nil {
		return nil, err
	}
	p.asn = asnFactory.GetWrapper()
	return p, nil
}

func geoFactoryConfig(cfg DatabaseConfig) *DatabaseConfig {
	return &DatabaseConfig{
		DatabaseFilePath:        cfg.DatabaseFilePath,
		DatabaseAutoUpdate:      cfg.DatabaseAutoUpdate,
		DatabaseAutoUpdateDir:   cfg.DatabaseAutoUpdateDir,
		DatabaseAutoUpdateToken: cfg.DatabaseAutoUpdateToken,
		DatabaseAutoUpdateCode:  cfg.DatabaseAutoUpdateCode,
	}
}

func asnFactoryConfig(geo DatabaseConfig) *DatabaseConfig {
	code := geo.AsnDatabaseAutoUpdateCode
	if code == "" {
		code = DefaultASNDatabaseCode
	}
	// ASN LITE is not on download.ip2location.com/lite/. Only download with a token.
	auto := geo.AsnDatabaseAutoUpdate && geo.DatabaseAutoUpdateToken != ""
	return &DatabaseConfig{
		DatabaseFilePath:        geo.AsnDatabaseFilePath,
		DatabaseAutoUpdate:      auto,
		DatabaseAutoUpdateDir:   geo.DatabaseAutoUpdateDir,
		DatabaseAutoUpdateToken: geo.DatabaseAutoUpdateToken,
		DatabaseAutoUpdateCode:  code,
		AllowMissing:            geo.AsnDatabaseFilePath == "",
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

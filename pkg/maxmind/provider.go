package maxmind

import (
	"context"
	"log/slog"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbwrappers"
)

// DownloadMinAge is how long a dated MMDB stays current before GET.
const DownloadMinAge = dbsource.DefaultMinAge

// DatabaseConfig is MaxMind source plus shared auto-update dir.
type DatabaseConfig struct {
	DatabaseAutoUpdateDir string
	Source                dbsource.Config
}

type geoNames struct {
	En string `maxminddb:"en"`
}

type geoCountry struct {
	IsoCode string   `maxminddb:"iso_code"`
	Names   geoNames `maxminddb:"names"`
}

type geoContinent struct {
	Code  string   `maxminddb:"code"`
	Names geoNames `maxminddb:"names"`
}

type geoCity struct {
	Names geoNames `maxminddb:"names"`
}

type geoSubdivision struct {
	IsoCode string `maxminddb:"iso_code"`
}

type mmdbRecord struct {
	Country      geoCountry       `maxminddb:"country"`
	Continent    geoContinent     `maxminddb:"continent"`
	City         geoCity          `maxminddb:"city"`
	Subdivisions []geoSubdivision `maxminddb:"subdivisions"`
}

type provider struct {
	db *dbwrappers.MMDB
}

// New opens the MaxMind DatabaseProvider (thin facade over the MMDB wrapper).
func New(ctx context.Context, config DatabaseConfig, logger *slog.Logger) (dbprovider.Provider, error) {
	db, err := dbwrappers.OpenMMDB(ctx, dbwrappers.MMDBConfig{
		Dir:             config.DatabaseAutoUpdateDir,
		Source:          config.Source,
		DefaultFileName: DefaultSeedFileName,
		MinAge:          DownloadMinAge,
	}, logger)
	if err != nil {
		return nil, err
	}
	return &provider{db: db}, nil
}

func (p *provider) Lookup(ip string) (dbprovider.Record, error) {
	var rec mmdbRecord
	if err := p.db.Lookup(ip, &rec); err != nil {
		return dbprovider.Record{}, err
	}
	region := ""
	if len(rec.Subdivisions) > 0 {
		region = rec.Subdivisions[0].IsoCode
	}
	return dbprovider.Record{
		Country:       rec.Country.IsoCode,
		CountryName:   rec.Country.Names.En,
		Continent:     rec.Continent.Names.En,
		ContinentCode: rec.Continent.Code,
		Region:        region,
		City:          rec.City.Names.En,
	}, nil
}

func (p *provider) Close() error {
	return nil
}

package ipinfo

import (
	"context"
	"log/slog"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbwrappers"
)

// DownloadMinAge is how long a dated MMDB stays current before GET.
const DownloadMinAge = dbsource.DefaultMinAge

// DatabaseConfig is IPinfo source plus shared auto-update dir.
type DatabaseConfig struct {
	DatabaseAutoUpdateDir string
	Source                dbsource.Config
}

type mmdbRecord struct {
	Country       string `maxminddb:"country"`
	CountryCode   string `maxminddb:"country_code"`
	Continent     string `maxminddb:"continent"`
	ContinentCode string `maxminddb:"continent_code"`
	Region        string `maxminddb:"region"`
	City          string `maxminddb:"city"`
	ASN           string `maxminddb:"asn"`
	ASName        string `maxminddb:"as_name"`
	ASDomain      string `maxminddb:"as_domain"`
}

type provider struct {
	db *dbwrappers.MMDB
}

// New opens the IPinfo DatabaseProvider (thin facade over the MMDB wrapper).
func New(ctx context.Context, config DatabaseConfig, logger *slog.Logger) (dbprovider.Provider, error) {
	db, err := dbwrappers.OpenMMDB(ctx, dbwrappers.MMDBConfig{
		Dir:             config.DatabaseAutoUpdateDir,
		Source:          config.Source,
		DefaultFileName: DefaultFileName,
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
	return dbprovider.Record{
		Country:       rec.CountryCode,
		CountryName:   rec.Country,
		Continent:     rec.Continent,
		ContinentCode: rec.ContinentCode,
		Region:        rec.Region,
		City:          rec.City,
		Isp:           rec.ASName,
		Domain:        rec.ASDomain,
		Asn:           rec.ASN,
	}, nil
}

func (p *provider) Close() error {
	return nil
}

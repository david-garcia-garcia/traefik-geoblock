package dbwrappers

import "github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"

// ipinfoRecord is the IPinfo Lite/Core MMDB schema.
type ipinfoRecord struct {
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

// IPinfo is an MMDB queried with IPinfo field tags.
type IPinfo struct {
	mmdb   *MMDB
	fields []string
}

// NewIPinfo returns a Provider that maps IPinfo tags onto Record.
func NewIPinfo(mmdb *MMDB, fields []string) *IPinfo {
	return &IPinfo{mmdb: mmdb, fields: fields}
}

// Lookup returns IPinfo Lite/Core fields for ip, then Keep(fields).
func (s *IPinfo) Lookup(ip string) (dbprovider.Record, error) {
	var rec ipinfoRecord
	if err := s.mmdb.Lookup(ip, &rec); err != nil {
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
	}.Keep(s.fields), nil
}

// Close does not close the shared MMDB.
func (s *IPinfo) Close() error {
	return nil
}

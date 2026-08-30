package dbwrappers

import "github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"

// geoNames is a GeoIP2 localized name map (English only).
type geoNames struct {
	En string `maxminddb:"en"`
}

// geoCountry is the nested GeoIP2 country object.
type geoCountry struct {
	IsoCode string   `maxminddb:"iso_code"`
	Names   geoNames `maxminddb:"names"`
}

// geoContinent is the nested GeoIP2 continent object.
type geoContinent struct {
	Code  string   `maxminddb:"code"`
	Names geoNames `maxminddb:"names"`
}

// geoCity is the nested GeoIP2 city object.
type geoCity struct {
	Names geoNames `maxminddb:"names"`
}

// geoSubdivision is one GeoIP2 subdivision (region).
type geoSubdivision struct {
	IsoCode string `maxminddb:"iso_code"`
}

// geoIP2Record is the nested GeoIP2 / GeoLite2 Country and City schema.
type geoIP2Record struct {
	Country      geoCountry       `maxminddb:"country"`
	Continent    geoContinent     `maxminddb:"continent"`
	City         geoCity          `maxminddb:"city"`
	Subdivisions []geoSubdivision `maxminddb:"subdivisions"`
}

// GeoIP2 is an MMDB queried with nested GeoIP2 / GeoLite2 tags.
type GeoIP2 struct {
	mmdb   *MMDB
	fields []string
}

// NewGeoIP2 returns a Provider that maps GeoIP2 tags onto Record.
func NewGeoIP2(mmdb *MMDB, fields []string) *GeoIP2 {
	return &GeoIP2{mmdb: mmdb, fields: fields}
}

// Lookup returns country.iso_code and related GeoIP2 fields for ip, then Keep(fields).
func (s *GeoIP2) Lookup(ip string) (dbprovider.Record, error) {
	var rec geoIP2Record
	if err := s.mmdb.Lookup(ip, &rec); err != nil {
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
	}.Keep(s.fields), nil
}

// Close does not close the shared MMDB.
func (s *GeoIP2) Close() error {
	return nil
}

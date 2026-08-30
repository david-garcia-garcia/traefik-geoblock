package dbwrappers

import "github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"

// flatMMDB is the flat tag layout used by IPinfo Lite/Core files.
type flatMMDB struct {
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

// nestedMMDB is the nested tag layout used by GeoIP2 / GeoLite2 files.
type nestedMMDB struct {
	Country      nestedCountry       `maxminddb:"country"`
	Continent    nestedContinent     `maxminddb:"continent"`
	City         nestedCity          `maxminddb:"city"`
	Subdivisions []nestedSubdivision `maxminddb:"subdivisions"`
}

type nestedNames struct {
	En string `maxminddb:"en"`
}

type nestedCountry struct {
	IsoCode string      `maxminddb:"iso_code"`
	Names   nestedNames `maxminddb:"names"`
}

type nestedContinent struct {
	Code  string      `maxminddb:"code"`
	Names nestedNames `maxminddb:"names"`
}

type nestedCity struct {
	Names nestedNames `maxminddb:"names"`
}

type nestedSubdivision struct {
	IsoCode string `maxminddb:"iso_code"`
}

// recordFromFlat maps flat MMDB tags onto Record.
func recordFromFlat(rec flatMMDB) dbprovider.Record {
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
	}
}

// recordFromNested maps nested MMDB tags onto Record.
func recordFromNested(rec nestedMMDB) dbprovider.Record {
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
	}
}

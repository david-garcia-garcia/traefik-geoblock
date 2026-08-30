package dbwrappers

import (
	"fmt"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
)

// Named fieldsPreconfigured values (vendor product ids).
const (
	PresetIP2Location     = "ip2location"
	PresetIP2LocationLite = "ip2location_lite"
	PresetIP2LocationASN  = "ip2location_asn"
	PresetIPinfoLite      = "ipinfo_lite"
	PresetIPinfoCore      = "ipinfo_core"
	PresetIPinfoPlus      = "ipinfo_plus"
	PresetMaxMindCountry  = "maxmind_country"
	PresetMaxMindCity     = "maxmind_city"
	PresetMaxMindASN      = "maxmind_asn"
)

func registerPresets() {
	registerIP2Location()
	registerIPinfo()
	registerMaxMind()
}

func ip2locMap(city, isp, domain, asn bool) FieldMap {
	m := FieldMap{
		"country_short": dbprovider.MetaCountry,
		"country_long":  dbprovider.MetaCountryName,
	}
	if city {
		m["region"] = dbprovider.MetaRegion
		m["city"] = dbprovider.MetaCity
	}
	if isp {
		m["isp"] = dbprovider.MetaIsp
	}
	if domain {
		m["domain"] = dbprovider.MetaDomain
	}
	if asn {
		m["asn"] = dbprovider.MetaAsn
	}
	return m
}

// registerIP2Location adds DB1–DB26, LITE DB1–DB11, and ASN LITE.
// Column sets follow the official IP2Location package comparison (Record keys only).
func registerIP2Location() {
	type cols struct{ city, isp, domain, asn bool }
	// Official matrix: https://github.com/ip2location/sample-databases
	db := map[int]cols{
		1:  {},
		2:  {isp: true},
		3:  {city: true},
		4:  {city: true, isp: true},
		5:  {city: true},
		6:  {city: true, isp: true},
		7:  {city: true, isp: true, domain: true},
		8:  {city: true, isp: true, domain: true},
		9:  {city: true},
		10: {city: true, isp: true, domain: true},
		11: {city: true},
		12: {city: true, isp: true, domain: true},
		13: {city: true},
		14: {city: true, isp: true, domain: true},
		15: {city: true},
		16: {city: true, isp: true, domain: true},
		17: {city: true},
		18: {city: true, isp: true, domain: true},
		19: {city: true, isp: true, domain: true},
		20: {city: true, isp: true, domain: true},
		21: {city: true},
		22: {city: true, isp: true, domain: true},
		23: {city: true, isp: true, domain: true},
		24: {city: true, isp: true, domain: true},
		25: {city: true, isp: true, domain: true},
		26: {city: true, isp: true, domain: true, asn: true},
	}
	for n, c := range db {
		register(fmt.Sprintf("ip2location_db%d", n), dbsource.TypeBIN, ip2locMap(c.city, c.isp, c.domain, c.asn))
	}
	for n := 1; n <= 11; n++ {
		c := db[n]
		register(fmt.Sprintf("ip2location_lite_db%d", n), dbsource.TypeBIN, ip2locMap(c.city, c.isp, c.domain, c.asn))
	}
	register(PresetIP2Location, dbsource.TypeBIN, ip2locMap(true, true, true, false))
	register(PresetIP2LocationLite, dbsource.TypeBIN, ip2locMap(false, false, false, false))
	register(PresetIP2LocationASN, dbsource.TypeBIN, FieldMap{"asn": dbprovider.MetaAsn})
}

func registerIPinfo() {
	lite := FieldMap{
		"country_code":   dbprovider.MetaCountry,
		"country":        dbprovider.MetaCountryName,
		"continent":      dbprovider.MetaContinent,
		"continent_code": dbprovider.MetaContinentCode,
		"as_name":        dbprovider.MetaIsp,
		"as_domain":      dbprovider.MetaDomain,
		"asn":            dbprovider.MetaAsn,
	}
	core := lite.Clone()
	core["region"] = dbprovider.MetaRegion
	core["city"] = dbprovider.MetaCity
	register(PresetIPinfoLite, dbsource.TypeMMDB, lite)
	register(PresetIPinfoCore, dbsource.TypeMMDB, core)
	register(PresetIPinfoPlus, dbsource.TypeMMDB, core.Clone())
}

func registerMaxMind() {
	country := FieldMap{
		"country.iso_code":   dbprovider.MetaCountry,
		"country.names.en":   dbprovider.MetaCountryName,
		"continent.names.en": dbprovider.MetaContinent,
		"continent.code":     dbprovider.MetaContinentCode,
	}
	city := country.Clone()
	city["subdivisions.0.iso_code"] = dbprovider.MetaRegion
	city["city.names.en"] = dbprovider.MetaCity
	asn := FieldMap{
		"autonomous_system_number":       dbprovider.MetaAsn,
		"autonomous_system_organization": dbprovider.MetaIsp,
	}
	register(PresetMaxMindCountry, dbsource.TypeMMDB, country)
	register(PresetMaxMindCity, dbsource.TypeMMDB, city)
	register(PresetMaxMindASN, dbsource.TypeMMDB, asn)
	registerAlias("geolite2_country", PresetMaxMindCountry)
	registerAlias("geolite2_city", PresetMaxMindCity)
	registerAlias("geolite2_asn", PresetMaxMindASN)
	registerAlias("geoip2_country", PresetMaxMindCountry)
	registerAlias("geoip2_city", PresetMaxMindCity)
	registerAlias("geoip2_asn", PresetMaxMindASN)
}

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
	PresetMaxMindCountry    = "maxmind_country"
	PresetMaxMindCity       = "maxmind_city"
	PresetMaxMindASN        = "maxmind_asn"
	PresetMaxMindISP        = "maxmind_isp"
	PresetMaxMindDomain     = "maxmind_domain"
	PresetMaxMindEnterprise = "maxmind_enterprise"
)

func registerPresets() {
	registerIP2Location()
	registerIPinfo()
	registerMaxMind()
}

func ip2locMap(city, isp, domain, asn bool) FieldMap {
	m := FieldMap{
		"country_short": {Key: dbprovider.MetaCountry},
		"country_long":  {Key: dbprovider.MetaCountryName},
	}
	if city {
		m["region"] = Field{Key: dbprovider.MetaRegion}
		m["city"] = Field{Key: dbprovider.MetaCity}
	}
	if isp {
		m["isp"] = Field{Key: dbprovider.MetaIsp}
	}
	if domain {
		m["domain"] = Field{Key: dbprovider.MetaDomain}
	}
	if asn {
		m["asn"] = Field{Key: dbprovider.MetaAsn}
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
	register(PresetIP2LocationASN, dbsource.TypeBIN, FieldMap{"asn": {Key: dbprovider.MetaAsn}})
}

func registerIPinfo() {
	lite := FieldMap{
		"country_code":   {Key: dbprovider.MetaCountry},
		"country":        {Key: dbprovider.MetaCountryName},
		"continent":      {Key: dbprovider.MetaContinent},
		"continent_code": {Key: dbprovider.MetaContinentCode},
		"as_name":        {Key: dbprovider.MetaIsp},
		"as_domain":      {Key: dbprovider.MetaDomain},
		"asn":            {Key: dbprovider.MetaAsn},
	}
	core := lite.Clone()
	core["region"] = Field{Key: dbprovider.MetaRegion}
	core["city"] = Field{Key: dbprovider.MetaCity}
	register(PresetIPinfoLite, dbsource.TypeMMDB, lite)
	register(PresetIPinfoCore, dbsource.TypeMMDB, core)
	register(PresetIPinfoPlus, dbsource.TypeMMDB, core.Clone())
}

func registerMaxMind() {
	country := FieldMap{
		"country.iso_code":   {Key: dbprovider.MetaCountry},
		"country.names.en":   {Key: dbprovider.MetaCountryName},
		"continent.names.en": {Key: dbprovider.MetaContinent},
		"continent.code":     {Key: dbprovider.MetaContinentCode},
	}
	city := country.Clone()
	city["subdivisions.0.iso_code"] = Field{Key: dbprovider.MetaRegion}
	city["city.names.en"] = Field{Key: dbprovider.MetaCity}
	asn := FieldMap{
		"autonomous_system_number":       {Key: dbprovider.MetaAsn, Type: FieldTypeUint32},
		"autonomous_system_organization": {Key: dbprovider.MetaIsp},
	}
	// GeoIP2 ISP / Domain are flat; Enterprise nests isp/domain/ASN under traits.
	isp := FieldMap{
		"isp":                      {Key: dbprovider.MetaIsp},
		"autonomous_system_number": {Key: dbprovider.MetaAsn, Type: FieldTypeUint32},
	}
	domain := FieldMap{
		"domain": {Key: dbprovider.MetaDomain},
	}
	enterprise := city.Clone()
	enterprise["traits.isp"] = Field{Key: dbprovider.MetaIsp}
	enterprise["traits.domain"] = Field{Key: dbprovider.MetaDomain}
	enterprise["traits.autonomous_system_number"] = Field{Key: dbprovider.MetaAsn, Type: FieldTypeUint32}
	register(PresetMaxMindCountry, dbsource.TypeMMDB, country)
	register(PresetMaxMindCity, dbsource.TypeMMDB, city)
	register(PresetMaxMindASN, dbsource.TypeMMDB, asn)
	register(PresetMaxMindISP, dbsource.TypeMMDB, isp)
	register(PresetMaxMindDomain, dbsource.TypeMMDB, domain)
	register(PresetMaxMindEnterprise, dbsource.TypeMMDB, enterprise)
	registerAlias("geolite2_country", PresetMaxMindCountry)
	registerAlias("geolite2_city", PresetMaxMindCity)
	registerAlias("geolite2_asn", PresetMaxMindASN)
	registerAlias("geoip2_country", PresetMaxMindCountry)
	registerAlias("geoip2_city", PresetMaxMindCity)
	registerAlias("geoip2_asn", PresetMaxMindASN)
	registerAlias("geoip2_isp", PresetMaxMindISP)
	registerAlias("geoip2_domain", PresetMaxMindDomain)
	registerAlias("geoip2_enterprise", PresetMaxMindEnterprise)
}

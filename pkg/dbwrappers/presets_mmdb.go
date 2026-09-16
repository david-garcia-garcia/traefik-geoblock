package dbwrappers

import (
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
)

// Named MMDB fieldsPreconfigured values (IPinfo and MaxMind product ids).
const (
	PresetIPinfoLite        = "ipinfo_lite"
	PresetIPinfoCore        = "ipinfo_core"
	PresetIPinfoPlus        = "ipinfo_plus"
	PresetMaxMindCountry    = "maxmind_country"
	PresetMaxMindCity       = "maxmind_city"
	PresetMaxMindASN        = "maxmind_asn"
	PresetMaxMindISP        = "maxmind_isp"
	PresetMaxMindDomain     = "maxmind_domain"
	PresetMaxMindEnterprise = "maxmind_enterprise"
)

// registerMMDBPresets adds IPinfo Lite/Core/Plus and MaxMind Country/City/ASN/ISP/Domain/Enterprise.
func registerMMDBPresets() {
	registerIPinfo()
	registerMaxMind()
}

// registerIPinfo adds Lite, Core, and Plus maps (Plus reuses Core columns).
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

// registerMaxMind adds GeoIP2/GeoLite2 maps and the product aliases.
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

package dbprovider

import "strings"

// Metadata keys a Provider may return. A BIN may leave fields empty
// (IP2Location LITE DB1 is country-only; ISP/domain need DB8; ASN needs ASN LITE).
const (
	MetaCountry       = "country"
	MetaCountryName   = "country_name"
	MetaContinent     = "continent"
	MetaContinentCode = "continent_code"
	MetaRegion        = "region"
	MetaCity          = "city"
	MetaIsp           = "isp"
	MetaDomain        = "domain"
	MetaAsn           = "asn"
)

// MetaKeys is the requestHeaderEnrich value list, in docs order.
func MetaKeys() []string {
	return []string{
		MetaCountry, MetaCountryName, MetaContinent, MetaContinentCode,
		MetaRegion, MetaCity, MetaIsp, MetaDomain, MetaAsn,
	}
}

// Record is the geo metadata for one IP. Country is used for allow/block rules.
type Record struct {
	Country       string
	CountryName   string
	Continent     string
	ContinentCode string
	Region        string
	City          string
	Isp           string
	Domain        string
	Asn           string
}

// Field returns the metadata value for key.
func (r Record) Field(key string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case MetaCountry:
		return r.Country
	case MetaCountryName:
		return r.CountryName
	case MetaContinent:
		return r.Continent
	case MetaContinentCode:
		return r.ContinentCode
	case MetaRegion:
		return r.Region
	case MetaCity:
		return r.City
	case MetaIsp:
		return r.Isp
	case MetaDomain:
		return r.Domain
	case MetaAsn:
		return r.Asn
	default:
		return ""
	}
}

// KnownMetaKey reports whether key is a supported requestHeaderEnrich value.
func KnownMetaKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case MetaCountry, MetaCountryName, MetaContinent, MetaContinentCode,
		MetaRegion, MetaCity, MetaIsp, MetaDomain, MetaAsn:
		return true
	default:
		return false
	}
}

// Provider looks up geo metadata for an IP. Close does not close shared wrappers.
type Provider interface {
	Lookup(ip string) (Record, error)
	Close() error
}

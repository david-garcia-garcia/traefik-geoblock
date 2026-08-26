package dbprovider

import "strings"

// Metadata keys a Provider may return. A BIN may leave fields empty
// (IP2Location LITE DB1 is country-only; ISP/domain need DB8; ASN needs ASN LITE).
const (
	MetaCountry = "country"
	MetaRegion  = "region"
	MetaCity    = "city"
	MetaIsp     = "isp"
	MetaDomain  = "domain"
	MetaAsn     = "asn"
)

// MetaKeys is the requestHeaderEnrich value list, in docs order.
func MetaKeys() []string {
	return []string{MetaCountry, MetaRegion, MetaCity, MetaIsp, MetaDomain, MetaAsn}
}

// Record is the geo metadata for one IP. Country is used for allow/block rules.
type Record struct {
	Country string
	Region  string
	City    string
	Isp     string
	Domain  string
	Asn     string
}

// Field returns the metadata value for key.
func (r Record) Field(key string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case MetaCountry:
		return r.Country
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
	case MetaCountry, MetaRegion, MetaCity, MetaIsp, MetaDomain, MetaAsn:
		return true
	default:
		return false
	}
}

// Provider opens a geo database, looks up metadata, and owns auto-update.
type Provider interface {
	Lookup(ip string) (Record, error)
	Close() error
}

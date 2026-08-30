package dbprovider

import (
	"fmt"
	"strings"
)

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

// Set writes value onto the named Record key. Unknown keys are ignored.
func (r *Record) Set(key, value string) {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case MetaCountry:
		r.Country = value
	case MetaCountryName:
		r.CountryName = value
	case MetaContinent:
		r.Continent = value
	case MetaContinentCode:
		r.ContinentCode = value
	case MetaRegion:
		r.Region = value
	case MetaCity:
		r.City = value
	case MetaIsp:
		r.Isp = value
	case MetaDomain:
		r.Domain = value
	case MetaAsn:
		r.Asn = value
	}
}

// FillEmpty copies non-empty src fields into empty fields of r.
func (r Record) FillEmpty(src Record) Record {
	if r.Country == "" {
		r.Country = src.Country
	}
	if r.CountryName == "" {
		r.CountryName = src.CountryName
	}
	if r.Continent == "" {
		r.Continent = src.Continent
	}
	if r.ContinentCode == "" {
		r.ContinentCode = src.ContinentCode
	}
	if r.Region == "" {
		r.Region = src.Region
	}
	if r.City == "" {
		r.City = src.City
	}
	if r.Isp == "" {
		r.Isp = src.Isp
	}
	if r.Domain == "" {
		r.Domain = src.Domain
	}
	if r.Asn == "" {
		r.Asn = src.Asn
	}
	return r
}

// Keep returns a copy with only the named keys. Empty keys keeps every field.
func (r Record) Keep(keys []string) Record {
	if len(keys) == 0 {
		return r
	}
	keep := make(map[string]bool, len(keys))
	for _, key := range keys {
		keep[strings.ToLower(strings.TrimSpace(key))] = true
	}
	var out Record
	if keep[MetaCountry] {
		out.Country = r.Country
	}
	if keep[MetaCountryName] {
		out.CountryName = r.CountryName
	}
	if keep[MetaContinent] {
		out.Continent = r.Continent
	}
	if keep[MetaContinentCode] {
		out.ContinentCode = r.ContinentCode
	}
	if keep[MetaRegion] {
		out.Region = r.Region
	}
	if keep[MetaCity] {
		out.City = r.City
	}
	if keep[MetaIsp] {
		out.Isp = r.Isp
	}
	if keep[MetaDomain] {
		out.Domain = r.Domain
	}
	if keep[MetaAsn] {
		out.Asn = r.Asn
	}
	return out
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

// boundLookup is a Lookup function used as a Provider.
type boundLookup struct {
	lookup func(ip string) (Record, error)
}

// Bind returns a Provider that calls lookup. Close is a no-op.
func Bind(lookup func(ip string) (Record, error)) Provider {
	return boundLookup{lookup: lookup}
}

// Lookup calls the bound function.
func (b boundLookup) Lookup(ip string) (Record, error) {
	if b.lookup == nil {
		return Record{}, fmt.Errorf("no lookup")
	}
	return b.lookup(ip)
}

// Close does not close a shared format wrapper.
func (b boundLookup) Close() error {
	return nil
}

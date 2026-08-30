package dbwrappers

import "github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"

// BINSource is a BIN queried through the IP2Location column map.
// Get_all fills country, region, city, isp, domain. Get_asn fills asn.
type BINSource struct {
	bin    *BIN
	fields []string
}

// NewBINSource returns a Provider that maps BIN columns onto Record.
// Empty fields keeps every key the map can fill (Get_all only).
func NewBINSource(bin *BIN, fields []string) *BINSource {
	return &BINSource{bin: bin, fields: fields}
}

// Lookup returns mapped BIN fields, then Keep(fields).
func (s *BINSource) Lookup(ip string) (dbprovider.Record, error) {
	var rec dbprovider.Record
	if s.wantGeo() {
		geo, err := s.bin.Lookup(ip)
		if err != nil {
			return dbprovider.Record{}, err
		}
		rec = geo
	}
	if s.wantAsn() {
		rec.Asn = s.bin.LookupASN(ip)
	}
	return rec.Keep(s.fields), nil
}

// wantGeo is true when fields is empty or lists a non-asn key.
func (s *BINSource) wantGeo() bool {
	if len(s.fields) == 0 {
		return true
	}
	for _, key := range s.fields {
		if key != dbprovider.MetaAsn {
			return true
		}
	}
	return false
}

// wantAsn is true when fields lists asn.
func (s *BINSource) wantAsn() bool {
	for _, key := range s.fields {
		if key == dbprovider.MetaAsn {
			return true
		}
	}
	return false
}

// Close does not close the shared BIN.
func (s *BINSource) Close() error {
	return nil
}

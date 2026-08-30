package dbwrappers

import "github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"

// ASNSource is a BIN queried as an IP2Location ASN source.
type ASNSource struct {
	bin *BIN
}

// NewASNSource returns a Provider that fills Record.Asn only.
func NewASNSource(bin *BIN) *ASNSource {
	return &ASNSource{bin: bin}
}

// Lookup returns ASN from the BIN.
func (s *ASNSource) Lookup(ip string) (dbprovider.Record, error) {
	if s == nil || s.bin == nil {
		return dbprovider.Record{}, nil
	}
	return dbprovider.Record{Asn: s.bin.LookupASN(ip)}, nil
}

// Close does not close the shared BIN.
func (s *ASNSource) Close() error {
	return nil
}

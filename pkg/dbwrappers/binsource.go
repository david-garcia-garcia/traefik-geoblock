package dbwrappers

import "github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"

// BINSource is a BIN queried as a catalog geo source.
type BINSource struct {
	bin *BIN
}

// NewBINSource returns a Provider that maps BIN Lookup onto Record.
func NewBINSource(bin *BIN) *BINSource {
	return &BINSource{bin: bin}
}

// Lookup returns geo fields from the BIN.
func (s *BINSource) Lookup(ip string) (dbprovider.Record, error) {
	return s.bin.Lookup(ip)
}

// Close does not close the shared BIN.
func (s *BINSource) Close() error {
	return nil
}

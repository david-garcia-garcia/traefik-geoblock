package dbprovider

import (
	"fmt"
	"sort"
)

// Named is one catalog source that can Lookup.
type Named struct {
	Key      string
	Provider Provider
}

// Combined merges Lookups from Named sources in lexicographic key order.
type Combined struct {
	sources []Named
}

// NewCombined sorts sources by Key and returns a Combined Provider.
func NewCombined(sources []Named) *Combined {
	out := append([]Named(nil), sources...)
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return &Combined{sources: out}
}

// Lookup fills empty Record fields from each source. A source error is skipped.
// If every source errors, the first error is returned.
func (c *Combined) Lookup(ip string) (Record, error) {
	if c == nil || len(c.sources) == 0 {
		return Record{}, fmt.Errorf("no catalog sources")
	}
	var out Record
	var firstErr error
	ok := false
	for _, src := range c.sources {
		rec, err := src.Provider.Lookup(ip)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		ok = true
		mergeEmpty(&out, rec)
	}
	if !ok {
		if firstErr != nil {
			return Record{}, firstErr
		}
		return Record{}, fmt.Errorf("no catalog sources")
	}
	return out, nil
}

// Close is a no-op. Shared wrappers stay open.
func (c *Combined) Close() error {
	return nil
}

// mergeEmpty copies non-empty src fields into empty dst fields.
func mergeEmpty(dst *Record, src Record) {
	*dst = dst.FillEmpty(src)
}

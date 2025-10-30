package geodb

import (
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbutils"
	"github.com/ip2location/ip2location-go/v9"
)

// Wrapper wraps ip2location.DB and allows for hot-swapping during updates
// This is the shared database interface used by all callers
type Wrapper struct {
	db      *ip2location.DB
	path    string
	version *dbutils.DBVersion
}

// Get_country_short performs IP country lookup (fast path - no locking)
func (w *Wrapper) Get_country_short(ip string) (ip2location.IP2Locationrecord, error) {
	return w.db.Get_country_short(ip)
}

// GetVersion returns the current database version (fast path - no locking)
func (w *Wrapper) GetVersion() *dbutils.DBVersion {
	return w.version
}

// GetPath returns the current database path (fast path - no locking)
func (w *Wrapper) GetPath() string {
	return w.path
}

// swapDatabase replaces the current database with a new one (internal method)
func (w *Wrapper) swapDatabase(newDB *ip2location.DB, newPath string, newVersion *dbutils.DBVersion) *ip2location.DB {
	oldDB := w.db
	w.db = newDB
	w.path = newPath
	w.version = newVersion

	return oldDB
}

package dbwrappers

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"log/slog"

	ip2loc "github.com/ip2location/ip2location-go/v9"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbutils"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/logging"
	"github.com/david-garcia-garcia/traefik-middleware-utilities/reclaim"
)

const (
	// DefaultBINMinAge is how long a dated BIN stays current before GET.
	DefaultBINMinAge = 30 * 24 * time.Hour
	// binASNPath is the IP2Location Get_all column name for ASN.
	binASNPath = "asn"
)

// BINConfig is one BIN file to resolve, open, and keep current.
type BINConfig struct {
	Dir             string
	Source          dbsource.Config
	AllowMissing    bool
	DefaultFileName string
	MinAge          time.Duration
	// OwnerPlugin is the creating middleware name on BIN log lines. Omitted from the share hash.
	OwnerPlugin string `json:"-"`
	// OwnerLevel is the log level for that owner logger. Omitted from the share hash.
	OwnerLevel string `json:"-"`
}

// BIN is one open IP2Location BIN (file handle) for the shared lifecycle, with
// temp-copy hot-swap.
//
// The published handle and its siblings (path, version) are not mutex-protected. A
// lookup may see a stale Path/Version or a handle from just before/after a hot-swap.
// That inconsistency is accepted: country data on the request path must not pay a lock,
// and a torn sibling is not a panic. Readers copy w.db once; closeHandle must not set
// w.db to nil (vendor Get_all on a nil *ip2loc.DB panics on d.metaok).
type BIN struct {
	cfg     BINConfig
	life    *lifecycle
	db      *ip2loc.DB
	path    string
	version *dbutils.DBVersion
	// closed is set before Stop so a late publish cannot go live.
	closed atomic.Bool
}

const keyPrefixBIN = "bin:"

// binKey is the process-table key: bin, catalog map key, then config hash.
func binKey(cfg BINConfig) string {
	return keyPrefixBIN + cfg.Source.Key + ":" + configHash(cfg)
}

// OpenBIN returns the singleton BIN for cfg and binds ctx on the process table.
// logger is the reclaim table logger. Wrapper lines use NewOwner when OwnerPlugin is set.
func OpenBIN(ctx context.Context, cfg BINConfig, logger *slog.Logger) (*BIN, error) {
	key := binKey(cfg)
	wrap := logger
	if cfg.OwnerPlugin != "" {
		wrap = logging.NewOwner(cfg.OwnerPlugin, cfg.OwnerLevel)
	}
	return reclaim.OpenTyped[*BIN](ctx, currentTable(), key, logger, func() (any, reclaim.Hooks, error) {
		created, err := newBIN(cfg, wrap)
		if err != nil {
			return nil, reclaim.Hooks{}, err
		}
		return created, reclaim.Hooks{
			Sleep: created.life.sleep,
			Wake:  created.life.wake,
			Close: created.life.close,
		}, nil
	})
}

func newBIN(cfg BINConfig, logger *slog.Logger) (*BIN, error) {
	if logger == nil {
		logger = slog.Default()
	}
	w := &BIN{cfg: cfg}
	w.life = &lifecycle{
		format:  w,
		subject: "BIN",
		// The vendor handle keeps reading the file, so a dated catalog file is served from a copy.
		servesFromFile: true,
		seedFirst:      true,
		allowMissing:   cfg.AllowMissing,
		source:         binSourceCfg(cfg),
		logger:         logger.With("key", cfg.Source.Key),
	}
	if err := w.life.initialize(); err != nil {
		return nil, err
	}
	w.life.startUpdate()
	return w, nil
}

// binSourceCfg is the catalog source for this BIN, with the BIN keep-current floor.
func binSourceCfg(cfg BINConfig) dbsource.Config {
	source := dbsource.WithDefaults(cfg.Source, cfg.Dir, dbsource.TypeBIN, cfg.MinAge)
	if source.MinAge <= 0 {
		source.MinAge = DefaultBINMinAge
	}
	if source.DefaultFileName == "" {
		source.DefaultFileName = cfg.DefaultFileName
	}
	return source
}

// publishFile opens path with the IP2Location SDK and publishes that handle without a
// lock. The handle it replaces is closed after a read grace; only then may the file
// that handle was reading be removed.
func (w *BIN) publishFile(path, sourcePath string, retire func()) (*dbutils.DBVersion, bool, error) {
	db, err := ip2loc.OpenDB(path)
	if err != nil {
		return nil, false, fmt.Errorf("failed to open database %s: %w", path, err)
	}
	version, err := dbutils.GetDatabaseVersion(path)
	if err != nil {
		db.Close()
		return nil, false, fmt.Errorf("failed to read database version from %s: %w", path, err)
	}
	// A handle opened after Close stays disposed so this generation cannot come back.
	if w.closed.Load() {
		db.Close()
		return version, false, nil
	}
	old := w.swapHandle(db, path, version)
	if old == nil {
		retire()
		return version, true, nil
	}
	disposeAfterGrace(func() { old.Close() }, retire)
	return version, true, nil
}

// swapHandle stores the published vendor handle and its siblings, then returns the previous handle.
// No lock: a concurrent lookup may observe a stale sibling or the previous handle. Accepted.
func (w *BIN) swapHandle(db *ip2loc.DB, path string, version *dbutils.DBVersion) *ip2loc.DB {
	old := w.db
	w.db = db
	w.path = path
	w.version = version
	return old
}

// refusePublish marks this BIN disposed. The lifecycle sets it before joining Stop.
func (w *BIN) refusePublish() {
	w.closed.Store(true)
}

// closeHandle Closes the vendor file. It does not set w.db to nil: a later Get_all on a
// nil *ip2loc.DB panics. In-flight lookups keep the pointer they already copied (that
// Get_all is undeterministic). Later lookups see closed and fail without touching it.
func (w *BIN) closeHandle() {
	if w.db != nil {
		w.db.Close()
	}
}

// LookupRecord fills Record from the BIN using fields (path → Record key).
// One Get_all; only mapped paths are copied.
func (w *BIN) LookupRecord(ip string, fields FieldMap) (dbprovider.Record, error) {
	if w == nil {
		return dbprovider.Record{}, fmt.Errorf("BIN is not open")
	}
	record, err := w.getAll(ip)
	if err != nil {
		return dbprovider.Record{}, err
	}
	country := record.Country_short
	if len(country) >= 7 && strings.EqualFold(country[:7], "invalid") {
		return dbprovider.Record{}, fmt.Errorf("%s", country)
	}
	var rec dbprovider.Record
	fields.apply(&rec, func(path string) string {
		return binColumn(record, path)
	})
	return rec, nil
}

// getAll copies the vendor pointer once and calls Get_all on that local.
// No lock on this path. After Close, closed is true so this fails without
// touching the pointer. A concurrent hot-swap may change w.db after the copy;
// this lookup keeps the handle it already took. Do not read w.db twice: a nil
// between the check and Get_all panics in the vendor query.
func (w *BIN) getAll(ip string) (ip2loc.IP2Locationrecord, error) {
	if w.closed.Load() {
		return ip2loc.IP2Locationrecord{}, fmt.Errorf("BIN is not open")
	}
	db := w.db
	if db == nil {
		return ip2loc.IP2Locationrecord{}, fmt.Errorf("BIN is not open")
	}
	return db.Get_all(ip)
}

// binColumn is one IP2Location Get_all column.
func binColumn(rec ip2loc.IP2Locationrecord, path string) string {
	switch path {
	case "country_short":
		// Vendor "-" is empty so Combined can fill country from a later source.
		return usableMeta(rec.Country_short)
	case "country_long":
		return usableMeta(rec.Country_long)
	case "region":
		return usableMeta(rec.Region)
	case "city":
		return usableMeta(rec.City)
	case "isp":
		return usableMeta(rec.Isp)
	case "domain":
		return usableMeta(rec.Domain)
	case binASNPath:
		return usableMeta(rec.Asn)
	default:
		return ""
	}
}

// Version is the BIN header version. May be stale during hot-swap. Accepted.
func (w *BIN) Version() *dbutils.DBVersion {
	return w.version
}

// Path is the file last opened (temp copy or source). May be stale during hot-swap. Accepted.
func (w *BIN) Path() string {
	return w.path
}

// SourcePath is the dated or seed file the live handle was copied from.
// May be stale during hot-swap. Accepted. The keep-current skip-compare uses this.
func (w *BIN) SourcePath() string {
	return w.life.sourcePath()
}

// Close stops the updater and the file handle. Tests may call this; production Close is the reclaim Hooks.Close.
func (w *BIN) Close() {
	w.life.close()
}

func usableMeta(value string) string {
	if value == "" || value == "-" {
		return ""
	}
	if strings.HasPrefix(value, "This parameter is unavailable") {
		return ""
	}
	if len(value) >= 7 && strings.EqualFold(value[:7], "invalid") {
		return ""
	}
	return value
}

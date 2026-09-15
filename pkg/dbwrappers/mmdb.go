package dbwrappers

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"log/slog"

	"github.com/oschwald/maxminddb-golang"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbutils"
	"github.com/david-garcia-garcia/traefik-middleware-utilities/reclaim"
)

// MMDBConfig is one MMDB file to resolve, open, and keep current.
type MMDBConfig struct {
	Dir             string
	Source          dbsource.Config
	DefaultFileName string
	MinAge          time.Duration
}

// MMDB is one open MaxMind DB (FromBytes) for the shared lifecycle.
//
// The published reader and its path are not mutex-protected. A lookup may see a stale
// Path or the previous generation's reader during a hot-swap. That inconsistency is
// accepted: the request path must not pay a lock. Readers copy w.db once, and a reader
// a swap replaced is Closed only after a read grace: vendor Close nils the buffer, so a
// lookup still inside that reader would fail (cannot call Lookup on a closed database)
// instead of returning the answer it was already resolving.
type MMDB struct {
	cfg  MMDBConfig
	life *lifecycle
	db   *maxminddb.Reader
	path string
	// closed is set before Stop so a late publish cannot go live.
	closed atomic.Bool
}

const keyPrefixMMDB = "mmdb:"

// mmdbKey is the process-table key: mmdb, catalog map key, then config hash.
func mmdbKey(cfg MMDBConfig) string {
	return keyPrefixMMDB + cfg.Source.Key + ":" + configHash(cfg)
}

// OpenMMDB returns the singleton MMDB for cfg and binds ctx on the process table.
func OpenMMDB(ctx context.Context, cfg MMDBConfig, logger *slog.Logger) (*MMDB, error) {
	key := mmdbKey(cfg)
	return reclaim.OpenTyped[*MMDB](ctx, currentTable(), key, logger, func() (any, reclaim.Hooks, error) {
		created, err := newMMDB(cfg, logger)
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

func newMMDB(cfg MMDBConfig, logger *slog.Logger) (*MMDB, error) {
	if logger == nil {
		logger = slog.Default()
	}
	w := &MMDB{cfg: cfg}
	w.life = &lifecycle{
		format:  w,
		subject: "MMDB",
		// FromBytes holds the whole file in memory, so a replaced file needs no temp copy.
		servesFromFile: false,
		// A bundled MMDB defaultFile may be a dummy fixture, so it does not go live
		// ahead of a dated catalog file.
		seedFirst:    false,
		allowMissing: false,
		source:       mmdbSourceCfg(cfg),
		logger:       logger.With("key", cfg.Source.Key),
	}
	if err := w.life.initialize(); err != nil {
		return nil, err
	}
	w.life.startUpdate()
	return w, nil
}

// mmdbSourceCfg is the catalog source for this MMDB.
func mmdbSourceCfg(cfg MMDBConfig) dbsource.Config {
	source := dbsource.WithDefaults(cfg.Source, cfg.Dir, dbsource.TypeMMDB, cfg.MinAge)
	if source.DefaultFileName == "" {
		source.DefaultFileName = cfg.DefaultFileName
	}
	return source
}

// publishFile reads path into memory and publishes that reader without a lock. The
// reader it replaces is Closed after a read grace; only then may the file that reader
// was given be removed. MMDB has no version header: the catalog file name carries the date.
func (w *MMDB) publishFile(path, sourcePath string, retire func()) (*dbutils.DBVersion, bool, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read MMDB %s: %w", path, err)
	}
	db, err := maxminddb.FromBytes(buf)
	if err != nil {
		return nil, false, fmt.Errorf("failed to open MMDB %s: %w", path, err)
	}
	// A reader opened after Close stays disposed so this generation cannot come back.
	if w.closed.Load() {
		_ = db.Close()
		return nil, false, nil
	}
	old := w.swapReader(db, path)
	if old == nil {
		retire()
		return nil, true, nil
	}
	disposeAfterGrace(func() { _ = old.Close() }, retire)
	return nil, true, nil
}

// swapReader stores the published reader and its path, then returns the previous reader.
// No lock: a concurrent lookup may observe a stale path or the previous reader. Accepted.
func (w *MMDB) swapReader(db *maxminddb.Reader, path string) *maxminddb.Reader {
	old := w.db
	w.db = db
	w.path = path
	return old
}

// refusePublish marks this MMDB disposed. The lifecycle sets it before joining Stop.
func (w *MMDB) refusePublish() {
	w.closed.Store(true)
}

// closeHandle Closes the live reader. It leaves the pointer in place: a closed vendor
// reader already fails its own lookups cleanly, unlike BIN where a nil handle panics.
// Later lookups see closed and fail without touching it.
func (w *MMDB) closeHandle() {
	if w.db != nil {
		_ = w.db.Close()
	}
}

// Path is the file last opened. May be stale during hot-swap. Accepted.
func (w *MMDB) Path() string {
	return w.path
}

// LookupRecord fills Record from the MMDB using fields (dotted path → Record key).
// Only those paths are decoded; unused MMDB keys are skipped.
func (w *MMDB) LookupRecord(ip string, fields FieldMap) (dbprovider.Record, error) {
	if len(fields) == 0 {
		return dbprovider.Record{}, w.Lookup(ip, &struct{}{})
	}
	extract := lookupExtract(fields)
	dest := extract.takeDest()
	defer extract.putDest(dest)
	if err := w.Lookup(ip, dest); err != nil {
		return dbprovider.Record{}, err
	}
	return extract.record(dest), nil
}

// Lookup decodes ip into dest (raw MMDB tags). It copies the vendor pointer once and
// calls Lookup on that local. No lock on this path. After Close, closed is true so this
// fails without touching the pointer. A concurrent hot-swap may change w.db after the
// copy; this lookup keeps the reader it already took, which stays readable for the grace.
func (w *MMDB) Lookup(ip string, dest any) error {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}
	if w.closed.Load() {
		return fmt.Errorf("MMDB is not open")
	}
	db := w.db
	if db == nil {
		return fmt.Errorf("MMDB is not open")
	}
	return db.Lookup(parsed, dest)
}

// Close stops the updater and the reader. Tests may call this; production Close is the reclaim Hooks.Close.
func (w *MMDB) Close() {
	w.life.close()
}

func configHash(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	h := fnv.New64a()
	_, _ = h.Write(b)
	return strconv.FormatUint(h.Sum64(), 16)
}

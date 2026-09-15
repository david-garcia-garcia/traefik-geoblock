package dbwrappers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"log/slog"

	ip2loc "github.com/ip2location/ip2location-go/v9"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbutils"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
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

// BIN is one open IP2Location BIN (file handle) with temp-copy hot-swap.
//
// The published handle and sibling fields (path, version, local copy, source
// path) are not mutex-protected. A lookup may see a stale Path/Version/SourcePath
// or a handle from just before/after a hot-swap. That inconsistency is accepted:
// country data on the request path must not pay a lock, and a torn sibling is
// not a panic. Readers copy w.db once; close must not set w.db to nil (vendor
// Get_all on a nil *ip2loc.DB panics on d.metaok).
type BIN struct {
	cfg                BINConfig
	logger             *slog.Logger
	db                 *ip2loc.DB
	path               string
	version            *dbutils.DBVersion
	currentLocalDbCopy string
	sourceDbPath       string
	updater            *dbsource.Updater
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
	var w *BIN
	v, err := currentTable().Open(ctx, key, logger, func() (any, error) {
		created, err := newBIN(cfg, wrap)
		if err != nil {
			return nil, err
		}
		w = created
		return created, nil
	}, reclaim.Hooks{
		Sleep: func() { w.sleep() },
		Wake:  func() { w.wake() },
		Close: func() { w.close() },
	})
	if err != nil {
		return nil, err
	}
	typed, ok := v.(*BIN)
	if !ok {
		return nil, fmt.Errorf("reclaim: %s: want *BIN, got %T", key, v)
	}
	return typed, nil
}

func newBIN(cfg BINConfig, logger *slog.Logger) (*BIN, error) {
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With("key", cfg.Source.Key)
	w := &BIN{cfg: cfg, logger: logger}
	if err := w.initialize(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Source.URL) != "" {
		w.startUpdate()
	}
	return w, nil
}

func (w *BIN) sourceCfg() dbsource.Config {
	cfg := dbsource.WithDefaults(w.cfg.Source, w.cfg.Dir, dbsource.TypeBIN, w.cfg.MinAge)
	if cfg.MinAge <= 0 {
		cfg.MinAge = DefaultBINMinAge
	}
	if cfg.DefaultFileName == "" {
		cfg.DefaultFileName = w.cfg.DefaultFileName
	}
	return cfg
}

// initialize resolves the catalog or seed file, copies it when dated, and opens the handle.
func (w *BIN) initialize() error {
	cfg := w.sourceCfg()
	resolved, err := dbsource.Resolve(cfg, w.logger)
	if err != nil && resolved == "" && !w.cfg.AllowMissing {
		return fmt.Errorf("failed to resolve database path: %w", err)
	}

	var targetPath string
	if resolved != "" {
		if latest, lerr := dbsource.Latest(cfg.Dir, cfg.Key, dbsource.TypeBIN); lerr == nil && latest != "" && latest == resolved {
			w.sourceDbPath = latest
			copied, cerr := w.createLocalCopy(latest)
			if cerr != nil {
				w.logger.Warn("local copy failed, opening source", "error", cerr)
				targetPath = latest
			} else {
				targetPath = copied
				w.currentLocalDbCopy = copied
			}
		} else {
			targetPath = resolved
		}
	}

	if w.sourceDbPath == "" {
		w.sourceDbPath = targetPath
	}

	if targetPath == "" {
		if !w.cfg.AllowMissing {
			return fmt.Errorf("database file not found")
		}
		w.logger.Info("no database file yet; waiting for auto-update")
		return nil
	}

	db, err := ip2loc.OpenDB(targetPath)
	if err != nil {
		return fmt.Errorf("failed to open database %s: %w", targetPath, err)
	}
	version, err := dbutils.GetDatabaseVersion(targetPath)
	if err != nil {
		db.Close()
		return fmt.Errorf("failed to read database version from %s: %w", targetPath, err)
	}
	w.db = db
	w.path = targetPath
	w.version = version
	w.logger.Info("BIN initialized", "path", targetPath, "source_path", w.sourceDbPath, "version", version.String())
	if time.Since(version.Date()) > 60*24*time.Hour {
		w.logger.Warn("ip2location database is more than 2 months old",
			"version", version.String(),
			"age", time.Since(version.Date()).Round(24*time.Hour))
	}
	return nil
}

// createLocalCopy writes a process-temp copy named bin_<catalogKey>_<unixNano>.BIN.
func (w *BIN) createLocalCopy(sourcePath string) (string, error) {
	tmpFile := filepath.Join(os.TempDir(), binCopyName(fileToken(w.cfg.Source.Key), time.Now().UnixNano()))
	if err := fileutils.Copy(sourcePath, tmpFile, false); err != nil {
		return "", fmt.Errorf("failed to create local copy: %w", err)
	}
	return tmpFile, nil
}

// fileToken is the catalog key as a temp-file name segment.
func fileToken(catalogKey string) string {
	var b strings.Builder
	for _, r := range catalogKey {
		if fileTokenRune(r) {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	return b.String()
}

// fileTokenRune reports whether r may appear in a temp-copy catalog-key segment.
func fileTokenRune(r rune) bool {
	return r == '.' || r == '_' || r == '-' ||
		(r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}

// binCopyName is the temp-copy basename: format, catalog key, Unix nanosecond.
func binCopyName(token string, unixNano int64) string {
	if token == "" {
		return fmt.Sprintf("bin_%d.BIN", unixNano)
	}
	return fmt.Sprintf("bin_%s_%d.BIN", token, unixNano)
}

// sleep stops the keep-current ticker while this wrapper is parked in grace.
func (w *BIN) sleep() {
	if w.updater != nil {
		w.updater.Stop()
	}
}

// wake starts the keep-current ticker after a reclaim.
func (w *BIN) wake() {
	w.startUpdate()
}

// startUpdate starts the keep-current ticker that hot-swaps a newer dated BIN.
func (w *BIN) startUpdate() {
	updater, err := dbsource.Start(w.sourceCfg(), w.logger, func(path string) {
		if path == "" || path == w.SourcePath() {
			return
		}
		if err := w.hotSwap(path); err != nil {
			w.logger.Error("failed to perform hot swap", "error", err)
		}
	})
	if err != nil {
		w.logger.Error("source updater", "error", err)
		return
	}
	w.updater = updater
}

// hotSwap opens a new dated catalog file and replaces the live handle.
func (w *BIN) hotSwap(newDatabasePath string) error {
	newLocalCopy, err := w.createLocalCopy(newDatabasePath)
	if err != nil {
		return err
	}
	newDB, err := ip2loc.OpenDB(newLocalCopy)
	if err != nil {
		os.Remove(newLocalCopy)
		return fmt.Errorf("hotSwap: failed to open new database: %w", err)
	}
	newVersion, err := dbutils.GetDatabaseVersion(newLocalCopy)
	if err != nil {
		newDB.Close()
		os.Remove(newLocalCopy)
		return fmt.Errorf("hotSwap: failed to read new database version: %w", err)
	}
	old := w.swapHandle(newDB, newLocalCopy, newVersion, newLocalCopy, newDatabasePath)
	if old != nil {
		go func() {
			time.Sleep(10 * time.Second)
			old.Close()
		}()
	}
	w.logger.Info("BIN hot-swapped", "new_version", newVersion.String(), "new_path", newLocalCopy, "source_path", newDatabasePath)
	return nil
}

// swapHandle stores the published vendor handle and sibling paths, then returns the previous handle.
// No lock: a concurrent lookup may observe a stale sibling or the previous handle. Accepted.
func (w *BIN) swapHandle(db *ip2loc.DB, path string, version *dbutils.DBVersion, currentLocalDbCopy, sourceDbPath string) *ip2loc.DB {
	old := w.db
	w.db = db
	w.path = path
	w.version = version
	w.currentLocalDbCopy = currentLocalDbCopy
	w.sourceDbPath = sourceDbPath
	return old
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
// No lock on this path. A concurrent hot-swap may change w.db after the copy;
// this lookup keeps the handle it already took. Do not read w.db twice: a nil
// between the check and Get_all panics in the vendor query.
func (w *BIN) getAll(ip string) (ip2loc.IP2Locationrecord, error) {
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
// May be stale during hot-swap. Accepted. startUpdate skip-compare uses this.
func (w *BIN) SourcePath() string {
	return w.sourceDbPath
}

// Close stops the updater and the file handle. Tests may call this; production Close is the reclaim Hooks.Close.
func (w *BIN) Close() {
	w.close()
}

// close stops the updater and Closes the vendor file. It does not set w.db to
// nil: a later Get_all on a nil *ip2loc.DB panics. In-flight lookups keep the
// pointer they already copied; after Close the vendor call is undeterministic.
func (w *BIN) close() {
	if w.updater != nil {
		w.updater.Stop()
	}
	if w.db != nil {
		w.db.Close()
	}
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

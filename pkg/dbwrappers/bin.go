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
)

// DefaultBINMinAge is how long a dated BIN stays current before GET.
const DefaultBINMinAge = 30 * 24 * time.Hour

// BINConfig is one BIN file to resolve, open, and keep current.
type BINConfig struct {
	Dir             string
	Source          dbsource.Config
	AllowMissing    bool
	DefaultFileName string
	MinAge          time.Duration
}

// BIN is one open IP2Location BIN (file handle) with temp-copy hot-swap.
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

var bins *Table[*BIN]

func binTable() *Table[*BIN] {
	tablesMu.Lock()
	defer tablesMu.Unlock()
	if bins == nil {
		bins = NewTable[*BIN](DefaultGrace, slog.Default())
	}
	return bins
}

// OpenBIN returns the singleton BIN for cfg and binds ctx on the table.
func OpenBIN(ctx context.Context, cfg BINConfig, logger *slog.Logger) (*BIN, error) {
	return binTable().Open(ctx, configHash(cfg), func() (*BIN, error) {
		return newBIN(cfg, logger)
	}, func(w *BIN) { w.close() })
}

func newBIN(cfg BINConfig, logger *slog.Logger) (*BIN, error) {
	if logger == nil {
		logger = slog.Default()
	}
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
	w.logger.Info("BIN initialized", "path", targetPath, "version", version.String())
	if time.Since(version.Date()) > 60*24*time.Hour {
		w.logger.Warn("ip2location database is more than 2 months old",
			"version", version.String(),
			"age", time.Since(version.Date()).Round(24*time.Hour))
	}
	return nil
}

func (w *BIN) createLocalCopy(sourcePath string) (string, error) {
	now := time.Now()
	timestamp := fmt.Sprintf("%s_%d", now.Format("20060102_150405"), now.Nanosecond())
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("IP2LOCATION_%s.BIN", timestamp))
	if err := fileutils.Copy(sourcePath, tmpFile, false); err != nil {
		return "", fmt.Errorf("failed to create local copy: %w", err)
	}
	w.currentLocalDbCopy = tmpFile
	return tmpFile, nil
}

func (w *BIN) startUpdate() {
	updater, err := dbsource.Start(w.sourceCfg(), w.logger, func(path string) {
		if path == "" || path == w.sourceDbPath {
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
	oldDB := w.db
	w.db = newDB
	w.path = newLocalCopy
	w.version = newVersion
	w.currentLocalDbCopy = newLocalCopy
	w.sourceDbPath = newDatabasePath
	if oldDB != nil {
		go func() {
			time.Sleep(10 * time.Second)
			oldDB.Close()
		}()
	}
	w.logger.Info("BIN hot-swapped", "new_version", newVersion.String(), "new_path", newLocalCopy)
	return nil
}

// Lookup returns country/region/city/isp/domain for ip.
func (w *BIN) Lookup(ip string) (dbprovider.Record, error) {
	if w == nil || w.db == nil {
		return dbprovider.Record{}, fmt.Errorf("BIN is not open")
	}
	record, err := w.db.Get_all(ip)
	if err != nil {
		return dbprovider.Record{}, err
	}
	country := record.Country_short
	if len(country) >= 7 && strings.EqualFold(country[:7], "invalid") {
		return dbprovider.Record{}, fmt.Errorf("%s", country)
	}
	return dbprovider.Record{
		Country: country,
		Region:  usableMeta(record.Region),
		City:    usableMeta(record.City),
		Isp:     usableMeta(record.Isp),
		Domain:  usableMeta(record.Domain),
	}, nil
}

// LookupASN returns the ASN string, or empty if the BIN is missing or has no ASN.
func (w *BIN) LookupASN(ip string) string {
	if w == nil || w.db == nil {
		return ""
	}
	record, err := w.db.Get_asn(ip)
	if err != nil {
		return ""
	}
	return usableMeta(record.Asn)
}

// GetCountryShort is the IP2Location country-only lookup (tests).
func (w *BIN) GetCountryShort(ip string) (ip2loc.IP2Locationrecord, error) {
	return w.db.Get_country_short(ip)
}

// Version is the BIN header version.
func (w *BIN) Version() *dbutils.DBVersion {
	return w.version
}

// Path is the file last opened (temp copy or source).
func (w *BIN) Path() string {
	return w.path
}

// SourcePath is the dated or seed file the live handle was copied from.
func (w *BIN) SourcePath() string {
	return w.sourceDbPath
}

func (w *BIN) close() {
	if w.updater != nil {
		w.updater.Stop()
	}
	if w.db != nil {
		w.db.Close()
		w.db = nil
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

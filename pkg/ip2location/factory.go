package ip2location

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"log/slog"

	ip2loc "github.com/ip2location/ip2location-go/v9"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbdownload"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbutils"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
)

// DownloadMinAge is how long a dated BIN stays current before GET.
const DownloadMinAge = 30 * 24 * time.Hour

// DatabaseConfig is IP2Location download slots plus shared auto-update dir.
type DatabaseConfig struct {
	DatabaseAutoUpdateDir string
	Download              dbdownload.Config
	// BinRole is geo vs asn for the default filename and temp copy. Not a catalog key.
	BinRole string

	// AsnDownload is used only on the parent config passed to New.
	AsnDownload dbdownload.Config

	// AllowMissing lets initialize succeed when the BIN is not on disk yet
	// (ASN first start: download lands later, then hot-swap).
	AllowMissing bool
}

// DatabaseWrapper wraps ip2location.DB and allows for hot-swapping during updates
type DatabaseWrapper struct {
	db      *ip2loc.DB
	path    string
	version *dbutils.DBVersion
}

// Get_country_short performs IP country lookup (fast path - no locking)
func (dw *DatabaseWrapper) Get_country_short(ip string) (ip2loc.IP2Locationrecord, error) {
	return dw.db.Get_country_short(ip)
}

// Lookup returns country/region/city for ip. Region and city are empty when
// the BIN does not include those columns (LITE DB1).
func (dw *DatabaseWrapper) Lookup(ip string) (dbprovider.Record, error) {
	record, err := dw.db.Get_all(ip)
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

func (dw *DatabaseWrapper) lookupASN(ip string) string {
	if dw == nil || dw.db == nil {
		return ""
	}
	record, err := dw.db.Get_asn(ip)
	if err != nil {
		return ""
	}
	return usableMeta(record.Asn)
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

// GetVersion returns the current database version (fast path - no locking)
func (dw *DatabaseWrapper) GetVersion() *dbutils.DBVersion {
	return dw.version
}

// GetPath returns the current database path (fast path - no locking)
func (dw *DatabaseWrapper) GetPath() string {
	return dw.path
}

// Close closes the database connection
func (dw *DatabaseWrapper) Close() error {
	if dw.db != nil {
		dw.db.Close()
		dw.db = nil
	}
	return nil
}

// swapDatabase replaces the current database with a new one (internal method)
func (dw *DatabaseWrapper) swapDatabase(newDB *ip2loc.DB, newPath string, newVersion *dbutils.DBVersion) *ip2loc.DB {
	oldDB := dw.db
	dw.db = newDB
	dw.path = newPath
	dw.version = newVersion

	return oldDB
}

// DatabaseFactory manages database instances and auto-updates for a specific database path
type DatabaseFactory struct {
	config             *DatabaseConfig
	logger             *slog.Logger
	wrapper            *DatabaseWrapper
	currentLocalDbCopy string
	sourceDbPath       string // Track the original database that was used for the current local copy
	download           *dbdownload.Slot
	factoryID          string // Unique identifier for this factory instance
}

// NewDatabaseFactory creates a new database factory instance
func NewDatabaseFactory(config *DatabaseConfig, logger *slog.Logger) (*DatabaseFactory, error) {
	// Generate unique factory ID and create wrapped logger
	factoryID := generateConfigHash(config)
	wrappedLogger := logger.With("factory_id", factoryID)

	factory := &DatabaseFactory{
		config:    config,
		logger:    wrappedLogger,
		wrapper:   &DatabaseWrapper{},
		factoryID: factoryID,
	}

	// Initialize the database
	if err := factory.initialize(); err != nil {
		return nil, fmt.Errorf("NewDatabaseFactory: failed to initialize database factory: %w", err)
	}

	if strings.TrimSpace(config.Download.URL) != "" {
		factory.startAutoUpdate()
	}

	return factory, nil
}

// GetWrapper returns the database wrapper for use
func (df *DatabaseFactory) GetWrapper() *DatabaseWrapper {
	return df.wrapper
}

// GetSourceDbPath returns the original database path that was used for the current active database
func (df *DatabaseFactory) GetSourceDbPath() string {
	return df.sourceDbPath
}

// GetFactoryID returns the unique identifier for this factory instance
func (df *DatabaseFactory) GetFactoryID() string {
	return df.factoryID
}

func (df *DatabaseFactory) binRole() string {
	if df.config.BinRole != "" {
		return df.config.BinRole
	}
	return dbutils.SlotGeo
}

// Close shuts down the factory and cleans up resources
func (df *DatabaseFactory) Close() error {
	if df.download != nil {
		df.download.Stop()
	}

	if df.wrapper != nil {
		df.wrapper.Close()
	}

	return nil
}

func (df *DatabaseFactory) downloadCfg() dbdownload.Config {
	cfg := dbdownload.WithDefaults(df.config.Download, df.config.DatabaseAutoUpdateDir, dbdownload.TypeBIN, DownloadMinAge)
	if cfg.DefaultFileName == "" {
		cfg.DefaultFileName = defaultFileNameForSlot(df.binRole())
	}
	return cfg
}

// initialize sets up the initial database using the best available version.
// Download Resolve picks dated file, catalog path, or the vendor default name.
func (df *DatabaseFactory) initialize() error {
	cfg := df.downloadCfg()
	resolved, err := dbdownload.Resolve(cfg, df.logger)
	if err != nil && resolved == "" && !df.config.AllowMissing {
		return fmt.Errorf("failed to resolve database path: %w", err)
	}

	var targetPath string
	if resolved != "" {
		if latest, lerr := dbdownload.Latest(cfg.Dir, cfg.Key, dbdownload.TypeBIN); lerr == nil && latest != "" && latest == resolved {
			df.sourceDbPath = latest
			copied, cerr := df.createLocalDatabaseCopy(latest)
			if cerr != nil {
				df.logger.Warn("local copy failed, opening source", "error", cerr)
				targetPath = latest
			} else {
				targetPath = copied
			}
		} else {
			targetPath = resolved
		}
	}

	df.logger.Debug("initializing database", "path", targetPath)

	// Track the source database path before opening (only if not already set by auto-update)
	if df.sourceDbPath == "" {
		df.sourceDbPath = targetPath
	}

	if targetPath == "" {
		if !df.config.AllowMissing {
			return fmt.Errorf("database file not found")
		}
		df.logger.Info("no database file yet; waiting for auto-update")
		return nil
	}

	// Open the database
	db, err := ip2loc.OpenDB(targetPath)
	if err != nil {
		return fmt.Errorf("failed to open database %s: %w", targetPath, err)
	}

	// Get and validate database version
	version, err := dbutils.GetDatabaseVersion(targetPath)
	if err != nil {
		db.Close()
		return fmt.Errorf("failed to read database version from %s: %w", targetPath, err)
	}

	// Initialize wrapper
	df.wrapper.db = db
	df.wrapper.path = targetPath
	df.wrapper.version = version

	df.logger.Info("database initialized",
		"path", targetPath,
		"version", version.String(),
		"age", time.Since(version.Date()).Round(24*time.Hour))

	// Check if database is older than 2 months
	if time.Since(version.Date()) > 60*24*time.Hour {
		df.logger.Warn("ip2location database is more than 2 months old",
			"version", version.String(),
			"age", time.Since(version.Date()).Round(24*time.Hour))
	}

	return nil
}

// createLocalDatabaseCopy creates a timestamped local copy that doesn't overwrite existing files
func (df *DatabaseFactory) createLocalDatabaseCopy(sourcePath string) (string, error) {
	// Always create unique timestamped copy with nanoseconds to guarantee uniqueness
	now := time.Now()
	timestamp := fmt.Sprintf("%s_%d", now.Format("20060102_150405"), now.Nanosecond())
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("IP2LOCATION-%s_%s.BIN", df.binRole(), timestamp))

	// Copy to temp location
	if err := fileutils.Copy(sourcePath, tmpFile, false); err != nil {
		return "", fmt.Errorf("failed to create local copy: %w", err)
	}

	df.currentLocalDbCopy = tmpFile
	df.logger.Debug("created local database copy", "source", sourcePath, "dest", tmpFile)
	return tmpFile, nil
}

// startAutoUpdate starts the shared download component.
func (df *DatabaseFactory) startAutoUpdate() {
	slot, err := dbdownload.Start(df.downloadCfg(), df.logger, func(path string) {
		if path == "" || path == df.sourceDbPath {
			return
		}
		if err := df.performHotSwap(path); err != nil {
			df.logger.Error("failed to perform hot swap", "error", err)
		}
	})
	if err != nil {
		df.logger.Error("download slot", "error", err)
		return
	}
	df.download = slot
}

// performHotSwap replaces the current database with a new one
func (df *DatabaseFactory) performHotSwap(newDatabasePath string) error {
	// Create new local copy with unique name
	newLocalCopy, err := df.createLocalDatabaseCopy(newDatabasePath)
	if err != nil {
		return err
	}

	// Open new database
	newDB, err := ip2loc.OpenDB(newLocalCopy)
	if err != nil {
		os.Remove(newLocalCopy)
		return fmt.Errorf("performHotSwap: failed to open new database: %w", err)
	}

	// Get version
	newVersion, err := dbutils.GetDatabaseVersion(newLocalCopy)
	if err != nil {
		newDB.Close()
		os.Remove(newLocalCopy)
		return fmt.Errorf("performHotSwap: failed to read new database version: %w", err)
	}

	// Perform the swap
	oldDB := df.wrapper.swapDatabase(newDB, newLocalCopy, newVersion)

	// Update tracking information
	df.currentLocalDbCopy = newLocalCopy
	df.sourceDbPath = newDatabasePath // Track the new source database

	// Close old database after brief delay for ongoing operations
	if oldDB != nil {
		go func() {
			time.Sleep(10 * time.Second) // Brief delay, not the most elegant approach, but simple. And if it panics, not really a big deal.
			oldDB.Close()
		}()
	}

	df.logger.Info("performHotSwap: database hot-swapped successfully",
		"new_version", newVersion.String(),
		"new_path", newLocalCopy)

	return nil
}

// factories is one DatabaseFactory per config hash (Traefik reloads call New again).
// The map stays in this package; Yaegi cannot type-assert values boxed as any elsewhere.
var (
	factoryLock = dbprovider.NewInstanceLock()
	factories   = map[string]*DatabaseFactory{}
)

// generateConfigHash creates a unique hash key from DatabaseConfig for singleton pattern
func generateConfigHash(config *DatabaseConfig) string {
	// Serialize the config to JSON for consistent hashing
	configBytes, err := json.Marshal(config)
	if err != nil {
		// Fallback to a simple key if marshaling fails
		return fmt.Sprintf("%s_%s_%s_%s",
			config.Download.Path,
			config.DatabaseAutoUpdateDir,
			config.Download.URL,
			config.Download.Key)
	}

	// Generate FNV hash
	hasher := fnv.New32()
	hasher.Write(configBytes)
	return strconv.FormatUint(uint64(hasher.Sum32()), 10)
}

// GetDatabaseFactory returns a singleton database factory for the given configuration
func GetDatabaseFactory(config *DatabaseConfig, logger *slog.Logger) (*DatabaseFactory, error) {
	key := generateConfigHash(config)
	var out *DatabaseFactory
	err := factoryLock.LoadOrStore(func() bool {
		f, ok := factories[key]
		if ok {
			out = f
		}
		return ok
	}, func() error {
		factory, err := NewDatabaseFactory(config, logger)
		if err != nil {
			return err
		}
		factories[key] = factory
		out = factory
		logger.Debug("created new database factory", "config_hash", key)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CleanupFactories closes all database factories (for testing/shutdown)
func CleanupFactories() {
	factoryLock.Reset(func() {
		for key, factory := range factories {
			factory.Close()
			delete(factories, key)
		}
	})
}

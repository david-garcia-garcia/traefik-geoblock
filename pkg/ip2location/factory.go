package ip2location

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"log/slog"

	ip2loc "github.com/ip2location/ip2location-go/v9"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbutils"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
)

// DatabaseConfig is IP2Location-only settings (BIN path and auto-update).
type DatabaseConfig struct {
	DatabaseFilePath        string
	DatabaseAutoUpdate      bool
	DatabaseAutoUpdateDir   string
	DatabaseAutoUpdateToken string
	DatabaseAutoUpdateCode  string

	// ASN LITE is a second BIN. Path is optional; code defaults to DefaultASNDatabaseCode.
	// AsnDatabaseAutoUpdate is opt-in and only downloads when a token is set
	// (ASN LITE is not on the public lite CDN).
	AsnDatabaseFilePath       string
	AsnDatabaseAutoUpdate     bool
	AsnDatabaseAutoUpdateCode string

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
	updateTicker       *time.Ticker
	stopChan           chan struct{}
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
		stopChan:  make(chan struct{}),
		factoryID: factoryID,
	}

	// Initialize the database
	if err := factory.initialize(); err != nil {
		return nil, fmt.Errorf("NewDatabaseFactory: failed to initialize database factory: %w", err)
	}

	// Start auto-update ticker if enabled
	if config.DatabaseAutoUpdate {
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

// Close shuts down the factory and cleans up resources
func (df *DatabaseFactory) Close() error {
	// Stop auto-update ticker
	if df.updateTicker != nil {
		df.updateTicker.Stop()
		close(df.stopChan)
	}

	// Close current database
	if df.wrapper != nil {
		df.wrapper.Close()
	}

	return nil
}

// initialize sets up the initial database using the best available version.
// Auto-update dir wins when it already has a BIN. The configured file path
// (ip2location_databaseFilePath / ip2location_asnDatabaseFilePath) is the seed
// used only when that dir is empty or auto-update is off.
func (df *DatabaseFactory) initialize() error {
	var targetPath string

	if df.config.DatabaseAutoUpdate {
		updatedPath, err := df.handleAutoUpdateInit("")
		if err != nil {
			df.logger.Warn("auto-update initialization failed, using configured database path", "error", err)
		} else if updatedPath != "" {
			targetPath = updatedPath
			df.logger.Debug("using auto-updated database", "path", updatedPath)
		}
	}

	if targetPath == "" {
		resolved, err := df.resolveDatabasePath()
		if err != nil {
			return fmt.Errorf("failed to resolve database path: %w", err)
		}
		targetPath = resolved
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

// resolveDatabasePath determines the best database path based on configuration
func (df *DatabaseFactory) resolveDatabasePath() (string, error) {
	databasePath := df.config.DatabaseFilePath
	defaultName := defaultFileNameForCode(df.config.DatabaseAutoUpdateCode)

	if df.config.AllowMissing {
		if databasePath != "" && fileutils.Exists(databasePath) {
			return databasePath, nil
		}
		return "", nil
	}

	databasePath, err := fileutils.Default.Search(databasePath, defaultName, df.logger)
	if err != nil {
		return "", fmt.Errorf("database file not found: %w", err)
	}

	return databasePath, nil
}

// handleAutoUpdateInit finds the newest available database for initialization (no downloads)
func (df *DatabaseFactory) handleAutoUpdateInit(fallbackPath string) (string, error) {
	if df.config.DatabaseAutoUpdateDir == "" {
		return "", fmt.Errorf("DatabaseAutoUpdateDir must be specified when auto-update is enabled")
	}

	// Try to find the latest database in the auto-update directory
	latest, err := findLatestDatabase(df.config.DatabaseAutoUpdateDir, df.config.DatabaseAutoUpdateCode)
	if err != nil {
		df.logger.Debug("no existing database found in auto-update directory", "error", err)
		// Use fallback database directly for initialization
		return fallbackPath, nil
	}

	if latest != "" {
		df.logger.Debug("found existing database in auto-update directory", "path", latest)
		// Track the original source before creating local copy
		df.sourceDbPath = latest
		// Create local copy for consistent access
		return df.createLocalDatabaseCopy(latest)
	}

	// Use fallback database directly
	return fallbackPath, nil
}

// createLocalDatabaseCopy creates a timestamped local copy that doesn't overwrite existing files
func (df *DatabaseFactory) createLocalDatabaseCopy(sourcePath string) (string, error) {
	// Always create unique timestamped copy with nanoseconds to guarantee uniqueness
	now := time.Now()
	timestamp := fmt.Sprintf("%s_%d", now.Format("20060102_150405"), now.Nanosecond())
	code := defaultDatabaseCode(df.config.DatabaseAutoUpdateCode)
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("IP2LOCATION-LITE-%s.IPV6_%s.BIN", code, timestamp))

	// Copy to temp location
	if err := fileutils.Copy(sourcePath, tmpFile, false); err != nil {
		return "", fmt.Errorf("failed to create local copy: %w", err)
	}

	df.currentLocalDbCopy = tmpFile
	df.logger.Debug("created local database copy", "source", sourcePath, "dest", tmpFile)
	return tmpFile, nil
}

// startAutoUpdate starts the auto-update ticker
func (df *DatabaseFactory) startAutoUpdate() {
	df.updateTicker = time.NewTicker(24 * time.Hour)

	go func() {
		df.logger.Debug("startAutoUpdate: starting auto-update ticker")

		// Run first check immediately
		df.checkAndUpdate()

		for {
			select {
			case <-df.updateTicker.C:
				df.checkAndUpdate()
			case <-df.stopChan:
				df.logger.Debug("startAutoUpdate: stopping auto-update ticker")
				return
			}
		}
	}()
}

// checkAndUpdate checks if an update is needed and performs actual downloads/updates
func (df *DatabaseFactory) checkAndUpdate() {
	currentVersion := df.wrapper.GetVersion()
	if currentVersion != nil && time.Since(currentVersion.Date()) < 30*24*time.Hour {
		df.logger.Debug("checkAndUpdate: database is recent, skipping update", "age", time.Since(currentVersion.Date()).Round(24*time.Hour))
		return
	}
	if currentVersion == nil {
		df.logger.Info("checkAndUpdate: no database open yet, attempting download")
	} else {
		df.logger.Info("checkAndUpdate: database is old, attempting download update", "age", time.Since(currentVersion.Date()).Round(24*time.Hour))
	}

	// Find current latest database
	latest, err := findLatestDatabase(df.config.DatabaseAutoUpdateDir, df.config.DatabaseAutoUpdateCode)
	if err != nil {
		df.logger.Debug("checkAndUpdate: no existing database found during update check", "error", err)
		latest = ""
	}

	// Attempt to download a newer version (actual download happens here)
	updateCfg := &DatabaseConfig{
		DatabaseAutoUpdateDir:   df.config.DatabaseAutoUpdateDir,
		DatabaseAutoUpdateToken: df.config.DatabaseAutoUpdateToken,
		DatabaseAutoUpdateCode:  df.config.DatabaseAutoUpdateCode,
	}

	if err := UpdateIfNeeded(latest, true, df.logger, updateCfg); err != nil {
		df.logger.Error("checkAndUpdate: background database update failed", "error", err)
		return
	}

	// Check if we got a new database
	newLatest, err := findLatestDatabase(df.config.DatabaseAutoUpdateDir, df.config.DatabaseAutoUpdateCode)
	if err != nil {
		df.logger.Error("checkAndUpdate: failed to find latest database after update attempt", "error", err)
		return
	}

	if newLatest == "" {
		df.logger.Debug("checkAndUpdate: no new database found after update attempt")
		return
	}
	if newLatest == latest && currentVersion != nil {
		df.logger.Debug("checkAndUpdate: no new database found after update attempt")
		return
	}

	// Perform hot swap
	if err := df.performHotSwap(newLatest); err != nil {
		df.logger.Error("checkAndUpdate: failed to perform hot swap", "error", err)
	}
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

// Global factory manager
var (
	factoryMutex sync.RWMutex
	factories    = make(map[string]*DatabaseFactory)
)

// generateConfigHash creates a unique hash key from DatabaseConfig for singleton pattern
func generateConfigHash(config *DatabaseConfig) string {
	// Serialize the config to JSON for consistent hashing
	configBytes, err := json.Marshal(config)
	if err != nil {
		// Fallback to a simple key if marshaling fails
		return fmt.Sprintf("%s_%v_%s_%s_%s",
			config.DatabaseFilePath,
			config.DatabaseAutoUpdate,
			config.DatabaseAutoUpdateDir,
			config.DatabaseAutoUpdateToken,
			config.DatabaseAutoUpdateCode)
	}

	// Generate FNV hash
	hasher := fnv.New32()
	hasher.Write(configBytes)
	return strconv.FormatUint(uint64(hasher.Sum32()), 10)
}

// GetDatabaseFactory returns a singleton database factory for the given configuration
func GetDatabaseFactory(config *DatabaseConfig, logger *slog.Logger) (*DatabaseFactory, error) {
	// Generate unique key from the entire configuration
	key := generateConfigHash(config)

	factoryMutex.RLock()
	if factory, exists := factories[key]; exists {
		factoryMutex.RUnlock()
		return factory, nil
	}
	factoryMutex.RUnlock()

	// Create new factory
	factoryMutex.Lock()
	defer factoryMutex.Unlock()

	// Double-check pattern
	if factory, exists := factories[key]; exists {
		return factory, nil
	}

	factory, err := NewDatabaseFactory(config, logger)
	if err != nil {
		return nil, err
	}

	factories[key] = factory
	logger.Debug("created new database factory", "config_hash", key)

	return factory, nil
}

// CleanupFactories closes all database factories (for testing/shutdown)
func CleanupFactories() {
	factoryMutex.Lock()
	defer factoryMutex.Unlock()

	for key, factory := range factories {
		factory.Close()
		delete(factories, key)
	}
}

package traefik_geoblock

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"log/slog"

	"github.com/ip2location/ip2location-go/v9"
)

// DatabaseConfig contains only the configuration needed for database management
type DatabaseConfig struct {
	DatabaseFilePath        string
	DatabaseAutoUpdate      bool
	DatabaseAutoUpdateDir   string
	DatabaseAutoUpdateToken string
	DatabaseAutoUpdateCode  string
}

// DatabaseWrapper wraps ip2location.DB and allows for hot-swapping during updates
type DatabaseWrapper struct {
	db      *ip2location.DB
	path    string
	version *DBVersion
}

// Get_country_short performs IP country lookup (fast path - no locking)
func (dw *DatabaseWrapper) Get_country_short(ip string) (ip2location.IP2Locationrecord, error) {
	return dw.db.Get_country_short(ip)
}

// GetVersion returns the current database version (fast path - no locking)
func (dw *DatabaseWrapper) GetVersion() *DBVersion {
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
func (dw *DatabaseWrapper) swapDatabase(newDB *ip2location.DB, newPath string, newVersion *DBVersion) *ip2location.DB {
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
}

// NewDatabaseFactory creates a new database factory instance
func NewDatabaseFactory(config *DatabaseConfig, logger *slog.Logger) (*DatabaseFactory, error) {
	factory := &DatabaseFactory{
		config:   config,
		logger:   logger,
		wrapper:  &DatabaseWrapper{},
		stopChan: make(chan struct{}),
	}

	// Initialize the database
	if err := factory.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize database factory: %w", err)
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

// initialize sets up the initial database using the best available version
func (df *DatabaseFactory) initialize() error {
	// Determine the target database path
	targetPath, err := df.resolveDatabasePath()
	if err != nil {
		return fmt.Errorf("failed to resolve database path: %w", err)
	}

	df.logger.Debug("initializing database", "path", targetPath)

	// Find the newest available database if auto-update is enabled (no downloads during init)
	if df.config.DatabaseAutoUpdate {
		if updatedPath, err := df.handleAutoUpdateInit(targetPath); err != nil {
			df.logger.Warn("auto-update initialization failed, using fallback database", "error", err)
		} else if updatedPath != "" {
			targetPath = updatedPath
			df.logger.Debug("using auto-updated database", "path", updatedPath)
		}
	}

	// Track the source database path before opening (only if not already set by auto-update)
	if df.sourceDbPath == "" {
		df.sourceDbPath = targetPath
	}

	// Open the database
	db, err := ip2location.OpenDB(targetPath)
	if err != nil {
		return fmt.Errorf("failed to open database %s: %w", targetPath, err)
	}

	// Get and validate database version
	version, err := GetDatabaseVersion(targetPath)
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

	// Search for database file if path is provided
	if databasePath != "" {
		databasePath = searchFile(databasePath, "IP2LOCATION-LITE-DB1.IPV6.BIN", df.logger)
	}

	if databasePath == "" {
		return "", fmt.Errorf("no database file path provided")
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
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("IP2LOCATION-LITE-DB1.IPV6_%s.BIN", timestamp))

	// Copy to temp location
	if err := copyFile(sourcePath, tmpFile, false); err != nil {
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
		df.logger.Debug("starting auto-update ticker")

		// Run first check immediately
		df.checkAndUpdate()

		for {
			select {
			case <-df.updateTicker.C:
				df.checkAndUpdate()
			case <-df.stopChan:
				df.logger.Debug("stopping auto-update ticker")
				return
			}
		}
	}()
}

// checkAndUpdate checks if an update is needed and performs actual downloads/updates
func (df *DatabaseFactory) checkAndUpdate() {
	currentVersion := df.wrapper.GetVersion()
	if currentVersion == nil {
		df.logger.Debug("no current version available, skipping update check")
		return
	}

	// Only update if database is older than 1 month
	if time.Since(currentVersion.Date()) < 30*24*time.Hour {
		df.logger.Debug("database is recent, skipping update", "age", time.Since(currentVersion.Date()).Round(24*time.Hour))
		return
	}

	df.logger.Info("database is old, attempting download update", "age", time.Since(currentVersion.Date()).Round(24*time.Hour))

	// Find current latest database
	latest, err := findLatestDatabase(df.config.DatabaseAutoUpdateDir, df.config.DatabaseAutoUpdateCode)
	if err != nil {
		df.logger.Debug("no existing database found during update check", "error", err)
		latest = ""
	}

	// Attempt to download a newer version (actual download happens here)
	updateCfg := &Config{
		DatabaseAutoUpdateDir:   df.config.DatabaseAutoUpdateDir,
		DatabaseAutoUpdateToken: df.config.DatabaseAutoUpdateToken,
		DatabaseAutoUpdateCode:  df.config.DatabaseAutoUpdateCode,
	}

	if err := UpdateIfNeeded(latest, false, df.logger, updateCfg); err != nil {
		df.logger.Error("background database update failed", "error", err)
		return
	}

	// Check if we got a new database
	newLatest, err := findLatestDatabase(df.config.DatabaseAutoUpdateDir, df.config.DatabaseAutoUpdateCode)
	if err != nil || newLatest == "" || newLatest == latest {
		df.logger.Debug("no new database found after update attempt")
		return
	}

	// Perform hot swap
	if err := df.performHotSwap(newLatest); err != nil {
		df.logger.Error("failed to perform hot swap", "error", err)
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
	newDB, err := ip2location.OpenDB(newLocalCopy)
	if err != nil {
		os.Remove(newLocalCopy)
		return fmt.Errorf("failed to open new database: %w", err)
	}

	// Get version
	newVersion, err := GetDatabaseVersion(newLocalCopy)
	if err != nil {
		newDB.Close()
		os.Remove(newLocalCopy)
		return fmt.Errorf("failed to read new database version: %w", err)
	}

	// Perform the swap
	oldDB := df.wrapper.swapDatabase(newDB, newLocalCopy, newVersion)

	// Update tracking information
	df.currentLocalDbCopy = newLocalCopy
	df.sourceDbPath = newDatabasePath // Track the new source database

	// Close old database after brief delay for ongoing operations
	if oldDB != nil {
		go func() {
			time.Sleep(1 * time.Second) // Brief delay, not the most elegant approach, but simple.
			oldDB.Close()
		}()
	}

	df.logger.Info("database hot-swapped successfully",
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

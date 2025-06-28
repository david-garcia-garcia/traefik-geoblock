package traefik_geoblock

import (
	"fmt"
	"os"
	"path/filepath"
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

// DatabaseFactory manages IP2Location database instances and handles initialization, auto-updates, and caching
type DatabaseFactory struct {
	db           *ip2location.DB
	databasePath string
	version      *DBVersion
	mu           sync.RWMutex
	initialized  bool
}

//nolint:gochecknoglobals
var (
	databaseFactory     *DatabaseFactory
	databaseFactoryOnce sync.Once
)

// GetDatabaseFactory returns the singleton DatabaseFactory instance
func GetDatabaseFactory() *DatabaseFactory {
	databaseFactoryOnce.Do(func() {
		databaseFactory = &DatabaseFactory{}
	})

	return databaseFactory
}

// ResetForTesting resets the global state for testing purposes
// This should ONLY be used in tests
func ResetDatabaseFactoryForTesting() {
	databaseFactory = nil
	databaseFactoryOnce = sync.Once{}
}

// Initialize sets up the database with the given configuration
// This method is idempotent and thread-safe
func (df *DatabaseFactory) Initialize(cfg *DatabaseConfig, logger *slog.Logger) error {
	df.mu.Lock()
	defer df.mu.Unlock()

	// Determine the target database path
	targetPath, err := df.resolveDatabasePath(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to resolve database path: %w", err)
	}

	// If already initialized with the same path, return early
	if df.initialized && df.databasePath == targetPath {
		logger.Debug("database factory already initialized with same path", "path", df.databasePath)
		return nil
	}

	// If we're already initialized with a different path, close and reinitialize
	if df.initialized {
		logger.Debug("reinitializing database factory with new path", "oldPath", df.databasePath, "newPath", targetPath)
		df.closeDatabase()
	}

	logger.Debug("initializing database factory", "path", targetPath)

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

	logger.Info("database factory initialized",
		"path", targetPath,
		"version", version.String(),
		"age", time.Since(version.Date()).Round(24*time.Hour))

	// Check if database is older than 2 months
	if time.Since(version.Date()) > 60*24*time.Hour {
		logger.Warn("ip2location database is more than 2 months old",
			"version", version.String(),
			"age", time.Since(version.Date()).Round(24*time.Hour))
	}

	df.db = db
	df.databasePath = targetPath
	df.version = version
	df.initialized = true

	return nil
}

// resolveDatabasePath determines the best database path based on configuration
func (df *DatabaseFactory) resolveDatabasePath(cfg *DatabaseConfig, logger *slog.Logger) (string, error) {
	databasePath := cfg.DatabaseFilePath

	// Search for database file if path is provided
	if databasePath != "" {
		databasePath = searchFile(databasePath, "IP2LOCATION-LITE-DB1.IPV6.BIN", logger)
	}

	// Handle auto-update configuration
	if cfg.DatabaseAutoUpdate {
		if cfg.DatabaseAutoUpdateDir == "" {
			return "", fmt.Errorf("DatabaseAutoUpdateDir must be specified when auto-update is enabled")
		}

		updatedPath, err := df.handleAutoUpdate(cfg, databasePath, logger)
		if err != nil {
			logger.Warn("auto-update failed, using fallback database", "error", err)
			if databasePath == "" {
				return "", fmt.Errorf("no database available after auto-update failure: %w", err)
			}
		} else if updatedPath != "" {
			databasePath = updatedPath
			logger.Debug("using auto-updated database", "path", updatedPath)
		}
	}

	if databasePath == "" {
		return "", fmt.Errorf("no database file path provided")
	}

	return databasePath, nil
}

// GetDatabase returns a database instance for IP lookups
// This method is thread-safe and can be called from multiple goroutines
func (df *DatabaseFactory) GetDatabase() (*ip2location.DB, error) {
	df.mu.RLock()
	defer df.mu.RUnlock()

	if !df.initialized {
		return nil, fmt.Errorf("database factory not initialized")
	}

	if df.db == nil {
		return nil, fmt.Errorf("database not available")
	}

	return df.db, nil
}

// GetDatabasePath returns the path to the current database file
func (df *DatabaseFactory) GetDatabasePath() string {
	df.mu.RLock()
	defer df.mu.RUnlock()
	return df.databasePath
}

// GetDatabaseVersion returns the version information of the current database
func (df *DatabaseFactory) GetDatabaseVersion() *DBVersion {
	df.mu.RLock()
	defer df.mu.RUnlock()
	return df.version
}

// IsInitialized returns whether the database factory has been initialized
func (df *DatabaseFactory) IsInitialized() bool {
	df.mu.RLock()
	defer df.mu.RUnlock()
	return df.initialized
}

// Close closes the database connection
// This should be called during application shutdown
func (df *DatabaseFactory) Close() error {
	df.mu.Lock()
	defer df.mu.Unlock()

	df.closeDatabase()
	return nil
}

// closeDatabase closes the database without locking (internal helper)
func (df *DatabaseFactory) closeDatabase() {
	if df.db != nil {
		df.db.Close()
		df.db = nil
	}
	df.initialized = false
}

// handleAutoUpdate manages the auto-update process and returns the path to the best available database
func (df *DatabaseFactory) handleAutoUpdate(cfg *DatabaseConfig, fallbackPath string, logger *slog.Logger) (string, error) {
	tmpFile := filepath.Join(os.TempDir(), "IP2LOCATION-LITE-DB1.IPV6.BIN")

	// Check if we already have a temp database from a previous initialization
	if fileExists(tmpFile) {
		logger.Debug("using existing database from temp location", "path", tmpFile)
		return tmpFile, nil
	}

	// Try to find the latest database in the auto-update directory
	latest, err := findLatestDatabase(cfg.DatabaseAutoUpdateDir, cfg.DatabaseAutoUpdateCode)
	if err != nil {
		logger.Debug("no existing database found in auto-update directory", "error", err)
		latest = ""
	}

	var newDatabase string

	if latest != "" {
		// Copy database to temporary location for faster access
		if err := copyFile(latest, tmpFile, false); err != nil {
			logger.Warn("failed to copy database to temp location", "error", err, "source", latest)
			if fileExists(tmpFile) {
				// Another process might have copied it
				newDatabase = tmpFile
			} else {
				newDatabase = latest // Fallback to original location
			}
		} else {
			newDatabase = tmpFile
			logger.Debug("copied database to temp location", "path", tmpFile)
		}
	}

	// Start background update process
	go func() {
		updateCfg := &Config{
			DatabaseAutoUpdateDir:   cfg.DatabaseAutoUpdateDir,
			DatabaseAutoUpdateToken: cfg.DatabaseAutoUpdateToken,
			DatabaseAutoUpdateCode:  cfg.DatabaseAutoUpdateCode,
		}
		if err := UpdateIfNeeded(latest, false, logger, updateCfg); err != nil {
			logger.Error("background database update failed", "error", err)
		}
	}()

	if newDatabase != "" {
		// Validate the database before using it
		if _, err := GetDatabaseVersion(newDatabase); err != nil {
			logger.Warn("invalid database file", "path", newDatabase, "error", err)
			return fallbackPath, fmt.Errorf("invalid database file: %w", err)
		}
		return newDatabase, nil
	}

	// No auto-update database available, use fallback
	return fallbackPath, nil
}

package geodb

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/autoupdate"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/contextawarefactory"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbutils"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
	"github.com/ip2location/ip2location-go/v9"
)

// instance is the internal geo database instance with auto-update capabilities
type instance struct {
	config             *Config
	logger             *slog.Logger
	wrapper            *Wrapper
	currentLocalDbCopy string
	sourceDbPath       string // Track the original database that was used for the current local copy
	updateTicker       *time.Ticker
	stopChan           chan struct{}
	instanceID         string // Unique identifier for this instance
	fileUtils          *fileutils.Utils
}

// newInstance creates a new geo database instance
// This is the creator function used directly by contextawarefactory
func newInstance(ctx context.Context, key configKey) (*instance, error) {
	// Reconstruct config from key fields
	config := &Config{
		DatabaseFilePath:        key.databaseFilePath,
		DatabaseAutoUpdate:      key.databaseAutoUpdate,
		DatabaseAutoUpdateDir:   key.databaseAutoUpdateDir,
		DatabaseAutoUpdateToken: key.databaseAutoUpdateToken,
		DatabaseAutoUpdateCode:  key.databaseAutoUpdateCode,
	}

	// Get the logger for this config (first caller's logger was stored)
	loggerMapMutex.RLock()
	logger := loggerMap[key]
	loggerMapMutex.RUnlock()
	
	if logger == nil {
		return nil, fmt.Errorf("no logger found for config key")

	// Generate unique instance ID for logging
	instanceID := fmt.Sprintf("%d", time.Now().UnixNano())
	wrappedLogger := logger.With("geodb_instance_id", instanceID)

	inst := &instance{
		config:     config,
		logger:     wrappedLogger,
		wrapper:    &Wrapper{},
		stopChan:   make(chan struct{}),
		instanceID: instanceID,
		fileUtils:  fileutils.New(),
	}

	// Initialize the database
	if err := inst.initialize(); err != nil {
		return nil, fmt.Errorf("newInstance: failed to initialize geo database: %w", err)
	}

	// Start auto-update ticker if enabled
	if config.DatabaseAutoUpdate {
		inst.startAutoUpdate()
	}

	logger.Debug("created new geodb instance")
	return inst, nil
}

// GetWrapper returns the database wrapper for use
func (inst *instance) GetWrapper() *Wrapper {
	return inst.wrapper
}

// GetSourceDbPath returns the original database path that was used for the current active database
func (inst *instance) GetSourceDbPath() string {
	return inst.sourceDbPath
}

// GetInstanceID returns the unique identifier for this instance
func (inst *instance) GetInstanceID() string {
	return inst.instanceID
}

// Close shuts down the instance and cleans up resources
func (inst *instance) Close() error {
	// Stop auto-update ticker
	if inst.updateTicker != nil {
		inst.updateTicker.Stop()
		close(inst.stopChan)
	}

	// Close the actual database (not the wrapper's Close which manages the handle)
	if inst.wrapper != nil && inst.wrapper.db != nil {
		inst.wrapper.db.Close()
		inst.wrapper.db = nil
	}

	return nil
}

// initialize sets up the initial database using the best available version
func (inst *instance) initialize() error {
	// Determine the target database path
	targetPath, err := inst.resolveDatabasePath()
	if err != nil {
		return fmt.Errorf("failed to resolve database path: %w", err)
	}

	inst.logger.Debug("initializing database", "path", targetPath)

	// Find the newest available database if auto-update is enabled (no downloads during init)
	if inst.config.DatabaseAutoUpdate {
		if updatedPath, err := inst.handleAutoUpdateInit(targetPath); err != nil {
			inst.logger.Warn("auto-update initialization failed, using fallback database", "error", err)
		} else if updatedPath != "" {
			targetPath = updatedPath
			inst.logger.Debug("using auto-updated database", "path", updatedPath)
		}
	}

	// Track the source database path before opening (only if not already set by auto-update)
	if inst.sourceDbPath == "" {
		inst.sourceDbPath = targetPath
	}

	// Open the database
	db, err := ip2location.OpenDB(targetPath)
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
	inst.wrapper.db = db
	inst.wrapper.path = targetPath
	inst.wrapper.version = version

	inst.logger.Info("database initialized",
		"path", targetPath,
		"version", version.String(),
		"age", time.Since(version.Date()).Round(24*time.Hour))

	// Check if database is older than 2 months
	if time.Since(version.Date()) > 60*24*time.Hour {
		inst.logger.Warn("ip2location database is more than 2 months old",
			"version", version.String(),
			"age", time.Since(version.Date()).Round(24*time.Hour))
	}

	return nil
}

// resolveDatabasePath determines the best database path based on configuration
func (inst *instance) resolveDatabasePath() (string, error) {
	databasePath := inst.config.DatabaseFilePath

	// Search for database file
	databasePath, err := inst.fileUtils.Search(databasePath, "IP2LOCATION-LITE-DB1.IPV6.BIN", inst.logger)
	if err != nil {
		return "", fmt.Errorf("database file not found: %w", err)
	}

	return databasePath, nil
}

// handleAutoUpdateInit finds the newest available database for initialization (no downloads)
func (inst *instance) handleAutoUpdateInit(fallbackPath string) (string, error) {
	if inst.config.DatabaseAutoUpdateDir == "" {
		return "", fmt.Errorf("DatabaseAutoUpdateDir must be specified when auto-update is enabled")
	}

	// Try to find the latest database in the auto-update directory
	latest, err := autoupdate.FindLatestDatabase(inst.config.DatabaseAutoUpdateDir, inst.config.DatabaseAutoUpdateCode)
	if err != nil {
		inst.logger.Debug("no existing database found in auto-update directory", "error", err)
		// Use fallback database directly for initialization
		return fallbackPath, nil
	}

	if latest != "" {
		inst.logger.Debug("found existing database in auto-update directory", "path", latest)
		// Track the original source before creating local copy
		inst.sourceDbPath = latest
		// Create local copy for consistent access
		return inst.createLocalDatabaseCopy(latest)
	}

	// Use fallback database directly
	return fallbackPath, nil
}

// createLocalDatabaseCopy creates a timestamped local copy that doesn't overwrite existing files
func (inst *instance) createLocalDatabaseCopy(sourcePath string) (string, error) {
	// Always create unique timestamped copy with nanoseconds to guarantee uniqueness
	now := time.Now()
	timestamp := fmt.Sprintf("%s_%d", now.Format("20060102_150405"), now.Nanosecond())
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("IP2LOCATION-LITE-DB1.IPV6_%s.BIN", timestamp))

	// Copy to temp location
	if err := inst.fileUtils.Copy(sourcePath, tmpFile, false); err != nil {
		return "", fmt.Errorf("failed to create local copy: %w", err)
	}

	inst.currentLocalDbCopy = tmpFile
	inst.logger.Debug("created local database copy", "source", sourcePath, "dest", tmpFile)
	return tmpFile, nil
}

// startAutoUpdate starts the auto-update ticker
func (inst *instance) startAutoUpdate() {
	inst.updateTicker = time.NewTicker(24 * time.Hour)

	go func() {
		inst.logger.Debug("startAutoUpdate: starting auto-update ticker")

		// Run first check immediately
		inst.checkAndUpdate()

		for {
			select {
			case <-inst.updateTicker.C:
				inst.checkAndUpdate()
			case <-inst.stopChan:
				inst.logger.Debug("startAutoUpdate: stopping auto-update ticker")
				return
			}
		}
	}()
}

// checkAndUpdate checks if an update is needed and performs actual downloads/updates
func (inst *instance) checkAndUpdate() {
	currentVersion := inst.wrapper.GetVersion()
	if currentVersion == nil {
		inst.logger.Debug("checkAndUpdate: no current version available, skipping update check")
		return
	}

	// Only update if database is older than 1 month
	if time.Since(currentVersion.Date()) < 30*24*time.Hour {
		inst.logger.Debug("checkAndUpdate: database is recent, skipping update", "age", time.Since(currentVersion.Date()).Round(24*time.Hour))
		return
	}

	inst.logger.Info("checkAndUpdate: database is old, attempting download update", "age", time.Since(currentVersion.Date()).Round(24*time.Hour))

	// Find current latest database
	latest, err := autoupdate.FindLatestDatabase(inst.config.DatabaseAutoUpdateDir, inst.config.DatabaseAutoUpdateCode)
	if err != nil {
		inst.logger.Debug("checkAndUpdate: no existing database found during update check", "error", err)
		latest = ""
	}

	// Attempt to download a newer version (actual download happens here)
	updateCfg := &autoupdate.Config{
		DatabaseAutoUpdateDir:   inst.config.DatabaseAutoUpdateDir,
		DatabaseAutoUpdateToken: inst.config.DatabaseAutoUpdateToken,
		DatabaseAutoUpdateCode:  inst.config.DatabaseAutoUpdateCode,
	}

	if err := autoupdate.UpdateIfNeeded(latest, true, inst.logger, updateCfg); err != nil {
		inst.logger.Error("checkAndUpdate: background database update failed", "error", err)
		return
	}

	// Check if we got a new database
	newLatest, err := autoupdate.FindLatestDatabase(inst.config.DatabaseAutoUpdateDir, inst.config.DatabaseAutoUpdateCode)
	if err != nil {
		inst.logger.Error("checkAndUpdate: failed to find latest database after update attempt", "error", err)
		return
	}

	if newLatest == "" || newLatest == latest {
		inst.logger.Debug("checkAndUpdate: no new database found after update attempt")
		return
	}

	// Perform hot swap
	if err := inst.performHotSwap(newLatest); err != nil {
		inst.logger.Error("checkAndUpdate: failed to perform hot swap", "error", err)
	}
}

// performHotSwap replaces the current database with a new one
func (inst *instance) PerformHotSwap(newDatabasePath string) error {
	return inst.performHotSwap(newDatabasePath)
}

func (inst *instance) performHotSwap(newDatabasePath string) error {
	// Create new local copy with unique name
	newLocalCopy, err := inst.createLocalDatabaseCopy(newDatabasePath)
	if err != nil {
		return err
	}

	// Open new database
	newDB, err := ip2location.OpenDB(newLocalCopy)
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
	oldDB := inst.wrapper.swapDatabase(newDB, newLocalCopy, newVersion)

	// Update tracking information
	inst.currentLocalDbCopy = newLocalCopy
	inst.sourceDbPath = newDatabasePath // Track the new source database

	// Close old database after brief delay for ongoing operations
	if oldDB != nil {
		go func() {
			time.Sleep(10 * time.Second) // Brief delay, not the most elegant approach, but simple. And if it panics, not really a big deal.
			oldDB.Close()
		}()
	}

	inst.logger.Info("performHotSwap: database hot-swapped successfully",
		"new_version", newVersion.String(),
		"new_path", newLocalCopy)

	return nil
}

// configKey holds only the fields relevant to database instance sharing
// Two plugins with the same database configuration will share the same instance
// Note: logger is NOT included here because it's not part of what makes databases equivalent
type configKey struct {
	databaseFilePath        string
	databaseAutoUpdate      bool
	databaseAutoUpdateDir   string
	databaseAutoUpdateToken string
	databaseAutoUpdateCode  string
}

// Global map to store logger for each config (first caller's logger wins)
var (
	loggerMapMutex sync.RWMutex
	loggerMap      = make(map[configKey]*slog.Logger)
)

// cleanupInstance is the cleanup function called when last reference is released
func cleanupInstance(inst *instance) error {
	if inst != nil {
		return inst.Close()
	}
	return nil
}

// Global context-aware instance manager
var globalInstanceManager = contextawarefactory.NewFactory(newInstance, cleanupInstance)

// DB wraps a Wrapper and keeps the reference counting handle alive
// This is what users get from Get() - they work with it like a Wrapper but it manages the lifecycle
type DB struct {
	*Wrapper // Embedded - all Wrapper methods available directly
	handle   *contextawarefactory.Handle[configKey, *instance]
	inst     *instance // Keep reference to instance for delegated methods
}

// Close releases the handle (decrements reference count)
func (db *DB) Close() error {
	if db.handle != nil {
		return db.handle.Release()
	}
	return nil
}

// PerformHotSwap performs a hot swap
func (db *DB) PerformHotSwap(newDatabasePath string) error {
	return db.inst.PerformHotSwap(newDatabasePath)
}

// GetSourceDbPath returns the source database path
func (db *DB) GetSourceDbPath() string {
	return db.inst.GetSourceDbPath()
}

// Get returns a context-aware geo database for the given configuration.
// Multiple callers with the same database configuration will share the same underlying database.
// When all plugin contexts using this database are cancelled, it is automatically cleaned up.
// Returns a DB that must be closed when done.
func Get(ctx context.Context, config *Config, logger *slog.Logger) (*DB, error) {
	// Create key from only database configuration fields
	// Logger is NOT part of the key - first caller's logger is used
	key := configKey{
		databaseFilePath:        config.DatabaseFilePath,
		databaseAutoUpdate:      config.DatabaseAutoUpdate,
		databaseAutoUpdateDir:   config.DatabaseAutoUpdateDir,
		databaseAutoUpdateToken: config.DatabaseAutoUpdateToken,
		databaseAutoUpdateCode:  config.DatabaseAutoUpdateCode,
	}

	// Store logger for this config (first one wins, subsequent callers' loggers are ignored)
	loggerMapMutex.Lock()
	if _, exists := loggerMap[key]; !exists {
		loggerMap[key] = logger
	}
	loggerMapMutex.Unlock()

	// Get or create instance using contextawarefactory
	handle, err := globalInstanceManager.GetOrCreate(ctx, key)
	if err != nil {
		return nil, err
	}

	// Return a DB that embeds the Wrapper and keeps the handle alive
	inst := handle.Value()
	return &DB{
		Wrapper: inst.GetWrapper(),
		handle:  handle,
		inst:    inst,
	}, nil
}

// CleanupAll closes all geo database instances (for testing/shutdown)
// This forcefully cleans up all instances immediately, useful for testing
func CleanupAll() {
	// Force cleanup all instances in the context-aware factory
	globalInstanceManager.ForceCleanupAll()
	
	// Clear logger map
	loggerMapMutex.Lock()
	loggerMap = make(map[configKey]*slog.Logger)
	loggerMapMutex.Unlock()
}

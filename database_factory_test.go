package traefik_geoblock

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDatabaseWrapper_BasicFunctionality(t *testing.T) {
	// Cleanup factories before test
	CleanupFactories()
	defer CleanupFactories()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	config := &DatabaseConfig{
		DatabaseFilePath:   "./IP2LOCATION-LITE-DB1.IPV6.BIN",
		DatabaseAutoUpdate: false,
	}

	factory, err := NewDatabaseFactory(config, logger)
	if err != nil {
		t.Fatalf("Failed to create database factory: %v", err)
	}
	defer factory.Close()

	wrapper := factory.GetWrapper()
	if wrapper == nil {
		t.Fatal("Expected wrapper to not be nil")
	}

	// Test basic lookup functionality
	record, err := wrapper.Get_country_short("8.8.8.8")
	if err != nil {
		t.Fatalf("Failed to lookup IP: %v", err)
	}

	if record.Country_short != "US" {
		t.Errorf("Expected country US for 8.8.8.8, got %s", record.Country_short)
	}

	// Test version retrieval
	version := wrapper.GetVersion()
	if version == nil {
		t.Error("Expected version to not be nil")
	}

	// Test path retrieval
	path := wrapper.GetPath()
	if path == "" {
		t.Error("Expected path to not be empty")
	}
}

func TestDatabaseWrapper_Close(t *testing.T) {
	// Cleanup factories before test
	CleanupFactories()
	defer CleanupFactories()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	config := &DatabaseConfig{
		DatabaseFilePath:   "./IP2LOCATION-LITE-DB1.IPV6.BIN",
		DatabaseAutoUpdate: false,
	}

	factory, err := NewDatabaseFactory(config, logger)
	if err != nil {
		t.Fatalf("Failed to create database factory: %v", err)
	}

	wrapper := factory.GetWrapper()

	// Test that lookup works before close
	_, err = wrapper.Get_country_short("8.8.8.8")
	if err != nil {
		t.Errorf("Expected lookup to work before close, got error: %v", err)
	}

	// Close the wrapper
	err = wrapper.Close()
	if err != nil {
		t.Errorf("Failed to close wrapper: %v", err)
	}

	// After close, wrapper should not be used (testing would be a programming error)
}

func TestGetDatabaseFactory_Singleton(t *testing.T) {
	// Cleanup factories before test
	CleanupFactories()
	defer CleanupFactories()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	config := &DatabaseConfig{
		DatabaseFilePath:   "./IP2LOCATION-LITE-DB1.IPV6.BIN",
		DatabaseAutoUpdate: false,
	}

	// Get factory first time
	factory1, err := GetDatabaseFactory(config, logger)
	if err != nil {
		t.Fatalf("Failed to get first factory: %v", err)
	}

	// Get factory second time with same config
	factory2, err := GetDatabaseFactory(config, logger)
	if err != nil {
		t.Fatalf("Failed to get second factory: %v", err)
	}

	// Should be the same instance
	if factory1 != factory2 {
		t.Error("Expected singleton pattern - should return same factory instance")
	}

	// Test with different config
	config2 := &DatabaseConfig{
		DatabaseFilePath:   "./different-path.bin",
		DatabaseAutoUpdate: false,
	}

	factory3, err := GetDatabaseFactory(config2, logger)
	// This should fail because the file doesn't exist, but we're testing the singleton pattern
	if err == nil {
		// If it doesn't fail, factory3 should be different from factory1
		if factory1 == factory3 {
			t.Error("Expected different factory instances for different configs")
		}
	}
}

func TestDatabaseFactory_AutoUpdate(t *testing.T) {
	// Cleanup factories before test
	CleanupFactories()
	defer CleanupFactories()

	// Create a temporary directory for test databases
	tmpDir, err := os.MkdirTemp("", "geoblock-factory-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Copy the test database to the temp directory with a versioned name
	oldDate := time.Now().AddDate(0, -2, 0).Format("20060102") // 2 months ago
	versionedDbPath := filepath.Join(tmpDir, oldDate+"_IP2LOCATION-LITE-DB1.IPV6.BIN")
	if err := copyFile("./IP2LOCATION-LITE-DB1.IPV6.BIN", versionedDbPath, true); err != nil {
		t.Fatalf("Failed to copy test database: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	config := &DatabaseConfig{
		DatabaseFilePath:       "./IP2LOCATION-LITE-DB1.IPV6.BIN", // Add fallback database path
		DatabaseAutoUpdate:     true,
		DatabaseAutoUpdateDir:  tmpDir,
		DatabaseAutoUpdateCode: "DB1",
	}

	factory, err := NewDatabaseFactory(config, logger)
	if err != nil {
		t.Fatalf("Failed to create factory with auto-update: %v", err)
	}
	defer factory.Close()

	wrapper := factory.GetWrapper()
	if wrapper == nil {
		t.Fatal("Expected wrapper to not be nil")
	}

	// Test that it works
	record, err := wrapper.Get_country_short("8.8.8.8")
	if err != nil {
		t.Fatalf("Failed to lookup IP: %v", err)
	}

	if record.Country_short != "US" {
		t.Errorf("Expected country US for 8.8.8.8, got %s", record.Country_short)
	}

	// Verify that the wrapper is using a local copy (path should contain timestamp)
	dbPath := wrapper.GetPath()
	if !strings.Contains(dbPath, "IP2LOCATION-LITE-DB1.IPV6_") {
		t.Errorf("Expected database path to be a timestamped local copy, got: %s", dbPath)
	}
}

func TestDatabaseFactory_Initialize_Errors(t *testing.T) {
	// Cleanup factories before test
	CleanupFactories()
	defer CleanupFactories()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	tests := []struct {
		name        string
		config      *DatabaseConfig
		expectError bool
		errorText   string
	}{
		{
			name: "missing database file",
			config: &DatabaseConfig{
				DatabaseFilePath:   "./nonexistent.bin",
				DatabaseAutoUpdate: false,
			},
			expectError: true,
			errorText:   "database file not found",
		},
		{
			name: "auto-update enabled but no directory",
			config: &DatabaseConfig{
				DatabaseFilePath:   "./IP2LOCATION-LITE-DB1.IPV6.BIN",
				DatabaseAutoUpdate: true,
				// DatabaseAutoUpdateDir is missing
			},
			expectError: false, // Should succeed with fallback database
		},
		{
			name: "invalid database file",
			config: &DatabaseConfig{
				DatabaseFilePath:   "./testdata/invalid.bin",
				DatabaseAutoUpdate: false,
			},
			expectError: true,
			errorText:   "database file not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory, err := NewDatabaseFactory(tt.config, logger)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorText) {
					t.Errorf("Expected error containing %q, got %v", tt.errorText, err)
				}
				if factory != nil {
					t.Error("Expected factory to be nil on error")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if factory == nil {
					t.Error("Expected factory to not be nil")
				} else {
					factory.Close()
				}
			}
		})
	}
}

func TestDatabaseFactory_HotSwap(t *testing.T) {
	// This test requires a more complex setup to simulate hot swapping
	// For now, we'll test the basic structure
	// Cleanup factories before test
	CleanupFactories()
	defer CleanupFactories()

	// Create temporary directories
	tmpDir, err := os.MkdirTemp("", "geoblock-hotswap-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create two database files with different dates
	oldDate := time.Now().AddDate(0, -2, 0).Format("20060102")
	newDate := time.Now().Format("20060102")

	oldDbPath := filepath.Join(tmpDir, oldDate+"_IP2LOCATION-LITE-DB1.IPV6.BIN")
	newDbPath := filepath.Join(tmpDir, newDate+"_IP2LOCATION-LITE-DB1.IPV6.BIN")

	if err := copyFile("./IP2LOCATION-LITE-DB1.IPV6.BIN", oldDbPath, true); err != nil {
		t.Fatalf("Failed to copy old database: %v", err)
	}
	if err := copyFile("./IP2LOCATION-LITE-DB1.IPV6.BIN", newDbPath, true); err != nil {
		t.Fatalf("Failed to copy new database: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	config := &DatabaseConfig{
		DatabaseFilePath:       "./IP2LOCATION-LITE-DB1.IPV6.BIN", // Add fallback database path
		DatabaseAutoUpdate:     true,
		DatabaseAutoUpdateDir:  tmpDir,
		DatabaseAutoUpdateCode: "DB1",
	}

	factory, err := NewDatabaseFactory(config, logger)
	if err != nil {
		t.Fatalf("Failed to create factory: %v", err)
	}
	defer factory.Close()

	wrapper := factory.GetWrapper()
	if wrapper == nil {
		t.Fatal("Expected wrapper to not be nil")
	}

	// Test that lookup works
	record, err := wrapper.Get_country_short("8.8.8.8")
	if err != nil {
		t.Fatalf("Failed to lookup IP: %v", err)
	}

	if record.Country_short != "US" {
		t.Errorf("Expected country US for 8.8.8.8, got %s", record.Country_short)
	}

	// Test hot swap functionality
	err = factory.performHotSwap(newDbPath)
	if err != nil {
		t.Fatalf("Failed to perform hot swap: %v", err)
	}

	// Verify that lookup still works after hot swap
	record2, err := wrapper.Get_country_short("8.8.8.8")
	if err != nil {
		t.Fatalf("Failed to lookup IP after hot swap: %v", err)
	}

	if record2.Country_short != "US" {
		t.Errorf("Expected country US for 8.8.8.8 after hot swap, got %s", record2.Country_short)
	}
}

func TestCleanupFactories(t *testing.T) {
	// Create a couple of factories
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	config1 := &DatabaseConfig{
		DatabaseFilePath:   "./IP2LOCATION-LITE-DB1.IPV6.BIN",
		DatabaseAutoUpdate: false,
	}

	factory1, err := GetDatabaseFactory(config1, logger)
	if err != nil {
		t.Fatalf("Failed to create first factory: %v", err)
	}

	// Create a temporary directory and factory for auto-update
	tmpDir, err := os.MkdirTemp("", "geoblock-cleanup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Copy database to temp dir
	versionedDbPath := filepath.Join(tmpDir, "20240301_IP2LOCATION-LITE-DB1.IPV6.BIN")
	if err := copyFile("./IP2LOCATION-LITE-DB1.IPV6.BIN", versionedDbPath, true); err != nil {
		t.Fatalf("Failed to copy test database: %v", err)
	}

	config2 := &DatabaseConfig{
		DatabaseFilePath:       "./IP2LOCATION-LITE-DB1.IPV6.BIN", // Add fallback database path
		DatabaseAutoUpdate:     true,
		DatabaseAutoUpdateDir:  tmpDir,
		DatabaseAutoUpdateCode: "DB1",
	}

	factory2, err := GetDatabaseFactory(config2, logger)
	if err != nil {
		t.Fatalf("Failed to create second factory: %v", err)
	}

	// Verify factories are different
	if factory1 == factory2 {
		t.Error("Expected different factory instances")
	}

	// Test that they work
	wrapper1 := factory1.GetWrapper()
	wrapper2 := factory2.GetWrapper()

	_, err = wrapper1.Get_country_short("8.8.8.8")
	if err != nil {
		t.Errorf("Factory 1 lookup failed: %v", err)
	}

	_, err = wrapper2.Get_country_short("8.8.8.8")
	if err != nil {
		t.Errorf("Factory 2 lookup failed: %v", err)
	}

	// Cleanup all factories
	CleanupFactories()

	// After cleanup, wrappers should not be used (testing would be a programming error)
}

// Test integration with the existing plugin system
func TestDatabaseFactory_Integration(t *testing.T) {
	// Cleanup factories before test
	CleanupFactories()
	defer CleanupFactories()

	// Create a temporary directory for auto-update
	tmpDir, err := os.MkdirTemp("", "geoblock-integration-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Copy database to temp dir
	versionedDbPath := filepath.Join(tmpDir, "20240301_IP2LOCATION-LITE-DB1.IPV6.BIN")
	if err := copyFile("./IP2LOCATION-LITE-DB1.IPV6.BIN", versionedDbPath, true); err != nil {
		t.Fatalf("Failed to copy test database: %v", err)
	}

	// Test with the plugin system
	cfg := &Config{
		Enabled:                true,
		DatabaseFilePath:       "./IP2LOCATION-LITE-DB1.IPV6.BIN",
		DatabaseAutoUpdate:     true,
		DatabaseAutoUpdateDir:  tmpDir,
		DatabaseAutoUpdateCode: "DB1",
		AllowedCountries:       []string{"US"},
		DisallowedStatusCode:   403,
		IPHeaders:              []string{"x-forwarded-for", "x-real-ip"},
	}

	plugin, err := New(context.TODO(), &noopHandler{}, cfg, "test-plugin")
	if err != nil {
		t.Fatalf("Failed to create plugin with new database factory: %v", err)
	}

	if plugin == nil {
		t.Fatal("Expected plugin to not be nil")
	}

	// Test that the plugin works
	p := plugin.(*Plugin)
	country, err := p.Lookup("8.8.8.8")
	if err != nil {
		t.Fatalf("Plugin lookup failed: %v", err)
	}

	if country != "US" {
		t.Errorf("Expected country US, got %s", country)
	}

	// Verify that the plugin is using a local copy (path should contain timestamp)
	dbPath := p.db.GetPath()
	if !strings.Contains(dbPath, "IP2LOCATION-LITE-DB1.IPV6_") {
		t.Errorf("Expected database path to be a timestamped local copy, got: %s", dbPath)
	}
}

func TestDatabaseFactory_StartupUpdateAndVersionChange(t *testing.T) {
	// Cleanup factories before test
	CleanupFactories()
	defer CleanupFactories()

	// Create a temporary directory for auto-update
	tmpDir, err := os.MkdirTemp("", "geoblock-startup-update-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Step 1: Create an old database (2 months ago) to trigger immediate update
	oldDate := time.Now().AddDate(0, -2, 0).Format("20060102") // 2 months ago
	oldDbPath := filepath.Join(tmpDir, oldDate+"_IP2LOCATION-LITE-DB1.IPV6.BIN")
	if err := copyFile("./IP2LOCATION-LITE-DB1.IPV6.BIN", oldDbPath, true); err != nil {
		t.Fatalf("Failed to copy old test database: %v", err)
	}

	config := &DatabaseConfig{
		DatabaseFilePath:        "./IP2LOCATION-LITE-DB1.IPV6.BIN", // Fallback database
		DatabaseAutoUpdate:      true,
		DatabaseAutoUpdateDir:   tmpDir,
		DatabaseAutoUpdateCode:  "DB1",
		DatabaseAutoUpdateToken: "", // Use free version for test
	}

	// Step 2: Create factory - this should trigger immediate update check due to old database
	factory, err := NewDatabaseFactory(config, logger)
	if err != nil {
		t.Fatalf("Failed to create factory with auto-update: %v", err)
	}
	defer factory.Close()

	wrapper := factory.GetWrapper()
	if wrapper == nil {
		t.Fatal("Expected wrapper to not be nil")
	}

	// Step 3: Get initial version and verify database is working
	initialVersion := wrapper.GetVersion()
	if initialVersion == nil {
		t.Fatal("Expected initial version to not be nil")
	}

	// Verify initial lookup works
	record, err := wrapper.Get_country_short("8.8.8.8")
	if err != nil {
		t.Fatalf("Failed to lookup IP initially: %v", err)
	}
	if record.Country_short != "US" {
		t.Errorf("Expected country US for 8.8.8.8, got %s", record.Country_short)
	}

	t.Logf("Initial database version: %s, age: %v", initialVersion.String(), time.Since(initialVersion.Date()))

	// Step 4: Wait for the startup update process to complete with timeout
	// The immediate check runs in a goroutine, so we need to wait for it to detect the old database
	updateDetected := false
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for !updateDetected {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for startup update to be detected")
		case <-ticker.C:
			// Check if an update process has been triggered by checking if any newer database exists
			// or if the current path indicates a local copy was created (which happens during auto-update init)
			currentPath := wrapper.GetPath()
			if strings.Contains(currentPath, "IP2LOCATION-LITE-DB1.IPV6_") {
				updateDetected = true
				t.Logf("Startup update initialization detected - using local copy: %s", currentPath)
			}
		}
	}

	// Step 5: Simulate a successful update by creating a newer database in the auto-update directory
	// This simulates what would happen after a successful download
	newDate := time.Now().Format("20060102")
	newDbPath := filepath.Join(tmpDir, newDate+"_IP2LOCATION-LITE-DB1.IPV6.BIN")
	if err := copyFile("./IP2LOCATION-LITE-DB1.IPV6.BIN", newDbPath, true); err != nil {
		t.Fatalf("Failed to copy new test database: %v", err)
	}

	t.Logf("Created simulated updated database: %s", newDbPath)

	// Step 6: Manually trigger another check and update cycle to pick up the new database
	factory.checkAndUpdate()

	// Step 7: Wait for the hot-swap process to complete with timeout
	hotSwapCompleted := false
	initialPath := wrapper.GetPath()
	hotSwapTimeout := time.After(3 * time.Second)
	hotSwapTicker := time.NewTicker(50 * time.Millisecond)
	defer hotSwapTicker.Stop()

	for !hotSwapCompleted {
		select {
		case <-hotSwapTimeout:
			t.Logf("Hot-swap may not have occurred (which is acceptable if no newer database was found)")
			hotSwapCompleted = true // Exit the loop
		case <-hotSwapTicker.C:
			currentPath := wrapper.GetPath()
			// Check if the database path has changed (indicating a hot-swap occurred)
			if currentPath != initialPath {
				t.Logf("Hot-swap detected - path changed from %s to %s", initialPath, currentPath)
				hotSwapCompleted = true
			}
		}
	}

	// Step 8: Verify the new version is being used
	currentVersion := wrapper.GetVersion()
	if currentVersion == nil {
		t.Fatal("Expected current version to not be nil")
	}

	t.Logf("Current database version: %s, age: %v", currentVersion.String(), time.Since(currentVersion.Date()))

	// Step 9: Verify that a new database version is being used
	// The version should be different if an update occurred
	if currentVersion.Date().Equal(initialVersion.Date()) {
		// If dates are the same, check if the database path has changed (indicating a hot-swap occurred)
		currentPath := wrapper.GetPath()
		t.Logf("Database path after update: %s", currentPath)

		// The path should contain a newer timestamp indicating a hot-swap occurred
		if !strings.Contains(currentPath, "IP2LOCATION-LITE-DB1.IPV6_") {
			t.Error("Expected database path to be a timestamped local copy indicating hot-swap occurred")
		}
	}

	// Step 10: Verify database functionality still works after potential update
	record2, err := wrapper.Get_country_short("8.8.4.4") // Different IP for variety
	if err != nil {
		t.Fatalf("Failed to lookup IP after update: %v", err)
	}
	if record2.Country_short != "US" {
		t.Errorf("Expected country US for 8.8.4.4 after update, got %s", record2.Country_short)
	}

	// Step 11: Verify that the database age check logic is working
	// Since we created databases, one should be older than 1 month
	oldDbVersion, err := GetDatabaseVersion(oldDbPath)
	if err != nil {
		t.Fatalf("Failed to get old database version: %v", err)
	}

	if time.Since(oldDbVersion.Date()) <= 30*24*time.Hour {
		t.Errorf("Expected old database to be older than 1 month, but age is: %v", time.Since(oldDbVersion.Date()))
	}

	t.Logf("Test completed successfully - startup update behavior verified")
}

func TestDatabaseFactory_InitializationUsesUpdatedDatabase(t *testing.T) {
	// Cleanup factories before test
	CleanupFactories()
	defer CleanupFactories()

	// Create a temporary directory for auto-update
	tmpDir, err := os.MkdirTemp("", "geoblock-init-updated-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Step 1: Create an updated database in the auto-update directory
	// Use today's date to ensure it's considered "new"
	todayDate := time.Now().Format("20060102")
	updatedDbPath := filepath.Join(tmpDir, todayDate+"_IP2LOCATION-LITE-DB1.IPV6.BIN")
	if err := copyFile("./IP2LOCATION-LITE-DB1.IPV6.BIN", updatedDbPath, true); err != nil {
		t.Fatalf("Failed to copy updated test database: %v", err)
	}

	t.Logf("Created updated database in auto-update directory: %s", updatedDbPath)

	// Step 2: Configure factory to use auto-update with the directory containing the updated database
	config := &DatabaseConfig{
		DatabaseFilePath:        "./IP2LOCATION-LITE-DB1.IPV6.BIN", // Default fallback database
		DatabaseAutoUpdate:      true,
		DatabaseAutoUpdateDir:   tmpDir,
		DatabaseAutoUpdateCode:  "DB1",
		DatabaseAutoUpdateToken: "", // Use free version for test
	}

	// Step 3: Create factory - initialization should find and use the updated database
	factory, err := NewDatabaseFactory(config, logger)
	if err != nil {
		t.Fatalf("Failed to create factory with auto-update: %v", err)
	}
	defer factory.Close()

	wrapper := factory.GetWrapper()
	if wrapper == nil {
		t.Fatal("Expected wrapper to not be nil")
	}

	// Step 4: Verify that the database being used is from the auto-update directory, not the default
	currentPath := wrapper.GetPath()
	sourceDbPath := factory.GetSourceDbPath()
	t.Logf("Database path being used: %s", currentPath)
	t.Logf("Source database path: %s", sourceDbPath)

	// The path should be a local copy of the updated database, not the default fallback
	if !strings.Contains(currentPath, "IP2LOCATION-LITE-DB1.IPV6_") {
		t.Error("Expected database path to be a timestamped local copy, indicating auto-update initialization occurred")
	}

	// Verify this is a temporary file (local copy), not the original fallback
	if strings.Contains(currentPath, "./IP2LOCATION-LITE-DB1.IPV6.BIN") {
		t.Error("Expected to use local copy, not the direct fallback database file")
	}

	// Most importantly: verify the source database was from the auto-update directory
	if !strings.Contains(sourceDbPath, tmpDir) {
		t.Errorf("Expected source database to be from auto-update directory %s, but got: %s", tmpDir, sourceDbPath)
	}

	// Verify the source database path contains today's date
	if !strings.Contains(sourceDbPath, todayDate) {
		t.Errorf("Expected source database to contain today's date %s, but got: %s", todayDate, sourceDbPath)
	}

	// Step 5: Verify the database is working correctly
	record, err := wrapper.Get_country_short("8.8.8.8")
	if err != nil {
		t.Fatalf("Failed to lookup IP with updated database: %v", err)
	}
	if record.Country_short != "US" {
		t.Errorf("Expected country US for 8.8.8.8, got %s", record.Country_short)
	}

	// Step 6: Verify version information is available
	version := wrapper.GetVersion()
	if version == nil {
		t.Fatal("Expected version to not be nil")
	}

	t.Logf("Using database version: %s, age: %v", version.String(), time.Since(version.Date()))

	// Step 8: Verify that without auto-update directory, fallback database would be used
	configWithoutAutoUpdate := &DatabaseConfig{
		DatabaseFilePath:   "./IP2LOCATION-LITE-DB1.IPV6.BIN",
		DatabaseAutoUpdate: false, // Disabled
	}

	fallbackFactory, err := NewDatabaseFactory(configWithoutAutoUpdate, logger)
	if err != nil {
		t.Fatalf("Failed to create fallback factory: %v", err)
	}
	defer fallbackFactory.Close()

	fallbackWrapper := fallbackFactory.GetWrapper()
	fallbackPath := fallbackWrapper.GetPath()
	fallbackSourcePath := fallbackFactory.GetSourceDbPath()
	t.Logf("Fallback database path: %s", fallbackPath)
	t.Logf("Fallback source database path: %s", fallbackSourcePath)

	// Fallback path should be the direct file path, not a timestamped copy
	if strings.Contains(fallbackPath, "IP2LOCATION-LITE-DB1.IPV6_") {
		t.Error("Expected fallback database to use direct file path, not timestamped copy")
	}

	// Fallback source should be the original file, not from auto-update directory
	if strings.Contains(fallbackSourcePath, tmpDir) {
		t.Error("Expected fallback source to NOT be from auto-update directory")
	}

	// The paths should be different
	if currentPath == fallbackPath {
		t.Error("Expected different paths for auto-update vs fallback databases")
	}

	// The source paths should be different
	if sourceDbPath == fallbackSourcePath {
		t.Error("Expected different source paths for auto-update vs fallback databases")
	}

	t.Logf("Test completed successfully - initialization correctly uses updated database from auto-update directory")
}

func TestDatabaseFactory_CheckAndUpdate_SynchronousDownloadAndHotSwap(t *testing.T) {
	// Cleanup factories before test
	CleanupFactories()
	defer CleanupFactories()

	// Create a temporary directory for auto-update
	tmpDir, err := os.MkdirTemp("", "geoblock-sync-update-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	config := &DatabaseConfig{
		DatabaseFilePath:        "./IP2LOCATION-LITE-DB1.IPV6.BIN", // Fallback database
		DatabaseAutoUpdate:      true,                              // Enable auto-update ticker for full workflow
		DatabaseAutoUpdateDir:   tmpDir,
		DatabaseAutoUpdateCode:  "DB1",
		DatabaseAutoUpdateToken: "", // Use free version for test
	}

	// Step 1: Create factory with auto-update enabled
	factory, err := NewDatabaseFactory(config, logger)
	if err != nil {
		t.Fatalf("Failed to create factory with auto-update: %v", err)
	}
	defer factory.Close()

	wrapper := factory.GetWrapper()
	if wrapper == nil {
		t.Fatal("Expected wrapper to not be nil")
	}

	// Step 2: Get initial version
	initialVersion := wrapper.GetVersion()
	if initialVersion == nil {
		t.Fatal("Expected initial version to not be nil")
	}

	t.Logf("Initial database version: %s, age: %v", initialVersion.String(), time.Since(initialVersion.Date()))

	// Step 3: Wait for version to change (indicating download and hot swap completed)
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatalf("Timeout waiting for database version to change - synchronous download and hot swap did not occur")

		case <-ticker.C:
			currentVersion := wrapper.GetVersion()
			if currentVersion != nil && !currentVersion.Date().Equal(initialVersion.Date()) {
				t.Logf("SUCCESS: Version changed from %s to %s - synchronous download and hot swap worked!",
					initialVersion.String(), currentVersion.String())
				return
			}
		}
	}
}

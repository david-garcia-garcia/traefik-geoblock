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
			errorText:   "failed to open database",
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
			errorText:   "failed to open database",
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

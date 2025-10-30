package geodb

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
)

const testDbFilePath = "../../IP2LOCATION-LITE-DB1.IPV6.BIN"

func TestWrapper_BasicFunctionality(t *testing.T) {
	// Cleanup factories before test
	CleanupAll()
	defer CleanupAll()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	ctx := context.Background()

	config := &Config{
		DatabaseFilePath:   testDbFilePath,
		DatabaseAutoUpdate: false,
	}

	db, err := Get(ctx, config, logger)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Test basic lookup functionality
	record, err := db.Get_country_short("8.8.8.8")
	if err != nil {
		t.Fatalf("Failed to lookup IP: %v", err)
	}

	if record.Country_short != "US" {
		t.Errorf("Expected country US for 8.8.8.8, got %s", record.Country_short)
	}

	// Test version retrieval
	version := db.GetVersion()
	if version == nil {
		t.Error("Expected version to not be nil")
	}

	// Test path retrieval
	path := db.GetPath()
	if path == "" {
		t.Error("Expected path to not be empty")
	}
}

func TestWrapper_Close(t *testing.T) {
	// Cleanup factories before test
	CleanupAll()
	defer CleanupAll()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	ctx := context.Background()

	config := &Config{
		DatabaseFilePath:   testDbFilePath,
		DatabaseAutoUpdate: false,
	}

	db, err := Get(ctx, config, logger)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	// Test that lookup works before close
	_, err = db.Get_country_short("8.8.8.8")
	if err != nil {
		t.Errorf("Expected lookup to work before close, got error: %v", err)
	}

	// Close the database
	err = db.Close()
	if err != nil {
		t.Errorf("Failed to close wrapper: %v", err)
	}

	// After close, wrapper should not be used (testing would be a programming error)
}

func TestGet_Singleton(t *testing.T) {
	// Cleanup factories before test
	CleanupAll()
	defer CleanupAll()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	ctx := context.Background()

	config := &Config{
		DatabaseFilePath:   testDbFilePath,
		DatabaseAutoUpdate: false,
	}

	// Get factory first time
	db1, err := Get(ctx, config, logger)
	if err != nil {
		t.Fatalf("Failed to get first factory: %v", err)
	}

	// Get factory second time with same config
	db2, err := Get(ctx, config, logger)
	if err != nil {
		t.Fatalf("Failed to get second factory: %v", err)
	}

	// The Wrapper objects themselves will be different (different handles)
	// But they should share the same underlying instance (verified via path)
	if db1 == db2 {
		t.Error("Expected different wrapper objects (different handles)")
	}

	// They should have the same database path (proving they share the underlying instance)
	if db1.GetPath() != db2.GetPath() {
		t.Error("Expected same database path (shared underlying instance)")
	}

	// Test with different config
	config2 := &Config{
		DatabaseFilePath:   "./different-path.bin",
		DatabaseAutoUpdate: false,
	}

	db3, err := Get(ctx, config2, logger)
	// This should fail because the file doesn't exist, but we're testing the singleton pattern
	if err == nil {
		// If it doesn't fail, db3 should be different from db1
		if db1 == db3 {
			t.Error("Expected different database instances for different configs")
		}
	}
}

func TestGeoDB_AutoUpdate(t *testing.T) {
	// Cleanup instances before test
	CleanupAll()
	defer CleanupAll()

	// Create a temporary directory for test databases
	tmpDir, err := os.MkdirTemp("", "geoblock-geodb-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	fu := fileutils.New()

	// Copy the test database to the temp directory with a versioned name
	oldDate := time.Now().AddDate(0, -2, 0).Format("20060102") // 2 months ago
	versionedDbPath := filepath.Join(tmpDir, oldDate+"_IP2LOCATION-LITE-DB1.IPV6.BIN")
	if err := fu.Copy(testDbFilePath, versionedDbPath, true); err != nil {
		t.Fatalf("Failed to copy test database: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	ctx := context.Background()

	config := &Config{
		DatabaseFilePath:        testDbFilePath, // Add fallback database path
		DatabaseAutoUpdate:      true,
		DatabaseAutoUpdateDir:   tmpDir,
		DatabaseAutoUpdateCode:  "DB1",
		DatabaseAutoUpdateToken: "",
	}

	db, err := Get(ctx, config, logger)
	if err != nil {
		t.Fatalf("Failed to create database with auto-update: %v", err)
	}
	defer db.Close()

	// Test that it works
	record, err := db.Get_country_short("8.8.8.8")
	if err != nil {
		t.Fatalf("Failed to lookup IP: %v", err)
	}

	if record.Country_short != "US" {
		t.Errorf("Expected country US for 8.8.8.8, got %s", record.Country_short)
	}

	// Verify that the database is using a local copy (path should contain timestamp)
	dbPath := db.GetPath()
	if !strings.Contains(dbPath, "IP2LOCATION-LITE-DB1.IPV6_") {
		t.Errorf("Expected database path to be a timestamped local copy, got: %s", dbPath)
	}
}

func TestGeoDB_HotSwap(t *testing.T) {
	// Cleanup instances before test
	CleanupAll()
	defer CleanupAll()

	// Create temporary directories
	tmpDir, err := os.MkdirTemp("", "geoblock-hotswap-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	fu := fileutils.New()

	// Create two database files with different dates
	oldDate := time.Now().AddDate(0, -2, 0).Format("20060102")
	newDate := time.Now().Format("20060102")

	oldDbPath := filepath.Join(tmpDir, oldDate+"_IP2LOCATION-LITE-DB1.IPV6.BIN")
	newDbPath := filepath.Join(tmpDir, newDate+"_IP2LOCATION-LITE-DB1.IPV6.BIN")

	if err := fu.Copy(testDbFilePath, oldDbPath, true); err != nil {
		t.Fatalf("Failed to copy old database: %v", err)
	}
	if err := fu.Copy(testDbFilePath, newDbPath, true); err != nil {
		t.Fatalf("Failed to copy new database: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	ctx := context.Background()

	config := &Config{
		DatabaseFilePath:        testDbFilePath, // Add fallback database path
		DatabaseAutoUpdate:      true,
		DatabaseAutoUpdateDir:   tmpDir,
		DatabaseAutoUpdateCode:  "DB1",
		DatabaseAutoUpdateToken: "",
	}

	db, err := Get(ctx, config, logger)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Test that lookup works
	record, err := db.Get_country_short("8.8.8.8")
	if err != nil {
		t.Fatalf("Failed to lookup IP: %v", err)
	}

	if record.Country_short != "US" {
		t.Errorf("Expected country US for 8.8.8.8, got %s", record.Country_short)
	}

	// Test hot swap functionality
	err = db.PerformHotSwap(newDbPath)
	if err != nil {
		t.Fatalf("Failed to perform hot swap: %v", err)
	}

	// Verify that lookup still works after hot swap
	record2, err := db.Get_country_short("8.8.8.8")
	if err != nil {
		t.Fatalf("Failed to lookup IP after hot swap: %v", err)
	}

	if record2.Country_short != "US" {
		t.Errorf("Expected country US for 8.8.8.8 after hot swap, got %s", record2.Country_short)
	}
}

func TestCleanupAll(t *testing.T) {
	// Create a couple of database instances
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	ctx := context.Background()

	config1 := &Config{
		DatabaseFilePath:   testDbFilePath,
		DatabaseAutoUpdate: false,
	}

	db1, err := Get(ctx, config1, logger)
	if err != nil {
		t.Fatalf("Failed to create first factory: %v", err)
	}

	// Create a temporary directory and factory for auto-update
	tmpDir, err := os.MkdirTemp("", "geoblock-cleanup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	fu := fileutils.New()

	// Copy database to temp dir
	versionedDbPath := filepath.Join(tmpDir, "20240301_IP2LOCATION-LITE-DB1.IPV6.BIN")
	if err := fu.Copy(testDbFilePath, versionedDbPath, true); err != nil {
		t.Fatalf("Failed to copy test database: %v", err)
	}

	config2 := &Config{
		DatabaseFilePath:        testDbFilePath, // Add fallback database path
		DatabaseAutoUpdate:      true,
		DatabaseAutoUpdateDir:   tmpDir,
		DatabaseAutoUpdateCode:  "DB1",
		DatabaseAutoUpdateToken: "",
	}

	db2, err := Get(ctx, config2, logger)
	if err != nil {
		t.Fatalf("Failed to create second factory: %v", err)
	}

	// Verify databases are different
	if db1 == db2 {
		t.Error("Expected different database instances")
	}

	// Test that they work
	_, err = db1.Get_country_short("8.8.8.8")
	if err != nil {
		t.Errorf("DB 1 lookup failed: %v", err)
	}

	_, err = db2.Get_country_short("8.8.8.8")
	if err != nil {
		t.Errorf("DB 2 lookup failed: %v", err)
	}

	// Cleanup all instances
	CleanupAll()

	// After cleanup, databases should not be used (testing would be a programming error)
}

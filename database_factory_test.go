package traefik_geoblock

import (
	"log/slog"
	"os"
	"testing"
)

func TestDatabaseFactory_Singleton(t *testing.T) {
	// Reset global state for test
	ResetDatabaseFactoryForTesting()

	factory1 := GetDatabaseFactory()
	factory2 := GetDatabaseFactory()

	if factory1 != factory2 {
		t.Error("expected same factory instance (singleton pattern)")
	}

	if factory1 == nil {
		t.Error("expected non-nil factory")
	}
}

func TestDatabaseFactory_Initialize(t *testing.T) {
	// Reset global state for test
	ResetDatabaseFactoryForTesting()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	factory := GetDatabaseFactory()

	// Test successful initialization
	dbConfig := &DatabaseConfig{
		DatabaseFilePath: dbFilePath,
	}

	err := factory.Initialize(dbConfig, logger)
	if err != nil {
		t.Fatalf("expected successful initialization, got error: %v", err)
	}

	if !factory.IsInitialized() {
		t.Error("expected factory to be initialized")
	}

	// Test that database instance is available
	db, err := factory.GetDatabase()
	if err != nil {
		t.Fatalf("expected to get database instance, got error: %v", err)
	}

	if db == nil {
		t.Error("expected non-nil database instance")
	}

	// Test that path is correct
	if factory.GetDatabasePath() != dbFilePath {
		t.Errorf("expected database path %s, got %s", dbFilePath, factory.GetDatabasePath())
	}
}

func TestDatabaseFactory_InitializeWithInvalidPath(t *testing.T) {
	// Reset global state for test
	ResetDatabaseFactoryForTesting()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	factory := GetDatabaseFactory()

	// Test initialization with invalid database path
	dbConfig := &DatabaseConfig{
		DatabaseFilePath: "/nonexistent/path/database.bin",
	}

	err := factory.Initialize(dbConfig, logger)
	if err == nil {
		t.Error("expected initialization to fail with invalid path")
	}

	// Ensure no state is cached after failure
	if factory.IsInitialized() {
		t.Error("expected factory not to be initialized after failure")
	}

	// Should not be able to get database instance after failed initialization
	db, err := factory.GetDatabase()
	if err == nil {
		t.Error("expected error when getting database from uninitialized factory")
	}
	if db != nil {
		t.Error("expected nil database from uninitialized factory")
	}
}

func TestDatabaseFactory_Reinitialize(t *testing.T) {
	// Reset global state for test
	ResetDatabaseFactoryForTesting()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	factory := GetDatabaseFactory()

	// Initialize with first path
	dbConfig1 := &DatabaseConfig{
		DatabaseFilePath: dbFilePath,
	}

	err := factory.Initialize(dbConfig1, logger)
	if err != nil {
		t.Fatalf("expected successful first initialization, got error: %v", err)
	}

	firstPath := factory.GetDatabasePath()

	// Test reinitialization with same path (should be idempotent)
	err = factory.Initialize(dbConfig1, logger)
	if err != nil {
		t.Fatalf("expected successful reinitialization with same path, got error: %v", err)
	}

	if factory.GetDatabasePath() != firstPath {
		t.Error("expected same path after reinitialization with same config")
	}
}

func TestDatabaseFactory_AutoUpdate(t *testing.T) {
	// Reset global state for test
	ResetDatabaseFactoryForTesting()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	factory := GetDatabaseFactory()

	// Test auto-update without directory (should fail)
	dbConfig := &DatabaseConfig{
		DatabaseFilePath:   dbFilePath,
		DatabaseAutoUpdate: true,
		// DatabaseAutoUpdateDir intentionally empty
	}

	err := factory.Initialize(dbConfig, logger)
	if err == nil {
		t.Error("expected initialization to fail when auto-update is enabled but no directory is specified")
	}

	// Ensure no state is cached after failure
	if factory.IsInitialized() {
		t.Error("expected factory not to be initialized after failure")
	}
}

func TestDatabaseFactory_Close(t *testing.T) {
	// Reset global state for test
	ResetDatabaseFactoryForTesting()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	factory := GetDatabaseFactory()

	// Initialize factory
	dbConfig := &DatabaseConfig{
		DatabaseFilePath: dbFilePath,
	}

	err := factory.Initialize(dbConfig, logger)
	if err != nil {
		t.Fatalf("expected successful initialization, got error: %v", err)
	}

	// Verify it's initialized
	if !factory.IsInitialized() {
		t.Error("expected factory to be initialized before close")
	}

	// Close the factory
	err = factory.Close()
	if err != nil {
		t.Errorf("expected successful close, got error: %v", err)
	}

	// Verify it's no longer initialized
	if factory.IsInitialized() {
		t.Error("expected factory not to be initialized after close")
	}

	// Should not be able to get database instance after close
	db, err := factory.GetDatabase()
	if err == nil {
		t.Error("expected error when getting database from closed factory")
	}
	if db != nil {
		t.Error("expected nil database from closed factory")
	}
}

package bufferedwriter

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/logging"
)

func TestWriter_BasicWrite(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "writer-test-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	logger := logging.CreateBootstrapLogger("test", "debug")
	ctx := context.Background()

	writer, err := New(ctx, tmpFile.Name(), 1024, 100*time.Millisecond, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()

	testData := []byte("test log line\n")
	_, err = writer.Write(testData)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for flush timer to fire
	time.Sleep(150 * time.Millisecond)

	// Verify data was written
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	if string(content) != string(testData) {
		t.Errorf("expected %q, got %q", string(testData), string(content))
	}
}

func TestWriter_GoroutineLifecycle(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "writer-test-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	logger := logging.CreateBootstrapLogger("test", "debug")
	ctx := context.Background()

	initial := runtime.NumGoroutine()

	writer, err := New(ctx, tmpFile.Name(), 1024, 100*time.Millisecond, logger)
	if err != nil {
		t.Fatal(err)
	}

	testData := []byte("test data\n")
	_, err = writer.Write(testData)
	if err != nil {
		t.Fatal(err)
	}

	afterCreate := runtime.NumGoroutine()
	if afterCreate <= initial {
		t.Error("goroutine should have started")
	}

	// Write more data
	testData2 := []byte("more data\n")
	_, err = writer.Write(testData2)
	if err != nil {
		t.Fatal(err)
	}

	// Close the writer - THIS should stop the goroutine
	writer.Close()
	time.Sleep(200 * time.Millisecond)

	afterClose := runtime.NumGoroutine()
	if afterClose > initial {
		t.Errorf("goroutine should have exited after close: initial=%d afterCreate=%d afterClose=%d", initial, afterCreate, afterClose)
	}

	// Verify all data was written
	content, _ := os.ReadFile(tmpFile.Name())
	expected := string(testData) + string(testData2)
	if string(content) != expected {
		t.Errorf("expected %q, got %q", expected, string(content))
	}
}

func TestWriter_SharedWriter(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "writer-test-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	logger := logging.CreateBootstrapLogger("test", "debug")
	ctx := context.Background()

	// Create first writer handle
	writer1, err := New(ctx, tmpFile.Name(), 1024, 100*time.Millisecond, logger)
	if err != nil {
		t.Fatal(err)
	}

	// Check factory stats - should have 1 instance with 1 reference
	stats := GetFactoryStats()
	if stats.InstanceCount != 1 || stats.TotalRefCount != 1 {
		t.Errorf("expected 1 instance with 1 ref, got %d instances with %d refs", stats.InstanceCount, stats.TotalRefCount)
	}

	// Create second writer handle to same file
	writer2, err := New(ctx, tmpFile.Name(), 1024, 100*time.Millisecond, logger)
	if err != nil {
		t.Fatal(err)
	}

	// Check factory stats - should still be 1 instance but with 2 references
	stats = GetFactoryStats()
	if stats.InstanceCount != 1 || stats.TotalRefCount != 2 {
		t.Errorf("expected 1 instance with 2 refs, got %d instances with %d refs", stats.InstanceCount, stats.TotalRefCount)
	}

	// Write from both handles
	testData1 := []byte("from writer1\n")
	testData2 := []byte("from writer2\n")

	_, err = writer1.Write(testData1)
	if err != nil {
		t.Fatal(err)
	}

	_, err = writer2.Write(testData2)
	if err != nil {
		t.Fatal(err)
	}

	// Close first handle
	writer1.Close()

	// Check factory stats - should have 1 reference left
	stats = GetFactoryStats()
	if stats.InstanceCount != 1 || stats.TotalRefCount != 1 {
		t.Errorf("after closing one handle, expected 1 instance with 1 ref, got %d instances with %d refs", stats.InstanceCount, stats.TotalRefCount)
	}

	// Close second handle
	writer2.Close()

	// Wait for cleanup
	time.Sleep(150 * time.Millisecond)

	// Check factory stats - should be empty now
	stats = GetFactoryStats()
	if stats.InstanceCount != 0 || stats.TotalRefCount != 0 {
		t.Errorf("after closing all handles, expected no instances, got %d instances with %d refs", stats.InstanceCount, stats.TotalRefCount)
	}

	// Verify all data was written
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	expected := string(testData1) + string(testData2)
	if string(content) != expected {
		t.Errorf("expected %q, got %q", expected, string(content))
	}
}

func TestWriter_GoroutineCleanup(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "writer-test-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	logger := logging.CreateBootstrapLogger("test", "debug")
	ctx := context.Background()

	initial := runtime.NumGoroutine()

	// Create multiple handles to same file
	handle1, err := New(ctx, tmpFile.Name(), 1024, 100*time.Millisecond, logger)
	if err != nil {
		t.Fatal(err)
	}

	handle2, err := New(ctx, tmpFile.Name(), 1024, 100*time.Millisecond, logger)
	if err != nil {
		t.Fatal(err)
	}

	handle3, err := New(ctx, tmpFile.Name(), 1024, 100*time.Millisecond, logger)
	if err != nil {
		t.Fatal(err)
	}

	afterCreate := runtime.NumGoroutine()
	// Context watcher goroutines + one flush timer = 4 new goroutines
	// (3 context watchers + 1 shared flush timer)
	if afterCreate <= initial {
		t.Error("expected more goroutines after creating handles")
	}

	// Factory should show 1 writer with 3 references
	stats := GetFactoryStats()
	if stats.InstanceCount != 1 || stats.TotalRefCount != 3 {
		t.Errorf("expected 1 instance with 3 refs, got %d instances with %d refs", stats.InstanceCount, stats.TotalRefCount)
	}

	// Close all handles
	handle1.Close()
	handle2.Close()
	handle3.Close()

	// Wait for goroutine cleanup
	time.Sleep(200 * time.Millisecond)

	afterClose := runtime.NumGoroutine()
	if afterClose > initial+2 { // Allow small margin
		t.Errorf("goroutines should have been cleaned up: initial=%d afterCreate=%d afterClose=%d",
			initial, afterCreate, afterClose)
	}

	// Factory should be empty
	stats = GetFactoryStats()
	if stats.InstanceCount != 0 {
		t.Errorf("expected no instances after cleanup, got %d", stats.InstanceCount)
	}
}

func TestWriter_WriteAfterClose(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "writer-test-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	logger := logging.CreateBootstrapLogger("test", "debug")
	ctx := context.Background()

	writer, err := New(ctx, tmpFile.Name(), 1024, 100*time.Millisecond, logger)
	if err != nil {
		t.Fatal(err)
	}

	// Write some data
	testData := []byte("test data\n")
	_, err = writer.Write(testData)
	if err != nil {
		t.Fatal(err)
	}

	// Close the writer
	writer.Close()

	// Try to write after close - should get error
	_, err = writer.Write([]byte("should fail\n"))
	if err == nil {
		t.Error("expected error when writing to closed writer")
	}

	if err != os.ErrClosed {
		t.Errorf("expected os.ErrClosed, got %v", err)
	}
}

func TestWriter_DifferentConfigurations(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "writer-test-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	logger := logging.CreateBootstrapLogger("test", "debug")
	ctx := context.Background()

	// Create writer with config 1: maxSize=1024, timeout=100ms
	writer1, err := New(ctx, tmpFile.Name(), 1024, 100*time.Millisecond, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer writer1.Close()

	// Should have 1 instance
	stats := GetFactoryStats()
	if stats.InstanceCount != 1 {
		t.Errorf("expected 1 instance, got %d", stats.InstanceCount)
	}

	// Create writer with config 2: same path, different maxSize
	writer2, err := New(ctx, tmpFile.Name(), 2048, 100*time.Millisecond, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer writer2.Close()

	// Should have 2 instances now (different configs)
	stats = GetFactoryStats()
	if stats.InstanceCount != 2 {
		t.Errorf("expected 2 instances for different maxSize, got %d", stats.InstanceCount)
	}

	// Create writer with config 3: same path, different timeout
	writer3, err := New(ctx, tmpFile.Name(), 1024, 200*time.Millisecond, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer writer3.Close()

	// Should have 3 instances now (all different configs)
	stats = GetFactoryStats()
	if stats.InstanceCount != 3 {
		t.Errorf("expected 3 instances for different timeout, got %d", stats.InstanceCount)
	}

	// Create another writer with same config as writer1 - should share
	writer4, err := New(ctx, tmpFile.Name(), 1024, 100*time.Millisecond, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer writer4.Close()

	// Should still have 3 instances but 4 total references
	stats = GetFactoryStats()
	if stats.InstanceCount != 3 || stats.TotalRefCount != 4 {
		t.Errorf("expected 3 instances with 4 refs, got %d instances with %d refs", stats.InstanceCount, stats.TotalRefCount)
	}
}

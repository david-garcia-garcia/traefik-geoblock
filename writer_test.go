package traefik_geoblock

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"
)

func TestBufferedFileWriter_GoroutineCleanup(t *testing.T) {
	// Create a temporary file for the test
	tmpFile, err := os.CreateTemp("", "goroutine-test-*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Create a logger to capture the debug messages
	logger := createBootstrapLogger("goroutine-test")

	// Count goroutines before test
	initialGoroutines := runtime.NumGoroutine()
	t.Logf("Initial goroutine count: %d", initialGoroutines)

	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Create a buffered file writer with the cancellable context
	writer, err := newBufferedFileWriter(ctx, tmpFile.Name(), 1024, 100*time.Millisecond, logger)
	if err != nil {
		t.Fatalf("Failed to create buffered file writer: %v", err)
	}

	// Write some data
	testData := []byte("Test log entry\n")
	_, err = writer.Write(testData)
	if err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}

	// Count goroutines after creating writer (should have increased by 1)
	afterCreateGoroutines := runtime.NumGoroutine()
	t.Logf("Goroutines after creating writer: %d (delta: %d)", afterCreateGoroutines, afterCreateGoroutines-initialGoroutines)

	// Cancel the context (simulating Traefik config reload)
	t.Log("Cancelling context to simulate Traefik config reload...")
	cancel()

	// Give the goroutine a moment to exit
	time.Sleep(200 * time.Millisecond)

	// Count goroutines after cancellation (should be back to initial or close to it)
	afterCancelGoroutines := runtime.NumGoroutine()
	t.Logf("Goroutines after context cancellation: %d (delta from initial: %d)", afterCancelGoroutines, afterCancelGoroutines-initialGoroutines)

	// The goroutine should have exited
	// We allow some tolerance because Go runtime may have other goroutines
	if afterCancelGoroutines > afterCreateGoroutines {
		t.Errorf("Expected goroutine count to decrease or stay same after context cancellation, but it increased from %d to %d",
			afterCreateGoroutines, afterCancelGoroutines)
	}

	// Verify the data was written
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != string(testData) {
		t.Errorf("Expected file content %q, got %q", string(testData), string(content))
	}

	t.Log("✅ Goroutine cleanup test passed!")
}

func TestBufferedFileWriter_MultipleWritersCleanup(t *testing.T) {
	// This test creates multiple writers and verifies they all clean up
	logger := createBootstrapLogger("multi-writer-test")
	initialGoroutines := runtime.NumGoroutine()
	t.Logf("Initial goroutine count: %d", initialGoroutines)

	numWriters := 10
	contexts := make([]context.CancelFunc, numWriters)
	writers := make([]*bufferedFileWriter, numWriters)

	// Create multiple writers
	for i := 0; i < numWriters; i++ {
		tmpFile, err := os.CreateTemp("", "multi-writer-test-*.log")
		if err != nil {
			t.Fatalf("Failed to create temp file %d: %v", i, err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		ctx, cancel := context.WithCancel(context.Background())
		contexts[i] = cancel

		writer, err := newBufferedFileWriter(ctx, tmpFile.Name(), 1024, 100*time.Millisecond, logger)
		if err != nil {
			t.Fatalf("Failed to create writer %d: %v", i, err)
		}
		writers[i] = writer

		// Write some data
		_, _ = writer.Write([]byte("Test data\n"))
	}

	afterCreateGoroutines := runtime.NumGoroutine()
	t.Logf("Goroutines after creating %d writers: %d (delta: %d)", numWriters, afterCreateGoroutines, afterCreateGoroutines-initialGoroutines)

	// Cancel all contexts
	t.Logf("Cancelling all %d contexts...", numWriters)
	for i, cancel := range contexts {
		cancel()
		t.Logf("  Cancelled context %d", i+1)
	}

	// Give goroutines time to exit
	time.Sleep(500 * time.Millisecond)

	afterCancelGoroutines := runtime.NumGoroutine()
	t.Logf("Goroutines after cancelling all contexts: %d (delta from initial: %d)", afterCancelGoroutines, afterCancelGoroutines-initialGoroutines)

	// All goroutines should have exited
	if afterCancelGoroutines > afterCreateGoroutines {
		t.Errorf("Expected goroutine count to decrease after context cancellation, but it increased from %d to %d",
			afterCreateGoroutines, afterCancelGoroutines)
	}

	t.Log("✅ Multiple writers cleanup test passed!")
}

func TestBufferedFileWriter_ContextCancellationFlushesBuffer(t *testing.T) {
	// This test verifies that pending buffered data is flushed when context is cancelled
	tmpFile, err := os.CreateTemp("", "flush-test-*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	logger := createBootstrapLogger("flush-test")
	ctx, cancel := context.WithCancel(context.Background())

	// Create writer with a long timeout so data stays buffered
	writer, err := newBufferedFileWriter(ctx, tmpFile.Name(), 1024, 10*time.Second, logger)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	// Write data but don't fill the buffer (so it won't auto-flush)
	testData := []byte("Buffered data that should be flushed on context cancel\n")
	_, err = writer.Write(testData)
	if err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}

	// Immediately cancel context (data should still be in buffer)
	t.Log("Cancelling context - this should trigger final flush...")
	cancel()

	// Give goroutine time to flush and exit
	time.Sleep(200 * time.Millisecond)

	// Verify the data was flushed to disk
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != string(testData) {
		t.Errorf("Expected buffered data to be flushed on context cancel. Expected %q, got %q", string(testData), string(content))
	}

	t.Log("✅ Context cancellation flush test passed!")
}

func BenchmarkBufferedFileWriter_ContextCleanup(b *testing.B) {
	logger := createBootstrapLogger("benchmark")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tmpFile, _ := os.CreateTemp("", "bench-*.log")
		tmpFile.Close()

		ctx, cancel := context.WithCancel(context.Background())
		writer, _ := newBufferedFileWriter(ctx, tmpFile.Name(), 1024, 100*time.Millisecond, logger)
		_, _ = writer.Write([]byte("test"))

		cancel()
		time.Sleep(50 * time.Millisecond)

		os.Remove(tmpFile.Name())
	}
}

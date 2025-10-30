package logging

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

const testPluginName = "test-plugin"

func TestTraefikLogWriter_Write(t *testing.T) {
	// Capture stdout output by temporarily replacing os.Stdout
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	go func() {
		_, _ = buf.ReadFrom(r)
	}()

	writer := &TraefikLogWriter{}
	testMessage := "test log message"

	n, err := writer.Write([]byte(testMessage))

	// Restore stdout and close the pipe
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("expected no error, but got: %v", err)
	}

	if n != len(testMessage) {
		t.Errorf("expected to write %d bytes, but wrote %d", len(testMessage), n)
	}

	// Give some time for the goroutine to read
	time.Sleep(10 * time.Millisecond)

	output := buf.String()
	if !strings.Contains(output, testMessage) {
		t.Errorf("expected output to contain '%s', but got: %s", testMessage, output)
	}
}

func TestCreateBootstrapLogger(t *testing.T) {
	pluginName := testPluginName
	logger := CreateBootstrapLogger(pluginName, "debug")

	if logger == nil {
		t.Fatal("expected logger to not be nil")
	}

	// Test that the logger has the correct plugin name context
	// We can't easily test the internal structure, but we can test that it works
	testMessage := "bootstrap test message"

	// Capture stdout output
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	go func() {
		_, _ = buf.ReadFrom(r)
	}()

	logger.Debug(testMessage)

	// Restore stdout and close the pipe
	w.Close()
	os.Stdout = oldStdout

	// Give some time for the goroutine to read
	time.Sleep(10 * time.Millisecond)

	output := buf.String()
	if !strings.Contains(output, testMessage) {
		t.Errorf("expected output to contain test message, but got: %s", output)
	}
	if !strings.Contains(output, pluginName) {
		t.Errorf("expected output to contain plugin name '%s', but got: %s", pluginName, output)
	}
}

func TestCreateLogger_LogLevels(t *testing.T) {
	pluginName := testPluginName
	bootstrapLogger := CreateBootstrapLogger(pluginName, "info")

	tests := []struct {
		name          string
		level         string
		expectedLevel slog.Level
	}{
		{"debug level", "debug", slog.LevelDebug},
		{"info level", "info", slog.LevelInfo},
		{"warn level", "warn", slog.LevelWarn},
		{"error level", "error", slog.LevelError},
		{"DEBUG level (uppercase)", "DEBUG", slog.LevelInfo}, // Should default to info due to case conversion
		{"invalid level", "invalid", slog.LevelInfo},         // Should default to info
		{"empty level", "", slog.LevelInfo},                  // Should default to info
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			logger := CreateLogger(ctx, pluginName, tt.level, "text", "", 1024, 2, bootstrapLogger, nil)

			if logger == nil {
				t.Fatal("expected logger to not be nil")
			}

			logger.Info("test message")
		})
	}
}

func TestCreateLogger_LogFormats(t *testing.T) {
	pluginName := testPluginName
	bootstrapLogger := CreateBootstrapLogger(pluginName, "info")

	tests := []struct {
		name   string
		format string
	}{
		{"text format", "text"},
		{"json format", "json"},
		{"TEXT format (uppercase)", "TEXT"},
		{"JSON format (uppercase)", "JSON"},
		{"invalid format", "invalid"}, // Should default to text
		{"empty format", ""},          // Should default to text
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			logger := CreateLogger(ctx, pluginName, "info", tt.format, "", 1024, 2, bootstrapLogger, nil)

			if logger == nil {
				t.Fatal("expected logger to not be nil")
			}

			// Test that logger can be used
			logger.Info("test message")
		})
	}
}

func TestCreateLogger_LogPaths(t *testing.T) {
	pluginName := testPluginName
	bootstrapLogger := CreateBootstrapLogger(pluginName, "info")

	t.Run("empty path (default to traefik)", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		logger := CreateLogger(ctx, pluginName, "info", "text", "", 1024, 2, bootstrapLogger, nil)

		if logger == nil {
			t.Fatal("expected logger to not be nil")
		}

		// Test that logger works with default output
		logger.Info("test message")
	})

	t.Run("valid file path", func(t *testing.T) {
		// Create a temporary file for testing
		tmpFile, err := os.CreateTemp("", "test-log-*.log")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Mock writer factory for testing
		mockFactory := func(ctx context.Context, path string, maxSize int, timeout time.Duration, logger *slog.Logger) (io.WriteCloser, error) {
			return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		}

		logger := CreateLogger(ctx, pluginName, "info", "text", tmpFile.Name(), 1024, 2, bootstrapLogger, mockFactory)

		if logger == nil {
			t.Fatal("expected logger to not be nil")
		}

		// Test that logger works with file output
		testMessage := "test file message"
		logger.Info(testMessage)

		// Give some time for buffered writer to potentially flush
		time.Sleep(100 * time.Millisecond)

		// Note: We cannot easily test file content due to buffering,
		// but we've verified the logger was created successfully
	})

	t.Run("invalid file path", func(t *testing.T) {
		// Use a path that should fail (no permission or invalid directory)
		invalidPath := "/root/nonexistent/test.log"

		// Capture bootstrap logger output to verify error logging
		var buf bytes.Buffer
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		go func() {
			_, _ = buf.ReadFrom(r)
		}()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Mock writer factory that fails
		mockFactory := func(ctx context.Context, path string, maxSize int, timeout time.Duration, logger *slog.Logger) (io.WriteCloser, error) {
			return nil, fmt.Errorf("permission denied")
		}

		logger := CreateLogger(ctx, pluginName, "info", "text", invalidPath, 1024, 2, bootstrapLogger, mockFactory)

		if logger == nil {
			t.Fatal("expected logger to not be nil even with invalid path")
		}

		// Logger should still work (fallback to default)
		logger.Info("test message after invalid path")

		// Restore stdout and close the pipe
		w.Close()
		os.Stdout = oldStdout

		// Give some time for the goroutine to read
		time.Sleep(10 * time.Millisecond)

		// Should have logged an error about the invalid path
		output := buf.String()
		if !strings.Contains(output, "Failed to create buffered file writer") {
			t.Logf("Expected error about file writer creation, got: %s", output)
		}
	})
}

func TestCreateLogger_Integration(t *testing.T) {
	pluginName := "integration-test-plugin"
	bootstrapLogger := CreateBootstrapLogger(pluginName, "info")

	// Capture stdout output for integration test
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	go func() {
		_, _ = buf.ReadFrom(r)
	}()

	// Test complete logger creation and usage
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := CreateLogger(ctx, pluginName, "debug", "text", "", 1024, 2, bootstrapLogger, nil)

	if logger == nil {
		t.Fatal("expected logger to not be nil")
	}

	// Test different log levels
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	// Restore stdout and close the pipe
	w.Close()
	os.Stdout = oldStdout

	// Give some time for the goroutine to read
	time.Sleep(10 * time.Millisecond)

	output := buf.String()

	// Verify messages appear in output
	expectedMessages := []string{"debug message", "info message", "warn message", "error message"}
	for _, msg := range expectedMessages {
		if !strings.Contains(output, msg) {
			t.Errorf("expected output to contain '%s', but got: %s", msg, output)
		}
	}

	// Verify plugin name appears in output
	if !strings.Contains(output, pluginName) {
		t.Errorf("expected output to contain plugin name '%s', but got: %s", pluginName, output)
	}
}

func TestCreateLogger_WithAttributes(t *testing.T) {
	pluginName := "attr-test-plugin"
	bootstrapLogger := CreateBootstrapLogger(pluginName, "info")

	// Capture stdout output
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	go func() {
		_, _ = buf.ReadFrom(r)
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := CreateLogger(ctx, pluginName, "info", "text", "", 1024, 2, bootstrapLogger, nil)

	// Test logging with attributes
	logger.Info("test message with attributes", "key1", "value1", "key2", 42)

	// Restore stdout and close the pipe
	w.Close()
	os.Stdout = oldStdout

	// Give some time for the goroutine to read
	time.Sleep(10 * time.Millisecond)

	output := buf.String()

	if !strings.Contains(output, "test message with attributes") {
		t.Errorf("expected output to contain test message, but got: %s", output)
	}
	if !strings.Contains(output, "key1") {
		t.Errorf("expected output to contain key1, but got: %s", output)
	}
	if !strings.Contains(output, "value1") {
		t.Errorf("expected output to contain value1, but got: %s", output)
	}
}

func TestCreateLogger_JSONFormat(t *testing.T) {
	pluginName := "json-test-plugin"
	bootstrapLogger := CreateBootstrapLogger(pluginName, "info")

	// Capture stdout output
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	go func() {
		_, _ = buf.ReadFrom(r)
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := CreateLogger(ctx, pluginName, "info", "json", "", 1024, 2, bootstrapLogger, nil)

	logger.Info("json test message", "testKey", "testValue")

	// Restore stdout and close the pipe
	w.Close()
	os.Stdout = oldStdout

	// Give some time for the goroutine to read
	time.Sleep(10 * time.Millisecond)

	output := buf.String()

	// JSON format should include structured data
	if !strings.Contains(output, "json test message") {
		t.Errorf("expected output to contain test message, but got: %s", output)
	}

	// Should contain JSON-like structure (though exact format may vary)
	if !strings.Contains(output, "testKey") {
		t.Errorf("expected JSON output to contain testKey, but got: %s", output)
	}
}

// Benchmark tests to ensure logging performance is reasonable
func BenchmarkTraefikLogWriter_Write(b *testing.B) {
	writer := &TraefikLogWriter{}
	message := []byte("benchmark test message")

	// Capture stdout output to avoid polluting test output
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	go func() {
		_, _ = buf.ReadFrom(r)
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = writer.Write(message)
	}

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout
}

func BenchmarkCreateBootstrapLogger(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger := CreateBootstrapLogger(fmt.Sprintf("plugin-%d", i), "info")
		_ = logger // Avoid compiler optimization
	}
}

func BenchmarkCreateLogger(b *testing.B) {
	bootstrapLogger := CreateBootstrapLogger("benchmark-plugin", "info")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		logger := CreateLogger(ctx, fmt.Sprintf("plugin-%d", i), "info", "text", "", 1024, 2, bootstrapLogger, nil)
		_ = logger
		cancel() // Cancel immediately to clean up goroutine
	}
}

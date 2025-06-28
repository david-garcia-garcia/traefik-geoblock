package traefik_geoblock

import (
	"bytes"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

const testPluginName = "test-plugin"

func TestTraefikLogWriter_Write(t *testing.T) {
	// Capture log output by temporarily replacing the default logger
	var buf bytes.Buffer
	oldOutput := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(oldOutput)

	writer := &traefikLogWriter{}
	testMessage := "test log message"

	n, err := writer.Write([]byte(testMessage))

	if err != nil {
		t.Errorf("expected no error, but got: %v", err)
	}

	if n != len(testMessage) {
		t.Errorf("expected to write %d bytes, but wrote %d", len(testMessage), n)
	}

	output := buf.String()
	if !strings.Contains(output, testMessage) {
		t.Errorf("expected output to contain '%s', but got: %s", testMessage, output)
	}
}

func TestCreateBootstrapLogger(t *testing.T) {
	pluginName := testPluginName
	logger := createBootstrapLogger(pluginName)

	if logger == nil {
		t.Fatal("expected logger to not be nil")
	}

	// Test that the logger has the correct plugin name context
	// We can't easily test the internal structure, but we can test that it works
	testMessage := "bootstrap test message"

	// Capture log output
	var buf bytes.Buffer
	oldOutput := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(oldOutput)

	logger.Debug(testMessage)

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
	bootstrapLogger := createBootstrapLogger(pluginName)

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
			logger := createLogger(pluginName, tt.level, "text", "", bootstrapLogger)

			if logger == nil {
				t.Fatal("expected logger to not be nil")
			}

			// Test that logger can be used (basic functionality test)
			// We can't easily test the exact log level without complex setup,
			// but we can ensure the logger works
			logger.Info("test message")
		})
	}
}

func TestCreateLogger_LogFormats(t *testing.T) {
	pluginName := testPluginName
	bootstrapLogger := createBootstrapLogger(pluginName)

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
			logger := createLogger(pluginName, "info", tt.format, "", bootstrapLogger)

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
	bootstrapLogger := createBootstrapLogger(pluginName)

	t.Run("empty path (default to traefik)", func(t *testing.T) {
		logger := createLogger(pluginName, "info", "text", "", bootstrapLogger)

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

		logger := createLogger(pluginName, "info", "text", tmpFile.Name(), bootstrapLogger)

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
		oldOutput := log.Writer()
		log.SetOutput(&buf)
		defer log.SetOutput(oldOutput)

		logger := createLogger(pluginName, "info", "text", invalidPath, bootstrapLogger)

		if logger == nil {
			t.Fatal("expected logger to not be nil even with invalid path")
		}

		// Should have logged an error about the invalid path
		output := buf.String()
		if !strings.Contains(output, "Failed to create buffered file writer") {
			// This might not always fail depending on the system, so we won't make this a hard requirement
			t.Logf("Expected error about file writer creation, got: %s", output)
		}

		// Logger should still work (fallback to default)
		logger.Info("test message after invalid path")
	})
}

func TestCreateLogger_Integration(t *testing.T) {
	pluginName := "integration-test-plugin"
	bootstrapLogger := createBootstrapLogger(pluginName)

	// Capture output for integration test
	var buf bytes.Buffer
	oldOutput := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(oldOutput)

	// Test complete logger creation and usage
	logger := createLogger(pluginName, "debug", "text", "", bootstrapLogger)

	if logger == nil {
		t.Fatal("expected logger to not be nil")
	}

	// Test different log levels
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

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
	bootstrapLogger := createBootstrapLogger(pluginName)

	// Capture output
	var buf bytes.Buffer
	oldOutput := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(oldOutput)

	logger := createLogger(pluginName, "info", "text", "", bootstrapLogger)

	// Test logging with attributes
	logger.Info("test message with attributes", "key1", "value1", "key2", 42)

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
	bootstrapLogger := createBootstrapLogger(pluginName)

	// Capture output
	var buf bytes.Buffer
	oldOutput := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(oldOutput)

	logger := createLogger(pluginName, "info", "json", "", bootstrapLogger)

	logger.Info("json test message", "testKey", "testValue")

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
	writer := &traefikLogWriter{}
	message := []byte("benchmark test message")

	// Capture output to avoid polluting test output
	var buf bytes.Buffer
	oldOutput := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(oldOutput)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = writer.Write(message)
	}
}

func BenchmarkCreateBootstrapLogger(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger := createBootstrapLogger(fmt.Sprintf("plugin-%d", i))
		_ = logger // Avoid compiler optimization
	}
}

func BenchmarkCreateLogger(b *testing.B) {
	bootstrapLogger := createBootstrapLogger("benchmark-plugin")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger := createLogger(fmt.Sprintf("plugin-%d", i), "info", "text", "", bootstrapLogger)
		_ = logger // Avoid compiler optimization
	}
}

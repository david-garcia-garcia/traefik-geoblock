package logging

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

const testPluginName = "test-plugin"

func TestRedact(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"a", "a"},
		{"ab", "ab"},
		{"abc", "ab*"},
		{"mysupersecretkey", "my**************"},
	}
	for _, tc := range cases {
		if got := Redact(tc.in); got != tc.want {
			t.Errorf("Redact(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestStdoutWriter_Write(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	go func() {
		_, _ = buf.ReadFrom(r)
	}()

	writer := &StdoutWriter{}
	testMessage := "test log message"

	n, err := writer.Write([]byte(testMessage))

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("expected no error, but got: %v", err)
	}

	if n != len(testMessage) {
		t.Errorf("expected to write %d bytes, but wrote %d", len(testMessage), n)
	}

	time.Sleep(10 * time.Millisecond)

	output := buf.String()
	if !strings.Contains(output, testMessage) {
		t.Errorf("expected output to contain '%s', but got: %s", testMessage, output)
	}
}

func TestNewBootstrap(t *testing.T) {
	pluginName := testPluginName
	logger := NewBootstrap(pluginName, "debug")

	if logger == nil {
		t.Fatal("expected logger to not be nil")
	}

	testMessage := "bootstrap test message"

	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	go func() {
		_, _ = buf.ReadFrom(r)
	}()

	logger.Debug(testMessage)

	w.Close()
	os.Stdout = oldStdout

	time.Sleep(10 * time.Millisecond)

	output := buf.String()
	if !strings.Contains(output, testMessage) {
		t.Errorf("expected output to contain test message, but got: %s", output)
	}
	if !strings.Contains(output, pluginName) {
		t.Errorf("expected output to contain plugin name '%s', but got: %s", pluginName, output)
	}
}

func TestNewOwner(t *testing.T) {
	logger := NewOwner("traefik-geoblock@kubernetescrd", "info")
	if logger == nil {
		t.Fatal("expected logger")
	}
}

func TestNew_LogLevels(t *testing.T) {
	pluginName := testPluginName
	bootstrapLogger := NewBootstrap(pluginName, "info")

	tests := []struct {
		name          string
		level         string
		expectedLevel slog.Level
	}{
		{"trace level", "trace", LevelTrace},
		{"debug level", "debug", slog.LevelDebug},
		{"info level", "info", slog.LevelInfo},
		{"warn level", "warn", slog.LevelWarn},
		{"error level", "error", slog.LevelError},
		{"DEBUG level (uppercase)", "DEBUG", slog.LevelInfo},
		{"invalid level", "invalid", slog.LevelInfo},
		{"empty level", "", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := New(pluginName, tt.level, "text", bootstrapLogger)

			if logger == nil {
				t.Fatal("expected logger to not be nil")
			}

			logger.Info("test message")
		})
	}
}

func TestNew_LogFormats(t *testing.T) {
	pluginName := testPluginName
	bootstrapLogger := NewBootstrap(pluginName, "info")

	tests := []struct {
		name   string
		format string
	}{
		{"text format", "text"},
		{"json format", "json"},
		{"TEXT format (uppercase)", "TEXT"},
		{"JSON format (uppercase)", "JSON"},
		{"invalid format", "invalid"},
		{"empty format", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := New(pluginName, "info", tt.format, bootstrapLogger)

			if logger == nil {
				t.Fatal("expected logger to not be nil")
			}

			logger.Info("test message")
		})
	}
}

func TestNew_Integration(t *testing.T) {
	pluginName := "integration-test-plugin"
	bootstrapLogger := NewBootstrap(pluginName, "info")

	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	go func() {
		_, _ = buf.ReadFrom(r)
	}()

	logger := New(pluginName, "debug", "text", bootstrapLogger)

	if logger == nil {
		t.Fatal("expected logger to not be nil")
	}

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	w.Close()
	os.Stdout = oldStdout

	time.Sleep(10 * time.Millisecond)

	output := buf.String()

	expectedMessages := []string{"debug message", "info message", "warn message", "error message"}
	for _, msg := range expectedMessages {
		if !strings.Contains(output, msg) {
			t.Errorf("expected output to contain '%s', but got: %s", msg, output)
		}
	}

	if !strings.Contains(output, pluginName) {
		t.Errorf("expected output to contain plugin name '%s', but got: %s", pluginName, output)
	}
}

func TestNew_TraceLevel(t *testing.T) {
	pluginName := "trace-test-plugin"
	bootstrapLogger := NewBootstrap(pluginName, "info")

	t.Run("debug does not emit trace", func(t *testing.T) {
		var buf bytes.Buffer
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		go func() { _, _ = buf.ReadFrom(r) }()

		logger := New(pluginName, "debug", "text", bootstrapLogger)
		Trace(logger, "trace only message")
		logger.Debug("debug message")

		w.Close()
		os.Stdout = oldStdout
		time.Sleep(10 * time.Millisecond)
		output := buf.String()
		if strings.Contains(output, "trace only message") {
			t.Errorf("debug level should not emit trace, got: %s", output)
		}
		if !strings.Contains(output, "debug message") {
			t.Errorf("expected debug message, got: %s", output)
		}
	})

	t.Run("trace emits TRACE label", func(t *testing.T) {
		var buf bytes.Buffer
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		go func() { _, _ = buf.ReadFrom(r) }()

		logger := New(pluginName, "trace", "text", bootstrapLogger)
		Trace(logger, "trace only message")

		w.Close()
		os.Stdout = oldStdout
		time.Sleep(10 * time.Millisecond)
		output := buf.String()
		if !strings.Contains(output, "trace only message") {
			t.Errorf("expected trace message, got: %s", output)
		}
		if !strings.Contains(output, "TRACE") {
			t.Errorf("expected TRACE level label, got: %s", output)
		}
	})
}

func TestNew_WithAttributes(t *testing.T) {
	pluginName := "attr-test-plugin"
	bootstrapLogger := NewBootstrap(pluginName, "info")

	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	go func() {
		_, _ = buf.ReadFrom(r)
	}()

	logger := New(pluginName, "info", "text", bootstrapLogger)

	logger.Info("test message with attributes", "key1", "value1", "key2", 42)

	w.Close()
	os.Stdout = oldStdout

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

func TestNew_JSONFormat(t *testing.T) {
	pluginName := "json-test-plugin"
	bootstrapLogger := NewBootstrap(pluginName, "info")

	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	go func() {
		_, _ = buf.ReadFrom(r)
	}()

	logger := New(pluginName, "info", "json", bootstrapLogger)

	logger.Info("json test message", "testKey", "testValue")

	w.Close()
	os.Stdout = oldStdout

	time.Sleep(10 * time.Millisecond)

	output := buf.String()

	if !strings.Contains(output, "json test message") {
		t.Errorf("expected output to contain test message, but got: %s", output)
	}

	if !strings.Contains(output, "testKey") {
		t.Errorf("expected JSON output to contain testKey, but got: %s", output)
	}
}

func BenchmarkStdoutWriter_Write(b *testing.B) {
	writer := &StdoutWriter{}
	message := []byte("benchmark test message")

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

	w.Close()
	os.Stdout = oldStdout
}

func BenchmarkNewBootstrap(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger := NewBootstrap(fmt.Sprintf("plugin-%d", i), "info")
		_ = logger
	}
}

func BenchmarkNew(b *testing.B) {
	bootstrapLogger := NewBootstrap("benchmark-plugin", "info")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger := New(fmt.Sprintf("plugin-%d", i), "info", "text", bootstrapLogger)
		_ = logger
	}
}

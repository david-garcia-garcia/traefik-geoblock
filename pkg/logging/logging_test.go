package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

// filedGeoBlockPrefix is the PascalMinder/geoblock#67 sample that CrowdSec could not UnmarshalJSON.
const filedGeoBlockPrefix = "INFO: GeoBlock:"

// captureStdout runs fn while os.Stdout is a pipe and returns the bytes written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()
	fn()
	_ = w.Close()
	os.Stdout = oldStdout
	<-done
	_ = r.Close()
	return buf.String()
}

// stdoutLines returns non-empty lines from captured stdout.
func stdoutLines(output string) []string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// assertNoFiledPrefix fails if any line contains the #67 GeoBlock prefix.
func assertNoFiledPrefix(t *testing.T, output string) {
	t.Helper()
	if strings.Contains(output, filedGeoBlockPrefix) {
		t.Errorf("stdout must not contain %q, got: %s", filedGeoBlockPrefix, output)
	}
}

// assertTextNotJSONObject fails unless every line is non-JSON and does not start with '{'.
func assertTextNotJSONObject(t *testing.T, output string) {
	t.Helper()
	lines := stdoutLines(output)
	if len(lines) == 0 {
		t.Fatal("expected at least one stdout line")
	}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "{") {
			t.Errorf("text line %d starts with '{': %s", i, line)
		}
		var object map[string]any
		if err := json.Unmarshal([]byte(trimmed), &object); err == nil {
			t.Errorf("text line %d unmarshaled as JSON: %s", i, line)
		}
	}
}

// assertJSONObjectLines fails unless every line unmarshals as a JSON object starting with '{'.
func assertJSONObjectLines(t *testing.T, output string) {
	t.Helper()
	lines := stdoutLines(output)
	if len(lines) == 0 {
		t.Fatal("expected at least one stdout line")
	}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "{") {
			t.Errorf("json line %d does not start with '{': %s", i, line)
		}
		var object map[string]any
		if err := json.Unmarshal([]byte(trimmed), &object); err != nil {
			t.Errorf("json line %d unmarshal: %v (%s)", i, err, line)
		}
	}
}

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

func TestStdoutLineShapes_CrowdSecSafe(t *testing.T) {
	bootstrap := NewBootstrap("GeoBlock", "info")

	t.Run("NewBootstrap text", func(t *testing.T) {
		output := captureStdout(t, func() {
			NewBootstrap("GeoBlock", "info").Info("allow local IPs: true")
		})
		assertNoFiledPrefix(t, output)
		assertTextNotJSONObject(t, output)
	})

	t.Run("NewOwner text", func(t *testing.T) {
		output := captureStdout(t, func() {
			NewOwner("traefik-geoblock@crd", "info").Info("wrapper ready")
		})
		assertNoFiledPrefix(t, output)
		assertTextNotJSONObject(t, output)
	})

	t.Run("New text", func(t *testing.T) {
		output := captureStdout(t, func() {
			New("GeoBlock", "info", "text", bootstrap).Info("allow local IPs: true")
		})
		assertNoFiledPrefix(t, output)
		assertTextNotJSONObject(t, output)
	})

	t.Run("New empty format", func(t *testing.T) {
		output := captureStdout(t, func() {
			New("GeoBlock", "info", "", bootstrap).Info("allow local IPs: true")
		})
		assertNoFiledPrefix(t, output)
		assertTextNotJSONObject(t, output)
	})

	t.Run("New json", func(t *testing.T) {
		output := captureStdout(t, func() {
			New("GeoBlock", "info", "json", bootstrap).Info("allow local IPs: true")
		})
		assertNoFiledPrefix(t, output)
		assertJSONObjectLines(t, output)
	})
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

package traefik_geoblock

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

// traefikLogWriter implements io.Writer and writes directly to stdout
type traefikLogWriter struct{}

func (w *traefikLogWriter) Write(p []byte) (n int, err error) {
	// Write directly to stdout - let the consumer decide routing
	return os.Stdout.Write(p)
}

// createBootstrapLogger creates a logger for initial plugin setup and configuration
func createBootstrapLogger(name string) *slog.Logger {
	var logLevel slog.Level = slog.LevelDebug

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	// Create a writer that writes directly to stdout
	writer := &traefikLogWriter{}
	handler := slog.NewTextHandler(writer, opts)
	return slog.New(handler).With("plugin", name)
}

// createLogger creates a configured logger based on the provided settings
func createLogger(name, level, format, path string, bufferSizeBytes, timeoutSeconds int, bootstrapLogger *slog.Logger) *slog.Logger {
	var logLevel slog.Level
	level = strings.ToLower(level) // Convert level to lowercase
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
		if level != "" {
			bootstrapLogger.Warn("Unknown log level", "level", level)
		}
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	// Create a writer that writes directly to stdout
	var writer io.Writer = &traefikLogWriter{}
	var destination string = "stdout"

	// Only attempt file writing if explicitly specified
	if path != "" {
		timeout := time.Duration(timeoutSeconds) * time.Second // Convert seconds to duration
		bw, err := newBufferedFileWriter(path, bufferSizeBytes, timeout)
		if err != nil {
			bootstrapLogger.Error("Failed to create buffered file writer for path '%s': %v\n", path, err)
		} else {
			writer = bw
			destination = path
		}
	}

	var handler slog.Handler
	format = strings.ToLower(format)
	if format == "json" {
		handler = slog.NewJSONHandler(writer, opts)
	} else {
		handler = slog.NewTextHandler(writer, opts)
		format = "text" // normalize format name
	}

	// This log here so that in the traefik logs we see where are the logs actually going to for the middleware
	if logLevel <= slog.LevelDebug {
		bootstrapLogger.Debug(fmt.Sprintf("Logging to %s with %s format at %s level", destination, format, logLevel))
	}
	return slog.New(handler).With("plugin", name)
}

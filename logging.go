package traefik_geoblock

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"strings"
	"time"
)

// traefikLogWriter implements io.Writer and uses fmt.Printf for output
type traefikLogWriter struct{}

func (w *traefikLogWriter) Write(p []byte) (n int, err error) {
	// https://github.com/traefik/traefik/issues/8204
	// Since v2.5.5, fmt.Println()/fmt.Printf() are catched and transfered to the Traefik logs, it's not perfect but we will improve that.
	log.Println(string(p))
	return len(p), nil
}

// createBootstrapLogger creates a logger for initial plugin setup and configuration
func createBootstrapLogger(name string) *slog.Logger {
	var logLevel slog.Level = slog.LevelDebug

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	// Create a custom writer that uses fmt.Printf
	fmtWriter := &traefikLogWriter{}
	var writer io.Writer = fmtWriter
	handler := slog.NewTextHandler(writer, opts)
	return slog.New(handler).With("plugin", name)
}

// createLogger creates a configured logger based on the provided settings
func createLogger(name, level, format, path string, bootstrapLogger *slog.Logger) *slog.Logger {
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

	// Create a custom writer that uses fmt.Printf
	fmtWriter := &traefikLogWriter{}
	var writer io.Writer = fmtWriter
	var destination string = "traefik"

	// Only attempt file writing if explicitly specified
	if path != "" {
		bw, err := newBufferedFileWriter(path, 2048, 2*time.Second)
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

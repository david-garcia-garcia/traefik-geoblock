package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

// TraefikLogWriter implements io.Writer and writes directly to stdout
type TraefikLogWriter struct{}

func (w *TraefikLogWriter) Write(p []byte) (n int, err error) {
	// Write directly to stdout - let the consumer decide routing
	return os.Stdout.Write(p)
}

// CreateBootstrapLogger creates a logger for initial plugin setup and configuration
func CreateBootstrapLogger(name string, level string) *slog.Logger {
	var logLevel slog.Level
	level = strings.ToLower(level)
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
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	writer := &TraefikLogWriter{}
	handler := slog.NewTextHandler(writer, opts)
	return slog.New(handler).With("plugin", name)
}

// CreateLogger creates a configured logger based on the provided settings
func CreateLogger(ctx context.Context, name, level, format, path string, bufferSizeBytes, timeoutSeconds int, bootstrapLogger *slog.Logger, writerFactory func(ctx context.Context, path string, maxSize int, timeout time.Duration, logger *slog.Logger) (io.WriteCloser, error)) *slog.Logger {
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
	var writer io.Writer = &TraefikLogWriter{}
	var destination string = "stdout"

	// Only attempt file writing if explicitly specified
	if path != "" && writerFactory != nil {
		timeout := time.Duration(timeoutSeconds) * time.Second // Convert seconds to duration
		bw, err := writerFactory(ctx, path, bufferSizeBytes, timeout, bootstrapLogger)
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

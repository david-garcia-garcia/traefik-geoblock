package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// StdoutWriter writes log bytes to process stdout.
type StdoutWriter struct{}

func (w *StdoutWriter) Write(p []byte) (n int, err error) {
	return os.Stdout.Write(p)
}

// NewBootstrap creates a text slog logger for plugin setup.
func NewBootstrap(name string, level string) *slog.Logger {
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

	handler := slog.NewTextHandler(&StdoutWriter{}, opts)
	return slog.New(handler).With("plugin", name)
}

// New creates a stdout slog logger from level and format.
func New(name, level, format string, bootstrap *slog.Logger) *slog.Logger {
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
		if level != "" {
			bootstrap.Warn("Unknown log level", "level", level)
		}
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	var writer io.Writer = &StdoutWriter{}

	var handler slog.Handler
	format = strings.ToLower(format)
	if format == "json" {
		handler = slog.NewJSONHandler(writer, opts)
	} else {
		handler = slog.NewTextHandler(writer, opts)
		format = "text"
	}

	if logLevel <= slog.LevelDebug {
		bootstrap.Debug(fmt.Sprintf("Logging to stdout with %s format at %s level", format, logLevel))
	}
	return slog.New(handler).With("plugin", name)
}

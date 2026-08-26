package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// LevelTrace is below debug. Per-request ServeHTTP logs use this level.
const LevelTrace slog.Level = slog.LevelDebug - 4

// StdoutWriter writes log bytes to process stdout.
type StdoutWriter struct{}

func (w *StdoutWriter) Write(p []byte) (n int, err error) {
	return os.Stdout.Write(p)
}

// Trace logs at LevelTrace. No-op unless logLevel is trace.
func Trace(logger *slog.Logger, msg string, args ...any) {
	logger.Log(context.Background(), LevelTrace, msg, args...)
}

func parseLevel(level string) (slog.Level, bool) {
	switch strings.ToLower(level) {
	case "trace":
		return LevelTrace, true
	case "debug":
		return slog.LevelDebug, true
	case "info":
		return slog.LevelInfo, true
	case "warn":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	default:
		return slog.LevelInfo, false
	}
}

func replaceLevelName(_ []string, a slog.Attr) slog.Attr {
	if a.Key != slog.LevelKey {
		return a
	}
	if lvl, ok := a.Value.Any().(slog.Level); ok && lvl == LevelTrace {
		a.Value = slog.StringValue("TRACE")
	}
	return a
}

func handlerOptions(level slog.Level) *slog.HandlerOptions {
	return &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: replaceLevelName,
	}
}

// NewBootstrap creates a text slog logger for plugin setup.
func NewBootstrap(name string, level string) *slog.Logger {
	logLevel, _ := parseLevel(level)
	handler := slog.NewTextHandler(&StdoutWriter{}, handlerOptions(logLevel))
	return slog.New(handler).With("plugin", name)
}

// New creates a stdout slog logger from level and format.
func New(name, level, format string, bootstrap *slog.Logger) *slog.Logger {
	logLevel, ok := parseLevel(level)
	if !ok && level != "" {
		bootstrap.Warn("Unknown log level", "level", level)
	}

	var writer io.Writer = &StdoutWriter{}

	var handler slog.Handler
	format = strings.ToLower(format)
	if format == "json" {
		handler = slog.NewJSONHandler(writer, handlerOptions(logLevel))
	} else {
		handler = slog.NewTextHandler(writer, handlerOptions(logLevel))
		format = "text"
	}

	if logLevel <= slog.LevelDebug {
		bootstrap.Debug(fmt.Sprintf("Logging to stdout with %s format at %s level", format, logLevel))
	}
	return slog.New(handler).With("plugin", name)
}

package reclaim

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"
)

// processLogger writes put/dispose to stdout so Traefik docker logs show incarnations.
func processLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

var (
	defaultMu    sync.Mutex
	defaultTable *Table
)

// Default returns the process-wide table, creating it on first use.
func Default() *Table {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultTable == nil {
		defaultTable = NewTable(DefaultGrace, processLogger())
	}
	return defaultTable
}

// Open is Default().Open: create-once for key on the process table and bind ctx.
// If the value has Close(), the table calls it when the incarnation ends.
func Open(ctx context.Context, key string, create func() (any, error)) (any, error) {
	return Default().Open(ctx, key, create)
}

// Reset tears down the process table (cancels every lifetime) and installs a fresh one. Tests only.
func Reset() {
	ResetWith(DefaultGrace, slog.Default())
}

// ResetWith replaces the process table after canceling the current one. Tests only.
func ResetWith(grace time.Duration, logger *slog.Logger) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultTable != nil {
		defaultTable.Reset()
	}
	defaultTable = NewTable(grace, logger)
}

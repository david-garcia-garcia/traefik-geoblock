package reclaim

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

var (
	defaultMu    sync.Mutex
	defaultTable *Table
)

// Default returns the process-wide table, creating it on first use.
func Default() *Table {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultTable == nil {
		defaultTable = NewTable(DefaultGrace, slog.Default())
	}
	return defaultTable
}

// Open is Default().Open: create-once for key on the process table and bind ctx.
func Open(ctx context.Context, key string, create func() (any, error), dispose func(any)) (any, error) {
	return Default().Open(ctx, key, create, dispose)
}

// Reset tears down the process table and installs a fresh one with product grace. Tests only.
func Reset() {
	ResetWith(DefaultGrace, slog.Default())
}

// ResetWith replaces the process table after disposing the current one. Tests only.
func ResetWith(grace time.Duration, logger *slog.Logger) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultTable != nil {
		defaultTable.Reset()
	}
	defaultTable = NewTable(grace, logger)
}

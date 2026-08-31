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
		defaultTable = NewTable(DefaultGrace)
	}
	return defaultTable
}

// Open is Default().Open: create-once for key on the process table and bind ctx.
// logger is required. If the value has Close(), the table calls it when the incarnation ends.
func Open(ctx context.Context, key string, logger *slog.Logger, create func() (any, error)) (any, error) {
	return Default().Open(ctx, key, logger, create)
}

// Reset tears down the process table (cancels every lifetime) and installs a fresh one. Tests only.
func Reset() {
	ResetWith(DefaultGrace)
}

// ResetWith replaces the process table after canceling the current one. Tests only.
func ResetWith(grace time.Duration) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultTable != nil {
		defaultTable.Reset()
	}
	defaultTable = NewTable(grace)
}

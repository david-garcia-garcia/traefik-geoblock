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

// Default is the process-wide table. Callers in any package share it.
func Default() *Table {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultTable == nil {
		defaultTable = NewTable(DefaultGrace, slog.Default())
	}
	return defaultTable
}

// Open binds ctx on Default for key.
func Open(ctx context.Context, key string, create func() (any, error), dispose func(any)) (any, error) {
	return Default().Open(ctx, key, create, dispose)
}

// Reset disposes Default and replaces it. Tests only.
func Reset() {
	ResetWith(DefaultGrace, slog.Default())
}

// ResetWith is Reset with a grace and logger. Tests only.
func ResetWith(grace time.Duration, logger *slog.Logger) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultTable != nil {
		defaultTable.Reset()
	}
	defaultTable = NewTable(grace, logger)
}

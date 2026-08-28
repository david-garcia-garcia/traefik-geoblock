package dbwrappers

import (
	"log/slog"
	"time"
)

// Reset disposes every singleton wrapper. Tests only.
func Reset() {
	ResetWith(DefaultGrace, slog.Default())
}

// ResetWith is Reset with a grace and logger. Tests only.
func ResetWith(grace time.Duration, logger *slog.Logger) {
	tablesMu.Lock()
	defer tablesMu.Unlock()
	if bins != nil {
		bins.Reset()
	}
	if mmdbs != nil {
		mmdbs.Reset()
	}
	bins = NewTable[*BIN](grace, logger)
	mmdbs = NewTable[*MMDB](grace, logger)
}

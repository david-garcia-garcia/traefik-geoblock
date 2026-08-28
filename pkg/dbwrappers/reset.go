package dbwrappers

import "log/slog"

// Reset disposes every singleton wrapper. Tests only.
func Reset() {
	tablesMu.Lock()
	defer tablesMu.Unlock()
	if bins != nil {
		bins.Reset()
	}
	if mmdbs != nil {
		mmdbs.Reset()
	}
	bins = NewTable[*BIN](DefaultGrace, slog.Default())
	mmdbs = NewTable[*MMDB](DefaultGrace, slog.Default())
}

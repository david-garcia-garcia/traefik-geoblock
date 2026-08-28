package dbwrappers

import (
	"log/slog"
	"time"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/reclaim"
)

// Reset disposes every singleton wrapper. Tests only.
func Reset() {
	reclaim.Reset()
}

// ResetWith is Reset with a grace and logger. Tests only.
func ResetWith(grace time.Duration, logger *slog.Logger) {
	reclaim.ResetWith(grace, logger)
}

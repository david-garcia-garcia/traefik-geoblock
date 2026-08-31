package dbwrappers

import (
	"time"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/reclaim"
)

// Reset disposes every singleton wrapper. Tests only.
func Reset() {
	reclaim.Reset()
}

// ResetWith is Reset with a grace. Tests only.
func ResetWith(grace time.Duration) {
	reclaim.ResetWith(grace)
}

package dbwrappers

import (
	"sync"
	"time"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/reclaim"
)

var (
	tableMu sync.Mutex
	table   = reclaim.New(reclaim.Config{Grace: reclaim.DefaultGrace})
)

func currentTable() *reclaim.Table {
	tableMu.Lock()
	defer tableMu.Unlock()
	return table
}

// Reset disposes every singleton wrapper. Tests only.
func Reset() {
	tableMu.Lock()
	defer tableMu.Unlock()
	table.Reset()
}

// ResetWith is Reset then a new table with grace. Tests only.
func ResetWith(grace time.Duration) {
	tableMu.Lock()
	defer tableMu.Unlock()
	table.Reset()
	table = reclaim.New(reclaim.Config{Grace: grace})
}

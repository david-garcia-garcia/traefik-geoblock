package dbdownload

import (
	"log/slog"
	"strings"
	"time"
)

// DefaultMinAge is used when Config.MinAge is unset (IPinfo / MaxMind cadence).
const DefaultMinAge = 24 * time.Hour

// WithDefaults fills Dir, DatabaseType, and MinAge when they are empty.
func WithDefaults(cfg Config, dir, databaseType string, minAge time.Duration) Config {
	cfg.Dir = dir
	if cfg.DatabaseType == "" {
		cfg.DatabaseType = databaseType
	}
	if cfg.MinAge <= 0 {
		if minAge > 0 {
			cfg.MinAge = minAge
		} else {
			cfg.MinAge = DefaultMinAge
		}
	}
	return cfg
}

// Start builds a Slot and runs its ticker when URL is set. A nil Slot means no download.
func Start(cfg Config, logger *slog.Logger, onUpdate func(string)) (*Slot, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, nil
	}
	slot, err := NewSlot(cfg, logger)
	if err != nil {
		return nil, err
	}
	slot.Start(onUpdate)
	return slot, nil
}

// Slot is one running download (ticker + update).
type Slot struct {
	cfg    Config
	logger *slog.Logger
	ticker *time.Ticker
	stop   chan struct{}
}

// NewSlot validates cfg. It does not start a ticker.
func NewSlot(cfg Config, logger *slog.Logger) (*Slot, error) {
	if err := Normalize(&cfg); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Slot{cfg: cfg, logger: logger.With("download", cfg.Key)}, nil
}

// Latest is the newest dated file for this slot.
func (s *Slot) Latest() (string, error) {
	return Latest(s.cfg.Dir, s.cfg.Key, s.cfg.DatabaseType)
}

// CanDownload reports whether a URL is configured.
func (s *Slot) CanDownload() bool {
	return s.cfg.URL != "" && s.cfg.Dir != ""
}

// UpdateIfNeeded runs one age check and optional GET.
func (s *Slot) UpdateIfNeeded() (string, error) {
	return UpdateIfNeeded(s.cfg, s.logger)
}

// Start runs an immediate check and a 24h ticker. onUpdate is called with a new path.
func (s *Slot) Start(onUpdate func(path string)) {
	if !s.CanDownload() {
		return
	}
	s.ticker = time.NewTicker(24 * time.Hour)
	s.stop = make(chan struct{})
	go func() {
		s.tick(onUpdate)
		for {
			select {
			case <-s.ticker.C:
				s.tick(onUpdate)
			case <-s.stop:
				return
			}
		}
	}()
}

func (s *Slot) tick(onUpdate func(path string)) {
	path, err := s.UpdateIfNeeded()
	if err != nil {
		s.logger.Error("database update failed", "error", err)
		return
	}
	if path == "" || onUpdate == nil {
		return
	}
	onUpdate(path)
}

// Stop ends the ticker.
func (s *Slot) Stop() {
	if s == nil {
		return
	}
	if s.ticker != nil {
		s.ticker.Stop()
	}
	if s.stop != nil {
		select {
		case <-s.stop:
		default:
			close(s.stop)
		}
	}
}

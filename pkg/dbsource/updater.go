package dbsource

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

// Start builds an Updater and runs its ticker when URL is set. A nil Updater means no download.
func Start(cfg Config, logger *slog.Logger, onUpdate func(string)) (*Updater, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, nil
	}
	u, err := newUpdater(cfg, logger)
	if err != nil {
		return nil, err
	}
	u.Start(onUpdate)
	return u, nil
}

// Updater is the keep-current loop for one source (ticker + GET).
type Updater struct {
	cfg    Config
	logger *slog.Logger
	ticker *time.Ticker
	stop   chan struct{}
}

func newUpdater(cfg Config, logger *slog.Logger) (*Updater, error) {
	if err := Normalize(&cfg); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Updater{cfg: cfg, logger: logger.With("source", cfg.Key)}, nil
}

// Latest is the newest dated file for this source key.
func (u *Updater) Latest() (string, error) {
	return Latest(u.cfg.Dir, u.cfg.Key, u.cfg.DatabaseType)
}

// CanDownload reports whether a URL is configured.
func (u *Updater) CanDownload() bool {
	return u.cfg.URL != "" && u.cfg.Dir != ""
}

// UpdateIfNeeded runs one age check and optional GET.
func (u *Updater) UpdateIfNeeded() (string, error) {
	return UpdateIfNeeded(u.cfg, u.logger)
}

// Start runs an immediate check and a 24h ticker. onUpdate is called with a new path.
func (u *Updater) Start(onUpdate func(path string)) {
	if !u.CanDownload() {
		return
	}
	u.ticker = time.NewTicker(24 * time.Hour)
	u.stop = make(chan struct{})
	go func() {
		u.tick(onUpdate)
		for {
			select {
			case <-u.ticker.C:
				u.tick(onUpdate)
			case <-u.stop:
				return
			}
		}
	}()
}

func (u *Updater) tick(onUpdate func(path string)) {
	path, err := u.UpdateIfNeeded()
	if err != nil {
		u.logger.Error("database update failed", "error", err)
		return
	}
	if path == "" || onUpdate == nil {
		return
	}
	onUpdate(path)
}

// Stop ends the ticker.
func (u *Updater) Stop() {
	if u == nil {
		return
	}
	if u.ticker != nil {
		u.ticker.Stop()
	}
	if u.stop != nil {
		select {
		case <-u.stop:
		default:
			close(u.stop)
		}
	}
}

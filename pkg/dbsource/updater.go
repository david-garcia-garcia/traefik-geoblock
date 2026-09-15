package dbsource

import (
	"log/slog"
	"time"
)

// DefaultMinAge is used when Config.MinAge is unset.
const DefaultMinAge = 30 * 24 * time.Hour

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

// Start builds an Updater and runs its ticker when Dir and Key can watch Latest.
// A nil Updater means nothing to watch. GET still requires URL.
func Start(cfg Config, logger *slog.Logger, onUpdate func(string)) (*Updater, error) {
	u, err := newUpdater(cfg, logger)
	if err != nil {
		return nil, err
	}
	if !u.canWatchLatest() {
		return nil, nil
	}
	u.Start(onUpdate)
	return u, nil
}

// Updater is the keep-current loop for one source: promote Latest on disk, then GET if stale.
type Updater struct {
	cfg    Config
	logger *slog.Logger
	ticker *time.Ticker
	stop   chan struct{}
	// done is closed when the ticker goroutine exits.
	done chan struct{}
}

func newUpdater(cfg Config, logger *slog.Logger) (*Updater, error) {
	if err := Normalize(&cfg); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Updater{cfg: cfg, logger: logger}, nil
}

// Latest is the newest dated file for this source key.
func (u *Updater) Latest() (string, error) {
	return Latest(u.cfg.Dir, u.cfg.Key, u.cfg.DatabaseType)
}

// canWatchLatest reports whether Dir and Key can resolve a dated catalog file.
func (u *Updater) canWatchLatest() bool {
	return u.cfg.Dir != "" && u.cfg.Key != ""
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
	u.ticker = time.NewTicker(24 * time.Hour)
	u.stop = make(chan struct{})
	u.done = make(chan struct{})
	go func() {
		defer close(u.done)
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
	// Promote a dated file already on disk (this pod or another writer). No GET.
	latest, err := u.Latest()
	if err != nil {
		u.logger.Error("latest dated file", "error", err)
	} else if latest != "" && onUpdate != nil {
		onUpdate(latest)
	}
	if !u.CanDownload() {
		return
	}
	path, err := u.UpdateIfNeeded()
	if err != nil {
		u.logger.Error("database update failed", "error", err)
		return
	}
	if path == "" || onUpdate == nil {
		return
	}
	// Sleep and Close both Stop+join. An in-flight GET still calls onUpdate:
	// Sleep is only parking the ticker, the wrapper is still live. Close sets
	// the wrapper disposed flag before Stop; that flag is what refuses the swap.
	onUpdate(path)
}

// Stop ends the ticker and waits for the ticker goroutine to exit.
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
	if u.done != nil {
		<-u.done
	}
}

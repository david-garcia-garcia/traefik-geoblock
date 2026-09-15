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

// UpdateTrigger says which tick step produced the path handed to onUpdate, so the
// caller's own trail can name the event instead of this package logging it too.
type UpdateTrigger string

const (
	// TriggerPromote is a dated file that was already on disk when the tick ran.
	TriggerPromote UpdateTrigger = "promote"
	// TriggerDownload is the path the tick's age check and GET returned.
	TriggerDownload UpdateTrigger = "download"
)

// Start builds an Updater and starts promote-and-GET when Dir and Key can watch Latest.
// A nil Updater means nothing to watch. GET still requires URL.
func Start(cfg Config, logger *slog.Logger, onUpdate func(string, UpdateTrigger)) (*Updater, error) {
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

// Updater promotes Latest on disk for one source, then GETs if that file is stale.
type Updater struct {
	cfg    Config
	logger *slog.Logger
	ticker *time.Ticker
	stop   chan struct{}
	// exited is closed when Start's goroutine returns so Stop can join.
	exited chan struct{}
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

// Start runs one promote-and-GET pass now and again every 24h. onUpdate is called
// with a new path and the tick step that produced it.
func (u *Updater) Start(onUpdate func(path string, trigger UpdateTrigger)) {
	u.ticker = time.NewTicker(24 * time.Hour)
	u.stop = make(chan struct{})
	u.exited = make(chan struct{})
	go func() {
		defer close(u.exited)
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

func (u *Updater) tick(onUpdate func(path string, trigger UpdateTrigger)) {
	// Promote a dated file already on disk (this pod or another writer). No GET.
	latest, err := u.Latest()
	if err != nil {
		u.logger.Error("latest dated file", "error", err)
	} else if latest != "" && onUpdate != nil {
		onUpdate(latest, TriggerPromote)
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
	onUpdate(path, TriggerDownload)
}

// Stop asks Start's goroutine to return and waits until it has.
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
	if u.exited != nil {
		<-u.exited
	}
}

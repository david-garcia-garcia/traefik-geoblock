package dbsource

import (
	"log/slog"
	"net/url"
	"strings"
	"sync"
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

// Updater is the keep-current loop for one source (ticker + GET). It can be stopped and started
// again: Stop does not return until the loop has finished, so a stopped Updater writes nothing.
type Updater struct {
	cfg    Config
	logger *slog.Logger
	// mu guards ticker and stop, which Start and Stop replace on every run.
	mu     sync.Mutex
	ticker *time.Ticker
	stop   chan struct{}
	// running is the loop goroutine, so Stop can join it.
	running sync.WaitGroup
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

// CanDownload reports whether a URL is configured.
func (u *Updater) CanDownload() bool {
	return u.cfg.URL != "" && u.cfg.Dir != ""
}

// UpdateIfNeeded runs one age check and optional GET.
func (u *Updater) UpdateIfNeeded() (string, error) {
	return UpdateIfNeeded(u.cfg, u.logger)
}

// Start runs an immediate check and a 24h ticker. onUpdate is called with a new path.
// A second Start while the loop is running is ignored; after Stop it starts a fresh loop, which
// is how a wrapper wakes. Safe on a nil Updater, which is what a source with no URL has.
func (u *Updater) Start(onUpdate func(path string)) {
	if u == nil || !u.CanDownload() {
		return
	}
	u.mu.Lock()
	if u.stop != nil {
		u.mu.Unlock()
		return
	}
	ticker := time.NewTicker(24 * time.Hour)
	stop := make(chan struct{})
	u.ticker, u.stop = ticker, stop
	u.running.Add(1)
	u.mu.Unlock()

	go func() {
		defer u.running.Done()
		u.tick(stop, onUpdate)
		for {
			select {
			case <-ticker.C:
				u.tick(stop, onUpdate)
			case <-stop:
				return
			}
		}
	}()
}

// tick runs one age check and optional GET, and skips both once stop is closed. It checks again
// before the callback, so a download that was already in flight cannot swap in a file for an
// updater that has been stopped.
func (u *Updater) tick(stop <-chan struct{}, onUpdate func(path string)) {
	if stopped(stop) {
		return
	}
	path, err := u.UpdateIfNeeded()
	if err != nil {
		u.logger.Error("database update failed", "key", u.cfg.Key, "error", withoutQuery(err, u.cfg.URL))
		return
	}
	if path == "" || onUpdate == nil || stopped(stop) {
		return
	}
	onUpdate(path)
}

// withoutQuery is err with every mention of rawURL cut back to scheme, host, and path. A
// transport failure renders as `Get "<full URL>": ...`, and the download URL carries the
// operator's API token in its query (ip2location, ipinfo), so the raw error cannot be logged.
func withoutQuery(err error, rawURL string) string {
	text := err.Error()
	if rawURL == "" {
		return text
	}
	parsed, parseErr := url.Parse(rawURL)
	if parseErr != nil {
		// Nothing reliable to redact down to, so name neither the URL nor the failure detail.
		return "download failed"
	}
	parsed.RawQuery, parsed.Fragment, parsed.User = "", "", nil
	return strings.ReplaceAll(text, rawURL, parsed.String())
}

// stopped reports whether the loop has been asked to end.
func stopped(stop <-chan struct{}) bool {
	select {
	case <-stop:
		return true
	default:
		return false
	}
}

// Stop ends the ticker and does not return until the loop goroutine has finished, so no download
// or update callback can land after it. Safe on a nil Updater, on one that never started, and
// when called twice.
func (u *Updater) Stop() {
	if u == nil {
		return
	}
	// Held across Wait so a Start cannot register a new loop while this one is being joined.
	u.mu.Lock()
	defer u.mu.Unlock()
	ticker, stop := u.ticker, u.stop
	u.ticker, u.stop = nil, nil
	if ticker != nil {
		ticker.Stop()
	}
	if stop != nil {
		close(stop)
	}
	u.running.Wait()
}

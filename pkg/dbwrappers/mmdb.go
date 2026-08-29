package dbwrappers

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"log/slog"

	"github.com/oschwald/maxminddb-golang"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/reclaim"
)

// MMDBConfig is one MMDB file to resolve, open, and keep current.
type MMDBConfig struct {
	Dir             string
	Source          dbsource.Config
	DefaultFileName string
	MinAge          time.Duration
}

// MMDB is one open MaxMind DB (FromBytes) with hot-swap and a download ticker.
type MMDB struct {
	mu      sync.RWMutex
	db      *maxminddb.Reader
	path    string
	logger  *slog.Logger
	cfg     MMDBConfig
	updater *dbsource.Updater
}

const keyPrefixMMDB = "mmdb:"

func mmdbKey(cfg MMDBConfig) string {
	return keyPrefixMMDB + configHash(cfg)
}

// OpenMMDB returns the singleton MMDB for cfg and binds ctx on the process table.
func OpenMMDB(ctx context.Context, cfg MMDBConfig, logger *slog.Logger) (*MMDB, error) {
	key := mmdbKey(cfg)
	v, err := reclaim.Open(ctx, key, func() (any, error) {
		return newMMDB(cfg, logger)
	}, func(v any) {
		if w, ok := v.(*MMDB); ok {
			w.close()
		}
	})
	if err != nil {
		return nil, err
	}
	w, ok := v.(*MMDB)
	if !ok {
		return nil, fmt.Errorf("reclaim: %s: want *MMDB, got %T", key, v)
	}
	return w, nil
}

func newMMDB(cfg MMDBConfig, logger *slog.Logger) (*MMDB, error) {
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With("key", cfg.Source.Key)
	w := &MMDB{logger: logger, cfg: cfg}
	path, err := dbsource.Resolve(w.sourceCfg(), w.logger)
	if err != nil {
		return nil, err
	}
	if err := w.open(path); err != nil {
		return nil, err
	}
	updater, err := dbsource.Start(w.sourceCfg(), w.logger, func(path string) {
		if path == "" || path == w.Path() {
			return
		}
		if err := w.open(path); err != nil {
			w.logger.Error("failed to open updated MMDB", "error", err)
		}
	})
	if err != nil {
		return nil, err
	}
	w.updater = updater
	return w, nil
}

func (w *MMDB) sourceCfg() dbsource.Config {
	cfg := dbsource.WithDefaults(w.cfg.Source, w.cfg.Dir, dbsource.TypeMMDB, w.cfg.MinAge)
	if cfg.DefaultFileName == "" {
		cfg.DefaultFileName = w.cfg.DefaultFileName
	}
	return cfg
}

func (w *MMDB) open(path string) error {
	buf, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read MMDB %s: %w", path, err)
	}
	db, err := maxminddb.FromBytes(buf)
	if err != nil {
		return fmt.Errorf("failed to open MMDB %s: %w", path, err)
	}
	w.mu.Lock()
	old := w.db
	w.db = db
	w.path = path
	w.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	w.logger.Info("MMDB opened", "path", path)
	return nil
}

// Path is the file last opened.
func (w *MMDB) Path() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.path
}

// Lookup decodes ip into dest (vendor schema tags).
func (w *MMDB) Lookup(ip string, dest any) error {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}
	w.mu.RLock()
	db := w.db
	w.mu.RUnlock()
	if db == nil {
		return fmt.Errorf("MMDB is not open")
	}
	return db.Lookup(parsed, dest)
}

func (w *MMDB) close() {
	if w.updater != nil {
		w.updater.Stop()
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.db != nil {
		_ = w.db.Close()
		w.db = nil
	}
}

func configHash(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	h := fnv.New64a()
	_, _ = h.Write(b)
	return strconv.FormatUint(h.Sum64(), 16)
}

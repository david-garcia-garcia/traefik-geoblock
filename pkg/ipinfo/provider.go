package ipinfo

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"log/slog"

	"github.com/oschwald/maxminddb-golang"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbutils"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
)

const (
	// DefaultFileName is the committed Lite MMDB at the module root.
	DefaultFileName = "ipinfo_lite.mmdb"
)

// DatabaseConfig is IPinfo Lite MMDB path and auto-update.
type DatabaseConfig struct {
	DatabaseFilePath        string
	DatabaseAutoUpdate      bool
	DatabaseAutoUpdateDir   string
	DatabaseAutoUpdateToken string
}

type liteRecord struct {
	Country       string `maxminddb:"country"`
	CountryCode   string `maxminddb:"country_code"`
	Continent     string `maxminddb:"continent"`
	ContinentCode string `maxminddb:"continent_code"`
	ASN           string `maxminddb:"asn"`
	ASName        string `maxminddb:"as_name"`
	ASDomain      string `maxminddb:"as_domain"`
}

type provider struct {
	mu     sync.RWMutex
	db     *maxminddb.Reader
	path   string
	logger *slog.Logger
	config DatabaseConfig

	updateTicker *time.Ticker
	stopChan     chan struct{}
}

var factories = dbprovider.NewRegistry()

// New opens the IPinfo Lite DatabaseProvider (singleton per config).
func New(config DatabaseConfig, logger *slog.Logger) (dbprovider.Provider, error) {
	key := configHash(config)
	v, err := factories.GetOrCreate(key, func() (any, error) {
		return newProvider(config, logger)
	})
	if err != nil {
		return nil, err
	}
	return v.(dbprovider.Provider), nil
}

func newProvider(config DatabaseConfig, logger *slog.Logger) (*provider, error) {
	p := &provider{
		logger:   logger,
		config:   config,
		stopChan: make(chan struct{}),
	}

	if config.DatabaseAutoUpdate && config.DatabaseAutoUpdateToken == "" {
		logger.Error("ipinfo_databaseAutoUpdate is true but ipinfo_databaseAutoUpdateToken is empty; IPinfo download skipped (Lite is not anonymous)")
	}

	if config.DatabaseAutoUpdate && config.DatabaseAutoUpdateDir == "" {
		return nil, fmt.Errorf("ipinfo_databaseAutoUpdateDir must be set when ipinfo_databaseAutoUpdate is true")
	}

	path, err := p.resolveInitPath()
	if err != nil {
		return nil, err
	}
	if err := p.open(path); err != nil {
		return nil, err
	}

	if p.canDownload() {
		p.startAutoUpdate()
	}
	return p, nil
}

func (p *provider) canDownload() bool {
	return p.config.DatabaseAutoUpdate && p.config.DatabaseAutoUpdateToken != "" && p.config.DatabaseAutoUpdateDir != ""
}

func (p *provider) resolveInitPath() (string, error) {
	if p.config.DatabaseAutoUpdate {
		latest, err := findLatestDatabase(p.config.DatabaseAutoUpdateDir)
		if err != nil {
			p.logger.Debug("no IPinfo MMDB in auto-update dir", "error", err)
		} else if latest != "" {
			p.logger.Debug("using IPinfo MMDB from auto-update dir", "path", latest)
			return latest, nil
		}
	}

	seed, err := resolveSeedPath(p.config.DatabaseFilePath, p.logger)
	if err != nil {
		return "", err
	}
	return seed, nil
}

func resolveSeedPath(configured string, logger *slog.Logger) (string, error) {
	if configured != "" && fileutils.Exists(configured) {
		return configured, nil
	}
	if found, err := fileutils.Default.Search(configured, DefaultFileName, logger); err == nil && found != "" {
		return found, nil
	}
	for _, cand := range []string{DefaultFileName, filepath.Join(".", DefaultFileName)} {
		if fileutils.Exists(cand) {
			return cand, nil
		}
	}
	return "", fmt.Errorf("IPinfo database file not found (set ipinfo_databaseFilePath or keep %s in the plugin tree)", DefaultFileName)
}

func (p *provider) open(path string) error {
	buf, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read IPinfo MMDB %s: %w", path, err)
	}
	db, err := maxminddb.FromBytes(buf)
	if err != nil {
		return fmt.Errorf("failed to open IPinfo MMDB %s: %w", path, err)
	}
	p.mu.Lock()
	old := p.db
	p.db = db
	p.path = path
	p.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	p.logger.Info("IPinfo database opened", "path", path)
	return nil
}

func (p *provider) Lookup(ip string) (dbprovider.Record, error) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return dbprovider.Record{}, fmt.Errorf("invalid IP address: %s", ip)
	}

	p.mu.RLock()
	db := p.db
	p.mu.RUnlock()
	if db == nil {
		return dbprovider.Record{}, fmt.Errorf("IPinfo database is not open")
	}

	var rec liteRecord
	if err := db.Lookup(parsed, &rec); err != nil {
		return dbprovider.Record{}, err
	}
	return dbprovider.Record{
		Country:       rec.CountryCode,
		CountryName:   rec.Country,
		Continent:     rec.Continent,
		ContinentCode: rec.ContinentCode,
		Isp:           rec.ASName,
		Domain:        rec.ASDomain,
		Asn:           rec.ASN,
	}, nil
}

func (p *provider) Close() error {
	if p.updateTicker != nil {
		p.updateTicker.Stop()
		select {
		case <-p.stopChan:
		default:
			close(p.stopChan)
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.db != nil {
		err := p.db.Close()
		p.db = nil
		return err
	}
	return nil
}

func (p *provider) startAutoUpdate() {
	p.updateTicker = time.NewTicker(24 * time.Hour)
	go func() {
		p.checkAndUpdate()
		for {
			select {
			case <-p.updateTicker.C:
				p.checkAndUpdate()
			case <-p.stopChan:
				return
			}
		}
	}()
}

func (p *provider) checkAndUpdate() {
	if !p.canDownload() {
		return
	}
	latest, _ := findLatestDatabase(p.config.DatabaseAutoUpdateDir)
	if latest != "" {
		if date, err := dbutils.GetDateFromName(latest); err == nil {
			if time.Since(date) < 24*time.Hour {
				p.logger.Debug("IPinfo MMDB is current", "path", latest)
				return
			}
		}
	}
	path, err := downloadAndUpdateDatabase(p.config, p.logger)
	if err != nil {
		p.logger.Error("IPinfo database update failed", "error", err)
		return
	}
	if path == "" || path == p.path {
		return
	}
	if err := p.open(path); err != nil {
		p.logger.Error("failed to open updated IPinfo MMDB", "error", err)
	}
}

func configHash(cfg DatabaseConfig) string {
	b, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Sprintf("%s_%v_%s_%s",
			cfg.DatabaseFilePath,
			cfg.DatabaseAutoUpdate,
			cfg.DatabaseAutoUpdateDir,
			cfg.DatabaseAutoUpdateToken)
	}
	h := fnv.New64a()
	_, _ = h.Write(b)
	return strconv.FormatUint(h.Sum64(), 16)
}

// resetFactories is for tests.
func resetFactories() {
	factories.Clear(func(v any) {
		_ = v.(*provider).Close()
	})
}

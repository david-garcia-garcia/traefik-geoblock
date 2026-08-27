package maxmind

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

// DatabaseConfig is MaxMind MMDB path, edition, and auto-update.
type DatabaseConfig struct {
	DatabaseFilePath        string
	DatabaseAutoUpdate      bool
	DatabaseAutoUpdateDir   string
	DatabaseAutoUpdateToken string
	DatabaseAutoUpdateCode  string
}

type geoNames struct {
	En string `maxminddb:"en"`
}

type geoCountry struct {
	IsoCode string   `maxminddb:"iso_code"`
	Names   geoNames `maxminddb:"names"`
}

type geoContinent struct {
	Code  string   `maxminddb:"code"`
	Names geoNames `maxminddb:"names"`
}

type geoCity struct {
	Names geoNames `maxminddb:"names"`
}

type geoSubdivision struct {
	IsoCode string `maxminddb:"iso_code"`
}

type mmdbRecord struct {
	Country      geoCountry       `maxminddb:"country"`
	Continent    geoContinent     `maxminddb:"continent"`
	City         geoCity          `maxminddb:"city"`
	Subdivisions []geoSubdivision `maxminddb:"subdivisions"`
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

var (
	factoryLock = dbprovider.NewInstanceLock()
	factories   = map[string]*provider{}
)

// New opens the MaxMind DatabaseProvider (singleton per config).
func New(config DatabaseConfig, logger *slog.Logger) (dbprovider.Provider, error) {
	config.DatabaseAutoUpdateCode = normalizeCode(config.DatabaseAutoUpdateCode)
	key := configHash(config)
	var out *provider
	err := factoryLock.LoadOrStore(func() bool {
		p, ok := factories[key]
		if ok {
			out = p
		}
		return ok
	}, func() error {
		p, err := newProvider(config, logger)
		if err != nil {
			return err
		}
		factories[key] = p
		out = p
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func newProvider(config DatabaseConfig, logger *slog.Logger) (*provider, error) {
	p := &provider{
		logger:   logger,
		config:   config,
		stopChan: make(chan struct{}),
	}

	if !knownEdition(config.DatabaseAutoUpdateCode) {
		return nil, fmt.Errorf("unsupported maxmind_databaseAutoUpdateCode %q (GeoLite2-Country, GeoLite2-City, GeoIP2-Country, GeoIP2-City)", config.DatabaseAutoUpdateCode)
	}

	if config.DatabaseAutoUpdate && !canParseToken(config.DatabaseAutoUpdateToken) {
		logger.Error("maxmind_databaseAutoUpdate is true but maxmind_databaseAutoUpdateToken is empty or not accountId:licenseKey; MaxMind download skipped")
	}

	if config.DatabaseAutoUpdate && config.DatabaseAutoUpdateDir == "" {
		return nil, fmt.Errorf("maxmind_databaseAutoUpdateDir must be set when maxmind_databaseAutoUpdate is true")
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

func canParseToken(token string) bool {
	_, _, ok := parseAccountToken(token)
	return ok
}

func (p *provider) canDownload() bool {
	return p.config.DatabaseAutoUpdate && canParseToken(p.config.DatabaseAutoUpdateToken) && p.config.DatabaseAutoUpdateDir != ""
}

func (p *provider) resolveInitPath() (string, error) {
	if p.config.DatabaseAutoUpdate {
		latest, err := findLatestDatabase(p.config.DatabaseAutoUpdateDir, p.config.DatabaseAutoUpdateCode)
		if err != nil {
			p.logger.Debug("no MaxMind MMDB in auto-update dir", "error", err)
		} else if latest != "" {
			p.logger.Debug("using MaxMind MMDB from auto-update dir", "path", latest)
			return latest, nil
		}
	}

	return resolveSeedPath(p.config.DatabaseFilePath, p.logger)
}

func resolveSeedPath(configured string, logger *slog.Logger) (string, error) {
	if configured != "" && fileutils.Exists(configured) {
		return configured, nil
	}
	if found, err := fileutils.Default.Search(configured, DefaultSeedFileName, logger); err == nil && found != "" {
		return found, nil
	}
	for _, cand := range []string{DefaultSeedFileName, filepath.Join(".", DefaultSeedFileName)} {
		if fileutils.Exists(cand) {
			return cand, nil
		}
	}
	return "", fmt.Errorf("MaxMind database file not found (set maxmind_databaseFilePath or keep %s in the plugin tree)", DefaultSeedFileName)
}

func (p *provider) open(path string) error {
	buf, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read MaxMind MMDB %s: %w", path, err)
	}
	db, err := maxminddb.FromBytes(buf)
	if err != nil {
		return fmt.Errorf("failed to open MaxMind MMDB %s: %w", path, err)
	}
	p.mu.Lock()
	old := p.db
	p.db = db
	p.path = path
	p.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	p.logger.Info("MaxMind database opened", "path", path)
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
		return dbprovider.Record{}, fmt.Errorf("MaxMind database is not open")
	}

	var rec mmdbRecord
	if err := db.Lookup(parsed, &rec); err != nil {
		return dbprovider.Record{}, err
	}
	region := ""
	if len(rec.Subdivisions) > 0 {
		region = rec.Subdivisions[0].IsoCode
	}
	return dbprovider.Record{
		Country:       rec.Country.IsoCode,
		CountryName:   rec.Country.Names.En,
		Continent:     rec.Continent.Names.En,
		ContinentCode: rec.Continent.Code,
		Region:        region,
		City:          rec.City.Names.En,
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
	latest, _ := findLatestDatabase(p.config.DatabaseAutoUpdateDir, p.config.DatabaseAutoUpdateCode)
	if latest != "" {
		if date, err := dbutils.GetDateFromName(latest); err == nil {
			if time.Since(date) < 24*time.Hour {
				p.logger.Debug("MaxMind MMDB is current", "path", latest)
				return
			}
		}
	}
	path, err := downloadAndUpdateDatabase(p.config, p.logger)
	if err != nil {
		p.logger.Error("MaxMind database update failed", "error", err)
		return
	}
	if path == "" || path == p.path {
		return
	}
	if err := p.open(path); err != nil {
		p.logger.Error("failed to open updated MaxMind MMDB", "error", err)
	}
}

func configHash(cfg DatabaseConfig) string {
	b, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Sprintf("%s_%v_%s_%s_%s",
			cfg.DatabaseFilePath,
			cfg.DatabaseAutoUpdate,
			cfg.DatabaseAutoUpdateDir,
			cfg.DatabaseAutoUpdateToken,
			normalizeCode(cfg.DatabaseAutoUpdateCode))
	}
	h := fnv.New64a()
	_, _ = h.Write(b)
	return strconv.FormatUint(h.Sum64(), 16)
}

func resetFactories() {
	factoryLock.Reset(func() {
		for k, p := range factories {
			_ = p.Close()
			delete(factories, k)
		}
	})
}

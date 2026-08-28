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

	"log/slog"

	"github.com/oschwald/maxminddb-golang"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbdownload"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
)

// DatabaseConfig is MaxMind MMDB seed path plus one download component config.
type DatabaseConfig struct {
	DatabaseFilePath      string
	DatabaseAutoUpdateDir string
	Download              dbdownload.Config
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

	download *dbdownload.Slot
}

var (
	factoryLock = dbprovider.NewInstanceLock()
	factories   = map[string]*provider{}
)

// New opens the MaxMind DatabaseProvider (singleton per config).
func New(config DatabaseConfig, logger *slog.Logger) (dbprovider.Provider, error) {
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
		logger: logger,
		config: config,
	}

	path, err := p.resolveInitPath()
	if err != nil {
		return nil, err
	}
	if err := p.open(path); err != nil {
		return nil, err
	}

	if err := p.startDownload(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *provider) downloadCfg() dbdownload.Config {
	return dbdownload.WithDefaults(p.config.Download, p.config.DatabaseAutoUpdateDir, dbdownload.TypeMMDB, dbdownload.DefaultMinAge)
}

func (p *provider) startDownload() error {
	slot, err := dbdownload.Start(p.downloadCfg(), p.logger, func(path string) {
		if path == "" || path == p.path {
			return
		}
		if err := p.open(path); err != nil {
			p.logger.Error("failed to open updated MaxMind MMDB", "error", err)
		}
	})
	if err != nil {
		return err
	}
	p.download = slot
	return nil
}

func (p *provider) resolveInitPath() (string, error) {
	if p.config.DatabaseAutoUpdateDir != "" && p.config.Download.Key != "" {
		latest, err := dbdownload.Latest(p.config.DatabaseAutoUpdateDir, p.config.Download.Key, dbdownload.TypeMMDB)
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
	if p.download != nil {
		p.download.Stop()
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

func configHash(cfg DatabaseConfig) string {
	b, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Sprintf("%s_%s_%s",
			cfg.DatabaseFilePath,
			cfg.DatabaseAutoUpdateDir,
			cfg.Download.URL)
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

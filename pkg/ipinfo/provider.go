package ipinfo

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net"
	"os"
	"strconv"
	"sync"

	"log/slog"

	"github.com/oschwald/maxminddb-golang"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbdownload"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
)

// DatabaseConfig is IPinfo download slot plus shared auto-update dir.
type DatabaseConfig struct {
	DatabaseAutoUpdateDir string
	Download              dbdownload.Config
}

// mmdbRecord is the union of Lite/Core/Plus fields we map onto Record.
// Lite leaves city/region empty; Core and Plus fill them.
type mmdbRecord struct {
	Country       string `maxminddb:"country"`
	CountryCode   string `maxminddb:"country_code"`
	Continent     string `maxminddb:"continent"`
	ContinentCode string `maxminddb:"continent_code"`
	Region        string `maxminddb:"region"`
	City          string `maxminddb:"city"`
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

	download *dbdownload.Slot
}

var (
	factoryLock = dbprovider.NewInstanceLock()
	factories   = map[string]*provider{}
)

// New opens the IPinfo Lite DatabaseProvider (singleton per config).
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

	path, err := dbdownload.Resolve(p.downloadCfg(), p.logger)
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
	cfg := dbdownload.WithDefaults(p.config.Download, p.config.DatabaseAutoUpdateDir, dbdownload.TypeMMDB, dbdownload.DefaultMinAge)
	if cfg.DefaultFileName == "" {
		cfg.DefaultFileName = DefaultFileName
	}
	return cfg
}

func (p *provider) startDownload() error {
	slot, err := dbdownload.Start(p.downloadCfg(), p.logger, func(path string) {
		if path == "" || path == p.path {
			return
		}
		if err := p.open(path); err != nil {
			p.logger.Error("failed to open updated IPinfo MMDB", "error", err)
		}
	})
	if err != nil {
		return err
	}
	p.download = slot
	return nil
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

	var rec mmdbRecord
	if err := db.Lookup(parsed, &rec); err != nil {
		return dbprovider.Record{}, err
	}
	return dbprovider.Record{
		Country:       rec.CountryCode,
		CountryName:   rec.Country,
		Continent:     rec.Continent,
		ContinentCode: rec.ContinentCode,
		Region:        rec.Region,
		City:          rec.City,
		Isp:           rec.ASName,
		Domain:        rec.ASDomain,
		Asn:           rec.ASN,
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
			cfg.Download.Path,
			cfg.DatabaseAutoUpdateDir,
			cfg.Download.URL)
	}
	h := fnv.New64a()
	_, _ = h.Write(b)
	return strconv.FormatUint(h.Sum64(), 16)
}

// resetFactories is for tests.
func resetFactories() {
	factoryLock.Reset(func() {
		for k, p := range factories {
			_ = p.Close()
			delete(factories, k)
		}
	})
}

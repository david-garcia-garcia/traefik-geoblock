package traefik_geoblock

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/http"
	"strconv"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/geoblock"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/logging"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/reclaim"
)

//go:generate go run ./tools/dbdownload/main.go -o ./seeds/IP2LOCATION-LITE-DB1.IPV6.BIN

const keyPrefixPlugin = "plugin:"

// Config is the Traefik plugin configuration Yaegi decodes.
type Config = geoblock.Config

// CreateConfig returns default plugin configuration.
func CreateConfig() *Config {
	return geoblock.CreateConfig()
}

// New is the Traefik Yaegi constructor. It reuses one Plugin per name+config on the process table.
func New(ctx context.Context, next http.Handler, cfg *Config, name string) (http.Handler, error) {
	if next == nil {
		return nil, fmt.Errorf("%s: no next handler provided", name)
	}
	if cfg == nil {
		return nil, fmt.Errorf("%s: no config provided", name)
	}

	bootstrapLogger := logging.NewBootstrap(name, cfg.LogLevel)
	logger := logging.New(name, cfg.LogLevel, cfg.LogFormat, bootstrapLogger)

	if !cfg.Enabled {
		return bindPlugin(ctx, next, name, cfg, nil, func() (*geoblock.Plugin, error) {
			return geoblock.NewCore(name, cfg, nil, logger)
		})
	}

	if err := geoblock.Prepare(cfg, name, logger); err != nil {
		return nil, err
	}
	// Bind wrappers to this New ctx even when the Plugin incarnation is reused.
	db, err := geoblock.OpenDatabase(ctx, cfg, bootstrapLogger)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return bindPlugin(ctx, next, name, cfg, db, func() (*geoblock.Plugin, error) {
		return geoblock.NewCore(name, cfg, db, logger)
	})
}

// bindPlugin stores or reclaims the Plugin for name+config, then attaches this next and provider.
func bindPlugin(ctx context.Context, next http.Handler, name string, cfg *Config, db dbprovider.Provider, create func() (*geoblock.Plugin, error)) (http.Handler, error) {
	v, err := reclaim.Open(ctx, pluginKey(name, cfg), func() (any, error) {
		return create()
	}, func(any) {})
	if err != nil {
		return nil, err
	}
	core, ok := v.(*geoblock.Plugin)
	if !ok {
		return nil, fmt.Errorf("%s: reclaim: want *geoblock.Plugin, got %T", name, v)
	}
	return core.ForRoute(next, db), nil
}

// pluginKey is the process-table key for one Plugin incarnation.
func pluginKey(name string, cfg *Config) string {
	return keyPrefixPlugin + name + ":" + pluginConfigHash(cfg)
}

// pluginConfigHash is JSON+FNV of cfg after Prepare. encoding/json sorts map keys.
func pluginConfigHash(cfg *Config) string {
	b, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Sprintf("%v", cfg)
	}
	h := fnv.New64a()
	_, _ = h.Write(b)
	return strconv.FormatUint(h.Sum64(), 16)
}

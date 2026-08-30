package traefik_geoblock

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/http"
	"strconv"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/geoblock"
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

// New is the Traefik Yaegi constructor. It reuses one Plugin per name+config and returns a Route.
func New(ctx context.Context, next http.Handler, cfg *Config, name string) (http.Handler, error) {
	if next == nil {
		return nil, fmt.Errorf("%s: no next handler provided", name)
	}
	if cfg == nil {
		return nil, fmt.Errorf("%s: no config provided", name)
	}
	// Normalize before hashing so two New with the same incoming config share a key.
	if err := geoblock.Prepare(cfg, name); err != nil {
		return nil, err
	}
	return bindPlugin(ctx, next, name, cfg)
}

// bindPlugin stores or reclaims the NewCore Plugin, then ForRoutes this next.
func bindPlugin(ctx context.Context, next http.Handler, name string, cfg *Config) (http.Handler, error) {
	stored, err := reclaim.Open(ctx, pluginKey(name, cfg), func() (any, error) {
		return geoblock.NewCore(name, cfg)
	})
	if err != nil {
		return nil, err
	}
	pluginInstance, ok := stored.(*geoblock.Plugin)
	if !ok {
		return nil, fmt.Errorf("%s: reclaim: want *geoblock.Plugin, got %T", name, stored)
	}
	return pluginInstance.ForRoute(next)
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

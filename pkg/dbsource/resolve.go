package dbsource

import (
	"log/slog"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
)

// Resolve returns the file to open: dated update, then catalog Path (existing file),
// then the bundled default at {TRAEFIK_PLUGIN_GEOBLOCK_PATH}/seeds/<name> or {env}/<name>.
func Resolve(cfg Config, logger *slog.Logger) (string, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if err := Normalize(&cfg); err != nil {
		return "", err
	}
	if latest, err := Latest(cfg.Dir, cfg.Key, cfg.DatabaseType); err == nil && latest != "" {
		return latest, nil
	}
	if cfg.Path != "" {
		if fileutils.Exists(cfg.Path) {
			return cfg.Path, nil
		}
		logger.Warn("seed was specified but not found", "path", cfg.Path)
	}
	if cfg.DefaultFileName == "" {
		return "", nil
	}
	found, err := fileutils.Default.Search("", cfg.DefaultFileName, logger)
	if err != nil {
		return "", err
	}
	return found, nil
}

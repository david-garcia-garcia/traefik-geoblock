package dbdownload

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
)

// Resolve returns the file to open: dated update, then catalog Path (existing file),
// then the bundled default name under TRAEFIK_PLUGIN_GEOBLOCK_PATH (including seeds/).
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
	if cfg.Path != "" && fileutils.Exists(cfg.Path) {
		return cfg.Path, nil
	}
	if cfg.DefaultFileName == "" {
		if cfg.Path != "" {
			return "", fmt.Errorf("database file not found: %s", cfg.Path)
		}
		return "", nil
	}
	if found, err := fileutils.Default.Search("", cfg.DefaultFileName, logger); err == nil && found != "" {
		return found, nil
	}
	for _, cand := range []string{
		cfg.DefaultFileName,
		filepath.Join(".", cfg.DefaultFileName),
		filepath.Join(SeedDir, cfg.DefaultFileName),
		filepath.Join(".", SeedDir, cfg.DefaultFileName),
	} {
		if fileutils.Exists(cand) {
			return cand, nil
		}
	}
	return "", fmt.Errorf("database file not found (%s)", cfg.DefaultFileName)
}

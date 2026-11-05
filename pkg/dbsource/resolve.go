package dbsource

import (
	"log/slog"
	"strings"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
)

// Resolve returns the file to open: dated update, then catalog Path (existing file),
// then the bundled default Search locates under the plugin directory.
func Resolve(cfg Config, logger *slog.Logger) (string, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if err := Normalize(&cfg); err != nil {
		return "", err
	}
	// Newest dated catalog file wins.
	if latest, err := Latest(cfg.Dir, cfg.Key, cfg.DatabaseType); err == nil && latest != "" {
		return latest, nil
	}
	// Operator seed path, or WARN and continue when it is missing.
	if cfg.Path != "" {
		if fileutils.Exists(cfg.Path) {
			return cfg.Path, nil
		}
		logger.Warn("seed was specified but not found", "path", cfg.Path)
	}
	return BundledFile(cfg)
}

// BundledFile is the plugin-directory defaultFile (seeds/<name> then <name>), or empty
// when the row has no defaultFile. It does not log: the caller picks the level of a miss.
func BundledFile(cfg Config) (string, error) {
	if strings.TrimSpace(cfg.DefaultFileName) == "" {
		return "", nil
	}
	return fileutils.Default.Search("", cfg.DefaultFileName)
}

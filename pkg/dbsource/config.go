package dbsource

import (
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"
)

const (
	TypeBIN  = "bin"
	TypeMMDB = "mmdb"

	ArchiveNone  = "none"
	ArchiveZIP   = "zip"
	ArchiveTarGz = "tar.gz"

	lockMaxAge = time.Hour

	// SeedDir is the committed bundled-database folder. Default filenames stay basenames.
	SeedDir = "seeds"
)

// Config is one catalog source the shared component can resolve and keep current.
type Config struct {
	Key             string
	URL             string
	Headers         map[string]string
	DatabaseType    string
	Archive         string
	Dir             string
	MinAge          time.Duration
	Path            string
	DefaultFileName string
}

// Ext is the on-disk suffix after unpack.
func Ext(databaseType string) string {
	if strings.EqualFold(strings.TrimSpace(databaseType), TypeBIN) {
		return ".BIN"
	}
	return ".mmdb"
}

// Normalize trims fields, infers empty archive from the URL path, and validates.
func Normalize(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("download config is nil")
	}
	cfg.Key = strings.TrimSpace(cfg.Key)
	cfg.URL = strings.TrimSpace(cfg.URL)
	cfg.Dir = strings.TrimSpace(cfg.Dir)
	cfg.Path = strings.TrimSpace(cfg.Path)
	cfg.DefaultFileName = strings.TrimSpace(cfg.DefaultFileName)
	cfg.DatabaseType = strings.ToLower(strings.TrimSpace(cfg.DatabaseType))
	cfg.Archive = strings.ToLower(strings.TrimSpace(cfg.Archive))
	if cfg.Archive == "tgz" {
		cfg.Archive = ArchiveTarGz
	}
	if cfg.DatabaseType != "" && cfg.DatabaseType != TypeBIN && cfg.DatabaseType != TypeMMDB {
		return fmt.Errorf("unknown databaseType %q (bin, mmdb)", cfg.DatabaseType)
	}
	if cfg.Archive != "" {
		switch cfg.Archive {
		case ArchiveNone, ArchiveZIP, ArchiveTarGz:
		default:
			return fmt.Errorf("unknown archive %q (none, zip, tar.gz)", cfg.Archive)
		}
	}
	if cfg.URL == "" {
		return nil
	}
	if cfg.Key == "" {
		return fmt.Errorf("download catalog key is empty")
	}
	if cfg.DatabaseType != TypeBIN && cfg.DatabaseType != TypeMMDB {
		return fmt.Errorf("unknown databaseType %q (bin, mmdb)", cfg.DatabaseType)
	}
	if cfg.Archive == "" {
		inferred, err := inferArchive(cfg.URL)
		if err != nil {
			return err
		}
		cfg.Archive = inferred
	}
	return nil
}

func inferArchive(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("download URL is not a valid URL")
	}
	name := strings.ToLower(path.Base(u.Path))
	switch {
	case strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz"):
		return ArchiveTarGz, nil
	case strings.HasSuffix(name, ".zip"):
		return ArchiveZIP, nil
	case strings.HasSuffix(name, ".mmdb") || strings.HasSuffix(name, ".bin"):
		return ArchiveNone, nil
	default:
		return "", fmt.Errorf("archive is empty and the URL path has no recognized extension")
	}
}

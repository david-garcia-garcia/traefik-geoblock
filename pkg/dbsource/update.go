package dbsource

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbutils"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
)

// Latest returns the newest dated file for key in dir.
func Latest(dir, key, databaseType string) (string, error) {
	if dir == "" || key == "" {
		return "", nil
	}
	return dbutils.FindLatestDatedFile(dir, dbutils.DatedKeyGlob(key, Ext(databaseType)))
}

// Update downloads and stores a dated file when the URL is set.
func Update(cfg Config, logger *slog.Logger) (string, error) {
	if err := Normalize(&cfg); err != nil {
		return "", err
	}
	if cfg.URL == "" {
		return Latest(cfg.Dir, cfg.Key, cfg.DatabaseType)
	}
	if cfg.Dir == "" {
		return "", fmt.Errorf("databaseAutoUpdateDir is required for download")
	}
	if logger == nil {
		logger = slog.Default()
	}

	if err := os.MkdirAll(cfg.Dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create download dir: %w", err)
	}

	lockFile := filepath.Join(cfg.Dir, "update-"+cfg.Key+".lock")
	if fi, err := os.Stat(lockFile); err == nil {
		age := time.Since(fi.ModTime())
		if age < lockMaxAge {
			logger.Debug("download already in progress", "key", cfg.Key, "age", age)
			return Latest(cfg.Dir, cfg.Key, cfg.DatabaseType)
		}
		logger.Warn("removing stale download lock", "key", cfg.Key, "age", age)
		if err := os.Remove(lockFile); err != nil {
			return "", fmt.Errorf("failed to remove stale lock: %w", err)
		}
	}
	lock, err := os.Create(lockFile)
	if err != nil {
		return "", fmt.Errorf("failed to create lock file: %w", err)
	}
	defer func() {
		lock.Close()
		_ = os.Remove(lockFile)
	}()

	tmpDir, err := os.MkdirTemp(cfg.Dir, "dbsource-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	resp, err := dbutils.HTTPGet(cfg.URL, cfg.Headers)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	contentType := resp.Header.Get("Content-Type")
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %s (%s)", resp.Status, dbutils.DownloadHint(
			cfg.Key, resp.Status, contentType, resp.ContentLength, dbutils.ReadPrefix(resp.Body, dbutils.DownloadHintPrefixBytes)))
	}

	tmpPath, err := unpackBody(resp.Body, tmpDir, cfg.Archive, cfg.DatabaseType)
	if err != nil {
		return "", fmt.Errorf("%w (%s)", err, dbutils.DownloadHint(
			cfg.Key, resp.Status, contentType, resp.ContentLength, ""))
	}

	datePrefix, err := fileDate(tmpPath, cfg.DatabaseType)
	if err != nil {
		return "", fmt.Errorf("%w (%s)", err, dbutils.DownloadHintFromFile(
			cfg.Key, resp.Status, contentType, tmpPath))
	}

	finalName := datePrefix + "_" + cfg.Key + Ext(cfg.DatabaseType)
	finalPath := filepath.Join(cfg.Dir, finalName)
	if fileutils.Exists(finalPath) {
		logger.Warn("the available database is not newer than the one already available, database did not update",
			"key", cfg.Key, "path", finalPath)
		return finalPath, nil
	}
	if err := fileutils.Copy(tmpPath, finalPath, false); err != nil {
		return "", fmt.Errorf("failed to copy download to %s: %w", finalPath, err)
	}
	logger.Info("database updated successfully", "key", cfg.Key, "path", finalPath)
	return finalPath, nil
}

// UpdateIfNeeded downloads when Latest is missing or older than MinAge.
func UpdateIfNeeded(cfg Config, logger *slog.Logger) (string, error) {
	if err := Normalize(&cfg); err != nil {
		return "", err
	}
	if cfg.URL == "" {
		return Latest(cfg.Dir, cfg.Key, cfg.DatabaseType)
	}
	latest, err := Latest(cfg.Dir, cfg.Key, cfg.DatabaseType)
	if err != nil {
		return "", err
	}
	minAge := cfg.MinAge
	if minAge <= 0 {
		minAge = 24 * time.Hour
	}
	if latest != "" {
		if date, err := dbutils.GetDateFromName(latest); err == nil && time.Since(date) < minAge {
			if logger != nil {
				logger.Debug("download is current", "key", cfg.Key, "path", latest)
			}
			return latest, nil
		}
	}
	return Update(cfg, logger)
}

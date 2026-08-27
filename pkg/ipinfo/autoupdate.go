package ipinfo

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/oschwald/maxminddb-golang"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbutils"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
)

const (
	dataDownloadBase = "https://ipinfo.io/data/"
	updateLockMaxAge = time.Hour
)

var getDownloadURL = defaultDownloadURL

func defaultDownloadURL(token, code string) string {
	if token == "" {
		return ""
	}
	return dataDownloadBase + fileNameForCode(code) + "?token=" + url.QueryEscape(token)
}

func packageDownloadURL(token, code string) string {
	return getDownloadURL(token, code)
}

// mmdbBuildDate is the MMDB metadata build_epoch as UTC (same role as the IP2Location BIN header date).
func mmdbBuildDate(reader *maxminddb.Reader) (time.Time, error) {
	if reader == nil || reader.Metadata.BuildEpoch == 0 {
		return time.Time{}, fmt.Errorf("IPinfo MMDB has no build_epoch")
	}
	return time.Unix(int64(reader.Metadata.BuildEpoch), 0).UTC(), nil
}

func findLatestDatabase(dir, code string) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create IPinfo auto-update dir: %w", err)
	}
	files, err := filepath.Glob(filepath.Join(dir, "*_"+fileNameForCode(code)))
	if err != nil {
		return "", err
	}
	var latest string
	var latestDate time.Time
	for _, f := range files {
		date, err := dbutils.GetDateFromName(f)
		if err != nil {
			continue
		}
		if latest == "" || date.After(latestDate) {
			latest = f
			latestDate = date
		}
	}
	return latest, nil
}

func downloadAndUpdateDatabase(cfg DatabaseConfig, logger *slog.Logger) (string, error) {
	if cfg.DatabaseAutoUpdateToken == "" {
		return "", fmt.Errorf("IPinfo download requires ipinfo_databaseAutoUpdateToken")
	}
	if cfg.DatabaseAutoUpdateDir == "" {
		return "", fmt.Errorf("ipinfo_databaseAutoUpdateDir is required for download")
	}

	if err := os.MkdirAll(cfg.DatabaseAutoUpdateDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create IPinfo auto-update dir: %w", err)
	}

	lockFile := filepath.Join(cfg.DatabaseAutoUpdateDir, "update-ipinfo.lock")
	if fi, err := os.Stat(lockFile); err == nil {
		age := time.Since(fi.ModTime())
		if age < updateLockMaxAge {
			logger.Debug("IPinfo update already in progress", "lock", lockFile, "age", age)
			return findLatestDatabase(cfg.DatabaseAutoUpdateDir, cfg.DatabaseAutoUpdateCode)
		}
		logger.Warn("removing stale IPinfo update lock", "lock", lockFile, "age", age)
		if err := os.Remove(lockFile); err != nil {
			return "", fmt.Errorf("failed to remove stale lock %s: %w", lockFile, err)
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

	tmpDir, err := os.MkdirTemp(cfg.DatabaseAutoUpdateDir, "ipinfo-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	downloadURL := packageDownloadURL(cfg.DatabaseAutoUpdateToken, cfg.DatabaseAutoUpdateCode)
	client := &http.Client{Timeout: downloadTimeoutFor(cfg.DatabaseAutoUpdateCode)}
	resp, err := client.Get(downloadURL) // #nosec G107
	if err != nil {
		return "", fmt.Errorf("IPinfo download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("IPinfo download failed with status: %s", resp.Status)
	}

	tmpPath := filepath.Join(tmpDir, fileNameForCode(cfg.DatabaseAutoUpdateCode))
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to create temp MMDB: %w", err)
	}
	if _, err := io.Copy(tmpFile, io.LimitReader(resp.Body, maxDownloadBytesFor(cfg.DatabaseAutoUpdateCode))); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to save IPinfo MMDB: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", err
	}

	buf, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to read downloaded IPinfo MMDB: %w", err)
	}
	reader, err := maxminddb.FromBytes(buf)
	if err != nil {
		return "", fmt.Errorf("downloaded file is not a valid MMDB: %w", err)
	}
	buildDate, err := mmdbBuildDate(reader)
	_ = reader.Close()
	if err != nil {
		return "", err
	}

	finalName := buildDate.Format("20060102") + "_" + fileNameForCode(cfg.DatabaseAutoUpdateCode)
	finalPath := filepath.Join(cfg.DatabaseAutoUpdateDir, finalName)
	if fileutils.Exists(finalPath) {
		logger.Warn("the available IPinfo database is not newer than the one already available, database did not update", "path", finalPath)
		return finalPath, nil
	}
	if err := fileutils.Copy(tmpPath, finalPath, false); err != nil {
		return "", fmt.Errorf("failed to copy IPinfo MMDB to %s: %w", finalPath, err)
	}
	logger.Info("IPinfo database updated", "path", finalPath)
	return finalPath, nil
}

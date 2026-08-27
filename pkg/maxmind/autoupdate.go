package maxmind

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/oschwald/maxminddb-golang"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbutils"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
)

const (
	downloadBase     = "https://download.maxmind.com/geoip/databases/"
	updateLockMaxAge = time.Hour
)

var getDownloadURL = defaultDownloadURL

func defaultDownloadURL(code string) string {
	return downloadBase + normalizeCode(code) + "/download?suffix=tar.gz"
}

func parseAccountToken(token string) (accountID, licenseKey string, ok bool) {
	token = strings.TrimSpace(token)
	i := strings.IndexByte(token, ':')
	if i <= 0 || i >= len(token)-1 {
		return "", "", false
	}
	return token[:i], token[i+1:], true
}

func mmdbBuildDate(reader *maxminddb.Reader) (time.Time, error) {
	if reader == nil || reader.Metadata.BuildEpoch == 0 {
		return time.Time{}, fmt.Errorf("MaxMind MMDB has no build_epoch")
	}
	epoch := reader.Metadata.BuildEpoch
	if uint64(epoch) > uint64(math.MaxInt64) {
		return time.Time{}, fmt.Errorf("MaxMind MMDB build_epoch overflows int64")
	}
	//nolint:gosec // G115: epoch is bounded to MaxInt64 above
	return time.Unix(int64(epoch), 0).UTC(), nil
}

func findLatestDatabase(dir, code string) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create MaxMind auto-update dir: %w", err)
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
	accountID, licenseKey, ok := parseAccountToken(cfg.DatabaseAutoUpdateToken)
	if !ok {
		return "", fmt.Errorf("MaxMind download requires maxmind_databaseAutoUpdateToken as accountId:licenseKey")
	}
	if cfg.DatabaseAutoUpdateDir == "" {
		return "", fmt.Errorf("maxmind_databaseAutoUpdateDir is required for download")
	}

	if err := os.MkdirAll(cfg.DatabaseAutoUpdateDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create MaxMind auto-update dir: %w", err)
	}

	lockFile := filepath.Join(cfg.DatabaseAutoUpdateDir, "update-maxmind.lock")
	if fi, err := os.Stat(lockFile); err == nil {
		age := time.Since(fi.ModTime())
		if age < updateLockMaxAge {
			logger.Debug("MaxMind update already in progress", "lock", lockFile, "age", age)
			return findLatestDatabase(cfg.DatabaseAutoUpdateDir, cfg.DatabaseAutoUpdateCode)
		}
		logger.Warn("removing stale MaxMind update lock", "lock", lockFile, "age", age)
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

	tmpDir, err := os.MkdirTemp(cfg.DatabaseAutoUpdateDir, "maxmind-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	downloadURL := getDownloadURL(cfg.DatabaseAutoUpdateCode)
	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("MaxMind download request: %w", err)
	}
	req.SetBasicAuth(accountID, licenseKey)

	client := &http.Client{Timeout: downloadTimeout()}
	resp, err := client.Do(req) // #nosec G107
	if err != nil {
		return "", fmt.Errorf("MaxMind download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("MaxMind download failed with status: %s", resp.Status)
	}

	tmpPath, err := extractMMDB(resp.Body, tmpDir)
	if err != nil {
		return "", err
	}

	buf, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to read extracted MaxMind MMDB: %w", err)
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
		logger.Warn("the available MaxMind database is not newer than the one already available, database did not update", "path", finalPath)
		return finalPath, nil
	}
	if err := fileutils.Copy(tmpPath, finalPath, false); err != nil {
		return "", fmt.Errorf("failed to copy MaxMind MMDB to %s: %w", finalPath, err)
	}
	logger.Info("MaxMind database updated", "path", finalPath)
	return finalPath, nil
}

func extractMMDB(r io.Reader, destDir string) (string, error) {
	gz, err := gzip.NewReader(io.LimitReader(r, maxDownloadBytes()))
	if err != nil {
		return "", fmt.Errorf("MaxMind download is not gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("MaxMind tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		name := path.Base(filepath.ToSlash(hdr.Name))
		if strings.Contains(hdr.Name, "..") || !strings.HasSuffix(strings.ToLower(name), ".mmdb") {
			continue
		}
		out := filepath.Join(destDir, name)
		f, err := os.Create(out)
		if err != nil {
			return "", fmt.Errorf("failed to create extracted MMDB: %w", err)
		}
		if _, err := io.Copy(f, io.LimitReader(tr, maxDownloadBytes())); err != nil {
			f.Close()
			return "", fmt.Errorf("failed to extract MMDB: %w", err)
		}
		if err := f.Close(); err != nil {
			return "", err
		}
		return out, nil
	}
	return "", fmt.Errorf("MaxMind archive contained no .mmdb")
}

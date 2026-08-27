package ip2location

import (
	"archive/zip"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbutils"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
)

const (
	liteDownloadURL  = "https://download.ip2location.com/lite/IP2LOCATION-LITE-DB1.IPV6.BIN.ZIP"
	tokenDownloadURL = "https://www.ip2location.com/download?token=%s&file=%s" // #nosec G101

	// DefaultASNDatabaseCode is the official IP2Location package code for
	// ASN LITE IPv6 BIN (file=DBASNLITEBINIPV6).
	DefaultASNDatabaseCode = "DBASNLITEBINIPV6"

	defaultGeoFileName = "IP2LOCATION-LITE-DB1.IPV6.BIN"
	defaultASNFileName = "IP2LOCATION-LITE-ASN.IPV6.BIN"

	// ASN LITE IPv6 BIN is ~264MB; geo LITE is much smaller.
	maxExtractBytes = 512 * 1024 * 1024
)

// ip2locationDownloadURL builds the auto-update download URL.
// Geo LITE DB1 is on the public lite CDN (no token). ASN LITE is not;
// it requires a download token and file= the official package code.
func ip2locationDownloadURL(token, dbCode string) string {
	if token != "" {
		return fmt.Sprintf(tokenDownloadURL, token, dbCode)
	}
	if isASNDatabaseCode(dbCode) {
		return ""
	}
	return liteDownloadURL
}

func isASNDatabaseCode(code string) bool {
	return strings.Contains(strings.ToUpper(code), "ASN")
}

func defaultFileNameForCode(code string) string {
	if isASNDatabaseCode(code) {
		return defaultASNFileName
	}
	return defaultGeoFileName
}

func defaultDatabaseCode(code string) string {
	if code != "" {
		return code
	}
	return "DB1"
}

// UpdateIfNeeded checks if the database needs updating and performs the update if necessary.
// If runSync is true, the update will be performed synchronously, otherwise it runs in background.
func UpdateIfNeeded(dbPath string, runSync bool, logger *slog.Logger, config *DatabaseConfig) error {
	var performUpdate bool
	if dbPath == "" {
		// Empty path means we need to update
		logger.Info("no database path provided, update needed")
		performUpdate = true
	} else {
		dbDate, err := dbutils.GetDateFromName(dbPath)
		if err != nil {
			logger.Warn("cannot determine database age", "error", err)
			performUpdate = true
		} else if time.Since(dbDate) > 30*24*time.Hour {
			// Database is older than a month, update
			logger.Info("database is older than 30 days, updating", "sync", runSync)
			performUpdate = true
		}
	}

	if !performUpdate {
		return nil
	}

	if runSync {
		return downloadAndUpdateDatabase(config, logger)
	}

	// Run update asynchronously
	go func() {
		if err := downloadAndUpdateDatabase(config, logger); err != nil {
			logger.Error("async database update failed", "error", err)
		}
	}()
	logger.Info("database update started asynchronously")
	return nil
}

// findLatestDatabase finds the most recent database file in the specified directory
func findLatestDatabase(dir string, dbCode string) (string, error) {
	dbCode = defaultDatabaseCode(dbCode)
	return dbutils.FindLatestDatedFile(dir, fmt.Sprintf("*IP2LOCATION-LITE-%s.IPV6.BIN", dbCode))
}

func downloadAndUpdateDatabase(cfg *DatabaseConfig, logger *slog.Logger) error {
	dbCode := defaultDatabaseCode(cfg.DatabaseAutoUpdateCode)
	if isASNDatabaseCode(dbCode) && cfg.DatabaseAutoUpdateToken == "" {
		return fmt.Errorf("ASN database download requires ip2location_databaseAutoUpdateToken (file=%s is not on the public lite CDN)", dbCode)
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(cfg.DatabaseAutoUpdateDir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// Create lock file
	lockFile := filepath.Join(cfg.DatabaseAutoUpdateDir, "update-"+dbCode+".lock")

	// Check if lock file exists and its age
	if fi, err := os.Stat(lockFile); err == nil {
		age := time.Since(fi.ModTime())
		if age < time.Hour {
			logger.Debug("another update is in progress", "lock", lockFile, "age", age)
			return nil
		}
		logger.Warn("removing stale lock file", "lock", lockFile, "age", age)
		if err := os.Remove(lockFile); err != nil {
			return fmt.Errorf("failed to remove stale lock file %s: %v", lockFile, err)
		}
	}

	// Create lock file
	lock, err := os.Create(lockFile)
	if err != nil {
		return fmt.Errorf("failed to create lock file: %w", err)
	}
	defer func() {
		lock.Close()
		os.Remove(lockFile)
	}()

	// Create temporary directory
	tmpDir, err := os.MkdirTemp(cfg.DatabaseAutoUpdateDir, "ip2location-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	downloadURL := ip2locationDownloadURL(cfg.DatabaseAutoUpdateToken, dbCode)

	resp, err := http.Get(downloadURL) // #nosec G107
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s (%s)", resp.Status, dbutils.DownloadHint(
			dbCode, resp.Status, contentType, resp.ContentLength, dbutils.ReadPrefix(resp.Body, dbutils.DownloadHintPrefixBytes)))
	}

	// Save and process zip file
	zipPath := filepath.Join(tmpDir, "database.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}

	if _, err := io.Copy(zipFile, resp.Body); err != nil {
		zipFile.Close()
		return fmt.Errorf("failed to save zip file: %w", err)
	}
	zipFile.Close()

	// Extract database file
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w (%s)", err, dbutils.DownloadHintFromFile(
			dbCode, resp.Status, contentType, zipPath))
	}
	defer reader.Close()

	var dbFile *os.File
	for _, file := range reader.File {
		if filepath.Ext(file.Name) == ".BIN" {
			rc, err := file.Open()
			if err != nil {
				return fmt.Errorf("failed to open zip entry: %w", err)
			}

			dbFile, err = os.Create(filepath.Join(tmpDir, "database.bin"))
			if err != nil {
				rc.Close()
				return fmt.Errorf("failed to create database file: %w", err)
			}

			limited := io.LimitReader(rc, maxExtractBytes)
			_, err = io.Copy(dbFile, limited) // #nosec G110
			rc.Close()
			if err != nil {
				dbFile.Close()
				return fmt.Errorf("failed to extract database: %w", err)
			}
			break
		}
	}

	if dbFile == nil {
		return fmt.Errorf("no database file found in archive")
	}
	dbFile.Close()

	// Verify database and get version for naming
	tmpDBPath := filepath.Join(tmpDir, "database.bin")
	version, err := dbutils.GetDatabaseVersion(tmpDBPath)
	if err != nil {
		return fmt.Errorf("invalid database file: %w", err)
	}

	// Use database version date for the filename
	finalName := fmt.Sprintf("%s_IP2LOCATION-LITE-%s.IPV6.BIN", version.Date().Format("20060102"), dbCode)
	finalPath := filepath.Join(cfg.DatabaseAutoUpdateDir, finalName)

	// Check if the database file already exists (same version already downloaded)
	if fileutils.Exists(finalPath) {
		logger.Warn("the available IP2Location database is not newer than the one already available, database did not update", "path", finalPath)
		return nil
	}

	if err := fileutils.Copy(tmpDBPath, finalPath, false); err != nil {
		return fmt.Errorf("failed to copy database to final location: %w", err)
	}

	logger.Info("database updated successfully" + finalPath)
	return nil
}

package traefik_geoblock

import (
	"bufio"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"log/slog"
)

// sharedIpLookupMonitor is the actual monitor that gets shared across multiple plugin instances
type sharedIpLookupMonitor struct {
	helper        *IpLookupHelper
	directoryPath string
	staticBlocks  []string
	lastFileInfo  map[string]time.Time // Track modification times of all files
	logger        *slog.Logger
	ticker        *time.Ticker
	stopChan      chan struct{}
	checkInterval time.Duration
}

// Removed IpLookupFileMonitor wrapper - just use sharedIpLookupMonitor directly

// Global monitor manager (similar to database factory pattern)
var (
	monitorMutex sync.RWMutex
	monitors     = make(map[string]*sharedIpLookupMonitor)
)

// generateMonitorHash creates a unique hash key from CIDR blocks and directory path
func generateMonitorHash(cidrBlocks []string, directoryPath string) string {
	config := map[string]interface{}{
		"directoryPath": directoryPath,
		"cidrBlocks":    cidrBlocks,
	}

	// Serialize to JSON for consistent hashing
	configBytes, err := json.Marshal(config)
	if err != nil {
		// Fallback to simple concatenation if marshaling fails
		allBlocks := strings.Join(cidrBlocks, "|")
		return fmt.Sprintf("dir:%s|blocks:%s", directoryPath, allBlocks)
	}

	// Generate MD5 hash (sufficient for cache key)
	hash := md5.Sum(configBytes)
	return fmt.Sprintf("%x", hash)
}

// NewIpLookupFileMonitor creates or reuses a shared IP lookup monitor
// Multiple calls with the same parameters will return references to the same underlying monitor
// The logger's plugin name (if available) will be used to track which routes are using shared monitors
func NewIpLookupFileMonitor(cidrBlocks []string, directoryPath string, logger *slog.Logger) (*sharedIpLookupMonitor, error) {
	// Generate unique key from parameters
	key := generateMonitorHash(cidrBlocks, directoryPath)

	monitorMutex.RLock()
	if shared, exists := monitors[key]; exists {
		monitorMutex.RUnlock()
		logger.Debug("reusing shared IP lookup monitor",
			"key", key,
			"directory", directoryPath,
			"static_blocks", len(cidrBlocks))

		return shared, nil
	}
	monitorMutex.RUnlock()

	// Create new shared monitor
	monitorMutex.Lock()
	defer monitorMutex.Unlock()

	// Double-check pattern
	if shared, exists := monitors[key]; exists {
		logger.Debug("reusing shared IP lookup monitor (double-check)", "key", key)
		return shared, nil
	}

	// Random jitter between 0-30 seconds to prevent thundering herd when many routes exist
	jitter := time.Duration(rand.Intn(30)) * time.Second
	checkInterval := 60*time.Second + jitter

	shared := &sharedIpLookupMonitor{
		staticBlocks:  cidrBlocks,
		directoryPath: directoryPath,
		logger:        logger,
		checkInterval: checkInterval,
		stopChan:      make(chan struct{}),
		lastFileInfo:  make(map[string]time.Time),
	}

	// Initial load
	if err := shared.reload(); err != nil {
		return nil, fmt.Errorf("failed to load initial IP blocks: %w", err)
	}

	// Start background directory monitoring if we have a directory path
	if directoryPath != "" {
		shared.startFileMonitoring()
	}

	// Register the shared monitor
	monitors[key] = shared

	logger.Debug("created new shared IP lookup monitor",
		"key", key,
		"directory", directoryPath,
		"static_blocks", len(cidrBlocks),
		"check_interval", checkInterval)

	return shared, nil
}

// IsContained checks if an IP is contained in any of the CIDR blocks
// This is super fast - just a direct call, no synchronization needed
func (m *sharedIpLookupMonitor) IsContained(ipAddr net.IP) (bool, int, error) {
	return m.helper.IsContained(ipAddr)
}

// startFileMonitoring starts a background goroutine that periodically checks for directory changes
func (m *sharedIpLookupMonitor) startFileMonitoring() {
	m.ticker = time.NewTicker(m.checkInterval)

	// using a ticker here on purpose instead of FS events to make sure it works on NFS
	go func() {
		defer m.ticker.Stop()

		for {
			select {
			case <-m.ticker.C:
				if err := m.checkAndReload(); err != nil {
					m.logger.Warn("failed to reload IP blocks", "error", err, "directory", m.directoryPath)
				}
			case <-m.stopChan:
				m.logger.Debug("stopping directory monitoring", "directory", m.directoryPath)
				return
			}
		}
	}()

	m.logger.Debug("started directory monitoring", "directory", m.directoryPath, "interval", m.checkInterval)
}

// checkAndReload checks if any files in the directory have been modified and reloads if necessary
func (m *sharedIpLookupMonitor) checkAndReload() error {
	// If no directory path is configured, nothing to check
	if m.directoryPath == "" {
		return nil
	}

	// Check if directory exists
	if _, err := os.Stat(m.directoryPath); err != nil {
		if os.IsNotExist(err) {
			// Directory doesn't exist, use only static blocks
			return nil
		}
		return fmt.Errorf("failed to stat directory %s: %w", m.directoryPath, err)
	}

	// Get current file info for all .txt files in directory
	currentFileInfo, err := m.getDirectoryFileInfo()
	if err != nil {
		return fmt.Errorf("failed to scan directory %s: %w", m.directoryPath, err)
	}

	// Check if anything changed
	changed := false

	// Check if any files were added or modified
	for fileName, modTime := range currentFileInfo {
		if lastModTime, exists := m.lastFileInfo[fileName]; !exists || !modTime.Equal(lastModTime) {
			changed = true
			m.logger.Debug("file change detected", "file", fileName, "old_time", lastModTime, "new_time", modTime)
			break
		}
	}

	// Check if any files were deleted
	if !changed {
		for fileName := range m.lastFileInfo {
			if _, exists := currentFileInfo[fileName]; !exists {
				changed = true
				m.logger.Debug("file deletion detected", "file", fileName)
				break
			}
		}
	}

	if !changed {
		return nil // No changes
	}

	m.logger.Debug("directory changes detected, reloading", "directory", m.directoryPath)
	return m.reload()
}

// reload reads all files in the directory and rebuilds the IP lookup helper
func (m *sharedIpLookupMonitor) reload() error {
	// Create empty helper and insert CIDRs directly to save memory
	newHelper := NewEmptyIpLookupHelper()
	totalBlocks := 0

	// Add static blocks first
	for _, cidr := range m.staticBlocks {
		if err := newHelper.AddCIDR(cidr); err != nil {
			return fmt.Errorf("failed to add static CIDR block %q: %w", cidr, err)
		}
		totalBlocks++
	}

	// Add blocks from directory if specified
	if m.directoryPath != "" {
		directoryBlocks, fileInfo, err := m.insertBlocksFromDirectory(newHelper)
		if err != nil {
			if os.IsNotExist(err) {
				m.logger.Debug("IP blocks directory does not exist, using only static blocks", "directory", m.directoryPath)
			} else {
				return fmt.Errorf("failed to read blocks from directory %s: %w", m.directoryPath, err)
			}
		} else {
			totalBlocks += directoryBlocks
			m.lastFileInfo = fileInfo
			m.logger.Debug("loaded IP blocks from directory", "directory", m.directoryPath, "files", len(fileInfo), "blocks", directoryBlocks)
		}
	}

	// Simple assignment - no synchronization needed!
	// Worst case: some requests use old helper for a few milliseconds, totally harmless
	m.helper = newHelper

	m.logger.Debug("reloaded IP blocks", "total_count", totalBlocks, "static_count", len(m.staticBlocks))
	return nil
}

// getDirectoryFileInfo returns modification times for all .txt files in the directory
func (m *sharedIpLookupMonitor) getDirectoryFileInfo() (map[string]time.Time, error) {
	fileInfo := make(map[string]time.Time)

	err := filepath.Walk(m.directoryPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-.txt files
		if info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".txt") {
			return nil
		}

		fileInfo[path] = info.ModTime()
		return nil
	})

	return fileInfo, err
}

// insertBlocksFromDirectory reads CIDR blocks from all .txt files in the directory and inserts them into the helper
func (m *sharedIpLookupMonitor) insertBlocksFromDirectory(helper *IpLookupHelper) (int, map[string]time.Time, error) {
	if _, err := os.Stat(m.directoryPath); err != nil {
		return 0, nil, err
	}

	var totalBlocks int
	fileInfo := make(map[string]time.Time)

	err := filepath.Walk(m.directoryPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			m.logger.Warn("error accessing file during directory scan", "file", path, "error", err)
			return nil // Continue with other files
		}

		// Skip directories and non-.txt files
		if info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".txt") {
			return nil
		}

		// Read blocks from this file
		blocks, err := m.readBlocksFromFile(path)
		if err != nil {
			m.logger.Warn("failed to read blocks from file", "file", path, "error", err)
			return nil // Continue with other files
		}

		for _, cidr := range blocks {
			if err := helper.AddCIDR(cidr); err != nil {
				m.logger.Warn("failed to add CIDR block", "cidr", cidr, "error", err)
			} else {
				totalBlocks++
			}
		}

		fileInfo[path] = info.ModTime()

		m.logger.Debug("loaded blocks from file", "file", path, "count", len(blocks))
		return nil
	})

	if err != nil {
		return 0, nil, err
	}

	return totalBlocks, fileInfo, nil
}

// readBlocksFromFile reads CIDR blocks from a single file, one per line
func (m *sharedIpLookupMonitor) readBlocksFromFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var blocks []string
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Validate CIDR format
		_, _, err := net.ParseCIDR(line)
		if err != nil {
			m.logger.Warn("invalid CIDR block in file", "file", filePath, "line", lineNum, "cidr", line, "error", err)
			continue
		}

		blocks = append(blocks, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return blocks, nil
}

// ForceRefresh forces an immediate refresh of the directory contents (for testing)
func (m *sharedIpLookupMonitor) ForceRefresh() error {
	return m.checkAndReload()
}

// GetSharedMonitorCount returns the number of active shared monitors
func GetSharedMonitorCount() int {
	monitorMutex.RLock()
	defer monitorMutex.RUnlock()
	return len(monitors)
}

// CleanupMonitors closes all shared monitors (for testing/shutdown)
func CleanupMonitors() {
	monitorMutex.Lock()
	defer monitorMutex.Unlock()

	for key, monitor := range monitors {
		if monitor.ticker != nil {
			close(monitor.stopChan)
			monitor.ticker.Stop()
		}
		delete(monitors, key)
	}
}

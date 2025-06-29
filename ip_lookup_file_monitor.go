package traefik_geoblock

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"log/slog"
)

// IpLookupFileMonitor is a simple wrapper that reads IP blocks from a directory once
type IpLookupFileMonitor struct {
	helper *IpLookupHelper
	logger *slog.Logger
}

// NewIpLookupFileMonitor creates a new IP lookup monitor by reading all .txt files in the directory once
func NewIpLookupFileMonitor(cidrBlocks []string, directoryPath string, logger *slog.Logger) (*IpLookupFileMonitor, error) {
	// Create empty helper and insert CIDRs directly to save memory
	helper := NewEmptyIpLookupHelper()

	// Add static blocks first
	for _, cidr := range cidrBlocks {
		if err := helper.AddCIDR(cidr); err != nil {
			return nil, fmt.Errorf("failed to add static CIDR block %q: %w", cidr, err)
		}
	}
	staticCount := helper.Count()

	// Add blocks from directory if specified
	if directoryPath != "" {
		directoryBlocks, err := insertBlocksFromDirectory(helper, directoryPath, logger)
		if err != nil {
			if os.IsNotExist(err) {
				logger.Debug("IP blocks directory does not exist, using only static blocks", "directory", directoryPath)
			} else {
				return nil, fmt.Errorf("failed to read blocks from directory %s: %w", directoryPath, err)
			}
		} else {
			logger.Debug("loaded IP blocks from directory", "directory", directoryPath, "blocks", directoryBlocks)
		}
	}

	logger.Debug("loaded IP blocks", "total_count", helper.Count(), "static_count", staticCount, "directory_count", helper.Count()-staticCount)

	return &IpLookupFileMonitor{
		helper: helper,
		logger: logger,
	}, nil
}

// IsContained checks if an IP is contained in any of the CIDR blocks
func (m *IpLookupFileMonitor) IsContained(ipAddr net.IP) (bool, int, error) {
	return m.helper.IsContained(ipAddr)
}

// insertBlocksFromDirectory reads CIDR blocks from all .txt files in the directory and inserts them into the helper
func insertBlocksFromDirectory(helper *IpLookupHelper, directoryPath string, logger *slog.Logger) (int, error) {
	if _, err := os.Stat(directoryPath); err != nil {
		return 0, err
	}

	// Track count before adding directory blocks
	countBefore := helper.Count()

	err := filepath.Walk(directoryPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logger.Warn("error accessing file during directory scan", "file", path, "error", err)
			return nil // Continue with other files
		}

		// Skip directories and non-.txt files
		if info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".txt") {
			return nil
		}

		// Read blocks from this file
		blocks, err := readBlocksFromFile(path, logger)
		if err != nil {
			logger.Warn("failed to read blocks from file", "file", path, "error", err)
			return nil // Continue with other files
		}

		successfullyAdded := 0
		for _, cidr := range blocks {
			if err := helper.AddCIDR(cidr); err != nil {
				logger.Warn("failed to add CIDR block", "cidr", cidr, "error", err)
			} else {
				successfullyAdded++
			}
		}

		logger.Debug("loaded blocks from file", "file", path, "attempted", len(blocks), "successful", successfullyAdded)
		return nil
	})

	if err != nil {
		return 0, err
	}

	// Return the actual number of blocks added (helper knows the truth)
	return helper.Count() - countBefore, nil
}

// readBlocksFromFile reads CIDR blocks from a single file, one per line
func readBlocksFromFile(filePath string, logger *slog.Logger) ([]string, error) {
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
			logger.Warn("invalid CIDR block in file", "file", filePath, "line", lineNum, "cidr", line, "error", err)
			continue
		}

		blocks = append(blocks, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return blocks, nil
}

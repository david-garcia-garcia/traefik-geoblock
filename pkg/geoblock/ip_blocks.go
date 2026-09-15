package geoblock

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"log/slog"

	"github.com/david-garcia-garcia/traefik-middleware-utilities/iplookup"
)

// loadIPBlockHelper stores static CIDRs then every .txt file under directoryPath.
// A missing directory is debug, not fatal.
func loadIPBlockHelper(cidrBlocks []string, directoryPath string, logger *slog.Logger) (*iplookup.Helper, error) {
	helper := iplookup.New()

	for _, cidr := range cidrBlocks {
		if err := helper.AddCIDR(cidr, ""); err != nil {
			return nil, fmt.Errorf("failed to add static CIDR block %q: %w", cidr, err)
		}
	}
	staticCount := helper.Count()

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
	return helper, nil
}

func insertBlocksFromDirectory(helper *iplookup.Helper, directoryPath string, logger *slog.Logger) (int, error) {
	if _, err := os.Stat(directoryPath); err != nil {
		return 0, err
	}

	countBefore := helper.Count()

	err := filepath.Walk(directoryPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logger.Warn("error accessing file during directory scan", "file", path, "error", err)
			return nil
		}

		if info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".txt") {
			return nil
		}

		blocks, err := readBlocksFromFile(path, logger)
		if err != nil {
			logger.Warn("failed to read blocks from file", "file", path, "error", err)
			return nil
		}

		successfullyAdded := 0
		for _, cidr := range blocks {
			if err := helper.AddCIDR(cidr, ""); err != nil {
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

	return helper.Count() - countBefore, nil
}

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
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if _, _, err := net.ParseCIDR(line); err != nil {
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

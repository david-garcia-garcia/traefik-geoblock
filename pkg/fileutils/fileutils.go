package fileutils

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// Utils provides utility functions for file operations
type Utils struct{}

// New creates a new Utils instance
func New() *Utils {
	return &Utils{}
}

// Exists checks if a file or directory exists
func (fu *Utils) Exists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

// ExistsAndIsFile checks if a file exists and is not a directory
func (fu *Utils) ExistsAndIsFile(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// ExistsAndIsDir checks if a file exists and is a directory
func (fu *Utils) ExistsAndIsDir(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

// Copy copies a file from src to dst.
// If dst exists, it will be overwritten only if overwrite is true.
func (fu *Utils) Copy(src string, dst string, overwrite bool) error {
	// Check if source file exists
	if !fu.ExistsAndIsFile(src) {
		return fmt.Errorf("source file does not exist: %s", src)
	}

	// Check if destination exists and handle according to overwrite parameter
	if fu.ExistsAndIsFile(dst) {
		// File exists - return error if overwrite is false
		if !overwrite {
			return fmt.Errorf("destination file already exists: %s", dst)
		}
	}

	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	// Create or truncate the destination file with same permissions as source
	destFile, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	// Copy the contents
	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Ensure all data is written to disk
	if err := destFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync destination file: %w", err)
	}

	return nil
}

// Search looks for a file in the filesystem, handling both direct paths and directory searches.
// If basePathOrFile is a direct path to an existing file, that path is returned.
// If basePathOrFile is a directory, it recursively searches for defaultFile within that directory.
//
// Parameters:
//   - basePathOrFile: Either a direct file path or directory to search in
//   - defaultFile: Filename to search for if basePathOrFile is a directory
//   - logger: Logger for error reporting
//
// Returns:
//   - The path to the found file, or an error if the file is not found
//
// The function will return an error if the file cannot be found after trying all fallback options.
func (fu *Utils) Search(basePathOrFile string, defaultFile string, logger *slog.Logger) (string, error) {

	// Check if we received a file path and if it exists return that
	if basePathOrFile != "" && fu.ExistsAndIsFile(basePathOrFile) {
		return basePathOrFile, nil
	}

	// If we are going to perform a search, defaultFileName must be provided
	if defaultFile == "" {
		return "", fmt.Errorf("fileutils [Search]: defaultFile must be provided when performing a search")
	}

	// The basePathOrFile must be a directory
	if fu.ExistsAndIsDir(basePathOrFile) {
		logger.Debug("fileutils [Search]: basePathOrFile is a directory, searching recursively for file.", "basePathOrFile", basePathOrFile)
	} else {
		// Try to fallback to the environment variable path
		envPath := os.Getenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH")
		if envPath != "" {
			if fu.ExistsAndIsDir(envPath) {
				logger.Debug("fileutils [Search]: using environment variable path TRAEFIK_PLUGIN_GEOBLOCK_PATH for file search.", "envPath", envPath)
				basePathOrFile = envPath
			} else {
				logger.Error("fileutils [Search]: TRAEFIK_PLUGIN_GEOBLOCK_PATH is not a directory", "envPath", envPath)
				return "", fmt.Errorf("fileutils [Search]: TRAEFIK_PLUGIN_GEOBLOCK_PATH is not a directory")
			}
		} else {
			return "", fmt.Errorf("fileutils [Search]: TRAEFIK_PLUGIN_GEOBLOCK_PATH not provided and basePathOrFile is not a directory or does not exist")
		}
	}

	// Try to search recursively in the provided directory
	originalPath := basePathOrFile
	foundPath := ""
	err := filepath.Walk(basePathOrFile, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors and continue walking
		}
		if !info.IsDir() {
			if filepath.Base(path) == defaultFile {
				foundPath = path        // Update foundPath with the found path
				return filepath.SkipAll // Stop walking once found
			}
		}
		return nil
	})

	if err != nil {
		// Log error but continue with fallback
		logger.Debug("error searching for file in specified path", "error", err, "path", originalPath)
	}

	// If found in the specified path, return it
	if foundPath != "" && fu.ExistsAndIsFile(foundPath) {
		return foundPath, nil
	}

	// No file found anywhere - return error
	logger.Error("could not find file", "file", defaultFile, "originalPath", originalPath, "envFallbackChecked", true)
	return "", fmt.Errorf("file not found: %s (searched in %s and TRAEFIK_PLUGIN_GEOBLOCK_PATH)", defaultFile, originalPath)
}

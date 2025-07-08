package traefik_geoblock

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// FileUtils provides utility functions for file operations
type FileUtils struct{}

// NewFileUtils creates a new FileUtils instance
func NewFileUtils() *FileUtils {
	return &FileUtils{}
}

// ExistsAndIsFile checks if a file exists and is not a directory
func (fu *FileUtils) Exists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

// ExistsAndIsFile checks if a file exists and is not a directory
func (fu *FileUtils) ExistsAndIsFile(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// ExistsAndIsDir checks if a file exists and is a directory
func (fu *FileUtils) ExistsAndIsDir(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

// Copy copies a file from src to dst.
// If dst exists, it will be overwritten only if overwrite is true.
func (fu *FileUtils) Copy(src string, dst string, overwrite bool) error {
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
func (fu *FileUtils) Search(basePathOrFile string, defaultFile string, logger *slog.Logger) (string, error) {

	// Check if we received a file path and if it exists return that
	if basePathOrFile != "" && fu.ExistsAndIsFile(basePathOrFile) {
		return basePathOrFile, nil
	}

	// If we are going to perform a search, defaultFileName must be provided
	if defaultFile == "" {
		return "", fmt.Errorf("database_factory [Search]: defaultFile must be provided when performing a search")
	}

	// The basePathOrFile must be a directory
	if fu.ExistsAndIsDir(basePathOrFile) {
		logger.Debug("database_factory [Search]: basePathOrFile is a directory, searching recursively for file.", "basePathOrFile", basePathOrFile)
	} else {
		// Try to fallback to the environment variable path
		envPath := os.Getenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH")
		if envPath != "" {
			if fu.ExistsAndIsDir(envPath) {
				logger.Debug("database_factory [Search]: using environment variable path TRAEFIK_PLUGIN_GEOBLOCK_PATH for file search.", "envPath", envPath)
				basePathOrFile = envPath
			} else {
				logger.Error("database_factory [Search]: TRAEFIK_PLUGIN_GEOBLOCK_PATH is not a directory", "envPath", envPath)
				return "", fmt.Errorf("database_factory [Search]: TRAEFIK_PLUGIN_GEOBLOCK_PATH is not a directory")
			}
		} else {
			return "", fmt.Errorf("database_factory [Search]: TRAEFIK_PLUGIN_GEOBLOCK_PATH not provided and basePathOrFile is not a directory or does not exist")
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

// Global instance for backward compatibility and convenience
var fileUtils = NewFileUtils()

// Convenience functions that delegate to the global instance
// These maintain backward compatibility with existing code

// fileExists is a convenience function that uses the global FileUtils instance
func fileExists(filename string) bool {
	return fileUtils.ExistsAndIsFile(filename)
}

// copyFile is a convenience function that uses the global FileUtils instance
func copyFile(src string, dst string, overwrite bool) error {
	return fileUtils.Copy(src, dst, overwrite)
}

// searchFile is a convenience function that uses the global FileUtils instance
func searchFile(baseFile string, defaultFile string, logger *slog.Logger) string {
	result, err := fileUtils.Search(baseFile, defaultFile, logger)
	if err != nil {
		// For backward compatibility, return empty string on error
		return ""
	}
	return result
}

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

// Exists checks if a file exists and is not a directory
func (fu *FileUtils) Exists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// Copy copies a file from src to dst.
// If dst exists, it will be overwritten only if overwrite is true.
func (fu *FileUtils) Copy(src string, dst string, overwrite bool) error {
	// Check if source file exists
	if !fu.Exists(src) {
		return fmt.Errorf("source file does not exist: %s", src)
	}

	// Check if destination exists and handle according to overwrite parameter
	if fu.Exists(dst) {
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
// If baseFile is a direct path to an existing file, that path is returned.
// If baseFile is a directory, it recursively searches for defaultFile within that directory.
//
// Parameters:
//   - baseFile: Either a direct file path or directory to search in
//   - defaultFile: Filename to search for if baseFile is a directory
//   - logger: Logger for error reporting
//
// Returns:
//   - The path to the found file, or the original baseFile path if not found
//
// The function will log errors if the file cannot be found or if there are issues during the search,
// but will not fail - it always returns a path.
func (fu *FileUtils) Search(baseFile string, defaultFile string, logger *slog.Logger) string {
	// Return early if baseFile is empty
	if baseFile == "" {
		return defaultFile
	}

	// Check if the file exists at the specified path
	if fu.Exists(baseFile) {
		return baseFile
	}

	err := filepath.Walk(baseFile, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors and continue walking
		}
		if !info.IsDir() {
			if filepath.Base(path) == defaultFile {
				baseFile = path         // Update baseFile with the found path
				return filepath.SkipAll // Stop walking once found
			}
		}
		return nil
	})

	if err != nil {
		// Log error but continue with original path
		logger.Error("error searching for file", "error", err)
	}

	if !fu.Exists(baseFile) {
		logger.Error("could not find file", "file", defaultFile, "path", baseFile)
	}

	return baseFile // Return found path or original path if not found
}

// Global instance for backward compatibility and convenience
var fileUtils = NewFileUtils()

// Convenience functions that delegate to the global instance
// These maintain backward compatibility with existing code

// fileExists is a convenience function that uses the global FileUtils instance
func fileExists(filename string) bool {
	return fileUtils.Exists(filename)
}

// copyFile is a convenience function that uses the global FileUtils instance
func copyFile(src string, dst string, overwrite bool) error {
	return fileUtils.Copy(src, dst, overwrite)
}

// searchFile is a convenience function that uses the global FileUtils instance
func searchFile(baseFile string, defaultFile string, logger *slog.Logger) string {
	return fileUtils.Search(baseFile, defaultFile, logger)
}

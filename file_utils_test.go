package traefik_geoblock

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileUtils_Exists(t *testing.T) {
	fu := NewFileUtils()

	t.Run("existing file or path", func(t *testing.T) {
		// Create a temporary file
		tmpFile, err := os.CreateTemp("", "test-*.txt")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		if !fu.Exists(tmpFile.Name()) {
			t.Error("expected file to exist")
		}
	})

	t.Run("non-existing file", func(t *testing.T) {
		if fu.Exists("/non/existing/file.txt") {
			t.Error("expected file not to exist")
		}
	})

	t.Run("directory should exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		if !fu.Exists(tmpDir) {
			t.Error("expected directory to exist")
		}
	})
}

func TestFileUtils_ExistsAndIsFile(t *testing.T) {
	fu := NewFileUtils()

	t.Run("existing file", func(t *testing.T) {
		// Create a temporary file
		tmpFile, err := os.CreateTemp("", "test-*.txt")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		if !fu.ExistsAndIsFile(tmpFile.Name()) {
			t.Error("expected file to exist")
		}
	})

	t.Run("non-existing file", func(t *testing.T) {
		if fu.ExistsAndIsFile("/non/existing/file.txt") {
			t.Error("expected file not to exist")
		}
	})

	t.Run("directory should not be considered as file", func(t *testing.T) {
		tmpDir := t.TempDir()
		if fu.ExistsAndIsFile(tmpDir) {
			t.Error("expected directory not to be considered as existing file")
		}
	})
}

func TestFileUtils_Copy(t *testing.T) {
	fu := NewFileUtils()

	t.Run("successful copy", func(t *testing.T) {
		// Create source file
		srcFile, err := os.CreateTemp("", "src-*.txt")
		if err != nil {
			t.Fatalf("failed to create source file: %v", err)
		}
		defer os.Remove(srcFile.Name())

		testContent := "test content"
		if _, err := srcFile.WriteString(testContent); err != nil {
			t.Fatalf("failed to write to source file: %v", err)
		}
		srcFile.Close()

		// Create destination path
		dstFile := filepath.Join(t.TempDir(), "dst.txt")

		// Copy the file
		if err := fu.Copy(srcFile.Name(), dstFile, false); err != nil {
			t.Fatalf("copy failed: %v", err)
		}

		// Verify the copy
		if !fu.ExistsAndIsFile(dstFile) {
			t.Error("destination file does not exist")
		}

		content, err := os.ReadFile(dstFile)
		if err != nil {
			t.Fatalf("failed to read destination file: %v", err)
		}

		if string(content) != testContent {
			t.Errorf("content mismatch: expected %q, got %q", testContent, string(content))
		}
	})

	t.Run("copy with overwrite", func(t *testing.T) {
		// Create source file
		srcFile, err := os.CreateTemp("", "src-*.txt")
		if err != nil {
			t.Fatalf("failed to create source file: %v", err)
		}
		defer os.Remove(srcFile.Name())

		newContent := "new content"
		if _, err := srcFile.WriteString(newContent); err != nil {
			t.Fatalf("failed to write to source file: %v", err)
		}
		srcFile.Close()

		// Create destination file with different content
		dstFile, err := os.CreateTemp("", "dst-*.txt")
		if err != nil {
			t.Fatalf("failed to create destination file: %v", err)
		}
		defer os.Remove(dstFile.Name())

		if _, err := dstFile.WriteString("old content"); err != nil {
			t.Fatalf("failed to write to destination file: %v", err)
		}
		dstFile.Close()

		// Copy with overwrite
		if err := fu.Copy(srcFile.Name(), dstFile.Name(), true); err != nil {
			t.Fatalf("copy with overwrite failed: %v", err)
		}

		// Verify the content was replaced
		content, err := os.ReadFile(dstFile.Name())
		if err != nil {
			t.Fatalf("failed to read destination file: %v", err)
		}

		if string(content) != newContent {
			t.Errorf("content mismatch: expected %q, got %q", newContent, string(content))
		}
	})

	t.Run("copy without overwrite should fail", func(t *testing.T) {
		// Create source file
		srcFile, err := os.CreateTemp("", "src-*.txt")
		if err != nil {
			t.Fatalf("failed to create source file: %v", err)
		}
		defer os.Remove(srcFile.Name())
		srcFile.Close()

		// Create destination file
		dstFile, err := os.CreateTemp("", "dst-*.txt")
		if err != nil {
			t.Fatalf("failed to create destination file: %v", err)
		}
		defer os.Remove(dstFile.Name())
		dstFile.Close()

		// Copy without overwrite should fail
		err = fu.Copy(srcFile.Name(), dstFile.Name(), false)
		if err == nil {
			t.Error("expected copy to fail when destination exists and overwrite is false")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("expected error about file already existing, got: %v", err)
		}
	})

	t.Run("copy non-existing source should fail", func(t *testing.T) {
		dstFile := filepath.Join(t.TempDir(), "dst.txt")
		err := fu.Copy("/non/existing/file.txt", dstFile, false)
		if err == nil {
			t.Error("expected copy to fail when source doesn't exist")
		}
		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("expected error about source not existing, got: %v", err)
		}
	})
}

func TestFileUtils_Search(t *testing.T) {
	fu := NewFileUtils()
	logger := slog.Default()

	t.Run("direct file path exists", func(t *testing.T) {
		// Create a test file
		tmpFile, err := os.CreateTemp("", "test-*.txt")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		result, err := fu.Search(tmpFile.Name(), "default.txt", logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != tmpFile.Name() {
			t.Errorf("expected %q, got %q", tmpFile.Name(), result)
		}
	})

	t.Run("search in directory", func(t *testing.T) {
		// Create a test directory structure
		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "subdir")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("failed to create subdirectory: %v", err)
		}

		// Create target file in subdirectory
		targetFile := filepath.Join(subDir, "target.bin")
		if err := os.WriteFile(targetFile, []byte("test"), 0600); err != nil {
			t.Fatalf("failed to create target file: %v", err)
		}

		result, err := fu.Search(tmpDir, "target.bin", logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != targetFile {
			t.Errorf("expected %q, got %q", targetFile, result)
		}
	})

	t.Run("file not found returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		nonExistentPath := filepath.Join(tmpDir, "nonexistent.txt")

		_, err := fu.Search(nonExistentPath, "default.txt", logger)
		if err == nil {
			t.Error("expected error for non-existent file")
		}
	})

	t.Run("empty base path returns error without env var", func(t *testing.T) {
		// Temporarily unset the environment variable to test basic behavior
		originalEnv := os.Getenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH")
		os.Unsetenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH")
		defer os.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", originalEnv)

		_, err := fu.Search("", "default.txt", logger)
		if err == nil {
			t.Error("expected error when no base path and no env var")
		}
	})

	t.Run("environment variable fallback", func(t *testing.T) {
		// Create a test directory for the environment variable
		envDir := t.TempDir()

		// Create a test file in the environment directory
		envFileName := "env-test.txt"
		envFilePath := filepath.Join(envDir, envFileName)
		if err := os.WriteFile(envFilePath, []byte("env content"), 0600); err != nil {
			t.Fatalf("failed to create env test file: %v", err)
		}

		// Set the environment variable
		originalEnv := os.Getenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH")
		os.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", envDir)
		defer os.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", originalEnv) // Restore original value

		// Test with empty base path (should use environment variable)
		result, err := fu.Search("", envFileName, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != envFilePath {
			t.Errorf("expected environment variable fallback %q, got %q", envFilePath, result)
		}

		// Test with non-existent base path (should fallback to environment variable)
		nonExistentPath := filepath.Join(t.TempDir(), "nonexistent", "path.txt")
		result2, err := fu.Search(nonExistentPath, envFileName, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result2 != envFilePath {
			t.Errorf("expected environment variable fallback %q, got %q", envFilePath, result2)
		}
	})

	t.Run("environment variable with empty base path", func(t *testing.T) {
		// Create a test directory for the environment variable
		envDir := t.TempDir()

		// Create test file with specific content
		testFileName := "env-only-test.txt"
		envFilePath := filepath.Join(envDir, testFileName)

		if err := os.WriteFile(envFilePath, []byte("env content"), 0600); err != nil {
			t.Fatalf("failed to create env test file: %v", err)
		}

		// Set the environment variable
		originalEnv := os.Getenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH")
		os.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", envDir)
		defer os.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", originalEnv) // Restore original value

		// Should use environment variable with empty base path
		result, err := fu.Search("", testFileName, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != envFilePath {
			t.Errorf("expected environment variable path %q, got %q", envFilePath, result)
		}

		// Verify content to ensure we got the right file
		content, err := os.ReadFile(result)
		if err != nil {
			t.Fatalf("failed to read result file: %v", err)
		}
		if string(content) != "env content" {
			t.Errorf("expected content 'env content', got %q", string(content))
		}
	})

	t.Run("environment variable fallback not found", func(t *testing.T) {
		// Create a test directory for the environment variable
		envDir := t.TempDir()

		// Set the environment variable but don't create the file
		originalEnv := os.Getenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH")
		os.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", envDir)
		defer os.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", originalEnv) // Restore original value

		nonExistentFileName := "nonexistent-env-file.txt"

		// Should try environment variable but return error since file not found
		_, err := fu.Search("", nonExistentFileName, logger)

		// Should get an error since file not found anywhere
		if err == nil {
			t.Error("expected error when file not found in environment variable path")
		}
	})
}

func TestFileUtils_BackwardCompatibility(t *testing.T) {
	// Test that the global convenience functions work
	t.Run("fileExists function", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "test-*.txt")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		if !fileExists(tmpFile.Name()) {
			t.Error("fileExists should return true for existing file")
		}

		if fileExists("/non/existing/file") {
			t.Error("fileExists should return false for non-existing file")
		}
	})

	t.Run("copyFile function", func(t *testing.T) {
		// Create source file
		srcFile, err := os.CreateTemp("", "src-*.txt")
		if err != nil {
			t.Fatalf("failed to create source file: %v", err)
		}
		defer os.Remove(srcFile.Name())

		testContent := "test content"
		if _, err := srcFile.WriteString(testContent); err != nil {
			t.Fatalf("failed to write to source file: %v", err)
		}
		srcFile.Close()

		// Copy using global function
		dstFile := filepath.Join(t.TempDir(), "dst.txt")
		if err := copyFile(srcFile.Name(), dstFile, false); err != nil {
			t.Fatalf("copyFile failed: %v", err)
		}

		// Verify
		if !fileExists(dstFile) {
			t.Error("destination file should exist")
		}
	})

	t.Run("searchFile function", func(t *testing.T) {
		logger := slog.Default()

		// Create test file
		tmpFile, err := os.CreateTemp("", "test-*.txt")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		result := searchFile(tmpFile.Name(), "default.txt", logger)
		if result != tmpFile.Name() {
			t.Errorf("expected %q, got %q", tmpFile.Name(), result)
		}
	})
}

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

	t.Run("existing file", func(t *testing.T) {
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

	t.Run("directory should not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		if fu.Exists(tmpDir) {
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
		if !fu.Exists(dstFile) {
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

		result := fu.Search(tmpFile.Name(), "default.txt", logger)
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

		result := fu.Search(tmpDir, "target.bin", logger)
		if result != targetFile {
			t.Errorf("expected %q, got %q", targetFile, result)
		}
	})

	t.Run("file not found returns original path", func(t *testing.T) {
		tmpDir := t.TempDir()
		nonExistentPath := filepath.Join(tmpDir, "nonexistent.txt")

		result := fu.Search(nonExistentPath, "default.txt", logger)
		if result != nonExistentPath {
			t.Errorf("expected %q, got %q", nonExistentPath, result)
		}
	})

	t.Run("empty base path returns default", func(t *testing.T) {
		result := fu.Search("", "default.txt", logger)
		if result != "default.txt" {
			t.Errorf("expected %q, got %q", "default.txt", result)
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

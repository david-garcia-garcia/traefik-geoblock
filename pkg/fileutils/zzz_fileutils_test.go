package fileutils

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileUtils_ExistsPath(t *testing.T) {
	fu := NewFileUtils()

	t.Run("existing file or path", func(t *testing.T) {
		// Create a temporary file
		tmpFile, err := os.CreateTemp("", "test-*.txt")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		if !fu.ExistsPath(tmpFile.Name()) {
			t.Error("expected file to exist")
		}
	})

	t.Run("non-existing file", func(t *testing.T) {
		if fu.ExistsPath("/non/existing/file.txt") {
			t.Error("expected file not to exist")
		}
	})

	t.Run("directory should exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		if !fu.ExistsPath(tmpDir) {
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

	t.Run("exact seeds path under plugin root env", func(t *testing.T) {
		envDir := t.TempDir()
		seedsDir := filepath.Join(envDir, "seeds")
		if err := os.MkdirAll(seedsDir, 0755); err != nil {
			t.Fatal(err)
		}
		targetFile := filepath.Join(seedsDir, "target.bin")
		if err := os.WriteFile(targetFile, []byte("test"), 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv(PluginPathEnv, envDir)
		result, err := fu.Search("", "target.bin", logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != targetFile {
			t.Errorf("expected %q, got %q", targetFile, result)
		}
	})

	t.Run("does not walk an arbitrary subdirectory", func(t *testing.T) {
		envDir := t.TempDir()
		subDir := filepath.Join(envDir, "subdir")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(subDir, "target.bin"), []byte("test"), 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv(PluginPathEnv, envDir)
		if _, err := fu.Search("", "target.bin", logger); err == nil {
			t.Fatal("walk of subdir must not find the file")
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
		var buf bytes.Buffer
		warnLogger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		t.Setenv(PluginPathEnv, "")
		_, err := fu.Search("", "default.txt", warnLogger)
		if err == nil {
			t.Error("expected error when no base path and no env var")
		}
		if !strings.Contains(err.Error(), "must be set to the plugin root") {
			t.Errorf("got %v", err)
		}
		if !strings.Contains(buf.String(), "must be set to the plugin root") {
			t.Errorf("unset miss should WARN: %s", buf.String())
		}
		if strings.Contains(strings.ToLower(buf.String()), "level=error") {
			t.Errorf("Search must not Error: %s", buf.String())
		}
	})

	t.Run("env path is a file not a directory", func(t *testing.T) {
		envFile := filepath.Join(t.TempDir(), "not-a-dir")
		if err := os.WriteFile(envFile, []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		warnLogger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		t.Setenv(PluginPathEnv, envFile)
		_, err := fu.Search("", "default.txt", warnLogger)
		if err == nil {
			t.Fatal("expected error when plugin-root env is a file")
		}
		if !strings.Contains(err.Error(), "matched no plugin directory") {
			t.Errorf("got %v", err)
		}
		if !strings.Contains(buf.String(), "matched no plugin directory") {
			t.Errorf("exclusive miss should WARN: %s", buf.String())
		}
	})

	t.Run("env glob locates gop plugin directory", func(t *testing.T) {
		layout := t.TempDir()
		pluginDir := filepath.Join(layout, "gop-12345", "src", "github.com", "david-garcia-garcia", "traefik-geoblock")
		seedsDir := filepath.Join(pluginDir, "seeds")
		if err := os.MkdirAll(seedsDir, 0755); err != nil {
			t.Fatal(err)
		}
		targetFile := filepath.Join(seedsDir, "target.bin")
		if err := os.WriteFile(targetFile, []byte("test"), 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv(PluginPathEnv, filepath.Join(layout, "*", "src", "github.com", "david-garcia-garcia", "traefik-geoblock"))
		result, err := fu.Search("", "target.bin", logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != targetFile {
			t.Errorf("expected %q, got %q", targetFile, result)
		}
	})

	t.Run("exact directory skips glob", func(t *testing.T) {
		base := t.TempDir()
		exactDir := filepath.Join(base, "plugin")
		otherDir := filepath.Join(base, "plugin-other")
		if err := os.MkdirAll(filepath.Join(exactDir, "seeds"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(otherDir, "seeds"), 0755); err != nil {
			t.Fatal(err)
		}
		exactFile := filepath.Join(exactDir, "seeds", "target.bin")
		otherFile := filepath.Join(otherDir, "seeds", "target.bin")
		if err := os.WriteFile(exactFile, []byte("exact"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(otherFile, []byte("other"), 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv(PluginPathEnv, exactDir)
		result, err := fu.Search("", "target.bin", logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != exactFile {
			t.Errorf("expected exact directory %q, got %q", exactFile, result)
		}
	})

	t.Run("env glob with zero matches does not fall through", func(t *testing.T) {
		hidden := t.TempDir()
		if err := os.MkdirAll(filepath.Join(hidden, "seeds"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(hidden, "seeds", "target.bin"), []byte("hidden"), 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv(PluginPathEnv, filepath.Join(t.TempDir(), "no-such", "*", "plugin"))
		if _, err := fu.Search("", "target.bin", logger); err == nil {
			t.Fatal("exclusive glob miss must not find another directory")
		}
	})

	t.Run("several glob matches prefer seeds defaultFile", func(t *testing.T) {
		layout := t.TempDir()
		dirA := filepath.Join(layout, "a", "src", "github.com", "david-garcia-garcia", "traefik-geoblock")
		dirB := filepath.Join(layout, "b", "src", "github.com", "david-garcia-garcia", "traefik-geoblock")
		if err := os.MkdirAll(dirA, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dirB, "seeds"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dirA, "target.bin"), []byte("root-a"), 0600); err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(dirB, "seeds", "target.bin")
		if err := os.WriteFile(want, []byte("seeds-b"), 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv(PluginPathEnv, filepath.Join(layout, "*", "src", "github.com", "david-garcia-garcia", "traefik-geoblock"))
		result, err := fu.Search("", "target.bin", logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != want {
			t.Errorf("expected seeded directory %q, got %q", want, result)
		}
	})

	t.Run("file miss under chosen directory is debug", func(t *testing.T) {
		var buf bytes.Buffer
		debugLogger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		t.Setenv(PluginPathEnv, t.TempDir())
		_, err := fu.Search("", "missing.bin", debugLogger)
		if err == nil {
			t.Fatal("expected file miss")
		}
		out := buf.String()
		if !strings.Contains(out, "bundled file not found") {
			t.Errorf("file miss should DEBUG: %s", out)
		}
		if strings.Contains(strings.ToLower(out), "level=error") {
			t.Errorf("Search must not Error: %s", out)
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

		if !Exists(tmpFile.Name()) {
			t.Error("Exists should return true for existing file")
		}

		if Exists("/non/existing/file") {
			t.Error("Exists should return false for non-existing file")
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
		if err := Copy(srcFile.Name(), dstFile, false); err != nil {
			t.Fatalf("Copy failed: %v", err)
		}

		// Verify
		if !Exists(dstFile) {
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

		result := Search(tmpFile.Name(), "default.txt", logger)
		if result != tmpFile.Name() {
			t.Errorf("expected %q, got %q", tmpFile.Name(), result)
		}
	})
}

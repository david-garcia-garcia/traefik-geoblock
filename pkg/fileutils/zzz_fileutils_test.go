package fileutils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

	t.Run("direct file path exists", func(t *testing.T) {
		// Create a test file
		tmpFile, err := os.CreateTemp("", "test-*.txt")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		result, err := fu.Search(tmpFile.Name(), "default.txt")
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
		setPluginEnv(t, envDir)
		result, err := fu.Search("", "target.bin")
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
		setPluginEnv(t, envDir)
		if _, err := fu.Search("", "target.bin"); err == nil {
			t.Fatal("walk of subdir must not find the file")
		}
	})

	t.Run("file not found returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		nonExistentPath := filepath.Join(tmpDir, "nonexistent.txt")

		_, err := fu.Search(nonExistentPath, "default.txt")
		if err == nil {
			t.Error("expected error for non-existent file")
		}
	})

	t.Run("env unset and Traefik defaults miss", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "plugins-local", "nothing-here")
		withDefaultPatterns(t, missing)
		setPluginEnv(t, "")
		_, err := fu.Search("", "default.txt")
		if err == nil {
			t.Fatal("expected error when no base path, no env var and no default match")
		}
		if !strings.Contains(err.Error(), PluginPathEnv+" is unset") || !strings.Contains(err.Error(), missing) {
			t.Errorf("error should say the env is unset and list the tried paths: %v", err)
		}
	})

	t.Run("env path is a file not a directory", func(t *testing.T) {
		envFile := filepath.Join(t.TempDir(), "not-a-dir")
		if err := os.WriteFile(envFile, []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
		setPluginEnv(t, envFile)
		_, err := fu.Search("", "default.txt")
		if err == nil {
			t.Fatal("expected error when plugin-root env is a file")
		}
		if !strings.Contains(err.Error(), "matched no plugin directory") {
			t.Errorf("got %v", err)
		}
	})

	t.Run("env glob locates gop plugin directory", func(t *testing.T) {
		layout := t.TempDir()
		pluginDir := filepath.Join(layout, "gop-12345", moduleDirForTest)
		targetFile := writePluginFile(t, pluginDir, "seeds/target.bin")
		setPluginEnv(t, filepath.Join(layout, "*", moduleDirForTest))
		result, err := fu.Search("", "target.bin")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != targetFile {
			t.Errorf("expected %q, got %q", targetFile, result)
		}
	})

	t.Run("exact directory skips glob", func(t *testing.T) {
		base := t.TempDir()
		exactFile := writePluginFile(t, filepath.Join(base, "plugin"), "seeds/target.bin")
		writePluginFile(t, filepath.Join(base, "plugin-other"), "seeds/target.bin")
		setPluginEnv(t, filepath.Join(base, "plugin"))
		result, err := fu.Search("", "target.bin")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != exactFile {
			t.Errorf("expected exact directory %q, got %q", exactFile, result)
		}
	})

	t.Run("env glob with zero matches does not fall through", func(t *testing.T) {
		hidden := t.TempDir()
		writePluginFile(t, hidden, "seeds/target.bin")
		withDefaultPatterns(t, hidden)
		setPluginEnv(t, filepath.Join(t.TempDir(), "no-such", "*", "plugin"))
		if _, err := fu.Search("", "target.bin"); err == nil {
			t.Fatal("exclusive glob miss must not find another directory")
		}
	})

	t.Run("several glob matches use the newest directory for every file", func(t *testing.T) {
		layout := t.TempDir()
		oldDir := filepath.Join(layout, "gop-1", moduleDirForTest)
		newDir := filepath.Join(layout, "gop-2", moduleDirForTest)
		writePluginFile(t, oldDir, "seeds/target.bin")
		writePluginFile(t, oldDir, "geoblockban.html")
		wantSeed := writePluginFile(t, newDir, "seeds/target.bin")
		wantHTML := writePluginFile(t, newDir, "geoblockban.html")
		// gop-1 sorts first but gop-2 is the newer unpack.
		setModTime(t, oldDir, time.Now().Add(-2*time.Hour))
		setModTime(t, newDir, time.Now().Add(-time.Hour))
		setPluginEnv(t, filepath.Join(layout, "*", moduleDirForTest))
		if got, err := fu.Search("", "target.bin"); err != nil || got != wantSeed {
			t.Errorf("seed: got %q, %v; want %q", got, err, wantSeed)
		}
		if got, err := fu.Search("", "geoblockban.html"); err != nil || got != wantHTML {
			t.Errorf("ban HTML: got %q, %v; want %q", got, err, wantHTML)
		}
	})

	t.Run("file missing in the newest directory does not borrow an older one", func(t *testing.T) {
		layout := t.TempDir()
		oldDir := filepath.Join(layout, "gop-1", moduleDirForTest)
		newDir := filepath.Join(layout, "gop-2", moduleDirForTest)
		writePluginFile(t, oldDir, "seeds/target.bin")
		writePluginFile(t, newDir, "geoblockban.html")
		setModTime(t, oldDir, time.Now().Add(-2*time.Hour))
		setModTime(t, newDir, time.Now().Add(-time.Hour))
		setPluginEnv(t, filepath.Join(layout, "*", moduleDirForTest))
		_, err := fu.Search("", "target.bin")
		if err == nil {
			t.Fatal("expected a miss in the chosen directory")
		}
		if !strings.Contains(err.Error(), newDir) {
			t.Errorf("error should name the chosen directory %q: %v", newDir, err)
		}
	})

	t.Run("file miss with env set names the directory and the env", func(t *testing.T) {
		envDir := t.TempDir()
		setPluginEnv(t, envDir)
		_, err := fu.Search("", "missing.bin")
		if err == nil {
			t.Fatal("expected file miss")
		}
		for _, want := range []string{envDir, "probably not the plugin root", filepath.Join(envDir, "seeds", "missing.bin")} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error should contain %q: %v", want, err)
			}
		}
	})

	t.Run("file miss with env unset does not blame the env", func(t *testing.T) {
		pluginDir := filepath.Join(t.TempDir(), "plugins-local", moduleDirForTest)
		writePluginFile(t, pluginDir, "seeds/other.bin")
		withDefaultPatterns(t, pluginDir)
		setPluginEnv(t, "")
		_, err := fu.Search("", "missing.bin")
		if err == nil {
			t.Fatal("expected file miss")
		}
		if !strings.Contains(err.Error(), pluginDir) || !strings.Contains(err.Error(), "default plugin paths") {
			t.Errorf("error should name the directory found from the defaults: %v", err)
		}
		if strings.Contains(err.Error(), "probably not the plugin root") {
			t.Errorf("unset env must not be blamed: %v", err)
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
		result, err := fu.Search("", envFileName)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != envFilePath {
			t.Errorf("expected environment variable fallback %q, got %q", envFilePath, result)
		}

		// Test with non-existent base path (should fallback to environment variable)
		nonExistentPath := filepath.Join(t.TempDir(), "nonexistent", "path.txt")
		result2, err := fu.Search(nonExistentPath, envFileName)
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
		result, err := fu.Search("", testFileName)
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
		_, err := fu.Search("", nonExistentFileName)

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
		// Create test file
		tmpFile, err := os.CreateTemp("", "test-*.txt")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		result := Search(tmpFile.Name(), "default.txt")
		if result != tmpFile.Name() {
			t.Errorf("expected %q, got %q", tmpFile.Name(), result)
		}
	})
}

func TestFileUtils_PluginDirDefaults(t *testing.T) {
	// The real defaults, resolved against the working directory like Traefik does.
	setPluginEnv(t, "")

	t.Run("catalog unpack under plugins-storage", func(t *testing.T) {
		root := chdirTemp(t)
		want := writePluginFile(t, filepath.Join(root, "plugins-storage", "sources", "gop-2132897758", moduleDirForTest), "seeds/target.bin")
		got, err := NewFileUtils().Search("", "target.bin")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !filepath.IsAbs(got) || !sameFile(t, got, want) {
			t.Errorf("got %q, want absolute %q", got, want)
		}
	})

	t.Run("local plugin wins over a catalog unpack", func(t *testing.T) {
		root := chdirTemp(t)
		writePluginFile(t, filepath.Join(root, "plugins-storage", "sources", "gop-1", moduleDirForTest), "seeds/target.bin")
		want := writePluginFile(t, filepath.Join(root, "plugins-local", moduleDirForTest), "seeds/target.bin")
		got, err := NewFileUtils().Search("", "target.bin")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !sameFile(t, got, want) {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

// moduleDirForTest is pluginModuleDir with OS separators.
var moduleDirForTest = filepath.FromSlash(pluginModuleDir)

// withDefaultPatterns swaps defaultPluginDirPatterns for the test.
func withDefaultPatterns(t *testing.T, patterns ...string) {
	t.Helper()
	saved := defaultPluginDirPatterns
	defaultPluginDirPatterns = patterns
	t.Cleanup(func() { defaultPluginDirPatterns = saved })
}

// writePluginFile creates dir/rel (slash-separated) and returns its path.
func writePluginFile(t *testing.T, dir, rel string) string {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(rel), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

// setModTime sets the modification time of path.
func setModTime(t *testing.T, path string, mod time.Time) {
	t.Helper()
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
}

// chdirTemp moves the process into a new temp directory until the test ends.
func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
	return dir
}

// sameFile is true when a and b are the same file on disk (temp dirs may be symlinked or 8.3 names).
func sameFile(t *testing.T, a, b string) bool {
	t.Helper()
	ia, err := os.Stat(a)
	if err != nil {
		t.Fatal(err)
	}
	ib, err := os.Stat(b)
	if err != nil {
		t.Fatal(err)
	}
	return os.SameFile(ia, ib)
}

// setPluginEnv sets PluginPathEnv until the test ends. It uses os.Setenv, not
// t.Setenv, so the code under test also sees it when run with `yaegi test`.
func setPluginEnv(t *testing.T, value string) {
	t.Helper()
	prev, had := os.LookupEnv(PluginPathEnv)
	if err := os.Setenv(PluginPathEnv, value); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(PluginPathEnv, prev)
		} else {
			_ = os.Unsetenv(PluginPathEnv)
		}
	})
}

package fileutils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FileUtils provides utility functions for file operations
type FileUtils struct {
	// mu guards the PluginDir cache.
	mu sync.Mutex
	// pluginDirKey is the PluginPathEnv value (and default patterns) pluginDir was found for.
	pluginDirKey string
	// pluginDir is the last directory PluginDir found.
	pluginDir string
}

// NewFileUtils creates a new FileUtils instance
func NewFileUtils() *FileUtils {
	return &FileUtils{}
}

// ExistsPath reports whether filename exists (file or directory).
func (fu *FileUtils) ExistsPath(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

// ExistsAndIsFile checks if a file exists and is not a directory
func (fu *FileUtils) ExistsAndIsFile(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// ExistsAndIsDir checks if a file exists and is a directory
func (fu *FileUtils) ExistsAndIsDir(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
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

// PluginPathEnv is an exact plugin directory or a glob (local clone or registry unpack).
const PluginPathEnv = "TRAEFIK_PLUGIN_GEOBLOCK_PATH"

const (
	// pluginModuleDir is this module under a Traefik plugin GOPATH.
	pluginModuleDir = "src/github.com/david-garcia-garcia/traefik-geoblock"
	// bundledSeedsDir is the committed seed folder under the plugin directory.
	bundledSeedsDir = "seeds"
)

// defaultPluginDirPatterns is where Traefik puts this plugin when PluginPathEnv is
// unset. Traefik resolves ./plugins-local and ./plugins-storage against its working
// directory ("/" in the official image), so these are relative too: the local plugin
// first, then the catalog unpack (plugins-storage/sources/gop-<random>/src/<module>).
// It is a var so tests do not depend on the host filesystem.
var defaultPluginDirPatterns = []string{
	filepath.FromSlash("plugins-local/" + pluginModuleDir),
	filepath.FromSlash("plugins-storage/sources/*/" + pluginModuleDir),
}

// Search returns an existing file path. It does not walk a directory tree and does
// not log: the error names what was tried, and the caller decides its level.
// If basePathOrFile is an existing file, that path is returned. Otherwise it joins
// PluginDir with seeds/<defaultFile>, then <defaultFile>.
func (fu *FileUtils) Search(basePathOrFile string, defaultFile string) (string, error) {
	// Configured path already names a file.
	if basePathOrFile != "" && fu.ExistsAndIsFile(basePathOrFile) {
		return basePathOrFile, nil
	}
	// No basename: cannot join seeds/ or plugin-root candidates.
	if defaultFile == "" {
		return "", fmt.Errorf("fileutils: defaultFile must be provided when performing a search")
	}
	pluginDir, err := fu.PluginDir()
	if err != nil {
		return "", err
	}
	seedsPath := filepath.Join(pluginDir, bundledSeedsDir, defaultFile)
	if fu.ExistsAndIsFile(seedsPath) {
		return seedsPath, nil
	}
	rootPath := filepath.Join(pluginDir, defaultFile)
	if fu.ExistsAndIsFile(rootPath) {
		return rootPath, nil
	}
	source := "found from Traefik's default plugin paths"
	if env := os.Getenv(PluginPathEnv); env != "" {
		source = fmt.Sprintf("from %s=%q; it is probably not the plugin root", PluginPathEnv, env)
	}
	return "", fmt.Errorf("bundled file %s not found in plugin directory %s (%s; tried %s and %s)",
		defaultFile, pluginDir, source, seedsPath, rootPath)
}

// PluginDir is the one plugin directory every bundled file is read from, so a seed
// and geoblockban.html never come from two unpacks. An exact PluginPathEnv directory
// wins. A set PluginPathEnv is otherwise a glob and never falls through to the
// defaults. Several matches: the most recently modified wins (Traefik unpacks the
// catalog plugin again at each start), then lexicographic order. A found directory
// is cached per PluginPathEnv value and checked again on reuse; a miss is not cached.
func (fu *FileUtils) PluginDir() (string, error) {
	env := os.Getenv(PluginPathEnv)
	key := env + "|" + strings.Join(defaultPluginDirPatterns, "|")
	fu.mu.Lock()
	defer fu.mu.Unlock()
	if fu.pluginDirKey == key && fu.pluginDir != "" && fu.ExistsAndIsDir(fu.pluginDir) {
		return fu.pluginDir, nil
	}
	dir, err := fu.findPluginDir(env)
	if err != nil {
		return "", err
	}
	fu.pluginDirKey = key
	fu.pluginDir = dir
	return dir, nil
}

// findPluginDir is PluginDir without the cache.
func (fu *FileUtils) findPluginDir(env string) (string, error) {
	if env != "" {
		if fu.ExistsAndIsDir(env) {
			return env, nil
		}
		if dir := fu.newestDir(env); dir != "" {
			return dir, nil
		}
		return "", fmt.Errorf("%s=%q matched no plugin directory", PluginPathEnv, env)
	}
	for _, pattern := range defaultPluginDirPatterns {
		if dir := fu.newestDir(pattern); dir != "" {
			if abs, err := filepath.Abs(dir); err == nil {
				return abs, nil
			}
			return dir, nil
		}
	}
	cwd, _ := os.Getwd()
	return "", fmt.Errorf("plugin directory not found: %s is unset and Traefik's default plugin paths matched nothing in %q (tried %s); set %s to the plugin root",
		PluginPathEnv, cwd, strings.Join(defaultPluginDirPatterns, ", "), PluginPathEnv)
}

// newestDir is the most recently modified directory pattern matches, ties broken by
// lexicographic order, or "" when nothing matches.
func (fu *FileUtils) newestDir(pattern string) string {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return ""
	}
	var newest string
	var newestMod time.Time
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil || !info.IsDir() {
			continue
		}
		if newest == "" || info.ModTime().After(newestMod) {
			// Two statements: Yaegi v0.16.1 (Traefik) drops this pair as one tuple assignment.
			newest = match
			newestMod = info.ModTime()
		}
	}
	return newest
}

// Default is the shared FileUtils used by package-level helpers.
var Default = NewFileUtils()

// Exists reports whether filename exists and is a file.
func Exists(filename string) bool {
	return Default.ExistsAndIsFile(filename)
}

// Copy copies src to dst using Default.
func Copy(src string, dst string, overwrite bool) error {
	return Default.Copy(src, dst, overwrite)
}

// Search finds a file using Default. On error it returns "".
func Search(baseFile string, defaultFile string) string {
	result, err := Default.Search(baseFile, defaultFile)
	if err != nil {
		return ""
	}
	return result
}

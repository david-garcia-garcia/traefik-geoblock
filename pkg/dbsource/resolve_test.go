package dbsource

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testLiteMMDB is the committed IPinfo Lite snapshot used as the Resolve fixture.
const testLiteMMDB = "ipinfo_lite.mmdb"

func TestResolve_DatedFileWins(t *testing.T) {
	dir := t.TempDir()
	dated := filepath.Join(dir, time.Now().Format("20060102")+"_lite.mmdb")
	src, err := os.ReadFile(repoFile(t, testLiteMMDB))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dated, src, 0600); err != nil {
		t.Fatal(err)
	}
	seed := filepath.Join(t.TempDir(), "other.mmdb")
	if err := os.WriteFile(seed, src, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Resolve(Config{
		Key:          "lite",
		Dir:          dir,
		DatabaseType: TypeMMDB,
		Path:         seed,
	}, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if got != dated {
		t.Errorf("got %q, want dated %q", got, dated)
	}
}

func TestResolve_CatalogPathFile(t *testing.T) {
	seed := repoFile(t, testLiteMMDB)
	got, err := Resolve(Config{Path: seed, DefaultFileName: "nope.mmdb"}, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if got != seed {
		t.Errorf("got %q, want %q", got, seed)
	}
}

func TestResolve_EmptyPathFindsEnvDefault(t *testing.T) {
	pluginRoot := filepath.Join(filepath.Dir(repoFile(t, testLiteMMDB)), "..")
	t.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", pluginRoot)
	got, err := Resolve(Config{DefaultFileName: testLiteMMDB}, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != testLiteMMDB {
		t.Errorf("got %q", got)
	}
	if filepath.Base(filepath.Dir(got)) != SeedDir {
		t.Errorf("bundled file should be under seeds/, got %q", got)
	}
}

func TestResolve_MissingPathUsesEnvBundled(t *testing.T) {
	pluginRoot := filepath.Join(filepath.Dir(repoFile(t, testLiteMMDB)), "..")
	t.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", pluginRoot)
	got, err := Resolve(Config{
		Path:            filepath.Join(t.TempDir(), "not-here.mmdb"),
		DefaultFileName: testLiteMMDB,
	}, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != testLiteMMDB {
		t.Errorf("missing catalog path should fall through to bundled, got %q", got)
	}
}

// TestResolve_MissingPathWarns records WARN when catalog Path is set but the file is gone.
func TestResolve_MissingPathWarns(t *testing.T) {
	pluginRoot := filepath.Join(filepath.Dir(repoFile(t, testLiteMMDB)), "..")
	t.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", pluginRoot)
	missing := filepath.Join(t.TempDir(), "not-here.mmdb")
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	got, err := Resolve(Config{
		Path:            missing,
		DefaultFileName: testLiteMMDB,
	}, logger)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != testLiteMMDB {
		t.Errorf("missing catalog path should fall through to bundled, got %q", got)
	}
	out := buf.String()
	if !strings.Contains(out, "seed was specified but not found") {
		t.Errorf("WARN missing, log: %s", out)
	}
	if !strings.Contains(out, filepath.Base(missing)) {
		t.Errorf("WARN path attr missing, log: %s", out)
	}
}

func TestResolve_EmptyDefaultFileNameSkipsSearch(t *testing.T) {
	t.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", t.TempDir())
	got, err := Resolve(Config{Key: "asnlite", Path: filepath.Join(t.TempDir(), "missing.BIN")}, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("ASN without bundled default must be empty, got %q", got)
	}
}

func TestResolve_EmptyKeySkipsLatest(t *testing.T) {
	dir := t.TempDir()
	dated := filepath.Join(dir, time.Now().Format("20060102")+"_lite.mmdb")
	if err := os.WriteFile(dated, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	seed := repoFile(t, testLiteMMDB)
	got, err := Resolve(Config{
		Dir:          dir,
		DatabaseType: TypeMMDB,
		Path:         seed,
	}, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if got != seed {
		t.Errorf("empty key should skip Latest, got %q want %q", got, seed)
	}
}

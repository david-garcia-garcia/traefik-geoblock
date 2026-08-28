package dbdownload

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResolve_DatedFileWins(t *testing.T) {
	dir := t.TempDir()
	dated := filepath.Join(dir, time.Now().Format("20060102")+"_lite.mmdb")
	src, err := os.ReadFile(repoFile(t, "ipinfo_lite.mmdb"))
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
	seed := repoFile(t, "ipinfo_lite.mmdb")
	got, err := Resolve(Config{Path: seed, DefaultFileName: "nope.mmdb"}, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if got != seed {
		t.Errorf("got %q, want %q", got, seed)
	}
}

func TestResolve_EmptyPathFindsEnvDefault(t *testing.T) {
	pluginRoot := filepath.Join(filepath.Dir(repoFile(t, "ipinfo_lite.mmdb")), "..")
	t.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", pluginRoot)
	got, err := Resolve(Config{DefaultFileName: "ipinfo_lite.mmdb"}, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "ipinfo_lite.mmdb" {
		t.Errorf("got %q", got)
	}
	if filepath.Base(filepath.Dir(got)) != SeedDir {
		t.Errorf("bundled file should be under seeds/, got %q", got)
	}
}

func TestResolve_MissingPathUsesEnvBundled(t *testing.T) {
	pluginRoot := filepath.Join(filepath.Dir(repoFile(t, "ipinfo_lite.mmdb")), "..")
	t.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", pluginRoot)
	got, err := Resolve(Config{
		Path:            filepath.Join(t.TempDir(), "not-here.mmdb"),
		DefaultFileName: "ipinfo_lite.mmdb",
	}, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "ipinfo_lite.mmdb" {
		t.Errorf("missing catalog path should fall through to bundled, got %q", got)
	}
}

func TestResolve_EmptyKeySkipsLatest(t *testing.T) {
	dir := t.TempDir()
	dated := filepath.Join(dir, time.Now().Format("20060102")+"_lite.mmdb")
	if err := os.WriteFile(dated, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	seed := repoFile(t, "ipinfo_lite.mmdb")
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

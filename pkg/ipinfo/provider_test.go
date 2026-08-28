package ipinfo

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"log/slog"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbwrappers"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
)

func testMMDB(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	p := filepath.Join(filepath.Dir(file), "..", "..", "seeds", DefaultFileName)
	if !fileutils.Exists(p) {
		t.Fatal("ipinfo_lite.mmdb not found; commit it under seeds/")
	}
	return p
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestLookup_PublicAndPrivate(t *testing.T) {
	dbwrappers.Reset()
	t.Cleanup(dbwrappers.Reset)

	p, err := New(DatabaseConfig{Source: dbsource.Config{Path: testMMDB(t)}}, testLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	rec, err := p.Lookup("8.8.8.8")
	if err != nil {
		t.Fatalf("Lookup 8.8.8.8: %v", err)
	}
	if rec.Country != "US" {
		t.Errorf("country: got %q want US", rec.Country)
	}
	if rec.CountryName != "United States" {
		t.Errorf("country_name: got %q", rec.CountryName)
	}
	if rec.Continent != "North America" || rec.ContinentCode != "NA" {
		t.Errorf("continent: got %q / %q", rec.Continent, rec.ContinentCode)
	}
	if rec.Asn != "AS15169" {
		t.Errorf("asn: got %q want AS15169", rec.Asn)
	}
	if rec.Isp != "Google LLC" {
		t.Errorf("isp: got %q want Google LLC", rec.Isp)
	}
	if rec.Domain != "google.com" {
		t.Errorf("domain: got %q want google.com", rec.Domain)
	}
	if rec.Region != "" || rec.City != "" {
		t.Errorf("Lite has no region/city, got region=%q city=%q", rec.Region, rec.City)
	}

	au, err := p.Lookup("1.1.1.1")
	if err != nil {
		t.Fatalf("Lookup 1.1.1.1: %v", err)
	}
	if au.Country != "AU" {
		t.Errorf("1.1.1.1 country: got %q want AU", au.Country)
	}

	de, err := p.Lookup("85.214.132.117")
	if err != nil {
		t.Fatalf("Lookup German test IP: %v", err)
	}
	if de.Country != "DE" || de.CountryName != "Germany" || de.Continent != "Europe" || de.ContinentCode != "EU" || de.Isp != "Strato GmbH" || de.Domain != "strato.de" || de.Asn != "AS6724" {
		t.Errorf("85.214.132.117 Lite fields: %+v", de)
	}

	priv, err := p.Lookup("127.0.0.1")
	if err != nil {
		t.Fatalf("Lookup 127.0.0.1: %v", err)
	}
	if priv.Country != "" {
		t.Errorf("private IP should have empty country, got %q", priv.Country)
	}

	_, err = p.Lookup("not-an-ip")
	if err == nil {
		t.Fatal("expected error for invalid IP")
	}
}

func TestNew_EmptyPathFindsBundled(t *testing.T) {
	dbwrappers.Reset()
	t.Cleanup(dbwrappers.Reset)

	t.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", filepath.Dir(testMMDB(t)))

	p, err := New(DatabaseConfig{}, testLogger())
	if err != nil {
		t.Fatalf("New with empty path: %v", err)
	}
	rec, err := p.Lookup("8.8.8.8")
	if err != nil || rec.Country != "US" {
		t.Fatalf("bundled MMDB lookup: rec=%+v err=%v", rec, err)
	}
}

func TestNew_EmptyMapUsesSeed(t *testing.T) {
	dbwrappers.Reset()
	t.Cleanup(dbwrappers.Reset)

	p, err := New(DatabaseConfig{
		Source: dbsource.Config{Path: testMMDB(t)},
		DatabaseAutoUpdateDir: t.TempDir(),
	}, testLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rec, err := p.Lookup("8.8.8.8")
	if err != nil || rec.Country != "US" {
		t.Fatalf("seed lookup without download URL: rec=%+v err=%v", rec, err)
	}
}

func TestNew_Singleton(t *testing.T) {
	dbwrappers.Reset()
	t.Cleanup(dbwrappers.Reset)

	cfg := DatabaseConfig{Source: dbsource.Config{Path: testMMDB(t)}}
	a, err := New(cfg, testLogger())
	if err != nil {
		t.Fatalf("New a: %v", err)
	}
	b, err := New(cfg, testLogger())
	if err != nil {
		t.Fatalf("New b: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	rec, err := b.Lookup("8.8.8.8")
	if err != nil || rec.Country != "US" {
		t.Fatalf("shared lookup after Close: rec=%+v err=%v", rec, err)
	}
}

func TestDownloadThroughComponent_HTTP(t *testing.T) {
	src, err := os.ReadFile(testMMDB(t))
	if err != nil {
		t.Fatal(err)
	}
	var gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.URL.Query().Get("token")
		_, _ = w.Write(src)
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	dl := dbsource.Config{
		Key:          "lite",
		URL:          srv.URL + "/ipinfo_lite.mmdb?token=t",
		DatabaseType: dbsource.TypeMMDB,
		Archive:      dbsource.ArchiveNone,
		Dir:          dir,
	}
	path, err := dbsource.Update(dl, testLogger())
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if gotToken != "t" {
		t.Errorf("token query: %q", gotToken)
	}
	if !strings.HasSuffix(path, "_lite.mmdb") {
		t.Errorf("dated name: %s", path)
	}

	dbwrappers.Reset()
	t.Cleanup(dbwrappers.Reset)
	p, err := New(DatabaseConfig{
		DatabaseAutoUpdateDir: dir,
		Source: dbsource.Config{
			Key:          "lite",
			DatabaseType: dbsource.TypeMMDB,
		},
	}, testLogger())
	if err != nil {
		t.Fatalf("open downloaded: %v", err)
	}
	rec, err := p.Lookup("8.8.8.8")
	if err != nil || rec.Country != "US" {
		t.Fatalf("downloaded lookup: rec=%+v err=%v", rec, err)
	}
}

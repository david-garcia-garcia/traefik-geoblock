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

	"github.com/oschwald/maxminddb-golang"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
)

func testMMDB(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	p := filepath.Join(filepath.Dir(file), "..", "..", DefaultFileName)
	if !fileutils.Exists(p) {
		t.Fatal("ipinfo_lite.mmdb not found; commit it at the module root")
	}
	return p
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestLookup_PublicAndPrivate(t *testing.T) {
	resetFactories()
	t.Cleanup(resetFactories)

	p, err := New(DatabaseConfig{DatabaseFilePath: testMMDB(t)}, testLogger())
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
	resetFactories()
	t.Cleanup(resetFactories)

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

func TestNew_AutoUpdateWithoutTokenUsesSeed(t *testing.T) {
	resetFactories()
	t.Cleanup(resetFactories)

	p, err := New(DatabaseConfig{
		DatabaseFilePath:      testMMDB(t),
		DatabaseAutoUpdate:    true,
		DatabaseAutoUpdateDir: t.TempDir(),
	}, testLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rec, err := p.Lookup("8.8.8.8")
	if err != nil || rec.Country != "US" {
		t.Fatalf("seed lookup after auto-update-without-token: rec=%+v err=%v", rec, err)
	}
}

func TestNew_AutoUpdateRequiresDir(t *testing.T) {
	resetFactories()
	t.Cleanup(resetFactories)

	_, err := New(DatabaseConfig{
		DatabaseFilePath:   testMMDB(t),
		DatabaseAutoUpdate: true,
	}, testLogger())
	if err == nil {
		t.Fatal("expected error when auto-update is on without dir")
	}
}

func TestNew_Singleton(t *testing.T) {
	resetFactories()
	t.Cleanup(resetFactories)

	cfg := DatabaseConfig{DatabaseFilePath: testMMDB(t)}
	a, err := New(cfg, testLogger())
	if err != nil {
		t.Fatalf("New a: %v", err)
	}
	b, err := New(cfg, testLogger())
	if err != nil {
		t.Fatalf("New b: %v", err)
	}
	if a != b {
		t.Fatal("expected same provider instance for the same config")
	}
}

func TestPackageCode(t *testing.T) {
	if fileNameForCode("") != DefaultFileName || fileNameForCode("LITE") != "ipinfo_lite.mmdb" {
		t.Fatalf("lite filename: %s %s", fileNameForCode(""), fileNameForCode("LITE"))
	}
	if fileNameForCode("core") != "ipinfo_core.mmdb" || fileNameForCode("plus") != "ipinfo_plus.mmdb" {
		t.Fatal("core/plus filenames")
	}
	if knownPackageCode("nope") {
		t.Fatal("unknown code should be rejected")
	}
}

func TestNew_UnknownCode(t *testing.T) {
	resetFactories()
	t.Cleanup(resetFactories)
	_, err := New(DatabaseConfig{DatabaseFilePath: testMMDB(t), DatabaseAutoUpdateCode: "free"}, testLogger())
	if err == nil {
		t.Fatal("expected error for unknown package code")
	}
}

func TestPackageDownloadURL(t *testing.T) {
	if got := packageDownloadURL("", "lite"); got != "" {
		t.Errorf("empty token: got %q", got)
	}
	got := packageDownloadURL("secret", "core")
	if !strings.Contains(got, "ipinfo_core.mmdb") || !strings.Contains(got, "token=secret") {
		t.Errorf("url: %s", got)
	}
}

func TestFindLatestDatabase(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"20230101_ipinfo_lite.mmdb", "20230301_ipinfo_lite.mmdb", "20230401_ipinfo_core.mmdb", "skip.mmdb"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	latest, err := findLatestDatabase(dir, "lite")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(latest, "20230301_ipinfo_lite.mmdb") {
		t.Errorf("latest lite: %s", latest)
	}
	core, err := findLatestDatabase(dir, "core")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(core, "20230401_ipinfo_core.mmdb") {
		t.Errorf("latest core: %s", core)
	}
}

func TestDownloadAndUpdateDatabase_HTTP(t *testing.T) {
	src, err := os.ReadFile(testMMDB(t))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("token") != "t" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write(src)
	}))
	t.Cleanup(srv.Close)

	prev := getDownloadURL
	getDownloadURL = func(token, code string) string {
		return srv.URL + "?token=" + token
	}
	t.Cleanup(func() { getDownloadURL = prev })

	dir := t.TempDir()
	path, err := downloadAndUpdateDatabase(DatabaseConfig{
		DatabaseAutoUpdateDir:   dir,
		DatabaseAutoUpdateToken: "t",
	}, testLogger())
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	reader, err := maxminddb.FromBytes(src)
	if err != nil {
		t.Fatalf("bundled MMDB: %v", err)
	}
	wantDate, err := mmdbBuildDate(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatalf("build_epoch: %v", err)
	}
	wantName := wantDate.Format("20060102") + "_" + fileNameForCode("")
	if !strings.HasSuffix(path, wantName) {
		t.Errorf("dated name: got %s, want suffix %s", path, wantName)
	}

	resetFactories()
	t.Cleanup(resetFactories)
	p, err := New(DatabaseConfig{DatabaseFilePath: path}, testLogger())
	if err != nil {
		t.Fatalf("open downloaded: %v", err)
	}
	rec, err := p.Lookup("8.8.8.8")
	if err != nil || rec.Country != "US" {
		t.Fatalf("downloaded lookup: rec=%+v err=%v", rec, err)
	}

	_, err = downloadAndUpdateDatabase(DatabaseConfig{
		DatabaseAutoUpdateDir:   dir,
		DatabaseAutoUpdateToken: "",
	}, testLogger())
	if err == nil {
		t.Fatal("expected error without token")
	}
}

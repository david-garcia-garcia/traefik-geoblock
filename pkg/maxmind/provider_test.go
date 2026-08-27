package maxmind

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"log/slog"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
)

const (
	dummyGB = "81.2.69.142"
	dummyCN = "175.16.199.1"
)

func testMMDB(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	p := filepath.Join(filepath.Dir(file), "..", "..", DefaultSeedFileName)
	if !fileutils.Exists(p) {
		t.Fatal("GeoIP2-Country-Test.mmdb not found; commit it at the module root")
	}
	return p
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestLookup_DummyCountry(t *testing.T) {
	resetFactories()
	t.Cleanup(resetFactories)

	p, err := New(DatabaseConfig{DatabaseFilePath: testMMDB(t)}, testLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	gb, err := p.Lookup(dummyGB)
	if err != nil {
		t.Fatalf("Lookup %s: %v", dummyGB, err)
	}
	if gb.Country != "GB" {
		t.Errorf("country: got %q want GB", gb.Country)
	}
	if gb.CountryName != "United Kingdom" {
		t.Errorf("country_name: got %q", gb.CountryName)
	}
	if gb.Continent != "Europe" || gb.ContinentCode != "EU" {
		t.Errorf("continent: got %q / %q", gb.Continent, gb.ContinentCode)
	}
	if gb.Asn != "" || gb.Isp != "" {
		t.Errorf("Country dummy has no ASN/ISP, got asn=%q isp=%q", gb.Asn, gb.Isp)
	}

	cn, err := p.Lookup(dummyCN)
	if err != nil {
		t.Fatalf("Lookup %s: %v", dummyCN, err)
	}
	if cn.Country != "CN" {
		t.Errorf("175.16.199.1 country: got %q want CN", cn.Country)
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
	rec, err := p.Lookup(dummyGB)
	if err != nil || rec.Country != "GB" {
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
	rec, err := p.Lookup(dummyGB)
	if err != nil || rec.Country != "GB" {
		t.Fatalf("seed lookup after auto-update-without-token: rec=%+v err=%v", rec, err)
	}
}

func TestNew_AutoUpdateInvalidTokenUsesSeed(t *testing.T) {
	resetFactories()
	t.Cleanup(resetFactories)

	p, err := New(DatabaseConfig{
		DatabaseFilePath:        testMMDB(t),
		DatabaseAutoUpdate:      true,
		DatabaseAutoUpdateDir:   t.TempDir(),
		DatabaseAutoUpdateToken: "not-a-pair",
	}, testLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rec, err := p.Lookup(dummyGB)
	if err != nil || rec.Country != "GB" {
		t.Fatalf("seed lookup after invalid token: rec=%+v err=%v", rec, err)
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

func TestNew_ASNCodeRejected(t *testing.T) {
	resetFactories()
	t.Cleanup(resetFactories)

	_, err := New(DatabaseConfig{
		DatabaseFilePath:       testMMDB(t),
		DatabaseAutoUpdateCode: "GeoLite2-ASN",
	}, testLogger())
	if err == nil {
		t.Fatal("expected error for GeoLite2-ASN")
	}
}

func TestParseAccountToken(t *testing.T) {
	id, key, ok := parseAccountToken("123456:secret")
	if !ok || id != "123456" || key != "secret" {
		t.Errorf("got id=%q key=%q ok=%v", id, key, ok)
	}
	if _, _, ok := parseAccountToken(""); ok {
		t.Error("empty token should fail")
	}
	if _, _, ok := parseAccountToken("nocolon"); ok {
		t.Error("token without colon should fail")
	}
	if _, _, ok := parseAccountToken(":onlykey"); ok {
		t.Error("empty account id should fail")
	}
}

func TestDefaultDownloadURL(t *testing.T) {
	got := defaultDownloadURL(CodeGeoLite2Country)
	want := "https://download.maxmind.com/geoip/databases/GeoLite2-Country/download?suffix=tar.gz"
	if got != want {
		t.Errorf("url: got %q want %q", got, want)
	}
}

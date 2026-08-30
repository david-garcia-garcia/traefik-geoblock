package dbwrappers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/maxmind"
)

const (
	dummyGB = "81.2.69.142"
	dummyCN = "175.16.199.1"
)

func testCountryMMDB(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	p := filepath.Join(filepath.Dir(file), "..", "..", "seeds", maxmind.DefaultSeedFileName)
	if !fileutils.Exists(p) {
		t.Fatal("GeoIP2-Country-Test.mmdb not found; commit it under seeds/")
	}
	return p
}

func TestGeoIP2_DummyCountry(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	mmdb, err := OpenMMDB(holdCtx(t), MMDBConfig{Source: dbsource.Config{Path: testCountryMMDB(t)}}, testLogger())
	if err != nil {
		t.Fatalf("OpenMMDB: %v", err)
	}
	src := NewGeoIP2(mmdb)

	gb, err := src.Lookup(dummyGB)
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

	cn, err := src.Lookup(dummyCN)
	if err != nil {
		t.Fatalf("Lookup %s: %v", dummyCN, err)
	}
	if cn.Country != "CN" {
		t.Errorf("175.16.199.1 country: got %q want CN", cn.Country)
	}

	priv, err := src.Lookup("127.0.0.1")
	if err != nil {
		t.Fatalf("Lookup 127.0.0.1: %v", err)
	}
	if priv.Country != "" {
		t.Errorf("private IP should have empty country, got %q", priv.Country)
	}

	_, err = src.Lookup("not-an-ip")
	if err == nil {
		t.Fatal("expected error for invalid IP")
	}
}

func TestGeoIP2_EmptyPathFindsBundled(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	t.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", filepath.Dir(testCountryMMDB(t)))

	mmdb, err := OpenMMDB(holdCtx(t), MMDBConfig{DefaultFileName: maxmind.DefaultSeedFileName}, testLogger())
	if err != nil {
		t.Fatalf("OpenMMDB with empty path: %v", err)
	}
	rec, err := NewGeoIP2(mmdb).Lookup(dummyGB)
	if err != nil || rec.Country != "GB" {
		t.Fatalf("bundled MMDB lookup: rec=%+v err=%v", rec, err)
	}
}

func TestGeoIP2_EmptyMapUsesSeed(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	mmdb, err := OpenMMDB(holdCtx(t), MMDBConfig{
		Dir:    t.TempDir(),
		Source: dbsource.Config{Path: testCountryMMDB(t)},
	}, testLogger())
	if err != nil {
		t.Fatalf("OpenMMDB: %v", err)
	}
	rec, err := NewGeoIP2(mmdb).Lookup(dummyGB)
	if err != nil || rec.Country != "GB" {
		t.Fatalf("seed lookup without download URL: rec=%+v err=%v", rec, err)
	}
}

func TestGeoIP2_CloseDoesNotBreakSharedWrapper(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	cfg := MMDBConfig{Source: dbsource.Config{Path: testCountryMMDB(t)}}
	first, err := OpenMMDB(holdCtx(t), cfg, testLogger())
	if err != nil {
		t.Fatalf("OpenMMDB a: %v", err)
	}
	second, err := OpenMMDB(holdCtx(t), cfg, testLogger())
	if err != nil {
		t.Fatalf("OpenMMDB b: %v", err)
	}
	if err := NewGeoIP2(first).Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	rec, err := NewGeoIP2(second).Lookup(dummyGB)
	if err != nil || rec.Country != "GB" {
		t.Fatalf("shared lookup after Close: rec=%+v err=%v", rec, err)
	}
}

func TestGeoIP2_DownloadThroughComponent(t *testing.T) {
	src, err := os.ReadFile(testCountryMMDB(t))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{Name: "GeoLite2-Country_20260101/GeoLite2-Country.mmdb", Mode: 0644, Size: int64(len(src))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(src); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(buf.Bytes())
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	dl := dbsource.Config{
		Key:          "geolite",
		URL:          srv.URL,
		Headers:      map[string]string{"Authorization": "Basic dGVzdA=="},
		DatabaseType: dbsource.TypeMMDB,
		Archive:      dbsource.ArchiveTarGz,
		Dir:          dir,
	}
	path, err := dbsource.Update(dl, testLogger())
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if gotAuth != "Basic dGVzdA==" {
		t.Errorf("Authorization: %q", gotAuth)
	}
	if !strings.HasSuffix(path, "_geolite.mmdb") {
		t.Errorf("dated name: %s", path)
	}

	Reset()
	t.Cleanup(Reset)
	mmdb, err := OpenMMDB(holdCtx(t), MMDBConfig{
		Dir: dir,
		Source: dbsource.Config{
			Key:          "geolite",
			DatabaseType: dbsource.TypeMMDB,
		},
	}, testLogger())
	if err != nil {
		t.Fatalf("open downloaded: %v", err)
	}
	rec, err := NewGeoIP2(mmdb).Lookup(dummyGB)
	if err != nil || rec.Country != "GB" {
		t.Fatalf("downloaded lookup: rec=%+v err=%v", rec, err)
	}
}

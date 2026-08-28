package dbdownload

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"log/slog"

	"github.com/oschwald/maxminddb-golang"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbutils"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func repoFile(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..")
	p := filepath.Join(root, name)
	if _, err := os.Stat(p); err != nil {
		p = filepath.Join(root, SeedDir, name)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return p
}

func TestNormalize_InfersArchive(t *testing.T) {
	cfg := Config{Key: "lite", URL: "https://example.com/IP2LOCATION-LITE-DB1.IPV6.BIN.ZIP", DatabaseType: TypeBIN}
	if err := Normalize(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Archive != ArchiveZIP {
		t.Errorf("archive: %s", cfg.Archive)
	}
}

func TestNormalize_ExtensionlessURLFails(t *testing.T) {
	cfg := Config{Key: "tok", URL: "https://example.com/download", DatabaseType: TypeBIN}
	err := Normalize(&cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "example.com") {
		t.Errorf("error leaked URL: %v", err)
	}
}

func TestNormalize_UnknownType(t *testing.T) {
	err := Normalize(&Config{Key: "x", URL: "https://example.com/a.mmdb", DatabaseType: "csv"})
	if err == nil || !strings.Contains(err.Error(), "databaseType") {
		t.Fatalf("got %v", err)
	}
}

func TestNormalize_UnknownArchive(t *testing.T) {
	err := Normalize(&Config{Key: "x", URL: "https://example.com/a.mmdb", DatabaseType: TypeMMDB, Archive: "rar"})
	if err == nil || !strings.Contains(err.Error(), "archive") {
		t.Fatalf("got %v", err)
	}
}

func TestUpdate_ZipBIN(t *testing.T) {
	src, err := os.ReadFile(repoFile(t, "IP2LOCATION-LITE-DB1.IPV6.BIN"))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("IP2LOCATION-LITE-DB1.IPV6.BIN")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(src); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("X-Test")
		_, _ = w.Write(buf.Bytes())
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	path, err := Update(Config{
		Key:          "litezip",
		URL:          srv.URL + "/db.ZIP",
		Headers:      map[string]string{"X-Test": "1"},
		DatabaseType: TypeBIN,
		Archive:      ArchiveZIP,
		Dir:          dir,
	}, testLogger())
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if gotAuth != "1" {
		t.Errorf("header: %q", gotAuth)
	}
	if !strings.Contains(filepath.Base(path), "_litezip.BIN") {
		t.Errorf("name: %s", path)
	}
	if _, err := dbutils.GetDatabaseVersion(path); err != nil {
		t.Fatalf("BIN: %v", err)
	}
}

func TestUpdate_RawMMDB(t *testing.T) {
	src, err := os.ReadFile(repoFile(t, "ipinfo_lite.mmdb"))
	if err != nil {
		t.Fatal(err)
	}
	var gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.URL.Query().Get("token")
		_, _ = w.Write(src)
	}))
	t.Cleanup(srv.Close)

	reader, err := maxminddb.FromBytes(src)
	if err != nil {
		t.Fatal(err)
	}
	wantDate, err := dbutils.MMDBBuildDate(reader.Metadata.BuildEpoch)
	_ = reader.Close()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	path, err := Update(Config{
		Key:          "lite",
		URL:          srv.URL + "/ipinfo_lite.mmdb?token=secret",
		DatabaseType: TypeMMDB,
		Dir:          dir,
	}, testLogger())
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if gotToken != "secret" {
		t.Errorf("token: %q", gotToken)
	}
	want := wantDate.Format("20060102") + "_lite.mmdb"
	if filepath.Base(path) != want {
		t.Errorf("name: %s want %s", path, want)
	}
}

func TestUpdate_TarGzMMDB(t *testing.T) {
	src, err := os.ReadFile(repoFile(t, "GeoIP2-Country-Test.mmdb"))
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

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(buf.Bytes())
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	path, err := Update(Config{
		Key:          "geolite",
		URL:          srv.URL,
		DatabaseType: TypeMMDB,
		Archive:      ArchiveTarGz,
		Dir:          dir,
	}, testLogger())
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !strings.Contains(filepath.Base(path), "_geolite.mmdb") {
		t.Errorf("name: %s", path)
	}
}

func TestUpdate_HintOmitsURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("nope"))
	}))
	t.Cleanup(srv.Close)

	_, err := Update(Config{
		Key:          "lite",
		URL:          srv.URL + "?token=secret",
		DatabaseType: TypeMMDB,
		Archive:      ArchiveNone,
		Dir:          t.TempDir(),
	}, testLogger())
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "token=secret") || strings.Contains(err.Error(), srv.URL) {
		t.Errorf("leaked URL: %v", err)
	}
}

package ip2location

import (
	"archive/zip"
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbdownload"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbutils"
)

func TestNew_LookupWithoutASNFile(t *testing.T) {
	CleanupFactories()
	defer CleanupFactories()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	p, err := New(DatabaseConfig{Download: dbdownload.Config{Path: testDBFile}}, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	rec, err := p.Lookup("8.8.8.8")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if rec.Country != "US" {
		t.Errorf("country: got %q", rec.Country)
	}
	if rec.Asn != "" {
		t.Errorf("expected empty ASN without ASN BIN, got %q", rec.Asn)
	}
}

func TestNew_LookupDB8(t *testing.T) {
	db8 := filepath.Join("..", "..", "testdata", "IP2LOCATION-DB8.BIN")
	if !fileExists(db8) {
		t.Skip("paid DB8 BIN not present; place testdata/IP2LOCATION-DB8.BIN")
	}

	CleanupFactories()
	defer CleanupFactories()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	p, err := New(DatabaseConfig{Download: dbdownload.Config{Path: db8}}, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	rec, err := p.Lookup("8.8.8.8")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if rec.Country != "US" || rec.Region != "California" || rec.City != "Mountain View" {
		t.Errorf("geo: %+v", rec)
	}
	if rec.Isp != "Google LLC" || rec.Domain != "google.com" {
		t.Errorf("isp/domain: isp=%q domain=%q", rec.Isp, rec.Domain)
	}
	if rec.Asn != "" {
		t.Errorf("DB8 has no ASN, got %q", rec.Asn)
	}
}

func TestNew_LookupWithASNFile(t *testing.T) {
	asnPath := os.Getenv("IP2LOCATION_ASN_BIN")
	if asnPath == "" {
		for _, candidate := range []string{
			filepath.Join("..", "..", "IP2LOCATION-LITE-ASN.IPV6.BIN"),
			filepath.Join("..", "..", "testdata", "IP2LOCATION-LITE-ASN.IPV6.BIN"),
			`D:\IP2LOCATION-LITE-ASN.IPV6.BIN`,
			filepath.Join("..", "..", "IP-ASN.BIN"),
		} {
			if fileExists(candidate) {
				asnPath = candidate
				break
			}
		}
	}
	if asnPath == "" {
		t.Skip("no ASN BIN; set IP2LOCATION_ASN_BIN or place IP2LOCATION-LITE-ASN.IPV6.BIN at repo root")
	}

	CleanupFactories()
	defer CleanupFactories()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	p, err := New(DatabaseConfig{
		Download: dbdownload.Config{Path: testDBFile},
		AsnDownload: dbdownload.Config{Path: asnPath},
	}, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	rec, err := p.Lookup("8.8.8.8")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if rec.Country != "US" {
		t.Errorf("country: got %q", rec.Country)
	}
	if rec.Asn == "" {
		t.Error("expected ASN from ASN BIN for 8.8.8.8")
	}
}

func TestGeoFactoryConfig(t *testing.T) {
	cfg := geoFactoryConfig(DatabaseConfig{
		Download:    dbdownload.Config{Path: testDBFile},
		AsnDownload: dbdownload.Config{URL: "https://example.com/asn.zip", Path: "/asn.bin"},
	})
	if cfg.AsnDownload.URL != "" || cfg.AsnDownload.Path != "" {
		t.Error("geo factory config must not carry ASN fields")
	}
	if cfg.Download.Path != testDBFile {
		t.Errorf("geo path: got %q", cfg.Download.Path)
	}
	if cfg.BinRole != dbutils.SlotGeo {
		t.Errorf("bin role: got %q", cfg.BinRole)
	}
}

func TestAsnFactoryConfig(t *testing.T) {
	cfg := asnFactoryConfig(DatabaseConfig{
		DatabaseAutoUpdateDir: "/data",
		AsnDownload: dbdownload.Config{
			URL:     "https://example.com/asn.zip",
			Headers: map[string]string{"X-Test": "1"},
		},
	})
	if cfg.BinRole != dbutils.SlotASN {
		t.Errorf("bin role: got %q", cfg.BinRole)
	}
	if !cfg.AllowMissing {
		t.Error("expected AllowMissing when ASN path is empty")
	}
	if cfg.Download.URL != "https://example.com/asn.zip" {
		t.Error("expected ASN factory to take the asn download URL")
	}
	if cfg.DatabaseAutoUpdateDir != "/data" || cfg.Download.Headers["X-Test"] != "1" {
		t.Error("expected ASN factory to reuse dir and headers")
	}
}

func TestAsnFactoryConfig_NoURLAllowsMissing(t *testing.T) {
	cfg := asnFactoryConfig(DatabaseConfig{
		DatabaseAutoUpdateDir: "/data",
	})
	if cfg.Download.URL != "" {
		t.Error("empty asn URL must stay empty")
	}
	if !cfg.AllowMissing {
		t.Error("expected AllowMissing when no ASN path")
	}
}

func TestProvider_Close(t *testing.T) {
	p := &provider{}
	if err := p.Close(); err != nil {
		t.Errorf("empty provider Close: %v", err)
	}
	_ = dbprovider.MetaAsn
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestNew_DownloadThroughComponent(t *testing.T) {
	src, err := os.ReadFile(testDBFile)
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

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(buf.Bytes())
	}))
	t.Cleanup(srv.Close)

	CleanupFactories()
	t.Cleanup(CleanupFactories)

	dir := t.TempDir()
	dl := dbdownload.Config{
		Key:          "litezip",
		URL:          srv.URL + "/db.ZIP",
		DatabaseType: dbdownload.TypeBIN,
		Archive:      dbdownload.ArchiveZIP,
		Dir:          dir,
	}
	path, err := dbdownload.Update(dl, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if !strings.Contains(filepath.Base(path), "_litezip.BIN") {
		t.Errorf("dated name: %s", path)
	}

	p, err := New(DatabaseConfig{
		DatabaseAutoUpdateDir: dir,
		Download: dbdownload.Config{
			Key:          "litezip",
			DatabaseType: dbdownload.TypeBIN,
		},
	}, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rec, err := p.Lookup("8.8.8.8")
	if err != nil || rec.Country != "US" {
		t.Fatalf("downloaded lookup: rec=%+v err=%v", rec, err)
	}
}

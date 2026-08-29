package ip2location

import (
	"archive/zip"
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbwrappers"
)

var testDBFile = filepath.Join("..", "..", "seeds", "IP2LOCATION-LITE-DB1.IPV6.BIN")

func TestNew_LookupWithoutASNFile(t *testing.T) {
	dbwrappers.Reset()
	defer dbwrappers.Reset()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	p, err := New(context.Background(), DatabaseConfig{Source: dbsource.Config{Path: testDBFile}}, logger)
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

	dbwrappers.Reset()
	defer dbwrappers.Reset()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	p, err := New(context.Background(), DatabaseConfig{Source: dbsource.Config{Path: db8}}, logger)
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

	dbwrappers.Reset()
	defer dbwrappers.Reset()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	p, err := New(context.Background(), DatabaseConfig{
		Source:    dbsource.Config{Path: testDBFile},
		AsnSource: dbsource.Config{Path: asnPath},
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

func TestGeoBINConfig(t *testing.T) {
	cfg := geoBINConfig(DatabaseConfig{
		Source:    dbsource.Config{Path: testDBFile},
		AsnSource: dbsource.Config{URL: "https://example.com/asn.zip", Path: "/asn.bin"},
	})
	if cfg.Source.URL != "" && cfg.Source.Path != testDBFile {
		t.Error("geo config must use the geo source")
	}
	if cfg.Source.Path != testDBFile {
		t.Errorf("geo path: got %q", cfg.Source.Path)
	}
	if cfg.DefaultFileName != defaultGeoFileName {
		t.Errorf("geo default name: got %q", cfg.DefaultFileName)
	}
}

func TestAsnBINConfig(t *testing.T) {
	cfg := asnBINConfig(DatabaseConfig{
		DatabaseAutoUpdateDir: "/data",
		AsnSource: dbsource.Config{
			URL:     "https://example.com/asn.zip",
			Headers: map[string]string{"X-Test": "1"},
		},
	})
	if cfg.DefaultFileName != defaultASNFileName {
		t.Errorf("asn default name: got %q", cfg.DefaultFileName)
	}
	if !cfg.AllowMissing {
		t.Error("expected AllowMissing when ASN path is empty")
	}
	if cfg.Source.URL != "https://example.com/asn.zip" {
		t.Error("expected ASN config to take the asn source URL")
	}
	if cfg.Dir != "/data" || cfg.Source.Headers["X-Test"] != "1" {
		t.Error("expected ASN config to reuse dir and headers")
	}
}

func TestAsnBINConfig_NoURLAllowsMissing(t *testing.T) {
	cfg := asnBINConfig(DatabaseConfig{
		DatabaseAutoUpdateDir: "/data",
	})
	if cfg.Source.URL != "" {
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

func TestNew_CloseDoesNotBreakSharedWrapper(t *testing.T) {
	dbwrappers.Reset()
	t.Cleanup(dbwrappers.Reset)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := DatabaseConfig{Source: dbsource.Config{Path: testDBFile}}
	a, err := New(context.Background(), cfg, logger)
	if err != nil {
		t.Fatalf("first New: %v", err)
	}
	b, err := New(context.Background(), cfg, logger)
	if err != nil {
		t.Fatalf("second New: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	rec, err := b.Lookup("8.8.8.8")
	if err != nil || rec.Country != "US" {
		t.Fatalf("shared lookup after Close: rec=%+v err=%v", rec, err)
	}
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

	dbwrappers.Reset()
	t.Cleanup(dbwrappers.Reset)

	dir := t.TempDir()
	dl := dbsource.Config{
		Key:          "litezip",
		URL:          srv.URL + "/db.ZIP",
		DatabaseType: dbsource.TypeBIN,
		Archive:      dbsource.ArchiveZIP,
		Dir:          dir,
	}
	path, err := dbsource.Update(dl, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if !strings.Contains(filepath.Base(path), "_litezip.BIN") {
		t.Errorf("dated name: %s", path)
	}

	p, err := New(context.Background(), DatabaseConfig{
		DatabaseAutoUpdateDir: dir,
		Source: dbsource.Config{
			Key:          "litezip",
			DatabaseType: dbsource.TypeBIN,
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

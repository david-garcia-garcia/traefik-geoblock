package ip2location

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
)

func TestNew_LookupWithoutASNFile(t *testing.T) {
	CleanupFactories()
	defer CleanupFactories()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	p, err := New(DatabaseConfig{DatabaseFilePath: testDBFile}, logger)
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

	p, err := New(DatabaseConfig{DatabaseFilePath: db8}, logger)
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
		DatabaseFilePath:    testDBFile,
		AsnDatabaseFilePath: asnPath,
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
		DatabaseFilePath:          testDBFile,
		AsnDatabaseFilePath:       "/asn.bin",
		AsnDatabaseAutoUpdate:     true,
		AsnDatabaseAutoUpdateCode: "DBASNLITEBIN",
	})
	if cfg.AsnDatabaseFilePath != "" || cfg.AsnDatabaseAutoUpdate || cfg.AsnDatabaseAutoUpdateCode != "" {
		t.Error("geo factory config must not carry ASN fields")
	}
	if cfg.DatabaseFilePath != testDBFile {
		t.Errorf("geo path: got %q", cfg.DatabaseFilePath)
	}
}

func TestAsnFactoryConfig(t *testing.T) {
	cfg := asnFactoryConfig(DatabaseConfig{
		DatabaseAutoUpdateDir:   "/data",
		DatabaseAutoUpdateToken: "tok",
		AsnDatabaseAutoUpdate:   true,
	})
	if cfg.DatabaseAutoUpdateCode != DefaultASNDatabaseCode {
		t.Errorf("code: got %q", cfg.DatabaseAutoUpdateCode)
	}
	if !cfg.AllowMissing {
		t.Error("expected AllowMissing when ASN path is empty")
	}
	if !cfg.DatabaseAutoUpdate {
		t.Error("expected ASN auto-update when a token is set")
	}
	if cfg.DatabaseAutoUpdateDir != "/data" || cfg.DatabaseAutoUpdateToken != "tok" {
		t.Error("expected ASN factory to reuse geo dir and token")
	}
}

func TestNew_AsnAutoUpdateWithoutTokenLogsError(t *testing.T) {
	CleanupFactories()
	defer CleanupFactories()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))
	_, err := New(DatabaseConfig{
		DatabaseFilePath:      testDBFile,
		AsnDatabaseAutoUpdate: true,
	}, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "level=ERROR") {
		t.Errorf("expected ERROR log, got %q", out)
	}
	if !strings.Contains(out, "ip2location_databaseAutoUpdateToken") {
		t.Errorf("expected token mentioned in error, got %q", out)
	}
}

func TestAsnFactoryConfig_NoTokenDisablesAutoUpdate(t *testing.T) {
	cfg := asnFactoryConfig(DatabaseConfig{
		DatabaseAutoUpdateDir: "/data",
		AsnDatabaseAutoUpdate: true,
	})
	if cfg.DatabaseAutoUpdate {
		t.Error("ASN auto-update must stay off without a download token")
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

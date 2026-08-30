package dbwrappers

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
)

func TestBINSource_LookupWithoutASN(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	bin, err := OpenBIN(holdCtx(t), BINConfig{Source: dbsource.Config{Path: testBIN}}, testLogger())
	if err != nil {
		t.Fatalf("OpenBIN: %v", err)
	}
	rec, err := bin.LookupRecord("8.8.8.8", nil)
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

func TestBINSource_LookupDB8(t *testing.T) {
	db8 := filepath.Join("..", "..", "testdata", "IP2LOCATION-DB8.BIN")
	if !fileutils.Exists(db8) {
		t.Skip("paid DB8 BIN not present; place testdata/IP2LOCATION-DB8.BIN")
	}

	Reset()
	t.Cleanup(Reset)

	bin, err := OpenBIN(holdCtx(t), BINConfig{Source: dbsource.Config{Path: db8}}, testLogger())
	if err != nil {
		t.Fatalf("OpenBIN: %v", err)
	}
	rec, err := bin.LookupRecord("8.8.8.8", nil)
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

func TestBINSource_CloseDoesNotBreakSharedWrapper(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	cfg := BINConfig{Source: dbsource.Config{Path: testBIN}}
	first, err := OpenBIN(holdCtx(t), cfg, testLogger())
	if err != nil {
		t.Fatalf("first OpenBIN: %v", err)
	}
	second, err := OpenBIN(holdCtx(t), cfg, testLogger())
	if err != nil {
		t.Fatalf("second OpenBIN: %v", err)
	}
	if err := dbprovider.Bind(func(ip string) (dbprovider.Record, error) {
		return first.LookupRecord(ip, nil)
	}).Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	rec, err := second.LookupRecord("8.8.8.8", nil)
	if err != nil || rec.Country != "US" {
		t.Fatalf("shared lookup after Close: rec=%+v err=%v", rec, err)
	}
}

func TestBINSource_DownloadThroughComponent(t *testing.T) {
	src, err := os.ReadFile(testBIN)
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

	Reset()
	t.Cleanup(Reset)

	dir := t.TempDir()
	dl := dbsource.Config{
		Key:          "litezip",
		URL:          srv.URL + "/db.ZIP",
		DatabaseType: dbsource.TypeBIN,
		Archive:      dbsource.ArchiveZIP,
		Dir:          dir,
	}
	path, err := dbsource.Update(dl, testLogger())
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if !strings.Contains(filepath.Base(path), "_litezip.BIN") {
		t.Errorf("dated name: %s", path)
	}

	bin, err := OpenBIN(holdCtx(t), BINConfig{
		Dir: dir,
		Source: dbsource.Config{
			Key:          "litezip",
			DatabaseType: dbsource.TypeBIN,
		},
	}, testLogger())
	if err != nil {
		t.Fatalf("OpenBIN: %v", err)
	}
	rec, err := bin.LookupRecord("8.8.8.8", nil)
	if err != nil || rec.Country != "US" {
		t.Fatalf("downloaded lookup: rec=%+v err=%v", rec, err)
	}
}

func TestBINSource_FieldsAsnOnlySkipsGeo(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	bin, err := OpenBIN(holdCtx(t), BINConfig{Source: dbsource.Config{Path: testBIN}}, testLogger())
	if err != nil {
		t.Fatalf("OpenBIN: %v", err)
	}
	rec, err := bin.LookupRecord("8.8.8.8", []string{dbprovider.MetaAsn})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if rec.Country != "" {
		t.Errorf("fields [asn] must not call Get_all, country=%q", rec.Country)
	}
}

func TestBINSource_LookupWithASNFile(t *testing.T) {
	asnPath := os.Getenv("IP2LOCATION_ASN_BIN")
	if asnPath == "" {
		for _, candidate := range []string{
			filepath.Join("..", "..", "IP2LOCATION-LITE-ASN.IPV6.BIN"),
			filepath.Join("..", "..", "testdata", "IP2LOCATION-LITE-ASN.IPV6.BIN"),
			`D:\IP2LOCATION-LITE-ASN.IPV6.BIN`,
			filepath.Join("..", "..", "IP-ASN.BIN"),
		} {
			if fileutils.Exists(candidate) {
				asnPath = candidate
				break
			}
		}
	}
	if asnPath == "" {
		t.Skip("no ASN BIN; set IP2LOCATION_ASN_BIN or place IP2LOCATION-LITE-ASN.IPV6.BIN at repo root")
	}

	Reset()
	t.Cleanup(Reset)

	geo, err := OpenBIN(holdCtx(t), BINConfig{Source: dbsource.Config{Path: testBIN}}, testLogger())
	if err != nil {
		t.Fatalf("geo OpenBIN: %v", err)
	}
	asn, err := OpenBIN(holdCtx(t), BINConfig{Source: dbsource.Config{Path: asnPath}}, testLogger())
	if err != nil {
		t.Fatalf("asn OpenBIN: %v", err)
	}
	merged := dbprovider.NewCombined([]dbprovider.Named{
		{Key: "geo", Provider: dbprovider.Bind(func(ip string) (dbprovider.Record, error) {
			return geo.LookupRecord(ip, nil)
		})},
		{Key: "asn", Provider: dbprovider.Bind(func(ip string) (dbprovider.Record, error) {
			return asn.LookupRecord(ip, []string{dbprovider.MetaAsn})
		})},
	})
	rec, err := merged.Lookup("8.8.8.8")
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

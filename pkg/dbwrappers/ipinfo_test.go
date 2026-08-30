package dbwrappers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
)

func TestIPinfo_PublicAndPrivate(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	mmdb, err := OpenMMDB(holdCtx(t), MMDBConfig{Source: dbsource.Config{Path: testLiteMMDB(t)}}, testLogger())
	if err != nil {
		t.Fatalf("OpenMMDB: %v", err)
	}
	rec, err := mmdb.LookupRecord("8.8.8.8", mustFields(t, PresetIPinfoLite))
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

	au, err := mmdb.LookupRecord("1.1.1.1", mustFields(t, PresetIPinfoLite))
	if err != nil {
		t.Fatalf("Lookup 1.1.1.1: %v", err)
	}
	if au.Country != "AU" {
		t.Errorf("1.1.1.1 country: got %q want AU", au.Country)
	}

	de, err := mmdb.LookupRecord("85.214.132.117", mustFields(t, PresetIPinfoLite))
	if err != nil {
		t.Fatalf("Lookup German test IP: %v", err)
	}
	if de.Country != "DE" || de.CountryName != "Germany" || de.Continent != "Europe" || de.ContinentCode != "EU" || de.Isp != "Strato GmbH" || de.Domain != "strato.de" || de.Asn != "AS6724" {
		t.Errorf("85.214.132.117 Lite fields: %+v", de)
	}

	priv, err := mmdb.LookupRecord("127.0.0.1", mustFields(t, PresetIPinfoLite))
	if err != nil {
		t.Fatalf("Lookup 127.0.0.1: %v", err)
	}
	if priv.Country != "" {
		t.Errorf("private IP should have empty country, got %q", priv.Country)
	}

	_, err = mmdb.LookupRecord("not-an-ip", mustFields(t, PresetIPinfoLite))
	if err == nil {
		t.Fatal("expected error for invalid IP")
	}
}

func TestIPinfo_FieldsCountryOnly(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	mmdb, err := OpenMMDB(holdCtx(t), MMDBConfig{Source: dbsource.Config{Path: testLiteMMDB(t)}}, testLogger())
	if err != nil {
		t.Fatalf("OpenMMDB: %v", err)
	}
	rec, err := mmdb.LookupRecord("8.8.8.8", FieldMap{"country_code": {Key: dbprovider.MetaCountry}})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if rec.Country != "US" {
		t.Errorf("country: got %q", rec.Country)
	}
	if rec.Asn != "" || rec.CountryName != "" {
		t.Errorf("fields [country] leaked other keys: %+v", rec)
	}
}

func TestIPinfo_EmptyPathFindsBundled(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	t.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", filepath.Dir(testLiteMMDB(t)))

	mmdb, err := OpenMMDB(holdCtx(t), MMDBConfig{DefaultFileName: "ipinfo_lite.mmdb"}, testLogger())
	if err != nil {
		t.Fatalf("OpenMMDB with empty path: %v", err)
	}
	rec, err := mmdb.LookupRecord("8.8.8.8", mustFields(t, PresetIPinfoLite))
	if err != nil || rec.Country != "US" {
		t.Fatalf("bundled MMDB lookup: rec=%+v err=%v", rec, err)
	}
}

func TestIPinfo_EmptyMapUsesSeed(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	mmdb, err := OpenMMDB(holdCtx(t), MMDBConfig{
		Dir:    t.TempDir(),
		Source: dbsource.Config{Path: testLiteMMDB(t)},
	}, testLogger())
	if err != nil {
		t.Fatalf("OpenMMDB: %v", err)
	}
	rec, err := mmdb.LookupRecord("8.8.8.8", mustFields(t, PresetIPinfoLite))
	if err != nil || rec.Country != "US" {
		t.Fatalf("seed lookup without download URL: rec=%+v err=%v", rec, err)
	}
}

func TestIPinfo_CloseDoesNotBreakSharedWrapper(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	cfg := MMDBConfig{Source: dbsource.Config{Path: testLiteMMDB(t)}}
	first, err := OpenMMDB(holdCtx(t), cfg, testLogger())
	if err != nil {
		t.Fatalf("OpenMMDB a: %v", err)
	}
	second, err := OpenMMDB(holdCtx(t), cfg, testLogger())
	if err != nil {
		t.Fatalf("OpenMMDB b: %v", err)
	}
	if err := dbprovider.Bind(func(ip string) (dbprovider.Record, error) {
		return first.LookupRecord(ip, mustFields(t, PresetIPinfoLite))
	}).Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	rec, err := second.LookupRecord("8.8.8.8", mustFields(t, PresetIPinfoLite))
	if err != nil || rec.Country != "US" {
		t.Fatalf("shared lookup after Close: rec=%+v err=%v", rec, err)
	}
}

func TestIPinfo_DownloadThroughComponent(t *testing.T) {
	src, err := os.ReadFile(testLiteMMDB(t))
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

	Reset()
	t.Cleanup(Reset)
	mmdb, err := OpenMMDB(holdCtx(t), MMDBConfig{
		Dir: dir,
		Source: dbsource.Config{
			Key:          "lite",
			DatabaseType: dbsource.TypeMMDB,
		},
	}, testLogger())
	if err != nil {
		t.Fatalf("open downloaded: %v", err)
	}
	rec, err := mmdb.LookupRecord("8.8.8.8", mustFields(t, PresetIPinfoLite))
	if err != nil || rec.Country != "US" {
		t.Fatalf("downloaded lookup: rec=%+v err=%v", rec, err)
	}
}

package dbwrappers

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/fileutils"
)

func testLiteMMDB(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	p := filepath.Join(filepath.Dir(file), "..", "..", "seeds", "ipinfo_lite.mmdb")
	if !fileutils.Exists(p) {
		t.Fatal("missing seeds/ipinfo_lite.mmdb")
	}
	return p
}

func TestOpenMMDB_LookupAndSingleton(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	cfg := MMDBConfig{Source: dbsource.Config{Path: testLiteMMDB(t)}}
	a, err := OpenMMDB(cfg, testLogger())
	if err != nil {
		t.Fatalf("OpenMMDB: %v", err)
	}
	b, err := OpenMMDB(cfg, testLogger())
	if err != nil {
		t.Fatalf("OpenMMDB 2: %v", err)
	}
	if a != b {
		t.Fatal("expected singleton")
	}
	var rec struct {
		CountryCode string `maxminddb:"country_code"`
	}
	if err := a.Lookup("8.8.8.8", &rec); err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if rec.CountryCode != "US" {
		t.Fatalf("country: %q", rec.CountryCode)
	}
}

func TestOpenMMDB_OpenIsHotSwap(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	path := testLiteMMDB(t)
	w, err := OpenMMDB(MMDBConfig{Source: dbsource.Config{Path: path}}, testLogger())
	if err != nil {
		t.Fatalf("OpenMMDB: %v", err)
	}
	if err := w.open(path); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	var rec struct {
		CountryCode string `maxminddb:"country_code"`
	}
	if err := w.Lookup("8.8.8.8", &rec); err != nil || rec.CountryCode != "US" {
		t.Fatalf("after reopen: %+v %v", rec, err)
	}
}

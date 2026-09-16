package dbwrappers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
)

func TestNewBIN_CorruptDatedUsesSeed(t *testing.T) {
	dir := t.TempDir()
	corrupt := filepath.Join(dir, "20990101_proof.BIN")
	if err := os.WriteFile(corrupt, make([]byte, 4096), 0600); err != nil {
		t.Fatal(err)
	}
	w, err := newBIN(BINConfig{
		Dir: dir,
		Source: dbsource.Config{
			Key:          "proof",
			DatabaseType: dbsource.TypeBIN,
			Path:         testBIN,
		},
	}, testLogger())
	if err != nil {
		t.Fatalf("newBIN: %v", err)
	}
	t.Cleanup(w.Close)
	if !strings.Contains(w.SourcePath(), filepath.Base(testBIN)) && w.SourcePath() != testBIN {
		t.Fatalf("source %q, want seed %s", w.SourcePath(), testBIN)
	}
	rec, lerr := w.LookupRecord("8.8.8.8", mustFields(t, PresetIP2LocationLite))
	if lerr != nil || rec.Country != "US" {
		t.Fatalf("lookup %+v err %v", rec, lerr)
	}
}

func TestNewMMDB_CorruptDatedUsesSeed(t *testing.T) {
	dir := t.TempDir()
	corrupt := filepath.Join(dir, "20990101_proof.mmdb")
	if err := os.WriteFile(corrupt, make([]byte, 4096), 0600); err != nil {
		t.Fatal(err)
	}
	seed := testLiteMMDB(t)
	w, err := newMMDB(MMDBConfig{
		Dir: dir,
		Source: dbsource.Config{
			Key:          "proof",
			DatabaseType: dbsource.TypeMMDB,
			Path:         seed,
		},
	}, testLogger())
	if err != nil {
		t.Fatalf("newMMDB: %v", err)
	}
	t.Cleanup(w.Close)
	if w.Path() == corrupt {
		t.Fatal("live path is the corrupt dated file")
	}
	var rec struct {
		CountryCode string `maxminddb:"country_code"`
	}
	if err := w.Lookup("8.8.8.8", &rec); err != nil || rec.CountryCode != "US" {
		t.Fatalf("lookup %+v err %v", rec, err)
	}
}

package dbwrappers

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbprovider"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
)

// testBINRecord looks up 8.8.8.8 with the LITE country map.
func testBINRecord(t *testing.T, w *BIN) dbprovider.Record {
	t.Helper()
	rec, err := w.LookupRecord("8.8.8.8", mustFields(t, PresetIP2LocationLite))
	if err != nil {
		t.Fatalf("LookupRecord: %v", err)
	}
	return rec
}

var testBIN = filepath.Join("..", "..", "seeds", "IP2LOCATION-LITE-DB1.IPV6.BIN")

// holdCtx is a cancelable Open context. Background and TODO panic in reclaim.Open.
func holdCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestOpenBIN_LookupAndSingleton(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	cfg := BINConfig{Source: dbsource.Config{Path: testBIN}}
	a, err := OpenBIN(holdCtx(t), cfg, testLogger())
	if err != nil {
		t.Fatalf("OpenBIN: %v", err)
	}
	b, err := OpenBIN(holdCtx(t), cfg, testLogger())
	if err != nil {
		t.Fatalf("OpenBIN 2: %v", err)
	}
	if a != b {
		t.Fatal("expected singleton")
	}
	rec := testBINRecord(t, a)
	if rec.Country != "US" {
		t.Fatalf("lookup: %+v", rec)
	}
	if a.Version() == nil || a.Path() == "" {
		t.Fatal("expected version and path")
	}
}

func TestOpenBIN_AllowMissing(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	w, err := OpenBIN(holdCtx(t), BINConfig{AllowMissing: true}, testLogger())
	if err != nil {
		t.Fatalf("AllowMissing: %v", err)
	}
	if _, err := w.LookupRecord("8.8.8.8", mustFields(t, PresetIP2LocationLite)); err == nil {
		t.Fatal("expected LookupRecord error when BIN is not open")
	}
}

func TestOpenBIN_EnvFallback(t *testing.T) {
	envDir := t.TempDir()
	Reset()
	t.Cleanup(Reset)
	src, err := os.ReadFile(testBIN)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(envDir, "IP2LOCATION-LITE-DB1.IPV6.BIN"), src, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRAEFIK_PLUGIN_GEOBLOCK_PATH", envDir)
	w, err := OpenBIN(holdCtx(t), BINConfig{
		Source:          dbsource.Config{Path: "/nonexistent/bad.bin"},
		DefaultFileName: "IP2LOCATION-LITE-DB1.IPV6.BIN",
	}, testLogger())
	if err != nil {
		t.Fatalf("env fallback: %v", err)
	}
	rec := testBINRecord(t, w)
	if rec.Country != "US" {
		t.Fatalf("env lookup: %+v", rec)
	}
}

func TestOpenBIN_HotSwap(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	w, err := OpenBIN(holdCtx(t), BINConfig{Source: dbsource.Config{Path: testBIN}}, testLogger())
	if err != nil {
		t.Fatalf("OpenBIN: %v", err)
	}
	if err := w.hotSwap(testBIN); err != nil {
		t.Fatalf("hotSwap: %v", err)
	}
	rec := testBINRecord(t, w)
	if rec.Country != "US" {
		t.Fatalf("after swap: %+v", rec)
	}
}

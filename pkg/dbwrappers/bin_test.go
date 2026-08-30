package dbwrappers

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
)

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
	rec, err := a.Lookup("8.8.8.8")
	if err != nil || rec.Country != "US" {
		t.Fatalf("lookup: %+v %v", rec, err)
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
	if w.LookupASN("8.8.8.8") != "" {
		t.Fatal("expected empty ASN")
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
	rec, err := w.GetCountryShort("8.8.8.8")
	if err != nil || rec.Country_short != "US" {
		t.Fatalf("env lookup: %+v %v", rec, err)
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
	rec, err := w.Lookup("8.8.8.8")
	if err != nil || rec.Country != "US" {
		t.Fatalf("after swap: %+v %v", rec, err)
	}
}

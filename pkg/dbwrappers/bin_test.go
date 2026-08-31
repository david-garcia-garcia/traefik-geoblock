package dbwrappers

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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

func TestFileToken(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"paid", "paid"},
		{"paid/prod", "paid_prod"},
		{"", ""},
		{"default_ip2location", "default_ip2location"},
	}
	for _, tc := range cases {
		if got := fileToken(tc.in); got != tc.want {
			t.Errorf("fileToken(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBinCopyName(t *testing.T) {
	if got := binCopyName("paid", 1756620895399542303); got != "bin_paid_1756620895399542303.BIN" {
		t.Fatalf("binCopyName: %s", got)
	}
	if got := binCopyName("", 1); got != "bin_1.BIN" {
		t.Fatalf("empty key: %s", got)
	}
}

func TestOpenBIN_InitLogsDatedCopy(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	dir := t.TempDir()
	src, err := os.ReadFile(testBIN)
	if err != nil {
		t.Fatal(err)
	}
	dated := filepath.Join(dir, "20260831_paid.BIN")
	if err := os.WriteFile(dated, src, 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})).
		With("owner_plugin", "traefik-geoblock@kubernetescrd")

	w, err := OpenBIN(holdCtx(t), BINConfig{
		Dir: dir,
		Source: dbsource.Config{
			Key:          "paid",
			DatabaseType: dbsource.TypeBIN,
		},
	}, logger)
	if err != nil {
		t.Fatal(err)
	}

	base := filepath.Base(w.Path())
	if !strings.HasPrefix(base, "bin_paid_") || !strings.HasSuffix(base, ".BIN") {
		t.Fatalf("temp basename %q", base)
	}
	if w.SourcePath() != dated {
		t.Fatalf("SourcePath()=%s want %s", w.SourcePath(), dated)
	}

	out := buf.String()
	if !strings.Contains(out, `msg="BIN initialized"`) {
		t.Fatalf("missing BIN initialized: %s", out)
	}
	if !strings.Contains(out, "source_path="+dated) && !strings.Contains(out, "source_path=\""+dated+"\"") {
		// text handler quotes paths with backslashes on Windows
		if !strings.Contains(out, "source_path=") || !strings.Contains(out, "20260831_paid.BIN") {
			t.Fatalf("missing source_path: %s", out)
		}
	}
	if !strings.Contains(out, "path=") || !strings.Contains(out, "bin_paid_") {
		t.Fatalf("missing temp path: %s", out)
	}
	if !strings.Contains(out, "owner_plugin=traefik-geoblock@kubernetescrd") {
		t.Fatalf("missing owner_plugin: %s", out)
	}
	if strings.Contains(out, " plugin=") {
		t.Fatalf("unexpected plugin=: %s", out)
	}

	if err := w.hotSwap(dated); err != nil {
		t.Fatalf("hotSwap: %v", err)
	}
	swapOut := buf.String()
	if !strings.Contains(swapOut, "BIN hot-swapped") {
		t.Fatalf("missing hot-swapped: %s", swapOut)
	}
	if !strings.Contains(swapOut, "source_path=") || !strings.Contains(swapOut, "20260831_paid.BIN") {
		t.Fatalf("hot-swap source_path: %s", swapOut)
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

package dbwrappers

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
	"github.com/david-garcia-garcia/traefik-geoblock/pkg/reclaim"
)

type recHandler struct {
	mu   sync.Mutex
	recs []slog.Record
}

func (h *recHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	h.recs = append(h.recs, r.Clone())
	h.mu.Unlock()
	return nil
}

func (h *recHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recHandler) WithGroup(string) slog.Handler      { return h }

func (h *recHandler) events() [][2]string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([][2]string, 0, len(h.recs))
	for _, r := range h.recs {
		var key string
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "key" {
				key = a.Value.String()
			}
			return true
		})
		out = append(out, [2]string{r.Message, key})
	}
	return out
}

func hasEvent(got [][2]string, msg, key string) bool {
	for _, g := range got {
		if g[0] == msg && g[1] == key {
			return true
		}
	}
	return false
}

func hasSubseq(got [][2]string, want [][2]string) bool {
	i := 0
	for _, g := range got {
		if i < len(want) && g == want[i] {
			i++
		}
	}
	return i == len(want)
}

func useShortLeases(t *testing.T) *recHandler {
	t.Helper()
	h := &recHandler{}
	ResetWith(25 * time.Millisecond)
	return h
}

func silentTickerURL(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/db.bin"
}

func TestOpenBIN_SameHashReclaimKeepsTicker(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	useShortLeases(t)
	cfg := BINConfig{
		Dir: t.TempDir(),
		Source: dbsource.Config{
			Path:         testBIN,
			URL:          silentTickerURL(t),
			Key:          "same",
			DatabaseType: dbsource.TypeBIN,
		},
		MinAge: 365 * 24 * time.Hour,
	}
	ctx1, cancel1 := context.WithCancel(context.Background())
	a, err := OpenBIN(ctx1, cfg, testLogger())
	if err != nil {
		t.Fatalf("OpenBIN: %v", err)
	}
	if a.updater == nil {
		t.Fatal("expected keep-current loop")
	}
	cancel1()
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	b, err := OpenBIN(ctx2, cfg, testLogger())
	if err != nil {
		t.Fatalf("OpenBIN reclaim: %v", err)
	}
	if a != b {
		t.Fatal("expected same wrapper")
	}
	if a.updater != b.updater {
		t.Fatal("expected one ticker")
	}
	rec := testBINRecord(t, b)
	if rec.Country != "US" {
		t.Fatalf("lookup: %+v", rec)
	}
	time.Sleep(80 * time.Millisecond)
	rec = testBINRecord(t, b)
	if rec.Country != "US" {
		t.Fatalf("after grace: %+v", rec)
	}
}

func TestOpenBIN_HashChangeDisposesOld(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	h := useShortLeases(t)
	cfg1 := BINConfig{
		Dir: t.TempDir(),
		Source: dbsource.Config{
			Path:         testBIN,
			URL:          silentTickerURL(t),
			Key:          "h1",
			DatabaseType: dbsource.TypeBIN,
		},
		MinAge: 365 * 24 * time.Hour,
	}
	copyPath := filepath.Join(t.TempDir(), "copy.BIN")
	src, err := os.ReadFile(testBIN)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copyPath, src, 0600); err != nil {
		t.Fatal(err)
	}
	cfg2 := BINConfig{
		Dir: t.TempDir(),
		Source: dbsource.Config{
			Path:         copyPath,
			URL:          silentTickerURL(t),
			Key:          "h2",
			DatabaseType: dbsource.TypeBIN,
		},
		MinAge: 365 * 24 * time.Hour,
	}
	key1 := binKey(cfg1)
	key2 := binKey(cfg2)
	if !strings.HasPrefix(key1, keyPrefixBIN+"h1:") {
		t.Fatalf("BIN key must include catalog key: %s", key1)
	}
	if !strings.HasPrefix(key2, keyPrefixBIN+"h2:") {
		t.Fatalf("BIN key must include catalog key: %s", key2)
	}
	spy := slog.New(h)
	ctx1, cancel1 := context.WithCancel(context.Background())
	h1, err := OpenBIN(ctx1, cfg1, spy)
	if err != nil {
		t.Fatalf("H1: %v", err)
	}
	if h1.updater == nil {
		t.Fatal("expected H1 ticker")
	}
	cancel1()
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	h2, err := OpenBIN(ctx2, cfg2, spy)
	if err != nil {
		t.Fatalf("H2: %v", err)
	}
	time.Sleep(80 * time.Millisecond)
	if _, err := h1.LookupRecord("8.8.8.8", mustFields(t, PresetIP2LocationLite)); err == nil {
		t.Fatal("H1 loop must be stopped")
	}
	rec := testBINRecord(t, h2)
	if rec.Country != "US" {
		t.Fatalf("H2 lookup: %+v", rec)
	}
	ev := h.events()
	if !hasSubseq(ev, [][2]string{{reclaim.MsgPut, key1}, {reclaim.MsgOrphan, key1}, {reclaim.MsgDispose, key1}}) ||
		!hasEvent(ev, reclaim.MsgPut, key2) {
		t.Fatalf("events: %+v keys %s %s", ev, key1, key2)
	}
	if hasEvent(ev, reclaim.MsgDispose, key2) {
		t.Fatal("H2 must not dispose")
	}
}

func TestOpenMMDB_SameHashReclaimKeepsTicker(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	useShortLeases(t)
	cfg := MMDBConfig{
		Dir: t.TempDir(),
		Source: dbsource.Config{
			Path:         testLiteMMDB(t),
			URL:          silentTickerURL(t),
			Key:          "same",
			DatabaseType: dbsource.TypeMMDB,
		},
		MinAge: 365 * 24 * time.Hour,
	}
	ctx1, cancel1 := context.WithCancel(context.Background())
	a, err := OpenMMDB(ctx1, cfg, testLogger())
	if err != nil {
		t.Fatalf("OpenMMDB: %v", err)
	}
	if a.updater == nil {
		t.Fatal("expected keep-current loop")
	}
	cancel1()
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	b, err := OpenMMDB(ctx2, cfg, testLogger())
	if err != nil {
		t.Fatalf("OpenMMDB reclaim: %v", err)
	}
	if a != b || a.updater != b.updater {
		t.Fatal("expected one wrapper and one ticker")
	}
	var rec struct {
		CountryCode string `maxminddb:"country_code"`
	}
	if err := b.Lookup("8.8.8.8", &rec); err != nil || rec.CountryCode != "US" {
		t.Fatalf("lookup: %+v %v", rec, err)
	}
	time.Sleep(80 * time.Millisecond)
	if err := b.Lookup("8.8.8.8", &rec); err != nil || rec.CountryCode != "US" {
		t.Fatalf("after grace: %+v %v", rec, err)
	}
}

func TestOpenMMDB_HashChangeDisposesOld(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	h := useShortLeases(t)
	lite := testLiteMMDB(t)
	cfg1 := MMDBConfig{
		Dir: t.TempDir(),
		Source: dbsource.Config{
			Path:         lite,
			URL:          silentTickerURL(t),
			Key:          "h1",
			DatabaseType: dbsource.TypeMMDB,
		},
		MinAge: 365 * 24 * time.Hour,
	}
	copyPath := filepath.Join(t.TempDir(), "copy.mmdb")
	src, err := os.ReadFile(lite)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copyPath, src, 0600); err != nil {
		t.Fatal(err)
	}
	cfg2 := MMDBConfig{
		Dir: t.TempDir(),
		Source: dbsource.Config{
			Path:         copyPath,
			URL:          silentTickerURL(t),
			Key:          "h2",
			DatabaseType: dbsource.TypeMMDB,
		},
		MinAge: 365 * 24 * time.Hour,
	}
	key1 := mmdbKey(cfg1)
	key2 := mmdbKey(cfg2)
	if !strings.HasPrefix(key1, keyPrefixMMDB+"h1:") {
		t.Fatalf("MMDB key must include catalog key: %s", key1)
	}
	if !strings.HasPrefix(key2, keyPrefixMMDB+"h2:") {
		t.Fatalf("MMDB key must include catalog key: %s", key2)
	}
	spy := slog.New(h)
	ctx1, cancel1 := context.WithCancel(context.Background())
	h1, err := OpenMMDB(ctx1, cfg1, spy)
	if err != nil {
		t.Fatalf("H1: %v", err)
	}
	if h1.updater == nil {
		t.Fatal("expected H1 ticker")
	}
	cancel1()
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	h2, err := OpenMMDB(ctx2, cfg2, spy)
	if err != nil {
		t.Fatalf("H2: %v", err)
	}
	time.Sleep(80 * time.Millisecond)
	var rec struct {
		CountryCode string `maxminddb:"country_code"`
	}
	if err := h1.Lookup("8.8.8.8", &rec); err == nil {
		t.Fatal("H1 loop must be stopped")
	}
	if err := h2.Lookup("8.8.8.8", &rec); err != nil || rec.CountryCode != "US" {
		t.Fatalf("H2 lookup: %+v %v", rec, err)
	}
	ev := h.events()
	if !hasSubseq(ev, [][2]string{{reclaim.MsgPut, key1}, {reclaim.MsgOrphan, key1}, {reclaim.MsgDispose, key1}}) ||
		!hasEvent(ev, reclaim.MsgPut, key2) {
		t.Fatalf("events: %+v", ev)
	}
	if hasEvent(ev, reclaim.MsgDispose, key2) {
		t.Fatal("H2 must not dispose")
	}
}

package dbwrappers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
)

// holdDownload serves body after release. held closes when the GET is inside the handler.
func holdDownload(t *testing.T, body []byte, suffix string) (rawURL string, held <-chan struct{}, release func()) {
	t.Helper()
	inFlight := make(chan struct{})
	gate := make(chan struct{})
	var released atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(inFlight)
		<-gate
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	unlock := func() {
		if released.CompareAndSwap(false, true) {
			close(gate)
		}
	}
	t.Cleanup(unlock)
	return srv.URL + suffix, inFlight, unlock
}

// waitHeld fails the test if the download handler is not entered in time.
func waitHeld(t *testing.T, held <-chan struct{}) {
	t.Helper()
	select {
	case <-held:
	case <-time.After(5 * time.Second):
		t.Fatal("GET never started")
	}
}

// closeWhileHeld starts Close, asserts it waits while the GET is held, then releases and joins.
func closeWhileHeld(t *testing.T, closeFn func(), release func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		closeFn()
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("Close returned while GET was still held")
	case <-time.After(150 * time.Millisecond):
	}
	release()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("Close did not join the ticker goroutine")
	}
}

// leftoverBINCopies lists process-temp BIN copies for catalogKey.
func leftoverBINCopies(catalogKey string) []string {
	pattern := filepath.Join(os.TempDir(), "bin_"+fileToken(catalogKey)+"_*.BIN")
	matches, _ := filepath.Glob(pattern)
	return matches
}

// TestOpenBIN_DelayedDownloadAfterClose holds the first GET until after Close.
func TestOpenBIN_DelayedDownloadAfterClose(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	body, err := os.ReadFile(testBIN)
	if err != nil {
		t.Fatal(err)
	}
	const catalogKey = "delayed-bin"
	rawURL, held, release := holdDownload(t, body, "/db.BIN")
	w, err := OpenBIN(holdCtx(t), BINConfig{
		Dir: t.TempDir(),
		Source: dbsource.Config{
			Path:         testBIN,
			URL:          rawURL,
			Key:          catalogKey,
			DatabaseType: dbsource.TypeBIN,
			Archive:      dbsource.ArchiveNone,
		},
	}, testLogger())
	if err != nil {
		t.Fatalf("OpenBIN: %v", err)
	}
	waitHeld(t, held)
	closeWhileHeld(t, w.Close, release)
	if _, err := w.LookupRecord("8.8.8.8", mustFields(t, PresetIP2LocationLite)); err == nil {
		t.Fatal("LookupRecord succeeded after Close")
	}
	if leftover := leftoverBINCopies(catalogKey); len(leftover) > 0 {
		t.Fatalf("late BIN temp copy remained: %v", leftover)
	}
}

// TestOpenMMDB_DelayedDownloadAfterClose holds the first GET until after Close.
func TestOpenMMDB_DelayedDownloadAfterClose(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	seed := testLiteMMDB(t)
	body, err := os.ReadFile(seed)
	if err != nil {
		t.Fatal(err)
	}
	rawURL, held, release := holdDownload(t, body, "/db.mmdb")
	w, err := OpenMMDB(holdCtx(t), MMDBConfig{
		Dir: t.TempDir(),
		Source: dbsource.Config{
			Path:         seed,
			URL:          rawURL,
			Key:          "delayed-mmdb",
			DatabaseType: dbsource.TypeMMDB,
			Archive:      dbsource.ArchiveNone,
		},
	}, testLogger())
	if err != nil {
		t.Fatalf("OpenMMDB: %v", err)
	}
	waitHeld(t, held)
	closeWhileHeld(t, w.Close, release)
	var rec struct {
		CountryCode string `maxminddb:"country_code"`
	}
	if err := w.Lookup("8.8.8.8", &rec); err == nil {
		t.Fatal("Lookup succeeded after Close")
	}
}

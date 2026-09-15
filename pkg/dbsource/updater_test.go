package dbsource

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// TestUpdaterStop_JoinsHeldGETAndRunsOnUpdate holds HTTPGet until Stop, then
// releases. Stop must not return while the GET is held. The in-flight GET still
// calls onUpdate (Sleep is not Close; the wrapper is still live).
func TestUpdaterStop_JoinsHeldGETAndRunsOnUpdate(t *testing.T) {
	body, err := os.ReadFile(repoFile(t, "ipinfo_lite.mmdb"))
	if err != nil {
		t.Fatal(err)
	}
	held := make(chan struct{})
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(held)
		<-release
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	var onUpdateCount atomic.Int32
	u, err := Start(Config{
		Key:          "join",
		URL:          srv.URL + "/db.mmdb",
		DatabaseType: TypeMMDB,
		Dir:          t.TempDir(),
	}, testLogger(), func(string) {
		onUpdateCount.Add(1)
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if u == nil {
		t.Fatal("expected updater")
	}

	select {
	case <-held:
	case <-time.After(5 * time.Second):
		t.Fatal("GET never started")
	}

	stopped := make(chan struct{})
	go func() {
		u.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
		t.Fatal("Stop returned while GET was still held")
	case <-time.After(150 * time.Millisecond):
	}

	close(release)

	select {
	case <-stopped:
	case <-time.After(15 * time.Second):
		t.Fatal("Stop did not join Start's goroutine")
	}

	if n := onUpdateCount.Load(); n != 1 {
		t.Fatalf("in-flight GET onUpdate: got %d, want 1", n)
	}

	u.Stop()
	(*Updater)(nil).Stop()
}

func TestUpdater_ReconcilesLatestWithoutURL(t *testing.T) {
	dir := t.TempDir()
	dated := filepath.Join(dir, "20260915_paid.BIN")
	if err := os.WriteFile(dated, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	got := make(chan string, 1)
	u, err := Start(Config{
		Key:          "paid",
		DatabaseType: TypeBIN,
		Dir:          dir,
	}, testLogger(), func(path string) {
		got <- path
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if u == nil {
		t.Fatal("expected updater without URL")
	}
	select {
	case path := <-got:
		if path != dated {
			t.Fatalf("onUpdate path=%s want %s", path, dated)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("onUpdate never ran")
	}
	u.Stop()
}

func TestUpdater_NoWatchDirReturnsNil(t *testing.T) {
	u, err := Start(Config{Key: "paid", URL: "https://example.com/db.BIN", DatabaseType: TypeBIN}, testLogger(), nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if u != nil {
		t.Fatal("expected nil updater without Dir")
	}
}

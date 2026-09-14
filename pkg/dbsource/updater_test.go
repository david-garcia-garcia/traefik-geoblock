package dbsource

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

// TestUpdaterStop_JoinsHeldGETAndSkipsOnUpdate holds HTTPGet until Stop, then
// releases. Stop must not return while the GET is held, and onUpdate must not run.
func TestUpdaterStop_JoinsHeldGETAndSkipsOnUpdate(t *testing.T) {
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
		t.Fatal("Stop did not join the ticker goroutine")
	}

	if n := onUpdateCount.Load(); n != 0 {
		t.Fatalf("onUpdate called %d times after Stop", n)
	}

	u.Stop()
	(*Updater)(nil).Stop()
}

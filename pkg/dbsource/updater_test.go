package dbsource

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// gatedSource serves one downloadable database and lets a test hold a request open.
type gatedSource struct {
	server   *httptest.Server
	requests atomic.Int32
	// entered is closed when the first request reaches the handler.
	entered chan struct{}
	// release lets every held request finish. Close it to let the download complete.
	release chan struct{}
	once    sync.Once
}

// newGatedSource starts a server whose first request blocks until the test releases it. It
// serves a real database, so a released download reaches the update callback.
func newGatedSource(t *testing.T) *gatedSource {
	t.Helper()
	body, err := os.ReadFile(repoFile(t, "ipinfo_lite.mmdb"))
	if err != nil {
		t.Fatalf("read seed database: %v", err)
	}
	source := &gatedSource{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	source.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		source.requests.Add(1)
		source.once.Do(func() { close(source.entered) })
		<-source.release
		_, _ = w.Write(body)
	}))
	t.Cleanup(func() {
		source.releaseAll()
		source.server.Close()
	})
	return source
}

// releaseAll unblocks every held request.
func (s *gatedSource) releaseAll() {
	select {
	case <-s.release:
	default:
		close(s.release)
	}
}

// url is the downloadable file this source serves.
func (s *gatedSource) url() string { return s.server.URL + "/db.mmdb" }

// gatedUpdater builds an Updater for source that writes into dir.
func gatedUpdater(t *testing.T, source *gatedSource, dir string) *Updater {
	t.Helper()
	updater, err := newUpdater(Config{
		Key:          "probe",
		URL:          source.url(),
		DatabaseType: TypeMMDB,
		Archive:      ArchiveNone,
		Dir:          dir,
		MinAge:       time.Hour,
	}, testLogger())
	if err != nil {
		t.Fatalf("new updater: %v", err)
	}
	return updater
}

// dirEntries is the sorted file names in dir.
func dirEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}

func TestUpdater_StopWaitsForTheUpdateLoop(t *testing.T) {
	dir := t.TempDir()
	source := newGatedSource(t)
	updater := gatedUpdater(t, source, dir)

	updater.Start(nil)
	<-source.entered

	stopped := make(chan struct{})
	go func() {
		updater.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
		t.Fatal("Stop returned while a download was still in flight")
	case <-time.After(80 * time.Millisecond):
	}

	source.releaseAll()
	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		t.Fatal("Stop never returned after the download finished")
	}

	// Nothing may land in the source directory once Stop has returned.
	settled := dirEntries(t, dir)
	time.Sleep(100 * time.Millisecond)
	if later := dirEntries(t, dir); len(later) != len(settled) {
		t.Fatalf("directory changed after Stop returned: %v then %v", settled, later)
	}
}

func TestUpdater_StoppedLoopDoesNotDownload(t *testing.T) {
	dir := t.TempDir()
	source := newGatedSource(t)
	source.releaseAll()
	updater := gatedUpdater(t, source, dir)

	stop := make(chan struct{})
	close(stop)
	var updates atomic.Int32
	updater.tick(stop, func(string) { updates.Add(1) })

	if got := source.requests.Load(); got != 0 {
		t.Fatalf("%d requests from a stopped loop, want 0", got)
	}
	if got := updates.Load(); got != 0 {
		t.Fatalf("%d update callbacks from a stopped loop, want 0", got)
	}
	if names := dirEntries(t, dir); len(names) != 0 {
		t.Fatalf("a stopped loop wrote %v", names)
	}
}

func TestUpdater_RunningTickInvokesTheCallback(t *testing.T) {
	dir := t.TempDir()
	source := newGatedSource(t)
	source.releaseAll()
	updater := gatedUpdater(t, source, dir)

	var updates atomic.Int32
	updater.tick(make(chan struct{}), func(string) { updates.Add(1) })

	// Without this, the stopped-loop tests below would pass on a download that never succeeds.
	if got := updates.Load(); got != 1 {
		t.Fatalf("%d update callbacks from a running loop, want 1", got)
	}
}

func TestUpdater_StopDuringADownloadSkipsTheCallback(t *testing.T) {
	dir := t.TempDir()
	source := newGatedSource(t)
	updater := gatedUpdater(t, source, dir)

	stop := make(chan struct{})
	var updates atomic.Int32
	done := make(chan struct{})
	go func() {
		updater.tick(stop, func(string) { updates.Add(1) })
		close(done)
	}()

	// The request is in flight: this is the window Stop cannot interrupt, so the download
	// finishes and only the second stop check can keep the callback from firing.
	<-source.entered
	close(stop)
	source.releaseAll()
	<-done

	if got := updates.Load(); got != 0 {
		t.Fatalf("%d update callbacks after stop, want 0", got)
	}
}

func TestUpdater_StopThenStartRunsAFreshLoop(t *testing.T) {
	dir := t.TempDir()
	source := newGatedSource(t)
	source.releaseAll()
	updater := gatedUpdater(t, source, dir)

	updater.Start(nil)
	waitRequests(t, source, 1)
	updater.Stop()

	updater.Start(nil)
	waitRequests(t, source, 2)

	// A second Start while the loop runs is ignored, so no third loop appears.
	updater.Start(nil)
	time.Sleep(100 * time.Millisecond)
	if got := source.requests.Load(); got != 2 {
		t.Fatalf("%d requests, want 2: a second Start started another loop", got)
	}
	updater.Stop()
}

func TestUpdater_StoppingTwiceIsSafe(t *testing.T) {
	dir := t.TempDir()
	source := newGatedSource(t)
	source.releaseAll()
	updater := gatedUpdater(t, source, dir)

	updater.Start(nil)
	waitRequests(t, source, 1)
	updater.Stop()
	updater.Stop()

	// Stopping one that never started, and a nil one, are both no-ops.
	fresh := gatedUpdater(t, source, dir)
	fresh.Stop()
	var missing *Updater
	missing.Stop()
}

// waitRequests fails if source has not seen want requests within ten seconds.
func waitRequests(t *testing.T, source *gatedSource, want int32) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if source.requests.Load() >= want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("saw %d requests, want %d", source.requests.Load(), want)
}

package dbwrappers

import (
	"sync"
	"testing"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/dbsource"
)

// TestBIN_LookupRecordVsHotSwap runs LookupRecord and getters while hotSwap publishes a new handle.
func TestBIN_LookupRecordVsHotSwap(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	w, err := OpenBIN(holdCtx(t), BINConfig{Source: dbsource.Config{Path: testBIN}}, testLogger())
	if err != nil {
		t.Fatalf("OpenBIN: %v", err)
	}
	fields := mustFields(t, PresetIP2LocationLite)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			rec, err := w.LookupRecord("8.8.8.8", fields)
			if err != nil {
				t.Errorf("LookupRecord during hotSwap: %v", err)
				return
			}
			if rec.Country != "US" {
				t.Errorf("LookupRecord during hotSwap: %+v", rec)
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = w.Path()
			_ = w.Version()
			_ = w.SourcePath()
		}
	}()

	for i := 0; i < 20; i++ {
		if err := w.life.hotSwap(testBIN, dbsource.TriggerPromote); err != nil {
			t.Fatalf("hotSwap %d: %v", i, err)
		}
	}
	close(stop)
	wg.Wait()

	rec := testBINRecord(t, w)
	if rec.Country != "US" {
		t.Fatalf("after swaps: %+v", rec)
	}
}

// TestBIN_LookupRecordVsClose runs LookupRecord while close Closes the vendor file.
// close does not nil w.db. Concurrent Close vs Get_all is undeterministic; it must not panic.
func TestBIN_LookupRecordVsClose(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	w, err := OpenBIN(holdCtx(t), BINConfig{Source: dbsource.Config{Path: testBIN}}, testLogger())
	if err != nil {
		t.Fatalf("OpenBIN: %v", err)
	}
	fields := mustFields(t, PresetIP2LocationLite)

	started := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		close(started)
		for i := 0; i < 20000; i++ {
			rec, err := w.LookupRecord("8.8.8.8", fields)
			if err != nil {
				continue
			}
			if rec.Country != "US" {
				t.Errorf("LookupRecord vs close: %+v", rec)
				return
			}
		}
	}()
	<-started
	w.life.close()
	wg.Wait()

	_, _ = w.LookupRecord("8.8.8.8", fields)
}

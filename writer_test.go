package traefik_geoblock

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"
)

func TestBufferedFileWriter_ContextCancellation(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "writer-test-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	logger := createBootstrapLogger("test")
	ctx, cancel := context.WithCancel(context.Background())

	initial := runtime.NumGoroutine()

	writer, err := newBufferedFileWriter(ctx, tmpFile.Name(), 1024, 100*time.Millisecond, logger)
	if err != nil {
		t.Fatal(err)
	}

	testData := []byte("test data\n")
	writer.Write(testData)

	afterCreate := runtime.NumGoroutine()
	if afterCreate <= initial {
		t.Error("goroutine should have started")
	}

	cancel()
	time.Sleep(200 * time.Millisecond)

	afterCancel := runtime.NumGoroutine()
	if afterCancel > afterCreate {
		t.Errorf("goroutine should have exited: before=%d after=%d", afterCreate, afterCancel)
	}

	// Verify data was written
	content, _ := os.ReadFile(tmpFile.Name())
	if string(content) != string(testData) {
		t.Errorf("data not flushed correctly")
	}
}

package bufferedwriter

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/david-garcia-garcia/traefik-geoblock/pkg/contextawarefactory"
)

// Writer provides thread-safe buffered file writing with automatic flushing.
//
// Design Overview:
// Uses the generic contextawarefactory.Factory for reference counting and lifecycle management.
// Multiple plugin instances can share the same file writer, preventing duplicate goroutines
// and file handles when multiple middlewares log to the same file.
//
// Key Features:
//   - Writers are shared based on (path, maxSize, timeout) configuration
//   - Automatic reference counting via generic factory
//   - Context-aware: automatically cleans up when plugin contexts are cancelled
//   - Thread-safe buffered writes with automatic timed flushing
//   - One flush timer goroutine per unique configuration (shared by all users)

// Global factory for managing shared buffered file writers
var writerFactory = contextawarefactory.NewFactory(createWriter, cleanupWriter)

// writerKey uniquely identifies a writer by its configuration
type writerKey struct {
	path    string
	maxSize int
	timeout time.Duration
}

// writer is the actual writer with buffer and flush timer
type writer struct {
	mu        sync.Mutex
	file      *os.File
	buffer    []byte
	path      string
	maxSize   int
	timeout   time.Duration
	lastFlush time.Time
	ctx       context.Context    // Writer's own context for the flush timer
	cancel    context.CancelFunc // Cancel function for stopping the flush timer
	closed    bool
	logger    *slog.Logger
}

// createWriter is the creator function used by the factory.
// It creates a new writer instance for the given key.
func createWriter(ctx context.Context, key writerKey) (*writer, error) {
	// Open the file
	file, err := os.OpenFile(key.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	// Create an independent context for this writer's flush timer
	// This context is separate from the factory's context tracking
	writerCtx, cancel := context.WithCancel(context.Background())

	w := &writer{
		file:      file,
		path:      key.path,
		buffer:    make([]byte, 0, key.maxSize),
		maxSize:   key.maxSize,
		timeout:   key.timeout,
		lastFlush: time.Now(),
		ctx:       writerCtx,
		cancel:    cancel,
		closed:    false,
		logger:    nil, // Logger will be set by caller if needed
	}

	// Start the flush timer goroutine
	go w.flushTimer()

	return w, nil
}

// cleanupWriter is the cleanup function used by the factory.
// It's called when the last reference to a writer is released.
func cleanupWriter(w *writer) error {
	if w.logger != nil {
		w.logger.Debug("bufferedFileWriter: writer fully disposed",
			"path", w.path,
			"maxSize", w.maxSize,
			"timeout", w.timeout)
	}

	// Cancel the flush timer context
	w.cancel()

	// Close the writer (flushes buffer and closes file)
	return w.close()
}

// New creates or returns a handle to a shared buffered file writer.
// The ctx parameter is used to track the caller's lifetime - when the context is cancelled,
// the handle is automatically released and the reference count is decremented.
func New(ctx context.Context, path string, maxSize int, timeout time.Duration, logger *slog.Logger) (io.WriteCloser, error) {
	key := writerKey{
		path:    path,
		maxSize: maxSize,
		timeout: timeout,
	}

	handle, err := writerFactory.GetOrCreate(ctx, key)
	if err != nil {
		return nil, err
	}

	// Set the logger on the writer (only matters for first creation, but safe to set always)
	writer := handle.Value()
	writer.logger = logger

	if logger != nil {
		stats := writerFactory.GetStats()
		logger.Debug("bufferedFileWriter: handle created",
			"path", path,
			"maxSize", maxSize,
			"timeout", timeout,
			"totalInstances", stats.InstanceCount,
			"totalReferences", stats.TotalRefCount)
	}

	// Return a wrapper that implements io.WriteCloser
	return &writerWrapper{handle: handle}, nil
}

// writerWrapper wraps the factory handle to implement io.WriteCloser
type writerWrapper struct {
	handle *contextawarefactory.Handle[writerKey, *writer]
}

func (w *writerWrapper) Write(p []byte) (n int, err error) {
	writer := w.handle.Value()
	return writer.write(p)
}

func (w *writerWrapper) Close() error {
	return w.handle.Release()
}

// write is the internal write method for writer
func (w *writer) write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, os.ErrClosed
	}

	w.buffer = append(w.buffer, p...)

	if len(w.buffer) >= w.maxSize {
		if err := w.flushLocked(); err != nil {
			return 0, err
		}
	}

	return len(p), nil
}

// close is the internal close method that actually closes the file
func (w *writer) close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}
	w.closed = true

	if err := w.flushLocked(); err != nil {
		return err
	}

	return w.file.Close()
}

func (w *writer) flushTimer() {
	ticker := time.NewTicker(w.timeout)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.mu.Lock()
			if !w.closed && time.Since(w.lastFlush) >= w.timeout && len(w.buffer) > 0 {
				_ = w.flushLocked()
			}
			w.mu.Unlock()

		case <-w.ctx.Done():
			w.mu.Lock()
			bufferSize := len(w.buffer)
			if !w.closed && bufferSize > 0 {
				_ = w.flushLocked()
			}
			w.mu.Unlock()

			if w.logger != nil {
				w.logger.Debug("bufferedFileWriter: flush timer goroutine exiting due to context cancellation",
					"path", w.path,
					"reason", w.ctx.Err(),
					"final_buffer_size", bufferSize)
			}
			return
		}
	}
}

func (w *writer) flushLocked() error {
	if len(w.buffer) == 0 {
		return nil
	}

	_, err := w.file.Write(w.buffer)
	if err != nil {
		return err
	}

	w.buffer = w.buffer[:0]
	w.lastFlush = time.Now()
	return nil
}

// GetFactoryStats returns statistics about the global writer factory (for testing/monitoring)
func GetFactoryStats() contextawarefactory.Stats {
	return writerFactory.GetStats()
}

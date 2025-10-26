package traefik_geoblock

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"
)

type bufferedFileWriter struct {
	mu        sync.Mutex
	file      *os.File
	buffer    []byte
	path      string
	maxSize   int
	timeout   time.Duration
	lastFlush time.Time
	ctx       context.Context
	closed    bool
	logger    *slog.Logger
}

func newBufferedFileWriter(ctx context.Context, path string, maxSize int, timeout time.Duration, logger *slog.Logger) (*bufferedFileWriter, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	w := &bufferedFileWriter{
		file:      file,
		path:      path,
		buffer:    make([]byte, 0, maxSize),
		maxSize:   maxSize,
		timeout:   timeout,
		lastFlush: time.Now(),
		ctx:       ctx,
		closed:    false,
		logger:    logger,
	}

	if logger != nil {
		logger.Debug("bufferedFileWriter: starting flush timer goroutine", "path", path)
	}
	go w.flushTimer()

	return w, nil
}

func (w *bufferedFileWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buffer = append(w.buffer, p...)

	if len(w.buffer) >= w.maxSize {
		if err := w.flushLocked(); err != nil {
			return 0, err
		}
	}

	return len(p), nil
}

func (w *bufferedFileWriter) Close() error {
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

func (w *bufferedFileWriter) flushTimer() {
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

func (w *bufferedFileWriter) flushLocked() error {
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

package traefik_geoblock

import (
	"context"
	"log/slog"
	"os"
	"time"
)

type bufferedFileWriter struct {
	file      *os.File
	path      string
	timeout   time.Duration
	ctx       context.Context
	logger    *slog.Logger
	writeChan chan []byte // Channel for writes (lock-free fast path)
}

func newBufferedFileWriter(ctx context.Context, path string, maxSize int, timeout time.Duration, logger *slog.Logger) (*bufferedFileWriter, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	w := &bufferedFileWriter{
		file:      file,
		path:      path,
		timeout:   timeout,
		ctx:       ctx,
		logger:    logger,
		writeChan: make(chan []byte, 1000), // Buffered channel for high throughput
	}

	if logger != nil {
		logger.Debug("bufferedFileWriter: starting flush timer goroutine", "path", path)
	}
	go w.flushTimer(maxSize)

	return w, nil
}

func (w *bufferedFileWriter) Write(p []byte) (n int, err error) {
	// Make a copy since caller might reuse the slice
	data := make([]byte, len(p))
	copy(data, p)

	select {
	case w.writeChan <- data:
		return len(p), nil
	case <-w.ctx.Done():
		return 0, w.ctx.Err()
	}
}

func (w *bufferedFileWriter) Close() error {
	close(w.writeChan)
	return w.file.Close()
}

func (w *bufferedFileWriter) flushTimer(maxSize int) {
	ticker := time.NewTicker(w.timeout)
	defer ticker.Stop()

	buffer := make([]byte, 0, maxSize)

	flush := func() {
		if len(buffer) > 0 {
			_, _ = w.file.Write(buffer)
			buffer = buffer[:0]
		}
	}

	for {
		select {
		case data, ok := <-w.writeChan:
			if !ok {
				// Channel closed, flush and exit
				flush()
				return
			}

			buffer = append(buffer, data...)

			// Flush if buffer is full
			if len(buffer) >= maxSize {
				flush()
			}

		case <-ticker.C:
			// Periodic flush
			flush()

		case <-w.ctx.Done():
			// Context cancelled - drain remaining writes from channel
			draining := true
			for draining {
				select {
				case data, ok := <-w.writeChan:
					if !ok {
						draining = false
					} else {
						buffer = append(buffer, data...)
					}
				default:
					draining = false
				}
			}

			bufferSize := len(buffer)
			flush()

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

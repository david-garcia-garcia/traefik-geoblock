package dbutils

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	HTTPGetTimeout  = 30 * time.Minute
	HTTPGetMaxBytes = 6 * 1024 * 1024 * 1024
)

// HTTPGet GETs rawURL with optional headers. It follows redirects.
// Errors and DownloadHint must not include the URL (tokens may be in the query).
func HTTPGet(rawURL string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("download request: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{Timeout: HTTPGetTimeout}
	resp, err := client.Do(req) // #nosec G107
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	return resp, nil
}

// LimitBody caps a download body at HTTPGetMaxBytes.
func LimitBody(r io.Reader) io.Reader {
	return io.LimitReader(r, HTTPGetMaxBytes)
}

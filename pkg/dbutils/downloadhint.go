package dbutils

import (
	"fmt"
	"io"
	"os"
	"unicode/utf8"
)

const DownloadHintPrefixBytes = 64

// ReadPrefix reads up to n bytes from r for a download error hint.
func ReadPrefix(r io.Reader, n int) string {
	if r == nil || n <= 0 {
		return ""
	}
	buf := make([]byte, n)
	got, _ := io.ReadFull(r, buf)
	if got <= 0 {
		return ""
	}
	return printablePrefix(buf[:got])
}

// DownloadHint describes a failed GET without the URL (tokens may be in the query).
func DownloadHint(key, status, contentType string, contentLength int64, prefix string) string {
	if len(prefix) > DownloadHintPrefixBytes {
		prefix = prefix[:DownloadHintPrefixBytes]
	}
	return fmt.Sprintf("key=%s status=%s content_type=%s content_length=%d prefix=%q",
		key, status, contentType, contentLength, prefix)
}

// DownloadHintFromFile reads a prefix from a saved body for a download error hint.
func DownloadHintFromFile(key, status, contentType, path string) string {
	fi, err := os.Stat(path)
	length := int64(-1)
	if err == nil {
		length = fi.Size()
	}
	f, err := os.Open(path)
	if err != nil {
		return DownloadHint(key, status, contentType, length, "")
	}
	defer f.Close()
	return DownloadHint(key, status, contentType, length, ReadPrefix(f, DownloadHintPrefixBytes))
}

func printablePrefix(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	return fmt.Sprintf("%x", b)
}

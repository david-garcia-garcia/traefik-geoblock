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
func DownloadHint(slot, status, contentType string, contentLength int64, prefix string) string {
	return fmt.Sprintf("slot=%s status=%s content_type=%s content_length=%d prefix=%q",
		slot, status, contentType, contentLength, prefix)
}

// DownloadHintFromFile reads a prefix from a saved body for a download error hint.
func DownloadHintFromFile(slot, status, contentType, path string) string {
	fi, err := os.Stat(path)
	length := int64(-1)
	if err == nil {
		length = fi.Size()
	}
	f, err := os.Open(path)
	if err != nil {
		return DownloadHint(slot, status, contentType, length, "")
	}
	defer f.Close()
	return DownloadHint(slot, status, contentType, length, ReadPrefix(f, DownloadHintPrefixBytes))
}

func printablePrefix(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	return fmt.Sprintf("%x", b)
}

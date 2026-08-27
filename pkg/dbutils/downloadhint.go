package dbutils

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
)

// DownloadHintPrefixBytes is how much of a rejected download body to quote in logs.
const DownloadHintPrefixBytes = 80

// DownloadHint describes a rejected vendor download. Do not pass the URL
// (tokens live in the query or Basic Auth).
func DownloadHint(code, status, contentType string, size int64, prefix []byte) string {
	if len(prefix) > DownloadHintPrefixBytes {
		prefix = prefix[:DownloadHintPrefixBytes]
	}
	return fmt.Sprintf("file=%s status=%s content_type=%q bytes=%d prefix=%s",
		code, status, contentType, size, strconv.Quote(string(prefix)))
}

// DownloadHintFromFile is DownloadHint using the saved body on disk.
func DownloadHintFromFile(code, status, contentType, path string) string {
	return DownloadHint(code, status, contentType, fileSize(path), ReadFilePrefix(path, DownloadHintPrefixBytes))
}

// ReadPrefix returns up to n bytes from r.
func ReadPrefix(r io.Reader, n int) []byte {
	if r == nil || n <= 0 {
		return nil
	}
	buf := make([]byte, n)
	got, _ := io.ReadFull(r, buf)
	if got <= 0 {
		return nil
	}
	return buf[:got]
}

// ReadFilePrefix returns up to n bytes from path.
func ReadFilePrefix(path string, n int) []byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	return ReadPrefix(f, n)
}

// TeePrefix returns a reader equivalent to r and the first n bytes so a later
// format error can quote the payload without a second fetch.
func TeePrefix(r io.Reader, n int) (io.Reader, []byte) {
	prefix := ReadPrefix(r, n)
	if len(prefix) == 0 {
		return r, nil
	}
	return io.MultiReader(bytes.NewReader(prefix), r), prefix
}

func fileSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return -1
	}
	return fi.Size()
}

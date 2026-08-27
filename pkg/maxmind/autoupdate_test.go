package maxmind

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractMMDB(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte("not-a-real-mmdb")
	hdr := &tar.Header{Name: "GeoLite2-Country_20260101/GeoLite2-Country.mmdb", Mode: 0644, Size: int64(len(body))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	out, err := extractMMDB(bytes.NewReader(buf.Bytes()), dir)
	if err != nil {
		t.Fatalf("extractMMDB: %v", err)
	}
	if filepath.Base(out) != "GeoLite2-Country.mmdb" {
		t.Errorf("extracted name: %s", out)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("extracted bytes mismatch")
	}
}

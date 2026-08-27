package dbutils

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var dbFilePath = filepath.Join("..", "..", "IP2LOCATION-LITE-DB1.IPV6.BIN")

func TestGetDateFromName(t *testing.T) {
	tests := []struct {
		name    string
		dbPath  string
		want    time.Time
		wantErr bool
	}{
		{
			name:    "valid filename",
			dbPath:  "20240315_IP2LOCATION-LITE-DB1.BIN",
			want:    time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name:    "valid filename with path",
			dbPath:  "/path/to/20240315_IP2LOCATION-LITE-DB1.BIN",
			want:    time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name:    "valid filename with windows path",
			dbPath:  `C:\path\to\20240315_IP2LOCATION-LITE-DB1.BIN`,
			want:    time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name:    "invalid date format",
			dbPath:  "invalid_IP2LOCATION-LITE-DB1.BIN",
			want:    time.Time{},
			wantErr: true,
		},
		{
			name:    "empty string",
			dbPath:  "",
			want:    time.Time{},
			wantErr: true,
		},
		{
			name:    "invalid year",
			dbPath:  "abcd0315_IP2LOCATION-LITE-DB1.BIN",
			want:    time.Time{},
			wantErr: true,
		},
		{
			name:    "invalid month",
			dbPath:  "2024xx15_IP2LOCATION-LITE-DB1.BIN",
			want:    time.Time{},
			wantErr: true,
		},
		{
			name:    "invalid day",
			dbPath:  "202403xx_IP2LOCATION-LITE-DB1.BIN",
			want:    time.Time{},
			wantErr: true,
		},
		{
			name:    "too short date",
			dbPath:  "2024_IP2LOCATION-LITE-DB1.BIN",
			want:    time.Time{},
			wantErr: true,
		},
		{
			name:    "no underscore",
			dbPath:  "20240315IP2LOCATION-LITE-DB1.BIN",
			want:    time.Time{},
			wantErr: true,
		},
		{
			name:    "leap year date",
			dbPath:  "20240229_IP2LOCATION-LITE-DB1.BIN",
			want:    time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name:    "last day of year",
			dbPath:  "20241231_IP2LOCATION-LITE-DB1.BIN",
			want:    time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetDateFromName(tt.dbPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDateFromName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !got.Equal(tt.want) {
				t.Errorf("GetDateFromName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMMDBBuildDate(t *testing.T) {
	if _, err := MMDBBuildDate(0); err == nil {
		t.Fatal("expected error for zero epoch")
	}
	got, err := MMDBBuildDate(1)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(time.Unix(1, 0).UTC()) {
		t.Errorf("got %v", got)
	}
}

func TestFindLatestDatedFile(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"20230101_ipinfo_lite.mmdb", "20230301_ipinfo_lite.mmdb", "20230401_other.mmdb", "skip.mmdb"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	latest, err := FindLatestDatedFile(dir, "*_ipinfo_lite.mmdb")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(latest, "20230301_ipinfo_lite.mmdb") {
		t.Errorf("latest: %s", latest)
	}
}

func TestGetDBVersion(t *testing.T) {
	// Test successful case
	version, err := GetDatabaseVersion(dbFilePath)
	if err != nil {
		t.Errorf("Expected no error for valid database, got: %v", err)
	}
	if version.Month != 4 {
		t.Errorf("Expected month 4, got: %d", version.Month)
	}

	// Test error case
	version, err = GetDatabaseVersion("20240315_IP2LOCATION-LITE-INVALID.BIN")
	if err == nil {
		t.Error("Expected error for invalid database, got nil")
	}
	if version != nil {
		t.Error("Expected nil version for invalid database")
	}
}

func TestDownloadHint_HTMLErrorPage(t *testing.T) {
	html := []byte("<html><title>NO PERMISSION</title></html>")
	got := DownloadHint("DB8BINIPV6", "200 OK", "text/html", int64(len(html)), html)
	for _, want := range []string{
		"file=DB8BINIPV6",
		"status=200 OK",
		`content_type="text/html"`,
		"NO PERMISSION",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("hint %q missing %q", got, want)
		}
	}
	if strings.Contains(got, "token=") || strings.Contains(got, "http://") || strings.Contains(got, "https://") {
		t.Errorf("hint must not include URL or token: %s", got)
	}
}

func TestDownloadHint_TruncatesPrefix(t *testing.T) {
	prefix := bytes.Repeat([]byte("A"), DownloadHintPrefixBytes+20)
	got := DownloadHint("DB1", "200 OK", "application/octet-stream", int64(len(prefix)), prefix)
	if strings.Count(got, "A") != DownloadHintPrefixBytes {
		t.Errorf("prefix A count=%d, want %d in %q", strings.Count(got, "A"), DownloadHintPrefixBytes, got)
	}
}

func TestDownloadHintFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "body")
	if err := os.WriteFile(path, []byte("<!DOCTYPE html>denied"), 0600); err != nil {
		t.Fatal(err)
	}
	got := DownloadHintFromFile("GeoLite2-Country", "200 OK", "text/html", path)
	if !strings.Contains(got, "denied") || !strings.Contains(got, "bytes=21") {
		t.Errorf("hint=%q", got)
	}
	if fileSize(filepath.Join(t.TempDir(), "missing")) != -1 {
		t.Error("missing file size should be -1")
	}
}

func TestTeePrefix(t *testing.T) {
	body := bytes.NewBufferString("PK\x03\x04rest-of-zip")
	r, prefix := TeePrefix(body, 4)
	if string(prefix) != "PK\x03\x04" {
		t.Fatalf("prefix=%q", prefix)
	}
	all, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(all) != "PK\x03\x04rest-of-zip" {
		t.Errorf("replay=%q", all)
	}
}

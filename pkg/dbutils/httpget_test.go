package dbutils

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPGet_StatusHeadersQuery(t *testing.T) {
	var gotAuth, gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotToken = r.URL.Query().Get("token")
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("body"))
	}))
	t.Cleanup(srv.Close)

	resp, err := HTTPGet(srv.URL+"?token=secret", map[string]string{"Authorization": "Bearer abc"})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: %s", resp.Status)
	}
	if gotAuth != "Bearer abc" {
		t.Errorf("Authorization: %q", gotAuth)
	}
	if gotToken != "secret" {
		t.Errorf("token query: %q", gotToken)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "body" {
		t.Errorf("body: %q", b)
	}
}

func TestHTTPGet_HintOmitsURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("nope"))
	}))
	t.Cleanup(srv.Close)

	resp, err := HTTPGet(srv.URL+"?token=secret", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	hint := DownloadHint("geo", resp.Status, resp.Header.Get("Content-Type"), resp.ContentLength, ReadPrefix(resp.Body, DownloadHintPrefixBytes))
	if strings.Contains(hint, srv.URL) || strings.Contains(hint, "token=secret") {
		t.Errorf("hint leaked URL: %s", hint)
	}
}

func TestDatedKeyGlob(t *testing.T) {
	if DatedKeyGlob("lite", ".mmdb") != "*_lite.mmdb" {
		t.Errorf("glob: %s", DatedKeyGlob("lite", ".mmdb"))
	}
}

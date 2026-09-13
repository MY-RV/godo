package update_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/my-rv/godo/internal/update"
)

func TestNewer(t *testing.T) {
	ok, err := update.Newer("0.1.0-dev", "v0.1.1")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	ok, err = update.Newer("v0.2.0", "v0.1.9")
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	ok, err = update.Newer("v0.1.0", "0.1.0")
	if err != nil || ok {
		t.Fatalf("same should not be newer: %v %v", ok, err)
	}
}

func TestClient_Latest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/releases/latest" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "tag_name": "v0.2.0",
  "assets": [
    {"name": "godo_0.2.0_darwin_arm64", "browser_download_url": "http://example.com/godo"}
  ]
}`))
	}))
	defer srv.Close()

	c := &update.Client{HTTP: srv.Client(), APIBase: srv.URL, GOOS: "darwin", GOARCH: "arm64"}
	tag, url, err := c.Latest()
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v0.2.0" || !strings.Contains(url, "example.com") {
		t.Fatalf("%q %q", tag, url)
	}
}

func TestClient_LatestNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := &update.Client{HTTP: srv.Client(), APIBase: srv.URL}
	_, _, err := c.Latest()
	if err == nil || !strings.Contains(err.Error(), "no GitHub releases") {
		t.Fatalf("%v", err)
	}
}

func TestDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("#!/bin/sh\necho ok\n"))
	}))
	defer srv.Close()
	dir := t.TempDir()
	dest := filepath.Join(dir, "godo")
	if err := update.Download(srv.Client(), srv.URL, dest); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(dest)
	if err != nil || !strings.Contains(string(data), "echo ok") {
		t.Fatalf("%q %v", data, err)
	}
}

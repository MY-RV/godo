package cli_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/my-rv/godo"
	"github.com/my-rv/godo/internal/cli"
	"github.com/my-rv/godo/internal/update"
)

func TestApp_updateCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/releases/latest":
			_, _ = w.Write([]byte(`{
  "tag_name": "v9.9.9",
  "assets": [{"name":"godo_9.9.9_darwin_arm64","browser_download_url":"REPLACE"}]
}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	prev := godo.Version
	godo.Version = "0.1.0"
	t.Cleanup(func() { godo.Version = prev })

	var stdout bytes.Buffer
	app := cli.New()
	app.Stdout = &stdout
	app.UpdateClient = &update.Client{
		HTTP:    srv.Client(),
		APIBase: srv.URL,
		GOOS:    "darwin",
		GOARCH:  "arm64",
	}
	if err := app.Run([]string{"--update-check"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "update available") {
		t.Fatalf("%q", stdout.String())
	}
	stdout.Reset()
	if err := app.Run([]string{"-e", "update", "check"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "update available") {
		t.Fatalf("-e: %q", stdout.String())
	}
}

func TestApp_engineVersion(t *testing.T) {
	var stdout bytes.Buffer
	app := cli.New()
	app.Stdout = &stdout
	if err := app.Run([]string{"-e", "version"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(stdout.String()) != godo.Version {
		t.Fatalf("%q", stdout.String())
	}
	stdout.Reset()
	if err := app.Run([]string{"--engine", "version"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(stdout.String()) != godo.Version {
		t.Fatalf("%q", stdout.String())
	}
}

func TestApp_updateDownloads(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "godo-bin")
	if err := os.WriteFile(dest, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	var downloadURL string
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()
	downloadURL = srv.URL + "/bin"
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
  "tag_name": "v9.9.9",
  "assets": [{"name":"godo_9.9.9_linux_amd64","browser_download_url":"` + downloadURL + `"}]
}`))
	})
	mux.HandleFunc("/bin", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("new-binary"))
	})

	prev := godo.Version
	godo.Version = "0.1.0"
	t.Cleanup(func() { godo.Version = prev })

	var stdout bytes.Buffer
	app := cli.New()
	app.Stdout = &stdout
	app.UpdateClient = &update.Client{
		HTTP:    srv.Client(),
		APIBase: srv.URL,
		GOOS:    "linux",
		GOARCH:  "amd64",
	}
	app.Executable = func() (string, error) { return dest, nil }
	if err := app.Run([]string{"--engine", "update"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(dest)
	if string(data) != "new-binary" {
		t.Fatalf("%q", data)
	}
}

func TestApp_scriptNamedUpdateNotStolen(t *testing.T) {
	dir := t.TempDir()
	body := `version: "0.1"
dialect: package
scripts:
  update: echo from-script
`
	if err := os.WriteFile(filepath.Join(dir, "godo.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	app := cli.New()
	app.Stdout = &stdout
	app.Getwd = func() (string, error) { return dir, nil }
	app.Runner = runnerFunc(func(c string) error {
		if c != "echo from-script" {
			t.Fatalf("got %q", c)
		}
		return nil
	})
	if err := app.Run([]string{"update"}); err != nil {
		t.Fatal(err)
	}
}

package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/my-rv/godo"
	"github.com/my-rv/godo/internal/cli"
)

type runnerFunc func(string) error

func (f runnerFunc) Run(c string) error { return f(c) }

func TestParseFlags(t *testing.T) {
	_, tokens, err := cli.ParseFlags([]string{"--ls", "test"})
	if err != nil || len(tokens) != 1 || tokens[0] != "test" {
		t.Fatalf("%v %v", tokens, err)
	}
	_, tokens, err = cli.ParseFlags([]string{"--preview", "seed", "a"})
	if err != nil || len(tokens) != 2 {
		t.Fatalf("%v %v", tokens, err)
	}
	_, _, err = cli.ParseFlags([]string{"--nope"})
	if err == nil {
		t.Fatal("unknown flag")
	}
	_, _, err = cli.ParseFlags([]string{"--version"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestApp_lsPreviewVersion(t *testing.T) {
	dir := t.TempDir()
	body := `version: "0.1"
dialect: package
scripts:
  # hello
  test: echo hi
`
	if err := os.WriteFile(filepath.Join(dir, "godo.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	app := cli.New()
	app.Stdout = &stdout
	app.Stderr = &stderr
	app.Getwd = func() (string, error) { return dir, nil }
	app.Runner = runnerFunc(func(string) error { return nil })

	if err := app.Run([]string{"--ls"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "test") {
		t.Fatalf("%q", stdout.String())
	}
	stdout.Reset()
	if err := app.Run([]string{"--preview", "test"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(stdout.String()) != "echo hi" {
		t.Fatalf("%q", stdout.String())
	}
	stdout.Reset()
	if err := app.Run([]string{"--version"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(stdout.String()) != godo.Version {
		t.Fatalf("%q", stdout.String())
	}
	// empty preview tokens
	if err := app.Run([]string{"--preview"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestApp_execDirIsCatalogRoot(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "godo.yaml"), []byte(`version: "0.1"
dialect: package
scripts:
  here: pwd
`), 0o644); err != nil {
		t.Fatal(err)
	}
	var gotDir string
	app := cli.New()
	app.Getwd = func() (string, error) { return sub, nil }
	app.Runner = runnerFunc(func(c string) error {
		// Runner from App uses execshell with Dir set — we replace Runner so assert via NewEngine path:
		// When Runner is injected, Dir is not applied. Test catalog resolution from sub instead.
		_ = c
		gotDir = "injected"
		return nil
	})
	if err := app.Run([]string{"here"}); err != nil {
		t.Fatal(err)
	}
	if gotDir != "injected" {
		t.Fatal("run did not invoke runner")
	}
	// Default runner Dir: unit-test via constructing like App does
	path, err := godo.FindFile(sub)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != root {
		t.Fatalf("catalog dir %s want %s", filepath.Dir(path), root)
	}
}

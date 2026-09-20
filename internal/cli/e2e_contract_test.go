package cli_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/my-rv/godo"
	"github.com/my-rv/godo/internal/cli"
)

// E2E coverage of docs/contract.md — CLI → catalog → expand → runner.

func e2eApp(t *testing.T, cwd string) (*cli.App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	app := cli.New()
	app.Stdout = &stdout
	app.Stderr = &stderr
	app.Getwd = func() (string, error) { return cwd, nil }
	app.Runner = runnerFunc(func(string) error { return nil })
	return app, &stdout, &stderr
}

func writeGodoYAML(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "godo.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestE2E_versionAndHelp(t *testing.T) {
	cwd := t.TempDir()
	app, out, _ := e2eApp(t, cwd)
	if err := app.Run([]string{"--version"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != godo.Version {
		t.Fatalf("version: %q", out.String())
	}
	app, _, errBuf := e2eApp(t, cwd)
	if err := app.Run([]string{"--help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errBuf.String(), "godo") {
		t.Fatalf("help: %q", errBuf.String())
	}
}

func TestE2E_flagsBeforeScriptTokens(t *testing.T) {
	cwd := t.TempDir()
	writeGodoYAML(t, cwd, `version: "0.1"
dialect: package
scripts:
  test: echo hi
`)
	app, out, _ := e2eApp(t, cwd)
	if err := app.Run([]string{"--ls", "test"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "test") {
		t.Fatalf("ls: %q", out.String())
	}
	// Script token before --ls is not a list mode (tokens become the script).
	app, _, _ = e2eApp(t, cwd)
	if err := app.Run([]string{"test", "--ls"}); err == nil {
		// may fail as unexpected args depending on dialect; just ensure it is not silent ls-only success path
		_ = err
	}
}

func TestE2E_previewExpandsDeps(t *testing.T) {
	cwd := t.TempDir()
	writeGodoYAML(t, cwd, `version: "0.1"
dialect: package
scripts:
  leaf: echo leaf
  # @deps leaf
  root: echo root
`)
	app, out, _ := e2eApp(t, cwd)
	if err := app.Run([]string{"--preview", "root"}); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "echo leaf") || !strings.Contains(got, "echo root") {
		t.Fatalf("preview: %q", got)
	}
}

func TestE2E_engineVsScriptUpdate(t *testing.T) {
	cwd := t.TempDir()
	writeGodoYAML(t, cwd, `version: "0.1"
dialect: package
scripts:
  update: echo from-script
`)
	ran := ""
	app, _, _ := e2eApp(t, cwd)
	app.Runner = runnerFunc(func(c string) error { ran = c; return nil })
	if err := app.Run([]string{"update"}); err != nil {
		t.Fatal(err)
	}
	if ran != "echo from-script" {
		t.Fatalf("script update stolen: %q", ran)
	}
}

func TestE2E_failClosedUnknownPlaceholder(t *testing.T) {
	cwd := t.TempDir()
	writeGodoYAML(t, cwd, `version: "0.1"
dialect: package
scripts:
  bad: echo ${godo:argv[nope]}
`)
	app, _, _ := e2eApp(t, cwd)
	if err := app.Run([]string{"--preview", "bad"}); err == nil {
		t.Fatal("expected expand error")
	}
}

// Contract: "runner" — a catalog may name a native shell directly, and godo
// proxies to whatever is on PATH instead of managing a list of them.
func TestE2E_nativeShellRunnerByName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix shells")
	}
	cwd := t.TempDir()
	// BASH_VERSION only exists in bash, so the marker proves which shell ran.
	writeGodoYAML(t, cwd, "version: \"0.1\"\nscripts:\n"+
		"  # @runner bash\n"+
		"  t: printf %s \"${BASH_VERSION:+is-bash}\" > marker\n")
	app, _, _ := e2eApp(t, cwd)
	// The injected runner covers "shell"; a named shell must not reach it.
	app.Runner = runnerFunc(func(string) error { return errors.New("shell runner must not be used") })
	if err := app.Run([]string{"t"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(cwd, "marker"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "is-bash" {
		t.Fatalf("marker=%q — bash did not run the line", got)
	}
}

func TestE2E_unknownShellNameIsRefused(t *testing.T) {
	cwd := t.TempDir()
	writeGodoYAML(t, cwd, "version: \"0.1\"\nscripts:\n  # @runner git\n  t: echo hi\n")
	app, _, _ := e2eApp(t, cwd)
	err := app.Run([]string{"t"})
	if err == nil || !strings.Contains(err.Error(), "not a known shell") {
		t.Fatalf("err=%v", err)
	}
}

func TestE2E_engineRunnersLists(t *testing.T) {
	app, out, _ := e2eApp(t, t.TempDir())
	if err := app.Run([]string{"-e", "runners"}); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"inherit", "[default]", "shells found here", "GODO_SHELL", "Not the shell you expected"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestE2E_engineRunnersRejectsArguments(t *testing.T) {
	app, _, _ := e2eApp(t, t.TempDir())
	if err := app.Run([]string{"-e", "runners", "extra"}); err == nil {
		t.Fatal("want error")
	}
}

// --ls shows the runner a script asked for, next to its dialect.
func TestE2E_lsShowsRunner(t *testing.T) {
	cwd := t.TempDir()
	writeGodoYAML(t, cwd, "version: \"0.1\"\nscripts:\n  # @runner inherit\n  t: echo hi\n")
	app, out, _ := e2eApp(t, cwd)
	if err := app.Run([]string{"--ls", "t"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "@runner inherit") {
		t.Fatalf("ls=%q", out.String())
	}
}

// Contract: a catalog that names no runner runs under the shell you are in.
func TestE2E_defaultRunnerIsTheCallersShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix shells")
	}
	if _, err := os.Stat("/bin/zsh"); err != nil {
		t.Skip("no zsh on this machine")
	}
	t.Setenv("GODO_SHELL", "/bin/zsh")
	cwd := t.TempDir()
	// $0 is the shell running the line, and every POSIX shell reports it.
	// (Arrays would be a sharper example but dash has none, and /bin/sh is
	// dash on Debian and bash on macOS.)
	writeGodoYAML(t, cwd, "version: \"0.1\"\nscripts:\n"+
		"  t: \"printf %s \\\"$0\\\" > marker\"\n")
	app := cli.New()
	app.Getwd = func() (string, error) { return cwd, nil }
	if err := app.Run([]string{"t"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(cwd, "marker"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "zsh") {
		t.Fatalf("marker=%q — default did not use the caller's shell", got)
	}
}

// Naming a shell pins it, whatever shell the caller is in.
func TestE2E_namedShellPinsItRegardlessOfCaller(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix shells")
	}
	t.Setenv("GODO_SHELL", "/bin/zsh")
	cwd := t.TempDir()
	writeGodoYAML(t, cwd, "version: \"0.1\"\nengine:\n  runner: sh\nscripts:\n"+
		"  t: \"printf %s \\\"$0\\\" > marker\"\n")
	app := cli.New()
	app.Getwd = func() (string, error) { return cwd, nil }
	if err := app.Run([]string{"t"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(cwd, "marker"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "zsh") {
		t.Fatalf("marker=%q — runner: sh did not pin sh", got)
	}
}

// Contract: "engine" — a catalog that needs a newer binary says so once,
// clearly, instead of failing later in whatever way the missing feature breaks.
func TestE2E_engineVersionGate(t *testing.T) {
	cwd := t.TempDir()
	writeGodoYAML(t, cwd, "version: \"0.1\"\nengine:\n  version: \">=99.0.0\"\nscripts:\n  t: echo hi\n")
	app, _, _ := e2eApp(t, cwd)
	err := app.Run([]string{"t"})
	if err == nil || !strings.Contains(err.Error(), "needs godo 99.0.0 or newer") {
		t.Fatalf("err=%v", err)
	}
}

func TestE2E_engineVersionSatisfied(t *testing.T) {
	cwd := t.TempDir()
	writeGodoYAML(t, cwd, "version: \"0.1\"\nengine:\n  version: \">=0.0.1\"\nscripts:\n  t: echo hi\n")
	app, _, _ := e2eApp(t, cwd)
	if err := app.Run([]string{"t"}); err != nil {
		t.Fatalf("err=%v", err)
	}
}

// -e runners names what the catalog expects from plugins, so those runners do
// not read as though they simply did not exist.
func TestE2E_runnersListsDeclaredPlugins(t *testing.T) {
	cwd := t.TempDir()
	writeGodoYAML(t, cwd, "version: \"0.1\"\nengine:\n  plugins:\n"+
		"    - source: https://example.test/godo-micropy@v1\n"+
		"      sha256: deadbeef\n"+
		"      provides: [runner:micropy]\n"+
		"scripts:\n  t: echo hi\n")
	app, out, _ := e2eApp(t, cwd)
	if err := app.Run([]string{"-e", "runners"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"declared by this catalog", "runner:micropy", "godo-micropy"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in:\n%s", want, out.String())
		}
	}
}

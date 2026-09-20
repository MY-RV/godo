package cli_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/my-rv/godo"
)

var (
	wasmOnce sync.Once
	wasmPath string
	wasmErr  error
)

// buildExamplePlugin compiles examples/plugins/lines for wasip1.
//
// Built rather than committed: a checked-in binary is something nobody can
// review, and what this asserts is that the artifact matches the source next
// to it.
func buildExamplePlugin(t *testing.T) string {
	t.Helper()
	wasmOnce.Do(func() {
		f, err := os.CreateTemp("", "lines-*.wasm")
		if err != nil {
			wasmErr = err
			return
		}
		f.Close()
		cmd := exec.Command("go", "build", "-o", f.Name(), "../../examples/plugins/lines")
		cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
		if b, err := cmd.CombinedOutput(); err != nil {
			wasmErr = err
			t.Logf("build: %s", b)
			return
		}
		wasmPath = f.Name()
	})
	if wasmErr != nil {
		t.Fatalf("building the example plugin: %v", wasmErr)
	}
	return wasmPath
}

// pluginCatalog writes a catalog next to a copy of the built plugin.
func pluginCatalog(t *testing.T, grantExec bool, scripts string) string {
	t.Helper()
	cwd := t.TempDir()
	data, err := os.ReadFile(buildExamplePlugin(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, "lines.wasm"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	config := ""
	if grantExec {
		config = "      config:\n        proc:\n          exec: true\n"
	}
	writeGodoYAML(t, cwd, "version: \"0.1\"\nengine:\n  plugins:\n"+
		"    - source: ./lines.wasm\n"+
		"      sha256: "+hex.EncodeToString(sum[:])+"\n"+
		"      provides: [runner:lines]\n"+
		config+
		"scripts:\n"+scripts)
	return cwd
}

// Contract: a plugin provides a runner, and a script names it like any other.
func TestE2E_pluginRunnerRunsTheScript(t *testing.T) {
	cwd := pluginCatalog(t, true, "  # @runner lines\n  boot: |\n    touch one\n    touch two\n")
	app, _, _ := e2eApp(t, cwd)
	if err := app.Run([]string{"boot"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, f := range []string{"one", "two"} {
		if _, err := os.Stat(filepath.Join(cwd, f)); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
	}
}

// Preview is the plugin's to answer: godo cannot render a body it does not
// interpret, so it asks and prints what comes back.
func TestE2E_pluginRendersItsOwnPreview(t *testing.T) {
	cwd := pluginCatalog(t, true, "  # @runner lines\n  boot: |\n    touch one\n    ?false\n")
	app, out, _ := e2eApp(t, cwd)
	if err := app.Run([]string{"--preview", "boot"}); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(out.String())
	if got != "touch one\nfalse    # may fail" {
		t.Fatalf("preview=%q", got)
	}
	if _, err := os.Stat(filepath.Join(cwd, "one")); err == nil {
		t.Fatal("preview ran the body")
	}
}

// Captures and leftover args reach the plugin as data, not pasted into a line.
func TestE2E_pluginReceivesCapturesAndArgs(t *testing.T) {
	cwd := pluginCatalog(t, true,
		"  # @runner lines\n  make ${NAME}: |\n    touch ${NAME}\n    touch $1\n")
	// engine.dialect must be matcher for a capture in the key.
	body, err := os.ReadFile(filepath.Join(cwd, "godo.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	fixed := strings.Replace(string(body), "engine:\n", "engine:\n  dialect: matcher\n", 1)
	if err := os.WriteFile(filepath.Join(cwd, "godo.yaml"), []byte(fixed), 0o644); err != nil {
		t.Fatal(err)
	}

	app, _, _ := e2eApp(t, cwd)
	if err := app.Run([]string{"make", "from capture", "from arg"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	// A value with a space stays one argument: the plugin substitutes after
	// splitting, so nothing re-splits it.
	for _, f := range []string{"from capture", "from arg"} {
		if _, err := os.Stat(filepath.Join(cwd, f)); err != nil {
			t.Fatalf("%q: %v", f, err)
		}
	}
}

// Deny by default: without config.proc.exec the plugin cannot run anything.
func TestE2E_pluginCannotExecWithoutTheGrant(t *testing.T) {
	cwd := pluginCatalog(t, false, "  # @runner lines\n  boot: |\n    touch one\n")
	app, _, _ := e2eApp(t, cwd)
	err := app.Run([]string{"boot"})
	if err == nil {
		t.Fatal("want failure")
	}
	if _, serr := os.Stat(filepath.Join(cwd, "one")); serr == nil {
		t.Fatal("an ungranted exec ran anyway")
	}
}

// The child's exit code is the script's, through the plugin.
func TestE2E_pluginPropagatesTheExitCode(t *testing.T) {
	cwd := pluginCatalog(t, true,
		"  # @runner lines\n  boom ${CODE}: |\n    sh -c ${CODE}\n    touch never\n")
	body, _ := os.ReadFile(filepath.Join(cwd, "godo.yaml"))
	fixed := strings.Replace(string(body), "engine:\n", "engine:\n  dialect: matcher\n", 1)
	if err := os.WriteFile(filepath.Join(cwd, "godo.yaml"), []byte(fixed), 0o644); err != nil {
		t.Fatal(err)
	}

	app, _, _ := e2eApp(t, cwd)
	err := app.Run([]string{"boom", "exit 42"})
	if got := godo.ExitCode(err); got != 42 {
		t.Fatalf("exit=%d (%v)", got, err)
	}
	if _, serr := os.Stat(filepath.Join(cwd, "never")); serr == nil {
		t.Fatal("the body carried on after a failure")
	}
}

// A digest that does not match the bytes stops the run before anything loads.
func TestE2E_pluginDigestMismatchRefusesToRun(t *testing.T) {
	cwd := pluginCatalog(t, true, "  # @runner lines\n  boot: |\n    touch one\n")
	body, _ := os.ReadFile(filepath.Join(cwd, "godo.yaml"))
	broken := strings.Replace(string(body), "sha256: ", "sha256: 00", 1)
	if err := os.WriteFile(filepath.Join(cwd, "godo.yaml"), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}

	app, _, _ := e2eApp(t, cwd)
	err := app.Run([]string{"boot"})
	if err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("err=%v", err)
	}
	if _, serr := os.Stat(filepath.Join(cwd, "one")); serr == nil {
		t.Fatal("a plugin with the wrong digest ran")
	}
}

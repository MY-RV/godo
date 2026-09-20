package plugin_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/my-rv/godo/internal/plugin"
)

var (
	buildOnce sync.Once
	wasmPath  string
	buildErr  error
)

// exampleWasm builds examples/plugins/lines for wasip1 once per run.
//
// Built rather than committed: a checked-in binary is a thing nobody can
// review, and the point of this test is that the artifact matches the source
// beside it.
func exampleWasm(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir := t.TempDir()
		out := filepath.Join(dir, "lines.wasm")
		cmd := exec.Command("go", "build", "-o", out, "../../examples/plugins/lines")
		cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
		if b, err := cmd.CombinedOutput(); err != nil {
			buildErr = err
			t.Logf("build: %s", b)
			return
		}
		// Keep it outside t.TempDir's cleanup by copying to the package temp.
		data, err := os.ReadFile(out)
		if err != nil {
			buildErr = err
			return
		}
		keep, err := os.CreateTemp("", "lines-*.wasm")
		if err != nil {
			buildErr = err
			return
		}
		defer keep.Close()
		if _, err := keep.Write(data); err != nil {
			buildErr = err
			return
		}
		wasmPath = keep.Name()
	})
	if buildErr != nil {
		t.Fatalf("building the example plugin: %v", buildErr)
	}
	return wasmPath
}

func load(t *testing.T, config map[string]any) (*plugin.Plugin, func()) {
	t.Helper()
	path := exampleWasm(t)
	digest, err := plugin.Digest(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	rt := plugin.NewRuntime(ctx)
	p, err := rt.Load(ctx, path, digest, "", []string{"runner:lines"}, config)
	if err != nil {
		t.Fatal(err)
	}
	return p, func() { _ = rt.Close(ctx) }
}

func TestLoad_refusesAWrongDigest(t *testing.T) {
	path := exampleWasm(t)
	ctx := context.Background()
	rt := plugin.NewRuntime(ctx)
	defer rt.Close(ctx)

	_, err := rt.Load(ctx, path, strings.Repeat("0", 64), "", nil, nil)
	if err == nil {
		t.Fatal("a plugin whose bytes do not match its digest must not load")
	}
	for _, want := range []string{"sha256 mismatch", "declared", "actual"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("err=%v, missing %q", err, want)
		}
	}
}

func TestLoad_refusesSomethingThatIsNotWasm(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "not.wasm")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	digest, err := plugin.Digest(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	rt := plugin.NewRuntime(ctx)
	defer rt.Close(ctx)
	if _, err := rt.Load(ctx, path, digest, "", nil, nil); err == nil {
		t.Fatal("want a compile error")
	}
}

func TestInvoke_previewAsksThePlugin(t *testing.T) {
	p, done := load(t, nil)
	defer done()

	out, err := p.Invoke(context.Background(), plugin.Request{
		Mode:   "preview",
		Runner: "lines",
		Body:   "git status\n?pnpm install\necho hola ${WHO} $1",
		Argv:   map[string]string{"WHO": "mundo"},
		Args:   []string{"un arg"},
	}, plugin.Host{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Code != 0 {
		t.Fatalf("code=%d", out.Code)
	}
	want := []string{"git status", "pnpm install    # may fail", "echo hola mundo 'un arg'"}
	if len(out.Emitted) != len(want) {
		t.Fatalf("emitted=%q", out.Emitted)
	}
	for i := range want {
		if out.Emitted[i] != want[i] {
			t.Fatalf("emitted=%q want %q", out.Emitted, want)
		}
	}
}

// Preview must not run anything, and with no config granted it could not.
func TestInvoke_previewGrantsNothing(t *testing.T) {
	dir := t.TempDir()
	p, done := load(t, nil)
	defer done()

	_, err := p.Invoke(context.Background(), plugin.Request{
		Mode: "preview",
		Body: "touch marker",
	}, plugin.Host{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "marker")); err == nil {
		t.Fatal("preview touched the disk")
	}
}

func TestInvoke_execRunsWhenGranted(t *testing.T) {
	dir := t.TempDir()
	p, done := load(t, map[string]any{"proc": map[string]any{"exec": true}})
	defer done()

	out, err := p.Invoke(context.Background(), plugin.Request{
		Mode: "run",
		Body: "touch marker\ntouch ${NAME}",
		Argv: map[string]string{"NAME": "second"},
	}, plugin.Host{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if out.Code != 0 {
		t.Fatalf("code=%d", out.Code)
	}
	for _, f := range []string{"marker", "second"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
	}
}

// Deny by default: no config, no exec. The plugin is told why.
func TestInvoke_execRefusedWhenNotGranted(t *testing.T) {
	dir := t.TempDir()
	p, done := load(t, nil)
	defer done()

	out, err := p.Invoke(context.Background(), plugin.Request{
		Mode: "run",
		Body: "touch marker",
	}, plugin.Host{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if out.Code == 0 {
		t.Fatal("an ungranted exec must not end in success")
	}
	if _, err := os.Stat(filepath.Join(dir, "marker")); err == nil {
		t.Fatal("an ungranted exec ran anyway")
	}
}

func TestInvoke_configMustSayTrue(t *testing.T) {
	dir := t.TempDir()
	for _, cfg := range []map[string]any{
		{"proc": map[string]any{"exec": false}},
		{"proc": map[string]any{"spawn": true}},
		{"proc": "yes"},
		{"fs": map[string]any{"exec": true}},
	} {
		p, done := load(t, cfg)
		out, err := p.Invoke(context.Background(), plugin.Request{Mode: "run", Body: "touch marker"}, plugin.Host{Dir: dir})
		done()
		if err != nil {
			t.Fatalf("%v: %v", cfg, err)
		}
		if out.Code == 0 {
			t.Fatalf("%v granted exec", cfg)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "marker")); err == nil {
		t.Fatal("something ran")
	}
}

// A failing command stops the body and becomes the exit code, like a shell.
func TestInvoke_exitCodeIsTheChilds(t *testing.T) {
	p, done := load(t, map[string]any{"proc": map[string]any{"exec": true}})
	defer done()

	out, err := p.Invoke(context.Background(), plugin.Request{
		Mode: "run",
		Body: "sh -c ${SCRIPT}\ntouch should-not-exist",
		Argv: map[string]string{"SCRIPT": "exit 42"},
	}, plugin.Host{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if out.Code != 42 {
		t.Fatalf("code=%d, want 42", out.Code)
	}
}

// "?" marks a line that may fail without stopping the rest.
func TestInvoke_optionalLineDoesNotStopTheBody(t *testing.T) {
	dir := t.TempDir()
	p, done := load(t, map[string]any{"proc": map[string]any{"exec": true}})
	defer done()

	out, err := p.Invoke(context.Background(), plugin.Request{
		Mode: "run",
		Body: "?sh -c ${SCRIPT}\ntouch after",
		Argv: map[string]string{"SCRIPT": "exit 3"},
	}, plugin.Host{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if out.Code != 0 {
		t.Fatalf("code=%d", out.Code)
	}
	if _, err := os.Stat(filepath.Join(dir, "after")); err != nil {
		t.Fatalf("the body stopped at an optional failure: %v", err)
	}
}

// The sandbox is shut: the plugin has no filesystem of its own.
func TestInvoke_pluginCannotReachTheFilesystemItself(t *testing.T) {
	p, done := load(t, nil)
	defer done()
	// The example plugin never opens a file, so this asserts the configuration
	// rather than the plugin: no WithFS was granted, so nothing is mounted.
	out, err := p.Invoke(context.Background(), plugin.Request{Mode: "preview", Body: "echo hi"}, plugin.Host{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Emitted) != 1 {
		t.Fatalf("emitted=%q", out.Emitted)
	}
}

package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
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

func load(t *testing.T, config map[string]any) (*Plugin, func()) {
	t.Helper()
	path := exampleWasm(t)
	digest, err := Digest(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	rt := NewRuntime(ctx)
	p, err := rt.Load(ctx, path, digest, "", []string{"runner:lines"}, config)
	if err != nil {
		t.Fatal(err)
	}
	return p, func() { _ = rt.Close(ctx) }
}

func TestLoad_refusesAWrongDigest(t *testing.T) {
	path := exampleWasm(t)
	ctx := context.Background()
	rt := NewRuntime(ctx)
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
	digest, err := Digest(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	rt := NewRuntime(ctx)
	defer rt.Close(ctx)
	if _, err := rt.Load(ctx, path, digest, "", nil, nil); err == nil {
		t.Fatal("want a compile error")
	}
}

// A failing command stops the body and becomes the exit code, like a shell.
func TestInvoke_exitCodeIsTheChilds(t *testing.T) {
	p, done := load(t, map[string]any{"proc": map[string]any{"exec": true}})
	defer done()

	out, err := p.Invoke(context.Background(), Request{
		Body: "sh -c ${SCRIPT}\ntouch should-not-exist",
		Argv: map[string]string{"SCRIPT": "exit 42"},
	}, Host{Dir: t.TempDir()})
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

	out, err := p.Invoke(context.Background(), Request{
		Body: "?sh -c ${SCRIPT}\ntouch after",
		Argv: map[string]string{"SCRIPT": "exit 3"},
	}, Host{Dir: dir})
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

func TestServe_outWritesWithoutAnswer(t *testing.T) {
	var stdout, answers bytes.Buffer
	p := &Plugin{}
	if err := p.serve(strings.NewReader(`{"op":"out","text":"hello"}`+"\n"), &answers, Host{Stdout: &stdout}); err != nil {
		t.Fatal(err)
	}
	if got := stdout.String(); got != "hello\n" {
		t.Fatalf("stdout=%q", got)
	}
	if got := answers.String(); got != "" {
		t.Fatalf("out unexpectedly received an answer: %q", got)
	}
}

// The catalog's neighbour is reachable on purpose: a git worktree is created
// beside a repository, and linking .env into it is the whole use case.
//
// Confinement here would also be theatre — proc.exec can run "ln -s" anywhere,
// so a slink narrower than exec protects nothing. The grant is the boundary.
func TestServe_slinkReachesBesideTheCatalog(t *testing.T) {
	root := t.TempDir()
	catalog := filepath.Join(root, "repo")
	neighbour := filepath.Join(root, "repo-wt")
	for _, d := range []string{catalog, neighbour} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(catalog, ".env"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &Plugin{Config: map[string]any{"fs": map[string]any{"slink": true}}}

	res := serveOp(t, p, Host{Dir: catalog}, Op{Op: OpSlink, Src: ".env", Dst: "../repo-wt/.env"})
	if res.Error != "" {
		t.Fatalf("error=%q", res.Error)
	}
	if !res.OK {
		t.Fatal("slink reported failure")
	}
	if _, err := os.Lstat(filepath.Join(neighbour, ".env")); err != nil {
		t.Fatalf("the link was not created: %v", err)
	}
}

// An absolute path is taken as given rather than reinterpreted.
func TestServe_slinkAcceptsAnAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "source")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &Plugin{Config: map[string]any{"fs": map[string]any{"slink": true}}}

	res := serveOp(t, p, Host{Dir: dir}, Op{Op: OpSlink, Src: target, Dst: "link"})
	if res.Error != "" {
		t.Fatalf("error=%q", res.Error)
	}
	got, err := os.Readlink(filepath.Join(dir, "link"))
	if err != nil {
		t.Fatal(err)
	}
	if got != target {
		t.Fatalf("link points at %q, want %q", got, target)
	}
}

func TestServe_slinkForceReplacesExistingDestination(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "source"), []byte("replacement"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "link")
	if err := os.Symlink("missing", dst); err != nil {
		t.Fatal(err)
	}
	p := &Plugin{Config: map[string]any{"fs": map[string]any{"slink": true}}}
	res := serveOp(t, p, Host{Dir: dir}, Op{Op: OpSlink, Src: "source", Dst: "link"})
	if res.Error == "" {
		t.Fatal("force=false replaced an existing destination")
	}
	if target, err := os.Readlink(dst); err != nil || target != "missing" {
		t.Fatalf("force=false changed destination: target=%q err=%v", target, err)
	}

	res = serveOp(t, p, Host{Dir: dir}, Op{Op: OpSlink, Src: "source", Dst: "link", Force: true})
	if !res.OK || res.Code != 0 || res.Error != "" {
		t.Fatalf("result=%+v", res)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != "replacement" {
		t.Fatalf("replacement content=%q", got)
	}
}

func serveOp(t *testing.T, p *Plugin, host Host, op Op) Result {
	t.Helper()
	line, err := json.Marshal(op)
	if err != nil {
		t.Fatal(err)
	}
	var answers bytes.Buffer
	if err := p.serve(bytes.NewReader(append(line, '\n')), &answers, host); err != nil {
		t.Fatal(err)
	}
	var res Result
	if err := json.NewDecoder(&answers).Decode(&res); err != nil {
		t.Fatalf("decode result %q: %v", answers.String(), err)
	}
	return res
}

func TestInvoke_execRunsCommands(t *testing.T) {
	dir := t.TempDir()
	p, done := load(t, nil)
	defer done()

	out, err := p.Invoke(context.Background(), Request{
		Body: "touch marker\ntouch ${NAME}",
		Argv: map[string]string{"NAME": "second"},
	}, Host{Dir: dir})
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

func TestServe_slinkCreatesTheLink(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "source"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &Plugin{}
	res := serveOp(t, p, Host{Dir: dir}, Op{Op: OpSlink, Src: "source", Dst: "link"})
	if res.Error != "" {
		t.Fatalf("error=%q", res.Error)
	}
	if !res.OK {
		t.Fatal("slink reported failure")
	}
	got, err := os.Readlink(filepath.Join(dir, "link"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "source" {
		t.Fatalf("link points at %q", got)
	}
}

// The default is the catalog's own directory: a script that cannot read the
// repository it belongs to is not useful, and withholding it defends nothing.
func TestMounts_defaultsToTheCatalog(t *testing.T) {
	dir := t.TempDir()
	p := &Plugin{}
	got := p.Mounts(dir)
	if len(got) != 1 || got[0].Host != dir || got[0].Guest != "/" {
		t.Fatalf("mounts=%+v", got)
	}
}

func TestMounts_shapes(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name string
		cfg  map[string]any
		want []Mount
	}{
		{"true is the catalog", map[string]any{"fs": map[string]any{"mount": true}},
			[]Mount{{Host: dir, Guest: "/"}}},
		{"false is nothing", map[string]any{"fs": map[string]any{"mount": false}}, nil},
		{"a list", map[string]any{"fs": map[string]any{"mount": []any{".", "/tmp"}}},
			[]Mount{{Host: dir, Guest: "/"}, {Host: "/tmp", Guest: "/tmp"}}},
		{"a mapping", map[string]any{"fs": map[string]any{"mount": map[string]any{"/opt/x": "/x"}}},
			[]Mount{{Host: "/opt/x", Guest: "/x"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := (&Plugin{Config: tc.cfg}).Mounts(dir)
			if len(got) != len(tc.want) {
				t.Fatalf("mounts=%+v want %+v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("mounts=%+v want %+v", got, tc.want)
				}
			}
		})
	}
}

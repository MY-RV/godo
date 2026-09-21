package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/sys"
)

// Host is what a plugin can reach through godo. Everything here is gated by
// the catalog's config: a capability the catalog did not grant is not a stub
// that fails, it is an op godo refuses to perform.
type Host struct {
	// Dir is the working directory for exec, normally the catalog's.
	Dir    string
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
}

// Outcome is what an invocation produced.
type Outcome struct {
	// Code is the plugin's exit code, and so the script's.
	Code int
}

// Invoke runs the plugin once.
//
// The plugin is a WASI command, so this instantiates the module, writes the
// request to its stdin, and answers ops until it exits. Its exit code is the
// result; its stderr goes to godo's, so a plugin can explain itself.
func (p *Plugin) Invoke(ctx context.Context, req Request, host Host) (Outcome, error) {
	req.API = APIVersion
	req.Config = p.Config

	inR, inW := io.Pipe()
	outR, outW := io.Pipe()

	stderr := host.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	// A wasm guest starts with nothing wired to the outside. The mounts below
	// are what the catalog asks for; everything else a script might reach for
	// — a network, a subprocess — has no syscall to reach through, whatever
	// anyone configures.
	cfg := wazero.NewModuleConfig().
		WithStdin(inR).
		WithStdout(outW).
		WithStderr(stderr).
		WithArgs("plugin").
		WithName("")
	if mounts := p.Mounts(host.Dir); len(mounts) > 0 {
		fs := wazero.NewFSConfig()
		for _, m := range mounts {
			fs = fs.WithDirMount(m.Host, m.Guest)
		}
		cfg = cfg.WithFSConfig(fs)
	}
	cfg = cfg.WithSysWalltime().WithSysNanotime()

	runErr := make(chan error, 1)
	go func() {
		_, err := p.rt.InstantiateModule(ctx, p.compiled, cfg)
		_ = outW.Close()
		runErr <- err
	}()

	// The request goes out on its own goroutine: an io.Pipe has no buffer, so
	// writing it inline would deadlock against a plugin that spoke first.
	writeErr := make(chan error, 1)
	go func() {
		line, err := json.Marshal(req)
		if err != nil {
			writeErr <- err
			return
		}
		_, err = inW.Write(append(line, '\n'))
		writeErr <- err
	}()

	out := Outcome{}
	loopErr := p.serve(outR, inW, host)

	_ = inW.Close()
	_ = inR.Close()

	err := <-runErr
	_ = outR.Close()
	if werr := <-writeErr; werr != nil && loopErr == nil {
		loopErr = werr
	}

	var exit *sys.ExitError
	switch {
	case err == nil:
		out.Code = 0
	case errors.As(err, &exit):
		out.Code = int(exit.ExitCode())
	default:
		return out, fmt.Errorf("plugin %s: %w", p.Source, err)
	}
	if loopErr != nil {
		return out, loopErr
	}
	return out, nil
}

// serve answers ops until the plugin's stdout closes.
func (p *Plugin) serve(r io.Reader, w io.Writer, host Host) error {
	scan := bufio.NewScanner(r)
	scan.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	enc := json.NewEncoder(w)

	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" {
			continue
		}
		// A line that is not an op is output. A guest has one stdout, and
		// everything inside it writes there — an imported module's print is
		// the builtin one, not whatever the plugin arranged for the script.
		// Treating that as a protocol error would mean a plugin's users could
		// not split their code into files.
		op, ok := readOp(line)
		if !ok {
			if _, err := fmt.Fprintln(orStd(host.Stdout, os.Stdout), line); err != nil {
				return fmt.Errorf("plugin %s: %w", p.Source, err)
			}
			continue
		}
		switch op.Op {
		case OpOut:
			if _, err := fmt.Fprintln(orStd(host.Stdout, os.Stdout), op.Text); err != nil {
				return fmt.Errorf("plugin %s: %w", p.Source, err)
			}
		case OpExec:
			res := p.execOp(op, host)
			if err := enc.Encode(res); err != nil {
				return fmt.Errorf("plugin %s: %w", p.Source, err)
			}
		case OpSlink:
			res := p.slinkOp(op, host)
			if err := enc.Encode(res); err != nil {
				return fmt.Errorf("plugin %s: %w", p.Source, err)
			}
		}
	}
	if err := scan.Err(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
		return fmt.Errorf("plugin %s: %w", p.Source, err)
	}
	return nil
}

// readOp parses a line as an op, or reports that it is output.
//
// Both halves matter. A line that is not JSON is plain output. A line that is
// JSON but carries no op godo knows is output too — scripts print JSON all the
// time, and swallowing a data structure because it has the shape of a message
// would be worse than the error it avoids.
//
// A line that is a valid op godo does obey, whoever printed it. Closing that
// would take a secret on every op, which breaks every plugin already built to
// answer without one — a worse failure, and for a coincidence inside a
// catalog whose scripts could call exec directly anyway.
func readOp(line string) (Op, bool) {
	if !strings.HasPrefix(line, "{") {
		return Op{}, false
	}
	var op Op
	if err := json.Unmarshal([]byte(line), &op); err != nil {
		return Op{}, false
	}
	switch op.Op {
	case OpExec, OpOut, OpSlink:
		return op, true
	}
	return Op{}, false
}

// execOp runs an argument vector.
func (p *Plugin) execOp(op Op, host Host) Result {
	if len(op.Argv) == 0 {
		return Result{Error: "exec: empty argv"}
	}
	cmd := exec.Command(op.Argv[0], op.Argv[1:]...)
	cmd.Dir = host.Dir
	if op.Dir != "" {
		cmd.Dir = op.Dir
	}
	var res Result
	if op.Capture {
		var stdout, stderr strings.Builder
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		res.Code = runAndCode(cmd, &res)
		res.Stdout = stdout.String()
		res.Stderr = stderr.String()
	} else {
		cmd.Stdout = orStd(host.Stdout, os.Stdout)
		cmd.Stderr = orStd(host.Stderr, os.Stderr)
		cmd.Stdin = host.Stdin
		res.Code = runAndCode(cmd, &res)
	}
	res.OK = res.Code == 0
	return res
}

func (p *Plugin) slinkOp(op Op, host Host) Result {
	// Paths resolve against the catalog's directory but are not confined to
	// it. A worktree is created beside a repository, not inside it, and the
	// link into it is the point.
	src := resolveAgainst(host.Dir, op.Src)
	dst := resolveAgainst(host.Dir, op.Dst)

	if _, err := os.Lstat(dst); err == nil {
		if !op.Force {
			return Result{Error: fmt.Sprintf("slink: dst exists: %s", op.Dst)}
		}
		if err := os.Remove(dst); err != nil {
			return Result{Error: fmt.Sprintf("slink: remove dst: %v", err)}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Result{Error: fmt.Sprintf("slink: dst: %v", err)}
	}

	// Windows support for os.Symlink is unverified on a real Windows host.
	if err := os.Symlink(src, dst); err != nil {
		return Result{Error: fmt.Sprintf("slink: %v", err)}
	}
	return Result{Code: 0, OK: true}
}

// resolveAgainst turns a plugin-supplied path into an absolute one.
//
// An absolute path is taken as given: a script that names /tmp means /tmp.
func resolveAgainst(dir, path string) string {
	if filepath.IsAbs(path) || dir == "" {
		return path
	}
	return filepath.Join(dir, path)
}

func runAndCode(cmd *exec.Cmd, res *Result) int {
	err := cmd.Run()
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	// Could not start at all: a missing binary is the plugin's problem to
	// report, not godo's to crash on.
	res.Error = err.Error()
	return 127
}

func orStd(w io.Writer, def io.Writer) io.Writer {
	if w == nil {
		return def
	}
	return w
}

// Mount is one directory the guest can see, and where it sees it.
type Mount struct {
	Host  string // the real path
	Guest string // where it appears inside the sandbox
}

// Mounts returns the directories config.fs.mount asks for.
//
// This is the one thing godo reads out of config, and it is configuration
// rather than permission: a guest cannot mount anything itself, so somebody
// has to say what it sees, and the catalog is the only one who knows. The
// default is the directory the godo.yaml lives in — a script that cannot read
// the repository it belongs to is not useful, and nothing is being defended
// against by withholding it.
//
// Three shapes, because a catalog that only wants its own directory should not
// have to say so twice:
//
//	fs: {mount: true}                       the catalog's directory, as /
//	fs: {mount: [".", "/tmp"]}              those, each at its own name
//	fs: {mount: {".": "/", "/tmp": "/tmp"}} explicit guest paths
//
// A relative host path resolves against the catalog. Nothing else is visible:
// a directory the config does not name does not exist inside the sandbox, and
// no path can climb out of one that it does.
func (p *Plugin) Mounts(catalogDir string) []Mount {
	raw, ok := p.configValue("fs", "mount")
	if !ok {
		if catalogDir == "" {
			return nil
		}
		return []Mount{{Host: catalogDir, Guest: "/"}}
	}
	resolve := func(host string) string {
		if host == "" {
			host = "."
		}
		if !filepath.IsAbs(host) && catalogDir != "" {
			return filepath.Join(catalogDir, host)
		}
		return host
	}
	switch v := raw.(type) {
	case bool:
		if !v || catalogDir == "" {
			return nil
		}
		return []Mount{{Host: catalogDir, Guest: "/"}}
	case []any:
		var out []Mount
		for i, item := range v {
			host, ok := item.(string)
			if !ok {
				continue
			}
			guest := host
			if i == 0 && (host == "." || host == "./") {
				// The first entry being the catalog is the common case, and it
				// belongs at the root so relative paths in a script just work.
				guest = "/"
			}
			out = append(out, Mount{Host: resolve(host), Guest: guest})
		}
		return out
	case map[string]any:
		var out []Mount
		for host, g := range v {
			guest, ok := g.(string)
			if !ok {
				continue
			}
			out = append(out, Mount{Host: resolve(host), Guest: guest})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Guest < out[j].Guest })
		return out
	}
	return nil
}

// configValue reads config[section][key] without deciding what it means.
func (p *Plugin) configValue(section, key string) (any, bool) {
	raw, ok := p.Config[section]
	if !ok {
		return nil, false
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	v, ok := m[key]
	return v, ok
}

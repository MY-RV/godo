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

	// The sandbox starts shut: no filesystem, no environment, no network, and
	// a frozen clock. Nothing is disabled here — nothing is granted. What the
	// catalog's config asks for is added back below, and only that.
	cfg := wazero.NewModuleConfig().
		WithStdin(inR).
		WithStdout(outW).
		WithStderr(stderr).
		WithArgs("plugin").
		WithName("")
	if dir := p.mountDir(host.Dir); dir != "" {
		// Scoped to one directory, which becomes the guest's root. A script
		// can reach the catalog it belongs to and nothing above it.
		cfg = cfg.WithFSConfig(wazero.NewFSConfig().WithDirMount(dir, "/"))
	}
	if p.granted("time", "wall") {
		cfg = cfg.WithSysWalltime().WithSysNanotime()
	}

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
		var op Op
		if err := json.Unmarshal([]byte(line), &op); err != nil {
			return fmt.Errorf("plugin %s: unreadable op %q: %w", p.Source, line, err)
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
		default:
			if err := enc.Encode(Result{Error: fmt.Sprintf("unknown op %q", op.Op)}); err != nil {
				return fmt.Errorf("plugin %s: %w", p.Source, err)
			}
		}
	}
	if err := scan.Err(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
		return fmt.Errorf("plugin %s: %w", p.Source, err)
	}
	return nil
}

// execOp runs an argument vector, if the catalog granted it.
func (p *Plugin) execOp(op Op, host Host) Result {
	if !p.granted("proc", "exec") {
		return Result{Error: "not granted: proc.exec — enable it under engine.plugins[].config.proc.exec"}
	}
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
	if !p.granted("fs", "slink") {
		return Result{Error: "not granted: fs.slink — enable it under engine.plugins[].config.fs.slink"}
	}
	// Paths resolve against the catalog's directory but are not confined to
	// it. A worktree is created beside a repository, not inside it, and the
	// link into it is the point. Confinement here would also be theatre:
	// proc.exec can run "ln -s" anywhere, so a slink narrower than exec
	// protects nothing. The grant is the boundary — withhold fs.slink from a
	// plugin you would not hand a shell.
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

// mountDir returns the directory to mount, or "" for no filesystem.
//
// config.fs.mount is a bool rather than a path: what gets mounted is the
// catalog's own directory, never somewhere the catalog names. A file that
// picks its own mount point could ask for "/" and the grant would mean
// nothing.
func (p *Plugin) mountDir(catalogDir string) string { return p.MountedDir(catalogDir) }

// MountedDir reports which directory this plugin would be given, or "" for
// none. Exported so a caller can show the grant without invoking anything.
func (p *Plugin) MountedDir(catalogDir string) string {
	if catalogDir == "" || !p.granted("fs", "mount") {
		return ""
	}
	return catalogDir
}

// grantNote lists what this plugin was granted, for a failure message.
//
// A script inside a shut sandbox fails in the language's own words — a missing
// file is ENOENT, not "you did not grant fs.mount" — and the connection is
// invisible from inside. This does not claim to know why something failed; it
// puts the grants next to the failure so the reader can see what was and was
// not available.
func (p *Plugin) grantNote() string {
	var have []string
	for _, c := range []struct{ section, key string }{
		{"proc", "exec"},
		{"fs", "mount"},
		{"fs", "slink"},
		{"time", "wall"},
	} {
		if p.granted(c.section, c.key) {
			have = append(have, c.section+"."+c.key)
		}
	}
	if len(have) == 0 {
		return "\n  granted: nothing — see engine.plugins[].config"
	}
	note := "\n  granted: " + strings.Join(have, ", ")
	if !p.granted("fs", "mount") {
		note += "\n  no filesystem: add fs.mount under its config if the script reads or writes files"
	}
	return note
}

// granted reports whether config grants section.key.
//
// Deny by default: an absent section, an absent key, or anything that is not
// exactly true, is not granted.
func (p *Plugin) granted(section, key string) bool {
	raw, ok := p.Config[section]
	if !ok {
		return false
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return false
	}
	v, ok := m[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

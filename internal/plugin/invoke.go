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
	// Emitted holds the lines the plugin contributed to --preview.
	Emitted []string
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

	cfg := wazero.NewModuleConfig().
		WithStdin(inR).
		WithStdout(outW).
		WithStderr(stderr).
		WithArgs("plugin").
		// No WithFS, no WithEnv, no WithSysWalltime: the sandbox stays shut.
		WithName("")

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
	loopErr := p.serve(outR, inW, host, &out)

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
func (p *Plugin) serve(r io.Reader, w io.Writer, host Host, out *Outcome) error {
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
		case OpEmit:
			// Preview output. No answer: a plugin that waited for one here
			// would hang, so emit is deliberately one-way.
			out.Emitted = append(out.Emitted, op.Line)
		case OpExec:
			res := p.execOp(op, host)
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

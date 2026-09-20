package execshell

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/my-rv/godo/internal/catalog"
)

// InheritRunner runs command lines through the shell the caller is already in.
//
// godo used to always use sh -c / cmd /C, which meant a zsh, fish or PowerShell
// user ran their catalog under a shell they did not choose — and the divergence
// was silent, not an error: "arr=(a b c); echo ${arr[1]}" prints a in zsh and b
// in sh, because zsh indexes arrays from 1.
//
// This does not make anything portable and is not meant to: if you are in
// PowerShell you write PowerShell. It makes godo honest about who runs the line.
//
// What it does not give you is your shell's *configuration*. Like sh -c, the
// shell is started non-interactively and does not read your rc file, so your
// aliases and functions are not there. It is your shell's grammar, not your
// shell's setup.
type InheritRunner struct {
	Dir    string
	Stdout *os.File
	Stderr *os.File
	Stdin  *os.File

	// Shell overrides detection. Empty → detect (see DetectShell).
	Shell string
}

// Run implements catalog.Runner.
func (r InheritRunner) Run(command string) error {
	shell := r.Shell
	if shell == "" {
		shell = DetectShell()
	}
	name, args := shellCommand(shell, command)
	cmd := exec.Command(name, args...)
	if r.Dir != "" {
		cmd.Dir = r.Dir
	}
	// Avoid typed-nil *os.File in io.Writer: interface != nil but Write fails.
	if r.Stdout != nil {
		cmd.Stdout = r.Stdout
	} else {
		cmd.Stdout = os.Stdout
	}
	if r.Stderr != nil {
		cmd.Stderr = r.Stderr
	} else {
		cmd.Stderr = os.Stderr
	}
	if r.Stdin != nil {
		cmd.Stdin = r.Stdin
	} else {
		cmd.Stdin = os.Stdin
	}
	err := cmd.Run()
	if err == nil {
		return nil
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return &catalog.ExitError{Code: ee.ExitCode(), Message: fmt.Sprintf("command failed: %s", command)}
	}
	return err
}

// DetectShell returns the shell to run a catalog line with.
//
// GODO_SHELL wins, always. Otherwise $SHELL on Unix; on Windows, the parent
// process when it is a shell, else %ComSpec%.
//
// Windows gets the parent process rather than an environment variable because
// the environment cannot answer: PowerShell sets PSModulePath and everything it
// starts inherits it, so a cmd.exe opened from PowerShell looks like
// PowerShell. A parent that is not a shell — a build tool, an editor, CI —
// falls through to %ComSpec%, which is also what a user who disagrees with any
// of this overrides with GODO_SHELL.
//
// $SHELL on Unix is the login shell, not necessarily the one running right
// now: bash started inside zsh still reports zsh. That is the convention every
// other tool follows, and GODO_SHELL is the way to disagree with it too.
func DetectShell() string {
	if s := strings.TrimSpace(os.Getenv("GODO_SHELL")); s != "" {
		return s
	}
	if runtime.GOOS == "windows" {
		if s := callerShell(); s != "" {
			return s
		}
		if c := strings.TrimSpace(os.Getenv("ComSpec")); c != "" {
			return c
		}
		return "cmd.exe"
	}
	if s := strings.TrimSpace(os.Getenv("SHELL")); s != "" {
		return s
	}
	return "/bin/sh"
}

// shellBase is the shell's name, lowercased and without a .exe suffix.
//
// filepath.Base is separator-aware per platform, and this has to recognise a
// Windows path even when the detection runs somewhere else (a test, a catalog
// pinning GODO_SHELL), so it splits on both separators itself.
func shellBase(shell string) string {
	s := shell
	if i := strings.LastIndexAny(s, `/\`); i >= 0 {
		s = s[i+1:]
	}
	return strings.TrimSuffix(strings.ToLower(s), ".exe")
}

// shellCommand returns the process and arguments that make shell run one
// command line, non-interactively and without reading a profile.
func shellCommand(shell, command string) (string, []string) {
	switch shellBase(shell) {
	case "cmd", "command":
		return shell, []string{"/C", command}
	case "powershell", "pwsh":
		// -NoProfile matches what -c does on Unix: the rc file is not read.
		return shell, []string{"-NoProfile", "-Command", command}
	default:
		// sh, bash, zsh, dash, ksh, fish, and anything else that follows the
		// convention. -c is close to universal; a shell that does not take it
		// is the case GODO_SHELL exists for.
		return shell, []string{"-c", command}
	}
}

// KnownShells are the shell names a catalog may name directly with
// "# @runner <name>".
//
// It is a list rather than "anything on PATH" because a runner name is not an
// arbitrary command: "# @runner git" would otherwise run "git -c <line>" and
// fail in a way nobody could read. A shell that is not here is reachable with
// GODO_SHELL and the default runner.
func KnownShells() []string {
	return []string{
		"sh", "bash", "zsh", "dash", "ksh", "ash", "fish", "nu",
		"cmd", "pwsh", "powershell",
	}
}

// NativeShell returns a runner for a shell named directly by a catalog.
//
// The name is logical, never a path: a catalog says "cmd" or "pwsh", not
// "cmd.exe" or a drive letter, so the same godo.yaml reads the same on every
// machine. The extension, if the platform wants one, is PATH's business.
func NativeShell(name, dir string) (InheritRunner, error) {
	if !isKnownShell(name) {
		return InheritRunner{}, fmt.Errorf("%q is not a known shell (%s); for another one set GODO_SHELL",
			name, strings.Join(KnownShells(), ", "))
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return InheritRunner{}, fmt.Errorf("shell %q is not installed here", name)
	}
	return InheritRunner{Dir: dir, Shell: path}, nil
}

func isKnownShell(name string) bool {
	for _, s := range KnownShells() {
		if s == name {
			return true
		}
	}
	return false
}

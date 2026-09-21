package execshell

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/my-rv/godo/internal/catalog"
)

func TestShellCommand_flagsPerShellFamily(t *testing.T) {
	cases := []struct {
		shell string
		want  []string
	}{
		{"/bin/sh", []string{"-c"}},
		{"/bin/bash", []string{"-c"}},
		{"/bin/zsh", []string{"-c"}},
		{"/usr/local/bin/fish", []string{"-c"}},
		{"/opt/weird/myshell", []string{"-c"}},
		{`C:\Windows\System32\cmd.exe`, []string{"/C"}},
		{"cmd", []string{"/C"}},
		{"powershell.exe", []string{"-NoProfile", "-Command"}},
		{"pwsh", []string{"-NoProfile", "-Command"}},
		{`C:\Program Files\PowerShell\7\pwsh.exe`, []string{"-NoProfile", "-Command"}},
	}
	for _, tc := range cases {
		name, args := shellCommand(tc.shell, "echo hi")
		if name != tc.shell {
			t.Fatalf("%s: name=%q", tc.shell, name)
		}
		if len(args) != len(tc.want)+1 {
			t.Fatalf("%s: args=%q", tc.shell, args)
		}
		for i, w := range tc.want {
			if args[i] != w {
				t.Fatalf("%s: args=%q want prefix %q", tc.shell, args, tc.want)
			}
		}
		if args[len(args)-1] != "echo hi" {
			t.Fatalf("%s: command not last: %q", tc.shell, args)
		}
	}
}

// Detection is a heuristic; GODO_SHELL is the documented way out of it.
func TestDetectShell_godoShellWins(t *testing.T) {
	t.Setenv("GODO_SHELL", "/opt/mine/sh")
	t.Setenv("SHELL", "/bin/zsh")
	if got := DetectShell(); got != "/opt/mine/sh" {
		t.Fatalf("got %q", got)
	}
}

func TestDetectShell_usesSHELLOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix branch")
	}
	t.Setenv("GODO_SHELL", "")
	t.Setenv("SHELL", "/bin/zsh")
	if got := DetectShell(); got != "/bin/zsh" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("SHELL", "")
	if got := DetectShell(); got != "/bin/sh" {
		t.Fatalf("fallback: got %q", got)
	}
}

// The point of the runner, as a test: the same line means different things in
// different shells, and godo must stop picking for you.
func TestInheritRunner_runsUnderTheCallersShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix shells")
	}
	if _, err := os.Stat("/bin/zsh"); err != nil {
		t.Skip("no zsh on this machine")
	}
	// $0 is the shell running the line. Arrays would be a sharper example but
	// dash has none, and /bin/sh is dash on Debian and bash on macOS.
	const line = `printf %s "$0"`

	got := captureRun(t, InheritRunner{Shell: "/bin/zsh"}, line)
	if !strings.Contains(got, "zsh") {
		t.Fatalf("zsh ran the line but $0 was %q", got)
	}
	got = captureRun(t, InheritRunner{Shell: "/bin/sh"}, line)
	if strings.Contains(got, "zsh") {
		t.Fatalf("sh ran the line but $0 was %q", got)
	}
}

func TestInheritRunner_propagatesTheChildExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix shells")
	}
	err := InheritRunner{Shell: "/bin/sh"}.Run("exit 42")
	if err == nil {
		t.Fatal("want failure")
	}
	if got := catalog.ExitCode(err); got != 42 {
		t.Fatalf("exit=%d (%v)", got, err)
	}
}

func TestInheritRunner_runsInTheCatalogDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix shells")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "marker"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got := captureRun(t, InheritRunner{Dir: dir, Shell: "/bin/sh"}, "ls marker")
	if got != "marker" {
		t.Fatalf("got %q", got)
	}
}

// captureRun runs one line and returns its trimmed stdout.
func captureRun(t *testing.T, r InheritRunner, line string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r.Stdout = f
	if err := r.Run(line); err != nil {
		t.Fatalf("run %q: %v", line, err)
	}
	b, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(bytes.TrimSpace(b)))
}

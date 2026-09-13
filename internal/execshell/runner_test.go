package execshell_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/my-rv/godo/internal/catalog"
	"github.com/my-rv/godo/internal/execshell"
)

func TestRunner_exitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell exit differs")
	}
	dir := t.TempDir()
	r := execshell.Runner{Dir: dir, Stdout: os.Stdout, Stderr: os.Stderr}
	err := r.Run("exit 42")
	var ee *catalog.ExitError
	if !errors.As(err, &ee) || ee.Code != 42 {
		t.Fatalf("%v", err)
	}
}

func TestRunner_usesDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pwd")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	r := execshell.Runner{Dir: dir}
	if err := r.Run("touch marker"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal(err)
	}
}

func TestRunner_zeroValueStdioEcho(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh")
	}
	// Mirrors CLI: Runner{Dir: ...} with nil File fields (typed-nil trap).
	r := execshell.Runner{Dir: t.TempDir()}
	if err := r.Run(`echo "Hello, World!"`); err != nil {
		t.Fatal(err)
	}
}

package execshell

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/my-rv/godo/internal/catalog"
)

// Runner runs command lines via the system shell.
type Runner struct {
	Dir    string
	Stdout *os.File
	Stderr *os.File
	Stdin  *os.File
}

// Run implements catalog.Runner.
func (r Runner) Run(command string) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}
	if r.Dir != "" {
		cmd.Dir = r.Dir
	}
	// Avoid typed-nil *os.File in io.Writer: interface != nil but Write panics/fails.
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
		code := ee.ExitCode()
		return &catalog.ExitError{Code: code, Message: fmt.Sprintf("command failed: %s", command)}
	}
	return err
}

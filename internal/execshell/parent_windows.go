//go:build windows

package execshell

import (
	"os"
	"syscall"
	"unsafe"
)

// callerShell returns the shell that started this process, or "".
//
// Windows has no $SHELL, and the environment cannot answer the question:
// PSModulePath is set by PowerShell but inherited by everything it starts, so
// a cmd.exe opened from PowerShell looks exactly like PowerShell. The parent
// process is the only honest answer, so that is what this reads.
//
// Empty when the parent is not a shell — a build tool, an editor, a CI runner —
// which is the common case and why the caller falls back to %ComSpec%.
func callerShell() string {
	name, err := processName(uint32(os.Getppid()))
	if err != nil || name == "" {
		return ""
	}
	if !isKnownShell(shellBase(name)) {
		return ""
	}
	return name
}

// processName returns the executable name of pid via the process snapshot.
func processName(pid uint32) (string, error) {
	snap, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return "", err
	}
	defer syscall.CloseHandle(snap)

	var e syscall.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	if err := syscall.Process32First(snap, &e); err != nil {
		return "", err
	}
	for {
		if e.ProcessID == pid {
			return syscall.UTF16ToString(e.ExeFile[:]), nil
		}
		if err := syscall.Process32Next(snap, &e); err != nil {
			return "", err
		}
	}
}

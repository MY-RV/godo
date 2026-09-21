//go:build windows

package execshell

import (
	"os"
	"syscall"
	"unsafe"
)

// callerShell returns the shell godo is running under, or "".
//
// Windows has no $SHELL, and the environment cannot answer the question:
// PSModulePath is set by PowerShell but inherited by everything it starts, so
// a cmd.exe opened from PowerShell looks exactly like PowerShell. The process
// tree is the only honest answer, so that is what this reads — see
// shellAncestor for why it is the tree and not just the parent.
//
// Empty when no shell is up there — a service, a CI runner — which is why the
// caller falls back to %ComSpec%.
func callerShell() string {
	table, err := processTable()
	if err != nil {
		return ""
	}
	return shellAncestor(table, uint32(os.Getpid()))
}

// processTable snapshots every running process once.
//
// One snapshot rather than one per hop: walking the tree would otherwise take
// a fresh snapshot at every level, and the tree could change underneath the
// walk between them.
func processTable() (map[uint32]proc, error) {
	snap, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(snap)

	var e syscall.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	if err := syscall.Process32First(snap, &e); err != nil {
		return nil, err
	}
	table := make(map[uint32]proc, 256)
	for {
		table[e.ProcessID] = proc{
			parent: e.ParentProcessID,
			name:   syscall.UTF16ToString(e.ExeFile[:]),
		}
		if err := syscall.Process32Next(snap, &e); err != nil {
			// ERROR_NO_MORE_FILES ends the walk; the table is complete.
			return table, nil
		}
	}
}

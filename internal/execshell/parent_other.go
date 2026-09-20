//go:build !windows

package execshell

// callerShell is unused off Windows: $SHELL answers the question there, and
// reading the parent process would need a different syscall on every Unix.
func callerShell() string { return "" }

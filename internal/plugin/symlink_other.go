//go:build !windows

package plugin

// symlinkHint has nothing to add off Windows: creating a symlink there is an
// ordinary file operation, and the OS error already says what went wrong.
func symlinkHint(error) string { return "" }

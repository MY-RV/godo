//go:build !windows

package plugin

import "os"

// linkPath makes dst point at src.
//
// A symlink is an ordinary file operation here, needs no privilege, and may
// dangle. Windows has none of those three, which is why it has its own.
func linkPath(src, dst string) error { return os.Symlink(src, dst) }

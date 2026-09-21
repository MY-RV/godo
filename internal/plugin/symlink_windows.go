//go:build windows

package plugin

import (
	"errors"
	"syscall"
)

// errPrivilegeNotHeld is ERROR_PRIVILEGE_NOT_HELD, what Windows answers when a
// process without SeCreateSymbolicLinkPrivilege tries to create a symlink.
//
// Spelled out rather than imported: the stdlib syscall package does not name
// this one, and Windows error numbers are a stable ABI.
const errPrivilegeNotHeld = syscall.Errno(1314)

// symlinkHint turns an OS refusal into something the reader can act on.
//
// On Windows creating a symlink is a privilege, not a file operation, so an
// ordinary user gets "A required privilege is not held by the client" — true,
// and useless unless you already know that Developer Mode is what grants it.
func symlinkHint(err error) string {
	if !errors.Is(err, errPrivilegeNotHeld) {
		return ""
	}
	return " (Windows grants this to an ordinary user only with Developer Mode on: " +
		"Settings > System > For developers > Developer Mode. Otherwise run as Administrator)"
}

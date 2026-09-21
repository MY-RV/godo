//go:build windows

package plugin

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// linkPath makes dst point at src using the link Windows actually offers.
//
// A symlink on Windows is a privilege, not a file operation: without Developer
// Mode or an elevated terminal the OS refuses one, so a plugin that links
// something worked on Unix and failed here. Windows has two links that need no
// privilege, and between them they cover what a symlink is used for:
//
//	a directory -> a junction (a mount-point reparse point)
//	a file      -> a hard link
//
// So that is what this uses. The mechanism is the platform's; the op is the
// same op.
//
// src must exist, because which link to make depends on what it is. That is
// the one place this cannot match Unix, where a link may dangle.
func linkPath(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s does not exist; on Windows the link is a junction for a directory "+
				"and a hard link for a file, so the target has to be there to say which", src)
		}
		return err
	}
	if info.IsDir() {
		return createJunction(src, dst)
	}
	if err := os.Link(src, dst); err != nil {
		return fmt.Errorf("%w (a hard link is how a file is linked on Windows; "+
			"it cannot cross volumes, so src and dst must be on the same drive)", err)
	}
	return nil
}

// createJunction makes dst a junction pointing at the directory src.
//
// A junction is a directory carrying a mount-point reparse point, so the
// directory is created first and the reparse point written into it. If writing
// it fails the empty directory is removed, because a bare directory where a
// link was asked for is worse than nothing.
func createJunction(src, dst string) error {
	target, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	if err := os.Mkdir(dst, 0o755); err != nil {
		return err
	}
	if err := writeMountPoint(dst, target); err != nil {
		os.Remove(dst)
		return err
	}
	return nil
}

// writeMountPoint sets the mount-point reparse point on an existing directory.
func writeMountPoint(dir, target string) error {
	buf, err := mountPointBuffer(target)
	if err != nil {
		return err
	}
	name, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}
	// BACKUP_SEMANTICS is what lets a directory be opened at all;
	// OPEN_REPARSE_POINT opens the directory itself rather than following a
	// reparse point that is already on it.
	h, err := windows.CreateFile(name, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)

	var returned uint32
	return windows.DeviceIoControl(h, windows.FSCTL_SET_REPARSE_POINT,
		&buf[0], uint32(len(buf)), nil, 0, &returned, nil)
}

package plugin

import (
	"encoding/binary"
	"fmt"
	"unicode/utf16"
)

// Sizes of REPARSE_DATA_BUFFER, which Windows defines as:
//
//	ULONG  ReparseTag;           4
//	USHORT ReparseDataLength;    2
//	USHORT Reserved;             2
//	USHORT SubstituteNameOffset; 2   <- the mount-point body starts here
//	USHORT SubstituteNameLength; 2
//	USHORT PrintNameOffset;      2
//	USHORT PrintNameLength;      2
//	WCHAR  PathBuffer[];
const (
	reparseHeader     = 8
	mountPointFields  = 8
	reparseTagMountPt = 0xA0000003 // IO_REPARSE_TAG_MOUNT_POINT
	maxReparseData    = 16384      // MAXIMUM_REPARSE_DATA_BUFFER_SIZE
)

// mountPointBuffer builds the reparse data that turns a directory into a
// junction pointing at target.
//
// It lives here, away from the syscall and without a build tag, because the
// layout is the part that is easy to get wrong and the part worth testing on
// whatever machine you happen to be on. The syscall around it is three lines.
//
// target must be an absolute path. PathBuffer holds both names, each
// NUL-terminated: the substitute name is the NT object path (\??\C:\x), which
// is what the filesystem follows, and the print name is the plain path, which
// is what Explorer shows.
func mountPointBuffer(target string) ([]byte, error) {
	sub := utf16.Encode([]rune(`\??\` + target))
	print16 := utf16.Encode([]rune(target))

	path := make([]byte, 0, (len(sub)+len(print16)+2)*2)
	path = appendUTF16(path, sub)
	printOffset := len(path)
	path = appendUTF16(path, print16)

	if reparseHeader+mountPointFields+len(path) > maxReparseData {
		return nil, fmt.Errorf("junction target is too long: %s", target)
	}

	buf := make([]byte, 0, reparseHeader+mountPointFields+len(path))
	buf = binary.LittleEndian.AppendUint32(buf, reparseTagMountPt)
	buf = binary.LittleEndian.AppendUint16(buf, uint16(mountPointFields+len(path)))
	buf = binary.LittleEndian.AppendUint16(buf, 0) // Reserved
	buf = binary.LittleEndian.AppendUint16(buf, 0) // SubstituteNameOffset
	buf = binary.LittleEndian.AppendUint16(buf, uint16(len(sub)*2))
	buf = binary.LittleEndian.AppendUint16(buf, uint16(printOffset))
	buf = binary.LittleEndian.AppendUint16(buf, uint16(len(print16)*2))
	return append(buf, path...), nil
}

// appendUTF16 writes code units little-endian and terminates them with a NUL,
// which the lengths above exclude and the offsets count.
func appendUTF16(dst []byte, s []uint16) []byte {
	for _, u := range s {
		dst = binary.LittleEndian.AppendUint16(dst, u)
	}
	return binary.LittleEndian.AppendUint16(dst, 0)
}

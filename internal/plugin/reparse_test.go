package plugin

import (
	"encoding/binary"
	"strings"
	"testing"
	"unicode/utf16"
)

// The layout is what Windows reads byte by byte, so it is checked field by
// field. Getting an offset wrong produces a directory that looks like a
// junction and points nowhere.
func TestMountPointBuffer(t *testing.T) {
	const target = `C:\Users\me\src\repo`
	buf, err := mountPointBuffer(target)
	if err != nil {
		t.Fatal(err)
	}

	tag := binary.LittleEndian.Uint32(buf[0:4])
	if tag != reparseTagMountPt {
		t.Fatalf("tag=%#x, want IO_REPARSE_TAG_MOUNT_POINT", tag)
	}
	dataLen := binary.LittleEndian.Uint16(buf[4:6])
	if int(dataLen) != len(buf)-reparseHeader {
		t.Fatalf("ReparseDataLength=%d, want %d (everything after the header)", dataLen, len(buf)-reparseHeader)
	}
	if r := binary.LittleEndian.Uint16(buf[6:8]); r != 0 {
		t.Fatalf("Reserved=%d, want 0", r)
	}

	subOff := binary.LittleEndian.Uint16(buf[8:10])
	subLen := binary.LittleEndian.Uint16(buf[10:12])
	printOff := binary.LittleEndian.Uint16(buf[12:14])
	printLen := binary.LittleEndian.Uint16(buf[14:16])
	path := buf[reparseHeader+mountPointFields:]

	if subOff != 0 {
		t.Fatalf("SubstituteNameOffset=%d, want 0", subOff)
	}
	// The substitute name is the NT object path: that prefix is what makes the
	// junction resolve at all.
	if got := decodeUTF16(path[subOff : subOff+subLen]); got != `\??\`+target {
		t.Fatalf("substitute name=%q", got)
	}
	if got := decodeUTF16(path[printOff : printOff+printLen]); got != target {
		t.Fatalf("print name=%q", got)
	}
	// Each name is NUL-terminated, and the lengths exclude the terminator —
	// which is why the print name starts two bytes past the end of the first.
	if int(printOff) != int(subLen)+2 {
		t.Fatalf("PrintNameOffset=%d, want %d (past the substitute name and its NUL)", printOff, subLen+2)
	}
	if u := binary.LittleEndian.Uint16(path[subLen : subLen+2]); u != 0 {
		t.Fatalf("substitute name is not NUL-terminated")
	}
	if u := binary.LittleEndian.Uint16(path[printOff+printLen:][:2]); u != 0 {
		t.Fatalf("print name is not NUL-terminated")
	}
}

// Windows caps reparse data at 16 KiB, and a refusal here is better than one
// from DeviceIoControl after the directory has already been created.
func TestMountPointBufferRefusesAnEnormousTarget(t *testing.T) {
	if _, err := mountPointBuffer(`C:\` + strings.Repeat("x", maxReparseData)); err == nil {
		t.Fatal("want an error")
	}
}

func decodeUTF16(b []byte) string {
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	return string(utf16.Decode(u))
}

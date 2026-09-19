package expand

import (
	"runtime"
	"strings"
)

// Quote renders s so the host shell reads it back as exactly one argument.
//
// The convention follows the shell the runner will use on this machine
// (sh -c on POSIX, cmd /C on Windows), so --preview output matches what runs.
//
// Windows caveat: cmd.exe expands %VAR% and delayed !VAR! before a command
// sees its arguments, and no quoting inside a command line fully suppresses
// that. Values containing % or ! are quoted defensively, but a catalog that
// must handle them exactly on Windows should not rely on cmd.
func Quote(s string) string {
	if runtime.GOOS == "windows" {
		return quoteWindows(s)
	}
	return quotePOSIX(s)
}

// shellSafePOSIX reports whether s can be interpolated bare into an sh line.
// Keeping common tokens unquoted is what makes previews readable: a preview of
// "godo test -v ./..." should read "go test -v ./...", not "go test '-v' './...'".
func shellSafePOSIX(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case strings.ContainsRune("_@%+=:,./-", r):
		default:
			return false
		}
	}
	return true
}

func quotePOSIX(s string) string {
	if shellSafePOSIX(s) {
		return s
	}
	// Single quotes are literal in sh; the only thing that cannot appear inside
	// them is a single quote, spliced back in as '\''.
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// shellSafeWindows is shellSafePOSIX minus the characters cmd.exe treats
// specially even inside double quotes.
func shellSafeWindows(s string) bool {
	if !shellSafePOSIX(s) {
		return false
	}
	return !strings.ContainsAny(s, "%!")
}

func quoteWindows(s string) string {
	if shellSafeWindows(s) {
		return s
	}
	// cmd.exe reads "" inside a quoted run as a literal quote. Backslashes
	// immediately before the closing quote would escape it, so they are doubled.
	q := strings.ReplaceAll(s, `"`, `""`)
	trail := len(q) - len(strings.TrimRight(q, `\`))
	q += strings.Repeat(`\`, trail)
	return `"` + q + `"`
}

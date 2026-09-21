package catalog

import (
	"errors"
	"fmt"
)

// Sentinel / typed errors for library consumers.
var (
	ErrNoMatch         = errors.New("no script matching tokens")
	ErrNoTokens        = errors.New("no script tokens")
	ErrUnexpectedArgs  = errors.New("unexpected args")
	ErrDependencyCycle = errors.New("dependency cycle")
	ErrInvalidCatalog  = errors.New("invalid catalog")
	ErrInvalidCapture  = errors.New("invalid capture name")
	ErrUnknownRunner   = errors.New("unknown runner")
)

// ExitError carries a process exit code from a failed command.
type ExitError struct {
	Code    int
	Message string
}

func (e *ExitError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("exit status %d", e.Code)
}

// ExitCode returns the process exit code. Uses errors.As for wrapped *ExitError.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	return 1
}

// IsCaptureName reports whether s is a valid ${nombre} identifier ([A-Za-z_][A-Za-z0-9_]*).
func IsCaptureName(s string) bool {
	if s == "" || s == "godo" {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r == '_':
			continue
		case i > 0 && r >= '0' && r <= '9':
			continue
		default:
			return false
		}
	}
	return true
}

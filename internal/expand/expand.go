package expand

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	reArgsSlice  = regexp.MustCompile(`\$\{godo:args\[(\d+)\.\.(\d+)\]\}`)
	reArgsIndex  = regexp.MustCompile(`\$\{godo:args\[(\d+)\]\}`)
	reArgsAll    = regexp.MustCompile(`\$\{godo:args\}`)
	reAnyPlace   = regexp.MustCompile(`\$\{([^}]*)\}`)
	reValidCap   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// HasArgsPlaceholder reports whether s references ${godo:args…}.
func HasArgsPlaceholder(s string) bool {
	return reArgsAll.MatchString(s) || reArgsIndex.MatchString(s) || reArgsSlice.MatchString(s)
}

// CommandsNeedArgs is true if any command uses ${godo:args…}.
func CommandsNeedArgs(cmds []string) bool {
	for _, c := range cmds {
		if HasArgsPlaceholder(c) {
			return true
		}
	}
	return false
}

// Expand applies captures and godo args placeholders in-process.
//
// ${godo:args[i..j]} uses half-open [i,j) (Go slice semantics).
// Out-of-range ${godo:args[i]} is an error. Unknown/invalid ${name} is an error.
func Expand(template string, captures map[string]string, args []string) (string, error) {
	var err error
	out := template

	out = reArgsSlice.ReplaceAllStringFunc(out, func(m string) string {
		if err != nil {
			return m
		}
		sub := reArgsSlice.FindStringSubmatch(m)
		i, _ := strconv.Atoi(sub[1])
		j, _ := strconv.Atoi(sub[2])
		if i < 0 || j < i {
			err = fmt.Errorf("invalid args slice [%d..%d)", i, j)
			return m
		}
		return strings.Join(safeSlice(args, i, j), " ")
	})
	if err != nil {
		return "", err
	}

	out = reArgsIndex.ReplaceAllStringFunc(out, func(m string) string {
		if err != nil {
			return m
		}
		sub := reArgsIndex.FindStringSubmatch(m)
		i, _ := strconv.Atoi(sub[1])
		if i < 0 || i >= len(args) {
			err = fmt.Errorf("${godo:args[%d]} out of range (len=%d)", i, len(args))
			return m
		}
		return args[i]
	})
	if err != nil {
		return "", err
	}

	out = reArgsAll.ReplaceAllString(out, strings.Join(args, " "))

	out = reAnyPlace.ReplaceAllStringFunc(out, func(m string) string {
		if err != nil {
			return m
		}
		inner := reAnyPlace.FindStringSubmatch(m)[1]
		if strings.HasPrefix(inner, "godo:") {
			err = fmt.Errorf("unhandled placeholder %s", m)
			return m
		}
		if !reValidCap.MatchString(inner) {
			err = fmt.Errorf("invalid capture name %q", inner)
			return m
		}
		if captures == nil {
			err = fmt.Errorf("unknown capture ${%s}", inner)
			return m
		}
		v, ok := captures[inner]
		if !ok {
			err = fmt.Errorf("unknown capture ${%s}", inner)
			return m
		}
		return v
	})
	return out, err
}

func safeSlice(args []string, i, j int) []string {
	if i < 0 {
		i = 0
	}
	if j > len(args) {
		j = len(args)
	}
	if i >= j || i >= len(args) {
		return nil
	}
	return args[i:j]
}

// ExpandAll expands each template.
func ExpandAll(templates []string, captures map[string]string, args []string) ([]string, error) {
	out := make([]string, len(templates))
	for i, t := range templates {
		e, err := Expand(t, captures, args)
		if err != nil {
			return nil, err
		}
		out[i] = e
	}
	return out, nil
}

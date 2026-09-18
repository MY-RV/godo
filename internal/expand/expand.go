// Package expand resolves godo placeholders before the host shell sees a line.
//
// godo works in two spaces, and each has its own syntax:
//
//	godo space   matcher keys and @deps entries. No shell is ever involved, so
//	             bare ${NAME} is a capture and values keep their token
//	             boundaries verbatim.
//	shell space  script bodies. The text belongs to the host shell, so godo
//	             claims only the ${godo:…} namespace and leaves every bare
//	             ${…} alone — "echo ${HOME}" reaches the shell untouched.
//
// Body grammar:
//
//	${godo:argv[NAME]}     capture bound by the matcher key
//	${godo:args}           all remaining args, space-joined
//	${godo:args[i]}        one remaining arg (0-based)
//	${godo:args[i..j]}     half-open slice [i,j) (Go semantics)
//	${…:raw}               any of the above, unquoted
//
// Values are shell-quoted by default: one argument in is one argument out,
// whatever it contains. The ":raw" suffix opts a single placeholder back into
// verbatim interpolation, for the cases where a glob or a shell construct is
// the point.
package expand

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// namespace is the only thing godo claims inside a script body.
const namespace = "${godo:"

var (
	reValidCap  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	reArgsIndex = regexp.MustCompile(`^args\[(\d+)\]$`)
	reArgsSlice = regexp.MustCompile(`^args\[(\d+)\.\.(\d+)\]$`)
	reArgv      = regexp.MustCompile(`^argv\[([^\]]*)\]$`)
	reDigits    = regexp.MustCompile(`^\d+$`)
)

// segment is one lexed piece of a body: literal text, or the spec of a
// ${godo:…} placeholder with the namespace prefix already stripped.
type segment struct {
	literal string
	spec    string
	isPlace bool
}

// lexBody splits a script body into literal and placeholder segments. Only
// ${godo:…} is a placeholder; everything else, bare ${…} included, is literal.
func lexBody(template string) ([]segment, error) {
	var segs []segment
	rest := template
	for {
		i := strings.Index(rest, namespace)
		if i < 0 {
			break
		}
		end := strings.IndexByte(rest[i:], '}')
		if end < 0 {
			return nil, fmt.Errorf("unterminated placeholder %q", rest[i:])
		}
		if i > 0 {
			segs = append(segs, segment{literal: rest[:i]})
		}
		segs = append(segs, segment{spec: rest[i+len(namespace) : i+end], isPlace: true})
		rest = rest[i+end+1:]
	}
	if rest != "" {
		segs = append(segs, segment{literal: rest})
	}
	return segs, nil
}

// hasArgsPlaceholder reports whether s references ${godo:args…}.
func hasArgsPlaceholder(s string) bool {
	segs, err := lexBody(s)
	if err != nil {
		return false
	}
	for _, seg := range segs {
		if seg.isPlace && strings.HasPrefix(strings.TrimSuffix(seg.spec, ":raw"), "args") {
			return true
		}
	}
	return false
}

// CommandsNeedArgs is true if any command uses ${godo:args…}.
func CommandsNeedArgs(cmds []string) bool {
	for _, c := range cmds {
		if hasArgsPlaceholder(c) {
			return true
		}
	}
	return false
}

// Expand resolves a script body (shell space).
//
// Values are shell-quoted unless the placeholder carries the ":raw" suffix.
// Out-of-range args and unknown captures are errors — expansion fails closed
// rather than emitting an empty string. Bare ${…} is never touched.
func Expand(template string, captures map[string]string, args []string) (string, error) {
	segs, err := lexBody(template)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	for _, seg := range segs {
		if !seg.isPlace {
			out.WriteString(seg.literal)
			continue
		}
		v, err := resolve(seg.spec, captures, args)
		if err != nil {
			return "", err
		}
		out.WriteString(v)
	}
	return out.String(), nil
}

// ExpandAll expands each body template.
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

// ExpandInvocation resolves a @deps entry (godo space) into tokens.
//
// Bare ${NAME} is a capture. Nothing is shell-quoted and nothing is re-split:
// one word in the entry is one token out, so a capture holding a space stays a
// single token instead of fragmenting the invocation.
func ExpandInvocation(line string, captures map[string]string) ([]string, error) {
	var tokens []string
	for _, word := range strings.Fields(line) {
		tok, err := expandWord(word, captures)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, tok)
	}
	return tokens, nil
}

func expandWord(word string, captures map[string]string) (string, error) {
	var out strings.Builder
	rest := word
	for {
		i := strings.Index(rest, "${")
		if i < 0 {
			break
		}
		end := strings.IndexByte(rest[i:], '}')
		if end < 0 {
			return "", fmt.Errorf("unterminated placeholder %q", rest[i:])
		}
		name := rest[i+2 : i+end]
		if strings.HasPrefix(name, "godo:") {
			return "", fmt.Errorf("${%s} is not available in @deps; deps are godo space, write ${NAME} for a capture", name)
		}
		v, err := lookupCapture(name, captures)
		if err != nil {
			return "", err
		}
		out.WriteString(rest[:i])
		out.WriteString(v)
		rest = rest[i+end+1:]
	}
	out.WriteString(rest)
	return out.String(), nil
}

// resolve turns one body placeholder spec into its replacement text.
func resolve(spec string, captures map[string]string, args []string) (string, error) {
	name, raw := spec, false
	if trimmed, ok := strings.CutSuffix(spec, ":raw"); ok {
		name, raw = trimmed, true
	}
	if name == "args" {
		return joinArgs(args, raw), nil
	}
	if m := reArgsIndex.FindStringSubmatch(name); m != nil {
		i, err := strconv.Atoi(m[1])
		if err != nil || i >= len(args) {
			return "", fmt.Errorf("${godo:args[%s]} out of range (len=%d)", m[1], len(args))
		}
		return quoteUnless(raw, args[i]), nil
	}
	if m := reArgsSlice.FindStringSubmatch(name); m != nil {
		i, errI := strconv.Atoi(m[1])
		j, errJ := strconv.Atoi(m[2])
		if errI != nil || errJ != nil || j < i {
			return "", fmt.Errorf("invalid args slice [%s..%s)", m[1], m[2])
		}
		return joinArgs(safeSlice(args, i, j), raw), nil
	}
	if m := reArgv.FindStringSubmatch(name); m != nil {
		if reDigits.MatchString(m[1]) {
			return "", fmt.Errorf("${godo:argv[%s]} indexes captures by name; for a positional argument use ${godo:args[%s]}", m[1], m[1])
		}
		v, err := lookupCapture(m[1], captures)
		if err != nil {
			return "", err
		}
		return quoteUnless(raw, v), nil
	}
	if name == "argv" {
		return "", fmt.Errorf("${godo:argv} needs a capture name, as ${godo:argv[NAME]}; for all remaining arguments use ${godo:args}")
	}
	return "", fmt.Errorf("unhandled placeholder ${godo:%s}", name)
}

func lookupCapture(name string, captures map[string]string) (string, error) {
	if !reValidCap.MatchString(name) {
		return "", fmt.Errorf("invalid capture name %q (want [A-Za-z_][A-Za-z0-9_]*)", name)
	}
	v, ok := captures[name]
	if !ok {
		return "", fmt.Errorf("unknown capture %q (not bound by the matcher key)", name)
	}
	return v, nil
}

func joinArgs(args []string, raw bool) string {
	if raw {
		return strings.Join(args, " ")
	}
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = Quote(a)
	}
	return strings.Join(quoted, " ")
}

func quoteUnless(raw bool, s string) string {
	if raw {
		return s
	}
	return Quote(s)
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

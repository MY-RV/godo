package catalog

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadFile parses a godo.yaml path into a Catalog (preserves script order + comments).
func LoadFile(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data, path)
}

// Parse parses godo.yaml bytes.
// dialects, if non-nil, restricts file/script dialects to registered names;
// nil → DefaultDialects().
func Parse(data []byte, path string, dialects ...*DialectRegistry) (*Catalog, error) {
	reg := DefaultDialects()
	if len(dialects) > 0 && dialects[0] != nil {
		reg = dialects[0]
	}

	var root yaml.Node
	if err := yaml.Unmarshal(normalizeBreaks(data), &root); err != nil {
		return nil, fmt.Errorf("%w: parse yaml: %v", ErrInvalidCatalog, err)
	}
	doc := &root
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return nil, fmt.Errorf("%w: empty yaml document", ErrInvalidCatalog)
		}
		doc = root.Content[0]
	}
	if doc.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: root must be a mapping", ErrInvalidCatalog)
	}

	cat := &Catalog{Path: path}
	var legacyDialect DialectName
	var scriptsNode *yaml.Node
	for i := 0; i < len(doc.Content); i += 2 {
		key := doc.Content[i]
		val := doc.Content[i+1]
		switch key.Value {
		case "version":
			cat.Version = strings.TrimSpace(scalarString(val))
		case "dialect":
			// Legacy position, kept because it shipped in 0.1 and 0.2.
			// engine.dialect is the current one and wins.
			legacyDialect = DialectName(strings.TrimSpace(scalarString(val)))
		case "engine":
			if err := val.Decode(&cat.Engine); err != nil {
				return nil, fmt.Errorf("%w: engine: %v", ErrInvalidCatalog, err)
			}
		case "scripts":
			scriptsNode = val
		}
	}
	if cat.Version == "" {
		return nil, fmt.Errorf("%w: version is required", ErrInvalidCatalog)
	}
	if err := validateEngine(cat.Engine); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCatalog, err)
	}
	cat.Dialect = cat.Engine.Dialect
	if cat.Dialect == "" {
		cat.Dialect = legacyDialect
	}
	cat.Runner = cat.Engine.Runner
	if cat.Dialect == "" {
		cat.Dialect = DialectPackage
	}
	// An unset runner stays unset: it means "whatever this engine runs with",
	// which is the Runner injected into it. Naming a default here would bake a
	// choice that belongs to the caller — godo the CLI runs your own shell,
	// while an embedder runs what it wired up.
	//
	// Runner names are not validated here either. A dialect has to resolve
	// before a script can be matched at all, so an unknown one is a load error;
	// a runner is only needed to execute, and the registry that could answer
	// for it belongs to the Engine. An unknown runner fails at plan time.
	if _, err := reg.Lookup(cat.Dialect); err != nil {
		return nil, fmt.Errorf("%w: dialect %q not implemented", ErrInvalidCatalog, cat.Dialect)
	}
	if scriptsNode == nil {
		return cat, nil
	}
	if scriptsNode.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: scripts must be a mapping", ErrInvalidCatalog)
	}

	seen := map[string]bool{}
	for i := 0; i < len(scriptsNode.Content); i += 2 {
		keyNode := scriptsNode.Content[i]
		valNode := scriptsNode.Content[i+1]
		script, err := scriptFromNodes(keyNode, valNode, cat.Dialect, reg, path)
		if err != nil {
			return nil, err
		}
		if seen[script.Key] {
			return nil, fmt.Errorf("%w: duplicate script key %q", ErrInvalidCatalog, script.Key)
		}
		seen[script.Key] = true
		cat.Scripts = append(cat.Scripts, script)
	}
	return cat, nil
}

// normalizeBreaks turns CRLF into LF before the YAML parser sees it.
//
// A catalog written on Windows has CRLF, and the parser files a comment
// differently for it: a decorator directly above its key became the *previous*
// key's foot comment instead of that key's head comment, so it decorated
// nothing. A blank line above the comment happened to hide it, which is why it
// read as "no space between the command and the comment breaks it".
//
// YAML treats CRLF as a line break, so normalizing is what the format already
// says. It also keeps a \r out of block scalars, where it would otherwise be
// handed to the shell as part of the command.
func normalizeBreaks(data []byte) []byte {
	if !bytes.Contains(data, []byte("\r\n")) {
		return data
	}
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}

func scriptFromNodes(key, val *yaml.Node, fileDialect DialectName, reg *DialectRegistry, path string) (Script, error) {
	s := Script{Key: key.Value}
	if err := rejectMisplacedDecorators(key, val); err != nil {
		return Script{}, fmt.Errorf("%w: script %q: %v", ErrInvalidCatalog, s.Key, err)
	}
	dec, err := parseDecorators(key.HeadComment)
	if err != nil {
		return Script{}, fmt.Errorf("%w: script %q: %v", ErrInvalidCatalog, s.Key, err)
	}
	s.Doc = dec.doc
	s.Deps = dec.deps
	s.Dialect = dec.dialect
	s.Runner = dec.runner
	if s.Dialect != "" {
		if _, err := reg.Lookup(DialectName(s.Dialect)); err != nil {
			return Script{}, fmt.Errorf("%w: script %q: @dialect %q not implemented", ErrInvalidCatalog, s.Key, s.Dialect)
		}
	}
	cmds, err := commandsFromValue(val)
	if err != nil {
		return Script{}, fmt.Errorf("%w: script %q: %v", ErrInvalidCatalog, s.Key, err)
	}
	if len(cmds) == 0 {
		return Script{}, fmt.Errorf("%w: script %q: empty command list", ErrInvalidCatalog, s.Key)
	}
	cmds, err = resolveIncludes(cmds, filepath.Dir(path), s.Key)
	if err != nil {
		return Script{}, err
	}
	s.Commands = cmds

	eff := EffectiveDialect(s, fileDialect)
	if eff == DialectPackage {
		if err := rejectCapturesInPackageKey(s.Key); err != nil {
			return Script{}, fmt.Errorf("%w: script %q: %v (use @dialect matcher)", ErrInvalidCatalog, s.Key, err)
		}
	}
	if eff == DialectMatcher {
		if err := validateMatcherKey(s.Key); err != nil {
			return Script{}, fmt.Errorf("%w: script %q: %v", ErrInvalidCatalog, s.Key, err)
		}
	}
	return s, nil
}

func rejectCapturesInPackageKey(key string) error {
	for _, part := range strings.Fields(key) {
		if strings.HasPrefix(part, "${") {
			return fmt.Errorf("package keys must be literals, found %q", part)
		}
	}
	return nil
}

func validateMatcherKey(key string) error {
	for _, part := range strings.Fields(key) {
		if !strings.HasPrefix(part, "${") {
			continue
		}
		if strings.HasPrefix(part, "${godo:") {
			return fmt.Errorf("pattern must not use ${godo:…} as a capture token")
		}
		if !strings.HasSuffix(part, "}") {
			return fmt.Errorf("malformed capture %q", part)
		}
		inner := part[2 : len(part)-1]
		if !IsCaptureName(inner) {
			return fmt.Errorf("%w: %q (want [A-Za-z_][A-Za-z0-9_]*)", ErrInvalidCapture, inner)
		}
	}
	return nil
}

// decoratorNames are the @-words parseDecorators answers to.
var decoratorNames = []string{"@deps", "@dependencies", "@dialect", "@runner"}

// rejectMisplacedDecorators fails on a decorator YAML puts somewhere godo does
// not read.
//
// Only the comment block *above* a key decorates it. A decorator written at
// the end of the line lands on the value as a line comment, and one written
// after the last key lands on that key as a foot comment; both were read by
// nobody and dropped in silence. The catalog then ran with a dialect or a
// dependency list its author believed they had written, which surfaces far
// from the line that caused it — a script matching nothing, or a dependency
// that never runs.
//
// So it is an error, and the error says where the decorator belongs. A comment
// only trips this if it starts with a decorator godo knows: prose mentioning
// "@deps" is a comment, and stays one.
func rejectMisplacedDecorators(key, val *yaml.Node) error {
	places := []struct {
		comment string
		where   string
	}{
		{key.LineComment, "at the end of the line"},
		{val.LineComment, "at the end of the line"},
		{key.FootComment, "below the key"},
		{val.FootComment, "below the key"},
	}
	for _, p := range places {
		if name := leadingDecorator(p.comment); name != "" {
			return fmt.Errorf("%s is %s, where it is not read; a decorator goes on its own line above the key", name, p.where)
		}
	}
	return nil
}

// leadingDecorator returns the decorator a comment opens with, or "".
func leadingDecorator(comment string) string {
	for _, line := range strings.Split(comment, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "#"))
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		for _, name := range decoratorNames {
			if fields[0] == name {
				return name
			}
		}
	}
	return ""
}

// decorators is the parsed @-block above a script key.
type decorators struct {
	doc     string
	deps    []string
	dialect string
	runner  string
}

func parseDecorators(headComment string) (decorators, error) {
	var dec decorators
	if headComment == "" {
		return dec, nil
	}
	var docLines []string
	for _, line := range strings.Split(headComment, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "#")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "@") {
			fields := strings.Fields(line)
			if len(fields) == 0 {
				continue
			}
			switch fields[0] {
			case "@deps", "@dependencies":
				rest := strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
				dec.deps = append(dec.deps, splitDepsList(rest)...)
			case "@dialect":
				if len(fields) < 2 || fields[1] == "" {
					return decorators{}, fmt.Errorf("@dialect requires a name")
				}
				dec.dialect = fields[1]
			case "@runner":
				if len(fields) < 2 || fields[1] == "" {
					return decorators{}, fmt.Errorf("@runner requires a name")
				}
				dec.runner = fields[1]
			default:
				return decorators{}, fmt.Errorf("unknown decorator %s", fields[0])
			}
			continue
		}
		docLines = append(docLines, line)
	}
	dec.doc = strings.Join(docLines, "\n")
	return dec, nil
}

func splitDepsList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func commandsFromValue(n *yaml.Node) ([]string, error) {
	switch n.Kind {
	case yaml.ScalarNode:
		if n.Tag == "!!null" || (n.Value == "" && n.Tag == "!!null") {
			return nil, fmt.Errorf("value must be string or string list")
		}
		// Reject bare YAML bool/int used as command by mistake only when tag is non-string?
		// Allow any scalar as command text (including "true").
		return []string{n.Value}, nil
	case yaml.SequenceNode:
		if len(n.Content) == 0 {
			return nil, fmt.Errorf("empty command list")
		}
		out := make([]string, 0, len(n.Content))
		for _, c := range n.Content {
			if c.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("command list entries must be strings")
			}
			out = append(out, c.Value)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("value must be string or string list")
	}
}

func scalarString(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	return n.Value
}

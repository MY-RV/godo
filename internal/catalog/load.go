package catalog

import (
	"fmt"
	"os"
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
	if err := yaml.Unmarshal(data, &root); err != nil {
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
	var scriptsNode *yaml.Node
	for i := 0; i < len(doc.Content); i += 2 {
		key := doc.Content[i]
		val := doc.Content[i+1]
		switch key.Value {
		case "version":
			cat.Version = strings.TrimSpace(scalarString(val))
		case "dialect":
			cat.Dialect = DialectName(strings.TrimSpace(scalarString(val)))
		case "scripts":
			scriptsNode = val
		}
	}
	if cat.Version == "" {
		return nil, fmt.Errorf("%w: version is required", ErrInvalidCatalog)
	}
	if cat.Dialect == "" {
		cat.Dialect = DialectPackage
	}
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
		script, err := scriptFromNodes(keyNode, valNode, cat.Dialect, reg)
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

func scriptFromNodes(key, val *yaml.Node, fileDialect DialectName, reg *DialectRegistry) (Script, error) {
	s := Script{Key: key.Value}
	doc, deps, dialect, err := parseDecorators(key.HeadComment)
	if err != nil {
		return Script{}, fmt.Errorf("%w: script %q: %v", ErrInvalidCatalog, s.Key, err)
	}
	s.Doc = doc
	s.Deps = deps
	s.Dialect = dialect
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

func parseDecorators(headComment string) (doc string, deps []string, dialect string, err error) {
	if headComment == "" {
		return "", nil, "", nil
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
				deps = append(deps, splitDepsList(rest)...)
			case "@dialect":
				if len(fields) < 2 || fields[1] == "" {
					return "", nil, "", fmt.Errorf("@dialect requires a name")
				}
				dialect = fields[1]
			default:
				return "", nil, "", fmt.Errorf("unknown decorator %s", fields[0])
			}
			continue
		}
		docLines = append(docLines, line)
	}
	return strings.Join(docLines, "\n"), deps, dialect, nil
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

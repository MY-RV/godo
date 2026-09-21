package catalog

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// PluginEntry is a plugin to write into a catalog.
type PluginEntry struct {
	Source   string
	SHA256   string
	Provides []string
	Config   string // raw YAML lines for the config block, or ""
}

// UpsertPlugin returns src with entry added under engine.plugins, replacing
// any existing entry that provides the same things.
//
// Installing a plugin a catalog already declares means bringing the
// declaration in line with what was just fetched — a placeholder digest
// becoming a real one is the ordinary case. Appending instead would leave two
// entries claiming one runner, which the loader rejects.
func UpsertPlugin(src []byte, entry PluginEntry) ([]byte, error) {
	if len(entry.Provides) == 0 {
		return InsertPlugin(src, entry)
	}
	doc, err := docNode(src)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(src), "\n")
	_, engineVal := childNode(doc, "engine")
	if engineVal == nil || engineVal.Kind != yaml.MappingNode {
		return InsertPlugin(src, entry)
	}
	_, pluginsVal := childNode(engineVal, "plugins")
	if pluginsVal == nil || pluginsVal.Kind != yaml.SequenceNode {
		return InsertPlugin(src, entry)
	}
	for _, item := range pluginsVal.Content {
		if !sameProvides(item, entry.Provides) {
			continue
		}
		from := item.Line - 1
		to := endOfNode(item, lines)
		// The config that is already there stays. What this plugin may do is
		// the catalog author's decision, reached once and often narrowed;
		// resetting it to a default on every install would quietly re-grant
		// what someone took away, and quietly drop what they added.
		if kept := configLines(item, lines); kept != "" {
			entry.Config = kept
		}
		block := entryLines(entry, strings.Repeat(" ", item.Column-3)+"item")
		out := append([]string{}, lines[:from]...)
		out = append(out, block...)
		out = append(out, lines[to:]...)
		return []byte(strings.Join(out, "\n")), nil
	}
	return InsertPlugin(src, entry)
}

// configLines returns an entry's config block, re-indented to sit under a
// fresh entry, or "" when it declares none.
func configLines(item *yaml.Node, lines []string) string {
	key, val := childNode(item, "config")
	if key == nil || val == nil {
		return ""
	}
	from := key.Line // the line after "config:"
	to := endOfNode(val, lines)
	if from >= to || to > len(lines) {
		return ""
	}
	strip := val.Column - 1
	var out []string
	for _, l := range lines[from:to] {
		if len(l) >= strip {
			l = l[strip:]
		} else {
			l = strings.TrimLeft(l, " ")
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// sameProvides reports whether a plugin entry node claims exactly these names.
func sameProvides(item *yaml.Node, want []string) bool {
	if item.Kind != yaml.MappingNode {
		return false
	}
	_, val := childNode(item, "provides")
	if val == nil || val.Kind != yaml.SequenceNode || len(val.Content) != len(want) {
		return false
	}
	for i, n := range val.Content {
		if n.Value != want[i] {
			return false
		}
	}
	return true
}

// InsertPlugin returns src with entry added under engine.plugins.
//
// It splices lines rather than re-encoding the document. A catalog is written
// by hand: its comments carry the @deps and @runner decorators, its blank lines
// group scripts, and its block scalars hold whitespace that means something.
// Round-tripping through a YAML encoder would preserve the data and lose the
// file, so the parser is used only to find *where* to write.
func InsertPlugin(src []byte, entry PluginEntry) ([]byte, error) {
	if entry.Source == "" || entry.SHA256 == "" {
		return nil, fmt.Errorf("plugin entry needs a source and a sha256")
	}
	doc, err := docNode(src)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(src), "\n")
	at, indent, err := pluginsAnchor(doc, lines)
	if err != nil {
		return nil, err
	}
	block := entryLines(entry, indent)
	out := append([]string{}, lines[:at]...)
	out = append(out, block...)
	out = append(out, lines[at:]...)
	return []byte(strings.Join(out, "\n")), nil
}

// pluginsAnchor finds the line to insert before, and the indent to write at.
//
// Three shapes, in the order they are looked for: an engine.plugins list to
// append to, an engine block that needs the key, and a file with no engine
// block at all.
func pluginsAnchor(doc *yaml.Node, lines []string) (at int, indent string, err error) {
	engineKey, engineVal := childNode(doc, "engine")
	if engineKey == nil {
		// No engine block: open one above scripts, or at the end.
		at = len(lines)
		if k, _ := childNode(doc, "scripts"); k != nil {
			at = k.Line - 1
			for at > 0 && strings.TrimSpace(lines[at-1]) == "" {
				at--
			}
		}
		return at, "engine", nil
	}
	if engineVal == nil || engineVal.Kind != yaml.MappingNode {
		return 0, "", fmt.Errorf("%w: engine must be a mapping", ErrInvalidCatalog)
	}

	pluginsKey, pluginsVal := childNode(engineVal, "plugins")
	if pluginsKey == nil {
		// engine exists but declares no plugins: add the key at its indent.
		return endOfNode(engineVal, lines), strings.Repeat(" ", engineVal.Column-1) + "plugins", nil
	}
	if pluginsVal == nil || pluginsVal.Kind != yaml.SequenceNode || len(pluginsVal.Content) == 0 {
		return pluginsKey.Line, strings.Repeat(" ", pluginsKey.Column+1) + "item", nil
	}
	last := pluginsVal.Content[len(pluginsVal.Content)-1]
	return endOfNode(last, lines), strings.Repeat(" ", last.Column-3) + "item", nil
}

// endOfNode returns the line just past a node's last content line.
//
// yaml.Node carries where a value starts, never where it ends, so the end is
// found by walking down past everything indented under it. Blank lines are
// walked over but not claimed: an entry belongs above the gap that separates
// it from what follows.
func endOfNode(n *yaml.Node, lines []string) int {
	col := n.Column
	last := n.Line
	for i := n.Line; i < len(lines); i++ {
		text := lines[i]
		if strings.TrimSpace(text) == "" {
			continue
		}
		if len(text)-len(strings.TrimLeft(text, " ")) < col-1 {
			break
		}
		last = i + 1
	}
	return last
}

func docNode(src []byte) (*yaml.Node, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
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
	return doc, nil
}

func childNode(m *yaml.Node, key string) (k, v *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i], m.Content[i+1]
		}
	}
	return nil, nil
}

// entryLines renders the entry at the depth its anchor asked for.
func entryLines(e PluginEntry, indent string) []string {
	var pad, out []string
	switch {
	case indent == "engine":
		out = append(out, "engine:", "  plugins:")
		pad = []string{"    "}
	case strings.HasSuffix(indent, "plugins"):
		base := strings.TrimSuffix(indent, "plugins")
		out = append(out, base+"plugins:")
		pad = []string{base + "  "}
	default:
		pad = []string{strings.TrimSuffix(indent, "item")}
	}
	p := pad[0]
	out = append(out,
		p+"- source: "+e.Source,
		p+"  sha256: "+e.SHA256,
	)
	if len(e.Provides) > 0 {
		out = append(out, p+"  provides: ["+strings.Join(e.Provides, ", ")+"]")
	}
	if e.Config != "" {
		out = append(out, p+"  config:")
		for _, l := range strings.Split(strings.TrimRight(e.Config, "\n"), "\n") {
			out = append(out, p+"    "+l)
		}
	}
	if indent == "engine" {
		out = append(out, "")
	}
	return out
}

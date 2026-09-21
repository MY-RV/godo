package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// filePrefix and fileSuffix bracket an included body.
const (
	filePrefix = "${godo:file("
	fileSuffix = ")}"
)

// resolveIncludes replaces a body that is exactly ${godo:file(path)} with the
// contents of that file.
//
// This is inclusion, not expansion: it happens when a body is read rather than
// when it is rendered, which is why it applies to every runner — a plugin body
// is never expanded, and still has to be able to live in a file.
//
// After this, an included body is indistinguishable from one pasted into the
// YAML. Whatever a runner does with ${godo:…} it does here too; inclusion
// changes where the text comes from, never what happens to it next.
//
// Only a whole value is accepted. Splicing a file into the middle of a line
// would paste newlines into a shell command and mean something different every
// time; a body that is a file is a body that is a file.
func resolveIncludes(cmds []string, dir, key string) ([]string, error) {
	out := make([]string, 0, len(cmds))
	for _, c := range cmds {
		trimmed := strings.TrimSpace(c)
		if !strings.HasPrefix(trimmed, filePrefix) {
			if i := strings.Index(c, filePrefix); i >= 0 {
				return nil, fmt.Errorf("%w: script %q: %s…%s must be the whole value, not part of a line",
					ErrInvalidCatalog, key, filePrefix, fileSuffix)
			}
			out = append(out, c)
			continue
		}
		if !strings.HasSuffix(trimmed, fileSuffix) {
			return nil, fmt.Errorf("%w: script %q: unterminated %s…%s", ErrInvalidCatalog, key, filePrefix, fileSuffix)
		}
		rel := strings.TrimSpace(trimmed[len(filePrefix) : len(trimmed)-len(fileSuffix)])
		if rel == "" {
			return nil, fmt.Errorf("%w: script %q: %s needs a path", ErrInvalidCatalog, key, filePrefix)
		}
		body, err := readInclude(dir, rel)
		if err != nil {
			return nil, fmt.Errorf("%w: script %q: %v", ErrInvalidCatalog, key, err)
		}
		out = append(out, body)
	}
	return out, nil
}

// readInclude reads a path relative to the catalog.
//
// Relative to the catalog and not to the caller's cwd: a script says where its
// body lives, and that does not change with where godo was run from. An
// absolute path is taken as given.
func readInclude(dir, rel string) (string, error) {
	p := rel
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

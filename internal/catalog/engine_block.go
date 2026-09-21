package catalog

import (
	"fmt"
	"strings"
)

// EngineSpec is the `engine:` block: what godo itself needs in order to run
// this catalog.
//
// Everything else in the file describes the scripts — which one answers your
// tokens, how its body runs. This describes the tool: the binary a catalog
// expects, and the plugins it wants loaded. Keeping them apart is what lets
// the toolchain side grow (a lockfile, a package manager) without the script
// side growing with it.
//
// Named EngineSpec, not Engine, because Engine is already the thing that
// resolves and runs a catalog.
type EngineSpec struct {
	// Version is the minimum godo binary, "0.3.0" or ">=0.3.0".
	Version string `yaml:"version"`
	// Dialect and Runner configure how godo reads and runs this file. They are
	// here rather than at the top level because they are settings for the tool,
	// not content of the catalog: scripts: is the data, engine: is the dial.
	Dialect DialectName `yaml:"dialect"`
	Runner  RunnerName  `yaml:"runner"`
	Plugins []Plugin    `yaml:"plugins"`
}

// Plugin is one entry of `engine.plugins`.
//
// Nothing loads these yet. They are parsed and validated so the shape is
// settled and a catalog can already declare what it expects; a script asking
// for a runner a plugin provides fails with that plugin named, rather than
// with "unknown runner".
type Plugin struct {
	Source string `yaml:"source"`
	// SHA256 is required. A plugin is third-party code that runs when someone
	// types `godo test`; without a digest there is nothing to verify it is the
	// code that was reviewed.
	SHA256 string `yaml:"sha256"`
	// Provides entries are "<kind>:<name>", kind being runner or dialect.
	Provides []string `yaml:"provides"`
	// Config is optional and entirely the plugin's: its keys, its meaning, its
	// defaults. godo carries it across and does not read it. What a plugin
	// treats as deny-all, or ignores, is the plugin's contract, not godo's.
	Config map[string]any `yaml:"config"`
}

// Minimum returns the minimum binary version the catalog asks for, and whether
// it asked at all.
func (e EngineSpec) Minimum() (string, bool) {
	v := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(e.Version), ">="))
	return v, v != ""
}

// ProviderOf returns the plugin that says it provides "<kind>:<name>".
func (e EngineSpec) ProviderOf(kind, name string) (Plugin, bool) {
	want := kind + ":" + name
	for _, p := range e.Plugins {
		for _, got := range p.Provides {
			if got == want {
				return p, true
			}
		}
	}
	return Plugin{}, false
}

// validateEngine checks the block at load time.
func validateEngine(e EngineSpec) error {
	if v := strings.TrimSpace(e.Version); v != "" {
		rest := strings.TrimSpace(strings.TrimPrefix(v, ">="))
		if rest == "" {
			return fmt.Errorf("engine.version %q: want a version, as \"0.3.0\" or \">=0.3.0\"", e.Version)
		}
		if strings.ContainsAny(rest, "<>=~^ ") {
			return fmt.Errorf("engine.version %q: only a minimum is supported, as \"0.3.0\" or \">=0.3.0\"", e.Version)
		}
	}
	seen := map[string]string{}
	for i, p := range e.Plugins {
		where := fmt.Sprintf("engine.plugins[%d]", i)
		if strings.TrimSpace(p.Source) == "" {
			return fmt.Errorf("%s: source is required", where)
		}
		if strings.TrimSpace(p.SHA256) == "" {
			return fmt.Errorf("%s (%s): sha256 is required", where, p.Source)
		}
		if len(p.Provides) == 0 {
			return fmt.Errorf("%s (%s): provides is required, as [runner:name]", where, p.Source)
		}
		for _, entry := range p.Provides {
			kind, name, ok := strings.Cut(entry, ":")
			if !ok || name == "" {
				return fmt.Errorf("%s (%s): provides %q: want \"<kind>:<name>\"", where, p.Source, entry)
			}
			switch kind {
			case "runner", "dialect":
			default:
				return fmt.Errorf("%s (%s): provides %q: kind must be runner or dialect", where, p.Source, entry)
			}
			if prev, dup := seen[entry]; dup {
				return fmt.Errorf("%s (%s): provides %q already provided by %s", where, p.Source, entry, prev)
			}
			seen[entry] = p.Source
		}
	}
	return nil
}

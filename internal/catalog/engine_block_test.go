package catalog_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/my-rv/godo/internal/catalog"
)

const pluginBlock = `version: "0.1"

engine:
  version: ">=0.3.0"
  plugins:
    - source: https://example.test/godo-micropy@v1.2.0
      sha256: deadbeef
      provides: [runner:micropy]
      config:
        proc: {exec: true, spawn: false}
        stdlib: [os, os.path]

scripts:
  # @runner micropy
  t: print("hi")
`

func TestParse_engineBlock(t *testing.T) {
	cat, err := catalog.Parse([]byte(pluginBlock), "x")
	if err != nil {
		t.Fatal(err)
	}
	min, ok := cat.Engine.Minimum()
	if !ok || min != "0.3.0" {
		t.Fatalf("minimum=%q ok=%v", min, ok)
	}
	if len(cat.Engine.Plugins) != 1 {
		t.Fatalf("plugins=%+v", cat.Engine.Plugins)
	}
	p := cat.Engine.Plugins[0]
	if p.SHA256 != "deadbeef" || p.Source == "" {
		t.Fatalf("plugin=%+v", p)
	}
	// Config is the plugin's own; godo carries it without interpreting it.
	if _, ok := p.Config["proc"]; !ok {
		t.Fatalf("config=%+v", p.Config)
	}
	got, ok := cat.Engine.ProviderOf("runner", "micropy")
	if !ok || got.Source != p.Source {
		t.Fatalf("ProviderOf=%+v %v", got, ok)
	}
	if _, ok := cat.Engine.ProviderOf("runner", "nope"); ok {
		t.Fatal("ProviderOf matched a name nothing provides")
	}
}

func TestParse_engineMinimumAcceptsBareVersion(t *testing.T) {
	cat, err := catalog.Parse([]byte("version: \"0.1\"\nengine:\n  version: 0.3.0\nscripts:\n  t: echo hi\n"), "x")
	if err != nil {
		t.Fatal(err)
	}
	if min, ok := cat.Engine.Minimum(); !ok || min != "0.3.0" {
		t.Fatalf("minimum=%q ok=%v", min, ok)
	}
}

func TestParse_noEngineBlockAsksForNothing(t *testing.T) {
	cat, err := catalog.Parse([]byte("version: \"0.1\"\nscripts:\n  t: echo hi\n"), "x")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cat.Engine.Minimum(); ok {
		t.Fatal("a catalog with no engine block must ask for no version")
	}
	if len(cat.Engine.Plugins) != 0 {
		t.Fatalf("plugins=%+v", cat.Engine.Plugins)
	}
}

func TestParse_engineRejects(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"no source", "engine:\n  plugins:\n    - sha256: a\n      provides: [runner:x]\n", "source is required"},
		{"no digest", "engine:\n  plugins:\n    - source: s\n      provides: [runner:x]\n", "sha256 is required"},
		{"no provides", "engine:\n  plugins:\n    - source: s\n      sha256: a\n", "provides is required"},
		{"bad kind", "engine:\n  plugins:\n    - source: s\n      sha256: a\n      provides: [wat:x]\n", "kind must be runner or dialect"},
		{"no name", "engine:\n  plugins:\n    - source: s\n      sha256: a\n      provides: [runner]\n", `want "<kind>:<name>"`},
		{"range syntax", "engine:\n  version: \"^1.2\"\n", "only a minimum is supported"},
		{"empty constraint", "engine:\n  version: \">=\"\n", "want a version"},
		{
			"duplicate provides",
			"engine:\n  plugins:\n    - source: a\n      sha256: x\n      provides: [runner:m]\n" +
				"    - source: b\n      sha256: y\n      provides: [runner:m]\n",
			"already provided by a",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := catalog.Parse([]byte("version: \"0.1\"\n"+tc.src+"scripts:\n  t: echo hi\n"), "x")
			if err == nil || !errors.Is(err, catalog.ErrInvalidCatalog) {
				t.Fatalf("err=%v", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v, want %q", err, tc.want)
			}
		})
	}
}

// A runner a plugin declares fails by naming the plugin, not as a typo.
func TestBuildPlan_pluginRunnerNamesItsPlugin(t *testing.T) {
	cat, err := catalog.Parse([]byte(pluginBlock), "x")
	if err != nil {
		t.Fatal(err)
	}
	eng := catalog.NewEngine(cat, runnerFunc(func(string) error { return nil }))
	_, err = eng.BuildPlan([]string{"t"})
	if !errors.Is(err, catalog.ErrUnknownRunner) {
		t.Fatalf("err=%v", err)
	}
	for _, want := range []string{"micropy", "godo-micropy", "cannot load plugins"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("err=%v, missing %q", err, want)
		}
	}
}

// dialect and runner live in engine:.
func TestParse_engineDialectAndRunner(t *testing.T) {
	src := "version: \"0.1\"\nengine:\n  dialect: matcher\n  runner: bash\nscripts:\n  t ${M}: echo ${godo:argv[M]}\n"
	cat, err := catalog.Parse([]byte(src), "x")
	if err != nil {
		t.Fatal(err)
	}
	if cat.Dialect != catalog.DialectMatcher {
		t.Fatalf("dialect=%q", cat.Dialect)
	}
	if cat.Runner != "bash" {
		t.Fatalf("runner=%q", cat.Runner)
	}
}

// Top-level dialect: shipped in 0.1 and 0.2, so it keeps working.
func TestParse_legacyTopLevelDialect(t *testing.T) {
	cat, err := catalog.Parse([]byte("version: \"0.1\"\ndialect: matcher\nscripts:\n  t ${M}: echo hi\n"), "x")
	if err != nil {
		t.Fatal(err)
	}
	if cat.Dialect != catalog.DialectMatcher {
		t.Fatalf("dialect=%q", cat.Dialect)
	}
}

// engine.dialect wins when both are present.
func TestParse_engineDialectBeatsLegacy(t *testing.T) {
	src := "version: \"0.1\"\ndialect: package\nengine:\n  dialect: matcher\nscripts:\n  t ${M}: echo hi\n"
	cat, err := catalog.Parse([]byte(src), "x")
	if err != nil {
		t.Fatal(err)
	}
	if cat.Dialect != catalog.DialectMatcher {
		t.Fatalf("dialect=%q", cat.Dialect)
	}
}

// config is the plugin's: optional, and godo carries it without reading it.
func TestParse_pluginConfigIsOptionalAndUninterpreted(t *testing.T) {
	src := "version: \"0.1\"\nengine:\n  plugins:\n" +
		"    - source: s\n      sha256: a\n      provides: [runner:x]\n" +
		"    - source: t\n      sha256: b\n      provides: [runner:y]\n" +
		"      config: {anything: [1, 2], nested: {deep: true}}\n" +
		"scripts:\n  t: echo hi\n"
	cat, err := catalog.Parse([]byte(src), "x")
	if err != nil {
		t.Fatal(err)
	}
	if cat.Engine.Plugins[0].Config != nil {
		t.Fatalf("absent config should stay nil, got %+v", cat.Engine.Plugins[0].Config)
	}
	cfg := cat.Engine.Plugins[1].Config
	if _, ok := cfg["anything"]; !ok {
		t.Fatalf("config=%+v", cfg)
	}
	if _, ok := cfg["nested"]; !ok {
		t.Fatalf("config=%+v", cfg)
	}
}

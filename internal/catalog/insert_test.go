package catalog_test

import (
	"strings"
	"testing"

	"github.com/my-rv/godo/internal/catalog"
)

var entry = catalog.PluginEntry{
	Source:   "./micropy.wasm",
	SHA256:   "abc123",
	Provides: []string{"runner:micropy"},
	Config:   "proc: {exec: true}\nfs:   {mount: true}",
}

// The catalog must survive the edit: its comments carry decorators, its blank
// lines group scripts, and a block scalar's whitespace is content.
func TestInsertPlugin_keepsEverythingElse(t *testing.T) {
	src := `version: "0.1"

engine:
  dialect: matcher

scripts:
  # Local gate
  # @deps vet, test
  ci: go build ./...

  # @runner micropy
  boot: |
    import os
        deliberately indented
    print("hi")
`
	got, err := catalog.InsertPlugin([]byte(src), entry)
	if err != nil {
		t.Fatal(err)
	}
	out := string(got)
	for _, want := range []string{
		"# Local gate", "# @deps vet, test", "# @runner micropy",
		"    import os", "        deliberately indented",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("lost %q from:\n%s", want, out)
		}
	}
	cat, err := catalog.Parse(got, "x")
	if err != nil {
		t.Fatalf("result does not parse: %v\n%s", err, out)
	}
	if len(cat.Engine.Plugins) != 1 || cat.Engine.Plugins[0].SHA256 != "abc123" {
		t.Fatalf("plugins=%+v", cat.Engine.Plugins)
	}
	if cat.Dialect != catalog.DialectMatcher {
		t.Fatalf("dialect lost: %q", cat.Dialect)
	}
	if len(cat.Scripts) != 2 {
		t.Fatalf("scripts=%d", len(cat.Scripts))
	}
	if cat.Scripts[0].Doc != "Local gate" || len(cat.Scripts[0].Deps) != 2 {
		t.Fatalf("decorators lost: %+v", cat.Scripts[0])
	}
	if cat.Scripts[1].Runner != "micropy" {
		t.Fatalf("@runner lost: %+v", cat.Scripts[1])
	}
}

func TestInsertPlugin_shapes(t *testing.T) {
	for _, tc := range []struct{ name, src string }{
		{"no engine block", "version: \"0.1\"\n\nscripts:\n  t: echo hi\n"},
		{"engine without plugins", "version: \"0.1\"\nengine:\n  dialect: matcher\nscripts:\n  t: echo hi\n"},
		{"engine with plugins", "version: \"0.1\"\nengine:\n  plugins:\n" +
			"    - source: ./a.wasm\n      sha256: aaa\n      provides: [runner:a]\n" +
			"scripts:\n  t: echo hi\n"},
		{"plugins with a config block", "version: \"0.1\"\nengine:\n  plugins:\n" +
			"    - source: ./a.wasm\n      sha256: aaa\n      provides: [runner:a]\n" +
			"      config:\n        proc: {exec: true}\n" +
			"scripts:\n  t: echo hi\n"},
		{"no scripts at all", "version: \"0.1\"\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := catalog.InsertPlugin([]byte(tc.src), entry)
			if err != nil {
				t.Fatalf("%v\n%s", err, tc.src)
			}
			cat, err := catalog.Parse(got, "x")
			if err != nil {
				t.Fatalf("does not parse: %v\n%s", err, got)
			}
			var found bool
			for _, p := range cat.Engine.Plugins {
				if p.SHA256 == "abc123" {
					found = true
				}
			}
			if !found {
				t.Fatalf("entry not inserted:\n%s", got)
			}
			// An existing entry must survive alongside the new one.
			if strings.Contains(tc.src, "a.wasm") && len(cat.Engine.Plugins) != 2 {
				t.Fatalf("existing entry lost:\n%s", got)
			}
		})
	}
}

// Inserting twice is how a second plugin arrives; both must be there.
func TestInsertPlugin_twice(t *testing.T) {
	src := []byte("version: \"0.1\"\nscripts:\n  t: echo hi\n")
	one, err := catalog.InsertPlugin(src, entry)
	if err != nil {
		t.Fatal(err)
	}
	second := entry
	second.Source = "./other.wasm"
	second.SHA256 = "def456"
	second.Provides = []string{"runner:other"}
	second.Config = ""
	two, err := catalog.InsertPlugin(one, second)
	if err != nil {
		t.Fatalf("%v\n%s", err, one)
	}
	cat, err := catalog.Parse(two, "x")
	if err != nil {
		t.Fatalf("does not parse: %v\n%s", err, two)
	}
	if len(cat.Engine.Plugins) != 2 {
		t.Fatalf("plugins=%d:\n%s", len(cat.Engine.Plugins), two)
	}
}

func TestInsertPlugin_rejectsIncomplete(t *testing.T) {
	src := []byte("version: \"0.1\"\nscripts:\n  t: echo hi\n")
	if _, err := catalog.InsertPlugin(src, catalog.PluginEntry{Source: "x"}); err == nil {
		t.Fatal("want an error without a digest")
	}
	if _, err := catalog.InsertPlugin([]byte("- not a mapping\n"), entry); err == nil {
		t.Fatal("want an error for a non-mapping root")
	}
}

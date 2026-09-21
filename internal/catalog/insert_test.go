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

// Installing a plugin the catalog already declares updates it, rather than
// leaving two entries claiming one runner.
func TestUpsertPlugin_replacesTheEntryForTheSameRunner(t *testing.T) {
	src := `version: "0.1"

engine:
  plugins:
    - source: https://example.test/micropy.wasm
      sha256: "replace me"
      provides: [runner:micropy]
      config:
        proc: {exec: true}
        fs: {mount: true}

scripts:
  # keep me
  t: echo hi
`
	got, err := catalog.UpsertPlugin([]byte(src), catalog.PluginEntry{
		Source:   "./local.wasm",
		SHA256:   "realdigest",
		Provides: []string{"runner:micropy"},
		Config:   "proc: {exec: true}",
	})
	if err != nil {
		t.Fatal(err)
	}
	cat, err := catalog.Parse(got, "x")
	if err != nil {
		t.Fatalf("does not parse: %v\n%s", err, got)
	}
	if len(cat.Engine.Plugins) != 1 {
		t.Fatalf("plugins=%d, want the entry replaced:\n%s", len(cat.Engine.Plugins), got)
	}
	p := cat.Engine.Plugins[0]
	if p.SHA256 != "realdigest" || p.Source != "./local.wasm" {
		t.Fatalf("entry=%+v", p)
	}
	if !strings.Contains(string(got), "# keep me") {
		t.Fatalf("lost a comment:\n%s", got)
	}
	if len(cat.Scripts) != 1 {
		t.Fatalf("scripts=%d", len(cat.Scripts))
	}
}

// A different runner is added beside, not over.
func TestUpsertPlugin_addsWhenNothingProvidesTheSame(t *testing.T) {
	src := "version: \"0.1\"\nengine:\n  plugins:\n" +
		"    - source: ./a.wasm\n      sha256: aaa\n      provides: [runner:a]\n" +
		"scripts:\n  t: echo hi\n"
	got, err := catalog.UpsertPlugin([]byte(src), catalog.PluginEntry{
		Source: "./b.wasm", SHA256: "bbb", Provides: []string{"runner:b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	cat, err := catalog.Parse(got, "x")
	if err != nil {
		t.Fatalf("does not parse: %v\n%s", err, got)
	}
	if len(cat.Engine.Plugins) != 2 {
		t.Fatalf("plugins=%d:\n%s", len(cat.Engine.Plugins), got)
	}
}

// What a plugin may do is the author's decision. install brings the digest in
// line; it does not re-grant what someone took away.
func TestUpsertPlugin_keepsTheExistingConfig(t *testing.T) {
	src := `version: "0.1"

engine:
  plugins:
    - source: https://example.test/micropy.wasm
      sha256: "replace me"
      provides: [runner:micropy]
      config:
        proc: {exec: true}
        fs: {mount: true, slink: true}
        time: {wall: true}

scripts:
  t: echo hi
`
	got, err := catalog.UpsertPlugin([]byte(src), catalog.PluginEntry{
		Source:   "./local.wasm",
		SHA256:   "realdigest",
		Provides: []string{"runner:micropy"},
		Config:   "proc: {exec: true}", // the default install would have written
	})
	if err != nil {
		t.Fatal(err)
	}
	cat, err := catalog.Parse(got, "x")
	if err != nil {
		t.Fatalf("does not parse: %v\n%s", err, got)
	}
	cfg := cat.Engine.Plugins[0].Config
	fs, ok := cfg["fs"].(map[string]any)
	if !ok {
		t.Fatalf("fs grants dropped: %+v\n%s", cfg, got)
	}
	if fs["mount"] != true || fs["slink"] != true {
		t.Fatalf("fs=%+v", fs)
	}
	if _, ok := cfg["time"]; !ok {
		t.Fatalf("time grant dropped: %+v", cfg)
	}
	if cat.Engine.Plugins[0].SHA256 != "realdigest" {
		t.Fatalf("digest not updated: %+v", cat.Engine.Plugins[0])
	}
}

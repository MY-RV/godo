package catalog_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/my-rv/godo/internal/catalog"
)

func TestInclude_wholeValueComesFromTheFile(t *testing.T) {
	dir := t.TempDir()
	body := "import os\nprint('hola')\n"
	if err := os.WriteFile(filepath.Join(dir, "script.py"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	path := writeCat(t, dir, "version: \"0.1\"\nscripts:\n  t: ${godo:file(./script.py)}\n")

	cat, err := catalog.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cat.Scripts[0].Commands[0] != body {
		t.Fatalf("body=%q", cat.Scripts[0].Commands[0])
	}
}

// A script says where its body lives; that does not change with where godo
// was run from.
func TestInclude_isRelativeToTheCatalog(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "s.py"), []byte("print(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeCat(t, root, "version: \"0.1\"\nscripts:\n  t: ${godo:file(s.py)}\n")

	// Resolved from a subdirectory: the catalog is found by walking up, and
	// the include still means the file beside it.
	found, err := catalog.FindFile(sub)
	if err != nil {
		t.Fatal(err)
	}
	cat, err := catalog.LoadFile(found)
	if err != nil {
		t.Fatal(err)
	}
	if cat.Scripts[0].Commands[0] != "print(1)" {
		t.Fatalf("body=%q", cat.Scripts[0].Commands[0])
	}
}

// Splicing a file into the middle of a line would paste newlines into a shell
// command; a body that is a file is a body that is a file.
func TestInclude_refusesPartOfALine(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "m.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := writeCat(t, dir, "version: \"0.1\"\nscripts:\n  t: echo ${godo:file(m.txt)}\n")
	_, err := catalog.LoadFile(path)
	if err == nil || !strings.Contains(err.Error(), "whole value") {
		t.Fatalf("err=%v", err)
	}
}

func TestInclude_reportsAMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := writeCat(t, dir, "version: \"0.1\"\nscripts:\n  t: ${godo:file(nope.py)}\n")
	_, err := catalog.LoadFile(path)
	if err == nil || !strings.Contains(err.Error(), "nope.py") {
		t.Fatalf("err=%v", err)
	}
}

func TestInclude_rejectsMalformed(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"no path", "  t: ${godo:file()}\n", "needs a path"},
		{"unterminated", "  t: \"${godo:file(x.py\"\n", "unterminated"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeCat(t, t.TempDir(), "version: \"0.1\"\nscripts:\n"+tc.body)
			_, err := catalog.LoadFile(path)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

// Inclusion is not expansion: it happens when the body is read, so it works
// for a runner whose bodies are never expanded.
func TestInclude_worksForAnyRunner(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "s.py"), []byte("print(godo.argv['X'])"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := writeCat(t, dir, "version: \"0.1\"\nscripts:\n  # @runner micropy\n  t: ${godo:file(s.py)}\n")
	cat, err := catalog.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// ${godo:argv[…]} inside the file is untouched: it is the plugin's text.
	if cat.Scripts[0].Commands[0] != "print(godo.argv['X'])" {
		t.Fatalf("body=%q", cat.Scripts[0].Commands[0])
	}
	if cat.Scripts[0].Runner != "micropy" {
		t.Fatalf("runner=%q", cat.Scripts[0].Runner)
	}
}

// A list of commands may mix included and inline entries.
func TestInclude_inAList(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "two.sh"), []byte("echo two"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := writeCat(t, dir, "version: \"0.1\"\nscripts:\n  t:\n    - echo one\n    - ${godo:file(two.sh)}\n")
	cat, err := catalog.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := cat.Scripts[0].Commands
	if len(got) != 2 || got[0] != "echo one" || got[1] != "echo two" {
		t.Fatalf("commands=%q", got)
	}
}

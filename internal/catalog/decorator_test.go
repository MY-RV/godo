package catalog_test

import (
	"strings"
	"testing"

	"github.com/my-rv/godo/internal/catalog"
)

// A decorator at the end of the line is the failure a Windows user hit on the
// 0.3.0 preview: YAML files it as a line comment, godo read nobody's comment,
// and the catalog ran with a dialect its author believed they had written.
func TestDecorator_trailingOnTheLineIsAnError(t *testing.T) {
	dir := t.TempDir()
	path := writeCat(t, dir, "version: \"0.1\"\nscripts:\n  ins: bun install  # @dialect matcher\n  b: echo b\n")

	_, err := catalog.LoadFile(path)
	if err == nil {
		t.Fatal("want an error, got a catalog that quietly ignored the decorator")
	}
	for _, want := range []string{"@dialect", "not read", "own line above the key"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not say %q", err, want)
		}
	}
}

// After the last key there is no next key to decorate, so YAML files it as
// that key's foot comment. Silently dropped before.
func TestDecorator_belowTheLastKeyIsAnError(t *testing.T) {
	dir := t.TempDir()
	path := writeCat(t, dir, "version: \"0.1\"\nscripts:\n  a: echo a\n  # @deps a\n")

	_, err := catalog.LoadFile(path)
	if err == nil || !strings.Contains(err.Error(), "@deps") {
		t.Fatalf("err=%v", err)
	}
}

// The check must not turn ordinary comments into errors.
func TestDecorator_proseMentioningOneIsStillAComment(t *testing.T) {
	dir := t.TempDir()
	path := writeCat(t, dir, "version: \"0.1\"\nscripts:\n  a: echo a  # like @deps but not\n  b: echo b\n")

	if _, err := catalog.LoadFile(path); err != nil {
		t.Fatalf("a comment that merely names a decorator is a comment: %v", err)
	}
}

// The placement that works has to keep working, blank line or not.
func TestDecorator_aboveTheKeyIsRead(t *testing.T) {
	for name, doc := range map[string]string{
		"no blank line": "version: \"0.1\"\nscripts:\n  a: echo a\n  # @dialect matcher\n  ${x}: echo ${godo:argv[x]}\n",
		"blank line":    "version: \"0.1\"\nscripts:\n  a: echo a\n\n  # @dialect matcher\n  ${x}: echo ${godo:argv[x]}\n",
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			cat, err := catalog.LoadFile(writeCat(t, dir, doc))
			if err != nil {
				t.Fatal(err)
			}
			if cat.Scripts[1].Dialect != "matcher" {
				t.Fatalf("dialect=%q, want matcher", cat.Scripts[1].Dialect)
			}
		})
	}
}

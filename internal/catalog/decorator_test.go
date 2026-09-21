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

// A catalog written on Windows has CRLF, and the YAML parser files comments
// differently for it: the decorator directly above its key became the previous
// key's foot comment and decorated nothing. A blank line above the comment
// happened to hide it, which is why the report was "no space between the
// command and the comment breaks it".
func TestDecorator_crlfCatalogReadsTheDecorator(t *testing.T) {
	doc := "version: \"0.1\"\nscripts:\n  a: echo a\n  # @dialect matcher\n  ${x}: echo ${godo:argv[x]}\n"
	dir := t.TempDir()
	path := writeCat(t, dir, strings.ReplaceAll(doc, "\n", "\r\n"))

	cat, err := catalog.LoadFile(path)
	if err != nil {
		t.Fatalf("a CRLF catalog is a catalog: %v", err)
	}
	if cat.Scripts[1].Dialect != "matcher" {
		t.Fatalf("dialect=%q, want matcher", cat.Scripts[1].Dialect)
	}
}

// The same normalization keeps \r out of a block scalar, where it would reach
// the shell as part of the command.
func TestDecorator_crlfDoesNotLeakIntoABody(t *testing.T) {
	doc := "version: \"0.1\"\nscripts:\n  a: |\n    echo one\n    echo two\n"
	dir := t.TempDir()
	path := writeCat(t, dir, strings.ReplaceAll(doc, "\n", "\r\n"))

	cat, err := catalog.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cat.Scripts[0].Commands[0], "\r") {
		t.Fatalf("body carries a carriage return: %q", cat.Scripts[0].Commands[0])
	}
}

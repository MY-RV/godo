package catalog_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/my-rv/godo/internal/catalog"
)

func writeCat(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "godo.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

type runnerFunc func(string) error

func (f runnerFunc) Run(c string) error { return f(c) }

func TestFindFile_walkUp(t *testing.T) {
	root := t.TempDir()
	writeCat(t, root, "version: \"0.1\"\ndialect: package\nscripts:\n  t: echo ok\n")
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := catalog.FindFile(sub)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(root, "godo.yaml") {
		t.Fatalf("got %s", got)
	}
}

func TestParse_rejectsReservedDialect(t *testing.T) {
	_, err := catalog.Parse([]byte("version: \"0.1\"\ndialect: nscript\nscripts: {}\n"), "x")
	if err == nil || !errors.Is(err, catalog.ErrInvalidCatalog) {
		t.Fatalf("err=%v", err)
	}
}

func TestParse_defaultsDialectToPackage(t *testing.T) {
	cat, err := catalog.Parse([]byte("version: \"0.1\"\nscripts:\n  t: echo ok\n"), "x")
	if err != nil {
		t.Fatal(err)
	}
	if cat.Dialect != catalog.DialectPackage {
		t.Fatalf("dialect=%q", cat.Dialect)
	}
	if len(cat.Scripts) != 1 || cat.Scripts[0].Key != "t" {
		t.Fatalf("%+v", cat.Scripts)
	}
}

func TestParse_rejectsPackageCaptureKey(t *testing.T) {
	_, err := catalog.Parse([]byte(`version: "0.1"
dialect: package
scripts:
  "test ${MODULE}": echo x
`), "x")
	if err == nil || !errors.Is(err, catalog.ErrInvalidCatalog) {
		t.Fatalf("err=%v", err)
	}
}

func TestParse_rejectsInvalidCaptureName(t *testing.T) {
	_, err := catalog.Parse([]byte(`version: "0.1"
dialect: matcher
scripts:
  "${foo-bar}": echo x
`), "x")
	if err == nil || !errors.Is(err, catalog.ErrInvalidCatalog) {
		t.Fatalf("err=%v", err)
	}
}

func TestParse_rejectsUnknownDecoratorAndEmptyDialect(t *testing.T) {
	_, err := catalog.Parse([]byte(`version: "0.1"
dialect: package
scripts:
  # @foo bar
  t: echo x
`), "x")
	if err == nil {
		t.Fatal("expected unknown decorator")
	}
	_, err = catalog.Parse([]byte(`version: "0.1"
dialect: package
scripts:
  # @dialect
  t: echo x
`), "x")
	if err == nil {
		t.Fatal("expected empty @dialect error")
	}
}

func TestParse_rejectsDuplicateKeysAndEmptyList(t *testing.T) {
	_, err := catalog.Parse([]byte(`version: "0.1"
dialect: package
scripts:
  t: echo a
  t: echo b
`), "x")
	if err == nil {
		t.Fatal("duplicate")
	}
	_, err = catalog.Parse([]byte(`version: "0.1"
dialect: package
scripts:
  t: []
`), "x")
	if err == nil {
		t.Fatal("empty list")
	}
}

func TestParse_decoratorsAliasAndOrder(t *testing.T) {
	cat, err := catalog.Parse([]byte(`version: "0.1"
dialect: package
scripts:
  # Unit
  test: go test ./...
  # @dependencies lint, test
  ci: go build ./...
  # @dialect matcher
  # @deps lint ${MODULE}
  test ${MODULE}: go test ./${MODULE}/...
`), "godo.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cat.Scripts[1].Deps[0] != "lint" || cat.Scripts[2].Dialect != "matcher" {
		t.Fatalf("%#v", cat.Scripts)
	}
}

func TestEngine_packageArgsPreviewUnexpected(t *testing.T) {
	dir := t.TempDir()
	writeCat(t, dir, `version: "0.1"
dialect: package
scripts:
  seed: echo ${godo:args}
  test: echo ok
  multi:
    - echo one
    - echo two
`)
	cat, _ := catalog.LoadFile(filepath.Join(dir, "godo.yaml"))
	var ran []string
	eng := catalog.NewEngine(cat, runnerFunc(func(c string) error {
		ran = append(ran, c)
		return nil
	}))
	lines, err := eng.PreviewLines([]string{"seed", "a", "b"})
	if err != nil || lines[0] != "echo a b" {
		t.Fatalf("%v %v", lines, err)
	}
	err = eng.Run([]string{"test", "extra"})
	if err == nil || !errors.Is(err, catalog.ErrUnexpectedArgs) {
		t.Fatalf("err=%v", err)
	}
	ran = nil
	if err := eng.Run([]string{"multi"}); err != nil {
		t.Fatal(err)
	}
	if len(ran) != 2 || ran[0] != "echo one" || ran[1] != "echo two" {
		t.Fatalf("%v", ran)
	}
}

func TestEngine_stopOnFirstFailure(t *testing.T) {
	dir := t.TempDir()
	writeCat(t, dir, `version: "0.1"
dialect: package
scripts:
  multi:
    - echo one
    - echo two
`)
	cat, _ := catalog.LoadFile(filepath.Join(dir, "godo.yaml"))
	var ran []string
	eng := catalog.NewEngine(cat, runnerFunc(func(c string) error {
		ran = append(ran, c)
		if c == "echo one" {
			return &catalog.ExitError{Code: 7, Message: "fail"}
		}
		return nil
	}))
	err := eng.Run([]string{"multi"})
	if catalog.ExitCode(err) != 7 || len(ran) != 1 {
		t.Fatalf("err=%v ran=%v", err, ran)
	}
	// wrapped ExitError
	err = fmt.Errorf("wrap: %w", &catalog.ExitError{Code: 3})
	if catalog.ExitCode(err) != 3 {
		t.Fatalf("ExitCode=%d", catalog.ExitCode(err))
	}
}

func TestEngine_depsCycleAndMatcher(t *testing.T) {
	dir := t.TempDir()
	writeCat(t, dir, `version: "0.1"
dialect: package
scripts:
  lint: echo lint
  # @deps lint
  test: echo test
`)
	cat, _ := catalog.LoadFile(filepath.Join(dir, "godo.yaml"))
	var ran []string
	eng := catalog.NewEngine(cat, runnerFunc(func(c string) error {
		ran = append(ran, c)
		return nil
	}))
	if err := eng.Run([]string{"test"}); err != nil {
		t.Fatal(err)
	}
	if len(ran) != 2 || ran[0] != "echo lint" {
		t.Fatalf("%v", ran)
	}

	writeCat(t, dir, `version: "0.1"
dialect: package
scripts:
  # @deps b
  a: echo a
  # @deps a
  b: echo b
`)
	cat, _ = catalog.LoadFile(filepath.Join(dir, "godo.yaml"))
	eng = catalog.NewEngine(cat, runnerFunc(func(string) error { return nil }))
	err := eng.Run([]string{"a"})
	if err == nil || !errors.Is(err, catalog.ErrDependencyCycle) {
		t.Fatalf("err=%v", err)
	}

	writeCat(t, dir, `version: "0.1"
dialect: matcher
scripts:
  # @deps lint ${MODULE}
  test ${MODULE}: echo test-${MODULE}
  lint ${MODULE}: echo lint-${MODULE}
`)
	cat, _ = catalog.LoadFile(filepath.Join(dir, "godo.yaml"))
	ran = nil
	eng = catalog.NewEngine(cat, runnerFunc(func(c string) error {
		ran = append(ran, c)
		return nil
	}))
	if err := eng.Run([]string{"test", "payments"}); err != nil {
		t.Fatal(err)
	}
	if ran[0] != "echo lint-payments" || ran[1] != "echo test-payments" {
		t.Fatalf("%v", ran)
	}
	err = eng.Run([]string{"test", "payments", "extra"})
	if err == nil || !errors.Is(err, catalog.ErrUnexpectedArgs) {
		t.Fatalf("err=%v", err)
	}
}

func TestEngine_dialectOverride(t *testing.T) {
	dir := t.TempDir()
	writeCat(t, dir, `version: "0.1"
dialect: package
scripts:
  test: echo unit
  # @dialect matcher
  "${GRP} ${SCR}": echo ${GRP}/${SCR} ${godo:args}
`)
	cat, err := catalog.LoadFile(filepath.Join(dir, "godo.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	lines, err := catalog.NewEngine(cat, nil).PreviewLines([]string{"svc", "seed", "x"})
	if err != nil || lines[0] != "echo svc/seed x" {
		t.Fatalf("%v %v", lines, err)
	}
}

func TestEngine_noMatchAndNoTokens(t *testing.T) {
	dir := t.TempDir()
	writeCat(t, dir, `version: "0.1"
dialect: package
scripts:
  test: echo ok
`)
	cat, _ := catalog.LoadFile(filepath.Join(dir, "godo.yaml"))
	eng := catalog.NewEngine(cat, nil)
	err := eng.Run([]string{"nope"})
	if err == nil || !errors.Is(err, catalog.ErrNoMatch) {
		t.Fatalf("%v", err)
	}
	_, err = eng.PreviewLines(nil)
	if err == nil || !errors.Is(err, catalog.ErrNoTokens) {
		t.Fatalf("%v", err)
	}
}

func TestParse_rejectsAtDialectReserved(t *testing.T) {
	_, err := catalog.Parse([]byte(`version: "0.1"
dialect: package
scripts:
  # @dialect nscript
  t: echo x
`), "x")
	if err == nil || !strings.Contains(err.Error(), "nscript") {
		t.Fatalf("%v", err)
	}
}

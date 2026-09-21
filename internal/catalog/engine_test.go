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
  test ${MODULE}: echo test-${godo:argv[MODULE]}
  lint ${MODULE}: echo lint-${godo:argv[MODULE]}
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
  "${GRP} ${SCR}": echo ${godo:argv[GRP]}/${godo:argv[SCR]} ${godo:args}
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

func TestEngine_depsDiamondRunsOnce(t *testing.T) {
	root := t.TempDir()
	writeCat(t, root, `version: "0.1"
scripts:
  build: echo build

  # @deps build
  test: echo test

  # @deps build
  lint: echo lint

  # @deps test, lint
  ci: echo ci
`)
	cat, err := catalog.LoadFile(filepath.Join(root, "godo.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	lines, err := catalog.NewEngine(cat, nil).PreviewLines([]string{"ci"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"echo build", "echo test", "echo lint", "echo ci"}
	if strings.Join(lines, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v want %v", lines, want)
	}
}

func TestEngine_sameDepDifferentCapturesBothRun(t *testing.T) {
	// Dedup keys on the expanded invocation, not the script, so lint pay and
	// lint auth are distinct nodes.
	root := t.TempDir()
	writeCat(t, root, `version: "0.1"
dialect: matcher
scripts:
  lint ${MODULE}: echo lint ${godo:argv[MODULE]}

  # @deps lint pay, lint auth, lint pay
  all: echo all
`)
	cat, err := catalog.LoadFile(filepath.Join(root, "godo.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	lines, err := catalog.NewEngine(cat, nil).PreviewLines([]string{"all"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"echo lint pay", "echo lint auth", "echo all"}
	if strings.Join(lines, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v want %v", lines, want)
	}
}

func TestEngine_argsAreQuotedIntoTheShellLine(t *testing.T) {
	root := t.TempDir()
	writeCat(t, root, "version: \"0.1\"\nscripts:\n  greet: echo hello ${godo:args}\n")
	cat, err := catalog.LoadFile(filepath.Join(root, "godo.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var ran []string
	eng := catalog.NewEngine(cat, runnerFunc(func(c string) error {
		ran = append(ran, c)
		return nil
	}))
	if err := eng.Run([]string{"greet", "a; touch PWNED"}); err != nil {
		t.Fatal(err)
	}
	if len(ran) != 1 || ran[0] != `echo hello 'a; touch PWNED'` {
		t.Fatalf("%q", ran)
	}
}

// --- runner axis ------------------------------------------------------------

// An unset runner stays unset and means "the Runner this engine was given",
// so an embedder's injected runner keeps working without naming anything.
func TestParse_unsetRunnerMeansTheInjectedOne(t *testing.T) {
	cat, err := catalog.Parse([]byte("version: \"0.1\"\nscripts:\n  t: echo ok\n"), "x")
	if err != nil {
		t.Fatal(err)
	}
	if cat.Runner != "" {
		t.Fatalf("file runner=%q, want empty", cat.Runner)
	}
	if got := catalog.EffectiveRunner(cat.Scripts[0], cat.Runner); got != "" {
		t.Fatalf("effective=%q, want empty", got)
	}
	var ran []string
	eng := catalog.NewEngine(cat, runnerFunc(func(c string) error { ran = append(ran, c); return nil }))
	if err := eng.Run([]string{"t"}); err != nil {
		t.Fatal(err)
	}
	if len(ran) != 1 || ran[0] != "echo ok" {
		t.Fatalf("ran=%q", ran)
	}
}

func TestParse_fileRunnerAndDecoratorOverride(t *testing.T) {
	src := "version: \"0.1\"\nengine:\n  runner: micropy\nscripts:\n" +
		"  inherits: echo a\n" +
		"  # @runner bash\n" +
		"  overrides: echo b\n"
	cat, err := catalog.Parse([]byte(src), "x")
	if err != nil {
		t.Fatal(err)
	}
	if cat.Runner != "micropy" {
		t.Fatalf("file runner=%q", cat.Runner)
	}
	if got := catalog.EffectiveRunner(cat.Scripts[0], cat.Runner); got != "micropy" {
		t.Fatalf("inherits=%q", got)
	}
	if got := catalog.EffectiveRunner(cat.Scripts[1], cat.Runner); got != "bash" {
		t.Fatalf("overrides=%q", got)
	}
}

func TestParse_runnerDecoratorRequiresName(t *testing.T) {
	src := "version: \"0.1\"\nscripts:\n  # @runner\n  t: echo ok\n"
	_, err := catalog.Parse([]byte(src), "x")
	if err == nil || !errors.Is(err, catalog.ErrInvalidCatalog) {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(err.Error(), "@runner requires a name") {
		t.Fatalf("err=%v", err)
	}
}

// An older binary has no @runner, and its unknown-decorator rejection is what
// stops a body meant for another runner from reaching the host shell.
func TestParse_unknownDecoratorStillRejected(t *testing.T) {
	src := "version: \"0.1\"\nscripts:\n  # @executor exec\n  t: echo ok\n"
	_, err := catalog.Parse([]byte(src), "x")
	if err == nil || !strings.Contains(err.Error(), "unknown decorator @executor") {
		t.Fatalf("err=%v", err)
	}
}

func TestBuildPlan_unknownRunnerFailsClosed(t *testing.T) {
	src := "version: \"0.1\"\nscripts:\n  # @runner nope\n  t: echo ok\n"
	cat, err := catalog.Parse([]byte(src), "x")
	if err != nil {
		t.Fatal(err)
	}
	eng := catalog.NewEngine(cat, runnerFunc(func(string) error { return nil }))
	if _, err := eng.BuildPlan([]string{"t"}); !errors.Is(err, catalog.ErrUnknownRunner) {
		t.Fatalf("err=%v", err)
	}
	// Nothing must execute, so --preview has to fail the same way.
	if _, err := eng.PreviewLines([]string{"t"}); !errors.Is(err, catalog.ErrUnknownRunner) {
		t.Fatalf("preview err=%v", err)
	}
}

func TestRun_dispatchesPerStepRunner(t *testing.T) {
	src := "version: \"0.1\"\nscripts:\n" +
		"  # @runner other\n" +
		"  dep: echo dep\n" +
		"  # @deps dep\n" +
		"  body: echo body\n"
	cat, err := catalog.Parse([]byte(src), "x")
	if err != nil {
		t.Fatal(err)
	}
	var shell, other []string
	reg := catalog.NewRunnerRegistry()
	if err := reg.Register("other", runnerFunc(func(c string) error {
		other = append(other, c)
		return nil
	})); err != nil {
		t.Fatal(err)
	}
	eng := catalog.NewEngine(cat, runnerFunc(func(c string) error {
		shell = append(shell, c)
		return nil
	}), catalog.WithRunners(reg))

	if err := eng.Run([]string{"body"}); err != nil {
		t.Fatal(err)
	}
	if len(other) != 1 || other[0] != "echo dep" {
		t.Fatalf("other=%v", other)
	}
	if len(shell) != 1 || shell[0] != "echo body" {
		t.Fatalf("shell=%v", shell)
	}
}

func TestBuildPlan_stepsRecordTheirRunner(t *testing.T) {
	src := "version: \"0.1\"\nscripts:\n" +
		"  # @runner other\n" +
		"  dep: echo dep\n" +
		"  # @deps dep\n" +
		"  body: echo body\n"
	cat, err := catalog.Parse([]byte(src), "x")
	if err != nil {
		t.Fatal(err)
	}
	reg := catalog.NewRunnerRegistry()
	_ = reg.Register("other", runnerFunc(func(string) error { return nil }))
	eng := catalog.NewEngine(cat, nil, catalog.WithRunners(reg))

	plan, err := eng.BuildPlan([]string{"body"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("steps=%+v", plan.Steps)
	}
	// The dep named one; the body named none, so it is the injected default.
	if plan.Steps[0].Runner != "other" || plan.Steps[1].Runner != "" {
		t.Fatalf("runners=%q %q", plan.Steps[0].Runner, plan.Steps[1].Runner)
	}
}

// A named runner comes from the registry, never from the injected default.
func TestRun_namedRunnerBeatsTheInjectedDefault(t *testing.T) {
	cat, err := catalog.Parse([]byte("version: \"0.1\"\nengine:\n  runner: inherit\nscripts:\n  t: echo ok\n"), "x")
	if err != nil {
		t.Fatal(err)
	}
	var injected, registered int
	reg := catalog.NewRunnerRegistry()
	_ = reg.Register(catalog.RunnerInherit, runnerFunc(func(string) error { registered++; return nil }))
	eng := catalog.NewEngine(cat, runnerFunc(func(string) error { injected++; return nil }), catalog.WithRunners(reg))
	if err := eng.Run([]string{"t"}); err != nil {
		t.Fatal(err)
	}
	if registered != 1 || injected != 0 {
		t.Fatalf("registered=%d injected=%d", registered, injected)
	}
}

func TestRun_nilShellRunnerStillErrors(t *testing.T) {
	cat, err := catalog.Parse([]byte("version: \"0.1\"\nscripts:\n  t: echo ok\n"), "x")
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.NewEngine(cat, nil).Run([]string{"t"}); err == nil ||
		!strings.Contains(err.Error(), "nil runner") {
		t.Fatalf("err=%v", err)
	}
}

func TestRunnerRegistry_rejectsEmptyNameAndNilRunner(t *testing.T) {
	reg := catalog.NewRunnerRegistry()
	if err := reg.Register("", runnerFunc(func(string) error { return nil })); err == nil {
		t.Fatal("want error for empty name")
	}
	if err := reg.Register("x", nil); err == nil {
		t.Fatal("want error for nil runner")
	}
	if _, err := reg.Lookup("x"); !errors.Is(err, catalog.ErrUnknownRunner) {
		t.Fatalf("err=%v", err)
	}
}

// --- args are the runner's question -----------------------------------------

// pluginRunner stands in for a plugin whose body is a program rather than a
// shell template: no ${godo:args} to find, and it takes args regardless.
type pluginRunner struct{ ran []string }

func (r *pluginRunner) Run(c string) error        { r.ran = append(r.ran, c); return nil }
func (r *pluginRunner) AcceptsArgs([]string) bool { return true }

type strictRunner struct{}

func (strictRunner) Run(string) error          { return nil }
func (strictRunner) AcceptsArgs([]string) bool { return false }

// Default policy, unchanged: no ${godo:args} in the body → extra tokens error.
func TestBuildPlan_defaultPolicyStillRejectsUnexpectedArgs(t *testing.T) {
	cat, err := catalog.Parse([]byte("version: \"0.1\"\nscripts:\n  t: echo hi\n"), "x")
	if err != nil {
		t.Fatal(err)
	}
	eng := catalog.NewEngine(cat, runnerFunc(func(string) error { return nil }))
	_, err = eng.BuildPlan([]string{"t", "extra"})
	if !errors.Is(err, catalog.ErrUnexpectedArgs) {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(err.Error(), "${godo:args}") {
		t.Fatalf("err=%v", err)
	}
}

// A plugin body has no placeholder, and still has to be able to take args.
func TestBuildPlan_argsAwareRunnerOverridesThePlaceholderPolicy(t *testing.T) {
	src := "version: \"0.1\"\nscripts:\n  # @runner micropy\n  t: print(godo.args)\n"
	cat, err := catalog.Parse([]byte(src), "x")
	if err != nil {
		t.Fatal(err)
	}
	reg := catalog.NewRunnerRegistry()
	pr := &pluginRunner{}
	if err := reg.Register("micropy", pr); err != nil {
		t.Fatal(err)
	}
	eng := catalog.NewEngine(cat, nil, catalog.WithRunners(reg))
	if err := eng.Run([]string{"t", "--flag", "a b"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(pr.ran) != 1 || pr.ran[0] != "print(godo.args)" {
		t.Fatalf("ran=%q", pr.ran)
	}
}

func TestBuildPlan_argsAwareRunnerCanRefuse(t *testing.T) {
	src := "version: \"0.1\"\nscripts:\n  # @runner strict\n  t: echo ${godo:args}\n"
	cat, err := catalog.Parse([]byte(src), "x")
	if err != nil {
		t.Fatal(err)
	}
	reg := catalog.NewRunnerRegistry()
	_ = reg.Register("strict", strictRunner{})
	eng := catalog.NewEngine(cat, nil, catalog.WithRunners(reg))
	_, err = eng.BuildPlan([]string{"t", "extra"})
	if !errors.Is(err, catalog.ErrUnexpectedArgs) {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(err.Error(), `runner "strict"`) {
		t.Fatalf("err=%v", err)
	}
}

// Deps are matched on their own, so the check uses the dep's runner.
func TestBuildPlan_depArgsUseTheDepRunner(t *testing.T) {
	src := "version: \"0.1\"\ndialect: matcher\nscripts:\n" +
		"  # @runner micropy\n" +
		"  seed ${ENV}: print(${godo:argv[ENV]})\n" +
		"  # @deps seed dev\n" +
		"  boot: echo up\n"
	cat, err := catalog.Parse([]byte(src), "x")
	if err != nil {
		t.Fatal(err)
	}
	reg := catalog.NewRunnerRegistry()
	_ = reg.Register("micropy", &pluginRunner{})
	eng := catalog.NewEngine(cat, runnerFunc(func(string) error { return nil }), catalog.WithRunners(reg))
	plan, err := eng.BuildPlan([]string{"boot"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 2 || plan.Steps[0].Runner != "micropy" {
		t.Fatalf("steps=%+v", plan.Steps)
	}
}

// An unknown runner is still caught before the args question is asked.
func TestBuildPlan_unknownRunnerBeatsUnexpectedArgs(t *testing.T) {
	src := "version: \"0.1\"\nscripts:\n  # @runner nope\n  t: echo hi\n"
	cat, err := catalog.Parse([]byte(src), "x")
	if err != nil {
		t.Fatal(err)
	}
	eng := catalog.NewEngine(cat, runnerFunc(func(string) error { return nil }))
	_, err = eng.BuildPlan([]string{"t", "extra"})
	if !errors.Is(err, catalog.ErrUnknownRunner) {
		t.Fatalf("err=%v", err)
	}
}

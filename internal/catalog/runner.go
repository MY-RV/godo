package catalog

import "fmt"

// RunnerName identifies an execution strategy.
//
// Dialect answers "which script responds to these tokens"; runner answers
// "how does the resolved body become a process". The two are independent: a
// script may be matched by any dialect and run by any runner.
type RunnerName string

const (
	// RunnerInherit runs the line through the shell the caller is already in.
	//
	// The CLI injects it as the default: the shell someone is using is their
	// own business, and godo proxying to a different one was a choice that is
	// not godo's to make.
	//
	// It is deliberately absent from DefaultRunners: internal/execshell imports
	// this package, so it cannot be a default registry entry without an import
	// cycle. An unset runner resolves to the Runner injected into the Engine,
	// which is also what keeps NewEngine(cat, runner) working unchanged.
	RunnerInherit RunnerName = "inherit"
)

// RunnerRegistry maps runner names to implementations (Open/Closed).
//
// Unlike Dialect, a Runner carries per-invocation state (working directory,
// stdio), so the name is supplied at registration rather than being a method
// on the type: one implementation can be registered twice with different
// configuration.
type RunnerRegistry struct {
	byName map[RunnerName]Runner

	// Resolve answers for names that were not registered ahead of time.
	//
	// It is how "# @runner bash" works without godo keeping a list of every
	// shell anyone might have: the name is looked up when a catalog asks for
	// it. godo does not manage those shells — it proxies to them.
	Resolve func(RunnerName) (Runner, error)
}

// NewRunnerRegistry returns an empty registry.
func NewRunnerRegistry() *RunnerRegistry {
	return &RunnerRegistry{byName: make(map[RunnerName]Runner)}
}

// DefaultRunners returns the registry a stock engine starts with.
//
// It is empty: an unset runner is the injected Runner. The function exists so
// engine wiring reads the same for both axes.
func DefaultRunners() *RunnerRegistry { return NewRunnerRegistry() }

// Register adds a runner under name.
func (r *RunnerRegistry) Register(name RunnerName, run Runner) error {
	if r.byName == nil {
		r.byName = make(map[RunnerName]Runner)
	}
	if name == "" {
		return fmt.Errorf("empty runner name")
	}
	if run == nil {
		return fmt.Errorf("nil runner for %q", name)
	}
	r.byName[name] = run
	return nil
}

// Lookup returns a runner by name.
func (r *RunnerRegistry) Lookup(name RunnerName) (Runner, error) {
	if r == nil {
		return nil, fmt.Errorf("%w: %q", ErrUnknownRunner, name)
	}
	if r.byName == nil {
		r.byName = make(map[RunnerName]Runner)
	}
	if run, ok := r.byName[name]; ok {
		return run, nil
	}
	if r.Resolve == nil {
		return nil, fmt.Errorf("%w: %q", ErrUnknownRunner, name)
	}
	run, err := r.Resolve(name)
	if err != nil {
		return nil, err
	}
	// Memoised: Lookup is called more than once per step (validate, then run).
	r.byName[name] = run
	return run, nil
}

// EffectiveRunner resolves the @runner override or the file default.
//
// Empty means "the Runner this engine was given". godo the CLI injects the
// caller's own shell there, so a catalog that names nothing runs under the
// shell you are in; an embedder injects whatever it wants and gets that.
func EffectiveRunner(script Script, fileDefault RunnerName) RunnerName {
	if script.Runner != "" {
		return RunnerName(script.Runner)
	}
	return fileDefault
}

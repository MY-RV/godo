package catalog

import (
	"errors"
	"fmt"
	"strings"

	"github.com/my-rv/godo/internal/expand"
)

// Runner executes an expanded command line (Dependency Inversion).
type Runner interface {
	Run(command string) error
}

// ArgsAwareRunner is a Runner that decides for itself whether a body takes the
// tokens left over after the match.
//
// The default policy reads the body for ${godo:args…} and rejects leftover
// tokens when it finds none. That is a fact about shell templates, not about
// godo: a plugin's body is a program, has no such placeholder, and would have
// every extra token rejected. "Does this body accept leftover args?" is the
// runner's question, and a runner that implements this answers it.
type ArgsAwareRunner interface {
	Runner
	AcceptsArgs(commands []string) bool
}

// Engine resolves matches, deps, expansion, and run/preview.
type Engine struct {
	Catalog  *Catalog
	Dialects *DialectRegistry
	Runners  *RunnerRegistry
	// Runner is the default: what a script that names no runner resolves to.
	// Named runners live in Runners.
	Runner Runner
}

// EngineOption configures NewEngine.
type EngineOption func(*Engine)

// WithRunners overrides the named-runner registry.
func WithRunners(r *RunnerRegistry) EngineOption {
	return func(e *Engine) {
		if r != nil {
			e.Runners = r
		}
	}
}

// WithDialects overrides the dialect registry.
func WithDialects(r *DialectRegistry) EngineOption {
	return func(e *Engine) {
		if r != nil {
			e.Dialects = r
		}
	}
}

// NewEngine wires defaults.
func NewEngine(cat *Catalog, runner Runner, opts ...EngineOption) *Engine {
	e := &Engine{
		Catalog:  cat,
		Dialects: DefaultDialects(),
		Runners:  DefaultRunners(),
		Runner:   runner,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Resolve picks the first script in definition order whose effective dialect matches tokens.
func (e *Engine) Resolve(tokens []string) (*Match, error) {
	if e.Catalog == nil {
		return nil, fmt.Errorf("%w: nil catalog", ErrInvalidCatalog)
	}
	if len(tokens) == 0 {
		return nil, ErrNoTokens
	}
	reg := e.Dialects
	if reg == nil {
		reg = DefaultDialects()
	}

	for _, s := range e.Catalog.Scripts {
		eff := EffectiveDialect(s, e.Catalog.Dialect)
		d, err := reg.Lookup(eff)
		if err != nil {
			// Skip scripts whose dialect is not registered (should be rare after load validation).
			continue
		}
		m, err := d.Match(s, tokens)
		if err != nil {
			if errors.Is(err, ErrNoMatch) || errors.Is(err, ErrNoTokens) {
				continue
			}
			return nil, err
		}
		m.Dialect = eff
		return m, nil
	}
	return nil, fmt.Errorf("%w: %v", ErrNoMatch, tokens)
}

// runnerFor resolves the Runner that executes one step.
//
// The registry wins. Otherwise the unset name means the Runner injected into
// the Engine.
func (e *Engine) runnerFor(name RunnerName) (Runner, error) {
	if r, err := e.Runners.Lookup(name); err == nil {
		return r, nil
	}
	if name == "" {
		if e.Runner == nil {
			return nil, fmt.Errorf("nil runner")
		}
		return e.Runner, nil
	}
	return nil, fmt.Errorf("%w: %q", ErrUnknownRunner, name)
}

// stepRunner returns the effective runner name for a script, rejecting names
// nothing answers to.
//
// This is a name check, not an instance check: BuildPlan must stay usable with
// a nil Runner, which is how PreviewLines expands without anything to execute.
func (e *Engine) stepRunner(s Script) (RunnerName, error) {
	name := EffectiveRunner(s, e.Catalog.Runner)
	if name == "" {
		return name, nil
	}
	if _, err := e.Runners.Lookup(name); err != nil {
		// A runner the catalog declared a plugin for is a different failure
		// from a typo, and saying so saves the reader the hunt.
		if p, ok := e.Catalog.Engine.ProviderOf("runner", string(name)); ok {
			return "", fmt.Errorf("%w: script %q asks for runner %q, provided by plugin %s — this build cannot load plugins",
				ErrUnknownRunner, s.Key, name, p.Source)
		}
		// Otherwise the registry's resolver knows why (unknown name, shell not
		// installed); its message is the one worth reading.
		return "", fmt.Errorf("script %q asks for runner %q: %w", s.Key, name, err)
	}
	return name, nil
}

// Plan is the expanded execution plan (deps + body).
type Plan struct {
	Match *Match
	Steps []PlanStep
}

// PlanStep is one command line to run or preview.
type PlanStep struct {
	Kind    string // "dep" | "body"
	Source  string
	Command string
	Runner  RunnerName // effective runner for this step
}

// BuildPlan expands deps + body without executing.
//
// Deps form a DAG: an invocation that several scripts depend on is emitted once,
// at its first (deepest-first) position, so a diamond does not duplicate work.
// Invocations are keyed by their expanded token line, so the same script reached
// with different captures is a different node.
func (e *Engine) BuildPlan(tokens []string) (*Plan, error) {
	m, err := e.Resolve(tokens)
	if err != nil {
		return nil, err
	}
	runner, err := e.stepRunner(m.Script)
	if err != nil {
		return nil, err
	}
	if err := e.validateArgs(m, runner); err != nil {
		return nil, err
	}
	plan := &Plan{Match: m}
	stack := map[string]bool{}
	done := map[string]bool{}
	if err := e.appendDeps(plan, m, stack, done); err != nil {
		return nil, err
	}
	cmds, err := expand.ExpandAll(m.Script.Commands, m.Captures, m.Args)
	if err != nil {
		return nil, err
	}
	for _, c := range cmds {
		plan.Steps = append(plan.Steps, PlanStep{Kind: "body", Source: m.Script.Key, Command: c, Runner: runner})
	}
	return plan, nil
}

// validateArgs rejects leftover tokens a script cannot take.
//
// Which tokens a script can take depends on the runner, so the check asks the
// runner when it has an opinion and falls back to the placeholder policy
// otherwise.
func (e *Engine) validateArgs(m *Match, runner RunnerName) error {
	if len(m.Args) == 0 {
		return nil
	}
	if ar, ok := e.argsAwareRunner(runner); ok {
		if ar.AcceptsArgs(m.Script.Commands) {
			return nil
		}
		return fmt.Errorf("%w: %v (script %q takes no args under runner %q)", ErrUnexpectedArgs, m.Args, m.Script.Key, runner)
	}
	if expand.CommandsNeedArgs(m.Script.Commands) {
		return nil
	}
	return fmt.Errorf("%w: %v (script %q has no ${godo:args})", ErrUnexpectedArgs, m.Args, m.Script.Key)
}

// argsAwareRunner reports whether the runner for this step answers the args
// question itself. Registry first, then the injected default, like runnerFor.
func (e *Engine) argsAwareRunner(name RunnerName) (ArgsAwareRunner, bool) {
	if r, err := e.Runners.Lookup(name); err == nil {
		ar, ok := r.(ArgsAwareRunner)
		return ar, ok
	}
	if name == "" {
		ar, ok := e.Runner.(ArgsAwareRunner)
		return ar, ok
	}
	return nil, false
}

// appendDeps walks m's dependencies depth-first.
//
// stack holds the current path and catches cycles; done spans the whole plan and
// collapses repeats. A node leaves stack once emitted but stays in done, so a
// re-visit is a skip while a re-entry is still a cycle.
func (e *Engine) appendDeps(plan *Plan, m *Match, stack, done map[string]bool) error {
	for _, dep := range m.Script.Deps {
		// A @deps entry is godo space: bare ${NAME} is a capture, and it resolves
		// to tokens rather than to a line that would have to be re-split.
		depTokens, err := expand.ExpandInvocation(dep, m.Captures)
		if err != nil {
			return fmt.Errorf("deps %q: %w", dep, err)
		}
		if len(depTokens) == 0 {
			continue
		}
		invKey := strings.Join(depTokens, " ")
		if stack[invKey] {
			return fmt.Errorf("%w: involving %q", ErrDependencyCycle, invKey)
		}
		if done[invKey] {
			continue
		}
		stack[invKey] = true

		depMatch, err := e.Resolve(depTokens)
		if err != nil {
			return fmt.Errorf("deps %q: %w", invKey, err)
		}
		depRunner, err := e.stepRunner(depMatch.Script)
		if err != nil {
			return err
		}
		if err := e.validateArgs(depMatch, depRunner); err != nil {
			return err
		}
		if err := e.appendDeps(plan, depMatch, stack, done); err != nil {
			return err
		}
		cmds, err := expand.ExpandAll(depMatch.Script.Commands, depMatch.Captures, depMatch.Args)
		if err != nil {
			return err
		}
		for _, c := range cmds {
			plan.Steps = append(plan.Steps, PlanStep{Kind: "dep", Source: invKey, Command: c, Runner: depRunner})
		}
		delete(stack, invKey)
		done[invKey] = true
	}
	return nil
}

// Run executes a plan via Runner (stop on first failure).
func (e *Engine) Run(tokens []string) error {
	plan, err := e.BuildPlan(tokens)
	if err != nil {
		return err
	}
	for _, step := range plan.Steps {
		runner, err := e.runnerFor(step.Runner)
		if err != nil {
			return err
		}
		if err := runner.Run(step.Command); err != nil {
			return err
		}
	}
	return nil
}

// PreviewLines returns expanded commands in order (deps + body).
func (e *Engine) PreviewLines(tokens []string) ([]string, error) {
	plan, err := e.BuildPlan(tokens)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(plan.Steps))
	for i, s := range plan.Steps {
		out[i] = s.Command
	}
	return out, nil
}

// ListAll returns all scripts for --ls with no tokens.
func (e *Engine) ListAll() []Script {
	if e.Catalog == nil {
		return nil
	}
	return append([]Script(nil), e.Catalog.Scripts...)
}

// ListMatches resolves --ls with tokens.
func (e *Engine) ListMatches(tokens []string) (*Match, error) {
	return e.Resolve(tokens)
}

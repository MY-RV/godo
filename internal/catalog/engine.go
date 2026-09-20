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

// Invocation is what a runner needs that a rendered command line cannot carry.
//
// A shell runner only ever wanted the finished line. A runner whose bodies are
// not shell text wants the body as written and the values as data — a plugin
// script reads a capture by name, it does not read a line godo already pasted
// it into.
type Invocation struct {
	Script   string // the catalog key, for messages
	Runner   RunnerName
	Body     string // the body as written
	Captures map[string]string
	Args     []string
}

// InvocationRunner receives the bound invocation instead of a rendered line,
// and answers for its own --preview.
//
// Preview is the runner's to answer because only it knows what its bodies do.
// godo can render a shell line because it wrote it; it cannot render a program
// it does not interpret, so it asks.
//
// A body reaching one of these is not expanded: ${godo:…} is shell-space
// syntax, and the values it would paste in are on the Invocation already.
type InvocationRunner interface {
	Runner
	RunInvocation(inv Invocation) error
	PreviewInvocation(inv Invocation) ([]string, error)
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
			return "", fmt.Errorf("%w: script %q asks for runner %q, which plugin %s should provide but did not",
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
	Command string     // the line, for runners that take one
	Runner  RunnerName // effective runner for this step
	// Inv is set when the step's runner takes the bound invocation instead of
	// a rendered line.
	Inv *Invocation
}

// steps turns one resolved match into plan steps.
//
// A runner that takes the bound invocation gets the body as written: ${godo:…}
// is shell-space syntax, and the values it would paste in travel on the
// Invocation instead.
func (e *Engine) steps(kind, source string, m *Match, runner RunnerName) ([]PlanStep, error) {
	if _, ok := e.invocationRunner(runner); ok {
		out := make([]PlanStep, 0, len(m.Script.Commands))
		for _, body := range m.Script.Commands {
			out = append(out, PlanStep{
				Kind: kind, Source: source, Command: body, Runner: runner,
				Inv: &Invocation{
					Script:   m.Script.Key,
					Runner:   runner,
					Body:     body,
					Captures: m.Captures,
					Args:     m.Args,
				},
			})
		}
		return out, nil
	}
	cmds, err := expand.ExpandAll(m.Script.Commands, m.Captures, m.Args)
	if err != nil {
		return nil, err
	}
	out := make([]PlanStep, 0, len(cmds))
	for _, c := range cmds {
		out = append(out, PlanStep{Kind: kind, Source: source, Command: c, Runner: runner})
	}
	return out, nil
}

// invocationRunner reports whether the runner takes the bound invocation.
func (e *Engine) invocationRunner(name RunnerName) (InvocationRunner, bool) {
	if r, err := e.Runners.Lookup(name); err == nil {
		ir, ok := r.(InvocationRunner)
		return ir, ok
	}
	if name == "" {
		ir, ok := e.Runner.(InvocationRunner)
		return ir, ok
	}
	return nil, false
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
	steps, err := e.steps("body", m.Script.Key, m, runner)
	if err != nil {
		return nil, err
	}
	plan.Steps = append(plan.Steps, steps...)
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
		steps, err := e.steps("dep", invKey, depMatch, depRunner)
		if err != nil {
			return err
		}
		plan.Steps = append(plan.Steps, steps...)
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
		if ir, ok := runner.(InvocationRunner); ok && step.Inv != nil {
			if err := ir.RunInvocation(*step.Inv); err != nil {
				return err
			}
			continue
		}
		if err := runner.Run(step.Command); err != nil {
			return err
		}
	}
	return nil
}

// PreviewLines returns the lines that would run, in order (deps + body).
//
// A runner that takes the bound invocation renders its own: godo can show a
// shell line because it wrote it, and has to ask for anything else.
func (e *Engine) PreviewLines(tokens []string) ([]string, error) {
	plan, err := e.BuildPlan(tokens)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, s := range plan.Steps {
		if s.Inv == nil {
			out = append(out, s.Command)
			continue
		}
		ir, ok := e.invocationRunner(s.Runner)
		if !ok {
			out = append(out, s.Command)
			continue
		}
		lines, err := ir.PreviewInvocation(*s.Inv)
		if err != nil {
			return nil, err
		}
		out = append(out, lines...)
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

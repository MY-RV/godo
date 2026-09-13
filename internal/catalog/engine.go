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

// Engine resolves matches, deps, expansion, and run/preview.
type Engine struct {
	Catalog  *Catalog
	Dialects *DialectRegistry
	Runner   Runner
}

// EngineOption configures NewEngine.
type EngineOption func(*Engine)

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
		m, err := d.Match([]Script{s}, tokens)
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
}

// BuildPlan expands deps + body without executing.
// Deps form a tree: diamond dependencies may run more than once (no DAG dedup).
func (e *Engine) BuildPlan(tokens []string) (*Plan, error) {
	m, err := e.Resolve(tokens)
	if err != nil {
		return nil, err
	}
	if err := validateArgs(m); err != nil {
		return nil, err
	}
	plan := &Plan{Match: m}
	stack := map[string]bool{}
	if err := e.appendDeps(plan, m, stack); err != nil {
		return nil, err
	}
	cmds, err := expand.ExpandAll(m.Script.Commands, m.Captures, m.Args)
	if err != nil {
		return nil, err
	}
	for _, c := range cmds {
		plan.Steps = append(plan.Steps, PlanStep{Kind: "body", Source: m.Script.Key, Command: c})
	}
	return plan, nil
}

func validateArgs(m *Match) error {
	need := expand.CommandsNeedArgs(m.Script.Commands)
	if !need && len(m.Args) > 0 {
		return fmt.Errorf("%w: %v (script %q has no ${godo:args})", ErrUnexpectedArgs, m.Args, m.Script.Key)
	}
	return nil
}

func (e *Engine) appendDeps(plan *Plan, m *Match, stack map[string]bool) error {
	for _, dep := range m.Script.Deps {
		if expand.HasArgsPlaceholder(dep) {
			return fmt.Errorf("deps must not use ${godo:args}")
		}
		expandedDep, err := expand.Expand(dep, m.Captures, nil)
		if err != nil {
			return fmt.Errorf("deps: %w", err)
		}
		depTokens := strings.Fields(expandedDep)
		if len(depTokens) == 0 {
			continue
		}
		invKey := strings.Join(depTokens, " ")
		if stack[invKey] {
			return fmt.Errorf("%w: involving %q", ErrDependencyCycle, invKey)
		}
		stack[invKey] = true

		depMatch, err := e.Resolve(depTokens)
		if err != nil {
			return fmt.Errorf("deps %q: %w", expandedDep, err)
		}
		if err := validateArgs(depMatch); err != nil {
			return err
		}
		if err := e.appendDeps(plan, depMatch, stack); err != nil {
			return err
		}
		cmds, err := expand.ExpandAll(depMatch.Script.Commands, depMatch.Captures, depMatch.Args)
		if err != nil {
			return err
		}
		for _, c := range cmds {
			plan.Steps = append(plan.Steps, PlanStep{Kind: "dep", Source: invKey, Command: c})
		}
		delete(stack, invKey)
	}
	return nil
}

// Run executes a plan via Runner (stop on first failure).
func (e *Engine) Run(tokens []string) error {
	plan, err := e.BuildPlan(tokens)
	if err != nil {
		return err
	}
	if e.Runner == nil {
		return fmt.Errorf("nil runner")
	}
	for _, step := range plan.Steps {
		if err := e.Runner.Run(step.Command); err != nil {
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

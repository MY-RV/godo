package catalog

import "fmt"

// Dialect decides whether one script answers to tokens (Strategy).
//
// Candidate order is the engine's business: it walks the catalog in definition
// order and asks each dialect about a single script, so "first match wins" means
// the same thing whatever dialect a script declares.
type Dialect interface {
	Name() DialectName
	Match(script Script, tokens []string) (*Match, error)
}

// DialectRegistry maps dialect names to implementations (Open/Closed).
type DialectRegistry struct {
	byName map[DialectName]Dialect
}

// NewDialectRegistry returns an empty registry.
func NewDialectRegistry() *DialectRegistry {
	return &DialectRegistry{byName: make(map[DialectName]Dialect)}
}

// DefaultDialects returns package + matcher registered.
func DefaultDialects() *DialectRegistry {
	r := NewDialectRegistry()
	_ = r.Register(PackageDialect{})
	_ = r.Register(MatcherDialect{})
	return r
}

// Register adds a dialect.
func (r *DialectRegistry) Register(d Dialect) error {
	if r.byName == nil {
		r.byName = make(map[DialectName]Dialect)
	}
	if d == nil {
		return fmt.Errorf("nil dialect")
	}
	name := d.Name()
	if name == "" {
		return fmt.Errorf("empty dialect name")
	}
	r.byName[name] = d
	return nil
}

// Lookup returns a dialect by name.
func (r *DialectRegistry) Lookup(name DialectName) (Dialect, error) {
	d, ok := r.byName[name]
	if !ok {
		return nil, fmt.Errorf("dialect %q not implemented", name)
	}
	return d, nil
}

// EffectiveDialect resolves @dialect override or file default.
func EffectiveDialect(script Script, fileDefault DialectName) DialectName {
	if script.Dialect != "" {
		return DialectName(script.Dialect)
	}
	return fileDefault
}

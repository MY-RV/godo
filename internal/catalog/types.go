package catalog

// FileName is the catalog filename searched via walk-up.
const FileName = "godo.yaml"

// DialectName identifies a matching strategy.
type DialectName string

const (
	DialectPackage DialectName = "package"
	DialectMatcher DialectName = "matcher"
	// Reserved names (not implemented; rejected at load until registered):
	DialectNscript DialectName = "nscript"
	DialectMatchns DialectName = "matchns"
)

// Script is one catalog entry (key + value + JSDoc decorators).
type Script struct {
	Key      string   // literal name or matcher pattern (as written)
	Commands []string // string or string[] from YAML
	Doc      string   // joined non-@ comment lines
	Deps     []string // @deps / @dependencies entries (raw, before expand)
	Dialect  string   // @dialect override; empty → file dialect
}

// Catalog is a loaded godo.yaml.
type Catalog struct {
	Path    string
	Version string
	Dialect DialectName
	Scripts []Script // definition order
}

// Match is a resolved script invocation.
type Match struct {
	Script   Script
	Captures map[string]string // pattern captures
	Args     []string          // remaining tokens for ${godo:args}
	Dialect  DialectName       // effective dialect used for this match
}

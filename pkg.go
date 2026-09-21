// Package godo is the public API for GoDo, a lightweight
// repo command catalog driven by godo.yaml.
//
// This root package is a stable public facade. Implementation lives under
// internal/catalog; the CLI entrypoint is cmd/godo.
package godo

import (
	"regexp"
	"runtime/debug"
	"strings"

	"github.com/my-rv/godo/internal/catalog"
)

// Version is the CLI/module version (not godo.yaml file.version).
// Override at link time:
//
//	-ldflags "-X github.com/my-rv/godo.Version=v0.1.0"
//
// Default stays -dev until a release build injects a tag.
// The initializer is a constant on purpose: -X only reaches a string variable
// that has one.
var Version = "0.1.0-dev"

const devVersion = "0.1.0-dev"

// Release is the version this binary should report: Version when a release
// build stamped it, and otherwise the tag `go install pkg@tag` recorded.
//
// It matters because `go install github.com/my-rv/godo/cmd/godo@v0.3.0` runs
// no linker flags of ours, so Version stays at its default and the binary
// would claim to be 0.1.0 — old enough for `engine.version` to refuse a
// catalog the binary actually satisfies.
func Release() string {
	if Version != devVersion {
		return Version
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok || !taggedVersion(bi.Main.Version) {
		return Version
	}
	return bi.Main.Version
}

// reTimestamp matches the 14-digit stamp inside a pseudo-version.
var reTimestamp = regexp.MustCompile(`[0-9]{14}`)

// taggedVersion reports whether v is a version someone tagged, as opposed to
// "(devel)" or a pseudo-version Go derived from a commit. A build from a
// working tree is a dev build however Go describes it, and saying so is more
// use than a number nobody released.
func taggedVersion(v string) bool {
	if v == "" || v == "(devel)" {
		return false
	}
	if strings.ContainsAny(v, "+ ") { // +dirty, +incompatible
		return false
	}
	return !reTimestamp.MatchString(v)
}

// Re-exported names and sentinels.
const FileName = catalog.FileName

type (
	DialectName     = catalog.DialectName
	RunnerName      = catalog.RunnerName
	RunnerRegistry  = catalog.RunnerRegistry
	Script          = catalog.Script
	Catalog         = catalog.Catalog
	EngineSpec      = catalog.EngineSpec
	Plugin          = catalog.Plugin
	Match           = catalog.Match
	ExitError       = catalog.ExitError
	Dialect         = catalog.Dialect
	DialectRegistry = catalog.DialectRegistry
	Runner          = catalog.Runner
	ArgsAwareRunner = catalog.ArgsAwareRunner
	Engine          = catalog.Engine
	EngineOption    = catalog.EngineOption
	Plan            = catalog.Plan
	PlanStep        = catalog.PlanStep
	PackageDialect  = catalog.PackageDialect
	MatcherDialect  = catalog.MatcherDialect
)

const (
	RunnerInherit = catalog.RunnerInherit

	DialectPackage = catalog.DialectPackage
	DialectMatcher = catalog.DialectMatcher
	DialectNscript = catalog.DialectNscript
	DialectMatchns = catalog.DialectMatchns
)

var (
	ErrNoMatch         = catalog.ErrNoMatch
	ErrNoTokens        = catalog.ErrNoTokens
	ErrUnexpectedArgs  = catalog.ErrUnexpectedArgs
	ErrDependencyCycle = catalog.ErrDependencyCycle
	ErrInvalidCatalog  = catalog.ErrInvalidCatalog
	ErrInvalidCapture  = catalog.ErrInvalidCapture
	ErrUnknownRunner   = catalog.ErrUnknownRunner
)

var (
	FindFile           = catalog.FindFile
	LoadFile           = catalog.LoadFile
	Parse              = catalog.Parse
	NewDialectRegistry = catalog.NewDialectRegistry
	DefaultDialects    = catalog.DefaultDialects
	EffectiveDialect   = catalog.EffectiveDialect
	NewRunnerRegistry  = catalog.NewRunnerRegistry
	DefaultRunners     = catalog.DefaultRunners
	EffectiveRunner    = catalog.EffectiveRunner
	NewEngine          = catalog.NewEngine
	WithDialects       = catalog.WithDialects
	WithRunners        = catalog.WithRunners
	ExitCode           = catalog.ExitCode
	IsCaptureName      = catalog.IsCaptureName
)

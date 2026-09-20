// Package godo is the public API for GoDo, a lightweight
// repo command catalog driven by godo.yaml.
//
// This root package is a stable public facade. Implementation lives under
// internal/catalog; the CLI entrypoint is cmd/godo.
package godo

import "github.com/my-rv/godo/internal/catalog"

// Version is the CLI/module version (not godo.yaml file.version).
// Override at link time:
//
//	-ldflags "-X github.com/my-rv/godo.Version=v0.1.0"
//
// Default stays -dev until a release build injects a tag.
var Version = "0.1.0-dev"

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

# Reference — Go API

Package: `github.com/my-rv/godo` (facade).

## Version

```go
var Version string // default 0.1.0-dev; override with ldflags
```

## Common entrypoints

| Symbol | |
|--------|--|
| `FileName` | `"godo.yaml"` |
| `FindFile` | Walk cwd parents for catalog |
| `LoadFile` / `Parse` | Load catalog |
| `NewEngine` / `WithDialects` / `WithRunners` | Engine wiring |
| `NewRunnerRegistry` / `DefaultRunners` / `EffectiveRunner` | Named runners (`RunnerInherit`) |

A catalog that names no runner uses the `Runner` injected into the engine, so
`NewEngine(cat, myRunner)` keeps working with nothing registered. The CLI
injects the caller's own shell there, which is why `inherit` is the default for
a `godo.yaml`.
| `ArgsAwareRunner` | Runner decides whether a body takes leftover tokens |
| `EngineSpec` / `Plugin` | The `engine:` block: binary minimum + declared plugins |
| `Engine.Run` / `PreviewLines` | Execute / expand |
| `ExitCode` | Map errors to process codes |

## Types (aliases)

`Catalog`, `Script`, `Engine`, `Dialect`, `Match`, `Plan`, … — see `pkg.go` and godoc.

## Errors

`ErrNoMatch`, `ErrNoTokens`, `ErrUnexpectedArgs`, `ErrDependencyCycle`, `ErrInvalidCatalog`, `ErrInvalidCapture`, `ErrUnknownRunner`, …

Pre-1.0: symbols may move; prefer pinning modules. [versioning.md](../dev/versioning.md).

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
| `NewEngine` / `WithDialects` | Runner |
| `Engine.Run` / `PreviewLines` | Execute / expand |
| `ExitCode` | Map errors to process codes |

## Types (aliases)

`Catalog`, `Script`, `Engine`, `Dialect`, `Match`, `Plan`, … — see `pkg.go` and godoc.

## Errors

`ErrNoMatch`, `ErrNoTokens`, `ErrUnexpectedArgs`, `ErrDependencyCycle`, `ErrInvalidCatalog`, `ErrInvalidCapture`, …

Pre-1.0: symbols may move; prefer pinning modules. [versioning.md](../dev/versioning.md).

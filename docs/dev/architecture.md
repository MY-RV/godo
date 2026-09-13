# Architecture — godo

**Public facade** (`github.com/my-rv/godo` via `pkg.go`) + **engine** (`internal/catalog`) + thin **CLI** (`cmd/godo` → `internal/cli`).

## Package map

| Layer | Path | Responsibility |
|-------|------|----------------|
| Facade | `pkg.go` | Stable `import "github.com/my-rv/godo"` surface |
| Engine | `internal/catalog` | Load/validate, dialects, `Engine`, errors |
| Expand | `internal/expand` | Placeholders (fail-closed) |
| Exec | `internal/execshell` | Shell runner; cwd = catalog dir |
| CLI | `internal/cli` | Flags → engine |
| Entrypoint | `cmd/godo` | `os.Exit` only |
| Docs / dogfood | `docs/` (product), `docs/dev/` (contributors), `godo.yaml` | |

Root stays free of engine source; only the facade (+ tests of the facade) live beside `go.mod`.

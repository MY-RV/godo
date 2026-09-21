# Versioning

Module: `github.com/my-rv/godo`

## SemVer

- **`v0.x`**: public API / CLI may change without a major bump. Prefer pinning to a
  commit or minor version in production consumers.
- **`v1.0.0` and later**: follow [Go module semantic import versioning](https://go.dev/doc/modules/version-numbers).
  Breaking changes require a new major module path (`/v2`, …).

## Two “versions”

| What | Where |
|------|--------|
| Binary / module version | `godo.Version` (ldflags `-X github.com/my-rv/godo.Version=…`) |
| Catalog file contract | field `version:` inside `godo.yaml` |

Do not confuse them.

Read the first one through `godo.Release()`, never `godo.Version` directly.
`go install …/cmd/godo@vX.Y.Z` links none of our flags, so `Version` stays at
its `0.1.0-dev` default; `Release()` takes the tag from Go's build info when it
finds one there. A build from a working tree keeps saying `0.1.0-dev` — Go
describes it with a pseudo-version, and no one released that.

`engine.version` is compared against `Release()`, so a binary installed at a
tag is judged by the tag.

## Stability today (v0.1)

| Surface | Stability |
|---------|-----------|
| CLI model (bare tokens = scripts; `-e` = engine) | Intentional; changes require contract update |
| `godo.yaml` dialects `package` / `matcher` | Evolving in v0 |
| Facade types (`LoadFile`, `Engine`, …) | Evolving in v0 |
| Dialects `nscript` / `matchns` | Backlog — not shipped |

See [CHANGELOG.md](../../CHANGELOG.md).

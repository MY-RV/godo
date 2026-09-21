# Documentation

Product docs for using and shipping **GoDo** (CLI/module: `godo`).

| Doc | |
|-----|--|
| [Overview](./overview.md) | What GoDo is |
| [Why GoDo exists](./story.md) | Origin story (EN) |
| [Por qué existe GoDo](./story.es.md) | Misma historia (ES) |
| [Getting started](./getting-started.md) | Install + first `godo.yaml` |
| [Install](./install.md) | Binaries, Go, self-update |
| [Distribution](./distribution.md) | Releases and package channels |
| [Release](./release.md) | Tags + GoReleaser (for publishers) |
| [Roadmap](./roadmap.md) | Promises for v0.1 / post-v0.1 / v1.0 |
| [Contract](./contract.md) | Normative CLI + file behavior |

## Guides

One topic per file (copy-paste examples):

| Doc | |
|-----|--|
| [Scripts](./guide/scripts.md) | Bare tokens + `godo.yaml` keys |
| [Engine](./guide/engine.md) | `-e` / `--engine` built-ins |
| [Preview and ls](./guide/preview-and-ls.md) | `--preview`, `--ls` |
| [Package dialect](./guide/package-dialect.md) | Exact names (default) |
| [Matcher dialect](./guide/matcher-dialect.md) | Pattern keys + captures |
| [Runners](./guide/runners.md) | Which shell runs your line; `runner:` / `# @runner` |
| [Deps](./guide/deps.md) | `# @deps` |
| [Placeholders](./guide/placeholders.md) | `${…}` / `${godo:args…}` |
| [Working directory](./guide/working-directory.md) | Catalog root, walk-up |
| [Library: Load and Run](./guide/library-load-run.md) | Embed the engine |

## Reference

| Doc | |
|-----|--|
| [CLI reference](./reference/cli.md) | Flag / engine tables |
| [API reference](./reference/api.md) | Public Go surface |

## Contributors

Code/layout/contribution detail: [`docs/dev/`](./dev/README.md).  
Also root [`CONTRIBUTING.md`](../CONTRIBUTING.md) and [`SECURITY.md`](../SECURITY.md).

Contract wins over guides when they disagree.

# Overview

**GoDo** is a lightweight **repo command catalog**: one `godo.yaml` lists named scripts; the `godo` binary runs them.

It is a versioned index of “how we do X in this repo,” with a small, predictable expand/exec model.

Product name: **GoDo**. CLI, module path, and file name stay lowercase `godo` (Go / Unix convention).

## Mental model

| Invocation | Meaning |
|------------|---------|
| `godo test` | Run the `test` script from `godo.yaml` |
| `godo -e version` | Built-in **engine** command (not a script) |
| `godo --preview test` | Expand deps + body; do not execute |

Bare tokens never compete with engine commands: builtins live behind `-e` / `--engine` (plus a few flag aliases like `--version`).

## Shape (v0.1)

- Config: `godo.yaml`
- Dialects: `package`, `matcher`
- Expand fails closed on bad placeholders / unexpected args

## Two audiences

1. **CLI users** — install a binary, drop a `godo.yaml`, run scripts  
2. **Go library users** — `import "github.com/my-rv/godo"` and drive the engine in-process  

## Why it exists

Origin story: [EN](./story.md) · [ES](./story.es.md).

## Source of truth

Behavioral rules: [contract.md](./contract.md).  
What we promise to ship: [roadmap.md](./roadmap.md).  
Layout (contributors): [dev/architecture.md](./dev/architecture.md).

# Contributing

Thanks for helping with **GoDo**. Keep changes small and contract-first.

Product name is **GoDo**; code/CLI identifiers stay `godo` (see [`docs/dev/standards.md`](./docs/dev/standards.md)).

## Prerequisites

- Go version from [`go.mod`](./go.mod)
- Read [`docs/contract.md`](./docs/contract.md) before changing CLI or `godo.yaml` behavior

## Setup

```bash
git clone https://github.com/MY-RV/godo.git
cd godo
go build -o godo ./cmd/godo
./godo ci
```

Dogfood:

```bash
./godo check
```

## Loop

```bash
go build -o godo ./cmd/godo   # once, or after CLI changes
./godo ci                     # vet, test, race, staticcheck, fuzz, build
```

Scripts live in [`godo.yaml`](./godo.yaml) — that is the project gate, not Make.

## Pull requests

- One concern per PR
- Behavior change ⇒ update the contract (and relevant docs) in the **same** PR
- Tests for the path you touch (happy + edge)
- No drive-by refactors
- Commit author should match your GitHub identity (e.g. `マイノル <97069334+MY-RV@users.noreply.github.com>`)

## Scope

Behavior changes need a contract update. Prefer the smallest change that fits [`docs/contract.md`](./docs/contract.md) and existing patterns.

## Code

See [`docs/dev/standards.md`](./docs/dev/standards.md) and [`docs/dev/architecture.md`](./docs/dev/architecture.md).

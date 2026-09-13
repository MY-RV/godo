# GoDo

**GoDo** (ve y haz). A thin repo command catalog: name the chores in `godo.yaml`, run them with `godo`.

The repo should carry a shared list of what you can run — same names for anyone in the tree. Many stacks only have partial maps; JavaScript gets closest with `package.json` `scripts`. GoDo is that idea as a small, language-agnostic file you can list and preview before it hits the shell. Full story: [EN](./docs/story.md) · [ES](./docs/story.es.md).

```bash
godo test
godo --preview check
godo -e version
```

## Install

```bash
go install github.com/my-rv/godo/cmd/godo@latest
```

Binaries (no Go): [docs/install.md](./docs/install.md).  
Brew / Scoop (after v0.1): [docs/distribution.md](./docs/distribution.md).

## Library

```go
import "github.com/my-rv/godo"

cat, err := godo.LoadFile("godo.yaml")
_ = godo.NewEngine(cat, nil).PreviewLines([]string{"test"})
```

## Docs

[Getting started](./docs/getting-started.md) · [Guides](./docs/README.md) · [Contract](./docs/contract.md) · [Roadmap](./docs/roadmap.md)

## Develop

```bash
go build -o godo ./cmd/godo
./godo ci
```

[Contributing](./CONTRIBUTING.md) · [Security](./SECURITY.md) · [MIT](./LICENSE)

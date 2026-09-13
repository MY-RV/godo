# Library: Load and Run

Embed godo’s catalog engine without the CLI. Dialect default matches the CLI: omit `dialect` → `package`.

## Load + preview

```go
package main

import (
	"fmt"
	"os"

	"github.com/my-rv/godo"
)

func main() {
	path, err := godo.FindFile(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cat, err := godo.LoadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	lines, err := godo.NewEngine(cat, nil).PreviewLines([]string{"check"})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, line := range lines {
		fmt.Println(line)
	}
}
```

Given a catalog with `@deps` on `check`, **expected** `lines` (same as `godo --preview check`):

```text
go vet ./...
go test ./...
go build -o app .
```

## Run

Pass a `Runner` (or `nil` only for preview). Production CLI uses `internal/execshell`; library callers supply their own:

```go
eng := godo.NewEngine(cat, myRunner)
err := eng.Run([]string{"test"})
```

Same match / expand / deps rules as `godo test`.

## API map

| Symbol | Same idea as |
|--------|----------------|
| `godo.FindFile(cwd)` | Walk-up for `godo.yaml` |
| `godo.LoadFile` / `Parse` | Parse catalog |
| `godo.NewEngine(cat, runner)` | Wire engine |
| `Engine.Run(tokens)` | `godo <tokens…>` |
| `Engine.PreviewLines(tokens)` | `godo --preview <tokens…>` |
| `godo.Version` | `godo --version` |

Full surface: [API reference](../reference/api.md).

## Next

- [Scripts](./scripts.md)
- [Preview and ls](./preview-and-ls.md)
- [Contract](../contract.md)

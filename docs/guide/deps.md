# Dependencies (`@deps`)

A `# @deps` line immediately above a script runs other scripts first, then the body.

`# @dependencies` is an alias of `@deps`.

## Basic order

```yaml
version: "0.1"

scripts:
  vet: go vet ./...
  test: go test ./...
  # local gate
  # @deps vet, test
  check: go build -o app .
```

```bash
godo --preview check
```

**Prints** (deps first, then body):

```text
go vet ./...
go test ./...
go build -o app .
```

```bash
godo --ls check
```

**Prints** the definition (deps listed, body not expanded):

```text
local gate
@deps vet
@deps test
check:
  go build -o app .
```

## Rules (with expected outcomes)

**Entries are comma-separated.** Each entry is its own script invocation (space-separated tokens inside the entry).

**Caller args are not forwarded to deps.**

```yaml
scripts:
  seed: echo seed ${godo:args}
  # @deps seed
  ship: echo ship ${godo:args}
```

```bash
godo --preview ship --prod
```

**Prints:**

```text
echo seed 
echo ship --prod
```

(`seed` got no leftover tokens; empty `${godo:args}` still expands in place.)

**No `${godo:args}` inside `@deps`.** Use captures already bound by a matcher key instead (`lint ${MODULE}`).

**Cycles error.** Diamond graphs may run a shared node more than once (no DAG dedup):

```yaml
scripts:
  leaf: echo leaf
  # @deps leaf
  mid: echo mid
  # @deps mid, leaf
  root: echo root
```

```bash
godo --preview root
```

**Prints:**

```text
echo leaf
echo mid
echo leaf
echo root
```

## Next

- [Preview and ls](./preview-and-ls.md)
- [Matcher dialect](./matcher-dialect.md)
- [Contract](../contract.md)

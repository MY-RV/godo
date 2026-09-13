# Scripts

Bare tokens select a script from `godo.yaml`.

## Run one script

```yaml
version: "0.1"

scripts:
  test: go test ./...
```

```bash
godo test
```

**Runs** (cwd = directory of `godo.yaml`):

```text
go test ./...
```

`dialect` may be omitted; it defaults to `package`.

## Command list

A YAML list runs in order and stops on the first failure.

```yaml
scripts:
  boot:
    - echo one
    - echo two
```

```bash
godo --preview boot
```

**Prints:**

```text
echo one
echo two
```

## Doc comments

A `#` line **without** `@`, immediately above the key, is the script doc.

```yaml
scripts:
  # run unit tests
  test: go test ./...
  vet: go vet ./...
```

```bash
godo --ls
```

**Prints:**

```text
test  # run unit tests
vet
```

## Next

- [Preview and ls](./preview-and-ls.md)
- [Engine commands](./engine.md)
- [Deps](./deps.md)
- [Contract](../contract.md)

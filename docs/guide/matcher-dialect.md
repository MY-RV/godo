# Matcher dialect

Keys are token patterns. First matching key (definition order) wins.

## Specific before general

```yaml
version: "0.1"
dialect: matcher

scripts:
  test ${MODULE}: go test ./${MODULE}/...
  test: go test ./...
```

```bash
godo --preview test internal/catalog
```

**Prints** (`MODULE` = `internal/catalog`):

```text
go test ./internal/catalog/...
```

```bash
godo --preview test
```

**Prints** (falls through to the exact key):

```text
go test ./...
```

## Captures in `@deps`

```yaml
scripts:
  lint ${MODULE}: go run ./lint ${MODULE}
  # @deps lint ${MODULE}
  test ${MODULE}: go test ./${MODULE}/...
```

```bash
godo --preview test internal/catalog
```

**Prints:**

```text
go run ./lint internal/catalog
go test ./internal/catalog/...
```

## Override one script (file stays package)

```yaml
version: "0.1"

scripts:
  # @dialect matcher
  "run ${GRP} ${SCR}": go run ./scripts/${GRP}/${SCR} ${godo:args}
```

```bash
godo --ls run _ _
```

**Prints:**

```text
@dialect matcher
run ${GRP} ${SCR}:
  go run ./scripts/${GRP}/${SCR} ${godo:args}
```

```bash
godo --preview run api seed --verbose
```

**Prints:**

```text
go run ./scripts/api/seed --verbose
```

## Leftover tokens

After the pattern matches, leftovers feed `${godo:args…}`. If the body does not use them and leftovers remain, godo errors (same rule as package).

## Next

- [Package dialect](./package-dialect.md)
- [Placeholders](./placeholders.md)
- [Preview and ls](./preview-and-ls.md)
- [Contract](../contract.md)

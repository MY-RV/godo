# Package dialect

Exact script names. This is the **default** when `dialect` is omitted.

## Match by name

```yaml
version: "0.1"

scripts:
  test: go test ./...
  seed: go run ./scripts/seed ${godo:args}
```

```bash
godo --preview test
```

**Prints:**

```text
go test ./...
```

```bash
godo --preview seed --env=dev
```

**Prints:**

```text
go run ./scripts/seed --env=dev
```

## Extra tokens without `${godo:args}`

```bash
godo --preview test --flag
```

**Errors:**

```text
godo: unexpected args: [--flag] (script "test" has no ${godo:args})
```

## Keys are literals

Package keys cannot contain `${capture}`. For patterns, use matcher (file-level or `# @dialect matcher`):

```yaml
scripts:
  test: go test ./...
  # @dialect matcher
  "run ${GRP} ${SCR}": go run ./scripts/${GRP}/${SCR} ${godo:args}
```

```bash
godo --preview run db migrate --dry
```

**Prints:**

```text
go run ./scripts/db/migrate --dry
```

## Next

- [Matcher dialect](./matcher-dialect.md)
- [Placeholders](./placeholders.md)
- [Contract](../contract.md)

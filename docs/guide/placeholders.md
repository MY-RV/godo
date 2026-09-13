# Placeholders

godo expands `${…}` **in process** before the shell runs the line. Host `$VAR` / `%VAR%` are not catalog binds.

## `${godo:args}`

```yaml
version: "0.1"

scripts:
  seed: go run ./scripts/seed ${godo:args}
```

```bash
godo --preview seed --env=dev
```

**Prints:**

```text
go run ./scripts/seed --env=dev
```

## Matcher capture + leftovers

```yaml
dialect: matcher

scripts:
  build ${name}: echo building ${name} -- ${godo:args}
```

```bash
godo --preview build api --release
```

**Prints:**

```text
echo building api -- --release
```

## Forms

| Form | Behavior |
|------|----------|
| `${name}` | Capture from a matcher key |
| `${godo:args}` | Remaining tokens after the match, space-joined |
| `${godo:args[i]}` | One token; **error** if out of range |
| `${godo:args[i..j]}` | Half-open slice `[i, j)` (Go semantics) |

Capture names must match `[A-Za-z_][A-Za-z0-9_]*`.

## Fail closed

Unknown or malformed `${…}` is an error — no silent empty string.

```yaml
scripts:
  bad: echo ${nope}
```

```bash
godo --preview bad
```

**Errors:**

```text
godo: unknown capture ${nope}
```

## Next

- [Matcher dialect](./matcher-dialect.md)
- [Package dialect](./package-dialect.md)
- [Contract](../contract.md)

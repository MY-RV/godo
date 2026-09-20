# Placeholders

godo works in **two spaces**, and each has its own syntax. The rule is one line:

> Bare braces where the text belongs to godo. `${godo:…}` where it belongs to the shell.

| Space | Where | Syntax | Meets a shell? |
|-------|-------|--------|----------------|
| godo | matcher keys, `@deps` entries | `${NAME}` | never |
| shell | script bodies | `${godo:…}` | always |

Because a body is shell text, godo claims **only** the `${godo:…}` namespace there
and leaves every other `${…}` alone. A collision is not resolved — it cannot occur.

```yaml
engine:
  dialect: matcher

scripts:
  # @deps lint ${MODULE}                        # godo space
  build ${MODULE}:                              # godo space
    - go build ./${godo:argv[MODULE]}/...       # shell space
    - echo $HOME and ${HOME}                    # the shell's, untouched
```

## `${godo:argv[NAME]}`

Consumes a capture bound by the matcher key.

```yaml
engine:
  dialect: matcher

scripts:
  build ${name}: echo building ${godo:argv[name]} -- ${godo:args}
```

```bash
godo --preview build api --release
```

**Prints:**

```text
echo building api -- --release
```

`${godo:argv[0]}` is an error on purpose: `argv` indexes captures by name. For a
positional argument use `${godo:args[0]}`.

## `${godo:args}`

Everything left over after the match.

```yaml
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

## Host environment variables

Nothing to escape — bodies are shell text, so write shell:

```yaml
scripts:
  a: echo $HOME          # shell expands
  b: echo ${HOME}        # shell expands
  c: echo $$             # the shell's PID
```

godo does not read any of these, does not validate them, and does not fail on
them. They reach the shell byte for byte.

## Quoting

Values godo substitutes are **shell-quoted**: one argument in is one argument
out, whatever it contains.

```bash
godo --preview greet 'a; touch PWNED'
```

**Prints:**

```text
echo hello 'a; touch PWNED'
```

Ordinary tokens are left bare so previews stay readable — `godo test -v ./...`
previews as `go test -v ./...`, not `go test '-v' './...'`.

### `:raw`

`:raw` turns quoting off for one placeholder, when the shell is meant to
interpret the value:

```yaml
scripts:
  find: ls ${godo:args:raw}
```

```bash
godo find '*.go'     # glob expands
```

Use it deliberately: a `:raw` placeholder fed untrusted input is a shell
injection.

## Forms

Inside a body, `${godo:…}` only:

| Form | Behavior |
|------|----------|
| `${godo:argv[NAME]}` | Capture bound by the matcher key; quoted |
| `${godo:args}` | Remaining tokens after the match, each quoted, space-joined |
| `${godo:args[i]}` | One token, quoted; **error** if out of range |
| `${godo:args[i..j]}` | Half-open slice `[i, j)` (Go semantics), quoted |
| `${godo:…:raw}` | Any of the above, interpolated verbatim |
| anything else `${…}` | Not godo's — passed to the shell untouched |

In a matcher key or a `@deps` entry, bare `${NAME}` only. `${godo:…}` is rejected
there: no shell is involved, so there is nothing to disambiguate from.

Capture names must match `[A-Za-z_][A-Za-z0-9_]*`.

## Fail closed

An unknown or malformed `${godo:…}` is an error — no silent empty string.

```yaml
scripts:
  bad: echo ${godo:argv[nope]}
```

```bash
godo --preview bad
```

**Errors:**

```text
godo: unknown capture "nope" (not bound by the matcher key)
```

Bare `${nope}` is **not** an error: it is not godo's to judge.

## Next

- [Matcher dialect](./matcher-dialect.md)
- [Package dialect](./package-dialect.md)
- [Contract](../contract.md)

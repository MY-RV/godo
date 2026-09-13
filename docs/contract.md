# Contract — GoDo

## CLI

```
godo [flags] [script-tokens...]
godo -e|--engine <command>
```

**Model:** bare tokens = scripts from `godo.yaml` (repo command-catalog style).  
godo’s **own** commands go after `-e` / `--engine` (they do not compete with script names).

| Flag | |
|------|--|
| `--help` | CLI help |
| `--version` | Binary version (alias of `-e version`) |
| `--ls` | List all scripts (name + doc if present) |
| `--ls <tokens...>` | Show the match(es) |
| `--preview [tokens...]` | Expand deps + body; do not run |
| `--update` | Alias of `-e update` |
| `--update-check` | Alias of `-e update check` |
| `-e, --engine <cmd>` | Built-in godo command |

### Engine commands (`-e`)

| | |
|--|--|
| `-e version` | Binary version |
| `-e update` | Download asset from GitHub Releases |
| `-e update check` | Report only whether an update is available |
| `-e help` | Engine help |

`godo update` (without `-e`) is a **catalog script** if one is defined.

Flags go **before** script tokens (`godo --ls test`, not `godo test --ls`).

No `--` separator between script and args. Script tokens go straight through.

To list a pattern with two captures, any match works; recommended:

```
godo --ls _ _
```

## File

`godo.yaml`

## Version

File field: `{file}.version`. Required.

```yaml
version: "0.1"
```

Contract version of the file (not the `godo` binary).

## Dialect

File field: `{file}.dialect`. Optional; default `package`.

```yaml
dialect: package   # omit → package
```

File default. Scripts may override with `# @dialect`.

## Bind

Godo expands placeholders **in-process** before `exec` / preview. No host-shell bind (`$VAR` / `%VAR%`).

| Placeholder | |
|-------------|--|
| `${name}` | Capture; `name` = `[A-Za-z_][A-Za-z0-9_]*` (fail-closed if invalid) |
| `${godo:args}` | Remaining tokens (space-joined) |
| `${godo:args[i]}` | One token; **error** if out of range |
| `${godo:args[i..j]}` | Half-open slice `[i,j)` (Go style) |

## Exec cwd

Commands run with working directory = **directory of the resolved `godo.yaml`** (not necessarily the caller’s cwd). Relative paths in the catalog stay stable from subdirs.

Trust: expanded lines pass through the host shell (`sh -c` / `cmd /C`).

## Script decorators (JSDoc style)

YAML comment block **immediately above** the script key. Apply to scripts only.

| Line | |
|------|--|
| `# text` (no `@`) | Doc |
| `# @dialect <name>` | Match this script with another dialect (override of `{file}.dialect`) |
| `# @deps a, b` | Run `a`, then `b`, then the value |
| `# @dependencies a, b` | Alias of `@deps` |

```yaml
# Local CI
# @deps lint, test
ci: go build ./...
```

```yaml
# @deps lint ${MODULE}
# @dialect matcher
test ${MODULE}: go test ./${MODULE}/...
```

- `@dialect` — per-script override; without it, uses `{file}.dialect`
- `@deps` / `@dependencies` — invocation like `godo …` (space-separated tokens; entries separated by `,`)
- literals and `${name}` from captures **already bound** by the match
- no `${godo:args}` / slices in `@deps`
- order = list order; stop on first failure
- cycle → error (on the **expanded** invocation)
- caller args are **not** forwarded to deps
- diamond (A→B,C and B→C): C may run more than once (**no** DAG dedup)
- `--preview`: deps + body, in order (deps already expanded)
- no `@` → no deps (manual composition via `godo …` in the value remains valid)

## Dialect `package`

**Literal** keys.

```
scripts.<name>: string | string[]
```

```yaml
version: "0.1"
dialect: package

scripts:
  # Unit tests
  test: go test ./...

  # Seed
  seed: go run -C scripts/service/seed . ${godo:args}

  # Run scripts/{group}/{script}
  # @dialect matcher
  "${GRP} ${SCR}": go run -C scripts/${GRP}/${SCR} . ${godo:args}

  # Local stack
  # @deps wait
  boot:
    - docker compose up -d
    - ./scripts/wait-healthy.sh
```

- `string` — one command
- `string[]` — in order; stop on first failure
- `${godo:args}` / `${godo:args[i]}` / `${godo:args[i..j]}` — optional in the value
- without `${godo:args}` → extra tokens error
- no captures in keys (that is `matcher`)

## Dialect `matcher`

Keys = routes over tokens (Express-style). First match in (definition order).  
`${name}` = capture; the user chooses the name.

```
scripts.<pattern>: string | string[]
```

```yaml
version: "0.1"
dialect: matcher

scripts:
  "${GRP} ${SCR}": go run -C scripts/${GRP}/${SCR} . ${godo:args}
  # @deps lint ${MODULE}
  test ${MODULE}: go test ./${MODULE}/...
  test: go test ./...
  seed: go run -C scripts/service/seed . ${godo:args}
```

Positional equivalent if the user prefers those names:

```yaml
  "${ARG0} ${ARG1}": go run -C scripts/${ARG0}/${ARG1} . ${godo:args}
```

Placeholders: see **Bind** (same grammar). Without `${godo:args}` in the value → no remainder; leftover tokens after the match error.

## Future dialects (reserved names)

| Dialect | |
|---------|--|
| `nscript` | from nkfile |
| `matchns` | matcher + nkfile |

Reserved: cannot be used as `{file}.dialect` / `@dialect` until implemented and registered.

## Common semantics

1. Resolve `godo.yaml` from cwd upward through parents.
2. Read `{file}.version` and `{file}.dialect` (omit dialect → `package`; dialect must be implemented).
3. Match → bind captures → expand `@deps` → run deps → expand value → execute (or `--preview` / `--ls`).
4. Exit code = of the command (or the first failure in a list / deps); catalog/match errors → `1`.
5. Exec cwd = directory of the `godo.yaml` (see **Exec cwd**).

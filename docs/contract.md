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

## Engine

File field: `{file}.engine`. Optional.

```yaml
engine:
  version: ">=0.3.0"
  dialect: matcher
  runner: bash
  plugins:
    - source: https://github.com/my-rv/godo-micropy@v1.2.0
      sha256: "661471…"
      provides: [runner:micropy]
      config:
        proc: {exec: true, spawn: false}
        fs:   {slink: true}
```

`scripts:` is the catalog — the data. `engine:` is every dial godo turns while
reading and running it: the binary it expects, the dialect, the runner, the
plugins. Keeping them apart is what lets the toolchain side grow without the
script side growing with it.

| Field | |
|-------|--|
| `version` | Minimum `godo` binary |
| `dialect` | How keys match tokens. Default `package` |
| `runner` | How a body becomes a process. Default: the shell you are in |
| `plugins` | Declared plugins (nothing loads them yet) |

### `engine.version`

The minimum `godo` binary, as `"0.3.0"` or `">=0.3.0"`. Only a minimum — no
ranges, no `^`, no `~`. A binary below it refuses the catalog before running
anything:

```
godo: ./godo.yaml needs godo 0.3.0 or newer; this is 0.2.0 (godo -e update)
```

Comparison drops any pre-release suffix, so a `-dev` build is judged by its
numbers.

### `engine.plugins`

```
godo -e plugins                    what this catalog declares, and its state
godo -e plugins install            fetch everything it declares
godo -e plugins install <source>   add one, and fetch it
```

`install <source>` computes the digest from the artifact and writes the entry
into `godo.yaml`. The digest is never asked for: a person cannot check a hash
by reading it, so asking for one is how wrong hashes get committed.

Artifacts live in `<user cache>/godo/plugins`, named by digest. A file sitting
beside the `godo.yaml` is loaded from where it is — asking someone to install
what they can already see would be ceremony, and its digest is checked either
way. Anything else must be installed first; a run is not the moment to discover
that something has to be downloaded.

`http://` sources are refused. An artifact is code, and its integrity cannot
rest on a transport anyone on the path can rewrite.

| Field | |
|-------|--|
| `source` | Required. An `https://` URL, or a path relative to the `godo.yaml` |
| `sha256` | **Required.** A plugin is third-party code that runs when someone types `godo test`; without a digest there is nothing to verify it is the code that was reviewed |
| `provides` | Required. `"<kind>:<name>"` entries, kind being `runner` or `dialect`. Two plugins may not provide the same one |
| `config` | Optional, and the plugin's: its keys, its meaning, its defaults. godo carries it across and reads only `fs.mount`, which says which directories the sandbox can see |

A script asking for a runner a plugin provides fails by naming that plugin:

```
godo: unknown runner: script "wt" asks for runner "micropy", provided by
plugin https://github.com/my-rv/godo-micropy@v1.2.0 — this build cannot load
plugins
```

`godo -e runners` lists what a catalog declares, beside what the machine has.

## Dialect

File field: `{file}.engine.dialect`. Optional; default `package`.

```yaml
engine:
  dialect: package   # omit → package
```

File default. Scripts may override with `# @dialect`.

A top-level `dialect:` is still read — it shipped in 0.1 and 0.2 — and
`engine.dialect` wins if both are present.

## Runner

File field: `{file}.engine.runner`. Optional; default `inherit`.

```yaml
engine:
  runner: inherit   # omit → inherit
```

File default. Scripts may override with `# @runner`.

`dialect` answers *which script responds to these tokens*; `runner` answers
*how the resolved body becomes a process*. They are independent: any dialect
may be paired with any runner.

| Runner | |
|--------|--|
| `inherit` | The shell you are already in. Default |
| *a shell name* | `sh`, `bash`, `zsh`, `dash`, `ksh`, `ash`, `fish`, `nu`, `cmd`, `pwsh`, `powershell` |

`godo -e runners` lists what is usable on the machine you are on.

godo does not manage these shells — it resolves the name on `PATH` and hands
the line over. The line is yours and the shell is yours; godo is the proxy.

### `inherit`

The default. Your shell runs your line:

```yaml
  arr: "arr=(a b c); echo ${arr[1]}"
```

```
zsh → a       (zsh indexes arrays from 1)
sh  → b       (sh indexes arrays from 0)
```

Selection, in order:

| | |
|--|--|
| `GODO_SHELL` | Always wins |
| Unix | `$SHELL`, else `/bin/sh` |
| Windows | the parent process when it is a shell, else `%ComSpec%` |

godo answers "which shell am I in" from the parent process, which is the only
thing that knows. No command run *inside* a shell can report it — it would only
describe the shell godo just started. To confirm it yourself, in your own
terminal: `echo $0` (sh, bash, zsh, dash, ksh), `echo $version` (fish),
`$PSVersionTable.PSVersion` (PowerShell), `echo %COMSPEC%` (cmd). `godo -e
runners` prints these too.

Windows reads the parent process because the environment cannot answer:
PowerShell sets `PSModulePath` and everything it starts inherits it, so a
`cmd.exe` opened from PowerShell would look like PowerShell. On Unix, `$SHELL`
is the login shell rather than the one running right now — bash started inside
zsh still reports zsh. `GODO_SHELL` is how you disagree with either.

The shell is started non-interactively and without a profile (`-c`, `/C`, or
`-NoProfile -Command`), so this gives you your shell's **grammar**, not your
shell's **setup** — your aliases and functions are not there.

**Consequence:** a catalog is read by the shell of whoever runs it, so a script
written in zsh syntax behaves differently for a teammate on bash. That is
deliberate — godo is a proxy and promises neither cross-OS nor cross-shell.

### A shell by name

To pin one shell for everyone, name it:

```yaml
  # @runner bash
  ci: shopt -s globstar && echo **/*.go
```

The name is **logical, never a path**: write `cmd`, not `cmd.exe`; `pwsh`, not
`pwsh.exe` or `ps1` (`.ps1` is a script extension, not the program). The
platform's extension is `PATH`'s business, so the same `godo.yaml` reads the
same everywhere.

A name outside the list above is refused rather than run — otherwise
`# @runner git` would quietly become `git -c <line>`. For a shell not on the
list, set `GODO_SHELL` and use `inherit`.

A shell that is not installed here fails when the plan is built, so nothing
executes.

Whether a script accepts the tokens left over after the match is also the
runner's question. A shell accepts them only when the body references
`${godo:args…}`; a runner whose body is a program answers for itself.

## Bind

Godo expands placeholders **in-process** before `exec` / preview, in two spaces:

| Space | Where | Syntax | Meets a shell |
|-------|-------|--------|---------------|
| godo | matcher keys, `@deps` | `${NAME}` | never |
| shell | script bodies | `${godo:…}` | always |

In a body godo claims **only** `${godo:…}`. Every other `${…}` is shell text and
is passed through untouched — `${HOME}` and `$$` are the host
shell's, never a catalog bind. Collision is structurally impossible, so there is
no escape syntax.

| Body placeholder | |
|------------------|--|
| `${godo:argv[NAME]}` | Capture; `NAME` = `[A-Za-z_][A-Za-z0-9_]*` (fail-closed if unbound or invalid) |
| `${godo:args}` | Remaining tokens (space-joined) |
| `${godo:args[i]}` | One token; **error** if out of range |
| `${godo:args[i..j]}` | Half-open slice `[i,j)` (Go style) |
| `${godo:…:raw}` | Any of the above, interpolated verbatim (no quoting) |

`${godo:argv[i]}` with a number is an error pointing at `${godo:args[i]}`.
`${godo:…}` inside a matcher key or a `@deps` entry is rejected.

Expanded values are **shell-quoted** (`sh` rules on POSIX, `cmd` rules on
Windows): one argument in is one argument out. `:raw` opts a single placeholder
out.

A value shaped like `--flag=…` is quoted from the `=` onward, so a preview
reads `--am='two words'` rather than `'--am=two words'`. Same single argument
to the shell; the flag name is not part of the value.

Those rules also hold for bash, zsh, dash, ksh and fish. PowerShell quotes
differently, so a value containing a backtick or `$` may not survive there.

Windows caveat: `cmd.exe` expands `%VAR%` and `!VAR!` before a command sees its
arguments, and no quoting on the command line fully suppresses that.

## Exec cwd

Commands run with working directory = **directory of the resolved `godo.yaml`** (not necessarily the caller’s cwd). Relative paths in the catalog stay stable from subdirs.

Trust: expanded lines pass through the host shell (`sh -c` / `cmd /C`). Catalog
text is trusted — it is repo code. Values substituted into it are quoted, so
arguments and captures are data, not shell syntax; `${godo:…:raw}` waives that for one
placeholder and puts the trust decision back on the catalog author.

## Script decorators (JSDoc style)

YAML comment block **immediately above** the script key. Apply to scripts only.

| Line | |
|------|--|
| `# text` (no `@`) | Doc |
| `# @dialect <name>` | Match this script with another dialect (override of `{file}.dialect`) |
| `# @runner <name>` | Run this script with another runner (override of `{file}.runner`) |
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
test ${MODULE}: go test ./${godo:argv[MODULE]}/...
```

- `@dialect` — per-script override; without it, uses `{file}.dialect`
- `@runner` — per-script override; without it, uses `{file}.runner`
- `@deps` / `@dependencies` — invocation like `godo …` (space-separated tokens; entries separated by `,`)
- literals and bare `${NAME}` from captures **already bound** by the match (godo space)
- no `${godo:…}` in `@deps`; entries resolve to tokens, so a capture holding a space stays one token
- order = list order; stop on first failure
- each step keeps **its own** runner: a dep declaring `@runner` runs under that runner, not the caller's
- cycle → error (on the **expanded** invocation)
- caller args are **not** forwarded to deps
- diamond (A→B,C and B→C): C runs **once**, at its first (deepest-first) position
- dedup keys on the **expanded** invocation, so `lint pay` and `lint auth` are distinct nodes
- `--preview`: deps + body, in order (deps already expanded)
- no `@` → no deps (manual composition via `godo …` in the value remains valid)

## Dialect `package`

**Literal** keys.

```
scripts.<name>: string | string[]
```

```yaml
version: "0.1"
engine:
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
- without `${godo:args}` → extra tokens error (see **Runner**)
- no captures in keys (that is `matcher`)

## Dialect `matcher`

Keys = routes over tokens (Express-style). First match in (definition order).  
`${NAME}` = capture; the user chooses the name. Consume it in the body as `${godo:argv[NAME]}`.

```
scripts.<pattern>: string | string[]
```

```yaml
version: "0.1"
engine:
  dialect: matcher

scripts:
  "${GRP} ${SCR}": go run -C scripts/${GRP}/${SCR} . ${godo:args}
  # @deps lint ${MODULE}
  test ${MODULE}: go test ./${godo:argv[MODULE]}/...
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
2. Read `{file}.version` and `{file}.engine` (omit `engine.dialect` → `package`, must be implemented; omit `engine.runner` → `inherit`).
3. Match → bind captures → expand `@deps` → run deps → expand value → execute with the step's runner (or `--preview` / `--ls`).
4. Exit code = of the command (or the first failure in a list / deps); catalog/match errors → `1`.
5. Exec cwd = directory of the `godo.yaml` (see **Exec cwd**).

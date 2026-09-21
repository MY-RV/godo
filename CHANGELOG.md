# Changelog

## [Unreleased]

Breaking. The shell that runs your scripts changed.

### Changed
- **`dialect` moved to `engine.dialect`.** A top-level `dialect:` keeps
  working — it shipped in 0.1 and 0.2 — and `engine.dialect` wins if both are
  present. `runner` only ever existed as `engine.runner`.
- **The default runner is the shell you are already in**, not `sh -c` /
  `cmd /C`. A zsh user gets zsh, a PowerShell user gets PowerShell. The shell
  someone uses is their own business; godo proxying to a different one was a
  choice that was not godo's to make.
  *Consequence, and it is deliberate:* a catalog is read by the shell of
  whoever runs it, so zsh syntax behaves differently for a teammate on bash.
  godo is a proxy and promises neither cross-OS nor cross-shell. To pin one
  shell for everyone, name it (`runner: sh`, `runner: bash`).
  *Library:* an unset runner means the `Runner` injected into the engine, so
  `NewEngine(cat, myRunner)` is unaffected.

### Added
- **Runner axis.** `{file}.runner` and `# @runner` say *how* a script's body
  becomes a process, the way `dialect` says *which* script answers the tokens.
  It exists so a plugin's body — which is not a shell line at all — has a way
  to say so; see the [design note](./docs/dev/runners-and-plugins.md).
- **`inherit`** (the default): the shell you are in. `GODO_SHELL` overrides
  detection; otherwise `$SHELL` on Unix, and on Windows the parent process when
  it is a shell, else `%ComSpec%`. Windows reads the parent because the
  environment cannot answer — PowerShell sets `PSModulePath` and everything it
  starts inherits it, so a `cmd.exe` opened from PowerShell would look like
  PowerShell.
- **A shell by name:** `# @runner sh` / `bash` / `zsh` / `dash` / `ksh` / `ash`
  / `fish` / `nu` / `cmd` / `pwsh` / `powershell`. godo does not manage these —
  it resolves the name on `PATH` and hands the line over. The name is logical,
  never a path: `cmd`, not `cmd.exe`; `pwsh`, not `ps1`. A name outside the
  list is refused rather than run, so `# @runner git` cannot quietly become
  `git -c <line>`.
- **`engine:` block** — every dial godo turns while reading and running a
  catalog, kept apart from `scripts:`, which is the data. Holds `version`
  (minimum binary, enforced before anything runs), `dialect`, `runner`, and
  `plugins`. `engine.plugins` takes `source`, a **required** `sha256`,
  `provides: [runner:name]`, and an optional `config` that is entirely the
  plugin's — godo carries it without reading it. Nothing loads plugins yet;
  they are parsed and validated so the shape is settled, and a script asking
  for a runner a plugin provides fails by naming that plugin instead of reading
  as a typo.
- **`${godo:file(path)}` as a whole script value** puts the body in a file, so
  a Python or shell script gets an editor that understands it. Inclusion rather
  than expansion: it happens when the body is read, works for every runner, and
  leaves `${godo:…}` inside the file alone.
- **`godo -e plugins install <source>`** fetches an artifact, computes its
  digest, stores it under `<user cache>/godo/plugins`, and writes the entry into
  `godo.yaml` — preserving the comments, blank lines and block scalars around
  it. Without a source it fetches everything the catalog declares.
  `godo -e plugins` lists what is declared and whether it is here.
- **`godo -e runners`** lists what is usable on the machine you are on, and how
  to confirm which shell you are in when the detected one looks wrong.
- `--ls <tokens>` prints `@runner` beside `@dialect`.
- `ArgsAwareRunner`: whether a script takes the tokens left over after the
  match is the runner's question. A shell keeps the `${godo:args…}` rule,
  message included; a runner whose body is a program answers for itself.
- Public `RunnerName`, `RunnerInherit`, `RunnerRegistry`, `NewRunnerRegistry`,
  `DefaultRunners`, `EffectiveRunner`, `WithRunners`, `ErrUnknownRunner`.

### Fixed
- A value shaped like `--flag=…` is quoted from the `=` onward, so `--preview`
  shows `git commit --am='two words'` instead of `git commit '--am=two words'`,
  which read as though the flag name were part of the message. Identical single
  argument to the shell — rendering only.

### Notes
- An unknown runner fails when the plan is built (`--preview` included), not at
  load: a dialect must resolve before a script can be matched at all, a runner
  only to execute. Nothing executes either way.
- A shell is started non-interactively and without a profile, so you get your
  shell's grammar, not your shell's setup — aliases and functions are not there.
- The Windows detection cross-compiles and vets but is unverified on a real
  Windows host.

## [0.2.0] — 2026-09-19

Breaking. Placeholder syntax inside script bodies changed; catalogs need editing.
Full rules in [docs/contract.md](./docs/contract.md).

### Changed
- **Bodies are shell space; godo claims only `${godo:…}` there.** A matcher
  capture is still *declared* `${NAME}` in the key and written `${NAME}` in
  `@deps`, but consumed `${godo:argv[NAME]}` in the body. Every other `${…}` in
  a body is passed to the host shell untouched.
  *Migration:* `${NAME}` in a body must become `${godo:argv[NAME]}`. A body left
  unedited does not error — the shell receives the braces and expands them to
  nothing. There is no automated check for this yet.
- **Substituted values are shell-quoted.** One argument in is one argument out,
  whatever it contains; shell metacharacters in a value are data, not syntax.
  Previously values were interpolated verbatim into the `sh -c` / `cmd /C` line.
  Ordinary tokens (flags, paths) stay unquoted so previews remain readable.
  *Migration:* a script relying on an argument carrying a glob or a shell
  operator needs `:raw`.
- **`@deps` forms a DAG.** A shared dependency runs once, at its first
  (deepest-first) position, instead of once per path. Keyed on the expanded
  invocation, so the same script reached with different captures still runs once
  per capture set.
- **`@deps` entries resolve to tokens** instead of to a line that is then
  re-split, so a capture holding a space no longer fragments the invocation.
- `Dialect.Match` takes a single `Script` instead of a slice.

### Added
- `${godo:argv[NAME]}` — consume a matcher capture in a body.
- `${godo:…:raw}` — opt one placeholder out of quoting.

### Removed
- The `$${…}` escape and the "unknown capture" failure for bare `${…}` in a
  body. Both existed only to rescue host environment variables from being read
  as captures.

### Fixed
- A value shaped like `--flag=…` is quoted from the `=` onward, so `--preview`
  shows `git commit --am='two words'` instead of `git commit '--am=two words'`,
  which read as though the flag name were part of the message. Identical single
  argument to the shell — rendering only.

### Notes
- The Windows quoting path is **untested**: CI runs Linux only, and the runner
  tests skip on Windows. `cmd.exe` also expands `%VAR%` before a command sees
  its arguments, which no command-line quoting fully suppresses — a value
  containing `%` is not safe on Windows.

## [0.1.0] — 2026-09-13

### Added
- Public facade (`pkg.go`) over `internal/catalog` engine.
- CLI: bare-token scripts from `godo.yaml` + `-e`/`--engine` builtins.
- Dialects `package` / `matcher`, deps, fail-closed expand.
- Self-update client for GitHub Releases (`-e update`).
- MIT LICENSE, CI, GoReleaser (archive + bare binaries), `scripts/release-local.sh`.
- Contract e2e tests and expand fuzz.
- Product docs (overview, getting started, guides, reference, distribution, roadmap).
- Origin story (EN/ES), SECURITY.md, CONTRIBUTING, GitHub issue/PR templates, CODEOWNERS.

### Fixed
- A value shaped like `--flag=…` is quoted from the `=` onward, so `--preview`
  shows `git commit --am='two words'` instead of `git commit '--am=two words'`,
  which read as though the flag name were part of the message. Identical single
  argument to the shell — rendering only.

### Notes
- Product display name: **GoDo**; identifiers remain lowercase `godo`.
- Pre-1.0: APIs and CLI may still change. Treat `v0.x` as evolving.

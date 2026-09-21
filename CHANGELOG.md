# Changelog

## [Unreleased]

## [0.3.0-preview.1] — 2026-09-20

**A preview.** It is a GitHub pre-release, so `godo -e update` does not offer
it and neither Homebrew nor Scoop will hand it to you — download it from
[Releases](https://github.com/MY-RV/godo/releases), or
`go install github.com/my-rv/godo/cmd/godo@v0.3.0-preview.1`. On a preview
binary `godo -e update check` reports no update, because the newest *release*
is still 0.2.0.

It is a preview because plugin loading is new and has been run on macOS and
Linux only. Everything below is what 0.3.0 will promise; the preview is
where it gets found out.

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
  plugin's — godo carries it across and reads only `fs.mount`, which says which
  directories the sandbox can see.
- **Plugins run.** A plugin is a WebAssembly (WASI) program; godo runs it with
  [wazero](https://wazero.io), which is pure Go, so the binary you already have
  is the whole runtime — no cgo, no toolchain, nothing to install. The artifact
  is checked against its `sha256` **before** it is compiled, so what runs is
  what was reviewed. A script picks one by name (`# @runner micropy`) and godo
  hands it the body; the plugin asks godo back for what it cannot do itself,
  over four ops — `exec`, `out`, `slink` and `fetch`. Its only view of the disk
  is the directories `config.fs.mount` names, the catalog's own directory by
  default. `--preview` prints the body and never starts the plugin.
  The first one is [godo-micropy](https://github.com/MY-RV/godo-micropy):
  MicroPython, so a script can be Python on every machine godo runs on.
- **`${godo:file(path)}` as a whole script value** puts the body in a file, so
  a Python or shell script gets an editor that understands it. Inclusion rather
  than expansion: it happens when the body is read, works for every runner, and
  leaves `${godo:…}` inside the file alone.
- **A `fetch` op**: plugins can make HTTP requests through godo, which does
  them with Go's client — the same on every platform godo ships to. Without it
  the only route to the network is exec'ing `curl`, which is the platform
  dependency a plugin exists to remove. Bodies travel base64 because they are
  bytes, and requests time out after 30 seconds.
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
- **A `go install …@v0.3.0` binary reports the version it was installed at.**
  Nothing links our `-ldflags` on that path, so the version stayed at its
  `0.1.0-dev` default — cosmetic until `engine.version` arrived, and then
  enough to make a catalog refuse a binary that actually satisfied it. The tag
  now comes from Go's build info. A build from a working tree still says
  `0.1.0-dev`: Go describes it with a pseudo-version, and that is not a release
  anyone made.
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

### Notes
- Product display name: **GoDo**; identifiers remain lowercase `godo`.
- Pre-1.0: APIs and CLI may still change. Treat `v0.x` as evolving.

# Changelog

## [Unreleased]

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

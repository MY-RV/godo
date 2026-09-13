# Changelog

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

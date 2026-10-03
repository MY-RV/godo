# Announce draft — godo v0.3.0

Not product docs. Copy into the GitHub Release body when tagging `v0.3.0`.
Delete or move to `archive/` after the release ships.

---

## godo v0.3.0

Stable. The preview line (`preview.1` / `preview.2`) is promoted. Homebrew,
Scoop and `godo -e update` carry this release once the tag is cut.

### Breaking

The shell that runs your scripts is **the one you are in**, not `sh -c` /
`cmd /C`. A zsh user gets zsh; a PowerShell user gets PowerShell. Pin a shell
for everyone with `engine.runner` or `# @runner bash`.

Catalog dials move under `engine:` (`version`, `dialect`, `runner`, `plugins`).
A top-level `dialect:` from 0.1 / 0.2 still works; `engine.dialect` wins if both
are present.

### Plugins

A script body that is not a shell line can still run: declare a WASM plugin
under `engine.plugins`, install it, name it with `# @runner`.

- Digest-pinned (`sha256` required); `godo -e plugins install` computes it
- Runtime is [wazero](https://wazero.io) — pure Go, nothing else to install
- `--preview` prints the body and never starts the plugin

How-to: [Plugins](../guide/plugins.md).  
Worked example: [`examples/plugins/lines`](../../examples/plugins/lines).  
Interpreter out of tree: [godo-micropy](https://github.com/MY-RV/godo-micropy).

`package` and `matcher` stay **built in**. Plugins extend runners, not matching.

### Also in 0.3

- `godo -e runners` — what is usable here, and how the default was chosen
- Windows: process-tree shell detection, CRLF-safe catalogs, `fs.slink` as
  junction / hard link (verified on a real Windows host during the preview)

Full notes: [CHANGELOG](../../CHANGELOG.md).

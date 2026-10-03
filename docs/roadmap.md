# Roadmap — promises

This is what we **commit to communicate**. Pre-1.0 APIs can still change within a minor.

## v0.1 (first public release) — shipped

| Promise | |
|---------|--|
| CLI model | Bare tokens = scripts; builtins behind `-e` / flag aliases |
| Dialects | `package` and `matcher` work per [contract.md](./contract.md) |
| Fail-closed expand | Bad placeholders / OOB args error |
| Docs | Contract + getting started + install without Go |
| Artifacts | GitHub Release binaries (archive + bare) + checksums |
| License | MIT |
| Self-update | `-e update` / `--update` against those Releases |

**Not promised in v0.1:** Homebrew/Scoop installs, dialects `nscript`/`matchns`, stable Go API.

## v0.3 — ready as `v0.3.0`

Breaking: the shell that runs your scripts changes, and file fields move into
`engine:`. It was held until plugin loading worked — `engine:` without a loader
is half a promise, and this is the release where the promise gets made.

Previewed as `v0.3.0-preview.1` / `preview.2`. Those stayed GitHub pre-releases
(`godo -e update`, Homebrew and Scoop never offered them). After the stable
tag, those channels carry `v0.3.0`.

| Promise | |
|---------|--|
| Runner axis | `engine.runner` / `# @runner` pick how a body becomes a process |
| Default runner | The shell **you are in**, not `sh` / `cmd` |
| Shell by name | `sh`, `bash`, `zsh`, `dash`, `ksh`, `ash`, `fish`, `nu`, `cmd`, `pwsh`, `powershell` |
| `engine:` block | `version`, `dialect`, `runner`, `plugins` — what godo needs, apart from what the scripts are |
| `engine.version` | Minimum binary, enforced before anything runs |
| `godo -e runners` | What is usable here, and how to check which shell you are in |
| Plugin loading | WASM via `wazero`, digest-pinned — see [plugins](./guide/plugins.md) |
| `godo -e plugins` | Declare, install and list plugins; the digest is computed, never typed |
| Compatibility | Top-level `dialect:` keeps working |

### Landed

Everything in the table above. Windows shell detection, CRLF catalogs and
`fs.slink` (junction / hard link) were verified on a real Windows host during
the preview. Plugin install + run is covered by e2e on macOS/Linux (and the
example plugin under `examples/plugins/lines`).

### Still true after v0.3.0

- The plugin **protocol** (`api: 1`) is not a forever freeze. A plugin is pinned
  by digest, so a protocol change cannot silently break a catalog — it fails by
  naming the plugin.
- `provides: dialect:…` is accepted in the catalog shape and not loaded.
  `package` and `matcher` stay built in.
- Dialects `nscript` / `matchns` stay reserved and unplanned.

## Post-v0.3 — intended (not promised dates)

- Shared family packaging continues: `MY-RV/homebrew-tap`, `MY-RV/scoop-bucket`
- Optional: winget (`MY-RV.Godo`), later choco / AUR / Nix as demand appears
- Dialect plugins (load `provides: dialect:…`), only if a real dialect needs it
- Engine command registry polish; more e2e
- Dialects backlog only if explicitly promoted here

## v1.0 — future promise

- SemVer-stable **facade** Go API for documented symbols
- Frozen CLI + `godo.yaml` contract (breaking ⇒ major / new module path)

# Roadmap — promises

This is what we **commit to communicate**. Pre-1.0 APIs can still change within a minor.

## v0.1 (first public release) — promised

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

## v0.3 — previewed as `v0.3.0-preview.2`

Breaking: the shell that runs your scripts changes, and file fields move into
`engine:`. It was held until plugin loading worked — `engine:` without a loader
is half a promise, and this is the release where the promise gets made.

A preview is a GitHub pre-release: `godo -e update` does not offer it, and
neither Homebrew nor Scoop carries it. It is on
[Releases](https://github.com/MY-RV/godo/releases) and on
`go install github.com/my-rv/godo/cmd/godo@v0.3.0-preview.2`.

| Promise | |
|---------|--|
| Runner axis | `engine.runner` / `# @runner` pick how a body becomes a process |
| Default runner | The shell **you are in**, not `sh` / `cmd` |
| Shell by name | `sh`, `bash`, `zsh`, `dash`, `ksh`, `ash`, `fish`, `nu`, `cmd`, `pwsh`, `powershell` |
| `engine:` block | `version`, `dialect`, `runner`, `plugins` — what godo needs, apart from what the scripts are |
| `engine.version` | Minimum binary, enforced before anything runs |
| `godo -e runners` | What is usable here, and how to check which shell you are in |
| Plugin loading | WASM via `wazero`, digest-pinned — see [runners and plugins](./dev/runners-and-plugins.md) |
| `godo -e plugins` | Declare, install and list plugins; the digest is computed, never typed |
| Compatibility | Top-level `dialect:` keeps working |

### Landed

Everything in the table above.

### Before v0.3.0 ships

- Preview feedback. Plugin loading is new, and the preview is where it gets
  found out. preview.1 on Windows already produced three fixes.
- **Windows verification.** preview.2 fixes shell detection, CRLF catalogs and
  `fs.slink`; the junction syscall behind `slink` has still been run by nobody.
- The plugin protocol is **not** frozen by this preview. A plugin is pinned by
  digest, so a protocol change cannot silently break a catalog — it fails by
  naming the plugin.

**Not promised in v0.3:** Windows shell detection is written from the
documented behavior of those shells; it compiles and vets for `windows/amd64`
but is unverified on a real Windows host. `GODO_SHELL` overrides it.

## Post-v0.1 — intended (not promised dates)

- Shared family packaging: `MY-RV/homebrew-tap` (`brew install --cask MY-RV/tap/godo`), `MY-RV/scoop-bucket`
- Optional: winget (`MY-RV.Godo`), later choco / AUR / Nix as demand appears
- Engine command registry polish; more e2e
- Dialects backlog only if explicitly promoted here

## v1.0 — future promise

- SemVer-stable **facade** Go API for documented symbols
- Frozen CLI + `godo.yaml` contract (breaking ⇒ major / new module path)

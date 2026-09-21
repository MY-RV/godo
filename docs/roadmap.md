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

## v0.3 — in progress, unreleased

Breaking: the shell that runs your scripts changes, and file fields move into
`engine:`. **Held until plugin loading works** — `engine:` without a loader is
half a promise, and this release is where the promise gets made.

| Promise | |
|---------|--|
| Runner axis | `engine.runner` / `# @runner` pick how a body becomes a process |
| Default runner | The shell **you are in**, not `sh` / `cmd` |
| Shell by name | `sh`, `bash`, `zsh`, `dash`, `ksh`, `ash`, `fish`, `nu`, `cmd`, `pwsh`, `powershell` |
| `engine:` block | `version`, `dialect`, `runner`, `plugins` — what godo needs, apart from what the scripts are |
| `engine.version` | Minimum binary, enforced before anything runs |
| `godo -e runners` | What is usable here, and how to check which shell you are in |
| Compatibility | Top-level `dialect:` keeps working |

### Landed

Everything in the table above.

### Still required before v0.3 ships

- **Plugin loading.** `engine.plugins` parses and validates; nothing reads the
  artifact yet. WASM via `wazero`, digest-pinned — see
  [runners and plugins](./dev/runners-and-plugins.md).

**Not promised in v0.3:** Windows shell detection is written from the
documented behavior of those shells; it compiles and vets for `windows/amd64`
but is unverified on a real Windows host. `GODO_SHELL` overrides it.

## Post-v0.1 — intended (not promised dates)

- Shared family packaging: `MY-RV/homebrew-tap` (`brew install --cask MY-RV/tap/godo`), `MY-RV/scoop-bucket`
- Optional: winget (`MY-RV.Godo`), later choco / AUR / Nix as demand appears
- Engine command registry polish; more e2e
- Dialects backlog only if explicitly promoted here
- Plugin loading (WASM via wazero, digest-pinned) — see
  [runners and plugins](./dev/runners-and-plugins.md)

## v1.0 — future promise

- SemVer-stable **facade** Go API for documented symbols
- Frozen CLI + `godo.yaml` contract (breaking ⇒ major / new module path)

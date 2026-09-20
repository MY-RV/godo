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

## Post-v0.1 — intended (not promised dates)

- Shared family packaging: `MY-RV/homebrew-tap` (`brew install --cask MY-RV/tap/godo`), `MY-RV/scoop-bucket`
- Optional: winget (`MY-RV.Godo`), later choco / AUR / Nix as demand appears
- Engine command registry polish; more e2e
- Dialects backlog only if explicitly promoted here

## v1.0 — future promise

- SemVer-stable **facade** Go API for documented symbols
- Frozen CLI + `godo.yaml` contract (breaking ⇒ major / new module path)

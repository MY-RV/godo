# Distribution

Package / binary name is always **`godo`**. Module import path stays `github.com/my-rv/godo` (Go is case-sensitive). GitHub owner display is **MY-RV** (same account).

## v0.1 (now)

| Channel | How |
|---------|-----|
| GitHub Releases | Download bare binary or archive — [install.md](./install.md) |
| Go | `go install github.com/my-rv/godo/cmd/godo@v0.1.0` |
| Homebrew | `brew install MY-RV/tap/godo` ([tap](https://github.com/MY-RV/homebrew-tap)) |
| Scoop | `scoop bucket add my-rv https://github.com/MY-RV/scoop-bucket` then `scoop install godo` |
| Self-update | `godo -e update` / `godo --update-check` |

## Family packaging

Shared across godo, hensu, and future CLIs:

| Store | Home | Install |
|-------|------|---------|
| Homebrew | [MY-RV/homebrew-tap](https://github.com/MY-RV/homebrew-tap) | `brew install MY-RV/tap/godo` |
| Scoop | [MY-RV/scoop-bucket](https://github.com/MY-RV/scoop-bucket) | `scoop bucket add my-rv https://github.com/MY-RV/scoop-bucket` then `scoop install godo` |

Fully-qualified Homebrew avoids clashing with unrelated taps that also expose a `godo` formula.

## Later / optional

| Channel | Notes |
|---------|--------|
| winget | Package id like `MY-RV.Godo`; command remains `godo` |
| Chocolatey / AUR / Nix / deb-rpm | Only if needed |
| Homebrew core / Scoop Main | Long-term; requires external review |

There is no universal `{store} install godo` binary — each manager has its own CLI — but the **product id** stays `godo`.

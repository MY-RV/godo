# Install

## Without Go (recommended for most users)

1. Open [Releases](https://github.com/MY-RV/godo/releases)
2. Download for your OS/arch:
   - bare: `godo_<ver>_darwin_arm64` (same layout as `scripts/release-local.sh`)
   - or archive: `godo_<ver>_darwin_arm64.tar.gz` / Windows `.zip`
3. Verify checksums (`checksums.txt` / `SHA256SUMS`)
4. `chmod +x godo` and place on `$PATH`
5. `godo --version`

Update later:

```bash
godo --update-check
godo --update
# or: godo -e update check
```

Override Releases API (tests/mirrors): `GODO_RELEASES_API`.

## With Go

```bash
go install github.com/my-rv/godo/cmd/godo@latest
# pin:
go install github.com/my-rv/godo/cmd/godo@v0.2.0
```

Local build:

```bash
go build -ldflags "-X github.com/my-rv/godo.Version=v0.2.0" -o godo ./cmd/godo
```

## Package managers

```bash
brew install --cask MY-RV/tap/godo
```

Homebrew ships pre-built binaries as a **cask**, and casks are macOS-only. On Linux use Go or the release binary.

If you previously installed the Formula (`brew install MY-RV/tap/godo`), uninstall it first: `brew uninstall godo`, then install the cask.

```powershell
scoop bucket add my-rv https://github.com/MY-RV/scoop-bucket
scoop install godo
```

On recent Homebrew, third-party taps may need `brew trust MY-RV/tap` once before install.

See [distribution.md](./distribution.md).

## Local release artifacts (no upload)

```bash
./scripts/release-local.sh
# or: ./godo release-local
```

Produces `dist/` + checksums with bare binaries.

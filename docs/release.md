# Release

## Versioning

| What | Where |
|------|--------|
| Binary / module | `godo.Version` (ldflags `-X github.com/my-rv/godo.Version=…`), read through `godo.Release()` |
| Catalog file | `version:` inside `godo.yaml` |
| Git tag | `vX.Y.Z` |

See [dev/versioning.md](./dev/versioning.md).

## Local (no upload)

```bash
./scripts/release-local.sh
```

Output: `dist/godo_<ver>_<os>_<arch>` + `SHA256SUMS`.

## GitHub

1. `./godo ci` green on `main` (bootstrap: `go build -o godo ./cmd/godo`)
2. Tag `vX.Y.Z` and push the tag
3. Workflow **release** runs GoReleaser → archives **and** bare binaries + checksums
4. Users: download, `go install …@vX.Y.Z`, or `godo -e update`

## Preview (pre-release)

Tag `vX.Y.Z-preview.N`. The suffix is the whole mechanism:

- GoReleaser marks the GitHub release **pre-release** (`release.prerelease: auto`)
- the Homebrew cask and the Scoop manifest are **not** updated — `skip_upload`
  is false only for a tag with no pre-release part
- `godo -e update` reads `/releases/latest`, which GitHub answers with the newest
  *release*, so a preview is never offered to anyone

So a preview reaches only the people who go and get it, which is the point of
one. Everything else is the same as above.

## After first release

Optional packaging homes (family): `MY-RV/homebrew-tap`, `MY-RV/scoop-bucket` — [distribution.md](./distribution.md).

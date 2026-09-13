# Release

## Versioning

| What | Where |
|------|--------|
| Binary / module | `godo.Version` (ldflags `-X github.com/my-rv/godo.Version=…`) |
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

## After first release

Optional packaging homes (family): `MY-RV/homebrew-tap`, `MY-RV/scoop-bucket` — [distribution.md](./distribution.md).

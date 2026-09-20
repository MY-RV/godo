# Getting started

## 1. Install without Go

Once [GitHub Releases](https://github.com/MY-RV/godo/releases) exist:

1. Download the asset for your OS/arch, e.g. `godo_0.1.0_darwin_arm64` (bare) or the `.tar.gz` / `.zip`
2. `chmod +x godo` (Unix)
3. Move it onto your `$PATH`
4. Check: `godo --version`

Full detail: [install.md](./install.md).

With Go available:

```bash
go install github.com/my-rv/godo/cmd/godo@latest
```

## 2. Add a catalog

Create `godo.yaml` at the repo root (2-space indent):

```yaml
version: "0.1"

scripts:
  test: go test ./...
  vet: go vet ./...
  # @deps vet, test
  check: go build -o app .
```

`version` is required. `engine.dialect` is optional and defaults to `package`. Script docs and `@deps` / `@dialect` are comments immediately above each key — start with [guide/scripts.md](./guide/scripts.md).

## 3. Run

```bash
godo test
godo --preview check   # see expanded lines, no exec
godo --ls              # list scripts
```

## 4. Next

- [Scripts](./guide/scripts.md)
- [Package dialect](./guide/package-dialect.md)
- [Deps](./guide/deps.md)
- Normative rules: [contract.md](./contract.md)

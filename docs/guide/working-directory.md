# Working directory

Commands run with cwd = the directory that contains the resolved `godo.yaml`, not your shell’s cwd.

## Walk-up

```text
repo/
  godo.yaml          ← catalog
  scripts/seed.go
packages/api/
  $ pwd
```

```bash
cd packages/api
godo --preview seed
```

If `repo/godo.yaml` is the nearest catalog and contains `seed: go run ./scripts/seed.go`, godo still runs that line with cwd `repo/`.

**Effect:** `./scripts/seed.go` resolves under `repo/`, not under `packages/api/`.

## Relative paths in bodies

```yaml
# repo/godo.yaml
scripts:
  seed: go run ./scripts/seed.go
```

```bash
# from anywhere under repo/
godo seed
```

**Runs** (from `repo/`):

```text
go run ./scripts/seed.go
```

## Next

- [Scripts](./scripts.md)
- [Library: Load and Run](./library-load-run.md)

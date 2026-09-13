# Engine commands

Built-ins live behind `-e` / `--engine`. They never compete with script names.

## Version

```bash
godo -e version
# same:
godo --version
```

**Prints** the binary version string (one line), e.g.:

```text
0.1.0-dev
```

## Help

```bash
godo --engine help
```

**Prints** engine help to stderr (built-in subcommands).

## Update

| Command | Behavior |
|---------|----------|
| `godo -e update` | Download newer GitHub Release binary into this executable |
| `godo -e update check` | Report only (`--update-check` alias) |

```bash
godo --update-check
```

**Example output** (already current):

```text
current: 0.1.0-dev
latest:  v0.1.0
already up to date
```

## Scripts keep their names

```yaml
scripts:
  update: echo from-script
```

```bash
godo --preview update
```

**Prints** (catalog script, not engine):

```text
echo from-script
```

```bash
godo -e update
```

Runs the **engine** self-update path (network / download), not `echo from-script`.

## Next

- [Scripts](./scripts.md)
- [Preview and ls](./preview-and-ls.md)
- [CLI reference](../reference/cli.md)

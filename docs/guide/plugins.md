# Plugins

A **plugin** is how a script body that is not a shell line still runs under
`godo`. It is a WebAssembly (WASI) program; godo loads it with
[wazero](https://wazero.io) — pure Go, so the binary you already have is the
whole runtime.

A dialect decides *which* script answers your tokens. A runner decides *how*
that body becomes a process. Plugins extend the **runner** axis. `package` and
`matcher` stay built in; matching the catalog is not a plugin's job.

## Declare, install, name

```yaml
version: "0.1"
engine:
  plugins:
    - source: ./lines.wasm
      sha256: "661471…"          # godo -e plugins install writes this
      provides: [runner:lines]
      config:
        proc: {exec: true}

scripts:
  # @runner lines
  boot: |
    git status
    ?pnpm install
```

```bash
godo -e plugins install          # fetch everything the catalog declares
godo --preview boot              # prints the body; never starts the plugin
godo boot                        # loads the wasm, runs the body
```

| | |
|--|--|
| `source` | `https://…` URL, or a path relative to the `godo.yaml` |
| `sha256` | Required. The bytes that run are the bytes that were reviewed |
| `provides` | What the plugin registers. Today only `runner:<name>` is loaded |
| `config` | The plugin's own block. godo carries it across and reads `fs.mount` |

`godo -e plugins install <source>` computes the digest from the artifact and
writes the entry into `godo.yaml`. The digest is never typed by hand — that is
how wrong hashes get committed.

A file sitting beside the catalog needs no install; its digest is checked
either way. Anything remote must be installed first. A run that finds it
missing says so and stops:

```
godo: plugin https://… is not installed
  run: godo -e plugins install
```

`http://` sources are refused. An artifact is code; its integrity cannot rest
on a transport anyone on the path can rewrite.

## What the plugin sees

godo writes one JSON request on the plugin's stdin (body, captures, leftover
args, config), then answers ops the plugin writes on stdout — `exec`, `out`,
`slink`, `fetch`. The plugin's exit code is the script's.

`--preview` prints the body and **does not start the plugin**. A plugin body is
a program; the only faithful answer without running it is the program itself.

Wire detail for authors: [plugin protocol](../dev/plugin-protocol.md).

## Worked example

[`examples/plugins/lines`](../../examples/plugins/lines) — body is one command
per line; `?` lines may fail without stopping the rest. Index:
[`examples/plugins`](../../examples/plugins).

```bash
GOOS=wasip1 GOARCH=wasm go build -o lines.wasm ./examples/plugins/lines
# put lines.wasm next to a godo.yaml, then:
godo -e plugins install ./lines.wasm
```

A real interpreter plugin lives out of tree:
[godo-micropy](https://github.com/MY-RV/godo-micropy) (MicroPython).

## Next

- [Runners](./runners.md)
- [Matcher dialect](./matcher-dialect.md) — captures become the plugin's `argv`
- [Engine](./engine.md) — `-e plugins`
- Design premise: [runners and plugins](../dev/runners-and-plugins.md)

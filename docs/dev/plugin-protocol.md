# Plugin protocol

How godo talks to a plugin. Not the contract — [`contract.md`](../contract.md)
covers the `godo.yaml` side; this is the wire.

## What a plugin is

A **WASI command**: a normal program with a `main()`, compiled to wasm.

```
godo → plugin stdin    one JSON request
godo ← plugin stdout   JSON ops, one per line
godo → plugin stdin    one JSON result per op that needs one
plugin exits           its exit code is the script's
```

Being a command rather than a set of exported functions keeps the contract
small: there is no memory-sharing ABI to get right, any language that targets
WASI can write one, and a plugin can be tested as an ordinary program —

```bash
echo '{"api":1,"mode":"preview","body":"echo hi","argv":{},"args":[]}' | ./plugin
```

## The sandbox

wazero's default, which is nothing: no filesystem, no network, no environment,
no clock. A plugin reaches the outside world only by asking godo, and godo only
honours what `engine.plugins[].config` grants.

## Request

One line, written once, before anything else.

```json
{
  "api": 1,
  "mode": "run",
  "runner": "lines",
  "body": "git status\n?pnpm install",
  "argv": {"BRANCH": "wt/example"},
  "args": ["--no-install"],
  "config": {"proc": {"exec": true}}
}
```

| Field | |
|-------|--|
| `api` | Protocol version. A plugin that speaks another major should exit non-zero rather than guess |
| `mode` | `"run"` or `"preview"` |
| `runner` | The name the catalog asked for, so one plugin can provide several |
| `body` | The script body, **verbatim**. `${godo:…}` is not expanded — that is shell-space syntax, and the values are on this request already |
| `argv` | The matcher's captures |
| `args` | Tokens left over after the match |
| `config` | The plugin's own block, verbatim |

A plugin must read the request before writing anything. The pipes are
unbuffered, so a plugin that spoke first would deadlock.

## Ops

| Op | Answered | Needs |
|----|----------|-------|
| `exec` | yes | `config.proc.exec` |
| `emit` | **no** | nothing |

```json
{"op":"exec","argv":["git","status"],"dir":"","capture":false}
{"op":"emit","line":"git status"}
```

`emit` is one-way on purpose: a plugin that waited for an answer would hang.

### Result

```json
{"code":0,"ok":true,"stdout":"","stderr":"","error":""}
```

`error` is godo refusing — an unknown op, or a capability the catalog did not
grant. A command that ran and failed is `code`, not `error`.

## Capabilities

Deny by default. An absent section, an absent key, or anything that is not
exactly `true`, is not granted:

```yaml
config:
  proc:
    exec: true
```

```
lines: line 1: not granted: proc.exec — enable it under
engine.plugins[].config.proc.exec
```

`config` is otherwise the plugin's: its keys, its meaning, its defaults. godo
reads only what it gates on.

## Preview

The plugin's to answer. godo can render a shell line because it wrote it; it
cannot render a program it does not interpret, so it asks with `mode:
"preview"` and prints whatever `emit` lines come back.

Which means a plugin **can** run things during a preview, because it is the one
holding the capability. A plugin that does is misbehaving, the same way a
`--dry-run` that writes is misbehaving. Grant `proc.exec` to plugins you trust
to tell the difference.

## Exit codes

The plugin's exit code is the script's. A plugin propagating a child's code
gets godo exiting with that code, like a shell would.

## Worked example

[`examples/plugins/lines`](../../examples/plugins/lines) — a runner whose body
is one command per line. Under 200 lines of ordinary Go, no wasm-specific code.

```bash
GOOS=wasip1 GOARCH=wasm go build -o lines.wasm ./examples/plugins/lines
shasum -a 256 lines.wasm
```

```yaml
engine:
  plugins:
    - source: ./lines.wasm
      sha256: "…"
      provides: [runner:lines]
      config:
        proc: {exec: true}

scripts:
  # @runner lines
  boot: |
    git status
    ?pnpm install
```

## Known limits

- **Local paths only.** `source` is a file on this machine. Fetching belongs
  with a lockfile and is not built.
- **Size.** A Go plugin carries Go's runtime — the example is ~4.5 MB. TinyGo
  or a C-family language produces far smaller wasm.
- **One instantiation per step.** Fine at godo's scale; it is not a server.
- **`exec` only.** No spawn, no filesystem ops. Those are the next capabilities
  and the reason `config` is shaped as sections.

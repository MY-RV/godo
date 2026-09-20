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

`--preview` never starts a plugin. It prints the body.

Being a command rather than a set of exported functions keeps the contract
small: there is no memory-sharing ABI to get right, any language that targets
WASI can write one, and a plugin can be tested as an ordinary program —

```bash
echo '{"api":1,"mode":"preview","body":"echo hi","argv":{},"args":[]}' | ./plugin
```

## The sandbox

It starts shut, and nothing here disables anything — nothing is granted. With
an empty `config`, measured from inside a plugin:

```
os.listdir(".")     OSError [Errno 44] ENOENT
open("/etc/passwd") OSError [Errno 44] ENOENT
os.getenv("HOME")   None
time.time()         1640995200.0      (frozen)
import socket       the module is not there
```

`engine.plugins[].config` adds back exactly what it names, and nothing else.

| Grant | Gives |
|-------|-------|
| `proc.exec` | the `exec` op |
| `fs.mount` | the catalog's own directory, as the guest's root |
| `time.wall` | the real clock instead of a frozen one |

`fs.mount` is a bool, not a path: what gets mounted is the directory the
`godo.yaml` lives in, never somewhere the file names. A catalog that chose its
own mount point could ask for `/`, and the grant would mean nothing.

## Request

One line, written once, before anything else.

```json
{
  "api": 1,
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

```json
{"op":"exec","argv":["git","status"],"dir":"","capture":false}
```

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

`godo --preview` prints the body and **does not start the plugin**. Nothing is
compiled, nothing is instantiated, no grant is needed.

A plugin body is a program. The only faithful answer to "what will this do"
without running it is the program itself; anything else is a guess. A guess is
useful, but it deserves its own flag rather than quietly borrowing this one —
that is a later `--predict`, which will add back a mode field and an output op.
Plugins ignore fields they do not know, so nothing written today breaks then.

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

## Licensing

A plugin is a separate artifact from a separate repository, and godo does not
ship one. So godo's own licensing is unaffected by what a plugin contains —
but the repository that *does* ship one carries whatever is inside it.

A repository distributing a `.wasm` is distributing everything compiled into
it. Whatever the interpreter or runtime inside it is licensed under, its notice
travels with the artifact. Check the build's own license inventory rather than
assuming, and ship the notices next to the `.wasm`, not only in the source
tree — the artifact is what people download.

godo's own third-party notices are in
[THIRD_PARTY_LICENSES.md](../../THIRD_PARTY_LICENSES.md) and ship inside the
release archives.

## Known limits

- **Local paths only.** `source` is a file on this machine. Fetching belongs
  with a lockfile and is not built.
- **Size.** A Go plugin carries Go's runtime — the example is ~4.5 MB. TinyGo
  or a C-family language produces far smaller wasm.
- **One instantiation per step.** Fine at godo's scale; it is not a server.
- **`exec` only.** No spawn. Filesystem access is the mount, not an op.
  `config` is shaped as sections so more can be added without moving anything.

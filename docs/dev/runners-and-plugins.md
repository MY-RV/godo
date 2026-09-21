# Runners and plugins — design note

Not the contract. [`contract.md`](../contract.md) is what we promise; nothing
here is in [`roadmap.md`](../roadmap.md).

## Premise

**godo is a proxy.** Same idea as `package.json` scripts: it names the chores
and hands the line to the shell. It does **not** promise to solve cross-OS. If
you are on Linux you write Linux (`&&`); on Windows you write Windows. Your
shell is your responsibility, and a catalog author who wants portability picks
portable commands.

The first layer is that proxy plus flavored water: dialects, `@deps`,
`--preview`, `--ls`. Native, harmless, no ambition beyond naming things well.

**Cross-OS is a plugin's job**, not the core's. A plugin like `micropy` —
MicroPython compiled to WASM — carries a body that runs the same everywhere
because it is not a shell line at all.

## What follows from that

Two axes, and only one of them was open:

| Axis | Question | |
|------|----------|--|
| Dialect | which script answers these tokens? | already open |
| Runner | how does the body become a process? | hardcoded to `sh -c` / `cmd /C` |

A MicroPython body has no shell that can run it. For `micropy` to exist at all,
godo needs a way for a script to say *this one is not a shell line*. That is
the whole reason the runner axis exists — `runner:` / `# @runner` and a registry,
with `shell` as the default.

It is a socket, not a feature. What was added alongside it is not portability
work either: `shell` hardcoded `sh` / `cmd`, so a zsh or PowerShell user ran
their catalog under a shell they did not pick. `inherit` — now the default — uses
the one they are in, and a catalog can name a shell outright (`# @runner
bash`). Your shell is your responsibility; godo just stops lying about which
one it is, and proxies to whatever `PATH` has.

A `shell` runner meaning "always `sh` / `cmd`" existed briefly and was removed:
it was a third thing between "your shell" and "this shell", it kept
`internal/execshell` carrying two runners, and almost nobody writes a line that
means the same in `sh` and in `cmd`. `# @runner sh` says it better.

## Deliberately not done

Things an earlier draft of this note argued for, dropped because they
contradict the premise:

- **A built-in `exec` runner** (argv, no shell). Its only real gain was a
  Windows `%VAR%` edge case; the quoting added in v0.2.0 already covers the
  rest. And it moves the body from the shell's ownership to godo's, which is
  how it ended up needing its own quote parser — a mini-shell inside godo, with
  rules that are neither `sh`'s nor PowerShell's. Wrong direction.
- **A built-in POSIX interpreter** (`mvdan.cc/sh`). Same reason, plus a
  dependency, and it is cross-OS work that belongs in a plugin.
- **A structured `Line` / `Part` expansion.** Only existed to serve `exec`.

The reserved dialect names `nscript` / `matchns` stay reserved and unplanned.

## Where a plugin is declared

`engine:` — kept apart from the script fields on purpose. `dialect:` and
`runner:` say how to read and run the scripts; `engine:` says what godo itself
needs to do it. That line is what lets the toolchain side grow — a lockfile, a
package manager — without the script side growing with it.

The block parses and validates today; nothing loads from it. `sha256` is
required from the start rather than added later, because a plugin is
third-party code that runs on `godo test` and a digest is the only thing that
says it is the code that was reviewed.

## What is actually left for plugins

None of this is built:

1. Load a `.wasm` at runtime (`wazero`: pure Go, no cgo, one artifact for every
   platform, sandboxed by default).
2. Ship MicroPython as that `.wasm`, with the host API from the prototype
   (`godo.argv`, `godo.args`, `godo.proc`, `godo.fs`).
3. Fetch and pin it: `sha256` required, no auto-update.

Two constraints worth keeping when that work starts:

- **What is pinned is which plugin, not what it may do.** A catalog's scripts
  already run with the shell's full reach, and a plugin body is a script in
  that same catalog. The `sha256` answers the question that has an answer:
  these are the bytes that were reviewed.
- **A guest reaches the outside only by asking**, because wasm has no network,
  no `fork` and no `subprocess`. That is the platform, not a policy godo
  enforces on a script its own author wrote.

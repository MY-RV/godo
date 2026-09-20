# Runners

A **dialect** decides *which* script answers your tokens. A **runner** decides
*how* that script's body becomes a process. They are independent — any dialect
pairs with any runner.

```yaml
version: "0.1"
engine:
  runner: inherit      # omit → inherit

scripts:
  test: go test ./...

  # @runner bash
  ci: shopt -s globstar && echo **/*.go
```

| Runner | |
|--------|--|
| `inherit` | The shell you are already in. **Default** |
| a shell name | `sh`, `bash`, `zsh`, `dash`, `ksh`, `ash`, `fish`, `nu`, `cmd`, `pwsh`, `powershell` |

godo does not manage these shells. It resolves the name on `PATH` and hands the
line over — the line is yours, the shell is yours, godo is the proxy.

## `inherit` — your shell

The default. Whatever shell you are in runs the line:

```yaml
scripts:
  arr: "arr=(a b c); echo ${arr[1]}"
```

```
$ godo arr        # from zsh
a
$ godo arr        # from bash
b
```

Not a bug: zsh indexes arrays from 1, bash from 0. Same line, two shells, two
answers — and that is the point. Before this, godo always used `sh`, so a zsh
user got `b` without being told why.

**Consequence, and it is deliberate:** a catalog is read by the shell of
whoever runs it. A script written in zsh syntax behaves differently for a
teammate on bash. godo is a proxy and promises neither cross-OS nor
cross-shell. If that matters for a script, name a shell.

## Naming a shell

Pins it for everyone, whatever shell they are in:

```yaml
scripts:
  # @runner bash
  ci: shopt -s globstar && echo **/*.go

  # @runner pwsh
  sign: Get-AuthenticodeSignature .\dist\godo.exe
```

The name is **logical, never a path**. Write `cmd`, not `cmd.exe`; `pwsh`, not
`pwsh.exe` or `ps1` — `.ps1` is a script extension, the program is `pwsh`. The
platform's extension is `PATH`'s business, so the same `godo.yaml` reads the
same on every machine.

A name outside the list is refused rather than run:

```
$ godo deploy
godo: script "deploy" asks for runner "git": unknown runner: "git" is not a
known shell (sh, bash, zsh, …); for another one set GODO_SHELL
```

Otherwise `# @runner git` would quietly become `git -c <line>`.

A shell that is not installed fails when the plan is built, so nothing runs.

## What godo picked, and how to disagree

```
$ godo -e runners
inherit    /bin/zsh  [default]

shells found here:
  sh         /bin/sh
  bash       /bin/bash
  zsh        /bin/zsh

Not the shell you expected? Run this in your terminal:
  echo $0            (sh, bash, zsh, dash, ksh)
  echo $version      (fish)

Then: GODO_SHELL=/path/to/shell
```

Selection order:

| | |
|--|--|
| `GODO_SHELL` | Always wins |
| Unix | `$SHELL`, else `/bin/sh` |
| Windows | the parent process when it is a shell, else `%ComSpec%` |

Windows reads the parent process because the environment cannot answer:
PowerShell sets `PSModulePath` and everything it starts inherits it, so a
`cmd.exe` opened from PowerShell would look like PowerShell.

On Unix, `$SHELL` is your *login* shell, not necessarily the one running right
now — bash started inside zsh still reports zsh. Every other tool follows that
convention; `GODO_SHELL` is how you disagree with it.

## What you do not get

Your shell's **grammar**, not your shell's **setup**. The shell starts
non-interactively and without a profile (`-c`, `/C`, `-NoProfile -Command`), so
your aliases and functions are not there — exactly as `sh -c` always behaved.

```yaml
scripts:
  # your alias "gs" does not exist here
  status: git status
```

## Plugins

`# @runner` is also how a body that is **not a shell line** says so — a plugin
carrying its own interpreter. Nothing ships yet; see
[runners and plugins](../dev/runners-and-plugins.md).

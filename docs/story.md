# Why GoDo exists

The repo should carry a shared list of what you can run. Same names, same meaning, for anyone who lands in the tree.

In Go, Java, C#, Rust, Python, and the rest, there often is no thin file that says “these are the named chores for this repo.” What you get instead: README scraps, `scripts/` folders, IDE buttons, CI YAML, long one-liners you paste when you remember them. Not wrong. Partial maps. Fine when you already know the place. Harder when someone new needs one spot: here is `check`, here is `seed`, here is what runs before the shell.

If you work with JavaScript day to day, you already know a good version of that idea. `package.json` has `scripts`. That field is a catalog: named chores in the repo, easy to launch. A lot of teams never need more. Yes — that ecosystem got there early. The clear referent, if you already use it.

Even there, something else still drifts: not so much the names, but which binary opens the project — `npm`, `pnpm`, `yarn`, or `bun`. Mix them and lockfiles fight. That is why `packageManager` and Corepack exist: pin the runner so a clone does not guess. Many teams already do that. Drift is habit, not destiny.

Personal shortcuts tried to fill the gap. Day to day with **Naoki** on [Rosvelt](https://rosvelt.com/), long entrypoints hurt: the language is not friendly to repo scripts, and the work crosses languages. `nkd` was a `.zshrc` helper for that. The machine died and took it with it — local, gone. Later, a private [MY-RV](https://github.com/MY-RV) `myrv` router could dispatch long forms from one laptop. Useful on one machine. Not a catalog everyone versions in the repo. If something like that happened to you, you already know the gap. Another private alias will not close it. Something the repo can carry will.

So **GoDo**. One `godo.yaml`. List it, preview it, run it:

```bash
godo --ls
godo --preview check
godo check
```

Commands can be anything — `go test`, `dotnet`, `cargo`, a shell line. The point is the shared names.

It is also a public bet. [goxdi](https://github.com/MY-RV/goxdi) was already out there; the idea was not to leave it alone — ship something serious others can use, not only drafts on a laptop.

Practical, not a sermon: if a JS repo already has solid `scripts` and a pinned package manager, you may not need another catalog. When the tree spans languages, or the real entrypoints are long forms nobody wants to retype, a small shared file helps. Prefer preview before trust.

[Getting started](./getting-started.md) · [Español](./story.es.md)

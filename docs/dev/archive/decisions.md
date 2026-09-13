# Decisions — godo

What was kept, what was rejected, and why. Not the contract; see [contract.md](../../contract.md).

## Keep

| Decision | Why |
|----------|-----|
| Own product (`godo`), own contract | The repo catalog is ours; other runners are not our public surface |
| YAML + dialects | Readable format; dialects grow without muddying the simple model |
| Dialect `package`: literal keys, value `string \| string[]`, optional `${godo:args}`/slices | package.json-style KISS + minimal arg power (option **C**) |
| Dialect `matcher`: patterns + first match in; free-named captures | Express power over argv; `${GRP}` or `${ARG0}` is the user’s choice |
| Future names `nscript`, `matchns` | Extend toward nkfile without reopening `package`/`matcher` |
| In-process bind (godo expansion), not host shell | Transparency + same template on Windows/Unix |
| Flags `--help` / `--ls` / `--preview` | Help, list, and see the expanded command; no subcommands |
| `--ls` with no tokens = all; with tokens = match(es) | One flag, two modes; no `list` subcommand |
| `--ls _ _` recommended for two-capture patterns | Any match works; `_ _` is a clear convention |
| No npm-style `--` separator between script and args | Args = trailing tokens; `${godo:args}` marks the hole |
| `{file}.version` YAML file field | Versioned file contract; future dialects do not break blindly |
| `{file}.dialect` YAML file field | Dialect belongs to the file, not the script |
| Script docs + meta = JSDoc-style comments | No `desc` / `deps` field; decorators on scripts only |
| `@deps` canonical; `@dependencies` alias | Short like JSDoc; alias for clarity |
| Walk-up for the file | Usable from subdirs |
| `godo X` inside the value | Composition always possible without a declared graph |
| `@deps` declares the graph; does not forward args | Clean preflight; each dep is its own match |
| `@deps` may use match captures (`lint ${MODULE}`) | Same bind as the value; without that the graph is useless in `matcher` |
| Cycle over the expanded invocation | `test payments` → `lint payments` is the real edge |

## Reject

| Decision | Why |
|----------|-----|
| Adopt an external recipe-file tool as *the* contract | Lock-in; we want our own minimal catalog |
| “Rich” schema (`deps:` / `env` / silent / default / dotenv / objects per script) | Too much; deps live in `@deps`, not in the YAML value |
| `godo run <script>` | Horrible |
| `desc` field | Comments |
| Godo reorders by specificity | First match in; the user orders |
| npm-style `--` as script-arg separator | Noise; `${godo:args}` is enough |
| Bind via host-shell `$VAR` / `%VAR%` | Not portable; less transparent than already-expanded argv |
| `package` as oneliner only with no args (**A**) | Incomplete; args are useful without routing |
| Day-0 routing inside `package` (**B**) | Turns `package` into `matcher` |
| Dialect name `route` | Preferred: `matcher` |
| Watch, includes, template engines, rich param systems | Out of scope |
| Parallelism / optional deps / pre-post | Out; linear `@deps` is enough |
| `${godo:args}` / slices inside `@deps` | Deps = closed invocations; rest args are not pushed into the graph |
| One doc mixing contract + rationale | Clean contract; why lives here |

## Accepted risk

| Risk | |
|------|--|
| Formatters / YAML round-trips that strip comments | The author picks tooling that does not overwrite them; godo does not invent a second docs channel |

## Inspiration (shape only)

List, walk-up, multiline, deps — only what is under **Keep** enters (deps via `@deps`, not a rich schema).

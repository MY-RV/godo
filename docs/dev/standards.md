# Standards — GoDo

Short rules. If it is not here, prefer the smallest change that matches existing code.

## Naming

- Product (humans / titles): **GoDo**.
- Identifiers (code, CLI, paths): lowercase `godo` — binary, `github.com/my-rv/godo`, `godo.yaml`, `${godo:args…}`.
- Do not rename the module or binary to `GoDo` / `go-do`.

## Language / module

- Go, module `github.com/my-rv/godo`.
- `cmd/godo/main.go` is entrypoint only — no business logic.
- Public import is `github.com/my-rv/godo` (`pkg.go` facade).
- Engine lives in `internal/catalog`; CLI/expand/exec under `internal/`.
- Do not import `internal/catalog` from outside this module.

## Behavior

- Contract in `docs/contract.md` wins over implementation guesses.
- In-process expansion only (no host `$VAR` bind).
- Fail closed on bad captures / unexpected args / unknown decorators.
- Commands run with cwd = directory of the resolved `godo.yaml`.
- Expanded lines go through the host shell — treat catalog + args as trusted input.

## Tests

- Table or focused cases for errors (`ErrNoMatch`, cycle, OOB args, parse rejects).
- Do not only test the happy path.
- Dogfood: keep `godo.yaml` scripts (`test`, `vet`, `check`, `ci`) green — project gate is `./godo ci`, not Make.

## Docs

- Product docs: `docs/`. Contributor/internals: `docs/dev/`.
- README stays short; link out.
- No feature in code without a contract line if it changes CLI or file semantics.

## Style

- No new dependency without a clear need (today: `yaml.v3` + stdlib).
- Avoid pattern theater: a registry/strategy is fine; extra layers are not.

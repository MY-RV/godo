# Preview and list

Flags go **before** script tokens. They never execute shell commands.

```bash
godo --preview check    # OK
godo check --preview    # --preview is NOT a flag here
```

---

## List all scripts

```yaml
version: "0.1"

scripts:
  # run unit tests
  test: go test ./...
  vet: go vet ./...
  # local gate
  # @deps vet, test
  check: go build -o app .
```

```bash
godo --ls
```

**Prints** name + doc (if any):

```text
test  # run unit tests
vet
check  # local gate
```

---

## Inspect one match

With tokens, `--ls` resolves the same match as a real run, then prints the **definition** (not expanded).

```bash
godo --ls check
```

**Prints:**

```text
local gate
@deps vet
@deps test
check:
  go build -o app .
```

```bash
godo --ls test
```

**Prints:**

```text
run unit tests
test:
  go test ./...
```

No match → error:

```bash
godo --ls no-such
```

```text
godo: no script matching tokens: [no-such]
```

---

## Probe a matcher key

`--ls` needs tokens that **fit the pattern**, including any literal prefix.

```yaml
scripts:
  # @dialect matcher
  "run ${GRP} ${SCR}": go run ./scripts/${godo:argv[GRP]}/${godo:argv[SCR]} ${godo:args}
```

```bash
godo --ls run _ _
```

`_` is just a token that fills `${GRP}` and `${SCR}`. Any two tokens work (`godo --ls run api seed`).

**Prints** the matched key (placeholders stay literal — this is inspect, not expand):

```text
@dialect matcher
run ${GRP} ${SCR}:
  go run ./scripts/${godo:argv[GRP]}/${godo:argv[SCR]} ${godo:args}
```

Wrong arity fails (two tokens do not match `run ${GRP} ${SCR}`):

```bash
godo --ls _ _
```

```text
godo: no script matching tokens: [_ _]
```

---

## Preview (expand, do not run)

`--preview` prints every shell line that would run, **after** `@deps` and `${godo:…}` expansion, one line per command.

```bash
godo --preview check
```

**Prints:**

```text
go vet ./...
go test ./...
go build -o app .
```

Matcher + captures + leftover args:

```bash
godo --preview run db migrate --dry
```

**Prints:**

```text
go run ./scripts/db/migrate --dry
```

Unknown placeholder fails closed (same as a real run):

```bash
godo --preview bad   # body: echo ${godo:argv[nope]}
```

```text
godo: unknown capture "nope" (not bound by the matcher key)
```

## Next

- [Deps](./deps.md)
- [Placeholders](./placeholders.md)
- [Matcher dialect](./matcher-dialect.md)
- [CLI reference](../reference/cli.md)

# Reference — CLI

Normative source: [contract.md](../contract.md).

## Flags

| Flag | |
|------|--|
| `--help` | CLI help |
| `--version` | ≡ `-e version` |
| `--ls [tokens…]` | List scripts / matches |
| `--preview [tokens…]` | Expand only |
| `--update` | ≡ `-e update` |
| `--update-check` | ≡ `-e update check` |
| `-e, --engine <cmd>` | Engine command |

## Engine commands

| | |
|--|--|
| `-e runners` | List runners usable on this machine |
| `-e plugins` | What the catalog declares, and whether it is installed |
| `-e plugins install [source]` | Fetch declared plugins, or add and fetch one |
| `-e version` | Print binary version |
| `-e update` | Install newer Release asset |
| `-e update check` | Check only |
| `-e help` | Engine help |

## Exit behavior

Non-zero on catalog/load/expand/run errors. See contract for details.

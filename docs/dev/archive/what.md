# What a repo command catalog is

An **executable index** of “how to do X in this repo”: named chores in one file, runnable with a short CLI, instead of README folklore + loose scripts + long one-liners.

```
godo.yaml (or similar)
        │
        ▼
$ godo test
```

## We vs “they” (generic)

| | **We (godo)** | **Other recipe runners** |
|--|---------------|---------------------------|
| Role | Own minimal catalog + contract | Existing community tools |
| File | `godo.yaml` | Various DSLs / YAML recipes |
| Stance | Ship our contract; do not adopt theirs as ours | Useful inspiration for list / walk-up / deps shape |

Inspiration taken only where it matches **Keep** in [decisions.md](./decisions.md) (list, walk-up, multiline, deps via `@deps`).

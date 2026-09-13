# Problem

## Without a shared catalog (the gap)

In a repo without a versioned command catalog:

- “how do I run tests” lives in the README, someone’s head, or unindexed `scripts/*.sh`
- each person invents the entrypoint (`go test`, `npm test`, different path)
- CI copies other commands → **local ≠ pipeline**
- no listable index of named chores

That is the gap a thin repo catalog is meant to cover: **named commands, checked in, discoverable**.

## The decision this brainstorm must close

1. **Do we ship our own file-based catalog?** (almost certainly yes)
2. **What is the minimal contract?** (stable names, dialects, expand rules)
3. **Which recipes are the repo gate?** (e.g. test / check / ci)

## Non-goals

- Binary startup microbenchmarks
- Replacing GitHub Actions
- Defining every monorepo recipe on day one

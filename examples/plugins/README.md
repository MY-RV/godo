# Example plugins

Worked examples of the godo plugin protocol. Product how-to:
[docs/guide/plugins.md](../../docs/guide/plugins.md). Wire:
[docs/dev/plugin-protocol.md](../../docs/dev/plugin-protocol.md).

| | |
|--|--|
| [`lines`](./lines) | Runner whose body is one command per line |

```bash
GOOS=wasip1 GOARCH=wasm go build -o lines.wasm ./examples/plugins/lines
```

Put `lines.wasm` next to a `godo.yaml`, then `godo -e plugins install ./lines.wasm`.
A real interpreter lives out of tree:
[godo-micropy](https://github.com/MY-RV/godo-micropy).

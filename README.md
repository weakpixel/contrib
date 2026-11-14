# Weakpixel contrib

Contributions to the [Weakpixel Engine](https://github.com/weakpixel/engine): optional add-ons that the engine does not depend on. Each contribution is a folder with its own Go module, docs and tests.

| Folder | Module | What |
| --- | --- | --- |
| [lua](lua/README.md) | `github.com/weakpixel/contrib/lua` | Lua (GopherLua) systems driven by the Game Object schemas, and LuaLS type stubs |

```bash
make all        # every contribution: generate, fmt, vet, test
make wasm-test  # every contribution under GOOS=js GOARCH=wasm (Node)
make lua        # one contribution
```

A contribution is tagged by folder (`lua/v0.0.1`). Licensed under the [MIT license](LICENSE); third-party notices are in each contribution's folder.

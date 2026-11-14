# contrib

Contributions to the [Weakpixel engine](https://github.com/weakpixel/engine): optional modules that build on it but do not belong in the engine. Every contribution is a folder with its own `go.mod`, its own README, docs and tests, so a game depends only on the ones it uses.

| Folder | Module | What |
| --- | --- | --- |
| [lua](lua/README.md) | `github.com/weakpixel/contrib/lua` | Lua (GopherLua) systems driven by registered Game Object schemas, and LuaLS stubs for editors |

```bash
make all         # every module: generate, fmt, vet, test -race
make wasm-test   # every module under GOOS=js GOARCH=wasm (Node)
```

Modules are tagged per folder (`lua/v0.0.1`). The first tag is pushed by the maintainer.

## Adding a contribution

Create a folder with its own `go.mod` (module `github.com/weakpixel/contrib/<name>`), add it to `MODULES` in the root `Makefile`, and list it in the table above. Each module keeps its documentation in its own folder.

## License

MIT, see [LICENSE](LICENSE). Third-party notices of a module are in its `THIRD_PARTY_NOTICES.md`.

# lua

Lua (GopherLua) systems for the [Weakpixel Engine](https://github.com/weakpixel/engine), driven by registered Game Object schemas. Module `github.com/weakpixel/contrib/lua`, package `ecslua`. It uses the `god` package of the weakpixel engine, the [ECS](https://github.com/weakpixel/ecs) and [GopherLua](https://github.com/yuin/gopher-lua); the weakpixel engine does not know about it.

```bash
go get github.com/weakpixel/contrib/lua
```

```go
import ecslua "github.com/weakpixel/contrib/lua"
```

A Lua system is a `system.Runner`, so it plugs into the weakpixel engine's stages with `AddRunner`. How to write the script and the Go side: [Getting started](doc/guide/getting-started.md).

- **Using it:** [Lua guide](doc/guide/README.md).
- **How it is built:** [Lua binding design](doc/design.md).
- **Editor support:** `go tool ecslua stubs` writes a LuaLS definition file from GOD files ([Editor support](doc/guide/editor-support.md)).
- **Example:** `go run ./example` (a Game Object, a Lua system, a Go system and a serialized GOD).

```bash
make all     # generate, fmt, vet, test -race
make bench   # per-entity cost of a Lua system
make stubs   # LuaLS stubs of the example in build/example.lua
make wasm-test
```

The weakpixel engine repository is private for now: `GOPRIVATE=github.com/weakpixel/engine` (the Makefile exports it).

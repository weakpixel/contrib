# lua

Lua (GopherLua) systems for the Weakpixel Engine, driven by registered Game Object schemas. Module `github.com/weakpixel/contrib/lua`, package `ecslua`; it requires the engine module `github.com/weakpixel/engine` (package `god` only).

```sh
go get github.com/weakpixel/contrib/lua
```

A Lua system is a `system.Runner`; add it to a `game.Engine` with `AddRunner`:

```go
import ecslua "github.com/weakpixel/contrib/lua"

rt := ecslua.NewRuntime(engine.World(), ecslua.Options{})
defer rt.Close()
engine.AddRunner(game.StageSimulate, rt.NewSystem("scripts/regen.lua"))
```

- **Using it:** [Lua guide](doc/guide/README.md).
- **How it is built:** [Lua binding design](doc/design.md).
- **Example:** `go run ./example` (a Game Object, a Lua system, a Go system and a serialized GOD).
- **Editor support:** `go run github.com/weakpixel/contrib/lua/cmd/ecslua stubs -o god.lua <GOD files>` writes LuaLS type stubs ([editor support](doc/guide/editor-support.md)).

```bash
make all      # generate, fmt, vet, test
make bench    # per-entity cost of a Lua system
make stubs    # LuaLS stubs of the example under build/god-doc/
make god-doc  # optional HTML and Markdown docs under build/god-doc/
```

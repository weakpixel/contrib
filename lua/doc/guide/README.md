# Lua scripting guide

Lua systems for the Weakpixel Engine. Module `github.com/weakpixel/contrib/lua` (package `ecslua`), on [GopherLua](https://github.com/yuin/gopher-lua) (pure Go, Lua 5.1). It is an optional layer on top of [`system`](https://github.com/weakpixel/ecs/blob/main/doc/api-systems.md) and the Game Objects of [`god`](https://github.com/weakpixel/engine/blob/main/doc/guide/ecs/game-object-definition.md), in a separate module: the engine never depends on Lua, and `ecs`, `god` and `ecs/system` know nothing about it.

```go
import ecslua "github.com/weakpixel/contrib/lua"
```

A script names a registered Game Object (`group:class`) and gets its components by their GOD names and JSON field names, validated against the GOD `TypeSchema`. No generated bindings: everything is driven by the registered `god.Schema` at runtime.

| Page | Read it to learn |
| --- | --- |
| [getting-started.md](getting-started.md) | Your first Lua system in 5 minutes |
| [scripts.md](scripts.md) | The script contract, objects, components, field types, readonly, asset fields, pointer lifetime |
| [world-api.md](world-api.md) | The global `world` table: spawn, despawn, set, remove, get, dump |
| [runtime.md](runtime.md) | Go side: `Runtime`, `Options`, `fs.FS` scripts, hot reload, sandbox, errors, performance |
| [editor-support.md](editor-support.md) | LuaLS type stubs from `weakpixel doc` |

A complete, runnable example (a Game Object declared in Go, a Lua system, a Go system and a serialized GOD) is in [example](../../example): `go run ./example` in this module. How the binding works inside: [design.md](../design.md).

## When to use Lua

Use it for behavior and tuning (regeneration, AI rules, spawners, balancing) that you want to edit without recompiling. Keep inner loops (movement, collision) in Go: a Lua system costs about 2 µs and 21 allocations per entity, and is not part of the 0-alloc hot path ([Performance](runtime.md#performance)).

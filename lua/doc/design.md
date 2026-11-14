# Lua binding design

How this module exposes Game Objects to Lua without generated bindings. The script contract, `world` API and options are in the [Lua guide](guide/README.md).

## Shape

- **A separate module**, the only place gopher-lua is imported. It uses exported `ecs`, `ecs/system` and `god` API only; nothing in the root module knows about Lua.
- **One `Runtime` per World**: one sandboxed `lua.LState`, single-goroutine like the World.
- **A script is a `system.Runner`** (`*System`, also an `Initializer`). `Init` loads and checks the script, so a typo surfaces before the first tick, and the system plugs into any `Schedule`, including `game.Engine`'s stages.
- **Pure Go Lua (GopherLua, Lua 5.1)**: no cgo, so the module cross-compiles like the rest of the project.

## Schema-driven, reflective binding

A script names a registered object (`group:class`). The runtime looks up its `god.Schema` and builds, per component, a query over the object's required components and a metatable:

- **Field access by JSON name.** `newFieldMap` maps each JSON name of the component's `TypeSchema` to the Go field index (`encoding/json` rules), so Lua sees exactly the names of the serialized GOD.
- **Metatables are cached** per (Go type, schema, readonly), so building them is a one-time cost.
- **Userdata are re-pointed, not recreated.** The object and component userdata passed to `update` are reused for every entity: the system points them at the next entity's components. That makes them free per entity, and it is why they are valid only during one call.
- **Writes are validated against the `TypeSchema`** (integer range, type, nullability) before they reach Go memory; tables (arrays, maps, opaque `x-go-type` values) round-trip through JSON and `TypeSchema.Validate`, so Lua and the GOD loader accept the same values.
- **Readonly (value) components** reject writes, matching the Go view, which hands out copies.
- **Asset fields** read and write keys through the world's `god.AssetResolver`, so handles never appear in Lua.

There are no generated typed bindings. They would remove the reflection and boxing cost, at the price of a generator step per game; the reflective binding was chosen because scripts are for behavior and tuning, not inner loops.

## Structural changes

`world.spawn`, `despawn`, `set` and `remove` validate their arguments at the call (so errors carry the script line) and queue the change on the schedule's `Commands`, applied after the stage. Iteration therefore never sees a structural change, the same rule Go systems follow.

## Hot reload

Same contract as asset hot reload ([asset-system.md](https://github.com/weakpixel/engine/blob/main/doc/design/asset-system.md#hot-reload)), so the engine has one mental model:

- Polled on **simulated time** (`ctx.Time.Delta`), not wall-clock, so tests are deterministic and `WatchInterval == 0` makes no file-system calls.
- **Candidate swap**: the new version must pass every `Init` check before it replaces the running one; a failure is reported once per file version and the old version keeps running.
- **Fresh environment** per load: script state resets, entity data (in the World) does not.
- A check costs about two allocations (`os.Stat`), once per interval; `TestWatchAllocsBounded` guards it.

## Cost

Per entity: one Lua call, boxing of the arguments and of every number read (GopherLua allocates each `LNumber` it hands out), and a fresh entity handle because scripts may store it. Measured: about 2 µs and 21 allocations per entity for three reads and two writes (darwin/amd64, 2026-10-05). `TestRunAllocsBounded` fails if the per-entity allocation count grows past 32; `BenchmarkLuaSystem` measures the time.

## Keeping docs and code in sync

The `world` API exists in three places: `world-api.go`, the LuaLS stub that `ecslua stubs` writes (`internal/stubs`), and the table in [world-api.md](guide/world-api.md). `TestWorldStubMatchesDocAPI` in `internal/stubs` compares all three.

The stubs command reads GOD files with its own small loader instead of the engine's `weakpixel doc`, so the engine carries no Lua-specific output. The Lua example's GOD, merged project GOD, LuaLS stubs and optional HTML and Markdown docs are generated below the ignored `build/` directory, configured by the module's `.weakpixel.json`. The runtime does not read the GOD file.

## Not built

Parallel scripts, coroutines that span ticks (state machines), generated typed bindings, instruction or memory limits (GopherLua has none; `TickBudget` bounds wall-clock time only).

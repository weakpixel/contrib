# The `world` API

A global `world` table, available in every script. Structural changes are **queued** on `ctx.Commands` and apply after the stage ([Commands](https://github.com/weakpixel/ecs/blob/main/doc/api-systems.md#commands)). Arguments are validated **immediately**, so errors carry the script file and line:

```
scripts/spawn.lua:7: component game:Health: $.Max: expected number, got string
```

| Call | Effect |
| --- | --- |
| `world.spawn("g:C", {["g:Comp"] = {...}, ...})` | Validate, queue a spawn. The entity gets the object's `god.ObjectClass` tag. Returns nothing. |
| `world.despawn(e)` | Queue a despawn. |
| `world.set(e, "g:C", "g:Comp", {...})` | Validate, queue add-or-replace of one component. |
| `world.remove(e, "g:C", "g:Comp")` | Queue removal. Only **optional** components. |
| `world.get(e, "g:C")` | The object for `e`, or `nil` if it lacks a required component. Current call only. Allocates. |
| `world.dump(e, "g:C")` | Components of `e` as a Lua table keyed by component key, like a file (a copy of what `god.EncodeComponent` writes; entity references are entity handles). |

Fields of type `ecs.Entity` ([entity references](https://github.com/weakpixel/engine/blob/main/doc/guide/ecs/game-object-definition.md#entity-references)) read as entity handles, or `nil` for none, and accept a handle or `nil`, also inside `spawn` and `set` tables. A stale handle is an error.

A component is named by its key, `"group:name"` ([identity](https://github.com/weakpixel/engine/blob/main/doc/guide/ecs/game-object-definition.md#identity)), in every call and on objects (`hero["game:Health"]`). Scripts may also use the bare name (`"Health"`, `hero.Health`) when exactly one component of the object's class has that name; with two (say `a:Part` and `b:Part`) only the keys work and `hero.Part` is an unknown component. An object may also hold any other registered component ([extra components](https://github.com/weakpixel/engine/blob/main/doc/guide/ecs/game-object-definition.md#extra-components)): it is optional (`nil` when absent), reached by key, or by bare name when no class component and no other registered component has that name. Files always use the key. The object key is part of each call because the set of components, and so the bare names, belong to the object.

## Examples

```lua
-- spawn
world.spawn("game:Hero", {
  ["game:Health"] = { Current = 10, Max = 10 },
  Weapon = { Ammo = 30 },                   -- optional component, by its bare name
})

-- add / replace an optional component later
world.set(e, "game:Hero", "Weapon", { Ammo = 30 })

-- remove it
world.remove(e, "game:Hero", "Weapon")

-- look at another entity (e.g. one stored earlier)
local other = world.get(targetEntity, "game:Hero")
if other then other.Health.Current = other.Health.Current - 5 end

-- debug
print(world.dump(e, "game:Hero")["game:Health"].Current)
```

## Timing

`spawn`, `despawn`, `set` and `remove` do not take effect inside the current call: `world.get` right after `world.spawn` will not find the new entity, and a `set` is not visible to the next entity in the same stage. Writes through `hero.Health.Current = ...` are immediate.

## Errors

| Situation | Message |
| --- | --- |
| dead or stale entity | `stale entity` (instead of the `ecs:` panic) |
| removing a required component | `component g:C.g:Comp is required and cannot be removed` |
| a component given twice to `spawn` (key and bare name) | `component g:Comp is given twice` |
| unknown component | `unknown component g:C.<name>` |
| bad value | `component <g:Comp>: $.<path>: expected <type>, got <value>` |

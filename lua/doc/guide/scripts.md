# Scripts

A script file returns one table:

```lua
return {
  object = "game:Hero",                 -- required: registered "group:class"
  init   = function() end,              -- optional, runs once on load
  update = function(dt, e, hero) end,   -- required: once per matching entity per tick
}
```

| Argument | |
| --- | --- |
| `dt` | tick delta in seconds (number) |
| `e` | entity handle: opaque, comparable, **safe to store** |
| `hero` | the object: `hero["game:Health"]` is the component `game:Health`; `hero.Health` works too while no other component of the object is named `Health` ([names](world-api.md)). An optional component the entity lacks is `nil` |

Each script has its own environment: globals of one script are invisible to others.

## Which entities?

Every entity that has all the **required** components of `object`. A `game:Hero` script also sees entities spawned as `game:Boss` if they carry Hero's required components (same rule as the generated `HeroView`).

## Fields

Access fields by JSON name, as in the serialized GOD:

```lua
update = function(dt, e, hero)
  local h = hero.Health
  if h.Current < h.Max then h.Current = h.Current + 1 end
  if hero.Weapon ~= nil then hero.Weapon.Ammo = hero.Weapon.Ammo - 1 end
  hero.Hitbox.Width = 2   -- nested struct: writes through
end
```

| Go / schema kind | Read | Write |
| --- | --- | --- |
| `integer` (`int8`..`int64`, `uint`..) | number | integral and within `minimum`/`maximum`, else error |
| `number` (`float32`, `float64`) | number | number |
| `boolean`, `string` | same | same type, else error |
| nested `object` | userdata onto the field (no copy) | assign its fields |
| `array`, maps (`additionalProperties`), `x-go-type`, `x-preserve-unknown-fields` | Lua table, a **copy** (via JSON) | table, validated with `TypeSchema.Validate`, then decoded |
| pointer / `nullable` | `nil` when null | `nil` stores null |
| asset field (`format: "asset"`, see [GOD asset fields](https://github.com/weakpixel/engine/blob/main/doc/guide/ecs/game-object-definition.md#asset-fields)) | the key string, `""` if unset | a key string, resolved to a handle (`""` clears) |
| entity reference (`ecs.Entity`, `format: "entity"`, see [GOD entity references](https://github.com/weakpixel/engine/blob/main/doc/guide/ecs/game-object-definition.md#entity-references)) | an entity handle, `nil` for the zero `Entity` | an entity handle or `nil`; a stale handle is an error |
| `x-runtime` field | as its type | as its type |

Notes:

- Unknown fields raise `lua: unknown field <Comp>.<name>`.
- You cannot assign a whole component (`hero.Health = 1`): set its fields, or use [`world.set`](world-api.md).
- Lua numbers are float64: an `int64` above 2^53 loses precision on read.
- **Required nullable fields.** Lua cannot store `nil` in a table, so when a table is converted (`world.spawn`, `world.set`, or assigning a table to a field), a *required* field that the schema marks `nullable` and that is absent from the table is stored as `null`. `Companion = {Name = "Rex"}` is valid although `Attachment` is required.

## Readonly components

A value component (`Label Label`, not `*Label`) is read-only: writes, including to nested fields, raise `component Label is readonly`. This matches the generated view's `Get`, which hands out a copy.

## Asset fields

Handles never appear in Lua; you see asset keys (see [engine levels as Game Objects](https://github.com/weakpixel/engine/blob/main/doc/guide/engine/game-objects.md)).

```lua
return {
  object = "gfx:AnimatedVisual",
  update = function(dt, e, v)
    if v.Animator.Frame == 3 then                    -- runtime field: readable
      v.Sprite.Tint = 0xff8080ff
    end
    if v.Sprite.ID == "" then print("no sprite yet") end
    v.Animator.Clip = "chars/player.aseprite.json#run" -- resolved to a handle
  end,
}
```

A non-empty key needs a resolver on the world (`god.SetAssetResolver`, in the weakpixel engine `resolver.Install`). Without one, reading a set handle or writing a non-empty key raises `no god.AssetResolver set`. A bad key: `resolve <kind> asset "<key>": <error>`. `world.spawn`/`world.set` tables accept asset keys too, but may **not** contain `x-runtime` fields.

## Pointers are short-lived

`hero` and its components (and objects returned by `world.get`) are re-pointed at each entity and valid **only during that call**. Don't keep them in a global or table. Keep `e` and call `world.get(e, "game:Hero")` later. This is the same rule as for `ecs` pointers ([Component access](https://github.com/weakpixel/ecs/blob/main/doc/api-low-level.md#component-access)).

```lua
local target                       -- script-level state, reset on hot reload
update = function(dt, e, hero)
  target = e                       -- fine
  -- target = hero.Health          -- WRONG: dangling after this call
end
```

## Script-level state

Locals and globals persist between ticks, but are reset on [hot reload](runtime.md#hot-reload). Keep durable state in components.

## Errors

Every error goes to `Options.OnError` with the script position (`scripts/x.lua:7: ...`); the failing call is abandoned and the tick continues with the next entity. The default `OnError` panics. Messages are listed in [runtime.md](runtime.md#errors).

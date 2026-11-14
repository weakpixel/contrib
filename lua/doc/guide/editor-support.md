# Editor support

`go tool ecslua stubs` turns the GOD files of a project into one [LuaLS](https://luals.github.io/) definition file, so editors complete and check your scripts.

```sh
go tool ecslua stubs -o god.lua build/god    # a folder of GOD files, searched recursively
go tool ecslua stubs -o - game.god.json      # a single file, to stdout
# in this repository: make stubs   (writes build/example.lua)
```

Put the file in a directory listed in LuaLS's `workspace.library` (`.luarc.json`). It is a `---@meta` file: types only, never run.

Annotate your script to type `hero`:

```lua
---@type GodScript<game.Hero>
return {
  object = "game:Hero",
  update = function(dt, e, hero)
    hero.Health.Current = hero.Health.Current + 1  -- completed and checked
  end,
}
```

| Generated | Meaning |
| --- | --- |
| `GodObject` | alias of every object key; completes `object = "..."` and `world.*` |
| `god.<group>.<Name>` | a class per component type (`god.gfx.Transform`) with fields and doc comments; unique because component keys are. Nested structs are `god.<group>.<Name>.<Field>` |
| `<group>.<Class>` | a class per object with a field per component under its key (`["gfx:Transform"]`) and, when unique in the object, its bare name (`Transform`); optional components are `Name?` (`["g:Name"]` gets a nullable type), readonly and embedded ones are noted in the description. `world.set` and `world.remove` overloads accept both names too |
| `Entity`, `GodScript<T>` | the entity handle and the table a script returns |
| `world` | the [world API](world-api.md), one overload per object, so `world.get(e, "game:Hero")` is `game.Hero?` and `world.set(e, "game:Hero", "Weapon", {...})` checks the value |

Types: `integer`, `number`, `string`, `boolean`, `T[]`, `table<string, T>`, `T?` (nullable), `any` for `x-go-type` / `x-preserve-unknown-fields`. A required nullable field and an optional field both read `Name?`, because Lua cannot tell them apart.

Add the tool to your module once with `go get -tool github.com/weakpixel/contrib/lua/cmd/ecslua`. Options: `-o file` (default `build/god.lua`, `-` is stdout), `-title text`. Inputs are GOD files (`*.god.json`), directories (searched recursively) or patterns such as `./...`; the default is `build/god`, where `weakpixel gen` writes them. Equal definitions in several files collapse into one, different ones under the same key are an error. The GOD files come from [`weakpixel gen`](https://github.com/weakpixel/engine/blob/main/doc/guide/tools.md#gen); the older `weakpixel doc --format lua` is gone.

# Editor support

`ecslua stubs` turns the GOD files of a project into one [LuaLS](https://luals.github.io/) definition file, so editors complete and check your scripts.

```sh
go run github.com/weakpixel/contrib/lua/cmd/ecslua stubs -o god.lua build/god/   # the specs folder
# in this module: make stubs   (writes build/god-doc/example.lua for the example)
```

The inputs are GOD files or directories, searched recursively for `*.god.json` (skipping `testdata`, `vendor`, `node_modules` and directories starting with `.` or `_`). Definitions found more than once merge when they are equal apart from their docs; a different definition under the same key is an error. Options: `-o <file>` (default `-`, standard output) and `-title <name>` (the project name in the header, default `Game`).

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

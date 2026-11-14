# Getting started

## 1. Declare a Game Object (Go)

```go
// +game:object
type Hero struct {
	// +game:component
	Health *Health
	// +game:component
	Weapon *Weapon // optional: pointer
}

type Health struct{ Current, Max int64 }
```

Run `go generate` (`wpix gen`) so the object is registered. See [Game Object API](https://github.com/weakpixel/engine/blob/main/doc/guide/ecs/api-game-object.md). Pointer components are optional and writable. Value components are required and read-only for scripts.

## 2. Write a script

`scripts/regen.lua`:

```lua
return {
  object = "game:Hero",
  update = function(dt, e, hero)
    local h = hero.Health
    if h.Current < h.Max then
      h.Current = h.Current + 1
    end
  end,
}
```

## 3. Run it as a system

```go
rt := ecslua.NewRuntime(world, ecslua.Options{})
defer rt.Close()

var s system.Schedule
s.AddStage("simulate")
s.AddRunner(rt.NewSystem("scripts/regen.lua"))

s.Run(world, time.Second/60)
```

`NewSystem` loads and checks the script in `Init` (the first `Run`), so a typo in `object` or a missing `update` surfaces before the first tick.

## 4. Spawn from Lua

```lua
return {
  object = "game:Hero",
  update = function(dt, e, hero)
    if hero.Health.Current <= 0 then
      world.despawn(e)
      world.spawn("game:Hero", { Health = { Current = 10, Max = 10 } })
    end
  end,
}
```

Structural changes are queued and apply after the stage.

## 5. Ship scripts in the binary

```go
//go:embed scripts/*.lua
var scripts embed.FS

s.AddRunner(rt.NewSystemFS(scripts, "scripts/regen.lua"))
```

## Where next

- Field types and errors: [scripts.md](scripts.md)
- Everything `world` can do: [world-api.md](world-api.md)
- Editing scripts while the game runs: [runtime.md](runtime.md#hot-reload)

-- Heals every Drifter a little each tick. A Drifter that is spent (0 hp) is
-- replaced by a fresh one at the same place, with the same heading, to show
-- world.despawn and world.spawn. Not used by `go run ./example`; see
-- example_test.go for how to run it.
local HEAL_PER_SECOND = 2

---@type GodScript<example.Drifter>
return {
  object = "example:Drifter",

  update = function(dt, e, d)
    local hp = d.Health

    if hp.Current <= 0 then
      world.despawn(e)
      world.spawn("example:Drifter", {
        Position = { X = d.Position.X, Y = d.Position.Y },
        Velocity = { X = d.Velocity.X, Y = d.Velocity.Y },
        Health   = { Current = hp.Max, Max = hp.Max },
        Label    = { Name = d.Label.Name .. " (reborn)" },
      })
      return
    end

    hp.Current = math.min(hp.Max, hp.Current + math.floor(HEAL_PER_SECOND * dt))
  end,
}

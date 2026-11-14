-- Moves every Drifter, bounces it off the walls of a 20 x 10 box and wears it
-- down by one hit point per tick. A spent Drifter is replaced by a fresh one
-- in the middle of the box (world.despawn + world.spawn).
local WIDTH, HEIGHT = 20, 10

---@type GodScript<example.Drifter>
return {
  object = "example:Drifter",

  update = function(dt, e, d)
    local pos, vel, hp = d.Position, d.Velocity, d.Health

    pos.X = pos.X + vel.X * dt
    pos.Y = pos.Y + vel.Y * dt
    if pos.X < 0 or pos.X > WIDTH then
      vel.X = -vel.X
      pos.X = math.max(0, math.min(WIDTH, pos.X))
    end
    if pos.Y < 0 or pos.Y > HEIGHT then
      vel.Y = -vel.Y
      pos.Y = math.max(0, math.min(HEIGHT, pos.Y))
    end

    hp.Current = hp.Current - 1
    if hp.Current <= 0 then
      world.despawn(e)
      world.spawn("example:Drifter", {
        Position = { X = WIDTH / 2, Y = HEIGHT / 2 },
        Velocity = { X = 0 - vel.X, Y = 0 - vel.Y },
        Health   = { Current = hp.Max, Max = hp.Max },
        Label    = { Name = d.Label.Name .. "'" },
      })
    end
  end,
}

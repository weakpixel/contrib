package ecslua_test

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	ecslua "github.com/weakpixel/contrib/lua"
	"github.com/weakpixel/contrib/lua/example/game"
	"github.com/weakpixel/ecs"
	"github.com/weakpixel/ecs/system"
)

// A Lua script drives Game Objects: wounded drifters heal, and a spent one is
// replaced by a fresh drifter. The script is example/scripts/heal.lua.
func Example() {
	world := ecs.NewWorld()
	drifters := game.NewDrifterView(world)

	newDrifter := func(name string, hp int64) ecs.Entity {
		return drifters.NewEntity(game.Drifter{
			Position: &game.Position{X: 1, Y: 2},
			Velocity: &game.Velocity{X: 3, Y: 0},
			Health:   &game.Health{Current: hp, Max: 5},
			Label:    game.Label{Name: name},
		})
	}
	wounded := newDrifter("wounded", 2)
	spent := newDrifter("spent", 0)

	rt := ecslua.NewRuntime(world, ecslua.Options{})
	defer rt.Close()

	var sched system.Schedule
	sched.AddStage("simulate")
	sched.AddRunner(rt.NewSystem("example/scripts/heal.lua"))
	sched.Run(world, time.Second) // one tick of one second

	fmt.Println("wounded:", drifters.Get(wounded).Health.Current, "hp")
	fmt.Println("spent alive:", world.IsAlive(spent))
	for _, d := range drifters.Each() {
		if d.Label.Name != "wounded" {
			fmt.Println(d.Label.Name+":", d.Health.Current, "hp at", d.Position.X, d.Position.Y)
		}
	}
	// Output:
	// wounded: 4 hp
	// spent alive: false
	// spent (reborn): 5 hp at 1 2
}

// A script edited while the game runs: Reload swaps the new version in
// between ticks, and a broken edit leaves the running version alone. Set
// Options.WatchInterval to have the system do this by itself.
func ExampleSystem_Reload() {
	dir, _ := os.MkdirTemp("", "reload")
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "heal.lua")
	write := func(amount string) {
		os.WriteFile(path, []byte(`return { object = "example:Drifter", update = function(dt, e, d)
			d.Health.Current = d.Health.Current + `+amount+`
		end }`), 0o644)
	}

	world := ecs.NewWorld()
	drifters := game.NewDrifterView(world)
	e := drifters.NewEntity(game.Drifter{
		Position: &game.Position{}, Velocity: &game.Velocity{},
		Health: &game.Health{Current: 0, Max: 100},
		Label:  game.Label{Name: "d"},
	})
	rt := ecslua.NewRuntime(world, ecslua.Options{})
	defer rt.Close()

	write("1")
	sys := rt.NewSystem(path)
	var sched system.Schedule
	sched.AddRunner(sys)
	sched.Run(world, time.Second)
	fmt.Println("after v1:", drifters.Get(e).Health.Current)

	write("10")
	fmt.Println("reload:", sys.Reload())
	sched.Run(world, time.Second)
	fmt.Println("after v2:", drifters.Get(e).Health.Current)

	os.WriteFile(path, []byte("not lua"), 0o644)
	fmt.Println("broken reload fails:", sys.Reload() != nil)
	sched.Run(world, time.Second)
	fmt.Println("still v2:", drifters.Get(e).Health.Current)
	// Output:
	// after v1: 1
	// reload: <nil>
	// after v2: 11
	// broken reload fails: true
	// still v2: 21
}

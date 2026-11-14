// Command example runs a Lua script as a Weakpixel Engine system.
//
// From contrib/lua:
//
//	go run ./example
//	go run ./example -ticks 20 -dt 0.25
//
// It builds a World, creates Game Objects (Drifters) from a serialized GOD
// document and from Go, runs the embedded scripts/drift.lua every tick next to a
// small Go system that prints the state, and finally writes the World back as
// a serialized GOD.
package main

import (
	"bytes"
	"embed"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	ecslua "github.com/weakpixel/contrib/lua"
	"github.com/weakpixel/contrib/lua/example/game"
	"github.com/weakpixel/ecs"
	"github.com/weakpixel/ecs/system"
	"github.com/weakpixel/engine/god"
)

// worldJSON is a serialized GOD (doc/guide/ecs/game-object-definition.md#3-serialized-god).
//
//go:embed world.json
var worldJSON []byte

// scripts holds the Lua scripts inside the binary, so the example runs without
// the source tree. -script runs a file from disk instead (and can be edited
// while the game runs when WatchInterval is set).
//
//go:embed scripts/*.lua
var scripts embed.FS

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "example:", err)
		os.Exit(1)
	}
}

// run is main without the process, so a test can call it.
func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("example", flag.ContinueOnError)
	ticks := fs.Int("ticks", 12, "number of ticks to run")
	dt := fs.Float64("dt", 0.5, "seconds per tick")
	script := fs.String("script", "", "Lua script file to run (default: the embedded scripts/drift.lua)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	world := ecs.NewWorld()
	drifters := game.NewDrifterView(world)

	// Objects come from a serialized GOD document ...
	if _, err := god.Deserialize(world, bytes.NewReader(worldJSON)); err != nil {
		return err
	}
	// ... and from the typed view generated for the Go struct.
	drifters.NewEntity(game.Drifter{
		Position: &game.Position{X: 10, Y: 5},
		Velocity: &game.Velocity{X: 0, Y: 4},
		Health:   &game.Health{Current: 4, Max: 4},
		Label:    game.Label{Name: "gamma"},
	})

	// Set WatchInterval (for example time.Second) to edit the script while the
	// game runs: see doc/guide/lua/runtime.md#hot-reload.
	rt := ecslua.NewRuntime(world, ecslua.Options{})
	defer rt.Close()

	var sched system.Schedule
	sched.AddStage("simulate")
	if *script != "" {
		sched.AddRunner(rt.NewSystem(*script))
	} else {
		sched.AddRunner(rt.NewSystemFS(scripts, "scripts/drift.lua"))
	}
	sched.AddStage("report")
	sched.Add(func(ctx *system.Context) { report(stdout, ctx, drifters) })

	fmt.Fprintf(stdout, "tick  %-8s %7s %7s %5s\n", "name", "x", "y", "hp")
	for range *ticks {
		sched.Run(world, time.Duration(*dt*float64(time.Second)))
	}

	// Write the World back as a serialized GOD.
	var objs []god.Object
	for e := range drifters.Each() {
		objs = append(objs, god.Object{SchemaMeta: god.SchemaMeta{Group: "example", Class: "Drifter"}, Entity: e})
	}
	var out bytes.Buffer
	if err := god.Serialize(world, &out, objs...); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "\nfinal state: %s\n", out.String())
	return nil
}

// report is a plain Go system: it prints every Drifter after the Lua system ran.
func report(w io.Writer, ctx *system.Context, drifters game.DrifterView) {
	for _, d := range drifters.Each() {
		fmt.Fprintf(w, "%4d  %-8s %7.2f %7.2f %2d/%d\n",
			ctx.Time.Tick, d.Label.Name, d.Position.X, d.Position.Y, d.Health.Current, d.Health.Max)
	}
}

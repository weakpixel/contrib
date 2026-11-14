package ecslua_test

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	ecslua "github.com/weakpixel/contrib/lua"
	"github.com/weakpixel/contrib/lua/internal/luatest"
	"github.com/weakpixel/ecs"
	"github.com/weakpixel/ecs/system"
)

const regenScript = `return { object = "test:TestObject", update = function(dt, e, h)
	local hp = h.Counter
	if hp.Current < hp.Max then hp.Current = hp.Current + 1 end
	h.Point.X = h.Point.X + h.Delta.X * dt
end }`

// warmSchedule returns a schedule that already ran once (so Init, query
// scans and metatable creation are behind it) over n objs.
func warmSchedule(tb testing.TB, n int) (*system.Schedule, *ecs.World) {
	tb.Helper()
	return warmScheduleWatch(tb, n, 0)
}

// warmScheduleWatch is warmSchedule with Options.WatchInterval set.
func warmScheduleWatch(tb testing.TB, n int, watch time.Duration) (*system.Schedule, *ecs.World) {
	tb.Helper()
	world := ecs.NewWorld()
	objs := luatest.NewTestObjectView(world)
	for range n {
		objs.NewEntity(obj(1))
	}
	rt := ecslua.NewRuntime(world, ecslua.Options{WatchInterval: watch, OnError: func(_ string, err error) { tb.Fatal(err) }})
	tb.Cleanup(rt.Close)

	var s system.Schedule
	s.AddRunner(rt.NewSystem(script(tb, regenScript)))
	s.Run(world, time.Second/60)
	return &s, world
}

// TestRunAllocsBounded documents that Lua systems are not 0-alloc (see
// doc/guide/runtime.md#performance) and guards against the per-entity cost growing.
// Measured at about 21 allocs per entity for regenScript: GopherLua boxes
// every number it receives or returns and each entity gets a new handle.
func TestRunAllocsBounded(t *testing.T) {
	const n = 100
	s, world := warmSchedule(t, n)
	allocs := testing.AllocsPerRun(20, func() { s.Run(world, time.Second/60) })
	perEntity := allocs / n
	t.Logf("%.1f allocs per Run, %.2f per entity", allocs, perEntity)
	if perEntity > 32 {
		t.Fatalf("%.2f allocs per entity, want <= 32", perEntity)
	}
}

// TestWatchAllocsBounded: with WatchInterval set and no file change, a tick
// that checks the file costs at most two more allocations (os.Stat's FileInfo and path).
// On js/wasm os.Stat goes through syscall/js and allocates per call, so the
// bound is a native one.
func TestWatchAllocsBounded(t *testing.T) {
	if runtime.GOOS == "js" {
		t.Skip("os.Stat allocates through syscall/js; the bound is for native builds")
	}
	const n = 100
	base, bw := warmSchedule(t, n)
	want := testing.AllocsPerRun(20, func() { base.Run(bw, time.Second/60) })
	// An interval below the tick delta makes every Run stat the file.
	s, world := warmScheduleWatch(t, n, time.Nanosecond)
	got := testing.AllocsPerRun(20, func() { s.Run(world, time.Second/60) })
	t.Logf("%.0f allocs per Run, %.0f with a stat every tick", want, got)
	if got > want+2 {
		t.Fatalf("watching adds %.0f allocs per tick, want <= 2", got-want)
	}
}

func BenchmarkLuaSystem(b *testing.B) {
	for _, n := range []int{1_000, 10_000} {
		b.Run(fmt.Sprintf("objs=%d", n), func(b *testing.B) {
			s, world := warmSchedule(b, n)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				s.Run(world, time.Second/60)
			}
		})
	}
}

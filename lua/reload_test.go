package ecslua_test

import (
	"os"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"

	ecslua "github.com/weakpixel/contrib/lua"
	"github.com/weakpixel/contrib/lua/internal/luatest"
	"github.com/weakpixel/ecs"
)

const tick = time.Second

// healthSetter returns a TestObject script that sets Counter.Current to hp.
func healthSetter(hp string) string {
	return `return { object = "test:TestObject", update = function(dt, e, h) h.Counter.Current = ` + hp + ` end }`
}

// edit rewrites the script and moves its modtime forward, so the change is
// seen even on file systems with coarse timestamps.
func edit(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour * time.Duration(1+editCount))
	editCount++
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
}

var editCount int

// watched returns a fixture with one obj and a watching system for body.
func watched(t *testing.T, body string) (*fixture, ecs.Entity, string) {
	t.Helper()
	f := newFixture(t, ecslua.Options{WatchInterval: tick})
	e := f.objs.NewEntity(obj(1))
	path := script(t, body)
	f.sched.AddRunner(f.rt.NewSystem(path))
	f.sched.Run(f.world, tick)
	return f, e, path
}

func hp(f *fixture, e ecs.Entity) int64 {
	return ecs.GetComp[luatest.Counter](f.world, e).Current
}

func TestReloadSwapsBehavior(t *testing.T) {
	f, e, path := watched(t, healthSetter("10"))
	if got := hp(f, e); got != 10 {
		t.Fatalf("hp = %d, want 10", got)
	}
	edit(t, path, healthSetter("20"))
	f.sched.Run(f.world, tick)
	f.noErr()
	if got := hp(f, e); got != 20 {
		t.Fatalf("hp = %d after edit, want 20", got)
	}
}

func TestReloadKeepsOldOnError(t *testing.T) {
	f, e, path := watched(t, healthSetter("10"))

	edit(t, path, `this is not lua`)
	f.sched.Run(f.world, tick)
	f.sched.Run(f.world, tick)
	f.sched.Run(f.world, tick)
	if len(f.errs) != 1 {
		t.Fatalf("%d errors for one bad file version, want 1: %v", len(f.errs), f.errs)
	}
	f.wantErr("reload failed, keeping previous version")
	ecs.GetComp[luatest.Counter](f.world, e).Current = 0
	f.sched.Run(f.world, tick)
	if got := hp(f, e); got != 10 {
		t.Fatalf("old version stopped running: hp = %d, want 10", got)
	}

	edit(t, path, healthSetter("30")) // fixed
	f.sched.Run(f.world, tick)
	if got := hp(f, e); got != 30 || len(f.errs) != 1 {
		t.Fatalf("hp = %d, errors = %v; want recovery to 30 without new errors", got, f.errs)
	}

	edit(t, path, `return 1`) // a second, different bad version reports again
	f.sched.Run(f.world, tick)
	if len(f.errs) != 2 {
		t.Fatalf("%d errors, want 2", len(f.errs))
	}
}

func TestReloadChangesObject(t *testing.T) {
	f, e, path := watched(t, healthSetter("10"))
	h := obj(5)
	ext := luatest.NewTestExtendedView(f.world).NewEntity(luatest.TestExtended{
		Point: h.Point, Counter: h.Counter, Delta: h.Delta, Label: h.Label,
		Extra: &luatest.Extra{Value: 1},
	})
	f.sched.Run(f.world, tick)
	if hp(f, e) != 10 || hp(f, ext) != 10 {
		t.Fatal("TestObject script should reach both entities")
	}

	edit(t, path, `return { object = "test:TestExtended", update = function(dt, e, b) b.Extra.Value = 7 end }`)
	f.sched.Run(f.world, tick)
	f.noErr()
	if got := ecs.GetComp[luatest.Extra](f.world, ext).Value; got != 7 {
		t.Fatalf("ext armor = %d, want 7", got)
	}
	if hp(f, e) != 10 {
		t.Fatal("the plain obj must no longer match the TestExtended script")
	}
}

func TestReloadRecoversFromFailedInit(t *testing.T) {
	f := newFixture(t, ecslua.Options{WatchInterval: tick})
	e := f.objs.NewEntity(obj(1))
	path := script(t, `return { object = "test:TestObject", update = function( end }`) // typo
	f.sched.AddRunner(f.rt.NewSystem(path))
	f.sched.Run(f.world, tick)
	f.sched.Run(f.world, tick)
	if len(f.errs) != 1 {
		t.Fatalf("%d errors, want 1: %v", len(f.errs), f.errs)
	}

	edit(t, path, healthSetter("42"))
	f.sched.Run(f.world, tick)
	if got := hp(f, e); got != 42 || len(f.errs) != 1 {
		t.Fatalf("hp = %d, errors = %v; want 42 and no new error", got, f.errs)
	}
}

func TestReloadResetsScriptState(t *testing.T) {
	f := newFixture(t, ecslua.Options{WatchInterval: tick})
	f.objs.NewEntity(obj(1))
	inits := 0
	f.rt.L.SetGlobal("count_init", f.rt.L.NewFunction(func(*lua.LState) int { inits++; return 0 }))
	path := script(t, `return { object = "test:TestObject", init = function() count_init() end, update = function() end }`)
	f.sched.AddRunner(f.rt.NewSystem(path))
	f.sched.Run(f.world, tick)
	edit(t, path, `return { object = "test:TestObject", init = function() count_init() end, update = function() end } -- v2`)
	f.sched.Run(f.world, tick)
	if inits != 2 {
		t.Fatalf("init ran %d times, want 2 (once per version)", inits)
	}
}

func TestReloadMethod(t *testing.T) {
	f := newFixture(t, ecslua.Options{}) // no watching
	e := f.objs.NewEntity(obj(1))
	path := script(t, healthSetter("10"))
	s := f.rt.NewSystem(path)
	f.sched.AddRunner(s)
	f.sched.Run(f.world, tick)

	edit(t, path, healthSetter("20"))
	f.sched.Run(f.world, tick)
	if hp(f, e) != 10 {
		t.Fatal("without WatchInterval the edit must not be picked up")
	}
	if err := s.Reload(); err != nil {
		t.Fatal(err)
	}
	f.sched.Run(f.world, tick)
	if got := hp(f, e); got != 20 {
		t.Fatalf("hp = %d after Reload, want 20", got)
	}

	edit(t, path, `return 1`)
	if err := s.Reload(); err == nil {
		t.Fatal("Reload of a bad script returned nil")
	}
	f.noErr() // Reload returns the error instead of calling OnError
	f.sched.Run(f.world, tick)
	if hp(f, e) != 20 {
		t.Fatal("old version must keep running")
	}
}

func TestWatchIntervalThrottlesStat(t *testing.T) {
	f := newFixture(t, ecslua.Options{WatchInterval: 10 * time.Second})
	e := f.objs.NewEntity(obj(1))
	path := script(t, healthSetter("10"))
	f.sched.AddRunner(f.rt.NewSystem(path))
	f.sched.Run(f.world, time.Second) // check at t=0 (also the Init)

	edit(t, path, healthSetter("20"))
	for range 8 { // t = 2..9s: before the next check
		f.sched.Run(f.world, time.Second)
	}
	if hp(f, e) != 10 {
		t.Fatal("edit seen before the interval elapsed")
	}
	f.sched.Run(f.world, 2*time.Second) // crosses t=10s
	f.sched.Run(f.world, time.Second)
	if got := hp(f, e); got != 20 {
		t.Fatalf("hp = %d, want 20 once the interval elapsed", got)
	}
}

func TestNoWatchNoStat(t *testing.T) {
	// Without WatchInterval a deleted file is never noticed: Run does not
	// touch the file system.
	f := newFixture(t, ecslua.Options{})
	e := f.objs.NewEntity(obj(1))
	path := script(t, healthSetter("10"))
	f.sched.AddRunner(f.rt.NewSystem(path))
	f.sched.Run(f.world, tick)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	f.sched.Run(f.world, tick)
	f.noErr()
	if hp(f, e) != 10 {
		t.Fatal("script stopped running")
	}
}

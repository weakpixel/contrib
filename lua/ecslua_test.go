package ecslua_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"

	ecslua "github.com/weakpixel/contrib/lua"
	"github.com/weakpixel/contrib/lua/internal/luatest"
	"github.com/weakpixel/ecs"
	"github.com/weakpixel/ecs/system"
	"github.com/weakpixel/engine/god"
)

// fixture is a World with TestObject entities, a Runtime that collects script
// errors instead of panicking, and a Schedule to run scripts in.
type fixture struct {
	t     *testing.T
	world *ecs.World
	objs  luatest.TestObjectView
	rt    *ecslua.Runtime
	errs  []error
	sched system.Schedule
}

func newFixture(t *testing.T, opt ecslua.Options) *fixture {
	t.Helper()
	f := &fixture{t: t, world: ecs.NewWorld()}
	f.objs = luatest.NewTestObjectView(f.world)
	if opt.OnError == nil {
		opt.OnError = func(_ string, err error) { f.errs = append(f.errs, err) }
	}
	f.rt = ecslua.NewRuntime(f.world, opt)
	t.Cleanup(f.rt.Close)
	return f
}

func obj(hp int64) luatest.TestObject {
	return luatest.TestObject{
		Point:   &luatest.Point{X: 1, Y: 2},
		Counter: &luatest.Counter{Current: hp, Max: 100, Active: true},
		Delta:   &luatest.Delta{X: 1},
		Label:   luatest.Label{Name: "Rex"},
	}
}

// script writes body to a temp file and returns its path.
func script(t testing.TB, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "test.lua")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// run adds the script as the only system and runs one tick.
func (f *fixture) run(body string) {
	f.t.Helper()
	f.sched = system.Schedule{}
	f.sched.AddRunner(f.rt.NewSystem(script(f.t, body)))
	f.sched.Run(f.world, time.Second/2)
}

func (f *fixture) wantErr(substr string) {
	f.t.Helper()
	if len(f.errs) == 0 {
		f.t.Fatalf("no script error, want one containing %q", substr)
	}
	if got := f.errs[0].Error(); !strings.Contains(got, substr) {
		f.t.Fatalf("error %q does not contain %q", got, substr)
	}
}

func (f *fixture) noErr() {
	f.t.Helper()
	if len(f.errs) != 0 {
		f.t.Fatalf("unexpected script errors: %v", f.errs)
	}
}

func count(v luatest.TestObjectView) int {
	n := 0
	for range v.Each() {
		n++
	}
	return n
}

func TestSandbox(t *testing.T) {
	f := newFixture(t, ecslua.Options{})
	f.world.NewEntity() // keep the world non-trivial
	f.objs.NewEntity(obj(10))
	f.run(`
		local missing = {}
		for _, name in ipairs{"os", "io", "debug", "package", "dofile", "loadfile", "load", "loadstring", "getfenv", "setfenv", "collectgarbage", "require"} do
			if _G[name] ~= nil then missing[#missing + 1] = name end
		end
		return { object = "test:TestObject", update = function()
			if #missing > 0 then error("still available: " .. table.concat(missing, ",")) end
			if math.floor(1.5) ~= 1 or string.upper("a") ~= "A" then error("libs missing") end
		end }`)
	f.noErr()
}

func TestTickBudgetAbortsLoop(t *testing.T) {
	f := newFixture(t, ecslua.Options{TickBudget: 20 * time.Millisecond})
	f.objs.NewEntity(obj(10))
	f.objs.NewEntity(obj(10))
	f.run(`return { object = "test:TestObject", update = function() while true do end end }`)
	if len(f.errs) != 1 {
		t.Fatalf("got %d errors, want exactly 1: %v", len(f.errs), f.errs)
	}
	f.wantErr("tick budget exceeded")
}

func TestScriptsDontShareGlobals(t *testing.T) {
	f := newFixture(t, ecslua.Options{})
	f.objs.NewEntity(obj(10))
	a := script(t, `leak = 1; return { object = "test:TestObject", update = function() end }`)
	b := script(t, `return { object = "test:TestObject", update = function() if leak ~= nil then error("leaked") end end }`)
	f.sched.AddRunner(f.rt.NewSystem(a), f.rt.NewSystem(b))
	f.sched.Run(f.world, time.Second)
	f.noErr()
}

func TestObjectAccess(t *testing.T) {
	f := newFixture(t, ecslua.Options{})
	armed := obj(50)
	armed.Optional = &luatest.Optional{Float: 5, Int: 12}
	withWeapon := f.objs.NewEntity(armed)
	bare := f.objs.NewEntity(obj(60))

	f.run(`return { object = "test:TestObject", update = function(dt, e, h)
		h.Counter.Current = h.Counter.Current + 1
		h.Point.X = h.Point.X + h.Delta.X * dt
		h.Delta.Y = 7
		if h.Optional ~= nil then h.Optional.Int = h.Optional.Int - 1 end
		if h.Label.Name ~= "Rex" then error("companion " .. tostring(h.Label.Name)) end
		if h.Label.Nested ~= nil then error("attachment should be nil") end
	end }`)
	f.noErr()

	got := f.objs.Get(withWeapon)
	if got.Counter.Current != 51 || got.Optional.Int != 11 || got.Delta.Y != 7 || got.Point.X != 1.5 {
		t.Fatalf("armed obj after script: counter=%+v optional=%+v delta=%+v point=%+v", got.Counter, got.Optional, got.Delta, got.Point)
	}
	got = f.objs.Get(bare)
	if got.Counter.Current != 61 || got.Optional != nil {
		t.Fatalf("bare obj after script: health=%+v weapon=%v", got.Counter, got.Optional)
	}
}

func TestWriteErrors(t *testing.T) {
	tests := []struct {
		name, stmt, want string
	}{
		{"fractional integer", `h.Counter.Current = 1.5`, "$.Current: expected integer, got 1.5"},
		{"wrong type", `h.Counter.Current = "x"`, "$.Current: expected integer, got string"},
		{"wrong type bool", `h.Counter.Active = 1`, "$.Active: expected boolean, got 1"},
		{"unknown field write", `h.Counter.Nope = 1`, "unknown field Counter.Nope"},
		{"unknown field read", `local _ = h.Counter.Nope`, "unknown field Counter.Nope"},
		{"unknown component", `local _ = h.Nope`, "unknown component test:TestObject.Nope"},
		{"readonly", `h.Label.Name = "x"`, "component Label is readonly"},
		{"assign component", `h.Counter = 1`, "cannot assign component Counter"},
		{"nested object", `h.Label.Nested = {Name = 1}`, "component Label is readonly"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, ecslua.Options{})
			f.objs.NewEntity(obj(10))
			f.run(`return { object = "test:TestObject", update = function(dt, e, h) ` + tc.stmt + ` end }`)
			f.wantErr(tc.want)
			if !strings.Contains(f.errs[0].Error(), "test.lua:1:") {
				t.Errorf("error %q lacks the script position", f.errs[0])
			}
		})
	}
}

func TestWorldSpawnValidates(t *testing.T) {
	const objTable = `{
		Point = {X = 1, Y = 2},
		Counter = %s, Delta = {X = 0, Y = 0},
		Label = {Name = "Rex"},
	}`
	spawn := func(health string) string {
		return fmt.Sprintf(`return { object = "test:TestObject", update = function() world.spawn("test:TestObject", `+objTable+`) end }`, health)
	}

	t.Run("valid spawn is queued and applied after the stage", func(t *testing.T) {
		f := newFixture(t, ecslua.Options{})
		f.objs.NewEntity(obj(10))
		f.run(spawn(`{Current = 5, Max = 5, Active = true}`))
		f.noErr()
		if n := count(f.objs); n != 2 {
			t.Fatalf("objs = %d, want 2", n)
		}
		// The spawned entity carries its class tag.
		tagged := 0
		for e := range f.objs.Each() {
			if meta, ok := god.ClassOf(f.world, e); ok && meta.Key() == "test:TestObject" &&
				ecs.HasComp[god.ObjectClass](f.world, e) {
				tagged++
			}
		}
		if tagged != 1 {
			t.Fatalf("tagged objs = %d, want 1 (the Lua-spawned one)", tagged)
		}
	})
	t.Run("invalid component", func(t *testing.T) {
		f := newFixture(t, ecslua.Options{})
		f.objs.NewEntity(obj(10))
		f.run(spawn(`{Current = "x", Max = 5, Active = true}`))
		f.wantErr("component test:Counter: $.Current: expected integer, got string")
		if n := count(f.objs); n != 1 {
			t.Fatalf("objs = %d, want 1 (nothing spawned)", n)
		}
	})
	t.Run("missing and unregistered components are reported together", func(t *testing.T) {
		f := newFixture(t, ecslua.Options{})
		f.objs.NewEntity(obj(10))
		f.run(`return { object = "test:TestObject", update = function() world.spawn("test:TestObject", {Nope = {}}) end }`)
		f.wantErr("component Nope is not registered")
		f.wantErr("missing required component")
	})
	t.Run("unknown object", func(t *testing.T) {
		f := newFixture(t, ecslua.Options{})
		f.objs.NewEntity(obj(10))
		f.run(`return { object = "test:TestObject", update = function() world.spawn("test:Nope", {}) end }`)
		f.wantErr("unknown object test:Nope")
	})
}

func TestDespawnQueued(t *testing.T) {
	f := newFixture(t, ecslua.Options{})
	dying := f.objs.NewEntity(obj(0))
	alive := f.objs.NewEntity(obj(5))
	f.run(`return { object = "test:TestObject", update = function(dt, e, h)
		if h.Counter.Current <= 0 then world.despawn(e); world.despawn(e) end
	end }`)
	f.noErr()
	if f.world.IsAlive(dying) || !f.world.IsAlive(alive) {
		t.Fatalf("dying alive=%v, alive alive=%v", f.world.IsAlive(dying), f.world.IsAlive(alive))
	}
}

func TestWorldGetSetRemoveDump(t *testing.T) {
	f := newFixture(t, ecslua.Options{})
	h := obj(10)
	h.Optional = &luatest.Optional{Float: 1, Int: 2}
	armed := f.objs.NewEntity(h)
	other := f.objs.NewEntity(obj(20))

	f.run(`return { object = "test:TestObject", update = function(dt, e, h)
		local o = world.get(e, "test:TestObject")
		if o == nil or o.Counter.Current ~= h.Counter.Current then error("get mismatch") end
		if world.get(e, "test:TestExtended") ~= nil then error("obj is not a ext") end
		local dump = world.dump(e, "test:TestObject")
		if dump["test:Counter"].Max ~= 100 or dump["test:Label"].Name ~= "Rex" then error("dump mismatch") end
		if h.Optional == nil then
			world.set(e, "test:TestObject", "Optional", {Float = 9, Int = 3})
		else
			world.remove(e, "test:TestObject", "Optional")
		end
	end }`)
	f.noErr()
	if f.objs.Get(armed).Optional != nil {
		t.Error("armed obj should have lost its weapon")
	}
	if w := f.objs.Get(other).Optional; w == nil || w.Float != 9 || w.Int != 3 {
		t.Errorf("other obj weapon = %+v, want {9 3}", w)
	}

	f = newFixture(t, ecslua.Options{})
	f.objs = luatest.NewTestObjectView(f.world)
	f.objs.NewEntity(obj(1))
	f.run(`return { object = "test:TestObject", update = function(dt, e) world.remove(e, "test:TestObject", "Counter") end }`)
	f.wantErr("component test:TestObject.test:Counter is required and cannot be removed")
}

func TestStaleEntity(t *testing.T) {
	f := newFixture(t, ecslua.Options{})
	first := f.objs.NewEntity(obj(1))
	f.run(`
		local kept
		return { object = "test:TestObject", update = function(dt, e)
			if kept == nil then kept = e; world.despawn(e) else world.despawn(kept) end
		end }`)
	f.noErr() // the entity is still alive when despawn is called, and gone after the stage
	if f.world.IsAlive(first) {
		t.Fatal("first obj should be despawned")
	}

	f.objs.NewEntity(obj(2))
	f.sched.Run(f.world, time.Second) // the script now holds a stale handle
	f.wantErr("stale entity")
}

func TestEntityHandleEquality(t *testing.T) {
	f := newFixture(t, ecslua.Options{})
	f.objs.NewEntity(obj(1))
	f.objs.NewEntity(obj(2))
	f.run(`
		local first
		return { object = "test:TestObject", update = function(dt, e)
			if first == nil then first = e; return end
			if e == first then error("distinct entities compare equal") end
			if world.get(e, "test:TestObject") == nil then error("lost entity") end
		end }`)
	f.noErr()
}

func TestInitRunsOnce(t *testing.T) {
	f := newFixture(t, ecslua.Options{})
	f.objs.NewEntity(obj(1))
	inits := 0
	f.rt.L.SetGlobal("count_init", f.rt.L.NewFunction(func(*lua.LState) int { inits++; return 0 }))
	f.sched.AddRunner(f.rt.NewSystem(script(t, `return { object = "test:TestObject", init = function() count_init() end, update = function() end }`)))
	f.sched.Run(f.world, time.Second)
	f.sched.Run(f.world, time.Second)
	if inits != 1 {
		t.Fatalf("init ran %d times, want 1", inits)
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name, body, want string
	}{
		{"not lua", `this is not lua`, "test.lua"},
		{"not a table", `return 1`, "script must return a table"},
		{"no object", `return { update = function() end }`, "unknown object"},
		{"unknown object", `return { object = "test:Nope", update = function() end }`, "unknown object test:Nope"},
		{"no colon", `return { object = "TestObject", update = function() end }`, "unknown object TestObject"},
		{"no update", `return { object = "test:TestObject" }`, "update must be a function"},
		{"init fails", `return { object = "test:TestObject", update = function() end, init = function() error("boom") end }`, "boom"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, ecslua.Options{})
			f.objs.NewEntity(obj(1))
			f.run(tc.body)
			f.wantErr(tc.want)
			f.sched.Run(f.world, time.Second) // a failed system stays disabled
		})
	}
}

func TestMissingScriptFile(t *testing.T) {
	f := newFixture(t, ecslua.Options{})
	f.sched.AddRunner(f.rt.NewSystem(filepath.Join(t.TempDir(), "nope.lua")))
	f.sched.Run(f.world, time.Second)
	f.wantErr("nope.lua")
}

func TestOnErrorDefaultPanics(t *testing.T) {
	world := ecs.NewWorld()
	objs := luatest.NewTestObjectView(world)
	objs.NewEntity(obj(1))
	rt := ecslua.NewRuntime(world, ecslua.Options{})
	defer rt.Close()

	var s system.Schedule
	s.AddRunner(rt.NewSystem(script(t, `return { object = "test:TestObject", update = function() error("boom") end }`)))
	defer func() {
		msg, _ := recover().(string)
		if !strings.HasPrefix(msg, "lua: ") || !strings.Contains(msg, "boom") {
			t.Fatalf("panic %q, want a \"lua: \" message containing boom", msg)
		}
	}()
	s.Run(world, time.Second)
}

func TestNewRuntimeNilWorld(t *testing.T) {
	defer func() {
		if msg, _ := recover().(string); msg != "lua: nil world" {
			t.Fatalf("panic %q, want %q", msg, "lua: nil world")
		}
	}()
	ecslua.NewRuntime(nil, ecslua.Options{})
}

func TestExtendedMatchesObjectScripts(t *testing.T) {
	// TestExtended carries every required TestObject component, so a TestObject script sees it.
	f := newFixture(t, ecslua.Options{})
	h := obj(10)
	ext := luatest.NewTestExtendedView(f.world).NewEntity(luatest.TestExtended{
		Point: h.Point, Counter: h.Counter, Delta: h.Delta, Label: h.Label,
		Extra: &luatest.Extra{Value: 1},
	})
	f.run(`return { object = "test:TestObject", update = function(dt, e, h) h.Counter.Current = 99 end }`)
	f.noErr()
	if hp := ecs.GetComp[luatest.Counter](f.world, ext).Current; hp != 99 {
		t.Fatalf("ext health = %d, want 99", hp)
	}
}

// An object may hold any registered component, not only its class's:
// scripts reach it by key or by its bare name when unambiguous.
func TestExtraComponents(t *testing.T) {
	f := newFixture(t, ecslua.Options{})
	e := f.objs.NewEntity(obj(10))
	f.run(`return { object = "test:TestObject", update = function(dt, e, h)
		if h.Extra ~= nil then error("obj has no armor yet") end
		world.set(e, "test:TestObject", "Extra", {Value = 4})
	end }`)
	f.noErr()
	if a := ecs.GetCompOrNil[luatest.Extra](f.world, e); a == nil || a.Value != 4 {
		t.Fatalf("Extra = %v", a)
	}
	f.run(`return { object = "test:TestObject", update = function(dt, e, h)
		if h["test:Extra"].Value ~= 4 then error("armor by key") end
		h.Extra.Value = 5
		if world.dump(e, "test:TestObject")["test:Extra"].Value ~= 5 then error("dump lacks armor") end
		world.remove(e, "test:TestObject", "Extra")
	end }`)
	f.noErr()
	if ecs.HasComp[luatest.Extra](f.world, e) {
		t.Fatal("Extra was not removed")
	}
	f.run(`return { object = "test:TestObject", update = function()
		world.spawn("test:TestObject", {
			Point = {X = 1, Y = 2},
			Counter = {Current = 1, Max = 1, Active = true}, Delta = {X = 0, Y = 0},
			Label = {Name = "Rex"}, Extra = {Value = 7},
		})
	end }`)
	f.noErr()
	armored := 0
	for e := range f.objs.Each() {
		if a := ecs.GetCompOrNil[luatest.Extra](f.world, e); a != nil && a.Value == 7 {
			armored++
		}
	}
	if armored != 1 {
		t.Fatalf("spawned objs with Extra = %d, want 1", armored)
	}
}

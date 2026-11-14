package ecslua_test

import (
	"testing"

	ecslua "github.com/weakpixel/contrib/lua"
	"github.com/weakpixel/contrib/lua/internal/luatest"
	"github.com/weakpixel/ecs"
)

// TestEntityRefFields checks Ref.Target (an ecs.Entity field) from Lua:
// it reads as an entity handle, dump shows the handle, a handle or nil can
// be assigned, and world.set accepts a handle inside a table.
func TestEntityRefFields(t *testing.T) {
	f := newFixture(t, ecslua.Options{})
	leader := f.objs.NewEntity(obj(77))
	scoutObj := obj(10)
	scoutObj.Ref = &luatest.Ref{Target: leader, Distance: 3}
	scout := f.objs.NewEntity(scoutObj)

	f.run(`return { object = "test:TestObject", update = function(dt, e, h)
		if h.Ref ~= nil then
			local target = h.Ref.Target
			if target == nil then error("nil target") end
			local other = world.get(target, "test:TestObject")
			if other == nil or other.Counter.Current ~= 77 then error("wrong target") end
			if world.dump(e, "test:TestObject")["test:Ref"].Target ~= target then error("dump lost the handle") end
			h.Ref.Target = nil
		else
			world.set(e, "test:TestObject", "Ref", {Target = e, Distance = 1})
		end
	end }`)
	f.noErr()

	if got := f.objs.Get(scout).Ref.Target; got != (ecs.Entity{}) {
		t.Fatalf("scout target = %v, want zero after assigning nil", got)
	}
	if got := f.objs.Get(leader).Ref; got == nil || got.Target != leader {
		t.Fatalf("leader Ref = %+v, want a self reference from world.set", got)
	}
}

func TestEntityRefFieldErrors(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"assign a number", `h.Ref.Target = 5`, "$.Target: expected entity, got 5"},
		{"set a string", `world.set(e, "test:TestObject", "Ref", {Target = "x", Distance = 1})`, `$.Target: invalid entity "x"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, ecslua.Options{})
			h := obj(1)
			h.Ref = &luatest.Ref{}
			f.objs.NewEntity(h)
			f.run(`return { object = "test:TestObject", update = function(dt, e, h) ` + tc.body + ` end }`)
			f.wantErr(tc.want)
		})
	}
}

package ecslua

import (
	"fmt"
	"reflect"

	lua "github.com/yuin/gopher-lua"

	"github.com/weakpixel/ecs"
	"github.com/weakpixel/engine/god"
)

// Entity references in Lua. A component field of type ecs.Entity (schema
// format "entity") reads as an entity handle (the userdata world.each and
// world.spawn use) or nil, and accepts the same on write. When a value goes
// through JSON (spawn tables, world.set, nested arrays and maps), a handle is
// written as the id "<ID>:<Generation>" and luaRefs turns it back into the
// live entity through god.DecodeComponent.

var entityType = reflect.TypeFor[ecs.Entity]()

// luaRefs are the god.Refs of a Runtime: ids name live entities directly.
type luaRefs struct{ world *ecs.World }

func luaRefID(e ecs.Entity) string { return fmt.Sprintf("%d:%d", e.ID, e.Generation) }

func (r luaRefs) RefID(e ecs.Entity) (string, error) {
	if !r.world.IsAlive(e) {
		return "", fmt.Errorf("stale entity %v", e)
	}
	return luaRefID(e), nil
}

func (r luaRefs) RefEntity(id string) (ecs.Entity, error) {
	var e ecs.Entity
	if _, err := fmt.Sscanf(id, "%d:%d", &e.ID, &e.Generation); err != nil || luaRefID(e) != id {
		return ecs.Entity{}, fmt.Errorf("invalid entity %q", id)
	}
	if !r.world.IsAlive(e) {
		return ecs.Entity{}, fmt.Errorf("stale entity %v", e)
	}
	return e, nil
}

// isEntityRef reports whether s describes an entity reference.
func isEntityRef(s *god.TypeSchema) bool { return s != nil && s.Format == god.FormatEntity }

// hasEntityRef reports whether s or anything below it is an entity reference.
func hasEntityRef(s *god.TypeSchema) bool {
	if s == nil {
		return false
	}
	if isEntityRef(s) || hasEntityRef(s.Items) || hasEntityRef(s.AdditionalProperties) {
		return true
	}
	for _, p := range s.Properties {
		if hasEntityRef(p) {
			return true
		}
	}
	return false
}

// entityToLua reads an ecs.Entity field: nil for the zero Entity, a handle
// otherwise.
func (rt *Runtime) entityToLua(e ecs.Entity) lua.LValue {
	if e == (ecs.Entity{}) {
		return lua.LNil
	}
	return rt.entityUserdata(e)
}

// assignEntity stores a handle (or nil, the zero Entity) into an ecs.Entity
// field. A stale handle is an error.
func (rt *Runtime) assignEntity(dst reflect.Value, lv lua.LValue, path string) error {
	if lv == lua.LNil {
		dst.SetZero()
		return nil
	}
	var e ecs.Entity
	ud, ok := lv.(*lua.LUserData)
	if ok {
		e, ok = ud.Value.(ecs.Entity)
	}
	if !ok {
		return fmt.Errorf("%s: expected entity, got %s", path, describe(lv))
	}
	if !rt.world.IsAlive(e) {
		return fmt.Errorf("%s: stale entity %v", path, e)
	}
	dst.Set(reflect.ValueOf(e))
	return nil
}

// refsToLua replaces the entity ids in lv (a Lua copy of JSON that
// EncodeComponent wrote with luaRefs) with entity handles, following s.
func (rt *Runtime) refsToLua(lv lua.LValue, s *god.TypeSchema) lua.LValue {
	if !hasEntityRef(s) {
		return lv
	}
	if isEntityRef(s) {
		str, ok := lv.(lua.LString)
		if !ok {
			return lv
		}
		e, err := (luaRefs{rt.world}).RefEntity(string(str))
		if err != nil {
			return lua.LNil
		}
		return rt.entityUserdata(e)
	}
	t, ok := lv.(*lua.LTable)
	if !ok {
		return lv
	}
	if s.Items != nil {
		for i := 1; i <= t.Len(); i++ {
			t.RawSetInt(i, rt.refsToLua(t.RawGetInt(i), s.Items))
		}
		return t
	}
	t.ForEach(func(k, v lua.LValue) {
		p := s.Properties[k.String()]
		if p == nil {
			p = s.AdditionalProperties
		}
		t.RawSet(k, rt.refsToLua(v, p))
	})
	return t
}

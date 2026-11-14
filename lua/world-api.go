package ecslua

import (
	"encoding/json"
	"errors"
	"fmt"

	lua "github.com/yuin/gopher-lua"

	"github.com/weakpixel/ecs"
	"github.com/weakpixel/engine/god"
)

// installWorldAPI defines the global `world` table (doc/guide/world-api.md).
// Structural changes are queued on the running system's Commands; spawn and
// set validate their arguments immediately so the Lua error carries the call
// site.
func (rt *Runtime) installWorldAPI() {
	L := rt.L
	world := L.NewTable()
	fns := map[string]lua.LGFunction{
		"despawn": rt.apiDespawn,
		"spawn":   rt.apiSpawn,
		"set":     rt.apiSet,
		"remove":  rt.apiRemove,
		"get":     rt.apiGet,
		"dump":    rt.apiDump,
	}
	for name, fn := range fns {
		world.RawSetString(name, L.NewFunction(fn))
	}
	L.SetGlobal("world", world)
}

// checkBinding reads argument n as an object key and returns its binding.
func (rt *Runtime) checkBinding(L *lua.LState, n int) *objectBinding {
	b, err := rt.binding(L.CheckString(n))
	if err != nil {
		L.RaiseError("%v", err)
	}
	return b
}

func (rt *Runtime) apiDespawn(L *lua.LState) int {
	e := rt.checkEntity(L, 1)
	rt.commands().Despawn(e)
	return 0
}

// apiSpawn implements world.spawn("g:C", {Comp = {...}}).
func (rt *Runtime) apiSpawn(L *lua.LState) int {
	b := rt.checkBinding(L, 1)
	tbl := L.CheckTable(2)

	raw := map[string]json.RawMessage{}
	var errs []error
	tbl.ForEach(func(k, v lua.LValue) {
		name, ok := k.(lua.LString)
		if !ok {
			errs = append(errs, fmt.Errorf("component name must be a string, got %s", k.Type()))
			return
		}
		// Bare names become keys, so CheckComponents sees the file form.
		var s *god.TypeSchema
		key := string(name)
		if i, known := b.byName[key]; known {
			s, key = b.comps[i].schema, b.comps[i].key
		}
		data, err := luaToJSON(v, s)
		if err != nil {
			errs = append(errs, fmt.Errorf("component %s: %w", name, err))
			return
		}
		if _, dup := raw[key]; dup {
			errs = append(errs, fmt.Errorf("component %s is given twice", key))
			return
		}
		raw[key] = data
	})
	if err := god.CheckComponents(b.schema.Group, b.schema.Class, raw); err != nil {
		errs = append(errs, err)
	}

	values := make([]any, 0, len(raw))
	for _, c := range b.comps {
		data, ok := raw[c.key]
		if !ok {
			continue
		}
		v, err := rt.decodeComponent(c, data)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		values = append(values, v)
	}
	if err := errors.Join(errs...); err != nil {
		L.RaiseError("%v", err)
	}
	// Tag the entity with its object class so tools (god.ClassOf,
	// god.Objects) know what it is without inference.
	if tag, ok := god.ClassTag(b.schema.Group, b.schema.Class); ok {
		values = append(values, tag)
	}
	rt.commands().Spawn(values...)
	return 0
}

// decodeComponent validates data against the component's schema and decodes
// it with god.DecodeComponent: asset keys resolve through the world's
// AssetResolver and entity references through luaRefs.
func (rt *Runtime) decodeComponent(c compBinding, data json.RawMessage) (any, error) {
	v, err := god.DecodeComponent(rt.world, c.component(), data, luaRefs{rt.world})
	if err != nil {
		return nil, fmt.Errorf("component %s: %w", c.key, err)
	}
	return v, nil
}

// checkComponent reads arguments n (object key) and n+1 (component key, or
// its bare name when unambiguous).
func (rt *Runtime) checkComponent(L *lua.LState, n int) (*objectBinding, compBinding) {
	b := rt.checkBinding(L, n)
	name := L.CheckString(n + 1)
	i, ok := b.byName[name]
	if !ok {
		L.RaiseError("unknown component %s.%s", b.key, name)
	}
	return b, b.comps[i]
}

// apiSet implements world.set(e, "g:C", "Comp", {...}): add or replace one
// component.
func (rt *Runtime) apiSet(L *lua.LState) int {
	e := rt.checkEntity(L, 1)
	_, c := rt.checkComponent(L, 2)
	data, err := luaToJSON(L.Get(4), c.schema)
	if err != nil {
		L.RaiseError("component %s: %v", c.key, err)
	}
	v, err := rt.decodeComponent(c, data)
	if err != nil {
		L.RaiseError("%v", err)
	}
	rt.commands().Do(func(w *ecs.World) {
		if w.IsAlive(e) { // may have been despawned by an earlier command
			w.SetComp(e, v)
		}
	})
	return 0
}

// apiRemove implements world.remove(e, "g:C", "Comp"); only optional
// components can be removed.
func (rt *Runtime) apiRemove(L *lua.LState) int {
	e := rt.checkEntity(L, 1)
	b, c := rt.checkComponent(L, 2)
	if !c.optional {
		L.RaiseError("component %s.%s is required and cannot be removed", b.key, c.key)
	}
	rt.commands().Do(func(w *ecs.World) {
		if w.IsAlive(e) && w.HasComp(e, c.id) {
			w.RemoveComp(e, c.id)
		}
	})
	return 0
}

// apiGet implements world.get(e, "g:C"): an object for e, or nil when e lacks
// a required component. It allocates.
func (rt *Runtime) apiGet(L *lua.LState) int {
	e := rt.checkEntity(L, 1)
	b := rt.checkBinding(L, 2)
	if !b.hasRequired(rt.world, e) {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(rt.newObject(b, e))
	return 1
}

// apiDump implements world.dump(e, "g:C"): the components as a Lua table (a
// copy), via god.SerializeComponents.
func (rt *Runtime) apiDump(L *lua.LState) int {
	e := rt.checkEntity(L, 1)
	b := rt.checkBinding(L, 2)
	out := L.NewTable()
	for _, c := range b.comps {
		v := rt.world.GetCompOrNil(e, c.id)
		if v == nil {
			if !c.optional {
				L.RaiseError("object %s: missing required component %s", b.key, c.key)
			}
			continue
		}
		raw, err := god.EncodeComponent(rt.world, c.component(), v, luaRefs{rt.world})
		if err != nil {
			L.RaiseError("component %s: %v", c.key, err)
		}
		lv, err := jsonToLua(L, raw)
		if err != nil {
			L.RaiseError("%v", err)
		}
		out.RawSetString(c.key, rt.refsToLua(lv, c.schema))
	}
	L.Push(out)
	return 1
}

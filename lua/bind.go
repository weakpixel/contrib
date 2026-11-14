package ecslua

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/weakpixel/ecs"
	"github.com/weakpixel/engine/god"
)

// metaKey identifies one struct metatable: the Go struct type, the schema
// describing it, and whether writes are allowed.
type metaKey struct {
	t        reflect.Type
	schema   *god.TypeSchema
	readonly bool
}

// structMeta returns the metatable that exposes a pointer to a struct of type
// t to Lua: reads and writes go to the Go fields by JSON name. Metatables are
// built once per (type, schema, readonly) and cached.
func (rt *Runtime) structMeta(t reflect.Type, s *god.TypeSchema, readonly bool) *lua.LTable {
	key := metaKey{t, s, readonly}
	if mt, ok := rt.metas[key]; ok {
		return mt
	}
	L := rt.L
	fields := newFieldMap(t, s)
	label := t.Name()

	mt := L.NewTable()
	rt.metas[key] = mt
	mt.RawSetString("__index", L.NewFunction(func(L *lua.LState) int {
		rv := reflect.ValueOf(L.CheckUserData(1).Value).Elem()
		name := L.CheckString(2)
		f, ok := fields[name]
		if !ok {
			L.RaiseError("unknown field %s.%s", label, name)
		}
		L.Push(rt.goToLua(rv.FieldByIndex(f.index), f.schema, readonly))
		return 1
	}))
	mt.RawSetString("__newindex", L.NewFunction(func(L *lua.LState) int {
		if readonly {
			L.RaiseError("component %s is readonly", label)
		}
		rv := reflect.ValueOf(L.CheckUserData(1).Value).Elem()
		name := L.CheckString(2)
		f, ok := fields[name]
		if !ok {
			L.RaiseError("unknown field %s.%s", label, name)
		}
		if err := rt.assign(rv.FieldByIndex(f.index), f.schema, L.Get(3), "$."+name); err != nil {
			L.RaiseError("%s: %v", label, err)
		}
		return 0
	}))
	return mt
}

// objectBinding is everything Lua needs to know about one registered Game
// Object: its components with their metatables and runtime IDs.
type objectBinding struct {
	key    string
	schema god.Schema
	// comps are the class's components, parallel to schema.Components,
	// then every other registered component type: an object may hold any
	// of them (they are optional).
	comps []compBinding
	// byName maps a component's key ("gfx:Transform") to its index in
	// comps, and its bare name ("Transform") when that is unambiguous: a
	// class component's name when no other class component has it, another
	// registered component's name when no class component and no other
	// registered component has it.
	byName   map[string]int
	required []int // indexes into comps, in schema order
	meta     *lua.LTable
}

type compBinding struct {
	group    string
	name     string // type name, without the group
	key      string // "group:name"
	id       ecs.ComponentID
	optional bool
	readonly bool
	meta     *lua.LTable
	schema   *god.TypeSchema
	decode   func(json.RawMessage) (any, error)
}

// component is the god.Component the binding was made from, for the codec.
func (c compBinding) component() god.Component {
	return god.Component{Group: c.group, Name: c.name, RuntimeID: c.id, Optional: c.optional, Readonly: c.readonly, Schema: c.schema, Decode: c.decode}
}

// binding returns the (cached) binding of the object registered as
// "group:class".
func (rt *Runtime) binding(key string) (*objectBinding, error) {
	if b, ok := rt.objects[key]; ok {
		return b, nil
	}
	i := strings.LastIndex(key, ":") // groups may contain ':', classes may not
	if i < 0 {
		return nil, fmt.Errorf("unknown object %s", key)
	}
	s, ok := god.LookupSchema(key[:i], key[i+1:])
	if !ok {
		return nil, fmt.Errorf("unknown object %s", key)
	}

	b := &objectBinding{key: key, schema: s, byName: map[string]int{}}
	for idx, c := range s.Components {
		// Decoding null yields the zero value of the component type for any
		// generated decoder, which is how the Go type is discovered without
		// codegen.
		zero, err := c.Decode(json.RawMessage("null"))
		if err != nil {
			return nil, fmt.Errorf("object %s: component %s: %w", key, c.Key(), err)
		}
		t := reflect.TypeOf(zero)
		if t == nil || t.Kind() != reflect.Struct {
			return nil, fmt.Errorf("object %s: component %s: unsupported type %v (only struct components are scriptable)", key, c.Key(), t)
		}
		b.comps = append(b.comps, compBinding{
			group: c.Group, name: c.Name, key: c.Key(), id: c.RuntimeID, optional: c.Optional, readonly: c.Readonly,
			meta: rt.structMeta(t, c.Schema, c.Readonly), schema: c.Schema, decode: c.Decode,
		})
		b.byName[c.Key()] = idx
		if !c.Optional {
			b.required = append(b.required, idx)
		}
	}
	classIDs := map[ecs.ComponentID]bool{}
	for _, c := range s.Components {
		classIDs[c.RuntimeID] = true
	}
	for rc := range god.Components() {
		if _, inClass := b.byName[rc.Key()]; inClass || classIDs[rc.RuntimeID] {
			continue
		}
		zero, err := rc.Decode(json.RawMessage("null"))
		t := reflect.TypeOf(zero)
		if err != nil || t == nil || t.Kind() != reflect.Struct {
			continue // not scriptable; only the class's components must be
		}
		b.byName[rc.Key()] = len(b.comps)
		b.comps = append(b.comps, compBinding{
			group: rc.Group, name: rc.Name, key: rc.Key(), id: rc.RuntimeID, optional: true,
			meta: rt.structMeta(t, rc.Schema, false), schema: rc.Schema, decode: rc.Decode,
		})
	}
	// The bare name is a shortcut for scripts, valid only when unambiguous;
	// the class's components come first.
	classNames, extraNames := map[string]int{}, map[string]int{}
	for idx, c := range b.comps {
		if idx < len(s.Components) {
			classNames[c.name]++
		} else {
			extraNames[c.name]++
		}
	}
	for idx, c := range b.comps {
		if idx < len(s.Components) && classNames[c.name] == 1 ||
			idx >= len(s.Components) && classNames[c.name] == 0 && extraNames[c.name] == 1 {
			b.byName[c.name] = idx
		}
	}
	b.meta = rt.objectMeta(b)
	rt.objects[key] = b
	return b, nil
}

// requiredIDs returns the runtime IDs of the required components, in the
// order System queries them.
func (b *objectBinding) requiredIDs() []ecs.ComponentID {
	ids := make([]ecs.ComponentID, len(b.required))
	for i, idx := range b.required {
		ids[i] = b.comps[idx].id
	}
	return ids
}

// objectRef is the Go value behind an object userdata: which entity it is
// currently pointed at and the component pointers known for it.
type objectRef struct {
	b     *objectBinding
	world *ecs.World
	e     ecs.Entity

	// ptr[i] is the live pointer of component i when the caller already has
	// it (System fills the required ones from the query); nil means "look it
	// up on access".
	ptr []any

	// slots[i] is the userdata handed out for component i. It is created on
	// first access and re-pointed on every access, so a loop over many
	// entities allocates no userdata per entity.
	slots []*lua.LUserData
}

func newObjectRef(b *objectBinding, w *ecs.World) *objectRef {
	return &objectRef{b: b, world: w, ptr: make([]any, len(b.comps)), slots: make([]*lua.LUserData, len(b.comps))}
}

// component returns the live pointer of component i, or nil if e lacks it.
func (r *objectRef) component(i int) any {
	if p := r.ptr[i]; p != nil {
		return p
	}
	return r.world.GetCompOrNil(r.e, r.b.comps[i].id)
}

// objectMeta builds the metatable of object userdata: field access yields the
// component (or nil for an absent optional one).
func (rt *Runtime) objectMeta(b *objectBinding) *lua.LTable {
	L := rt.L
	mt := L.NewTable()
	mt.RawSetString("__index", L.NewFunction(func(L *lua.LState) int {
		ref := L.CheckUserData(1).Value.(*objectRef)
		name := L.CheckString(2)
		i, ok := b.byName[name]
		if !ok {
			L.RaiseError("unknown component %s.%s", b.key, name)
		}
		p := ref.component(i)
		if p == nil {
			L.Push(lua.LNil)
			return 1
		}
		slot := ref.slots[i]
		if slot == nil {
			slot = L.NewUserData()
			L.SetMetatable(slot, b.comps[i].meta)
			ref.slots[i] = slot
		}
		slot.Value = p
		L.Push(slot)
		return 1
	}))
	mt.RawSetString("__newindex", L.NewFunction(func(L *lua.LState) int {
		L.RaiseError("cannot assign component %s: set its fields, or use world.set", L.CheckString(2))
		return 0
	}))
	return mt
}

// newObject returns an object userdata for e. It allocates; System reuses one
// userdata for a whole loop instead.
func (rt *Runtime) newObject(b *objectBinding, e ecs.Entity) *lua.LUserData {
	ref := newObjectRef(b, rt.world)
	ref.e = e
	ud := rt.L.NewUserData()
	ud.Value = ref
	rt.L.SetMetatable(ud, b.meta)
	return ud
}

// hasRequired reports whether e has every required component of b.
func (b *objectBinding) hasRequired(w *ecs.World, e ecs.Entity) bool {
	for _, idx := range b.required {
		if !w.HasComp(e, b.comps[idx].id) {
			return false
		}
	}
	return true
}

// newEntityMeta builds the metatable of entity handles: comparable with ==
// and printable. Entity handles may be stored by scripts.
func newEntityMeta(L *lua.LState) *lua.LTable {
	mt := L.NewTable()
	mt.RawSetString("__eq", L.NewFunction(func(L *lua.LState) int {
		a, aok := L.CheckUserData(1).Value.(ecs.Entity)
		b, bok := L.CheckUserData(2).Value.(ecs.Entity)
		L.Push(lua.LBool(aok && bok && a == b))
		return 1
	}))
	mt.RawSetString("__tostring", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(fmt.Sprintf("entity(%v)", L.CheckUserData(1).Value)))
		return 1
	}))
	return mt
}

// pushEntity returns a new entity handle userdata for e.
func (rt *Runtime) entityUserdata(e ecs.Entity) *lua.LUserData {
	ud := rt.L.NewUserData()
	ud.Value = e
	rt.L.SetMetatable(ud, rt.entityMeta)
	return ud
}

// checkEntity reads argument n as an entity handle of this world, raising a
// Lua error for anything else or for a stale entity.
func (rt *Runtime) checkEntity(L *lua.LState, n int) ecs.Entity {
	e, ok := L.CheckUserData(n).Value.(ecs.Entity)
	if !ok {
		L.ArgError(n, "entity expected")
	}
	if !rt.world.IsAlive(e) {
		L.RaiseError("stale entity")
	}
	return e
}

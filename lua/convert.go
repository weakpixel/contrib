package ecslua

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/weakpixel/ecs"
	"github.com/weakpixel/engine/god"
)

const maxTableDepth = 64 // guards against cyclic Lua tables

// luaToAny converts a Lua value to the shapes encoding/json produces
// (nil, bool, float64, string, []any, map[string]any). The schema, if given,
// decides whether an empty table is an array or an object.
func luaToAny(v lua.LValue, s *god.TypeSchema, depth int) (any, error) {
	switch v := v.(type) {
	case *lua.LNilType:
		return nil, nil
	case lua.LBool:
		return bool(v), nil
	case lua.LNumber:
		return float64(v), nil
	case lua.LString:
		return string(v), nil
	case *lua.LUserData:
		if e, ok := v.Value.(ecs.Entity); ok && isEntityRef(s) {
			return luaRefID(e), nil
		}
		return nil, fmt.Errorf("unsupported value of type %s", v.Type())
	case *lua.LTable:
		if depth > maxTableDepth {
			return nil, errors.New("table is nested too deeply (cyclic?)")
		}
		return tableToAny(v, s, depth)
	default:
		return nil, fmt.Errorf("unsupported value of type %s", v.Type())
	}
}

func tableToAny(t *lua.LTable, s *god.TypeSchema, depth int) (any, error) {
	wantArray := s != nil && s.Type == "array"
	n := t.Len()
	if n > 0 || wantArray {
		// Sequence: every key must be 1..n.
		count := 0
		t.ForEach(func(lua.LValue, lua.LValue) { count++ })
		if count != n {
			return nil, errors.New("table mixes array and object keys")
		}
		var items *god.TypeSchema
		if s != nil {
			items = s.Items
		}
		out := make([]any, 0, n)
		for i := 1; i <= n; i++ {
			e, err := luaToAny(t.RawGetInt(i), items, depth+1)
			if err != nil {
				return nil, fmt.Errorf("[%d]: %w", i-1, err)
			}
			out = append(out, e)
		}
		return out, nil
	}

	out := map[string]any{}
	var firstErr error
	t.ForEach(func(k, v lua.LValue) {
		if firstErr != nil {
			return
		}
		ks, ok := k.(lua.LString)
		if !ok {
			firstErr = fmt.Errorf("expected string key, got %s", k.Type())
			return
		}
		var vs *god.TypeSchema
		if s != nil {
			if p := s.Properties[string(ks)]; p != nil {
				vs = p
			} else {
				vs = s.AdditionalProperties
			}
		}
		e, err := luaToAny(v, vs, depth+1)
		if err != nil {
			firstErr = fmt.Errorf("%s: %w", ks, err)
			return
		}
		out[string(ks)] = e
	})
	if firstErr == nil && s != nil {
		// Lua cannot store nil in a table, so a required field that may be
		// null is null when absent (the JSON form of an unset pointer).
		for _, name := range s.Required {
			if _, present := out[name]; !present && s.Properties[name] != nil && s.Properties[name].Nullable {
				out[name] = nil
			}
		}
	}
	return out, firstErr
}

// luaToJSON converts v to JSON, shaped by s.
func luaToJSON(v lua.LValue, s *god.TypeSchema) (json.RawMessage, error) {
	a, err := luaToAny(v, s, 0)
	if err != nil {
		return nil, err
	}
	return json.Marshal(a)
}

// anyToLua converts a value decoded by encoding/json into a Lua value.
func anyToLua(L *lua.LState, v any) lua.LValue {
	switch v := v.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(v)
	case float64:
		return lua.LNumber(v)
	case string:
		return lua.LString(v)
	case []any:
		t := L.CreateTable(len(v), 0)
		for _, e := range v {
			t.Append(anyToLua(L, e))
		}
		return t
	case map[string]any:
		t := L.CreateTable(0, len(v))
		for k, e := range v {
			t.RawSetString(k, anyToLua(L, e))
		}
		return t
	default:
		return lua.LNil
	}
}

// jsonToLua converts raw JSON into a Lua value (a copy).
func jsonToLua(L *lua.LState, raw json.RawMessage) (lua.LValue, error) {
	var a any
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, err
	}
	return anyToLua(L, a), nil
}

// goToLua reads the Go value v (a field of a component) as a Lua value.
// Plain-object struct fields become userdata onto the field; scalars are
// copied; everything else is copied through JSON. ro marks the result
// read-only.
func (rt *Runtime) goToLua(v reflect.Value, s *god.TypeSchema, ro bool) lua.LValue {
	L := rt.L
	if s != nil && s.XAsset != "" {
		return rt.assetToLua(v, s)
	}
	if isEntityRef(s) && v.Type() == entityType {
		return rt.entityToLua(v.Interface().(ecs.Entity))
	}
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return lua.LNumber(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return lua.LNumber(v.Uint())
	case reflect.Float32, reflect.Float64:
		return lua.LNumber(v.Float())
	case reflect.Bool:
		return lua.LBool(v.Bool())
	case reflect.String:
		return lua.LString(v.String())
	case reflect.Pointer:
		if v.IsNil() {
			return lua.LNil
		}
		if v.Elem().Kind() == reflect.Struct && isPlainObject(s) {
			return rt.structUserdata(v.Interface(), v.Type().Elem(), s, ro)
		}
		return rt.goToLua(v.Elem(), s, ro)
	case reflect.Struct:
		if isPlainObject(s) && v.CanAddr() {
			return rt.structUserdata(v.Addr().Interface(), v.Type(), s, ro)
		}
	}

	if hasEntityRef(s) {
		// Encode through the codec so entities become ids, then turn the
		// ids into handles.
		raw, err := god.EncodeComponent(rt.world, god.Component{Schema: s}, v.Interface(), luaRefs{rt.world})
		if err != nil {
			L.RaiseError("cannot read %s: %v", v.Type(), err)
		}
		out, err := jsonToLua(L, raw)
		if err != nil {
			L.RaiseError("cannot read %s: %v", v.Type(), err)
		}
		return rt.refsToLua(out, s)
	}
	raw, err := json.Marshal(v.Interface())
	if err != nil {
		L.RaiseError("cannot read %s: %v", v.Type(), err)
	}
	out, err := jsonToLua(L, raw)
	if err != nil {
		L.RaiseError("cannot read %s: %v", v.Type(), err)
	}
	return out
}

// structUserdata wraps ptr (a pointer to a struct of type t) in userdata.
func (rt *Runtime) structUserdata(ptr any, t reflect.Type, s *god.TypeSchema, ro bool) *lua.LUserData {
	ud := rt.L.NewUserData()
	ud.Value = ptr
	rt.L.SetMetatable(ud, rt.structMeta(t, s, ro))
	return ud
}

// assign stores the Lua value lv into dst, a settable field described by s.
// path is the schema path used in error messages ("$.Max"). Errors report
// every violation of a JSON-shaped value, like god.TypeSchema.Validate.
func (rt *Runtime) assign(dst reflect.Value, s *god.TypeSchema, lv lua.LValue, path string) error {
	if s != nil && s.XAsset != "" {
		return rt.assignAsset(dst, s, lv, path)
	}
	if isEntityRef(s) && dst.Type() == entityType {
		return rt.assignEntity(dst, lv, path)
	}
	switch dst.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		f, err := wantInteger(lv, s, path)
		if err != nil {
			return err
		}
		if f < -(1<<63) || f >= 1<<63 || dst.OverflowInt(int64(f)) {
			return fmt.Errorf("%s: %v is out of range for %s", path, f, dst.Type())
		}
		dst.SetInt(int64(f))
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		f, err := wantInteger(lv, s, path)
		if err != nil {
			return err
		}
		if f < 0 || f >= 1<<64 || dst.OverflowUint(uint64(f)) {
			return fmt.Errorf("%s: %v is out of range for %s", path, f, dst.Type())
		}
		dst.SetUint(uint64(f))
		return nil
	case reflect.Float32, reflect.Float64:
		n, ok := lv.(lua.LNumber)
		if !ok {
			return fmt.Errorf("%s: expected number, got %s", path, describe(lv))
		}
		if dst.OverflowFloat(float64(n)) {
			return fmt.Errorf("%s: %v is out of range for %s", path, float64(n), dst.Type())
		}
		dst.SetFloat(float64(n))
		return nil
	case reflect.Bool:
		b, ok := lv.(lua.LBool)
		if !ok {
			return fmt.Errorf("%s: expected boolean, got %s", path, describe(lv))
		}
		dst.SetBool(bool(b))
		return nil
	case reflect.String:
		str, ok := lv.(lua.LString)
		if !ok {
			return fmt.Errorf("%s: expected string, got %s", path, describe(lv))
		}
		dst.SetString(string(str))
		return nil
	case reflect.Pointer:
		if lv == lua.LNil {
			if !s.Nullable {
				return fmt.Errorf("%s: null is not allowed", path)
			}
			dst.SetZero()
			return nil
		}
		elem := reflect.New(dst.Type().Elem())
		if err := rt.assign(elem.Elem(), s, lv, path); err != nil {
			return err
		}
		dst.Set(elem)
		return nil
	}

	// Structs, slices, maps, arrays and values of custom encoding: copy
	// through JSON, validated as a whole.
	raw, err := luaToJSON(lv, s)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := s.Validate(raw); err != nil {
		return errors.New(rebase(err.Error(), path))
	}
	if hasEntityRef(s) {
		// Entity ids in raw become live entities through the codec.
		c := god.Component{Schema: s, Decode: func(raw json.RawMessage) (any, error) {
			ptr := reflect.New(dst.Type())
			err := json.Unmarshal(raw, ptr.Interface())
			return ptr.Elem().Interface(), err
		}}
		v, err := god.DecodeComponent(rt.world, c, raw, luaRefs{rt.world})
		if err != nil {
			return errors.New(rebase(err.Error(), path))
		}
		dst.Set(reflect.ValueOf(v))
		return nil
	}
	ptr := reflect.New(dst.Type())
	if err := json.Unmarshal(raw, ptr.Interface()); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	dst.Set(ptr.Elem())
	return nil
}

// wantInteger checks that lv is an integral number inside the schema's bounds.
func wantInteger(lv lua.LValue, s *god.TypeSchema, path string) (float64, error) {
	n, ok := lv.(lua.LNumber)
	if !ok {
		return 0, fmt.Errorf("%s: expected integer, got %s", path, describe(lv))
	}
	f := float64(n)
	if f != math.Trunc(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("%s: expected integer, got %v", path, f)
	}
	if (s.Minimum != nil && f < *s.Minimum) || (s.Maximum != nil && f > *s.Maximum) {
		return 0, fmt.Errorf("%s: %v is out of range", path, f)
	}
	return f, nil
}

// describe names the kind of a Lua value for error messages, like the "got"
// part of TypeSchema.Validate messages.
func describe(v lua.LValue) string {
	switch v := v.(type) {
	case *lua.LNilType:
		return "null"
	case lua.LNumber:
		return fmt.Sprint(float64(v))
	case *lua.LTable:
		return "table"
	default:
		return v.Type().String()
	}
}

// rebase rewrites the "$" roots of Validate's error lines to path, so an error
// about a nested value reads "$.Companion.Name: ..." instead of "$.Name: ...".
func rebase(msg, path string) string {
	lines := strings.Split(msg, "\n")
	for i, l := range lines {
		if rest, ok := strings.CutPrefix(l, "$"); ok {
			lines[i] = path + rest
		}
	}
	return strings.Join(lines, "\n")
}

// assetResolver is the world's god.AssetResolver, or nil.
func (rt *Runtime) assetResolver() god.AssetResolver {
	if p := ecs.GetResourceOrNil[god.AssetResolver](rt.world); p != nil {
		return *p
	}
	return nil
}

// assetToLua reads an asset field (a handle in Go) as its key string. An
// unset (zero) handle is "".
func (rt *Runtime) assetToLua(v reflect.Value, s *god.TypeSchema) lua.LValue {
	if v.IsZero() {
		return lua.LString("")
	}
	r := rt.assetResolver()
	if r == nil {
		rt.L.RaiseError("cannot read %s asset: no god.AssetResolver set", s.XAsset)
	}
	key, err := r.AssetKey(s.XAsset, v.Interface())
	if err != nil {
		rt.L.RaiseError("cannot read %s asset: %v", s.XAsset, err)
	}
	return lua.LString(key)
}

// assignAsset resolves the key string lv into the handle field dst. "" resets
// the field to the zero handle.
func (rt *Runtime) assignAsset(dst reflect.Value, s *god.TypeSchema, lv lua.LValue, path string) error {
	key, ok := lv.(lua.LString)
	if !ok {
		return fmt.Errorf("%s: expected string, got %s", path, describe(lv))
	}
	if key == "" {
		dst.SetZero()
		return nil
	}
	r := rt.assetResolver()
	if r == nil {
		return fmt.Errorf("%s: cannot resolve %s asset %q: no god.AssetResolver set", path, s.XAsset, string(key))
	}
	h, err := r.ResolveAsset(s.XAsset, string(key))
	switch {
	case err != nil:
		return fmt.Errorf("%s: resolve %s asset %q: %w", path, s.XAsset, string(key), err)
	case h == nil || !reflect.TypeOf(h).AssignableTo(dst.Type()):
		return fmt.Errorf("%s: resolver returned %T for %s asset %q, want %s", path, h, s.XAsset, string(key), dst.Type())
	}
	dst.Set(reflect.ValueOf(h))
	return nil
}

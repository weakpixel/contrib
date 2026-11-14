package ecslua_test

import (
	"fmt"
	"testing"

	ecslua "github.com/weakpixel/contrib/lua"
	"github.com/weakpixel/ecs"
	"github.com/weakpixel/engine/god"
)

// Pack exercises the value kinds that gametypes does not have: unsigned
// integers, slices, maps, nullable pointers to structs and nested structs.
type Pack struct {
	Count uint8
	Items []string
	Tags  map[string]int64
	Slot  *PackSlot
	Box   PackBox `json:"box"`
}

type PackSlot struct{ Name string }
type PackBox struct{ W, H float64 }

func registerPack(t *testing.T) *ecs.World {
	t.Helper()
	god.RegisterSchema(god.Schema{
		SchemaMeta: god.SchemaMeta{Group: "lua_test", Class: "Bag"},
		Components: []god.Component{{
			Group:     "lua_test",
			Name:      "Pack",
			RuntimeID: ecs.RegisterComp[Pack](),
			Decode:    god.DecodeFunc[Pack](),
			Schema: god.MustParseSchema(`{
				"type": "object",
				"properties": {
					"Count": {"type": "integer", "minimum": 0, "maximum": 255},
					"Items": {"type": "array", "items": {"type": "string"}, "nullable": true},
					"Tags":  {"type": "object", "additionalProperties": {"type": "integer", "format": "int64"}, "nullable": true},
					"Slot":  {"type": "object", "nullable": true, "properties": {"Name": {"type": "string"}}, "required": ["Name"]},
					"box":   {"type": "object", "properties": {"W": {"type": "number"}, "H": {"type": "number"}}, "required": ["W", "H"]}
				},
				"required": ["Count", "Items", "Tags", "Slot", "box"]
			}`),
		}},
	})
	return ecs.NewWorld()
}

func TestFieldKinds(t *testing.T) {
	world := registerPack(t)
	e := world.NewEntity(Pack{Count: 3, Items: []string{"a", "b"}, Tags: map[string]int64{"x": 1}, Slot: &PackSlot{Name: "s"}})
	var errs []error
	rt := ecslua.NewRuntime(world, ecslua.Options{OnError: func(_ string, err error) { errs = append(errs, err) }})
	defer rt.Close()

	run := func(t *testing.T, body string) {
		t.Helper()
		errs = nil
		f := &fixture{t: t, world: world, rt: rt}
		f.run(`return { object = "lua_test:Bag", update = function(dt, e, bag) local p = bag.Pack ` + body + ` end }`)
		f.errs = errs
	}
	check := func(t *testing.T, want string) {
		t.Helper()
		f := &fixture{t: t, errs: errs}
		if want == "" {
			f.noErr()
		} else {
			f.wantErr(want)
		}
	}

	t.Run("reads", func(t *testing.T) {
		run(t, `
			if p.Count ~= 3 or #p.Items ~= 2 or p.Items[2] ~= "b" or p.Tags.x ~= 1 or p.Slot.Name ~= "s" then error("bad read") end
			if p.box.W ~= 0 then error("box") end`)
		check(t, "")
	})
	t.Run("writes", func(t *testing.T) {
		run(t, `
			p.Count = 200
			p.Items = {"x", "y", "z"}
			p.Tags = {a = 1, b = 2}
			p.Slot = {Name = "new"}
			p.box.W = 4
			p.box = {W = 5, H = 6}`)
		check(t, "")
		got := ecs.GetComp[Pack](world, e)
		if got.Count != 200 || len(got.Items) != 3 || got.Tags["b"] != 2 || got.Slot.Name != "new" || got.Box != (PackBox{5, 6}) {
			t.Fatalf("pack after writes = %+v slot=%+v", got, got.Slot)
		}
	})
	t.Run("nullable pointer", func(t *testing.T) {
		run(t, `p.Slot = nil`)
		check(t, "")
		if ecs.GetComp[Pack](world, e).Slot != nil {
			t.Fatal("slot should be nil")
		}
		run(t, `if p.Slot ~= nil then error("not nil") end`)
		check(t, "")
	})

	errTests := []struct{ name, stmt, want string }{
		{"unsigned range", `p.Count = 256`, "$.Count: 256 is out of range"},
		{"unsigned negative", `p.Count = -1`, "$.Count: -1 is out of range"},
		{"array element type", `p.Items = {1}`, "$.Items[0]: expected string"},
		{"array not table", `p.Items = 5`, "$.Items: expected array, got number"},
		{"map value type", `p.Tags = {a = "x"}`, "$.Tags.a: expected integer"},
		{"struct missing field", `p.Slot = {}`, "$.Slot: missing required property \"Name\""},
		{"nested unknown field", `p.box = {W = 1, H = 2, Z = 3}`, "$.box: unknown property \"Z\""},
		{"mixed keys", `p.Items = {"a", x = 1}`, "mixes array and object keys"},
		{"function value", `p.Items = {print}`, "unsupported value"},
	}
	for _, tc := range errTests {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc.stmt)
			check(t, tc.want)
		})
	}
}

// Icon has an asset handle and a runtime field, both json:"-" as the
// generator requires.
type Icon struct {
	Handle  iconHandle `json:"-"`
	Layer   int
	Elapsed float64 `json:"-"`
}

type iconHandle struct{ N int }

// iconResolver maps "a" -> 1, "b" -> 2 and back.
type iconResolver struct{}

func (iconResolver) ResolveAsset(kind, path string) (any, error) {
	switch path {
	case "a":
		return iconHandle{1}, nil
	case "b":
		return iconHandle{2}, nil
	}
	return nil, fmt.Errorf("no asset %q", path)
}

func (iconResolver) AssetKey(kind string, h any) (string, error) {
	return string(rune('a' + h.(iconHandle).N - 1)), nil
}

func TestAssetFields(t *testing.T) {
	god.RegisterSchema(god.Schema{
		SchemaMeta: god.SchemaMeta{Group: "lua_test", Class: "Badge"},
		Components: []god.Component{{
			Group:     "lua_test",
			Name:      "Icon",
			RuntimeID: ecs.RegisterComp[Icon](),
			Decode:    god.DecodeFunc[Icon](),
			Schema: god.MustParseSchema(`{
				"type": "object",
				"properties": {
					"Handle": {"type": "string", "format": "asset", "x-asset": "sprite"},
					"Layer": {"type": "integer"},
					"Elapsed": {"type": "number", "x-runtime": true}
				},
				"required": ["Layer"]
			}`),
		}},
	})
	world := ecs.NewWorld()
	e := world.NewEntity(Icon{Handle: iconHandle{2}, Layer: 1})
	var errs []error
	rt := ecslua.NewRuntime(world, ecslua.Options{OnError: func(_ string, err error) { errs = append(errs, err) }})
	defer rt.Close()

	run := func(t *testing.T, body, want string) {
		t.Helper()
		errs = nil
		f := &fixture{t: t, world: world, rt: rt}
		f.run(`return { object = "lua_test:Badge", update = function(dt, e, b) local i = b.Icon ` + body + ` end }`)
		f.errs = errs
		if want == "" {
			f.noErr()
		} else {
			f.wantErr(want)
		}
	}

	t.Run("without resolver", func(t *testing.T) {
		run(t, `local _ = i.Handle`, "no god.AssetResolver")
		run(t, `i.Handle = "a"`, "no god.AssetResolver")
		run(t, `i.Handle = ""`, "") // clearing needs no resolver
		if ecs.GetComp[Icon](world, e).Handle != (iconHandle{}) {
			t.Fatal("handle not cleared")
		}
		world.SetComp(e, Icon{Handle: iconHandle{2}, Layer: 1})
	})

	var r god.AssetResolver = iconResolver{}
	ecs.SetResource(world, &r)

	t.Run("read as path", func(t *testing.T) {
		run(t, `if i.Handle ~= "b" then error("got " .. i.Handle) end`, "")
	})
	t.Run("write path", func(t *testing.T) {
		run(t, `i.Handle = "a"`, "")
		if got := ecs.GetComp[Icon](world, e).Handle; got != (iconHandle{1}) {
			t.Fatalf("handle = %+v", got)
		}
	})
	t.Run("unset reads empty", func(t *testing.T) {
		run(t, `i.Handle = ""  if i.Handle ~= "" then error("not empty") end`, "")
	})
	t.Run("runtime field is scriptable", func(t *testing.T) {
		run(t, `i.Elapsed = 1.5  if i.Elapsed ~= 1.5 then error("elapsed") end`, "")
		if ecs.GetComp[Icon](world, e).Elapsed != 1.5 {
			t.Fatal("elapsed not written")
		}
	})
	t.Run("set resolves keys", func(t *testing.T) {
		// world.set takes its handles from keys, like Deserialize.
		run(t, `world.set(e, "lua_test:Badge", "Icon", {Handle = "b", Layer = 7})`, "")
		if got := ecs.GetComp[Icon](world, e); got.Handle != (iconHandle{2}) || got.Layer != 7 {
			t.Fatalf("after world.set: %+v", got)
		}
		run(t, `world.set(e, "lua_test:Badge", "Icon", {Handle = "zzz", Layer = 7})`, `resolve sprite asset "zzz"`)
		run(t, `world.set(e, "lua_test:Badge", "Icon", {Layer = 7, Elapsed = 1})`, "$.Elapsed: runtime field")
	})
	t.Run("errors", func(t *testing.T) {
		run(t, `i.Handle = "zzz"`, `resolve sprite asset "zzz": no asset "zzz"`)
		run(t, `i.Handle = 3`, "$.Handle: expected string, got 3")
	})
}

type PartA struct{ N int }
type PartB struct{ N int }

// TestComponentKeys checks D9: a component is always reachable by its key
// ("group:name"), and by its bare name only when that is unique in the
// object.
func TestComponentKeys(t *testing.T) {
	schema := god.MustParseSchema(`{"type": "object", "properties": {"N": {"type": "integer"}}, "required": ["N"]}`)
	god.RegisterSchema(god.Schema{
		SchemaMeta: god.SchemaMeta{Group: "lua_test", Class: "Twin"},
		Components: []god.Component{
			{Group: "a", Name: "Part", Field: "A", RuntimeID: ecs.RegisterComp[PartA](), Decode: god.DecodeFunc[PartA](), Schema: schema},
			{Group: "b", Name: "Part", Field: "B", RuntimeID: ecs.RegisterComp[PartB](), Decode: god.DecodeFunc[PartB](), Schema: schema},
			{Group: "lua_test", Name: "Pack", Field: "Pack", Optional: true, RuntimeID: ecs.RegisterComp[Pack](), Decode: god.DecodeFunc[Pack](), Schema: god.MustParseSchema(`{"type": "object", "x-preserve-unknown-fields": true}`)},
		},
	})
	world := ecs.NewWorld()
	e := world.NewEntity(PartA{1}, PartB{2})
	var errs []error
	rt := ecslua.NewRuntime(world, ecslua.Options{OnError: func(_ string, err error) { errs = append(errs, err) }})
	defer rt.Close()
	run := func(body string) *fixture {
		errs = nil
		f := &fixture{t: t, world: world, rt: rt}
		f.run(`return { object = "lua_test:Twin", update = function(dt, e, o) ` + body + ` end }`)
		f.errs = errs
		return f
	}

	run(`if o["a:Part"].N ~= 1 or o["b:Part"].N ~= 2 then error("keys") end
		if o.Pack ~= nil or o["lua_test:Pack"] ~= nil then error("absent optional") end
		o["b:Part"].N = 5`).noErr()
	if got := ecs.GetComp[PartB](world, e).N; got != 5 {
		t.Fatalf("b:Part.N = %d, want 5", got)
	}
	run(`local _ = o.Part`).wantErr("unknown component lua_test:Twin.Part")
	run(`world.spawn("lua_test:Twin", {["a:Part"] = {N = 3}, ["b:Part"] = {N = 4}})`).noErr()
	run(`world.spawn("lua_test:Twin", {["a:Part"] = {N = 3}, ["b:Part"] = {N = 4}, Pack = {}, ["lua_test:Pack"] = {}})`).wantErr("component lua_test:Pack is given twice")
}

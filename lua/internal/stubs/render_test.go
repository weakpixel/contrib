package stubs

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/weakpixel/engine/god"
)

func TestRenderLuaGolden(t *testing.T) {
	checkGolden(t, "sample.lua.golden", renderLua(loadSample(t)))
}

func TestRenderLuaDeterministic(t *testing.T) {
	if renderLua(loadSample(t)) != renderLua(loadSample(t)) {
		t.Error("Lua output differs between runs")
	}
}

func TestClassNames(t *testing.T) {
	doc := loadSample(t)

	var comps []string
	for _, c := range doc.Components {
		comps = append(comps, componentClass(c))
	}
	// Classes are named by key, so two Transform types do not clash.
	want := []string{"god.ext.Stamp", "god.ext.Transform", "god.game.Companion", "god.game.Health", "god.game.Score", "god.game.Transform", "god.game.Weapon"}
	if !slices.Equal(comps, want) {
		t.Errorf("component classes = %v, want %v", comps, want)
	}

	tests := []struct {
		obj  object
		want string
	}{
		{object{Group: "game", Class: "Hero"}, "game.Hero"},
		{object{Group: "my/game.v2", Class: "Boss"}, "my_game_v2.Boss"},
	}
	for _, tt := range tests {
		if got := objectClass(&tt.obj); got != tt.want {
			t.Errorf("objectClass(%s) = %q, want %q", tt.obj.key(), got, tt.want)
		}
	}
}

func TestLuaType(t *testing.T) {
	parse := god.MustParseSchema
	tests := []struct {
		name, schema, want string
		nested             []string
	}{
		{"string", `{"type":"string"}`, "string", nil},
		{"bytes", `{"type":"string","format":"byte"}`, "string", nil},
		{"integer", `{"type":"integer","format":"int64"}`, "integer", nil},
		{"number", `{"type":"number"}`, "number", nil},
		{"boolean", `{"type":"boolean"}`, "boolean", nil},
		{"nullable", `{"type":"string","nullable":true}`, "string?", nil},
		{"array", `{"type":"array","items":{"type":"integer"}}`, "integer[]", nil},
		{"fixed array", `{"type":"array","items":{"type":"number"},"minItems":3,"maxItems":3}`, "number[]", nil},
		{"nullable items", `{"type":"array","items":{"type":"string","nullable":true}}`, "(string?)[]", nil},
		{"nullable array", `{"type":"array","nullable":true,"items":{"type":"string"}}`, "string[]?", nil},
		{"map", `{"type":"object","additionalProperties":{"type":"integer"}}`, "table<string, integer>", nil},
		{"map of arrays", `{"type":"object","additionalProperties":{"type":"array","items":{"type":"string"}}}`, "table<string, string[]>", nil},
		{"nested object", `{"type":"object","properties":{"A":{"type":"string"}},"required":["A"]}`, "N", []string{"N"}},
		{"array of objects", `{"type":"array","items":{"type":"object","properties":{"A":{"type":"string"}}}}`, "N[]", []string{"N"}},
		{"map of objects", `{"type":"object","additionalProperties":{"type":"object","properties":{"A":{"type":"string"}}}}`, "table<string, N>", []string{"N"}},
		{"empty object", `{"type":"object"}`, "table", nil},
		{"x-go-type", `{"x-go-type":"time.Time"}`, "any", nil},
		{"preserve unknown", `{"x-preserve-unknown-fields":true}`, "any", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var nested []luaNested
			if got := luaType(parse(tt.schema), "N", &nested); got != tt.want {
				t.Errorf("luaType = %q, want %q", got, tt.want)
			}
			var names []string
			for _, n := range nested {
				names = append(names, n.name)
			}
			if !slices.Equal(names, tt.nested) {
				t.Errorf("nested classes = %v, want %v", names, tt.nested)
			}
		})
	}
}

func TestLuaFieldRules(t *testing.T) {
	out := renderLua(loadSample(t))
	for _, want := range []string{
		"---@field Attachment? god.game.Companion.Attachment\n", // optional and nullable: one "?"
		"---@field Tags? string[]\n",
		"---@field Weapon? god.game.Weapon The weapon the hero carries, if any.\n",
		"---@field [\"game:Weapon\"] god.game.Weapon? The weapon the hero carries, if any.\n", // bracket form: nullable type
		"---@field Companion god.game.Companion (readonly)\n",
		"---@field [\"game:Transform\"] god.game.Transform Where the hero is. (embedded via Common.Base)\n",
		"---@overload fun(e: Entity, object: \"game:Hero\", component: \"Weapon\")\n", // only optionals are removable
		"---@overload fun(e: Entity, object: \"game:Hero\", component: \"game:Weapon\")\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q", want)
		}
	}
	if strings.Contains(out, `component: "Health")`) || strings.Contains(out, `component: "game:Health")`) {
		t.Error("a required component is offered to world.remove")
	}
}

func TestFieldName(t *testing.T) {
	for in, want := range map[string]string{"Health": "Health", "a_b1": "a_b1", "my-field": `["my-field"]`, "1a": `["1a"]`, "": `[""]`} {
		if got := fieldName(in); got != want {
			t.Errorf("fieldName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestWorldStubMatchesDocAPI keeps the stub's world functions equal to the
// `world.*` calls documented in doc/guide/world-api.md and registered by
// world-api.go.
func TestWorldStubMatchesDocAPI(t *testing.T) {
	want := slices.Clone(luaWorldFuncs)
	slices.Sort(want)

	doc, err := os.ReadFile(filepath.Join("..", "..", "doc", "guide", "world-api.md"))
	if err != nil {
		t.Fatal(err)
	}
	check := func(source string, names []string) {
		t.Helper()
		slices.Sort(names)
		names = slices.Compact(names)
		if !slices.Equal(names, want) {
			t.Errorf("%s declares world functions %v, the stub has %v", source, names, want)
		}
	}
	// Only the table rows of the page start with "| `world.", so the code
	// examples don't count.
	check("doc/guide/world-api.md", submatches(`\| `+"`"+`world\.(\w+)\(`, string(doc)))

	impl, err := os.ReadFile(filepath.Join("..", "..", "world-api.go"))
	if err != nil {
		t.Fatal(err)
	}
	check("world-api.go", submatches(`"(\w+)":\s+rt\.api\w+,`, string(impl)))
}

func submatches(pattern, s string) []string {
	var out []string
	for _, m := range regexp.MustCompile(pattern).FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	return out
}

var update = flag.Bool("update", false, "update golden files")

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}
	if string(want) != got {
		t.Errorf("%s differs from golden output (run with -update, review the diff):\n%s", name, got)
	}
}

func TestRender(t *testing.T) {
	out, err := Render("My Game", []string{filepath.Join("testdata", "sample.god.json")})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "---@meta\n-- My Game: LuaLS definitions") {
		t.Errorf("not a LuaLS meta file: %.80s", out)
	}
	if _, err := Render("x", []string{"missing.god.json"}); err == nil {
		t.Error("a missing input is not an error")
	}
}

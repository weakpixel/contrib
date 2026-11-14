package ecslua

import (
	"reflect"
	"slices"
	"strings"

	"github.com/weakpixel/engine/god"
)

// field is one JSON-visible field of a component struct.
type field struct {
	index  []int // path for reflect.Value.FieldByIndex (embedded structs are flattened)
	schema *god.TypeSchema
}

// newFieldMap maps the JSON names of struct type t to its fields, using the
// naming rules of encoding/json (json tags, "-", flattened embedded structs).
// s is the authority for which names exist: a Go field without a property in
// the schema is not exposed to Lua.
func newFieldMap(t reflect.Type, s *god.TypeSchema) map[string]*field {
	m := map[string]*field{}
	collectFields(t, nil, s, m)
	return m
}

func collectFields(t reflect.Type, prefix []int, s *god.TypeSchema, m map[string]*field) {
	for i := range t.NumField() {
		sf := t.Field(i)
		tag := sf.Tag.Get("json")
		name, _, _ := strings.Cut(tag, ",")
		if tag == "-" {
			// The generator keeps +game:asset and +game:runtime fields json:"-"
			// but lists them in the schema under their Go name.
			if ps := s.Properties[sf.Name]; sf.IsExported() && ps != nil && (ps.XAsset != "" || ps.XRuntime) {
				m[sf.Name] = &field{index: slices.Concat(prefix, []int{i}), schema: ps}
			}
			continue
		}
		path := slices.Concat(prefix, []int{i})
		if sf.Anonymous && name == "" && sf.Type.Kind() == reflect.Struct {
			collectFields(sf.Type, path, s, m) // flattened, like encoding/json
			continue
		}
		if !sf.IsExported() {
			continue
		}
		if name == "" {
			name = sf.Name
		}
		if ps := s.Properties[name]; ps != nil {
			m[name] = &field{index: path, schema: ps}
		}
	}
}

// isPlainObject reports whether s describes a struct that Lua can reach into
// field by field (as opposed to maps, arrays and values of custom encoding,
// which are copied through JSON).
func isPlainObject(s *god.TypeSchema) bool {
	return s != nil && s.Type == "object" && s.Properties != nil && s.GoType == "" && !s.PreserveUnknownFields
}

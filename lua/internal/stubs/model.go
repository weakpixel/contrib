package stubs

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"

	"github.com/weakpixel/engine/god"
)

// supportedVersion is the only "version" of a GOD file the stubs command reads.
const supportedVersion = 0

// generatorName is the "generator" of the merged GOD that weakpixel doc
// --format god writes. Directory walks skip such files, so a merged GOD in the
// specs folder is not read back as an input.
const generatorName = "weakpixel doc"

// document is the merged content of all loaded definition files.
type document struct {
	Title      string
	Groups     []*group     // sorted by name
	Components []*component // sorted by key
}

// group is the set of objects sharing a group name.
type group struct {
	Name    string
	Objects []*object // sorted by class
}

// object is one Game Object.
type object struct {
	Group, Class string
	Doc          string
	Source       string // definition file it came from
	Components   []objectComponent

	def god.Schema // as read
}

// objectComponent is a component as one object uses it.
type objectComponent struct {
	Field    string // Go field name in the object struct
	Optional bool
	Readonly bool
	Via      []string // embedded objects it is promoted through, outermost first
	Doc      string   // doc of the object field
	Comp     *component
}

// Embedded reports whether the component comes from an embedded object.
func (c objectComponent) Embedded() bool { return len(c.Via) > 0 }

// component is a component type, shared by every object that uses it.
type component struct {
	Group, Name string
	Key         string // "group:name", unique: used as title and anchor
	GoType      string
	Doc         string // type doc
	Source      string // definition file it came from first

	def god.ComponentSchema // as read
}

// godFile is a GOD file written by weakpixel gen.
type godFile struct {
	Version    int                   `json:"version"`
	Generator  string                `json:"generator"`
	Package    string                `json:"package,omitempty"`
	Components []god.ComponentSchema `json:"components"`
	Objects    []god.Schema          `json:"objects"`
}

// key identifies the object, "group:class".
func (o *object) key() string { return o.Group + ":" + o.Class }

// loadDocument reads and merges the definition files. Components merge by
// group:name and objects by group:class: identical definitions (docs
// ignored) collapse into one, a different definition under the same key is
// an error naming both files. All problems (unreadable file, bad JSON,
// unsupported version, conflicts, unknown components) are reported together.
func loadDocument(title string, files []string) (*document, error) {
	var errs []error
	objects := map[string]*object{}
	comps := map[string]*component{}

	var loaded []godFile
	var sources []string
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		var f godFile
		if err := json.Unmarshal(data, &f); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", file, err))
			continue
		}
		if f.Version != supportedVersion {
			errs = append(errs, fmt.Errorf("%s: unsupported version %d (want %d); regenerate with weakpixel gen", file, f.Version, supportedVersion))
			continue
		}
		loaded = append(loaded, f)
		sources = append(sources, file)
	}

	// Components first: objects of one file may use components of another.
	for i, f := range loaded {
		for _, cs := range f.Components {
			key := cs.Key()
			if prev := comps[key]; prev != nil {
				if !sameComponent(prev.def, cs) {
					errs = append(errs, fmt.Errorf("%s: component %s differs from the one in %s", sources[i], key, prev.Source))
				}
				continue
			}
			comps[key] = newComponent(cs, sources[i])
		}
	}
	for i, f := range loaded {
		for _, s := range f.Objects {
			o := &object{Group: s.Group, Class: s.Class, Doc: s.Doc, Source: sources[i], def: s}
			if prev := objects[o.key()]; prev != nil {
				if !sameObject(prev.def, s) {
					errs = append(errs, fmt.Errorf("%s: object %s differs from the one in %s", sources[i], o.key(), prev.Source))
				}
				continue
			}
			for _, c := range s.Components {
				comp := comps[c.Key()]
				if comp == nil {
					errs = append(errs, fmt.Errorf("%s: object %s uses component %s, which no input defines", sources[i], o.key(), c.Key()))
					continue
				}
				o.Components = append(o.Components, objectComponent{
					Field: c.Field, Optional: c.Optional, Readonly: c.Readonly, Via: c.EmbeddedVia, Doc: c.Doc, Comp: comp,
				})
			}
			objects[o.key()] = o
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return buildDocument(title, objects, comps), nil
}

// sameComponent compares two definitions of a component type, ignoring
// documentation.
func sameComponent(a, b god.ComponentSchema) bool {
	return a.GoType == b.GoType && reflect.DeepEqual(a.Schema.WithoutDescriptions(), b.Schema.WithoutDescriptions())
}

// sameObject compares two definitions of an object, ignoring documentation.
func sameObject(a, b god.Schema) bool {
	return a.GoType == b.GoType && slices.EqualFunc(a.Components, b.Components, func(x, y god.Component) bool {
		return x.Key() == y.Key() && x.Field == y.Field && x.Optional == y.Optional && x.Readonly == y.Readonly
	})
}

// newComponent builds the documentation of a component type.
func newComponent(cs god.ComponentSchema, source string) *component {
	c := &component{Group: cs.Group, Name: cs.Name, Key: cs.Key(), GoType: cs.GoType, Doc: cs.Doc, Source: source, def: cs}
	return c
}

// buildDocument sorts and cross-links the loaded objects and components.
func buildDocument(title string, objects map[string]*object, comps map[string]*component) *document {
	doc := &document{Title: title}

	groups := map[string]*group{}
	for _, o := range objects {
		g := groups[o.Group]
		if g == nil {
			g = &group{Name: o.Group}
			groups[o.Group] = g
			doc.Groups = append(doc.Groups, g)
		}
		g.Objects = append(g.Objects, o)
	}
	slices.SortFunc(doc.Groups, func(a, b *group) int { return cmp.Compare(a.Name, b.Name) })
	for _, g := range doc.Groups {
		slices.SortFunc(g.Objects, func(a, b *object) int { return cmp.Compare(a.Class, b.Class) })
	}

	for _, c := range comps {
		doc.Components = append(doc.Components, c)
	}
	slices.SortFunc(doc.Components, func(a, b *component) int { return cmp.Compare(a.Key, b.Key) })
	return doc
}

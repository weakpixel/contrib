// Package stubs writes LuaLS definition files for scripts from the GOD files
// of a project: one class per component and Game Object, an alias of the
// object keys, and the `world` API typed against them.
package stubs

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/weakpixel/engine/god"
)

// supportedVersion is the only "version" of a GOD file the stubs read.
const supportedVersion = 0

// godFileSuffix ends the name of every GOD file.
const godFileSuffix = ".god.json"

// document is the merged content of all loaded GOD files.
type document struct {
	Title      string
	Objects    []*object    // sorted by group, then class
	Components []*component // sorted by key
}

// object is one Game Object.
type object struct {
	Group, Class string
	Doc          string
	Components   []objectComponent

	source string     // GOD file it came from
	def    god.Schema // as read, to detect conflicts
}

// key identifies the object, "group:class".
func (o *object) key() string { return o.Group + ":" + o.Class }

// objectComponent is a component as one object uses it.
type objectComponent struct {
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
	Key         string // "group:name"
	Doc         string

	source string              // GOD file it came from first
	def    god.ComponentSchema // as read
}

// godFile is the part of a GOD file the stubs read.
type godFile struct {
	Version    int                   `json:"version"`
	Components []god.ComponentSchema `json:"components"`
	Objects    []god.Schema          `json:"objects"`
}

// load reads the GOD files named by inputs (files, or directories searched
// recursively for *.god.json) and merges them. Components merge by
// group:name and objects by group:class: identical definitions (docs
// ignored) collapse into one, a different definition under the same key is
// an error naming both files. All problems are reported together.
func load(title string, inputs []string) (*document, error) {
	files, err := collect(inputs)
	errs := []error{err} // nil is dropped by errors.Join; keep reading the files found
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
			errs = append(errs, fmt.Errorf("%s: unsupported version %d (want %d); regenerate it with go generate", file, f.Version, supportedVersion))
			continue
		}
		loaded = append(loaded, f)
		sources = append(sources, file)
	}

	// Components first: objects of one file may use components of another.
	comps := map[string]*component{}
	for i, f := range loaded {
		for _, cs := range f.Components {
			key := cs.Key()
			if prev := comps[key]; prev != nil {
				if !sameComponent(prev.def, cs) {
					errs = append(errs, fmt.Errorf("%s: component %s differs from the one in %s", sources[i], key, prev.source))
				}
				continue
			}
			comps[key] = &component{Group: cs.Group, Name: cs.Name, Key: key, Doc: cs.Doc, source: sources[i], def: cs}
		}
	}
	objects := map[string]*object{}
	for i, f := range loaded {
		for _, s := range f.Objects {
			o := &object{Group: s.Group, Class: s.Class, Doc: s.Doc, source: sources[i], def: s}
			if prev := objects[o.key()]; prev != nil {
				if !sameObject(prev.def, s) {
					errs = append(errs, fmt.Errorf("%s: object %s differs from the one in %s", sources[i], o.key(), prev.source))
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
					Optional: c.Optional, Readonly: c.Readonly, Via: c.EmbeddedVia, Doc: c.Doc, Comp: comp,
				})
			}
			objects[o.key()] = o
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	doc := &document{Title: title}
	for _, o := range objects {
		doc.Objects = append(doc.Objects, o)
	}
	slices.SortFunc(doc.Objects, func(a, b *object) int {
		return cmp.Or(cmp.Compare(a.Group, b.Group), cmp.Compare(a.Class, b.Class))
	})
	for _, c := range comps {
		doc.Components = append(doc.Components, c)
	}
	slices.SortFunc(doc.Components, func(a, b *component) int { return cmp.Compare(a.Key, b.Key) })
	return doc, nil
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

// collect expands inputs into the sorted, de-duplicated list of GOD files
// they name: a file is taken as is, a directory is searched recursively for
// *.god.json (skipping testdata, vendor, node_modules and directories
// starting with "." or "_"). A directory without GOD files is an error.
func collect(inputs []string) ([]string, error) {
	var errs []error
	seen := map[string]bool{}
	var files []string
	add := func(path string) {
		if abs, err := filepath.Abs(path); err == nil {
			if seen[abs] {
				return
			}
			seen[abs] = true
		}
		files = append(files, path)
	}
	for _, input := range inputs {
		info, err := os.Stat(input)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !info.IsDir() {
			add(input)
			continue
		}
		var found []string
		err = filepath.WalkDir(input, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if path != input && skippedDir(d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(d.Name(), godFileSuffix) {
				found = append(found, path)
			}
			return nil
		})
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if len(found) == 0 {
			errs = append(errs, fmt.Errorf("%s: no %s files", input, godFileSuffix))
		}
		slices.Sort(found)
		for _, f := range found {
			add(f)
		}
	}
	if len(inputs) == 0 {
		errs = append(errs, errors.New("no GOD files given"))
	}
	return files, errors.Join(errs...)
}

// skippedDir reports whether the directory walk ignores a directory below
// its root, following the go tool.
func skippedDir(name string) bool {
	switch name {
	case "vendor", "testdata", "node_modules":
		return true
	}
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

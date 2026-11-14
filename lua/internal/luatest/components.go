package luatest

import "github.com/weakpixel/ecs"

// Point is a plain struct component.
type Point struct{ X, Y float64 }

// Delta is a second plain struct component.
type Delta struct{ X, Y float64 }

// Counter is a plain component with integer and boolean fields.
type Counter struct {
	Current, Max int64
	Active       bool
}

// Optional is an optional component of TestObject.
type Optional struct {
	Float float64
	Int   int64
}

// Nested is a nested value in Label; it is not a component itself.
type Nested struct{ Name string }

// Label is a readonly (value) component with a nested pointer.
type Label struct {
	Name   string
	Nested *Nested
}

// Ref holds an entity reference.
type Ref struct {
	Target   ecs.Entity
	Distance float64
}

// Extra is a component only TestExtended requires.
type Extra struct{ Value int64 }

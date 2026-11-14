package game

// Position is where an entity is, in world units.
type Position struct {
	X, Y float64
}

// Velocity is how far an entity moves per second, in world units.
type Velocity struct {
	X, Y float64
}

// Health is an entity's hit points. The entity is spent when Current is 0.
type Health struct {
	Current, Max int64
}

// Label is a readonly (value) component: a name that scripts can read but
// not change.
type Label struct {
	Name string
}

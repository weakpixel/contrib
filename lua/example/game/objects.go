package game

// Drifter is a thing that drifts around a box and wears out.
// +game:object
type Drifter struct {
	// Where the drifter is.
	// +game:component
	Position *Position

	// Where it is heading.
	// +game:component
	Velocity *Velocity

	// How much wear it has left.
	// +game:component
	Health *Health

	// Its name, copied out on Get and read-only for scripts.
	// +game:component
	Label Label
}

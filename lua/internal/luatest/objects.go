package luatest

// TestObject is the object the ecslua tests run scripts against.
// +game:object
type TestObject struct {
	// +game:component
	Point *Point
	// +game:component
	Counter *Counter
	// +game:component
	Delta *Delta
	// +game:component
	Label Label
	// +game:component
	// +game:optional
	Optional *Optional
	// +game:component
	// +game:optional
	Ref *Ref
}

// TestExtended has every required TestObject component plus Extra, so
// TestObject scripts see it too.
// +game:object
type TestExtended struct {
	// +game:component
	Point *Point
	// +game:component
	Counter *Counter
	// +game:component
	Delta *Delta
	// +game:component
	Label Label
	// +game:component
	// +game:optional
	Optional *Optional
	// +game:component
	Extra *Extra
}

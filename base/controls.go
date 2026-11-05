package base

// The controls here are NSH's: each owns its presses, keyboard activation,
// disabled state, role and value for assistive technology, and holds no
// value of its own. The app passes the value and sets it from OnChange or
// OnPress, and styles every part, whose Checked, Indeterminate, Selected
// and Disabled styles follow the control.

// is reads an optional state.
func is(fn func() bool) bool { return fn != nil && fn() }

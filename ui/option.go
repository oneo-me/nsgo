package ui

import "github.com/nsgo-dev/nsgo/core"

// Option is one choice of Tabs or a Select. Label names it. In Tabs, Icon
// leads the label, Disabled turns it off, and Content shows in place of
// both, inheriting the tab's text style, with Label still naming the tab
// for assistive technology. In a Select, Leading goes before the label,
// in its menu's row and in the field while chosen, and Trailing at the
// end of its row, where a button's presses stay its own.
type Option[T comparable] struct {
	Value    T
	Label    string
	Icon     func() *core.SVG
	Disabled func() bool
	Content  core.Node
	Leading  core.Node
	Trailing core.Node
	TestID   string
}

package ui

import "github.com/oneo-me/nsgo/core"

// Option is one choice of Tabs or a Select. Label names it. In Tabs, Icon
// leads the label, Disabled turns it off, and Content shows in place of
// both, inheriting the tab's text style, with Label still naming the tab
// for assistive technology.
type Option[T comparable] struct {
	Value    T
	Label    string
	Icon     func() *core.SVG
	Disabled func() bool
	Content  core.Node
	TestID   string
}

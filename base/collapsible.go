package base

import "github.com/oneo-me/nsgo/core"

// Collapsible is a region whose Content expands and collapses: a "show
// more", a section of settings. Its children show either way, among them
// the app's trigger that flips Open (a Button with Expanded). Content is
// made the first time it opens and kept while closed, out of layout and
// hidden from assistive technology, so what is inside keeps its state.
type Collapsible struct {
	Open      func() bool
	Content   core.Node
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
	TestID    string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (c Collapsible) Render(cx *core.Scope) core.Node {
	opened := core.NewMemo(cx, func() bool { return is(c.Open) })
	made := false
	content := core.Show{When: func() bool { made = made || opened.Get(); return made }, Then: func(*core.Scope) core.Node {
		return core.View{
			StyleFunc: func() core.Style {
				if opened.Get() {
					return core.Style{}
				}
				return core.Style{Display: core.DisplayNone}
			},
			Children: []core.Node{c.Content}}
	}}
	return core.View{Class: c.Class, Style: c.Style, StyleFunc: c.StyleFunc, TestID: c.TestID, OnLayout: c.OnLayout, Tooltip: c.Tooltip,
		Children: append(append([]core.Node(nil), c.Children...), content)}
}

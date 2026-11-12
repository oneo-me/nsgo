package base

import "github.com/nsgo-dev/nsgo/core"

// ColorEyeDropper is a button picking a color on the screen with the
// system's sampler, for the picker around it and OnPick; disabled where
// the platform has none (core.Env.CanPickScreenColor).
type ColorEyeDropper struct {
	Disabled  func() bool
	OnPick    func(color core.Color)
	Children  []core.Node
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Label     string
	TestID    string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (e ColorEyeDropper) Render(cx *core.Scope) core.Node {
	picker := colorKey.Use(cx)
	picking := false
	env := cx.Env()
	disabled := func() bool {
		return is(e.Disabled) || !env.CanPickScreenColor() || picker != nil && picker.isDisabled()
	}
	return core.View{Class: e.Class, Style: e.Style, StyleFunc: e.StyleFunc, Disabled: disabled, Accessible: true, Role: core.RoleButton, Label: e.Label,
		TestID: e.TestID, OnLayout: e.OnLayout, Tooltip: e.Tooltip, Children: e.Children,
		OnPress: func(core.PressEvent) {
			if picking || core.Untracked(cx, disabled) {
				return
			}
			picking = true
			cx.PickScreenColor(func(c core.Color, ok bool) {
				picking = false
				if !ok {
					return
				}
				if picker != nil && !core.Untracked(cx, picker.isDisabled) {
					picker.set(picker.now().follow(c))
					picker.end()
				}
				if e.OnPick != nil {
					e.OnPick(c)
				}
			})
		}}
}

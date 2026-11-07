package base

import (
	"strconv"

	"github.com/oneo-me/nsgo/core"
)

// Rating is a score from 0 to Max, 5 when 0, in stars the app draws. A
// press on a star gives that score, or 0 when it is the score already, and
// on desktops the pointer previews the score under it. Assistive
// technology reads one adjustable element, and it is a Tab stop the arrow
// keys adjust.
type Rating struct {
	Value    func() int
	OnChange func(value int)
	Max      int
	// Readonly shows the score and takes nothing.
	Readonly bool
	Disabled func() bool
	// ValueText is what assistive technology reads as the value, the
	// number when nil.
	ValueText func(value int) string
	// Star draws star i, from 1, which filled says is under the score.
	Star      func(i int, filled func() bool) core.Node
	StarClass string
	StarStyle core.Style
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
func (r Rating) Render(cx *core.Scope) core.Node {
	env := cx.Env()
	onDesktop := core.Untracked(cx, env.Desktop)
	limit := r.Max
	if limit <= 0 {
		limit = 5
	}
	value := func() int {
		if r.Value == nil {
			return 0
		}
		return min(max(r.Value(), 0), limit)
	}
	locked := func() bool { return r.Readonly || is(r.Disabled) }
	set := func(v int) {
		if v = min(max(v, 0), limit); v != core.Untracked(cx, value) && !core.Untracked(cx, locked) && r.OnChange != nil {
			r.OnChange(v)
		}
	}
	by := func(d int) func() { return func() { set(core.Untracked(cx, value) + d) } }
	hover := core.NewSignal(cx, 0)
	shown := func() int {
		if h := hover.Get(); h > 0 && !locked() {
			return h
		}
		return value()
	}
	var stars []core.Node
	for i := 1; i <= limit; i++ {
		v := core.View{Class: r.StarClass, Style: r.StarStyle}
		if r.Star != nil {
			v.Children = []core.Node{r.Star(i, func() bool { return i <= shown() })}
		}
		if r.TestID != "" {
			v.TestID = r.TestID + "-" + strconv.Itoa(i)
		}
		if !r.Readonly {
			v.NoFocus = true
			v.OnPress = func(core.PressEvent) {
				if core.Untracked(cx, value) == i {
					set(0)
				} else {
					set(i)
				}
			}
			if onDesktop {
				v.OnMouseEnter = func(core.MouseEvent) { hover.Set(i) }
				v.OnMouseLeave = func(core.MouseEvent) { hover.Set(0) }
			}
		}
		stars = append(stars, v)
	}
	focused := core.NewSignal(cx, false)
	stars = append(stars, bindWhile(focused.Get, core.KeyAuto, true,
		controlKey{"right", true, by(1)}, controlKey{"up", true, by(1)}, controlKey{"left", true, by(-1)}, controlKey{"down", true, by(-1)},
		controlKey{"home", false, func() { set(0) }}, controlKey{"end", false, func() { set(limit) }}))
	view := core.View{Class: r.Class, TestID: r.TestID, Accessible: true, Role: core.RoleAdjustable, Label: r.Label, Disabled: r.Disabled, OnLayout: r.OnLayout, Tooltip: r.Tooltip,
		AccessibilityValue: func() core.AccessibilityValue {
			v := core.AccessibilityValue{Min: 0, Max: float32(limit), Now: float32(value())}
			if r.ValueText != nil {
				v.Text = r.ValueText(value())
			}
			return v
		},
		Style: r.Style, StyleFunc: r.StyleFunc, Children: stars}
	if !r.Readonly {
		view.AccessibilityActions = []core.AccessibilityAction{{Name: core.ActionIncrement}, {Name: core.ActionDecrement}}
		view.OnAccessibilityAction = func(name string) {
			switch name {
			case core.ActionIncrement:
				by(1)()
			case core.ActionDecrement:
				by(-1)()
			}
		}
		if onDesktop {
			view.OnPress = func(core.PressEvent) {}
			view.OnFocus = func() { focused.Set(true) }
			view.OnBlur = func() { focused.Set(false) }
		}
	}
	return view
}

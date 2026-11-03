package base

import (
	"math"

	"github.com/oneo-me/nsgo/core"
)

// The controls here are NSH's: each owns its presses, keyboard activation,
// disabled state, role and value for assistive technology, and holds no
// value of its own. The app passes the value and sets it from OnChange or
// OnPress, and styles every part, whose Checked, Indeterminate, Selected
// and Disabled styles follow the control.

// is reads an optional state.
func is(fn func() bool) bool { return fn != nil && fn() }

// Checkbox is checked, unchecked or indeterminate: an option, an item of a
// selection, a "select all". Its parts are the indicator, the box holding
// the app's Indicator mark, then its children, the label assistive
// technology reads.
type Checkbox struct {
	Checked func() bool
	// Indeterminate is neither checked nor unchecked, over Checked while
	// it holds; a press then asks for checked.
	Indeterminate func() bool
	Disabled      func() bool
	// OnChange receives the value a press asks for: true from unchecked
	// and indeterminate, false from checked. The checkbox shows what
	// Checked returns.
	OnChange func(checked bool)
	// Role is what it is for assistive technology; RoleCheckbox when 0.
	Role core.Role

	Class              string
	Style              core.Style
	StyleFunc          func() core.Style
	IndicatorClass     string
	IndicatorStyle     core.Style
	IndicatorStyleFunc func() core.Style
	// Indicator is the mark inside the indicator, which the app shows by
	// the value it owns.
	Indicator core.Node
	Children  []core.Node
	Label     string
	Hint      string
	TestID    string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (c Checkbox) Render(*core.Scope) core.Node {
	role := c.Role
	if role == core.RoleNone {
		role = core.RoleCheckbox
	}
	// Indeterminate wins over checked, as gpui-base's CheckboxState holds
	// one of the three.
	shows := func() core.PartStates {
		mixed := is(c.Indeterminate)
		return core.PartStates{Checked: is(c.Checked) && !mixed, Indeterminate: mixed, Disabled: is(c.Disabled)}
	}
	var mark []core.Node
	if c.Indicator != nil {
		mark = []core.Node{c.Indicator}
	}
	return core.View{Class: c.Class, Style: c.Style, StyleFunc: c.StyleFunc, Role: role, Label: c.Label, Hint: c.Hint,
		TestID: c.TestID, OnLayout: c.OnLayout, Tooltip: c.Tooltip, Disabled: c.Disabled, Indeterminate: c.Indeterminate,
		Checked: func() bool { return is(c.Checked) && !is(c.Indeterminate) },
		OnPress: func(core.PressEvent) {
			if c.OnChange != nil {
				c.OnChange(is(c.Indeterminate) || !is(c.Checked))
			}
		},
		Children: append([]core.Node{core.View{Class: c.IndicatorClass, Style: c.IndicatorStyle, StyleFunc: c.IndicatorStyleFunc,
			PartStates: shows, Children: mark}}, c.Children...)}
}

// Radio is one option of a set the user picks exactly one of, in a
// RadioGroup with the others. Its children are the app's indicator and
// label; a GroupStyle under Style.Group follows the radio's Checked.
type Radio struct {
	Checked  func() bool
	Disabled func() bool
	// OnChange receives true when an unchecked radio is pressed: the app
	// checks it and unchecks the others. A checked radio reports nothing,
	// as a radio cannot uncheck itself.
	OnChange  func(checked bool)
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
	Label     string
	Hint      string
	TestID    string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (r Radio) Render(*core.Scope) core.Node {
	return core.View{Class: r.Class, Style: r.Style, StyleFunc: r.StyleFunc, Role: core.RoleRadio, Label: r.Label, Hint: r.Hint, TestID: r.TestID, OnLayout: r.OnLayout, Tooltip: r.Tooltip,
		// Checked and selected both: some screen readers read one, some
		// the other, as gpui-base's Radio states toggled and selected.
		Disabled: r.Disabled, Checked: r.Checked, Selected: r.Checked,
		OnPress: func(core.PressEvent) {
			if r.OnChange != nil && !is(r.Checked) {
				r.OnChange(true)
			}
		},
		Children: r.Children}
}

// RadioGroup holds a set of Radios, which assistive technology hears as
// one set. It holds no value: each radio is told whether it is checked.
type RadioGroup struct {
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
	Label     string
	TestID    string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (g RadioGroup) Render(*core.Scope) core.Node {
	return core.View{Class: g.Class, Style: g.Style, StyleFunc: g.StyleFunc, Role: core.RoleRadioGroup, Label: g.Label, TestID: g.TestID, OnLayout: g.OnLayout, Tooltip: g.Tooltip,
		Children: g.Children}
}

// Switch is on or off and takes effect at once, as a setting. Its parts
// are a track and a thumb inside it, then its children, the label.
type Switch struct {
	Checked  func() bool
	Disabled func() bool
	// OnChange receives the value a press asks for, the opposite of
	// Checked's.
	OnChange func(checked bool)

	Class          string
	Style          core.Style
	StyleFunc      func() core.Style
	TrackClass     string
	TrackStyle     core.Style
	TrackStyleFunc func() core.Style
	ThumbClass     string
	ThumbStyle     core.Style
	ThumbStyleFunc func() core.Style
	Children       []core.Node
	Label          string
	Hint           string
	TestID         string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (s Switch) Render(*core.Scope) core.Node {
	shows := func() core.PartStates { return core.PartStates{Checked: is(s.Checked), Disabled: is(s.Disabled)} }
	thumb := core.View{Class: s.ThumbClass, Style: s.ThumbStyle, StyleFunc: s.ThumbStyleFunc, PartStates: shows}
	track := core.View{Class: s.TrackClass, Style: s.TrackStyle, StyleFunc: s.TrackStyleFunc, PartStates: shows,
		Children: []core.Node{thumb}}
	return core.View{Class: s.Class, Style: s.Style, StyleFunc: s.StyleFunc, Role: core.RoleSwitch, Label: s.Label, Hint: s.Hint, TestID: s.TestID, OnLayout: s.OnLayout, Tooltip: s.Tooltip,
		Disabled: s.Disabled, Checked: s.Checked,
		OnPress: func(core.PressEvent) {
			if s.OnChange != nil {
				s.OnChange(!is(s.Checked))
			}
		},
		Children: append([]core.Node{track}, s.Children...)}
}

// Tab is one tab of a tab list, in Tabs with the others. The app keeps
// which is selected and shows its content beside the list.
type Tab struct {
	Selected func() bool
	Disabled func() bool
	// OnPress runs on every press, the selected tab's included.
	OnPress   func(core.PressEvent)
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
	Label     string
	Hint      string
	TestID    string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (t Tab) Render(*core.Scope) core.Node {
	press := t.OnPress
	if press == nil {
		press = func(core.PressEvent) {}
	}
	return core.View{Class: t.Class, Style: t.Style, StyleFunc: t.StyleFunc, Role: core.RoleTab, Label: t.Label, Hint: t.Hint, TestID: t.TestID, OnLayout: t.OnLayout, Tooltip: t.Tooltip,
		Disabled: t.Disabled, Selected: t.Selected, OnPress: press, Children: t.Children}
}

// Tabs is the tab list around a set of Tabs, which assistive technology
// hears as one list. It holds no value and no content.
type Tabs struct {
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
	Label     string
	TestID    string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (t Tabs) Render(*core.Scope) core.Node {
	return core.View{Class: t.Class, Style: t.Style, StyleFunc: t.StyleFunc, Role: core.RoleTabList, Label: t.Label, TestID: t.TestID, OnLayout: t.OnLayout, Tooltip: t.Tooltip,
		Children: t.Children}
}

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

// Progress is a linear progress bar: how far a task of a known extent is,
// or that one of an unknown extent is under way. Its parts are a track
// and an indicator inside it, as wide as Value's share of the track while
// determinate, then its children, which assistive technology reads.
type Progress struct {
	// Value is a percentage from 0 to 100, clamped; NaN is none yet.
	Value func() float32
	// Indeterminate leaves the indicator's width and motion to the app,
	// and reports no value.
	Indeterminate      func() bool
	Class              string
	Style              core.Style
	StyleFunc          func() core.Style
	TrackClass         string
	TrackStyle         core.Style
	TrackStyleFunc     func() core.Style
	IndicatorClass     string
	IndicatorStyle     core.Style
	IndicatorStyleFunc func() core.Style
	Children           []core.Node
	Label              string
	TestID             string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// progressMax is the range of Progress.Value, gpui-base's percentage.
const progressMax = 100

// Render implements core.Node.
func (p Progress) Render(*core.Scope) core.Node {
	value := func() float32 {
		if p.Value == nil {
			return 0
		}
		v := p.Value()
		if math.IsNaN(float64(v)) {
			return 0
		}
		return min(max(v, 0), progressMax)
	}
	// The app's width wins, as for an indeterminate bar's.
	indicator := core.View{Class: p.IndicatorClass, StyleFunc: func() core.Style {
		var s core.Style
		if !is(p.Indeterminate) {
			s.Width = core.Pct(value())
		}
		s = s.Merge(p.IndicatorStyle)
		if p.IndicatorStyleFunc != nil {
			s = s.Merge(p.IndicatorStyleFunc())
		}
		return s
	}}
	return core.View{Class: p.Class, Style: p.Style, StyleFunc: p.StyleFunc, Role: core.RoleProgressBar, Label: p.Label, TestID: p.TestID, OnLayout: p.OnLayout, Tooltip: p.Tooltip,
		Accessible: true,
		AccessibilityValue: func() core.AccessibilityValue {
			if is(p.Indeterminate) {
				return core.AccessibilityValue{Min: 0, Max: progressMax, NoNow: true}
			}
			return core.AccessibilityValue{Min: 0, Max: progressMax, Now: value()}
		},
		Children: append([]core.Node{core.View{Class: p.TrackClass, Style: p.TrackStyle, StyleFunc: p.TrackStyleFunc,
			Children: []core.Node{indicator}}}, p.Children...)}
}

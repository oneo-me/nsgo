package base

import (
	"reflect"
	"slices"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

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
func (t Tab) Render(cx *core.Scope) core.Node {
	press := t.OnPress
	if press == nil {
		press = func(core.PressEvent) {}
	}
	return rovingView(cx, core.View{Class: t.Class, Style: t.Style, StyleFunc: t.StyleFunc, Role: core.RoleTab, Label: t.Label, Hint: t.Hint, TestID: t.TestID, OnLayout: t.OnLayout, Tooltip: t.Tooltip,
		Disabled: t.Disabled, Selected: t.Selected, OnPress: press, Children: t.Children}, t.Selected, func() { press(core.PressEvent{}) })
}

// tabsIndicator is how a tab list's indicator moves to another tab:
// gpui-component's tab bar's, a little springy.
var tabsIndicator = core.LayoutTransition{Curve: core.LayoutSpring, Damping: 0.85, Duration: 362 * time.Millisecond}

// Tabs is the tab list around a set of Tabs, which assistive technology
// hears as one list and the keyboard focus as one Tab stop: the selected
// tab, else the first enabled one. Left and Up, Right and Down move to the
// previous and next enabled tab, around the ends, Home and End to the
// first and last, and the tab reached reports a press. It holds no value
// and no content.
type Tabs struct {
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	// IndicatorClass and IndicatorStyle draw the list's indicator, a view
	// behind the tabs and absolutely positioned among them, with neither
	// no indicator. Along the list's main axis it spans the selected tab,
	// inside its own margins on that axis, and across it they place it;
	// a change of tab moves it there natively. It is hidden while no tab
	// is selected and takes no presses or focus.
	IndicatorClass string
	IndicatorStyle core.Style
	Children       []core.Node
	Label          string
	TestID         string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (t Tabs) Render(cx *core.Scope) core.Node {
	group := newRoving(cx, true)
	children := append(slices.Clip(t.Children), group.keys())
	if t.IndicatorClass != "" || !reflect.ValueOf(t.IndicatorStyle).IsZero() {
		children = append([]core.Node{t.indicator(group)}, children...)
	}
	return core.View{Class: t.Class, Style: t.Style, StyleFunc: t.StyleFunc, Role: core.RoleTabList, Label: t.Label, TestID: t.TestID, OnLayout: t.OnLayout, Tooltip: t.Tooltip,
		Children: children}
}

// indicator is the view on the selected tab.
func (t Tabs) indicator(group *rovingGroup) core.Node {
	style := core.Style{Position: core.Absolute}.Merge(t.IndicatorStyle)
	return core.View{Class: t.IndicatorClass, Style: style, PointerEvents: core.PointerEventsNone,
		AccessibilityElementsHidden: func() bool { return true },
		StyleFunc: func() core.Style {
			if group.current() == nil {
				return core.Style{Display: core.DisplayNone}
			}
			return core.Style{}
		},
		Follow: group.current, FollowTransition: &tabsIndicator}
}

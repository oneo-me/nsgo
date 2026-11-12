// Package base holds NSGo's built-in unstyled components: controls,
// disclosure, progress, popups, menus, dialogs, sheets, navigation, safe
// areas, resizable panels, a dock and the keyboard. They own behavior and
// accessibility and take every visual choice from the app.
package base

import "github.com/nsgo-dev/nsgo/core"

// Button is a pressable container with the button role. Its look is the
// style and children the app gives it, its states included.
type Button struct {
	OnPress     func(core.PressEvent)
	OnLongPress func(core.PressEvent)
	// OnPressDown and OnPressUp follow a press going down on the button
	// and ending, released, moved away or cancelled, for the app's own
	// press feedback.
	OnPressDown func(core.PressEvent)
	OnPressUp   func(core.PressEvent)
	// Disabled stops presses and is reported to assistive technology.
	Disabled func() bool
	// Selected marks a toggled trigger or the current item: assistive
	// technology says so, and Style.Selected applies.
	Selected func() bool
	// Expanded makes the button expandable, as a disclosure, and expanded
	// while it returns true.
	Expanded func() bool
	// Role is what it is for assistive technology; RoleButton when 0.
	Role core.Role
	// Class is Tailwind CSS classes under Style, as core.View's, as each
	// part's class is under its style.
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Label     string
	LabelFunc func() string
	// Hint says what pressing the button does, for assistive technology.
	Hint string
	// Tooltip shows above the button once the pointer rests over it on
	// desktops, as core.View's.
	Tooltip core.Node
	TestID  string
	// HitSlop is how far outside the button presses count.
	HitSlop core.Edges
	// NoFocus keeps it out of the Tab stops, for a button of a row the
	// keyboard reaches another way.
	NoFocus bool
	// AccessibilityElementsHidden hides it from assistive technology while
	// it returns true, as a leaf's chevron.
	AccessibilityElementsHidden func() bool
	// OnLayout is its view's, as a core.View's.
	OnLayout func(core.Bounds)
	// Desktop holds the props only desktops have, as a core.View's.
	Desktop  core.DesktopViewProps
	Children []core.Node
}

// Render implements core.Node.
func (b Button) Render(cx *core.Scope) core.Node {
	role := b.Role
	if role == core.RoleNone {
		role = core.RoleButton
	}
	return barItem(cx, core.View{
		Class:                       b.Class,
		Style:                       b.Style,
		StyleFunc:                   b.StyleFunc,
		Children:                    b.Children,
		OnPress:                     b.OnPress,
		OnLongPress:                 b.OnLongPress,
		OnPressDown:                 b.OnPressDown,
		OnPressUp:                   b.OnPressUp,
		Disabled:                    b.Disabled,
		Selected:                    b.Selected,
		Expanded:                    b.Expanded,
		Role:                        role,
		Label:                       b.Label,
		LabelFunc:                   b.LabelFunc,
		Desktop:                     b.Desktop,
		Tooltip:                     b.Tooltip,
		Hint:                        b.Hint,
		HitSlop:                     b.HitSlop,
		NoFocus:                     b.NoFocus,
		AccessibilityElementsHidden: b.AccessibilityElementsHidden,
		TestID:                      b.TestID, OnLayout: b.OnLayout,
	}, nil)
}

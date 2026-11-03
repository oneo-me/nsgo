package driver

// AccessibilityValue is a range's or a textual element's value
// (accessibilityValue). The zero value is no value.
type AccessibilityValue struct {
	// Min, Max and Now are a range, when Max is above Min: a slider's or
	// a progress bar's.
	Min, Max, Now float32
	// NoNow leaves a range without its current value, as an indeterminate
	// progress bar's.
	NoNow bool
	// Text is read instead of Now, or alone without a range.
	Text string
}

// IsRange reports whether v is a range.
func (v AccessibilityValue) IsRange() bool { return v.Max > v.Min }

// AccessibilityAction is an action assistive technology offers on an
// element; OnAccessibilityAction receives its Name.
type AccessibilityAction struct {
	// Name is a standard action (ActionActivate and the others), which
	// the screen reader performs with its own gesture or key, or a custom
	// action's name.
	Name string
	// Label is what a custom action is called in the screen reader's
	// actions menu; Name when empty.
	Label string
}

// Standard accessibility actions.
const (
	// ActionActivate replaces the press a screen reader's double tap or
	// VO-Space makes.
	ActionActivate = "activate"
	// ActionIncrement and ActionDecrement adjust an adjustable element:
	// a swipe up or down on iOS, TalkBack's volume keys, VO-Up and Down.
	ActionIncrement = "increment"
	ActionDecrement = "decrement"
	// ActionLongPress replaces a long press (Android).
	ActionLongPress = "longpress"
	// ActionExpand and ActionCollapse change an expandable element
	// (Android).
	ActionExpand   = "expand"
	ActionCollapse = "collapse"
	// ActionEscape is VoiceOver's two-finger scrub on iOS, AXCancel on
	// macOS and Escape on the web.
	ActionEscape = "escape"
	// ActionMagicTap is VoiceOver's two-finger double tap (iOS).
	ActionMagicTap = "magicTap"
)

// IsStandardAction reports whether name is one of the standard actions,
// which platforms map to their own gestures rather than list as custom
// actions.
func IsStandardAction(name string) bool {
	switch name {
	case ActionActivate, ActionIncrement, ActionDecrement, ActionLongPress,
		ActionExpand, ActionCollapse, ActionEscape, ActionMagicTap:
		return true
	}
	return false
}

// Accessible says whether a view is one element for assistive technology.
type Accessible uint8

const (
	// AccessibleAuto leaves it to the platform: views with a role, a
	// label or a press handler are elements.
	AccessibleAuto Accessible = iota
	// AccessibleYes makes the view one element: its descendants are not
	// exposed separately, and without a label it reads their text.
	AccessibleYes
	// AccessibleNo makes the view no element; its descendants still are.
	AccessibleNo
)

// LiveRegion says how screen readers announce changes of a view's label
// and text (accessibilityLiveRegion). Android and the web announce them
// natively; macOS and iOS announce a changed label or text themselves.
type LiveRegion uint8

const (
	LiveRegionNone LiveRegion = iota
	// LiveRegionPolite is announced when the user is idle.
	LiveRegionPolite
	// LiveRegionAssertive interrupts what is being read.
	LiveRegionAssertive
)

// HasAction reports whether p offers the action named name.
func (p *ViewProps) HasAction(name string) bool {
	for _, a := range p.ActionList() {
		if a.Name == name {
			return true
		}
	}
	return false
}

// ActionList returns p's actions.
func (p *ViewProps) ActionList() []AccessibilityAction {
	if p.Actions == nil {
		return nil
	}
	return *p.Actions
}

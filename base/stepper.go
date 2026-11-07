package base

import (
	"strconv"

	"github.com/oneo-me/nsgo/core"
)

// StepStatus is where a step of a Stepper stands.
type StepStatus uint8

const (
	StepUpcoming StepStatus = iota
	StepCurrent
	StepComplete
	// StepFailed is a step that Failed says did.
	StepFailed
)

// StepperStep is step i of a Stepper as Item draws it.
type StepperStep struct {
	Index int
	// Status is where it stands, following Current.
	Status func() StepStatus
	// Travel is the step progress came from and the one it is at, so a
	// jump can stagger what it crosses.
	Travel func() [2]int
}

// Stepper is where a person is in a flow of Count steps. Current is the
// step in progress, Count once every one is done; out-of-range values
// clamp. It owns each step's status and what assistive technology hears of
// it, and with OnSelect completed steps are buttons that return to them;
// their look, and the flow, are the app's.
type Stepper struct {
	Count   int
	Current func() int
	// Failed says step i failed.
	Failed func(i int) bool
	// OnSelect makes completed steps buttons.
	OnSelect func(step int)
	// StepLabel is what assistive technology reads for step i, with its
	// status, and StepHint after it.
	StepLabel func(i int, status StepStatus) string
	StepHint  func(i int) string
	// Item draws what step i holds around its pressable head; Head draws
	// the head.
	Head func(cx *core.Scope, step StepperStep) []core.Node
	Item func(cx *core.Scope, step StepperStep, head core.Node) core.Node
	// StepStyle and StepStyleFunc are the head's.
	StepClass     string
	StepStyle     core.Style
	StepStyleFunc func() core.Style
	// Caption goes after the steps, naming the current one: assistive
	// technology hears it as it changes.
	Caption      core.Node
	CaptionClass string
	CaptionStyle core.Style
	ListClass    string
	ListStyle    core.Style
	Class        string
	Style        core.Style
	StyleFunc    func() core.Style
	Label        string
	TestID       string
	// Tooltip is its view's, as a core.View's.
	Tooltip core.Node
}

// Render implements core.Node.
func (s Stepper) Render(cx *core.Scope) core.Node {
	current := func() int {
		if s.Current == nil {
			return 0
		}
		return min(max(s.Current(), 0), s.Count)
	}
	last := core.Untracked(cx, current)
	travel := core.NewMemo(cx, func() [2]int {
		to := current()
		from := last
		last = to
		return [2]int{from, to}
	})
	status := func(i int) StepStatus {
		switch c := current(); {
		case s.Failed != nil && s.Failed(i):
			return StepFailed
		case i < c:
			return StepComplete
		case i == c:
			return StepCurrent
		}
		return StepUpcoming
	}
	var items []core.Node
	for i := range s.Count {
		step := StepperStep{Index: i, Status: func() StepStatus { return status(i) }, Travel: travel.Get}
		label := func() string {
			if s.StepLabel == nil {
				return ""
			}
			return s.StepLabel(i, status(i))
		}
		v := core.View{Class: s.StepClass, Style: s.StepStyle, StyleFunc: s.StepStyleFunc, LabelFunc: label}
		if s.StepHint != nil {
			v.Hint = s.StepHint(i)
		}
		if s.TestID != "" {
			v.TestID = s.TestID + "-" + strconv.Itoa(i)
		}
		if s.Head != nil {
			v.Children = s.Head(cx, step)
		}
		if s.OnSelect != nil {
			v.Role = core.RoleButton
			v.Disabled = func() bool { return i > current() }
			v.OnPress = func(core.PressEvent) {
				if i < core.Untracked(cx, current) {
					s.OnSelect(i)
				}
			}
		} else {
			v.Accessible = true
		}
		var node core.Node = v
		if s.Item != nil {
			node = s.Item(cx, step, v)
		}
		items = append(items, node)
	}
	children := []core.Node{core.View{Class: s.ListClass, Style: s.ListStyle, Children: items}}
	if s.Caption != nil {
		children = append(children, core.View{Class: s.CaptionClass, Style: s.CaptionStyle, LiveRegion: core.LiveRegionPolite, Children: []core.Node{s.Caption}})
	}
	return core.View{Class: s.Class, Style: s.Style, StyleFunc: s.StyleFunc, Label: s.Label, TestID: s.TestID, Tooltip: s.Tooltip, Children: children}
}

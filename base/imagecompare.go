package base

import (
	"math"
	"strconv"

	"github.com/oneo-me/nsgo/core"
)

// compareStep is how far a screen reader's step moves a comparison's
// divider.
const compareStep = 5

// ImageCompare lays Before over After and shows it up to a divider:
// Position is the share of the frame, from 0 to 100, Before covers, 50 when
// nil. A press moves the divider there and a drag moves it along; screen
// readers step it as a slider. Both sides are laid out at the frame's size,
// so their content lines up. The app sizes it, and draws the Divider, a
// view on the divider's line, and Children over the sides.
type ImageCompare struct {
	Before, After core.Node
	Position      func() float32
	OnChange      func(position float32)
	// Vertical puts Before on top, the divider across.
	Vertical     bool
	Divider      core.Node
	DividerClass string
	DividerStyle core.Style
	Children     []core.Node
	Class        string
	Style        core.Style
	StyleFunc    func() core.Style
	Label        string
	TestID       string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (p ImageCompare) Render(cx *core.Scope) core.Node {
	size := core.NewSignal(cx, core.Size{})
	share := func(v float32) float32 { return min(max(v, 0), 100) }
	at := func() float32 {
		if p.Position == nil {
			return 50
		}
		return share(p.Position())
	}
	set := func(v float32) {
		if v = share(v); p.OnChange != nil && v != core.Untracked(cx, at) {
			p.OnChange(v)
		}
	}
	extent := func() core.Points {
		s := size.GetUntracked()
		if p.Vertical {
			return s.Height
		}
		return s.Width
	}
	var start float32
	pan := func(e core.PanEvent) {
		d, whole := e.Translation.X, extent()
		if p.Vertical {
			d = e.Translation.Y
		}
		if e.Phase == core.GestureBegan {
			start = core.Untracked(cx, at)
		}
		if whole > 0 {
			set(start + float32(d/whole*100))
		}
	}
	fill := core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0)}
	full := fill.Merge(core.Style{Right: core.Pt(0), Bottom: core.Pt(0)})
	reveal := core.View{Style: fill.Merge(core.Style{Overflow: core.Hidden}), StyleFunc: func() core.Style {
		if p.Vertical {
			return core.Style{Right: core.Pt(0), Height: core.Pct(at())}
		}
		return core.Style{Bottom: core.Pt(0), Width: core.Pct(at())}
	}, Children: []core.Node{core.View{Style: fill, StyleFunc: func() core.Style {
		s := size.Get()
		return core.Style{Width: core.Pt(s.Width), Height: core.Pt(s.Height)}
	}, Children: []core.Node{p.Before}}}}
	children := append([]core.Node{core.View{Style: full, Children: []core.Node{p.After}}, reveal}, p.Children...)
	if p.Divider != nil {
		children = append(children, core.View{Class: p.DividerClass, PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute}.Merge(p.DividerStyle),
			StyleFunc: func() core.Style {
				if p.Vertical {
					return core.Style{Top: core.Pct(at())}
				}
				return core.Style{Left: core.Pct(at())}
			}, Children: []core.Node{p.Divider}})
	}
	activation := core.PanActivation{ActiveOffsetX: 2, FailOffsetY: 12}
	if p.Vertical {
		activation = core.PanActivation{ActiveOffsetY: 2, FailOffsetX: 12}
	}
	return core.View{Class: p.Class, Style: p.Style, StyleFunc: p.StyleFunc, TestID: p.TestID, Role: core.RoleAdjustable, Label: p.Label, Accessible: true, Tooltip: p.Tooltip,
		AccessibilityValue: func() core.AccessibilityValue {
			return core.AccessibilityValue{Min: 0, Max: 100, Now: at(), Text: strconv.Itoa(int(math.Round(float64(at())))) + "%"}
		},
		AccessibilityActions: []core.AccessibilityAction{{Name: core.ActionIncrement}, {Name: core.ActionDecrement}},
		OnAccessibilityAction: func(name string) {
			switch name {
			case core.ActionIncrement:
				set(core.Untracked(cx, at) + compareStep)
			case core.ActionDecrement:
				set(core.Untracked(cx, at) - compareStep)
			}
		},
		OnLayout: func(b core.Bounds) {
			size.Set(b.Size)
			if p.OnLayout != nil {
				p.OnLayout(b)
			}
		},
		OnPress: func(e core.PressEvent) {
			if whole := extent(); whole > 0 {
				d := e.Position.X
				if p.Vertical {
					d = e.Position.Y
				}
				set(float32(d / whole * 100))
			}
		},
		OnPan: pan, PanActivation: activation, Children: children}
}

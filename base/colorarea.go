package base

import (
	"math"

	"github.com/oneo-me/nsgo/core"
)

// ColorArea picks the saturation and brightness of HSB under a draggable
// thumb, over the hue's color. It has no size or look of its own: the app
// styles the panel and the thumb, and FillThumb fills it with the color.
type ColorArea struct {
	Disabled   func() bool
	PanelClass string
	PanelStyle core.Style
	ThumbClass string
	ThumbStyle core.Style
	FillThumb  bool
	Class      string
	Style      core.Style
	Label      string
	TestID     string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (a ColorArea) Render(cx *core.Scope) core.Node {
	picker := colorStateOf(cx)
	disabled := func() bool { return is(a.Disabled) || picker.isDisabled() }
	var size core.Size
	var drag *core.Point
	focused := core.NewSignal(cx, false)
	env := cx.Env()
	place := func(at core.Point) {
		if size.Width <= 0 || size.Height <= 0 {
			return
		}
		x, y := float64(at.X), float64(at.Y)
		if core.Untracked(cx, env.RTL) {
			x = float64(size.Width) - x
		}
		s := min(max(x/float64(size.Width), 0), 1) * 100
		b := min(max(1-y/float64(size.Height), 0), 1) * 100
		now := picker.now()
		picker.set(now.withTriple(ColorHSB, [3]float64{now.channel(ColorHSB, ChannelHue), math.Round(s), math.Round(b)}))
	}
	nudge := func(n colorNudge) {
		if core.Untracked(cx, disabled) {
			return
		}
		now := picker.now()
		s, b := now.channel(ColorHSB, ChannelSaturation), now.channel(ColorHSB, ChannelBrightness)
		switch {
		case n.x != 0:
			s = math.Round(s + n.x)
		case n.y != 0:
			b = math.Round(b + n.y)
		case n.min:
			b = 0
		case n.max:
			b = 100
		}
		picker.set(now.withTriple(ColorHSB, [3]float64{now.channel(ColorHSB, ChannelHue), min(max(s, 0), 100), min(max(b, 0), 100)}))
		picker.end()
	}
	release := func(core.ResponderEvent) {
		if drag != nil {
			drag = nil
			picker.end()
		}
	}
	panel := core.View{Class: a.PanelClass, Style: colorFill.Merge(core.Style{OverflowX: core.Hidden, OverflowY: core.Hidden, PointerEvents: core.PointerEventsNone}).Merge(a.PanelStyle),
		NoFocus: true, PartStates: func() core.PartStates { return core.PartStates{Disabled: is(a.Disabled)} },
		StyleFunc: func() core.Style { return core.Style{Background: picker.model.Get().hueColor()} },
		Children: []core.Node{
			core.View{Style: colorFill, StyleFunc: func() core.Style {
				// White at the least saturation's side.
				angle := float32(90)
				if env.RTL() {
					angle = 270
				}
				return core.Style{Gradients: []core.Gradient{core.LinearGradient(angle, 0xffffffff, core.Transparent)}}
			}},
			core.View{Style: colorFill.Merge(core.Style{Gradients: []core.Gradient{core.LinearGradient(180, core.Transparent, 0x000000ff)}})},
		}}
	thumb := core.View{Class: a.ThumbClass, Style: colorThumb.Merge(core.Style{PointerEvents: core.PointerEventsNone}).Merge(a.ThumbStyle),
		StyleFunc: func() core.Style {
			m := picker.model.Get()
			x := m.channel(ColorHSB, ChannelSaturation) / 100
			if env.RTL() {
				x = 1 - x
			}
			s := core.Style{Left: core.Pct(float32(x * 100)), Top: core.Pct(float32(100 - m.channel(ColorHSB, ChannelBrightness)))}
			if a.FillThumb {
				s.Background = m.opaque()
			}
			return s
		},
		Disabled: disabled, Accessible: true, Role: core.RoleAdjustable, Label: a.Label,
		AccessibilityValue: func() core.AccessibilityValue {
			m := picker.model.Get()
			return core.AccessibilityValue{Max: 100, Now: float32(m.channel(ColorHSB, ChannelBrightness)), Text: m.format(ColorFormatHex)}
		},
		AccessibilityActions: colorActions,
		OnAccessibilityAction: func(name string) {
			switch name {
			case "increment":
				nudge(colorNudge{y: 1})
			case "decrement":
				nudge(colorNudge{y: -1})
			}
		},
		// A press makes it a stop of the keyboard focus; the drag is the
		// area's.
		OnPress: func(core.PressEvent) {},
		OnFocus: func() { focused.Set(true) }, OnBlur: func() { focused.Set(false) }}
	return core.View{Class: a.Class, Style: a.Style, TestID: a.TestID, Tooltip: a.Tooltip, Disabled: disabled, NoFocus: true,
		OnLayout: func(b core.Bounds) {
			size = b.Size
			if a.OnLayout != nil {
				a.OnLayout(b)
			}
		},
		Responder: &core.Responder{Keep: true,
			Claims: []core.TouchClaim{{Ask: func(core.ResponderEvent) bool { return !core.Untracked(cx, disabled) }}},
			OnGrant: func(e core.ResponderEvent) {
				at := e.Position
				drag = &at
				place(at)
			},
			OnMove: func(e core.ResponderEvent) {
				if drag != nil {
					d := e.Gesture.Delta
					place(core.Point{X: drag.X + d.X, Y: drag.Y + d.Y})
				}
			},
			OnRelease: release, OnTerminate: release},
		Children: []core.Node{panel, thumb, colorKeys(focused.Get, env.RTL, areaKeys, nudge)}}
}

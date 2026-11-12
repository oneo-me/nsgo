package base

import (
	"slices"

	"github.com/nsgo-dev/nsgo/core"
)

// colorMaxSegments is the most gradients a track takes.
const colorMaxSegments = 12

// ColorSlider picks a channel of the color in any space along a track of
// what each of its values gives. It has no size or look of its own: the
// app styles the track and the thumb. An alpha track shows CheckerboardColor
// squares under it.
type ColorSlider struct {
	ColorSpace ColorSpace
	Channel    ColorChannel
	// Horizontal unless Axis says Vertical.
	Axis       Axis
	Disabled   func() bool
	TrackClass string
	TrackStyle core.Style
	ThumbClass string
	ThumbStyle core.Style
	// ThumbStyleFunc follows what it reads; the thumb shows data-dragging
	// while it moves.
	ThumbStyleFunc func() core.Style
	FillThumb      bool
	// CheckerboardColor shows squares CheckerboardSize wide, 4 when 0.
	CheckerboardColor core.Color
	CheckerboardSize  core.Points
	Class             string
	Style             core.Style
	Label             string
	TestID            string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (c ColorSlider) Render(cx *core.Scope) core.Node {
	picker := colorStateOf(cx)
	space, channel := c.ColorSpace, resolveChannel(c.ColorSpace, c.Channel)
	spec := colorSpecOf(space, channel)
	vertical := c.Axis == Vertical
	env := cx.Env()
	mirrored := func() bool { return !vertical && env.RTL() }
	disabled := func() bool { return is(c.Disabled) || picker.isDisabled() }
	rail := core.NewSignal(cx, core.Bounds{})
	var drag *float64
	dragging := core.NewSignal(cx, false)
	focused := core.NewSignal(cx, false)
	value := func() float64 { return picker.model.Get().channel(space, channel) }
	fraction := func() float64 { return spec.fraction(value()) }
	set := func(v float64) { picker.set(picker.now().withChannel(space, channel, spec.snap(v))) }
	length := func() float64 {
		r := rail.GetUntracked()
		if vertical {
			return float64(r.Size.Height)
		}
		return float64(r.Size.Width)
	}
	setAt := func(points float64) {
		if l := length(); l > 0 {
			set(spec.at(points / l))
		}
	}
	along := func(at core.Point) float64 {
		r := rail.GetUntracked()
		switch {
		case vertical:
			return float64(r.Origin.Y + r.Size.Height - at.Y)
		case core.Untracked(cx, mirrored):
			return float64(r.Origin.X + r.Size.Width - at.X)
		}
		return float64(at.X - r.Origin.X)
	}
	offset := func(e core.ResponderEvent) float64 {
		d := e.Gesture.Delta
		switch {
		case vertical:
			return -float64(d.Y)
		case core.Untracked(cx, mirrored):
			return -float64(d.X)
		}
		return float64(d.X)
	}
	dragTo := func(e core.ResponderEvent) {
		if drag != nil {
			setAt(*drag + offset(e))
		}
	}
	release := func(core.ResponderEvent) {
		if drag != nil {
			drag = nil
			dragging.Set(false)
			picker.end()
		}
	}
	nudge := func(n colorNudge) {
		if core.Untracked(cx, disabled) {
			return
		}
		to := core.Untracked(cx, value) + (n.x+n.y)*spec.step
		switch {
		case n.min:
			to = 0
		case n.max:
			to = spec.max
		}
		set(to)
		picker.end()
	}
	claim := []core.TouchClaim{{Ask: func(core.ResponderEvent) bool { return !core.Untracked(cx, disabled) }}}
	// The track's colors, once per change of the color; a segment restyles
	// only when its two stops change.
	stops := core.NewMemoFunc(cx, func() []core.Color {
		n := colorSegments(space, channel)
		m := picker.model.Get()
		out := make([]core.Color, n+1)
		for k := range out {
			out[k] = m.track(space, channel, float64(k)/float64(n))
		}
		return out
	}, slices.Equal[[]core.Color])
	segment := func(k int) core.Node {
		return core.Func(func(cx *core.Scope) core.Node {
			ends := core.NewMemo(cx, func() [2]core.Color {
				s := stops.Get()
				n := len(s) - 1
				switch {
				case k >= n:
					return [2]core.Color{}
				case vertical:
					// A column runs top to bottom: the top is its end.
					return [2]core.Color{s[n-k-1], s[n-k]}
				}
				return [2]core.Color{s[k], s[k+1]}
			})
			return core.View{Style: calendarGrow, NoFocus: true, StyleFunc: func() core.Style {
				e := ends.Get()
				if e == [2]core.Color{} {
					return core.Style{Display: core.DisplayNone}
				}
				angle := float32(90)
				switch {
				case vertical:
					angle = 0
				case env.RTL():
					angle = 270
				}
				return core.Style{Gradients: []core.Gradient{core.LinearGradient(angle, e[0], e[1])}}
			}}
		})
	}
	bars := make([]core.Node, colorMaxSegments)
	for k := range bars {
		bars[k] = segment(k)
	}
	colors := core.View{Style: colorFill, NoFocus: true, Children: bars}
	if !vertical {
		colors.Style = colors.Style.Merge(calendarRow)
	}
	track := core.View{Class: c.TrackClass, NoFocus: true,
		Style:      core.Style{OverflowX: core.Hidden, OverflowY: core.Hidden, PointerEvents: core.PointerEventsNone}.Merge(c.TrackStyle),
		PartStates: func() core.PartStates { return core.PartStates{Disabled: is(c.Disabled)} },
		OnLayout:   func(b core.Bounds) { rail.Set(b) },
		Children:   []core.Node{Checkerboard(c.CheckerboardColor, cmpPoints(c.CheckerboardSize, 4)), colors}}
	thumb := core.View{Class: c.ThumbClass, Style: colorThumb.Merge(c.ThumbStyle),
		StyleFunc: func() core.Style {
			r, f := rail.Get(), float32(fraction())
			if mirrored() {
				f = 1 - f
			}
			var s core.Style
			if vertical {
				s = core.Style{Top: core.Pt(r.Origin.Y + core.Points(1-f)*r.Size.Height), Left: core.Pt(r.Origin.X + r.Size.Width/2)}
			} else {
				s = core.Style{Left: core.Pt(r.Origin.X + core.Points(f)*r.Size.Width), Top: core.Pt(r.Origin.Y + r.Size.Height/2)}
			}
			if c.FillThumb {
				m := picker.model.Get()
				s.Background = m.opaque()
				if channel == ChannelAlpha {
					s.Background = m.color()
				}
			}
			if c.ThumbStyleFunc != nil {
				s = s.Merge(c.ThumbStyleFunc())
			}
			return s
		},
		PartStates: func() core.PartStates { return dragStates(dragging.Get()) },
		Disabled:   disabled, Accessible: true, Role: core.RoleAdjustable, Label: c.Label,
		AccessibilityValue: func() core.AccessibilityValue {
			now := value()
			return core.AccessibilityValue{Max: float32(spec.max), Now: float32(now), Text: spec.format(now)}
		},
		AccessibilityActions: colorActions,
		OnAccessibilityAction: func(name string) {
			switch name {
			case "increment":
				nudge(colorNudge{x: 1})
			case "decrement":
				nudge(colorNudge{x: -1})
			}
		},
		OnPress: func(core.PressEvent) {},
		OnFocus: func() { focused.Set(true) }, OnBlur: func() { focused.Set(false) },
		Responder: &core.Responder{Keep: true, Claims: claim,
			OnGrant: func(core.ResponderEvent) {
				from := core.Untracked(cx, fraction) * length()
				drag = &from
				dragging.Set(true)
			},
			OnMove: dragTo, OnRelease: release, OnTerminate: release}}
	return core.View{Class: c.Class, Style: c.Style, Disabled: disabled, NoFocus: true, TestID: c.TestID, OnLayout: c.OnLayout, Tooltip: c.Tooltip,
		Responder: &core.Responder{Keep: true, Claims: claim,
			OnGrant: func(e core.ResponderEvent) {
				at := along(e.Position)
				drag = &at
				dragging.Set(true)
				setAt(at)
			},
			OnMove: dragTo, OnRelease: release, OnTerminate: release},
		Children: []core.Node{track, thumb, colorKeys(focused.Get, mirrored, sliderKeys, nudge)}}
}

// cmpPoints is p, or def when 0.
func cmpPoints(p, def core.Points) core.Points {
	if p == 0 {
		return def
	}
	return p
}

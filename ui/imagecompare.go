package ui

import (
	"math"
	"strconv"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// compareStep is how far a screen reader's step moves a comparison's
// divider.
const compareStep = 5

// ImageCompare lays Before over After in a rounded frame and shows it up
// to a white divider and knob: Position is the share of the frame, from 0
// to 100, Before covers, 50 when nil. A press moves the divider there and a
// drag moves it along; screen readers step it as a slider. Both sides are
// laid out at the frame's size, so their content lines up. BeforeLabel and
// AfterLabel caption the sides.
type ImageCompare struct {
	Before, After core.Node
	Position      func() float32
	OnChange      func(position float32)
	// Vertical puts Before on top, the divider across.
	Vertical                bool
	BeforeLabel, AfterLabel string
	Label                   string
	// AspectRatio is the frame's width over its height: 3:2 when 0.
	AspectRatio float32
	TestID      string
	Style       core.Style
}

// Render implements core.Node.
func (p ImageCompare) Render(cx *core.Scope) core.Node {
	ratio := p.AspectRatio
	if ratio == 0 {
		ratio = 3.0 / 2
	}
	// The captions and the divider are white on black, which read on any
	// picture.
	white := core.RGB(0xffffff)
	caption := func(label string, s core.Style) core.Node {
		if label == "" {
			return nil
		}
		return core.View{PointerEvents: core.PointerEventsNone, Style: TypeCaption.Merge(core.Style{Position: core.Absolute,
			Padding: core.Symmetric(2, 8), CornerRadius: RadiusFull, Background: Black(0.5), Color: white, FontWeight: Medium}).Merge(s),
			Children: []core.Node{line(label, core.Style{}, 1)}}
	}
	inset := core.Pt(12)
	after := core.Style{Top: inset, Right: inset}
	if p.Vertical {
		after = core.Style{Bottom: inset, Left: inset}
	}
	const knob = 32
	chevrons := lucide.ChevronsLeftRight
	edge := core.Style{Background: white, Shadows: []core.BoxShadow{{Blur: 6, Color: Black(0.3)}},
		AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}
	if p.Vertical {
		chevrons = lucide.ChevronsUpDown
		edge.Left, edge.Right, edge.Height, edge.Margin = core.Pt(0), core.Pt(0), core.Pt(2), core.Edges{Top: -1}
	} else {
		edge.Top, edge.Bottom, edge.Width, edge.Margin = core.Pt(0), core.Pt(0), core.Pt(2), core.Edges{Left: -1}
	}
	knobView := core.View{Style: core.Style{Width: core.Pt(knob), Height: core.Pt(knob), FlexShrink: 0, CornerRadius: knob / 2,
		Background: white, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
		Shadows: []core.BoxShadow{{Offset: core.Point{Y: 1}, Blur: 4, Color: Black(0.3)}}},
		Children: []core.Node{Icon(chevrons, knob/2, Fixed(core.RGB(0x262626)))}}
	cursor := core.CursorColResize
	if p.Vertical {
		cursor = core.CursorRowResize
	}
	var captions []core.Node
	for _, c := range []core.Node{caption(p.BeforeLabel, core.Style{Top: inset, Left: inset}), caption(p.AfterLabel, after)} {
		if c != nil {
			captions = append(captions, c)
		}
	}
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
	children := append([]core.Node{core.View{Style: full, Children: []core.Node{p.After}}, reveal}, captions...)
	children = append(children, core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute}.Merge(edge),
		StyleFunc: func() core.Style {
			if p.Vertical {
				return core.Style{Top: core.Pct(at())}
			}
			return core.Style{Left: core.Pct(at())}
		}, Children: []core.Node{knobView}})
	activation := core.PanActivation{ActiveOffsetX: 2, FailOffsetY: 12}
	if p.Vertical {
		activation = core.PanActivation{ActiveOffsetY: 2, FailOffsetX: 12}
	}
	return core.View{TestID: p.TestID, Role: core.RoleAdjustable, Label: p.Label, Accessible: true,
		Style: core.Style{AspectRatio: ratio, Overflow: core.Hidden, CornerRadius: RadiusCard, Cursor: cursor,
			MinWidth: core.Pt(80), MinHeight: core.Pt(80)}.Merge(p.Style),
		StyleFunc: Filled(ColorsOf(cx).Inset),
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
		OnLayout: func(b core.Bounds) { size.Set(b.Size) },
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

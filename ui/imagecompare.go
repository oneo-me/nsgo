package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// ImageCompare is base.ImageCompare in a rounded frame, with a white
// divider and knob. BeforeLabel and AfterLabel caption the sides.
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
	return base.ImageCompare{Before: p.Before, After: p.After, Position: p.Position, OnChange: p.OnChange, Vertical: p.Vertical, Label: p.Label, TestID: p.TestID,
		Divider: knobView, DividerStyle: edge, Children: captions,
		Style: core.Style{AspectRatio: ratio, Overflow: core.Hidden, CornerRadius: RadiusCard, Cursor: cursor,
			MinWidth: core.Pt(80), MinHeight: core.Pt(80)}.Merge(p.Style),
		StyleFunc: Filled(ColorsOf(cx).Inset)}
}

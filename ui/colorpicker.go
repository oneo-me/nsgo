package ui

import (
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/base/locales"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
)

// Sizes of a color picker.
const (
	colorWidth    core.Points = 256
	colorPlane    core.Points = 160
	colorKnob     core.Points = 18
	colorSlider   core.Points = 20
	colorTrack    core.Points = 12
	colorChannelW core.Points = 6
	colorChannelH core.Points = 16
	colorChecks   core.Points = 6
	colorTool     core.Points = 24
	colorCheckers core.Color  = 0xcdcdcdff
)

// ColorPicker is a color view: a saturation and brightness plane,
// hue and opacity sliders, the fields in RGB, HSB or HSL, hex and opacity,
// and a menu that copies the color, 256 pt wide: base.ColorPicker's parts
// in a look. An eyedropper shows where the platform has
// one (core.Env.CanPickScreenColor).
type ColorPicker struct {
	Value        func() core.Color
	DefaultValue core.Color
	// Format is how OnChange and OnChangeEnd write the color, and the
	// notation the fields start in.
	Format base.ColorFormat
	// NoAlpha leaves the opacity slider and field out.
	NoAlpha     bool
	Disabled    func() bool
	OnChange    func(value core.Color, formatted string)
	OnChangeEnd func(value core.Color, formatted string)
	TestID      string
	// Style merges over the picker's.
	Style core.Style
}

// Render implements core.Node.
func (p ColorPicker) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	grown := func(scale float32) *core.Style {
		return &core.Style{Transform: core.Transform{Scale: scale, TranslateXPercent: -50, TranslateYPercent: -50}}
	}
	motion := core.Transition{Duration: 150 * time.Millisecond, Easing: core.EaseOut, Properties: core.TransitionTransform}
	if ReduceMotion(cx) {
		motion = core.Transition{}
	}
	knob := core.Style{Width: core.Pt(colorChannelW), Height: core.Pt(colorChannelH), CornerRadius: colorChannelW / 2, Transition: motion,
		Data: []core.DataStyle{{Name: "dragging", Style: grown(1.35)}}}
	// The bars reach as far as the plane, the room for the knob's press
	// past their ends.
	slider := core.Style{Height: core.Pt(colorSlider), JustifyContent: core.JustifyCenter, Margin: core.Edges{Left: -colorSlider / 2, Right: -colorSlider / 2}}
	track := core.Style{Height: core.Pt(colorTrack), CornerRadius: colorTrack / 2, Margin: core.Edges{Left: colorSlider / 2, Right: colorSlider / 2}}
	thumb := func() core.Style {
		return FinOf(cx).Knob().Over(core.Style{Background: colors.Knob(), BorderWidth: 1, BorderColor: colors.Border(), Focus: FocusRing(cx, 2)})
	}
	children := []core.Node{
		base.ColorArea{Style: core.Style{Height: core.Pt(colorPlane)}, PanelStyle: core.Style{CornerRadius: RadiusControl},
			PanelStyleFunc: func() core.Style { return core.Style{Focus: FocusRing(cx, 2)} },
			// The thumb is a ring on the plane's own color.
			ThumbStyle: core.Style{Width: core.Pt(colorKnob), Height: core.Pt(colorKnob), CornerRadius: colorKnob / 2, BorderWidth: 2, BorderColor: 0xffffffff,
				OutlineWidth: 1, OutlineColor: Black(0.4), Transition: motion, Data: []core.DataStyle{{Name: "dragging", Style: grown(1.15)}}},
			Label: locales.ColorSaturationBrightness(cx)},
		base.ColorSlider{Style: slider, ColorSpace: base.ColorHSB, Channel: base.ChannelHue, TrackStyle: track, ThumbStyle: knob, ThumbStyleFunc: thumb, Label: locales.ColorHue(cx)},
	}
	if !p.NoAlpha {
		checked := track.Merge(core.Style{Background: 0xffffffff})
		children = append(children, base.ColorSlider{Style: slider, Channel: base.ChannelAlpha, TrackStyle: checked, ThumbStyle: knob, ThumbStyleFunc: thumb, Label: locales.ColorAlpha(cx),
			CheckerboardColor: colorCheckers, CheckerboardSize: colorChecks})
	}
	number := func(in base.NumberInput) core.Node {
		return NumberInput{HideSteppers: true, Style: colorShare(1)}.look(cx, in)
	}
	last := []core.Node{colorHexField(cx)}
	if !p.NoAlpha {
		last = append(last, base.ColorField{Channel: base.ChannelAlpha, Letter: "A", Number: number})
	}
	children = append(children, colorTools(cx), base.ColorChannels{Number: number, Style: colorRow}, core.View{Style: colorRow, Children: last})
	return base.ColorPicker{Value: p.Value, DefaultValue: p.DefaultValue, Format: p.Format, NoAlpha: p.NoAlpha, Disabled: p.Disabled, OnChange: p.OnChange, OnChangeEnd: p.OnChangeEnd,
		TestID: p.TestID, Style: core.Style{Width: core.Pt(colorWidth), MaxWidth: core.Pct(100), Gap: 8, Disabled: Dimmed()}.Merge(p.Style), Children: children}
}

// colorRow is a row of a picker's fields, which share it.
var colorRow = core.Style{Direction: core.Row, Gap: 8}

// colorShare has a field take a part of its row.
func colorShare(part float32) core.Style {
	return core.Style{Width: core.Pt(0), MinWidth: core.Pt(0), FlexGrow: part, FlexShrink: 1}
}

// colorTools are the row of the notation's menu, the eyedropper and the
// copy menu.
func colorTools(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	items := func(parts base.ColorMenuParts) []core.Node {
		rows := make([]core.Node, len(parts.Choices))
		for i, c := range parts.Choices {
			rows[i] = MenuItem{Text: c.Text, Checked: c.Checked, OnSelect: c.OnSelect}
		}
		return rows
	}
	env := cx.Env()
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 4}, Children: []core.Node{
		base.ColorNotationMenu{Draw: func(parts base.ColorMenuParts) core.Node {
			return Menu{Label: parts.Label, Trigger: Button{Variant: ButtonGhost}, Align: core.AlignStart, Children: items(parts),
				Face: core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 4, Padding: core.Symmetric(0, 8), Height: core.Pt(ButtonHeight)}, Children: []core.Node{
					core.Text{ContentFunc: parts.Current, MaxLines: 1, Style: TypeBody, StyleFunc: Colored(colors.Text)},
					Icon(lucide.ChevronDown, 12, colors.Muted)}}}
		}},
		core.View{Style: core.Style{FlexGrow: 1}},
		core.Show{When: env.CanPickScreenColor, Then: func(cx *core.Scope) core.Node {
			return base.ColorEyeDropper{Label: locales.ColorEyeDropper(cx),
				Style: core.Style{Width: core.Pt(colorTool), Height: core.Pt(colorTool), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
					CornerRadius: RadiusControl, Disabled: Dimmed()},
				StyleFunc: func() core.Style {
					return core.Style{Hover: &core.Style{Background: colors.Hover()}, Active: &core.Style{Background: colors.Pressed()}, Focus: FocusRing(cx, -2)}
				},
				Children: []core.Node{Icon(lucide.Pipette, 14, colors.Muted)}}
		}},
		base.ColorCopyMenu{Draw: func(parts base.ColorMenuParts) core.Node {
			return Menu{Label: parts.Label, Trigger: Button{Variant: ButtonGhost, Icon: lucide.Copy, Side: colorTool, IconSize: 14, Tint: colors.Muted}, Align: core.AlignEnd, Children: items(parts)}
		}},
	}}
}

// colorHexField is the color's hex digits after a # that is no control.
func colorHexField(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	return core.View{Style: colorShare(2).Merge(core.Style{Direction: core.Row, AlignItems: core.AlignStretch, Height: core.Pt(ButtonHeight), CornerRadius: RadiusControl,
		BorderWidth: 1, Overflow: core.Hidden, Transition: quick}),
		StyleFunc: func() core.Style {
			return core.Style{BorderColor: colors.FieldBorder(), Background: colors.Field(), Hover: &core.Style{BorderColor: Mix(colors.FieldBorder(), colors.Text(), 0.25)},
				FocusWithin: &core.Style{BorderColor: colors.Accent()}}
		},
		Children: []core.Node{
			core.View{Style: core.Style{AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, MinWidth: core.Pt(ButtonHeight - 6), Padding: core.Edges{Left: 8}},
				Children: []core.Node{core.Text{Content: "#", MaxLines: 1, Style: TypeLabel, StyleFunc: Colored(colors.Muted)}}},
			base.ColorField{Digits: true, Label: locales.ColorHex(cx), Field: func(in core.Input) core.Node {
				in.Style = TypeBody.Merge(core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(32), Padding: core.Edges{Left: 4, Right: 10}})
				in.StyleFunc = func() core.Style {
					return core.Style{Color: colors.Text(), PlaceholderColor: colors.Faint(), SelectionColor: colors.Now().Selection, CaretColor: colors.Accent()}
				}
				return in
			}}}}
}

// ColorWell is a color picker: a swatch 48 × 32 that opens a
// ColorPicker beside it.
type ColorWell struct {
	Value func() core.Color
	// Picker is what opens: its Value and DefaultValue go unused.
	Picker       ColorPicker
	OnChange     func(value core.Color)
	Open         func() bool
	OnOpenChange func(open bool, reason base.Reason)
	// Label names the well for assistive technology.
	Label  string
	TestID string
}

// Render implements core.Node.
func (w ColorWell) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	picker := w.Picker
	picker.Value = w.Value
	onChange := picker.OnChange
	picker.OnChange = func(c core.Color, formatted string) {
		if w.OnChange != nil {
			w.OnChange(c)
		}
		if onChange != nil {
			onChange(c, formatted)
		}
	}
	return base.Popover{Open: w.Open, OnOpenChange: w.OnOpenChange, Offset: 4, Margin: 8, Label: w.Label, TestID: w.TestID,
		Style: core.Style{Width: core.Pt(48), Height: core.Pt(32), AlignSelf: core.AlignStart, Padding: core.All(4), CornerRadius: RadiusControl / 2, BorderWidth: 1},
		StyleFunc: func() core.Style {
			return core.Style{BorderColor: colors.Border(), Focus: FocusRing(cx, 2)}
		},
		Trigger: core.View{Style: core.Style{FlexGrow: 1, CornerRadius: RadiusControl / 2, OverflowX: core.Hidden, OverflowY: core.Hidden, Background: 0xffffffff}, Children: []core.Node{
			base.Checkerboard(colorCheckers, colorChecks),
			core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Left: core.Pt(0)},
				StyleFunc: func() core.Style { return core.Style{Background: core.SetColor(w.Value())} }}}},
		PopupStyle:     core.Style{Padding: core.All(12), MaxWidth: core.Pct(100)},
		PopupStyleFunc: func() core.Style { return FinOf(cx).Menu().Over(core.Style{CornerRadius: RadiusCard, BorderWidth: 1}) },
		Children:       []core.Node{picker}}
}

package ui

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/base/locales"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// Sizes of a color picker.
const (
	colorThumb    core.Points = 14
	colorTrack    core.Points = 10
	colorChecks   core.Points = 6
	colorWidth    core.Points = 256
	colorSwatch   core.Points = 22
	colorCheckers core.Color  = 0xcccccc80
)

// ColorPicker is a color by a saturation and brightness area, hue and
// opacity sliders, a hex field and swatches, 256 pt wide. The eyedropper
// shows where the platform has one (core.Env.CanPickScreenColor).
type ColorPicker struct {
	Value        func() core.Color
	DefaultValue core.Color
	// Format is how OnChange and OnChangeEnd write the color.
	Format base.ColorFormat
	// Presets are colors a press picks.
	Presets []core.Color
	// NoAlpha leaves the opacity slider out.
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
	thumb := core.Style{Width: core.Pt(colorThumb), Height: core.Pt(colorThumb), CornerRadius: colorThumb / 2, BorderWidth: 2, BorderColor: 0xffffffff,
		Shadows: []core.BoxShadow{{Offset: core.Point{Y: 1}, Blur: 3, Color: Black(0.3)}}}
	track := core.Style{Height: core.Pt(colorTrack), CornerRadius: colorTrack / 2}
	slider := core.Style{Height: core.Pt(18), JustifyContent: core.JustifyCenter}
	field := func(in core.Input) core.Node {
		in.Style = in.Style.Merge(core.Style{FlexGrow: 1, FlexShrink: 1})
		return Input{Input: in}
	}
	well := func(style core.Style) func() core.Style {
		return func() core.Style {
			return style.Merge(core.Style{BorderColor: colors.FieldBorder(), Selected: &core.Style{BorderColor: colors.Accent()}})
		}
	}
	shape := core.Style{Width: core.Pt(colorSwatch), Height: core.Pt(colorSwatch), CornerRadius: RadiusControl - 2, BorderWidth: 1, Disabled: Dimmed()}
	children := []core.Node{
		base.ColorArea{Style: core.Style{Height: core.Pt(160)}, PanelStyle: core.Style{CornerRadius: RadiusControl}, ThumbStyle: thumb, FillThumb: true,
			Label: locales.ColorSaturationBrightness(cx)},
		base.ColorSlider{Style: slider, ColorSpace: base.ColorHSB, Channel: base.ChannelHue, TrackStyle: track, ThumbStyle: thumb, FillThumb: true, Label: locales.ColorHue(cx)},
	}
	if !p.NoAlpha {
		children = append(children, base.ColorSlider{Style: slider, Channel: base.ChannelAlpha, TrackStyle: track, ThumbStyle: thumb, FillThumb: true, Label: locales.ColorAlpha(cx),
			CheckerboardColor: colorCheckers, CheckerboardSize: colorChecks})
	}
	row := []core.Node{
		base.ColorSwatch{Style: shape, StyleFunc: well(core.Style{}), CheckerboardColor: colorCheckers, CheckerboardSize: colorChecks},
		base.ColorField{Label: locales.ColorHex(cx), Field: field},
	}
	if core.Untracked(cx, cx.Env().CanPickScreenColor) {
		row = append(row, base.ColorEyeDropper{Label: locales.ColorEyeDropper(cx),
			Style: core.Style{Width: core.Pt(ButtonHeight), Height: core.Pt(ButtonHeight), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
				CornerRadius: RadiusControl, Disabled: Dimmed()},
			StyleFunc: func() core.Style {
				return core.Style{Hover: &core.Style{Background: colors.Hover()}, Active: &core.Style{Background: colors.Pressed()}, Focus: FocusRing(cx, -2)}
			},
			Children: []core.Node{Icon(lucide.Pipette, iconSize, colors.Muted)}})
	}
	children = append(children, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: row})
	if len(p.Presets) > 0 {
		presets := make([]core.Node, len(p.Presets))
		for i, c := range p.Presets {
			presets[i] = base.ColorSwatch{Color: func() core.Color { return c }, Style: shape, StyleFunc: well(core.Style{}),
				CheckerboardColor: colorCheckers, CheckerboardSize: colorChecks}
		}
		children = append(children, core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 6}, Children: presets})
	}
	return base.ColorPicker{Value: p.Value, DefaultValue: p.DefaultValue, Format: p.Format, Disabled: p.Disabled, OnChange: p.OnChange, OnChangeEnd: p.OnChangeEnd,
		TestID: p.TestID, Style: core.Style{Width: core.Pt(colorWidth), MaxWidth: core.Pct(100), Gap: 8}.Merge(p.Style), Children: children}
}

// ColorWell is a swatch 48 × 28 that opens a ColorPicker beside it.
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
		Style: core.Style{Width: core.Pt(48), Height: core.Pt(ButtonHeight), AlignSelf: core.AlignStart, Padding: core.All(3), CornerRadius: RadiusControl, BorderWidth: 1},
		StyleFunc: func() core.Style {
			return core.Style{BorderColor: colors.FieldBorder(), Background: colors.Field(), Focus: FocusRing(cx, 2)}
		},
		Trigger: core.View{Style: core.Style{FlexGrow: 1, CornerRadius: RadiusControl - 3, OverflowX: core.Hidden, OverflowY: core.Hidden},
			StyleFunc: func() core.Style { return core.Style{Background: core.SetColor(w.Value())} }},
		PopupStyle:     core.Style{Padding: core.All(12)},
		PopupStyleFunc: func() core.Style { return FinOf(cx).Menu().Over(core.Style{CornerRadius: RadiusCard, BorderWidth: 1}) },
		Children:       []core.Node{picker}}
}

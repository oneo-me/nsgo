package ui

import (
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

// Text is a line in ui's type: size, weight and a color it
// follows.
func Text(content string, size core.Points, weight core.FontWeight, c func() core.Color) core.Text {
	return core.Text{Content: content, Style: core.Style{FontSize: size, FontWeight: weight},
		StyleFunc: func() core.Style { return core.Style{Color: c()} }}
}

// Tabular returns t with figures of one width, for values that change.
func Tabular(t core.Text) core.Text {
	t.Style.FontVariant = core.FontTabularNums
	return t
}

// Textf is a line whose content follows fn.
func Textf(fn func() string, size core.Points, weight core.FontWeight, c func() core.Color) core.Text {
	t := Text("", size, weight, c)
	t.ContentFunc = fn
	return t
}

// Weights, short.
const (
	Regular  = core.FontWeightNormal
	Medium   = core.FontWeightMedium
	Semibold = core.FontWeightSemibold
	Bold     = core.FontWeightBold
)

// constant is s as a string that may change.
func constant(s string) func() string { return func() string { return s } }

// Touch reports whether the device takes touches rather than a pointer,
// for what a pointer alone can do: a desktop's pointer and a TV's remote,
// which moves a focus, take none. A control's size is the same on every
// device.
func Touch(cx *core.Scope) bool {
	idiom := core.Untracked(cx, cx.Env().Idiom)
	return idiom != core.IdiomDesktop && idiom != core.IdiomTV
}

// RowHeight is a row's height.
const RowHeight core.Points = 36

// FillRest grows to fill its container.
var FillRest = core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), MinHeight: core.Pt(0)}

// transparent is no color that is set, unlike 0: a border kept for its
// width, a fill a state replaces.
var transparent = core.RGBA(0xffffff, 0)

// is reads an optional state.
func is(fn func() bool) bool { return fn != nil && fn() }

// ReduceMotion reports whether the user asked for less motion as the
// tree is built.
func ReduceMotion(cx *core.Scope) bool { return core.Untracked(cx, cx.Env().ReduceMotion) }

// quick is the transition of a control's fill and edge between states.
var quick = core.Transition{Duration: 120 * time.Millisecond, Easing: core.EaseOut,
	Properties: core.TransitionBackground | core.TransitionBorder}

// The type the components set text in, by role: small print, running
// text, a control's text, a dialog's or a group's title, a page's
// heading, and code.
var (
	TypeCaption = core.Style{FontSize: 12, LineHeight: 16}
	TypeBody    = core.Style{FontSize: 13, LineHeight: 18}
	TypeLabel   = core.Style{FontSize: 13, LineHeight: 18, FontWeight: Medium}
	TypeTitle   = core.Style{FontSize: 15, LineHeight: 20, FontWeight: Semibold}
	TypeHeading = core.Style{FontSize: 20, LineHeight: 26, FontWeight: Semibold}
	TypeCode    = core.Style{FontSize: 12, LineHeight: 18, FontFamily: Mono}
)

// iconSize is an icon's size beside text of the body's size.
const iconSize core.Points = 16

// Smaller and larger radii than RadiusControl: marks inside controls
// (a checkbox, a chip), and capsules and circles.
const (
	RadiusSmall core.Points = 4
	RadiusFull  core.Points = 9999
)

// line is content in s, wrapping unless lines says how many it takes.
func line(content string, s core.Style, lines int) core.Node {
	return core.Text{Content: content, MaxLines: lines, Style: s}
}

// Dimmed is a disabled control: faded, with nothing under the pointer.
func Dimmed() *core.Style {
	return &core.Style{Opacity: core.Alpha(0.45), Cursor: core.CursorDefault}
}

// scrim is the backdrop behind a dialog or a sheet.
var scrim = core.Style{Background: Black(0.4)}

// surface is what every popup is drawn on, read in a StyleFunc: a card
// with the soft shadow of what floats.
func surface(cx *core.Scope) core.Style {
	colors := ColorsOf(cx)
	return core.Style{Background: colors.Surface(), Color: colors.Text(), CornerRadius: RadiusCard, BorderCurve: core.BorderCurveContinuous,
		BorderWidth: 1, BorderColor: colors.Border(), Shadows: FinOf(cx).Menu().Shadows}
}

// sheetPanel is a popup as a sheet at side, rounded where it meets the
// app, read in a StyleFunc.
func sheetPanel(cx *core.Scope, side core.Placement) core.Style {
	colors := ColorsOf(cx)
	s := core.Style{Background: colors.Surface(), Color: colors.Text(), Shadows: FinOf(cx).Menu().Shadows}
	const r = 16
	switch side {
	case core.PlaceTop:
		s.CornerRadii = core.Corners{BottomLeft: r, BottomRight: r}
	case core.PlaceLeft:
		s.CornerRadii = core.Corners{TopRight: r, BottomRight: r}
		s.Width, s.MaxWidth, s.Height = core.Pt(320), core.Pct(85), core.Pct(100)
	case core.PlaceRight:
		s.CornerRadii = core.Corners{TopLeft: r, BottomLeft: r}
		s.Width, s.MaxWidth, s.Height = core.Pt(320), core.Pct(85), core.Pct(100)
	default:
		s.CornerRadii = core.Corners{TopLeft: r, TopRight: r}
		s.MaxHeight = core.Pct(90)
	}
	return s
}

// sheetContent is a sheet's padded content at side, a grabber over it at
// the bottom, clear of the safe area.
func sheetContent(cx *core.Scope, side core.Placement, children []core.Node) []core.Node {
	pad := core.All(16)
	var out []core.Node
	if side == core.PlaceBottom {
		pad.Top = 8
		out = append(out, core.View{Style: core.Style{AlignSelf: core.AlignCenter, Width: core.Pt(36), Height: core.Pt(5),
			CornerRadius: 3, Margin: core.Edges{Bottom: 8}}, StyleFunc: Filled(ColorsOf(cx).Border)})
	}
	safe := core.EnvEdges{Bottom: core.EnvSafeArea}
	switch side {
	case core.PlaceTop:
		safe = core.EnvEdges{Top: core.EnvSafeArea, Right: core.EnvSafeArea, Left: core.EnvSafeArea}
	case core.PlaceLeft:
		safe = core.EnvEdges{Top: core.EnvSafeArea, Bottom: core.EnvSafeArea, Left: core.EnvSafeArea}
	case core.PlaceRight:
		safe = core.EnvEdges{Top: core.EnvSafeArea, Right: core.EnvSafeArea, Bottom: core.EnvSafeArea}
	}
	return []core.Node{core.View{Style: core.Style{Padding: pad, PaddingEnv: safe, Gap: 12, FlexShrink: 1},
		Children: append(out, children...)}}
}

// tooltipBubble is a tooltip's look: content in the page's color on a
// small bubble of the text's.
func tooltipBubble(cx *core.Scope, content string) core.Node {
	colors := ColorsOf(cx)
	return core.View{Style: core.Style{CornerRadius: RadiusSmall, Padding: core.Symmetric(4, 8), MaxWidth: core.Pt(280)},
		StyleFunc: func() core.Style {
			return core.Style{Background: colors.Text(), Shadows: FinOf(cx).Raised().Shadows}
		},
		Children: []core.Node{core.Text{Content: content, Style: TypeCaption, StyleFunc: Colored(colors.Background)}}}
}

// Inherited is no color, for an icon in the color of the text it is in.
func Inherited() core.Color { return 0 }

// Icon draws a Lucide glyph size points square in the tint, which it
// follows; nothing for nil.
func Icon(svg func() *core.SVG, size core.Points, tint func() core.Color) core.Node {
	i := core.Icon{Style: core.Style{Width: core.Pt(size), Height: core.Pt(size)},
		StyleFunc: func() core.Style { return core.Style{Color: tint()} }}
	if svg != nil {
		i.SVG = svg()
	}
	return i
}

// IconOf draws the glyph svg returns, which it follows.
func IconOf(svg func() *core.SVG, size core.Points, tint func() core.Color) core.Node {
	i := Icon(nil, size, tint).(core.Icon)
	i.SVGFunc = svg
	return i
}

// CardStyle is a card's surface, flat, read in a
// StyleFunc.
func CardStyle(cx *core.Scope) core.Style {
	colors := ColorsOf(cx)
	return core.Style{Background: colors.Surface(), CornerRadius: RadiusCard, BorderCurve: core.BorderCurveContinuous,
		BorderWidth: 1, BorderColor: colors.Border()}
}

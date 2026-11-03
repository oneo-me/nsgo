// Package ui is the gallery's look and the components it shares, as NSH's
// nsh-gallery-ui is its gallery's: colors, finishes and motion here,
// styled components over base and core in components.go, the parts
// several demos share in parts.go, Onexa's sidebar shell in shell.go,
// Onevra's settings surface in settings.go and the line icons. It
// composes base and core and is maintained with the gallery, not
// published as an API.
//
// The look is Onevra's gallery: a warm page, demos on flat bordered cards
// under bold titles, flat bordered controls 28 pt tall
// in 13 pt text, 8 pt corners and one terracotta accent. On touch screens
// controls keep that look and take presses 44 pt across.
//
// Components are structs with a Render method, as Onexa's are: the app
// sets the fields and handles changes. Each reads its colors where it is
// built (ColorsOf, TintsOf, FinOf), so a subtree's theme or forced
// appearance reaches it, and reads them in StyleFuncs, Draws and the like,
// which follow a change of theme or appearance in place, as NSH's styles
// do: nothing is built again.
package ui

import (
	"time"

	"github.com/oneo-me/nsgo/core"
)

// Colors are the gallery's colors where a scope is: each method reads its
// color from the palette shown there, the theme's roles under the
// gallery's names and the ones Theme adds. A StyleFunc or Draw that calls
// one follows the palette; a method value (colors.Muted) is a color to
// hand to what reads it so.
type Colors struct{ cx *core.Scope }

// ColorsOf is the gallery's colors where cx is.
func ColorsOf(cx *core.Scope) Colors { return Colors{cx} }

func (c Colors) role(name string) core.Color { return c.cx.Palette().Colors[name] }

// Background is the page behind the cards; Surface a card; Inset a
// surface inside a card; Raised a control on a card.
func (c Colors) Background() core.Color { return c.cx.Palette().Background }
func (c Colors) Surface() core.Color    { return c.cx.Palette().Surface }
func (c Colors) Inset() core.Color      { return c.role("inset") }
func (c Colors) Raised() core.Color     { return c.role("raised") }

// Field is an input's well, below the card and the insets it sits on;
// FieldBorder its edge.
func (c Colors) Field() core.Color       { return c.role("field") }
func (c Colors) FieldBorder() core.Color { return c.role("fieldBorder") }
func (c Colors) Border() core.Color      { return c.cx.Palette().Border }

// Line is the hairlines inside insets.
func (c Colors) Line() core.Color  { return c.role("line") }
func (c Colors) Text() core.Color  { return c.cx.Palette().Text }
func (c Colors) Muted() core.Color { return c.cx.Palette().TextMuted }
func (c Colors) Faint() core.Color { return c.role("faint") }

// Accent is the accent as a foreground: text, icons, lines, small marks.
// AccentFill is the accent as a fill, with OnAccent on it.
func (c Colors) Accent() core.Color     { return c.cx.Palette().Accent }
func (c Colors) AccentFill() core.Color { return c.role("accentFill") }
func (c Colors) OnAccent() core.Color   { return c.cx.Palette().AccentText }

// Light is the main action's button, light on the dark page and dark on
// the light one, with OnLight on it.
func (c Colors) Light() core.Color   { return c.role("light") }
func (c Colors) OnLight() core.Color { return c.role("onLight") }

// Track and Knob are a switch's when off.
func (c Colors) Track() core.Color { return c.role("track") }
func (c Colors) Knob() core.Color  { return c.role("knob") }

// Hover is a row under the pointer, Pressed one held down.
func (c Colors) Hover() core.Color   { return c.cx.Palette().SurfaceHover }
func (c Colors) Pressed() core.Color { return c.cx.Palette().SurfacePressed }

// Code is code inside insets.
func (c Colors) Code() core.Color   { return c.role("code") }
func (c Colors) Info() core.Color   { return c.role("info") }
func (c Colors) Warn() core.Color   { return c.cx.Palette().Warning }
func (c Colors) Danger() core.Color { return c.cx.Palette().Danger }
func (c Colors) Up() core.Color     { return c.cx.Palette().Success }
func (c Colors) Down() core.Color   { return c.role("down") }

// Colored is a StyleFunc that colors text and icons c.
func Colored(c func() core.Color) func() core.Style {
	return func() core.Style { return core.Style{Color: c()} }
}

// Filled is a StyleFunc that fills a view with c.
func Filled(c func() core.Color) func() core.Style {
	return func() core.Style { return core.Style{Background: c()} }
}

// Palette is the gallery's colors at one moment, for code that reads
// many at once, such as a Draw.
type Palette struct {
	Background, Surface, Inset, Raised, Field, FieldBorder, Border, Line core.Color
	Text, Muted, Faint, Accent, AccentFill, OnAccent, Light, OnLight     core.Color
	Track, Knob, Hover, Pressed, Code, Info, Warn, Danger, Up, Down      core.Color
}

// Now is the colors where c is as they are now; a Draw that calls it
// draws again when they change.
func (c Colors) Now() Palette {
	p := c.cx.Palette()
	r := p.Colors
	return Palette{
		Background: p.Background, Surface: p.Surface, Inset: r["inset"], Raised: r["raised"],
		Field: r["field"], FieldBorder: r["fieldBorder"], Border: p.Border, Line: r["line"],
		Text: p.Text, Muted: p.TextMuted, Faint: r["faint"],
		Accent: p.Accent, AccentFill: r["accentFill"], OnAccent: p.AccentText,
		Light: r["light"], OnLight: r["onLight"], Track: r["track"], Knob: r["knob"],
		Hover: p.SurfaceHover, Pressed: p.SurfacePressed, Code: r["code"],
		Info: r["info"], Warn: p.Warning, Danger: p.Danger, Up: p.Success, Down: r["down"],
	}
}

// Themed is what build returns, built again when what it reads (the
// palette) changes: a component's styles by part, for its StylesFunc.
func Themed[T any](cx *core.Scope, build func() T) func() T {
	return core.NewMemoFunc(cx, build, nil).Get
}

// Fixed is c as a color to hand to what takes one that may change.
func Fixed(c core.Color) func() core.Color { return func() core.Color { return c } }

// Theme returns the gallery's theme: Onevra's (core.DefaultTheme), with
// the roles the gallery adds in Palette.Colors, each derived from Onevra's
// as its components derive theirs: insets and tracks recess with the
// text's color, fields and hairlines take the border, Light is the
// inverse of the page.
func Theme() *core.Theme {
	t := core.DefaultTheme()
	t.Light.Colors = map[string]core.Color{
		"inset": core.RGB(0xf2f1ee), "raised": core.RGB(0xfefefd), "field": core.RGB(0xfdfdfc), "fieldBorder": core.RGB(0xdeded5),
		"line": core.RGB(0xe6e5de), "faint": core.RGB(0xa29e95), "accentFill": core.RGB(0xbd481b),
		"light": core.RGB(0x33332f), "onLight": core.RGB(0xfcfbf9), "track": core.RGB(0xc4c4bc), "knob": core.RGB(0xffffff),
		"code": core.RGB(0x3a3631), "info": core.RGB(0x3d63d1), "down": core.RGB(0xb4444c),
	}
	t.Dark.Colors = map[string]core.Color{
		"inset": core.RGB(0x2a2825), "raised": core.RGB(0x36332e), "field": core.RGB(0x262421), "fieldBorder": core.RGB(0x504b43),
		"line": core.RGB(0x3d3933), "faint": core.RGB(0x857e73), "accentFill": core.RGB(0xed986e),
		"light": core.RGB(0xeeeae2), "onLight": core.RGB(0x211f1c), "track": core.RGB(0x68635b), "knob": core.RGB(0xf7f5f1),
		"code": core.RGB(0xe2dcd2), "info": core.RGB(0x7ea8ff), "down": core.RGB(0xe58f95),
	}
	return t
}

// Tints are translucent tints of the tones where a scope is, for fills
// behind text, read as ColorsOf's are.
type Tints struct{ cx *core.Scope }

// TintsOf is the tones' tints where cx is.
func TintsOf(cx *core.Scope) Tints { return Tints{cx} }

// tint is c at the opacity of a tint in the appearance shown.
func (t Tints) tint(c core.Color, light, dark float32) core.Color {
	if t.cx.Dark() {
		return Alpha(c, dark)
	}
	return Alpha(c, light)
}

func (t Tints) Accent() core.Color {
	colors := ColorsOf(t.cx)
	if t.cx.Dark() {
		return Alpha(colors.Accent(), 0.14)
	}
	return Alpha(colors.AccentFill(), 0.12)
}
func (t Tints) Info() core.Color   { return t.tint(ColorsOf(t.cx).Info(), 0.10, 0.16) }
func (t Tints) Warn() core.Color   { return t.tint(ColorsOf(t.cx).Warn(), 0.11, 0.16) }
func (t Tints) Danger() core.Color { return t.tint(ColorsOf(t.cx).Danger(), 0.10, 0.16) }
func (t Tints) Up() core.Color     { return t.tint(ColorsOf(t.cx).Up(), 0.10, 0.16) }
func (t Tints) Down() core.Color   { return t.tint(ColorsOf(t.cx).Down(), 0.10, 0.16) }

// Fins are the finishes where a scope is, read as ColorsOf's colors are.
type Fins struct{ cx *core.Scope }

// FinOf is the finishes where cx is.
func FinOf(cx *core.Scope) Fins { return Fins{cx} }

// shown is the finishes of the appearance shown.
func (f Fins) shown() *finishes {
	if f.cx.Dark() {
		return &darkFinishes
	}
	return &lightFinishes
}

func (f Fins) Card() Finish   { return f.shown().card }
func (f Fins) Inset() Finish  { return f.shown().inset }
func (f Fins) Field() Finish  { return f.shown().field }
func (f Fins) Raised() Finish { return f.shown().raised }
func (f Fins) Light() Finish  { return f.shown().light }
func (f Fins) Accent() Finish { return f.shown().accent }
func (f Fins) Track() Finish  { return f.shown().track }
func (f Fins) Knob() Finish   { return f.shown().knob }

// Pill is lit glass floating over a track: the selected segment, a small
// round button.
func (f Fins) Pill() Finish { return f.shown().pill }

// Chip is a tinted capsule's rim and highlight (badges).
func (f Fins) Chip() Finish { return f.shown().chip }

// Glass is a bar floating over content: translucent, blurred, rimmed.
func (f Fins) Glass() Finish { return f.shown().glass }

// Menu is a floating panel (menus, the composer): glass, denser for
// reading.
func (f Fins) Menu() Finish { return f.shown().menu }

// Alpha returns c at opacity a.
func Alpha(c core.Color, a float32) core.Color {
	r, g, b, _ := c.Components()
	return core.RGBA(uint32(r*255+0.5)<<16|uint32(g*255+0.5)<<8|uint32(b*255+0.5), a)
}

// Mix is a mixed with b by t, from 0 (a) to 1 (b), alpha included.
func Mix(a, b core.Color, t float32) core.Color {
	ar, ag, ab, aa := a.Components()
	br, bg, bb, ba := b.Components()
	f := float64(t)
	m := func(x, y float64) uint32 { return uint32((x+(y-x)*f)*255 + 0.5) }
	return core.RGBA(m(ar, br)<<16|m(ag, bg)<<8|m(ab, bb), float32(aa+(ba-aa)*f))
}

// Finish is a surface's light and depth: a gradient over its color, its
// shadows, a rim and, for glass, a blur of what is behind.
type Finish struct {
	Background core.Color
	Gradients  []core.Gradient
	Shadows    []core.BoxShadow
	Border     core.Color
	Backdrop   []core.Filter
}

// Style is the finish as a style to merge into a surface's.
func (f Finish) Style() core.Style {
	return core.Style{Background: f.Background, Gradients: f.Gradients, Shadows: f.Shadows, BorderColor: f.Border, BackdropFilters: f.Backdrop}
}

// Over merges the finish into s, keeping what s sets.
func (f Finish) Over(s core.Style) core.Style { return f.Style().Merge(s) }

// finishes are Onevra's: flat surfaces, a hairline shadow under raised
// controls and the knob, and a soft one under what floats.
type finishes struct {
	card, inset, field, raised, light, accent, track, knob, pill, chip, glass, menu Finish
}

// Shadow is one of CSS's box-shadow layers: x, y, blur, spread, color.
func Shadow(x, y, blur, spread core.Points, c core.Color) core.BoxShadow {
	return core.BoxShadow{Offset: core.Point{X: x, Y: y}, Blur: blur, Spread: spread, Color: c}
}

// Inner is an inset shadow layer.
func Inner(x, y, blur core.Points, c core.Color) core.BoxShadow {
	return core.BoxShadow{Offset: core.Point{X: x, Y: y}, Blur: blur, Color: c, Inset: true}
}

// Black, White and Paper are black, white and the warm shadow color at
// opacity a.
func Black(a float32) core.Color { return core.RGBA(0x000000, a) }

func White(a float32) core.Color { return core.RGBA(0xffffff, a) }

func Paper(a float32) core.Color { return core.RGBA(0x1e1408, a) }

func shadows(s ...core.BoxShadow) []core.BoxShadow { return s }

var darkFinishes = finishes{
	raised: Finish{Shadows: shadows(Shadow(0, 1, 2, 0, Black(0.2)))},
	knob:   Finish{Shadows: shadows(Shadow(0, 1, 2, 0, Black(0.35)))},
	pill:   Finish{Background: core.RGB(0x36332e), Border: core.RGB(0x36332e), Shadows: shadows(Shadow(0, 1, 2, 0, Black(0.4)))},
	glass: Finish{Background: core.RGBA(0x211f1c, 0.92), Border: core.RGB(0x504b43),
		Shadows: shadows(Shadow(0, 4, 16, 0, Black(0.3))), Backdrop: []core.Filter{core.Blur(20)}},
	menu: Finish{Background: core.RGBA(0x211f1c, 0.92), Border: core.RGB(0x504b43),
		Shadows: shadows(Shadow(0, 4, 16, 0, Black(0.3))), Backdrop: []core.Filter{core.Blur(24)}},
}

var lightFinishes = finishes{
	raised: Finish{Shadows: shadows(Shadow(0, 1, 2, 0, Black(0.07)))},
	knob:   Finish{Shadows: shadows(Shadow(0, 1, 2, 0, Black(0.2)))},
	pill:   Finish{Background: core.RGB(0xfefefd), Border: core.RGB(0xfefefd), Shadows: shadows(Shadow(0, 1, 2, 0, Black(0.14)))},
	glass: Finish{Background: core.RGBA(0xfcfbf9, 0.92), Border: core.RGB(0xdeded5),
		Shadows: shadows(Shadow(0, 4, 16, 0, Black(0.14))), Backdrop: []core.Filter{core.Blur(20)}},
	menu: Finish{Background: core.RGBA(0xfcfbf9, 0.92), Border: core.RGB(0xdeded5),
		Shadows: shadows(Shadow(0, 4, 16, 0, Black(0.14))), Backdrop: []core.Filter{core.Blur(24)}},
}

// Tone names a color role for badges, figures and meters.
type Tone uint8

const (
	ToneNeutral Tone = iota
	ToneAccent
	ToneInfo
	ToneWarn
	ToneDanger
	ToneUp
	ToneDown
)

// Color is the tone's foreground, read where ColorsOf's colors are.
func (t Tone) Color(cx *core.Scope) core.Color {
	colors := ColorsOf(cx)
	switch t {
	case ToneAccent:
		return colors.Accent()
	case ToneInfo:
		return colors.Info()
	case ToneWarn:
		return colors.Warn()
	case ToneDanger:
		return colors.Danger()
	case ToneUp:
		return colors.Up()
	case ToneDown:
		return colors.Down()
	}
	return colors.Muted()
}

// Shade is the tone's color, for what takes one that may change.
func (t Tone) Shade(cx *core.Scope) func() core.Color {
	return func() core.Color { return t.Color(cx) }
}

// Foreground is the tone's color for text and icons: the text's when
// neutral.
func (t Tone) Foreground(cx *core.Scope) func() core.Color {
	return func() core.Color {
		if t == ToneNeutral {
			return ColorsOf(cx).Text()
		}
		return t.Color(cx)
	}
}

// Tint is the tone's translucent fill, read where ColorsOf's colors are.
func (t Tone) Tint(cx *core.Scope) core.Color {
	tints := TintsOf(cx)
	switch t {
	case ToneAccent:
		return tints.Accent()
	case ToneInfo:
		return tints.Info()
	case ToneWarn:
		return tints.Warn()
	case ToneDanger:
		return tints.Danger()
	case ToneUp:
		return tints.Up()
	case ToneDown:
		return tints.Down()
	}
	return ColorsOf(cx).Raised()
}

// Mono is the monospaced family, for code (API names, identifiers).
const Mono = core.FontMonospace

// Corner radii.
const (
	RadiusCard    core.Points = 12
	RadiusInset   core.Points = 8
	RadiusControl core.Points = 8
)

// The motion every control shares: quick, slightly springy.
var (
	pressSpring  = core.SpringConfig{Stiffness: 520, Damping: 34}
	SettleSpring = core.SpringConfig{Stiffness: 320, Damping: 30}
	EaseOutCubic = core.Easing{Kind: core.EasingCubic, Mode: core.EaseModeOut}
	enterTiming  = core.TimingConfig{Duration: 420 * time.Millisecond, Easing: EaseOutCubic}
	switchTiming = core.TimingConfig{Duration: 200 * time.Millisecond, Easing: core.Easing{Kind: core.EasingCubic, Mode: core.EaseModeInOut}}
)

// Spring and Timing start an animation of v on the platform.
func Spring(cx *core.Scope, v *core.AnimatedValue, to float64, c core.SpringConfig) {
	cx.StartAnimation(core.Spring(v, to, c), nil)
}

func Timing(cx *core.Scope, v *core.AnimatedValue, to float64, c core.TimingConfig) {
	cx.StartAnimation(core.Timing(v, to, c), nil)
}

// Lerp maps v's 0 to 1 onto from to to.
func Lerp(v core.AnimatedNode, from, to float64) core.AnimatedNode {
	return core.Interpolate(v, core.Interpolation{Input: []float64{0, 1}, Output: []float64{from, to}})
}

// Hovered is a pressable under the pointer: a whisper of the text color
// over its surface, quieter than pressed and than selected. Like the rest
// of a style, read it in a StyleFunc.
func Hovered(cx *core.Scope) *core.Style {
	return &core.Style{Background: Alpha(ColorsOf(cx).Text(), 0.05)}
}

// FocusRing is the keyboard focus: a 2-point ring of info inside the
// view's edge (offset 2 puts it outside, around a filled button), read in
// a StyleFunc.
func FocusRing(cx *core.Scope, offset core.Points) *core.Style {
	return &core.Style{OutlineWidth: 2, OutlineColor: ColorsOf(cx).Info(), OutlineOffset: offset}
}

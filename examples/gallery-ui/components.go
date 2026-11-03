package ui

import (
	"fmt"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// Text is a line in the gallery's type: size, weight and a color it
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

// Note is a quiet line of explanation under a demo: Text, or what
// TextFunc returns.
type Note struct {
	Text     string
	TextFunc func() string
}

// Render implements core.Node.
func (n Note) Render(cx *core.Scope) core.Node {
	fn := n.TextFunc
	if fn == nil {
		fn = constant(n.Text)
	}
	t := Textf(fn, 13, Regular, ColorsOf(cx).Muted)
	t.Style.LineHeight = 18
	return t
}

// Code is a monospaced line, for API names and values.
type Code struct{ Text string }

// Render implements core.Node.
func (c Code) Render(cx *core.Scope) core.Node {
	t := Text(c.Text, 12, Regular, ColorsOf(cx).Code)
	t.Style.FontFamily = Mono
	return t
}

// Press is a pressable that sinks a little under the finger and springs
// back; the spring runs on the platform, only press in and out reach the
// engine.
type Press struct {
	OnPress, OnLongPress func()
	Disabled             func() bool
	// Scale is how far it shrinks (default 0.97).
	Scale float64
	// StyleFunc merges over Style, as a View's does.
	Style     core.Style
	StyleFunc func() core.Style
	// Role is a button's by default.
	Role core.Role
	// LabelFunc replaces Label, as a View's does.
	Label     string
	LabelFunc func() string
	Selected  func() bool
	HitSlop   core.Points
	TestID    string
	Children  []core.Node
}

// Render implements core.Node.
func (p Press) Render(cx *core.Scope) core.Node {
	s := core.NewAnimatedValue(cx, 1)
	scale := p.Scale
	if scale == 0 {
		scale = 0.97
	}
	role := p.Role
	if role == 0 {
		role = core.RoleButton
	}
	var onPress, onLongPress func(core.PressEvent)
	if p.OnPress != nil {
		onPress = func(core.PressEvent) { p.OnPress() }
	}
	if p.OnLongPress != nil {
		onLongPress = func(core.PressEvent) { p.OnLongPress() }
	}
	style := p.Style
	style.Animated = &core.AnimatedStyle{Scale: s}
	style.Disabled = &core.Style{Opacity: core.Alpha(0.4)}
	return core.View{OnPress: onPress, OnLongPress: onLongPress, Disabled: p.Disabled, Selected: p.Selected,
		OnPressDown: func(core.PressEvent) { Spring(cx, s, scale, pressSpring) },
		OnPressUp:   func(core.PressEvent) { Spring(cx, s, 1, pressSpring) },
		Role:        role, Label: p.Label, LabelFunc: p.LabelFunc, HitSlop: core.All(p.HitSlop), TestID: p.TestID,
		Style: style, StyleFunc: p.StyleFunc, Children: p.Children}
}

// Appear fades and lifts its content in once mounted; Index staggers
// siblings, 40 ms apart, and Flex lets it grow and shrink.
type Appear struct {
	Index    int
	Flex     bool
	Children []core.Node
}

// Render implements core.Node.
func (a Appear) Render(cx *core.Scope) core.Node {
	t := core.NewAnimatedValue(cx, 0)
	c := enterTiming
	c.Delay = time.Duration(min(8, a.Index)) * 40 * time.Millisecond
	Timing(cx, t, 1, c)
	s := core.Style{Animated: &core.AnimatedStyle{Opacity: t, TranslateY: Lerp(t, 14, 0)}}
	if a.Flex {
		s = core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0), MinWidth: core.Pt(0)}.Merge(s)
	}
	return core.View{Style: s, Children: a.Children}
}

// IconTile is a rounded tile Size points square around an icon.
type IconTile struct {
	Icon IconName
	Size core.Points
}

// Render implements core.Node.
func (t IconTile) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	size := t.Size
	return core.View{StyleFunc: func() core.Style {
		return FinOf(cx).Raised().Over(core.Style{
			Width: core.Pt(size), Height: core.Pt(size), CornerRadius: size * 0.3, BorderCurve: core.BorderCurveContinuous,
			BorderWidth: 1, BorderColor: colors.Border(), Background: colors.Raised(),
			AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
		})
	}, Children: []core.Node{Icon(t.Icon, size*0.5, colors.Text)}}
}

// Card is a demo's container: a header on the page of a Title, a
// Description and Actions at its end, then a surface holding the Children
// and an optional Footer.
type Card struct {
	Title       string
	Description string
	// Actions end the header (a switch, a small button, a value).
	Actions  []core.Node
	Children []core.Node
	// Footer follows the children on its own line: a status or a caption.
	Footer []core.Node
	// Flush lays the children edge to edge, without the surface's padding
	// and gaps (rows, a full-bleed stage).
	Flush bool
	// Style merges over the surface's.
	Style core.Style
}

// CardStyle is a card's surface, read in a StyleFunc.
func CardStyle(cx *core.Scope) core.Style {
	colors := ColorsOf(cx)
	shadows := []core.BoxShadow{Shadow(0, 1, 2, 0, Paper(0.04)), Shadow(0, 4, 12, -4, Paper(0.08))}
	if cx.Dark() {
		shadows = []core.BoxShadow{Shadow(0, 1, 2, 0, Black(0.3)), Shadow(0, 6, 16, -6, Black(0.45))}
	}
	return core.Style{Background: colors.Surface(), CornerRadius: RadiusInset, BorderCurve: core.BorderCurveContinuous,
		BorderWidth: 1, BorderColor: colors.Border(), Shadows: shadows}
}

// Render implements core.Node.
func (c Card) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	var head core.Node
	if c.Title != "" {
		title := Text(c.Title, 13, Semibold, colors.Text)
		title.Role = core.RoleHeader
		titles := []core.Node{title}
		if c.Description != "" {
			description := Text(c.Description, 12, Regular, colors.Muted)
			description.MaxLines = 2
			titles = append(titles, description)
		}
		nodes := append([]core.Node{core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: titles}}, c.Actions...)
		head = core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignEnd, Gap: 10, Padding: core.Symmetric(0, 2)}, Children: nodes}
	}
	return panel(cx, panelProps{head: head, flush: c.Flush, children: c.Children, footer: c.Footer, style: c.Style})
}

// Section is a titled card of controls, the demos' sections: a heading
// with a quieter Note, or what NoteFunc returns, after it.
type Section struct {
	Title    string
	Note     string
	NoteFunc func() string
	Children []core.Node
}

// Render implements core.Node.
func (s Section) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	note := s.NoteFunc
	if note == nil {
		note = constant(s.Note)
	}
	heading := Text(s.Title, 13, Semibold, colors.Text)
	heading.Role, heading.Style.FlexGrow, heading.Style.FlexShrink = core.RoleHeader, 1, 1
	head := core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Gap: 8, Padding: core.Symmetric(0, 2)},
		Children: []core.Node{heading, Textf(note, 12, Regular, colors.Faint)}}
	return panel(cx, panelProps{testID: "section-" + s.Title, head: head, children: s.Children})
}

type panelProps struct {
	testID   string
	head     core.Node
	flush    bool
	children []core.Node
	footer   []core.Node
	style    core.Style
}

// panel is a card: the head on the page, then the surface with the body
// and the footer after a hairline. A flush body clips to the surface's
// inner corners.
func panel(cx *core.Scope, p panelProps) core.Node {
	colors := ColorsOf(cx)
	const inner = RadiusInset - 1
	var parts []core.Node
	if len(p.children) > 0 {
		body := core.Style{Gap: 12, Padding: core.All(14)}
		if p.flush {
			r := core.Corners{TopLeft: inner, TopRight: inner}
			if len(p.footer) == 0 {
				r.BottomLeft, r.BottomRight = inner, inner
			}
			body = core.Style{Overflow: core.Hidden, CornerRadii: r, BorderCurve: core.BorderCurveContinuous}
		}
		parts = append(parts, core.View{Style: body, Children: p.children})
	}
	if len(p.footer) > 0 {
		s := core.Style{Gap: 6, Padding: core.Symmetric(10, 14)}
		if len(parts) > 0 {
			s.Borders = core.Edges{Top: 1}
		}
		parts = append(parts, core.View{Style: s,
			StyleFunc: func() core.Style { return core.Style{BorderColors: core.EdgeColors{Top: colors.Line()}} }, Children: p.footer})
	}
	surface := core.View{StyleFunc: func() core.Style { return CardStyle(cx).Merge(p.style) }, Children: parts}
	if p.head == nil {
		surface.TestID = p.testID
		return surface
	}
	return core.View{TestID: p.testID, Style: core.Style{Gap: 8}, Children: []core.Node{p.head, surface}}
}

// Inset is the darker surface demos set apart. Zero Gap and Padding take
// 10 pt gaps and 12 pt padding, unless Exact takes them as given.
type Inset struct {
	Gap, Padding core.Points
	Exact        bool
	Children     []core.Node
}

// Render implements core.Node.
func (in Inset) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	gap, padding := in.Gap, in.Padding
	if !in.Exact {
		if gap == 0 {
			gap = 10
		}
		if padding == 0 {
			padding = 12
		}
	}
	return core.View{StyleFunc: func() core.Style {
		return core.Style{Background: colors.Inset(), CornerRadius: RadiusInset, BorderCurve: core.BorderCurveContinuous,
			BorderWidth: 1, BorderColor: colors.Line(), Padding: core.All(padding), Gap: gap}
	}, Children: in.Children}
}

// Caption is a Label, or what LabelFunc returns, on the left and a
// quieter Note on the right.
type Caption struct {
	Label     string
	LabelFunc func() string
	Note      func() string
}

// Render implements core.Node.
func (c Caption) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	label := c.LabelFunc
	if label == nil {
		label = constant(c.Label)
	}
	l := Textf(label, 14, Regular, colors.Muted)
	if c.Note == nil && c.LabelFunc != nil {
		return l
	}
	l.Style.FlexGrow, l.Style.FlexShrink = 1, 1
	children := []core.Node{l}
	if c.Note != nil {
		children = append(children, Textf(c.Note, 13, Regular, colors.Faint))
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Gap: 8}, Children: children}
}

// Figure is a large figure with a Unit and a Note in its Tone after it.
type Figure struct {
	Value func() string
	Unit  string
	Note  func() string
	Tone  Tone
}

// Render implements core.Node.
func (f Figure) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	spans := []core.Span{{ContentFunc: f.Value}}
	if f.Unit != "" {
		spans = append(spans, core.Span{Content: " " + f.Unit, Style: core.Style{FontSize: 17, FontWeight: Regular}, StyleFunc: Colored(colors.Muted)})
	}
	if f.Note != nil {
		spans = append(spans, core.Span{ContentFunc: func() string { return " " + f.Note() }, Style: core.Style{FontSize: 15, FontWeight: Medium},
			StyleFunc: Colored(f.Tone.Shade(cx))})
	}
	return core.Text{Spans: spans, Style: core.Style{FontSize: 32, FontWeight: Semibold, LetterSpacing: -0.5},
		StyleFunc: func() core.Style { return core.Style{Color: colors.Text()} }}
}

// Badge is a small label in a tinted capsule: Text, or what TextFunc
// returns, in Tone, or the one ToneFunc returns. Outline draws only its
// rim.
type Badge struct {
	Text     string
	TextFunc func() string
	Tone     Tone
	ToneFunc func() Tone
	Outline  bool
}

// Render implements core.Node.
func (b Badge) Render(cx *core.Scope) core.Node {
	label := b.TextFunc
	if label == nil {
		label = constant(b.Text)
	}
	tone := b.ToneFunc
	if tone == nil {
		tone = func() Tone { return b.Tone }
	}
	fg := func() core.Color { return tone().Foreground(cx)() }
	return core.View{Style: core.Style{Padding: core.Symmetric(0, 10), Height: core.Pt(26), CornerRadius: 13, JustifyContent: core.JustifyCenter, BorderWidth: 1},
		StyleFunc: func() core.Style {
			if b.Outline {
				return core.Style{BorderColor: tone().Color(cx)}
			}
			return core.Style{Background: tone().Tint(cx), Shadows: FinOf(cx).Chip().Shadows, BorderColor: Alpha(fg(), 0.1)}
		}, Children: []core.Node{Textf(label, 12, Medium, fg)}}
}

// Stat is a label above a value in its Tone, for grids of figures.
type Stat struct {
	Label string
	Value func() string
	Tone  Tone
	Width core.Length
}

// Render implements core.Node.
func (s Stat) Render(cx *core.Scope) core.Node {
	return core.View{Style: core.Style{Width: s.Width, Gap: 3, Padding: core.Symmetric(4, 0)}, Children: []core.Node{
		Text(s.Label, 12, Regular, ColorsOf(cx).Muted),
		Textf(s.Value, 15, Regular, s.Tone.Foreground(cx)),
	}}
}

// Legend is a colored square and a label with an optional value, for
// chart legends.
type Legend struct {
	Color func() core.Color
	Label string
	Value func() string
}

// Render implements core.Node.
func (l Legend) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	spans := []core.Span{{Content: l.Label}}
	if l.Value != nil {
		spans = append(spans, core.Span{ContentFunc: func() string { return " " + l.Value() }, StyleFunc: Colored(colors.Text)})
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6}, Children: []core.Node{
		core.View{Style: core.Style{Width: core.Pt(9), Height: core.Pt(9), CornerRadius: 2.5}, StyleFunc: Filled(l.Color)},
		core.Text{Spans: spans, Style: core.Style{FontSize: 13}, StyleFunc: Colored(colors.Muted)},
	}}
}

// Row is a row inside an inset, with a hairline above all but the First.
type Row struct {
	First    bool
	OnPress  func()
	Children []core.Node
}

// Render implements core.Node.
func (r Row) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	v := core.View{Style: core.Style{MinHeight: core.Pt(44), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10},
		StyleFunc: func() core.Style {
			var s core.Style
			if !r.First {
				s.Borders, s.BorderColors = core.Edges{Top: 1}, core.EdgeColors{Top: colors.Line()}
			}
			if r.OnPress != nil {
				s.Active = &core.Style{Background: colors.Raised()}
			}
			return s
		}, Children: r.Children}
	if r.OnPress != nil {
		v.OnPress = func(core.PressEvent) { r.OnPress() }
	}
	return v
}

// ButtonVariant is a button's look.
type ButtonVariant uint8

const (
	// ButtonPrimary is light on the dark page and dark on the light one:
	// the main action. It is the default.
	ButtonPrimary ButtonVariant = iota
	// ButtonSecondary is a raised, bordered action.
	ButtonSecondary
	// ButtonAccent is filled with the accent.
	ButtonAccent
	// ButtonGhost is only a rim.
	ButtonGhost
)

// Button is a button 44 pt tall, or 32 pt Small for pointer-sized
// surfaces: Text with an optional leading Icon. base.Button gives the
// behavior; the look is the gallery's.
type Button struct {
	Text    string
	Variant ButtonVariant
	// Color fills it with that color instead.
	Color    func() core.Color
	Icon     IconName
	Disabled func() bool
	// Flex grows it to share a row with its siblings.
	Flex    float32
	Small   bool
	TestID  string
	OnPress func()
}

// Render implements core.Node.
func (b Button) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	scale := core.NewAnimatedValue(cx, 1)
	r := RadiusControl
	height, padding, gap, font, iconSize := core.Points(44), core.Points(18), core.Points(8), core.Points(15), core.Points(16)
	if b.Small {
		r, height, padding, gap, font, iconSize = 8, 32, 12, 6, 13, 14
	}
	fg := colors.Text
	switch {
	case b.Color != nil:
		fg = Fixed(core.RGB(0xffffff))
	case b.Variant == ButtonPrimary:
		fg = colors.OnLight
	case b.Variant == ButtonAccent:
		fg = colors.OnAccent
	}
	face := func() core.Style {
		fin := FinOf(cx)
		bg, f := colors.Raised(), fin.Raised()
		switch {
		case b.Color != nil:
			bg, f = b.Color(), Finish{}
		case b.Variant == ButtonPrimary:
			bg, f = colors.Light(), fin.Light()
		case b.Variant == ButtonAccent:
			bg, f = colors.AccentFill(), fin.Accent()
		case b.Variant == ButtonGhost:
			bg, f = 0, Finish{}
		}
		s := f.Over(core.Style{Height: core.Pt(height), Padding: core.Symmetric(0, padding), CornerRadius: r, BorderCurve: core.BorderCurveContinuous,
			Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: gap, Background: bg})
		if b.Variant == ButtonSecondary || b.Variant == ButtonGhost {
			s.BorderWidth, s.BorderColor = 1, colors.Border()
		}
		return s
	}
	var children []core.Node
	if b.Icon != "" {
		children = append(children, Icon(b.Icon, iconSize, fg))
	}
	children = append(children, Text(b.Text, font, Semibold, fg))
	return base.Button{
		OnPress:     func(core.PressEvent) { b.OnPress() },
		OnPressDown: func(core.PressEvent) { Spring(cx, scale, 0.97, pressSpring) },
		OnPressUp:   func(core.PressEvent) { Spring(cx, scale, 1, pressSpring) },
		Disabled:    b.Disabled, Label: b.Text, TestID: b.TestID,
		Style: core.Style{FlexGrow: b.Flex, FlexShrink: b.Flex, CornerRadius: r, Animated: &core.AnimatedStyle{Scale: scale},
			Hover: &core.Style{Opacity: core.Alpha(0.88)}, Disabled: &core.Style{Opacity: core.Alpha(0.4)}},
		StyleFunc: func() core.Style { return core.Style{Focus: FocusRing(cx, 2)} },
		Children:  []core.Node{core.View{StyleFunc: face, Children: children}},
	}
}

// RoundButton is a round icon button, 44 pt unless Size says otherwise:
// light as ButtonPrimary, the default, and dark otherwise. IconFunc and
// LabelFunc replace Icon and Label.
type RoundButton struct {
	Icon      IconName
	IconFunc  func() IconName
	Label     string
	LabelFunc func() string
	Variant   ButtonVariant
	Size      core.Points
	OnPress   func()
}

// Render implements core.Node.
func (b RoundButton) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	size := b.Size
	if size == 0 {
		size = 44
	}
	name := b.IconFunc
	if name == nil {
		name = func() IconName { return b.Icon }
	}
	label := b.LabelFunc
	if label == nil {
		label = constant(b.Label)
	}
	primary := b.Variant == ButtonPrimary
	fg := colors.Text
	if primary {
		fg = colors.OnLight
	}
	face := func() core.Style {
		s := core.Style{Width: core.Pt(size), Height: core.Pt(size), CornerRadius: size / 2, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}
		if primary {
			s = FinOf(cx).Light().Over(s)
			s.Background = colors.Light()
		} else {
			s = FinOf(cx).Pill().Over(s)
			s.BorderWidth = 1
		}
		return s
	}
	return Press{OnPress: b.OnPress, Scale: 0.92, LabelFunc: label,
		Children: []core.Node{core.View{StyleFunc: face, Children: []core.Node{IconOf(name, size*0.42, fg)}}}}
}

// Switch is an on and off switch in the gallery's look: base.Switch, the
// knob springing across and the track changing color on the platform.
// Label is what assistive technology reads.
type Switch struct {
	Checked  func() bool
	OnChange func(bool)
	Label    string
}

// Render implements core.Node.
func (s Switch) Render(cx *core.Scope) core.Node {
	look := NewSwitchLook(cx, s.Checked, false)
	// 8 pt more to press on every side, without taking more room.
	return base.Switch{Checked: s.Checked, OnChange: s.OnChange, Label: s.Label,
		Style:          core.Style{Margin: core.All(-8), Padding: core.All(8)},
		TrackStyleFunc: look.Track, ThumbStyleFunc: look.Thumb}
}

// SwitchLook is the gallery's switch as the styles of base.Switch's
// parts: a track that turns orange and a knob that springs across, both
// on the platform.
type SwitchLook struct {
	Track, Thumb func() core.Style
}

// NewSwitchLook makes a switch's look, 50 × 30 or, small, 38 × 22 for
// pointer-sized rows. on is the value the app owns; the knob follows it.
func NewSwitchLook(cx *core.Scope, on func() bool, small bool) SwitchLook {
	colors := ColorsOf(cx)
	width, height, pad, knob := core.Points(50), core.Points(30), core.Points(3), core.Points(22)
	if small {
		width, height, pad, knob = 38, 22, 2, 16
	}
	start := 0.0
	shown := core.Untracked(cx, on)
	if shown {
		start = 1
	}
	t := core.NewAnimatedValue(cx, start)
	x := Lerp(t, 0, float64(width-2-2*pad-knob))
	env := cx.Env()
	cx.Effect(func() {
		next := on()
		if next == shown {
			return
		}
		shown = next
		to := 0.0
		if next {
			to = 1
		}
		// Reduced motion: the knob and the track change in place.
		if env.ReduceMotion() {
			t.Set(to)
		} else {
			Spring(cx, t, to, SettleSpring)
		}
	})
	track := func() core.Style {
		return FinOf(cx).Track().Over(core.Style{Width: core.Pt(width), Height: core.Pt(height), CornerRadius: height / 2, Padding: core.All(pad),
			BorderWidth: 1, BorderColor: colors.Border(), Background: colors.Track(),
			Animated: &core.AnimatedStyle{Background: core.Interpolate(t, core.Interpolation{Input: []float64{0, 1}, Colors: []core.Color{colors.Track(), colors.AccentFill()}})}})
	}
	return SwitchLook{Track: track, Thumb: func() core.Style {
		knobShadows := FinOf(cx).Knob().Shadows
		if cx.Dark() {
			knobShadows = []core.BoxShadow{Inner(0, 1, 0, White(0.2)), Shadow(0, 1, 3, 0, Black(0.4))}
		}
		bg := colors.Knob()
		if on() {
			bg = colors.OnAccent()
		}
		return core.Style{Width: core.Pt(knob), Height: core.Pt(knob), CornerRadius: knob / 2, Background: bg,
			// A drop of glass: a specular top over the knob's color.
			Gradients: gradA(White(0.202), White(0.098)), Shadows: knobShadows,
			Animated: &core.AnimatedStyle{TranslateX: x}}
	}}
}

// Option is one segment of a segmented control; Count shows a quiet
// number after the label when set.
type Option[T comparable] struct {
	Value T
	Label string
	Count func() int
}

// Segmented is equal segments with an indicator that springs to the
// selected one, 40 pt tall or 32 pt Small: base.Tabs, a tab per segment,
// in the gallery's look.
type Segmented[T comparable] struct {
	Options  []Option[T]
	Value    func() T
	OnChange func(T)
	Small    bool
}

// Render implements core.Node.
func (sc Segmented[T]) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	width := core.NewSignal(cx, core.Points(0))
	x := core.NewAnimatedValue(cx, 0)
	index := func() int {
		v := sc.Value()
		for i, o := range sc.Options {
			if o.Value == v {
				return i
			}
		}
		return 0
	}
	// The track's border and padding: the indicator is placed inside the
	// border, so the segments share what is left of the measured width
	// after both.
	const border, pad = 1, 3
	segment := func() core.Points { return (width.Get() - 2*(border+pad)) / core.Points(max(1, len(sc.Options))) }
	height := core.Points(40)
	if sc.Small {
		height = 32
	}
	// The first placement jumps; later ones spring.
	placed := false
	cx.Effect(func() {
		to := float64(core.Points(index()) * segment())
		if width.Get() <= 0 {
			return
		}
		if placed {
			Spring(cx, x, to, SettleSpring)
		} else {
			x.Set(to)
		}
		placed = true
	})
	font := core.Points(14)
	if sc.Small {
		font = 13
	}
	children := []core.Node{core.View{PointerEvents: core.PointerEventsNone, StyleFunc: func() core.Style {
		return FinOf(cx).Pill().Over(core.Style{Position: core.Absolute, Top: core.Pt(pad), Left: core.Pt(pad), Bottom: core.Pt(pad),
			Width: core.Pt(max(0, segment())), CornerRadius: (height - 2*(border+pad)) / 2, BorderWidth: 1,
			Animated: &core.AnimatedStyle{TranslateX: x}})
	}}}
	for _, o := range sc.Options {
		selected := func() bool { return sc.Value() == o.Value }
		label := []core.Node{core.Text{Content: o.Label, MaxLines: 1, StyleFunc: func() core.Style {
			c := colors.Muted()
			if selected() {
				c = colors.Text()
			}
			return core.Style{FontSize: font, FontWeight: Medium, Color: c}
		}}}
		if o.Count != nil {
			label = append(label, Textf(func() string { return fmt.Sprint(o.Count()) }, 12, Regular, colors.Faint))
		}
		children = append(children, base.Tab{Selected: selected, OnPress: func(core.PressEvent) { sc.OnChange(o.Value) }, Label: o.Label,
			Style:    core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 5},
			Children: label})
	}
	return core.View{OnLayout: func(b core.Bounds) { width.Set(b.Size.Width) }, Children: []core.Node{base.Tabs{
		Style: core.Style{Height: core.Pt(height), Direction: core.Row, Padding: core.All(pad), CornerRadius: height / 2, BorderWidth: border},
		StyleFunc: func() core.Style {
			return core.Style{Background: colors.Inset(), Shadows: FinOf(cx).Track().Shadows, BorderColor: colors.Border()}
		},
		Children: children,
	}}}
}

// Meter is a horizontal meter of Value, from 0 to 1, in its Tone, with a
// Marker at a limit when it is above 0.
type Meter struct {
	Value  func() float64
	Tone   Tone
	Marker float64
}

// Render implements core.Node.
func (m Meter) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	children := []core.Node{core.View{StyleFunc: func() core.Style {
		return core.Style{Width: core.Pct(float32(min(max(m.Value(), 0), 1) * 100)), Height: core.Pt(6), CornerRadius: 3, Background: m.Tone.Color(cx)}
	}}}
	if m.Marker > 0 {
		children = append(children, core.View{Style: core.Style{Position: core.Absolute, Left: core.Pct(float32(m.Marker * 100)), Top: core.Pt(-3),
			Width: core.Pt(2), Height: core.Pt(12), CornerRadius: 1}, StyleFunc: Filled(colors.Text)})
	}
	return core.View{Style: core.Style{Height: core.Pt(6), CornerRadius: 3}, StyleFunc: Filled(colors.Raised), Children: children}
}

// Sheet is a panel at the bottom over a dimmed backdrop while Open
// (base.Sheet in the gallery's look); a press on the backdrop, Escape or
// the back button calls OnClose.
type Sheet struct {
	Open     func() bool
	OnClose  func()
	Children []core.Node
}

// Render implements core.Node.
func (s Sheet) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	env := cx.Env()
	return base.Sheet{
		Open: s.Open,
		OnOpenChange: func(open bool, _ base.Reason) {
			if !open {
				s.OnClose()
			}
		},
		BackdropStyle: core.Style{Background: Black(0.4)},
		PopupStyleFunc: func() core.Style {
			return core.Style{Gap: 12, Padding: core.Edges{Top: 8, Right: 16, Bottom: env.SafeArea().Bottom + 16, Left: 16},
				CornerRadii: core.Corners{TopLeft: 16, TopRight: 16}, Background: colors.Surface()}
		},
		Children: append([]core.Node{core.View{Style: core.Style{Width: core.Pt(36), Height: core.Pt(5), CornerRadius: 3, AlignSelf: core.AlignCenter},
			StyleFunc: Filled(colors.Raised)}}, s.Children...),
	}
}

// Field is an Input in its well: the field's color, a rim and an inner
// shadow; the input's own style sets its size and padding.
type Field struct{ Input base.Input }

// Render implements core.Node.
func (f Field) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	in := f.Input
	in.Style = core.Style{FontSize: 15}.Merge(in.Style)
	own := in.StyleFunc
	in.StyleFunc = func() core.Style {
		s := core.Style{Color: colors.Text(), PlaceholderColor: colors.Faint(), SelectionColor: colors.Accent()}
		if own != nil {
			s = s.Merge(own())
		}
		return s
	}
	return core.View{StyleFunc: func() core.Style {
		return FinOf(cx).Field().Over(core.Style{CornerRadius: RadiusControl, BorderCurve: core.BorderCurveContinuous,
			BorderWidth: 1, BorderColor: colors.FieldBorder(), Background: colors.Field()})
	}, Children: []core.Node{in}}
}

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
	t := Textf(fn, 12, Regular, ColorsOf(cx).Muted)
	t.Style.LineHeight = 17
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
		return core.Style{
			Width: core.Pt(size), Height: core.Pt(size), CornerRadius: size * 0.25, BorderCurve: core.BorderCurveContinuous,
			BorderWidth: 1, BorderColor: colors.Border(), Background: colors.Raised(),
			AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
		}
	}, Children: []core.Node{Icon(t.Icon, size*0.5, colors.Accent)}}
}

// Card is a demo's container: a header on the page of a bold Title, a
// Description and Actions at its end, then a flat bordered surface holding
// the Children and an optional Footer.
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

// CardStyle is a card's surface, flat as Onevra's are, read in a
// StyleFunc.
func CardStyle(cx *core.Scope) core.Style {
	colors := ColorsOf(cx)
	return core.Style{Background: colors.Surface(), CornerRadius: RadiusCard, BorderCurve: core.BorderCurveContinuous,
		BorderWidth: 1, BorderColor: colors.Border()}
}

// Render implements core.Node.
func (c Card) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	var head core.Node
	if c.Title != "" {
		title := Text(c.Title, 13, Bold, colors.Text)
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

// Section is a titled card of controls, the demos' sections: a bold
// heading with a quieter Note, or what NoteFunc returns, after it.
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
	heading := Text(s.Title, 13, Bold, colors.Text)
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
	const inner = RadiusCard - 1
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

// Inset is the recessed surface demos set apart. Zero Gap and Padding
// take 10 pt gaps and 12 pt padding, unless Exact takes them as given.
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
			Padding: core.All(padding), Gap: gap}
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
	l := Textf(label, 13, Regular, colors.Muted)
	if c.Note == nil && c.LabelFunc != nil {
		return l
	}
	l.Style.FlexGrow, l.Style.FlexShrink = 1, 1
	children := []core.Node{l}
	if c.Note != nil {
		children = append(children, Textf(c.Note, 12, Regular, colors.Faint))
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

// Badge is a small label in a tinted capsule, as Onevra's: Text, or what
// TextFunc returns, in Tone, or the one ToneFunc returns. Outline draws
// only its rim.
type Badge struct {
	Text     string
	TextFunc func() string
	Tone     Tone
	ToneFunc func() Tone
	Outline  bool
}

// Render implements core.Node.
func (b Badge) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	label := b.TextFunc
	if label == nil {
		label = constant(b.Text)
	}
	tone := b.ToneFunc
	if tone == nil {
		tone = func() Tone { return b.Tone }
	}
	// A tinted tone's text leans to the text's color, and its rim to the
	// page's, as Onevra's badges do.
	fg := func() core.Color {
		if tone() == ToneNeutral {
			return colors.Text()
		}
		return Mix(tone().Color(cx), colors.Text(), 0.3)
	}
	return core.View{Style: core.Style{Padding: core.Symmetric(2, 8), AlignSelf: core.AlignStart, CornerRadius: 12, JustifyContent: core.JustifyCenter, BorderWidth: 1},
		StyleFunc: func() core.Style {
			t := tone()
			if b.Outline {
				return core.Style{BorderColor: Mix(colors.Background(), t.Color(cx), 0.5)}
			}
			if t == ToneNeutral {
				return core.Style{Background: colors.Surface(), BorderColor: colors.Border()}
			}
			return core.Style{Background: Mix(colors.Background(), t.Color(cx), 0.12), BorderColor: Mix(colors.Background(), t.Color(cx), 0.3)}
		}, Children: []core.Node{Textf(label, 11, Medium, fg)}}
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
		Textf(s.Value, 14, Medium, s.Tone.Foreground(cx)),
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
	v := core.View{Style: core.Style{MinHeight: core.Pt(rowHeight(cx)), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10},
		StyleFunc: func() core.Style {
			var s core.Style
			if !r.First {
				s.Borders, s.BorderColors = core.Edges{Top: 1}, core.EdgeColors{Top: colors.Line()}
			}
			if r.OnPress != nil {
				s.Active = &core.Style{Background: colors.Hover()}
			}
			return s
		}, Children: r.Children}
	if r.OnPress != nil {
		v.OnPress = func(core.PressEvent) { r.OnPress() }
	}
	return v
}

// ButtonVariant is a button's look, one of Onevra's.
type ButtonVariant uint8

const (
	// ButtonPrimary is the inverse of the page: the text's color with the
	// page's on it. It is the default.
	ButtonPrimary ButtonVariant = iota
	// ButtonSecondary is a quiet fill of the text's color.
	ButtonSecondary
	// ButtonAccent is filled with the accent, Onevra's primary.
	ButtonAccent
	// ButtonGhost is only a rim, Onevra's outline.
	ButtonGhost
)

// touch reports whether the device takes touches rather than a pointer:
// its controls keep their look and take presses 44 pt across.
func touch(cx *core.Scope) bool { return !core.Untracked(cx, cx.Env().Desktop) }

// rowHeight is a row's height: 36 pt, or 44 pt for a finger.
func rowHeight(cx *core.Scope) core.Points {
	if touch(cx) {
		return 44
	}
	return 36
}

// slop is the room to press that a control height points tall needs on
// every side for a finger, as a margin it takes back.
func slop(cx *core.Scope, height core.Points) core.Style {
	if !touch(cx) || height >= 44 {
		return core.Style{}
	}
	d := (44 - height) / 2
	return core.Style{Margin: core.Symmetric(-d, 0), Padding: core.Symmetric(d, 0)}
}

// Button is Onevra's button, 28 pt tall: Text with an optional leading
// Icon. base.Button gives the behavior; the look is the gallery's. Small
// is the same button, kept for callers that ask for a dense one.
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

// ButtonHeight is a button's height.
const ButtonHeight core.Points = 28

// Render implements core.Node.
func (b Button) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	fg := colors.Text
	switch {
	case b.Color != nil:
		fg = Fixed(core.RGB(0xffffff))
	case b.Variant == ButtonPrimary:
		fg = colors.OnLight
	case b.Variant == ButtonAccent:
		fg = colors.OnAccent
	}
	// fill is the face at rest, under the pointer and held down.
	fill := func() (rest, hover, pressed, border core.Color) {
		text := colors.Text()
		switch {
		case b.Color != nil:
			c := b.Color()
			return c, Mix(c, Black(1), 0.12), Mix(c, Black(1), 0.25), c
		case b.Variant == ButtonPrimary:
			c := colors.Light()
			return c, Mix(c, colors.Background(), 0.12), Mix(c, colors.Background(), 0.25), c
		case b.Variant == ButtonAccent:
			p := cx.Palette()
			return p.Accent, p.AccentHover, p.AccentPressed, p.Accent
		case b.Variant == ButtonSecondary:
			// Over the card it sits on, as Onevra's is over the page.
			bg := colors.Surface()
			return Mix(bg, text, 0.08), Mix(bg, text, 0.12), Mix(bg, text, 0.18), Mix(bg, text, 0.08)
		}
		return 0, colors.Hover(), colors.Pressed(), colors.Border()
	}
	var children []core.Node
	if b.Icon != "" {
		children = append(children, Icon(b.Icon, 14, fg))
	}
	children = append(children, Text(b.Text, 13, Regular, fg))
	face := func() core.Style {
		rest, hover, pressed, border := fill()
		return core.Style{Height: core.Pt(ButtonHeight), Padding: core.Symmetric(0, 14), CornerRadius: RadiusControl, BorderCurve: core.BorderCurveContinuous,
			Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 6,
			Background: rest, BorderWidth: 1, BorderColor: border,
			GroupStyle: &core.GroupStyle{Hover: &core.Style{Background: hover}, Active: &core.Style{Background: pressed}}}
	}
	return base.Button{
		OnPress:  func(core.PressEvent) { b.OnPress() },
		Disabled: b.Disabled, Label: b.Text, TestID: b.TestID,
		Style: core.Style{Group: "button", FlexGrow: b.Flex, FlexShrink: b.Flex, CornerRadius: RadiusControl, Cursor: core.CursorPointer,
			Disabled: &core.Style{Opacity: core.Alpha(0.4)}}.Merge(slop(cx, ButtonHeight)),
		StyleFunc: func() core.Style { return core.Style{Focus: FocusRing(cx, 2)} },
		Children:  []core.Node{core.View{StyleFunc: face, Children: children}},
	}
}

// RoundButton is a round icon button, 32 pt unless Size says otherwise:
// the inverse of the page as ButtonPrimary, the default, and Onevra's
// raised toolbar face otherwise. IconFunc and LabelFunc replace Icon and
// Label.
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
		size = 32
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
			s.Background = colors.Light()
		} else {
			s = FinOf(cx).Raised().Over(s)
			s.Background, s.BorderWidth, s.BorderColor = colors.Raised(), 1, colors.Border()
			s.GroupStyle = &core.GroupStyle{Hover: &core.Style{Background: colors.Hover()}, Active: &core.Style{Background: colors.Pressed()}}
		}
		return s
	}
	return Press{OnPress: b.OnPress, Scale: 0.96, LabelFunc: label, HitSlop: max(0, (44-size)/2), Style: core.Style{Group: "button", Cursor: core.CursorPointer},
		Children: []core.Node{core.View{StyleFunc: face, Children: []core.Node{IconOf(name, size*0.45, fg)}}}}
}

// Switch is an on and off switch in Onevra's look: base.Switch, the knob
// sliding across and the track changing color on the platform. Label is
// what assistive technology reads.
type Switch struct {
	Checked  func() bool
	OnChange func(bool)
	Label    string
}

// Render implements core.Node.
func (s Switch) Render(cx *core.Scope) core.Node {
	look := NewSwitchLook(cx, s.Checked, false)
	// More to press on every side, without taking more room.
	return base.Switch{Checked: s.Checked, OnChange: s.OnChange, Label: s.Label,
		Style:          core.Style{Margin: core.All(-8), Padding: core.All(8), Cursor: core.CursorPointer},
		TrackStyleFunc: look.Track, ThumbStyleFunc: look.Thumb}
}

// SwitchLook is Onevra's switch as the styles of base.Switch's parts: a
// track that turns to the accent and a capsule knob that slides across,
// both on the platform.
type SwitchLook struct {
	Track, Thumb func() core.Style
}

// NewSwitchLook makes a switch's look, 40 × 20 or, small, 30 × 16 for
// dense rows. on is the value the app owns; the knob follows it.
func NewSwitchLook(cx *core.Scope, on func() bool, small bool) SwitchLook {
	colors := ColorsOf(cx)
	width, height, pad := core.Points(40), core.Points(20), core.Points(2)
	if small {
		width, height = 30, 16
	}
	knobH := height - 2*pad
	knobW := knobH * 1.3
	start := 0.0
	shown := core.Untracked(cx, on)
	if shown {
		start = 1
	}
	t := core.NewAnimatedValue(cx, start)
	x := Lerp(t, 0, float64(width-2*pad-knobW))
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
			Timing(cx, t, to, switchTiming)
		}
	})
	track := func() core.Style {
		return core.Style{Width: core.Pt(width), Height: core.Pt(height), CornerRadius: height / 2, Padding: core.All(pad), Background: colors.Track(),
			Animated: &core.AnimatedStyle{Background: core.Interpolate(t, core.Interpolation{Input: []float64{0, 1}, Colors: []core.Color{colors.Track(), colors.AccentFill()}})}}
	}
	return SwitchLook{Track: track, Thumb: func() core.Style {
		return core.Style{Width: core.Pt(knobW), Height: core.Pt(knobH), CornerRadius: knobH / 2, Background: colors.Knob(),
			Shadows: FinOf(cx).Knob().Shadows, Animated: &core.AnimatedStyle{TranslateX: x}}
	}}
}

// Option is one segment of a segmented control; Count shows a quiet
// number after the label when set.
type Option[T comparable] struct {
	Value T
	Label string
	Count func() int
}

// Segmented is Onevra's segmented control: equal segments in a recessed
// capsule, the selected one on a raised thumb that slides to it, 28 pt
// tall: base.Tabs, a tab per segment, in the gallery's look. Small is the
// same control.
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
	// The track's padding: the segments share what is left of the
	// measured width.
	const pad = 2
	segment := func() core.Points { return (width.Get() - 2*pad) / core.Points(max(1, len(sc.Options))) }
	height := ButtonHeight + 2*pad
	// The first placement jumps; later ones slide.
	placed := false
	env := cx.Env()
	cx.Effect(func() {
		to := float64(core.Points(index()) * segment())
		if width.Get() <= 0 {
			return
		}
		if placed && !env.ReduceMotion() {
			Spring(cx, x, to, SettleSpring)
		} else {
			x.Set(to)
		}
		placed = true
	})
	children := []core.Node{core.View{PointerEvents: core.PointerEventsNone, StyleFunc: func() core.Style {
		return FinOf(cx).Pill().Over(core.Style{Position: core.Absolute, Top: core.Pt(pad), Left: core.Pt(pad), Bottom: core.Pt(pad),
			Width: core.Pt(max(0, segment())), CornerRadius: (height - 2*pad) / 2,
			Animated: &core.AnimatedStyle{TranslateX: x}})
	}}}
	for _, o := range sc.Options {
		selected := func() bool { return sc.Value() == o.Value }
		label := []core.Node{core.Text{Content: o.Label, MaxLines: 1, StyleFunc: func() core.Style {
			c, w := Alpha(colors.Text(), 0.6), Regular
			if selected() {
				c, w = colors.Text(), Semibold
			}
			return core.Style{FontSize: 13, FontWeight: w, Color: c}
		}}}
		if o.Count != nil {
			label = append(label, Textf(func() string { return fmt.Sprint(o.Count()) }, 12, Regular, colors.Faint))
		}
		children = append(children, base.Tab{Selected: selected, OnPress: func(core.PressEvent) { sc.OnChange(o.Value) }, Label: o.Label,
			Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 5,
				Padding: core.Symmetric(0, 12), Cursor: core.CursorPointer},
			Children: label})
	}
	return core.View{OnLayout: func(b core.Bounds) { width.Set(b.Size.Width) }, Children: []core.Node{base.Tabs{
		Style:     core.Style{Height: core.Pt(height), Direction: core.Row, Padding: core.All(pad), CornerRadius: height / 2},
		StyleFunc: func() core.Style { return core.Style{Background: Alpha(colors.Text(), 0.06)} },
		Children:  children,
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
	return core.View{Style: core.Style{Height: core.Pt(6), CornerRadius: 3}, StyleFunc: Filled(colors.Border), Children: children}
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
			StyleFunc: Filled(colors.Border)}}, s.Children...),
	}
}

// Field is an Input in Onevra's text box: the field's color and a rim;
// the input's own style sets its size and padding.
type Field struct{ Input base.Input }

// Render implements core.Node.
func (f Field) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	in := f.Input
	in.Style = core.Style{FontSize: 13}.Merge(in.Style)
	own := in.StyleFunc
	in.StyleFunc = func() core.Style {
		s := core.Style{Color: colors.Text(), PlaceholderColor: colors.Faint(), SelectionColor: colors.Accent()}
		if own != nil {
			s = s.Merge(own())
		}
		return s
	}
	return core.View{StyleFunc: func() core.Style {
		return core.Style{CornerRadius: RadiusControl, BorderCurve: core.BorderCurveContinuous,
			BorderWidth: 1, BorderColor: colors.FieldBorder(), Background: colors.Field()}
	}, Children: []core.Node{in}}
}

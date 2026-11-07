package main

import (
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/desktop"
	"github.com/oneo-me/nsgo/icons/lucide"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// The parts the gallery's demos share, in ui's look.

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
	t := ui.Textf(fn, 12, ui.Regular, ui.ColorsOf(cx).Muted)
	t.Style.LineHeight = 17
	return t
}

// Code is a monospaced line, for API names and values.
type Code struct{ Text string }

// Render implements core.Node.
func (c Code) Render(cx *core.Scope) core.Node {
	t := ui.Text(c.Text, 12, ui.Regular, ui.ColorsOf(cx).Code)
	t.Style.FontFamily = ui.Mono
	return t
}

// panelMotion eases a pressable panel between its states.
var panelMotion = core.Transition{Duration: 120 * time.Millisecond, Easing: core.EaseOut,
	Properties: core.TransitionBackground | core.TransitionBorder}

// Panel is a card: Head on the page, then the
// surface with the Children and the Footer after a hairline. Flush lays
// the children edge to edge, clipped to the surface's inner corners. OnPress
// makes the surface a button named Label.
type Panel struct {
	Head     core.Node
	Flush    bool
	Children []core.Node
	Footer   []core.Node
	OnPress  func()
	Label    string
	TestID   string
	// Style merges over the surface's.
	Style core.Style
}

// Render implements core.Node.
func (p Panel) Render(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	const inner = ui.RadiusCard - 1
	var parts []core.Node
	if len(p.Children) > 0 {
		body := core.Style{Gap: 12, Padding: core.All(14)}
		if p.Flush {
			r := core.Corners{TopLeft: inner, TopRight: inner}
			if len(p.Footer) == 0 {
				r.BottomLeft, r.BottomRight = inner, inner
			}
			body = core.Style{Overflow: core.Hidden, CornerRadii: r, BorderCurve: core.BorderCurveContinuous}
		}
		parts = append(parts, core.View{Style: body, Children: p.Children})
	}
	if len(p.Footer) > 0 {
		s := core.Style{Gap: 6, Padding: core.Symmetric(10, 14)}
		if len(parts) > 0 {
			s.Borders = core.Edges{Top: 1}
		}
		parts = append(parts, core.View{Style: s,
			StyleFunc: func() core.Style { return core.Style{BorderColors: core.EdgeColors{Top: colors.Line()}} }, Children: p.Footer})
	}
	surface := core.View{StyleFunc: func() core.Style { return ui.CardStyle(cx).Merge(p.Style) }, Children: parts}
	if p.OnPress != nil {
		surface.Role, surface.Label, surface.OnPress = core.RoleButton, p.Label, func(core.PressEvent) { p.OnPress() }
		surface.Style = core.Style{Transition: panelMotion}
		surface.StyleFunc = func() core.Style {
			colors := ui.ColorsOf(cx)
			return ui.CardStyle(cx).Merge(core.Style{Hover: &core.Style{BorderColor: colors.FieldBorder()},
				Active: &core.Style{Background: colors.Raised()}, Focus: ui.FocusRing(cx, 2)}).Merge(p.Style)
		}
	}
	if p.Head == nil {
		surface.TestID = p.TestID
		return surface
	}
	return core.View{TestID: p.TestID, Style: core.Style{Gap: 8}, Children: []core.Node{p.Head, surface}}
}

// Card is a demo's titled card: a bold Title with a Description, then
// Actions, above a panel of the Children and an optional Footer. UIOnly
// marks a component package base has no form of.
type Card struct {
	Title       string
	Description string
	UIOnly      bool
	Actions     []core.Node
	Children    []core.Node
	Footer      []core.Node
	// Flush lays the children edge to edge.
	Flush bool
	// OnPress makes the panel a button named Label, else Title.
	OnPress func()
	Label   string
	TestID  string
	// Style merges over the panel's.
	Style core.Style
}

// Render implements core.Node.
func (c Card) Render(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	var head core.Node
	if c.Title != "" {
		title := ui.Text(c.Title, 13, ui.Bold, colors.Text)
		title.Role = core.RoleHeader
		titles := []core.Node{title}
		if c.Description != "" {
			description := ui.Text(c.Description, 12, ui.Regular, colors.Muted)
			description.MaxLines = 2
			titles = append(titles, description)
		}
		nodes := []core.Node{core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: titles}}
		if c.UIOnly {
			nodes = append(nodes, core.View{TestID: "ui-only-" + c.Title, Style: core.Style{FlexShrink: 0}, Children: []core.Node{
				ui.Badge{Text: locales.For(cx).UiOnly(), Variant: ui.BadgeNeutral}}})
		}
		head = core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignEnd, Gap: 10, Padding: core.Symmetric(0, 2)}, Children: append(nodes, c.Actions...)}
	}
	label := c.Label
	if label == "" {
		label = c.Title
	}
	return Panel{TestID: c.TestID, Head: head, Flush: c.Flush, Children: c.Children, Footer: c.Footer, Style: c.Style, OnPress: c.OnPress, Label: label}
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
	c := ui.EnterTiming
	c.Delay = time.Duration(min(8, a.Index)) * 40 * time.Millisecond
	ui.Timing(cx, t, 1, c)
	s := core.Style{Animated: &core.AnimatedStyle{Opacity: t, TranslateY: ui.Lerp(t, 14, 0)}}
	if a.Flex {
		s = core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0), MinWidth: core.Pt(0)}.Merge(s)
	}
	return core.View{Style: s, Children: a.Children}
}

// IconTile is a rounded tile Size points square around an icon.
type IconTile struct {
	Icon func() *core.SVG
	Size core.Points
}

// Render implements core.Node.
func (t IconTile) Render(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	size := t.Size
	return core.View{StyleFunc: func() core.Style {
		return core.Style{
			Width: core.Pt(size), Height: core.Pt(size), CornerRadius: size * 0.25, BorderCurve: core.BorderCurveContinuous,
			BorderWidth: 1, BorderColor: colors.Border(), Background: colors.Raised(),
			AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
		}
	}, Children: []core.Node{ui.Icon(t.Icon, size*0.5, colors.Accent)}}
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
	colors := ui.ColorsOf(cx)
	note := s.NoteFunc
	if note == nil {
		note = constant(s.Note)
	}
	heading := ui.Text(s.Title, 13, ui.Bold, colors.Text)
	heading.Role, heading.Style.FlexGrow, heading.Style.FlexShrink = core.RoleHeader, 1, 1
	head := core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Gap: 8, Padding: core.Symmetric(0, 2)},
		Children: []core.Node{heading, ui.Textf(note, 12, ui.Regular, colors.Faint)}}
	return Panel{TestID: "section-" + s.Title, Head: head, Children: s.Children}
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
	colors := ui.ColorsOf(cx)
	label := c.LabelFunc
	if label == nil {
		label = constant(c.Label)
	}
	l := ui.Textf(label, 13, ui.Regular, colors.Muted)
	if c.Note == nil && c.LabelFunc != nil {
		return l
	}
	l.Style.FlexGrow, l.Style.FlexShrink = 1, 1
	children := []core.Node{l}
	if c.Note != nil {
		children = append(children, ui.Textf(c.Note, 12, ui.Regular, colors.Faint))
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
	colors := ui.ColorsOf(cx)
	spans := []core.Span{{ContentFunc: f.Value}}
	if f.Unit != "" {
		spans = append(spans, core.Span{Content: " " + f.Unit, Style: core.Style{FontSize: 17, FontWeight: ui.Regular}, StyleFunc: ui.Colored(colors.Muted)})
	}
	if f.Note != nil {
		spans = append(spans, core.Span{ContentFunc: func() string { return " " + f.Note() }, Style: core.Style{FontSize: 15, FontWeight: ui.Medium},
			StyleFunc: ui.Colored(f.Tone.Shade(cx))})
	}
	return core.Text{Spans: spans, Style: core.Style{FontSize: 32, FontWeight: ui.Semibold, LetterSpacing: -0.5},
		StyleFunc: func() core.Style { return core.Style{Color: colors.Text()} }}
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
		ui.Text(s.Label, 12, ui.Regular, ui.ColorsOf(cx).Muted),
		ui.Textf(s.Value, 14, ui.Medium, s.Tone.Foreground(cx)),
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
	colors := ui.ColorsOf(cx)
	spans := []core.Span{{Content: l.Label}}
	if l.Value != nil {
		spans = append(spans, core.Span{ContentFunc: func() string { return " " + l.Value() }, StyleFunc: ui.Colored(colors.Text)})
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6}, Children: []core.Node{
		core.View{Style: core.Style{Width: core.Pt(9), Height: core.Pt(9), CornerRadius: 2.5}, StyleFunc: ui.Filled(l.Color)},
		core.Text{Spans: spans, Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
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
	colors := ui.ColorsOf(cx)
	v := core.View{Style: core.Style{MinHeight: core.Pt(ui.RowHeight), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10},
		StyleFunc: func() core.Style {
			var s core.Style
			if !r.First {
				s.Borders, s.BorderColors = core.Edges{Top: 1}, core.EdgeColors{Top: colors.Line()}
			}
			return s
		}, Children: r.Children}
	if r.OnPress != nil {
		v.OnPress = func(core.PressEvent) { r.OnPress() }
	}
	return v
}

// Hint is instructions for the device's input: touch on phones and
// tablets, pointer on desktops.
func Hint(cx *core.Scope, touch, pointer string) string {
	if cx.Env().Desktop() {
		return pointer
	}
	return touch
}

// BindRefresh is desktops' way to refresh what phones pull down: Ctrl+R /
// Cmd+R calls refresh while the calling component shows. Browsers keep
// the key to reload the page. It returns the key's label, or "" where it
// is not bound.
func BindRefresh(cx *core.Scope, refresh func()) string {
	if !desktop.IsAvailable(cx) {
		return ""
	}
	cx.OnKey("mod+r", refresh)
	return cx.KeyLabel("mod+r")
}

// API is the names of the API a demo shows, in small faint code.
type API struct{ Text string }

// Render implements core.Node.
func (a API) Render(cx *core.Scope) core.Node {
	return core.Text{Content: a.Text, Style: core.Style{FontSize: 12, FontFamily: ui.Mono}, StyleFunc: ui.Colored(ui.ColorsOf(cx).Faint)}
}

// APILine is what a demo shows on the left and the API's Names on the
// right.
type APILine struct{ Label, Names string }

// Render implements core.Node.
func (a APILine) Render(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Gap: 8}, Children: []core.Node{
		core.Text{Content: a.Label, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
		API{Text: a.Names},
	}}
}

// SwitchRow is a setting: a Label with its API's Code and a Note under
// it, and a switch at the end.
type SwitchRow struct {
	Label, Code, Note string
	Checked           func() bool
	OnChange          func(bool)
}

// Render implements core.Node.
func (r SwitchRow) Render(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	titles := []core.Node{ui.Text(r.Label, 13, ui.Regular, colors.Text)}
	if r.Code != "" {
		titles = append(titles, API{Text: r.Code})
	}
	if r.Note != "" {
		titles = append(titles, ui.Text(r.Note, 12, ui.Regular, colors.Muted))
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, MinHeight: core.Pt(ui.RowHeight)}, Children: []core.Node{
		core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: titles},
		ui.Switch{Checked: r.Checked, OnChange: r.OnChange, Label: r.Label},
	}}
}

// LinkRow is a row that opens a page: an icon tile, a Title and the
// API's name as its Note, a chevron. Divided draws a hairline above it.
type LinkRow struct {
	Icon        func() *core.SVG
	Title, Note string
	Divided     bool
	OnPress     func()
}

// Render implements core.Node.
func (r LinkRow) Render(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	row := core.View{OnPress: func(core.PressEvent) { r.OnPress() }, Role: core.RoleButton, Label: r.Title, Children: []core.Node{
		core.View{Style: core.Style{MinHeight: core.Pt(44), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Padding: core.Symmetric(6, 8), CornerRadius: ui.RadiusControl},
			Children: []core.Node{
				IconTile{Icon: r.Icon, Size: 28},
				core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: []core.Node{
					ui.Text(r.Title, 13, ui.Medium, colors.Text),
					core.Text{Content: r.Note, MaxLines: 1, Style: core.Style{FontSize: 12, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Faint)},
				}},
				ui.Icon(lucide.ChevronRight, 14, colors.Faint),
			}},
	}}
	if !r.Divided {
		return row
	}
	return core.View{StyleFunc: func() core.Style {
		return core.Style{Borders: core.Edges{Top: 1}, BorderColors: core.EdgeColors{Top: colors.Line()}}
	}, Children: []core.Node{row}}
}

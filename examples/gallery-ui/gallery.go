package ui

import (
	"github.com/oneo-me/nsgo/icons/lucide"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/desktop"
)

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
	Icon func() *core.SVG
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
	return core.Text{Content: a.Text, Style: core.Style{FontSize: 12, FontFamily: Mono}, StyleFunc: Colored(ColorsOf(cx).Faint)}
}

// APILine is what a demo shows on the left and the API's Names on the
// right.
type APILine struct{ Label, Names string }

// Render implements core.Node.
func (a APILine) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Gap: 8}, Children: []core.Node{
		core.Text{Content: a.Label, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 13}, StyleFunc: Colored(colors.Muted)},
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
	colors := ColorsOf(cx)
	titles := []core.Node{Text(r.Label, 13, Regular, colors.Text)}
	if r.Code != "" {
		titles = append(titles, API{Text: r.Code})
	}
	if r.Note != "" {
		titles = append(titles, Text(r.Note, 12, Regular, colors.Muted))
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, MinHeight: core.Pt(rowHeight(cx))}, Children: []core.Node{
		core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: titles},
		Switch{Checked: r.Checked, OnChange: r.OnChange, Label: r.Label},
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
	colors := ColorsOf(cx)
	row := Press{OnPress: r.OnPress, Scale: 0.98, Label: r.Title, Style: core.Style{Group: "row", Cursor: core.CursorPointer}, Children: []core.Node{
		core.View{Style: core.Style{MinHeight: core.Pt(44), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Padding: core.Symmetric(6, 8), CornerRadius: RadiusControl},
			StyleFunc: func() core.Style {
				return core.Style{GroupStyle: &core.GroupStyle{Hover: &core.Style{Background: colors.Hover()}}}
			}, Children: []core.Node{
				IconTile{Icon: r.Icon, Size: 28},
				core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: []core.Node{
					Text(r.Title, 13, Medium, colors.Text),
					core.Text{Content: r.Note, MaxLines: 1, Style: core.Style{FontSize: 12, FontFamily: Mono}, StyleFunc: Colored(colors.Faint)},
				}},
				Icon(lucide.ChevronRight, 14, colors.Faint),
			}},
	}}
	if !r.Divided {
		return row
	}
	return core.View{StyleFunc: func() core.Style {
		return core.Style{Borders: core.Edges{Top: 1}, BorderColors: core.EdgeColors{Top: colors.Line()}}
	}, Children: []core.Node{row}}
}

// Tool is Onevra's toolbar button, a raised capsule 28 pt tall: its Text
// after a dot that lights with the accent while On, when On is set. Label
// is what assistive technology reads.
type Tool struct {
	Label, Text string
	On          func() bool
	OnPress     func()
}

// Render implements core.Node.
func (t Tool) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	fin := FinOf(cx)
	children := []core.Node{}
	if t.On != nil {
		children = append(children, core.View{StyleFunc: func() core.Style {
			c := colors.Faint
			if t.On() {
				c = colors.Accent
			}
			return core.Style{Width: core.Pt(6), Height: core.Pt(6), CornerRadius: 3, Background: c()}
		}})
	}
	children = append(children, Text(t.Text, 13, Regular, colors.Text))
	return base.Button{OnPress: func(core.PressEvent) { t.OnPress() }, Selected: t.On, Label: t.Label,
		StyleFunc: func() core.Style {
			return fin.Raised().Over(core.Style{Height: core.Pt(ButtonHeight), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8,
				CornerRadius: ButtonHeight / 2, BorderWidth: 1, BorderColor: Alpha(colors.Border(), 0.8), Cursor: core.CursorPointer,
				Padding: core.Symmetric(0, 12), Background: colors.Raised(), Transition: core.Transition{Duration: 150e6},
				Hover: &core.Style{Background: colors.Hover()}, Active: &core.Style{Background: colors.Pressed()}})
		},
		Children: children}
}

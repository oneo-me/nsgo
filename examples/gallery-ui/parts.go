package ui

import (
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/desktop"
)

// The parts several demos share: layout helpers, rows, a toolbar's
// button, a field and resize handles' looks.

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

// HStack lays its children out in a row with a gap, centered across.
type HStack struct {
	Gap      core.Points
	Children []core.Node
}

// Render implements core.Node.
func (h HStack) Render(*core.Scope) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: h.Gap}, Children: h.Children}
}

// Wrap lays its children out in a row that wraps, 8 pt apart.
type Wrap struct{ Children []core.Node }

// Render implements core.Node.
func (w Wrap) Render(*core.Scope) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, AlignItems: core.AlignCenter, Gap: 8}, Children: w.Children}
}

// Spacer takes the free space of a row.
type Spacer struct{}

// Render implements core.Node.
func (Spacer) Render(*core.Scope) core.Node { return core.View{Style: core.Style{FlexGrow: 1}} }

// Cell is an equal share of a row for its Child.
type Cell struct{ Child core.Node }

// Render implements core.Node.
func (c Cell) Render(*core.Scope) core.Node {
	return core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pt(0)}, Children: []core.Node{c.Child}}
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
	titles := []core.Node{Text(r.Label, 15, Regular, colors.Text)}
	if r.Code != "" {
		titles = append(titles, API{Text: r.Code})
	}
	if r.Note != "" {
		titles = append(titles, Text(r.Note, 13, Regular, colors.Faint))
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, MinHeight: core.Pt(44)}, Children: []core.Node{
		core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: titles},
		Switch{Checked: r.Checked, OnChange: r.OnChange, Label: r.Label},
	}}
}

// LinkRow is a row that opens a page: an icon tile, a Title and the
// API's name as its Note, a chevron. Divided draws a hairline above it.
type LinkRow struct {
	Icon        IconName
	Title, Note string
	Divided     bool
	OnPress     func()
}

// Render implements core.Node.
func (r LinkRow) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	row := Press{OnPress: r.OnPress, Scale: 0.98, Label: r.Title, Children: []core.Node{
		core.View{Style: core.Style{MinHeight: core.Pt(52), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Padding: core.Symmetric(6, 8)}, Children: []core.Node{
			IconTile{Icon: r.Icon, Size: 32},
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: []core.Node{
				Text(r.Title, 15, Medium, colors.Text),
				core.Text{Content: r.Note, MaxLines: 1, Style: core.Style{FontSize: 12, FontFamily: Mono}, StyleFunc: Colored(colors.Faint)},
			}},
			Icon(IconChevron, 14, colors.Faint),
		}},
	}}
	if !r.Divided {
		return row
	}
	return core.View{StyleFunc: func() core.Style {
		return core.Style{Borders: core.Edges{Top: 1}, BorderColors: core.EdgeColors{Top: colors.Line()}}
	}, Children: []core.Node{row}}
}

// Tool is a toolbar's button, 32 pt tall: its Text after a dot that
// lights with the accent while On, when On is set. Label is what
// assistive technology reads.
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
	children = append(children, Text(t.Text, 13, Semibold, colors.Text))
	return base.Button{OnPress: func(core.PressEvent) { t.OnPress() }, Selected: t.On, Label: t.Label,
		StyleFunc: func() core.Style {
			return fin.Raised().Over(core.Style{Height: core.Pt(32), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, CornerRadius: 8, BorderWidth: 1, BorderColor: colors.Border(),
				Padding: core.Symmetric(0, 12), Background: colors.Raised(), Transition: core.Transition{Duration: 150e6}, Active: &core.Style{Transform: core.Transform{Scale: 0.97}}})
		},
		Children: children}
}

// Input is a base.Input styled as a field of its own, 44 pt tall, its
// rim the accent while focused.
type Input struct{ Input base.Input }

// Render implements core.Node.
func (f Input) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	fin := FinOf(cx)
	in := f.Input
	focused := core.NewSignal(cx, false)
	onFocus, onBlur := in.OnFocus, in.OnBlur
	in.OnFocus = func() {
		focused.Set(true)
		if onFocus != nil {
			onFocus()
		}
	}
	in.OnBlur = func() {
		focused.Set(false)
		if onBlur != nil {
			onBlur()
		}
	}
	in.StyleFunc = func() core.Style {
		border := colors.FieldBorder
		if focused.Get() {
			border = colors.Accent
		}
		return fin.Field().Over(core.Style{PlaceholderColor: colors.Faint(), SelectionColor: colors.Accent(), Height: core.Pt(44),
			Padding: core.Symmetric(0, 12), FontSize: 15, Color: colors.Text(), Background: colors.Field(),
			BorderWidth: 1, BorderColor: border(), CornerRadius: RadiusControl})
	}
	return in
}

// HandleThumb is the thumb of a resize handle, as gpui-component draws
// it: none at rest, so the hairlines stay quiet; a faint pill grows in
// under the pointer, and a long solid one while pressed or dragged.
func HandleThumb() core.Style {
	return core.Style{Height: core.Pt(44), Width: core.Pt(3), CornerRadius: 1.5, Opacity: core.Alpha(0.001),
		Transform:  core.Transform{ScaleY: 0.001},
		Transition: core.Transition{Duration: 150 * time.Millisecond, Easing: core.EaseOut},
		GroupStyle: &core.GroupStyle{
			Hover:  &core.Style{Opacity: core.Alpha(0.4), Transform: core.Transform{ScaleY: 0.45}},
			Active: &core.Style{Opacity: core.Alpha(0.9), Transform: core.Transform{ScaleY: 1}},
		}}
}

// HandleThumbColor is the thumb's color, for a ThumbStyleFunc.
func HandleThumbColor(cx *core.Scope) func() core.Style { return Filled(ColorsOf(cx).Muted) }

// HandleLine is a handle's line, which stays as it is: its thumb shows
// where to drag. It is a StyleFunc.
func HandleLine(cx *core.Scope) func() core.Style { return Filled(ColorsOf(cx).Border) }

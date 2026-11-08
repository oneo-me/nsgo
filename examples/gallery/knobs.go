package main

import (
	"fmt"
	"slices"

	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// What sets a component's props beside it: knobs, each named after the
// prop it sets, in a Playground's card or the panel of a stagePage.

// knobSliderWidth is how wide a numberKnob's slider is.
const knobSliderWidth core.Points = 180

// formWidth is the most a Fill form takes of its stage, centered in it.
const formWidth core.Points = 360

// stageWidth is the narrowest pane that fits a stagePage's panel beside
// its stage.
const stageWidth core.Points = 760

// Knob is a control under Name, the API it sets. The control fills what
// the name leaves, unless Tight.
type Knob struct {
	Name    string
	Tight   bool
	Control core.Node
}

// Render implements core.Node.
func (k Knob) Render(cx *core.Scope) core.Node {
	align := core.AlignStretch
	if k.Tight {
		align = core.AlignEnd
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, MinHeight: core.Pt(28)}, Children: []core.Node{
		core.View{Style: core.Style{FlexShrink: 0, MinWidth: core.Pt(88)}, Children: []core.Node{API{Text: k.Name}}},
		core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), AlignItems: align}, Children: []core.Node{k.Control}},
	}}
}

// boolKnob sets a boolean prop.
func boolKnob(name string, s *core.Signal[bool]) core.Node {
	return Knob{Name: name, Tight: true, Control: ui.Switch{Checked: s.Get, OnChange: s.Set, Label: name, Size: ui.SizeSmall}}
}

// choiceKnob sets a prop to one of options: small tabs at the row's end
// while they fit one, else a select.
func choiceKnob[T comparable](name string, s *core.Signal[T], options ...ui.Option[T]) core.Node {
	if len(options) > 4 {
		return Knob{Name: name, Control: ui.Select[T]{Label: name, TestID: "knob-" + name, Options: options, Value: s.Get, OnChange: s.Set, Style: core.Style{Width: core.Pct(100)}}}
	}
	return Knob{Name: name, Tight: true, Control: ui.Tabs[T]{Size: ui.SizeSmall, Label: name, Options: options, Value: s.Get, OnChange: s.Set,
		Style: core.Style{AlignSelf: core.AlignEnd}}}
}

// numberKnob sets a number from lo to hi, step by step: its value, then a
// slider at the row's end.
func numberKnob(name string, s *core.Signal[float64], lo, hi, step float64) core.Node {
	return Knob{Name: name, Tight: true, Control: core.Func(func(cx *core.Scope) core.Node {
		value := ui.Textf(func() string { return fmt.Sprintf("%.4g", s.Get()) }, 12, ui.Regular, ui.ColorsOf(cx).Muted)
		value.Style.MinWidth = core.Pt(32)
		value.Style.TextAlign = core.TextAlignEnd
		return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, MaxWidth: core.Pct(100)}, Children: []core.Node{
			ui.Tabular(value),
			ui.Slider{Label: name, Min: lo, Max: hi, Step: step, Value: s.Get, OnChange: s.Set, Style: core.Style{Width: core.Pt(knobSliderWidth), FlexShrink: 1}},
		}}
	})}
}

// stageRow lays a stage's variants side by side, on more rows where they do
// not fit.
func stageRow(variants ...core.Node) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 12}, Children: variants}
}

// stageRows stacks what is a row wide itself, such as a group of tabs, each
// centered on a row of its own.
func stageRows(forms ...core.Node) core.Node {
	rows := make([]core.Node, len(forms))
	for i, f := range forms {
		rows[i] = stageRow(f)
	}
	return core.View{Style: core.Style{Gap: 12}, Children: rows}
}

// sizes are a component's sizes, small to large.
var sizes = []ui.Size{ui.SizeSmall, ui.SizeDefault, ui.SizeLarge}

// options are a choiceKnob's options, each labeled by the value's name.
func options[T comparable](values []T, names ...string) []ui.Option[T] {
	out := make([]ui.Option[T], len(values))
	for i, v := range values {
		out[i] = ui.Option[T]{Value: v, Label: names[i]}
	}
	return out
}

// Form is a form of a component on a Playground's stage. Stage builds it,
// again whenever a signal it reads changes. It centers on the stage; Fill
// gives what has no width of its own formWidth to stretch across, and Start
// stretches it across the whole stage from the top instead, for what lists
// its results downward.
type Form struct {
	Name  string
	Fill  bool
	Start bool
	Stage func() core.Node
}

// Playground is a card of a component, edge to edge, above the knobs that
// set its props. Base is its form in package base, above Stage, its ui
// form, Title, Fill and Start being that form's. BaseKnobs set the props base has
// on both forms, and a hairline keeps Knobs, ui's own, under them. Stage
// shows every variant at the default size; Sizes, a block of its own under
// it, shows every size of the default variant. Event, when set, is the
// last thing the forms reported, under them, for what shows no change of
// its own.
type Playground struct {
	Title     string
	Note      string
	UIOnly    bool
	Fill      bool
	Start     bool
	Base      Form
	Stage     func() core.Node
	Sizes     func() core.Node
	BaseKnobs []core.Node
	Knobs     []core.Node
	Event     func() string
}

// Render implements core.Node.
func (p Playground) Render(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	hairline := core.View{Style: core.Style{Height: core.Pt(1)}, StyleFunc: ui.Filled(colors.Line)}
	form := Form{Name: p.Title, Fill: p.Fill, Start: p.Start, Stage: p.Stage}
	stages := []core.Node{form.stage("")}
	if p.Base.Stage != nil {
		stages = []core.Node{p.Base.stage(p.Base.Name), hairline, form.stage("ui")}
	}
	if p.Sizes != nil {
		stages = append(stages, hairline, Form{Name: p.Title + "-sizes", Fill: p.Fill, Stage: p.Sizes}.stage("Size"))
	}
	if p.Event != nil {
		stages = append(stages, hairline, eventLine(cx, p.Event))
	}
	knobs := slices.Clone(p.BaseKnobs)
	if len(knobs) > 0 && len(p.Knobs) > 0 {
		knobs = append(knobs, core.View{Style: core.Style{Height: core.Pt(1), Margin: core.Symmetric(4, 0)}, StyleFunc: ui.Filled(colors.Line)})
	}
	knobs = append(knobs, p.Knobs...)
	var footer []core.Node
	if len(knobs) > 0 {
		footer = []core.Node{core.View{Style: core.Style{Gap: 2}, Children: knobs}}
	}
	return Card{Title: p.Title, Description: p.Note, UIOnly: p.UIOnly, Flush: true, Children: stages, Footer: footer}
}

// stage is the form on the pixel grid, with caption in its corner when it
// shares the card with another form.
func (f Form) stage(caption string) core.Node {
	s := core.Style{MinHeight: core.Pt(96), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 8, Padding: core.All(16)}
	// As wide as the form, which then centers even if it starts itself.
	box := core.Style{MaxWidth: core.Pct(100), AlignItems: core.AlignCenter}
	if f.Fill {
		box = core.Style{Width: core.Pct(100), MaxWidth: core.Pt(formWidth), AlignItems: core.AlignStretch}
	}
	if f.Start {
		s.AlignItems, s.JustifyContent = core.AlignStretch, core.JustifyStart
		box = core.Style{AlignItems: core.AlignStretch}
	}
	children := []core.Node{core.View{TestID: "form-" + f.Name, Style: box,
		Children: []core.Node{core.Dynamic(func(*core.Scope) core.Node { return f.Stage() })}}}
	if caption != "" {
		s.Padding.Top = 30
		children = append(children, core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Top: core.Pt(8), Left: core.Pt(14)},
			Children: []core.Node{API{Text: caption}}})
	}
	return PixelGrid{TestID: "stage-" + f.Name, Style: s, Children: children}
}

// eventLine is the last event a playground's forms reported, a dash until
// there is one.
func eventLine(cx *core.Scope, last func() string) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Padding: core.Symmetric(8, 16)}, Children: []core.Node{
		ui.Text(tr.LastEvent(), 12, ui.Regular, colors.Muted),
		core.Text{TestID: "last-event", MaxLines: 1, ContentFunc: func() string {
			if e := last(); e != "" {
				return e
			}
			return "—"
		}, Style: core.Style{FlexShrink: 1, FontSize: 12, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Text)},
	}}
}

// wideStage reports whether the pane fits a stagePage's panel beside its
// stage.
func (g *gallery) wideStage() bool {
	if g.desktop() {
		return g.paneWidth() >= stageWidth
	}
	return g.env.Window().Width >= stageWidth
}

// stagePage is a complex component's page: the component fills it and the
// panel, which scrolls on its own, sets its options, beside it where the
// pane is wide enough and under it where not.
func stagePage(cx *core.Scope, stage core.Node, panel ...core.Node) core.Node {
	colors := ui.ColorsOf(cx)
	return core.View{StyleFunc: func() core.Style {
		direction := core.Column
		if shell.wideStage() {
			direction = core.Row
		}
		return fillPage.Merge(core.Style{Direction: direction, Background: colors.Background()})
	}, Children: []core.Node{
		core.View{TestID: "stage", StyleFunc: func() core.Style {
			// The panel beside it scrolls to the page's edge, so only the
			// stage keeps the page's margin there.
			margin := core.Edges{Top: 16, Right: 16, Left: 16}
			if shell.wideStage() {
				margin = core.Edges{Top: 16, Bottom: 16, Left: 16}
			}
			return core.Style{FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pt(0), MinHeight: core.Pt(240), MinWidth: core.Pt(0), Margin: margin}
		}, Children: []core.Node{stage}},
		core.Scroll{TestID: "stage-panel", StyleFunc: func() core.Style {
			if shell.wideStage() {
				return core.Style{Width: core.Pt(332), FlexShrink: 0}
			}
			return core.Style{MaxHeight: core.Pt(312), FlexShrink: 0}
		}, Children: []core.Node{core.View{Style: core.Style{Padding: core.All(16), Gap: 12}, Children: panel}}},
	}}
}

// statusLine is a value a page reads back, after its label.
func statusLine(cx *core.Scope, label, id string, value func() string) core.Node {
	colors := ui.ColorsOf(cx)
	return core.View{Style: core.Style{Direction: core.Row, Gap: 8, Padding: core.Symmetric(4, 0)}, Children: []core.Node{
		ui.Text(label, 12, ui.Regular, colors.Muted),
		core.Text{ContentFunc: value, TestID: id, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 12, FontFamily: ui.Mono, TextAlign: core.TextAlignEnd},
			StyleFunc: ui.Colored(colors.Text)},
	}}
}

// pressed reports a press of the component named name.
func pressed(cx *core.Scope, last *core.Signal[string], name string) func() {
	tr := locales.For(cx)
	return func() { last.Set(tr.KitPressed(name)) }
}

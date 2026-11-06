package main

import (
	"fmt"

	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// What sets a component's props beside it: knobs, each named after the
// prop it sets, in a Playground's card or the panel of a stagePage.

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
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, MinHeight: core.Pt(32)}, Children: []core.Node{
		core.View{Style: core.Style{FlexShrink: 0, MinWidth: core.Pt(88)}, Children: []core.Node{API{Text: k.Name}}},
		core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), AlignItems: align}, Children: []core.Node{k.Control}},
	}}
}

// boolKnob sets a boolean prop.
func boolKnob(name string, s *core.Signal[bool]) core.Node {
	return Knob{Name: name, Tight: true, Control: ui.Switch{Checked: s.Get, OnChange: s.Set, Label: name, Small: true}}
}

// choiceKnob sets a prop to one of options: a segmented control while
// they fit one, else a select.
func choiceKnob[T comparable](name string, s *core.Signal[T], options ...ui.Option[T]) core.Node {
	if len(options) > 4 {
		return Knob{Name: name, Control: ui.Select[T]{Label: name, TestID: "knob-" + name, Options: options, Value: s.Get, OnChange: s.Set, Style: core.Style{Width: core.Pct(100)}}}
	}
	return Knob{Name: name, Control: ui.Tabs[T]{Equal: true, Label: name, Options: options, Value: s.Get, OnChange: s.Set}}
}

// numberKnob sets a number from lo to hi, step by step.
func numberKnob(name string, s *core.Signal[float64], lo, hi, step float64) core.Node {
	return Knob{Name: name, Control: core.Func(func(cx *core.Scope) core.Node {
		value := ui.Textf(func() string { return fmt.Sprintf("%.4g", s.Get()) }, 12, ui.Regular, ui.ColorsOf(cx).Muted)
		value.Style.MinWidth = core.Pt(32)
		value.Style.TextAlign = core.TextAlignEnd
		return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
			ui.Slider{Label: name, Min: lo, Max: hi, Step: step, Value: s.Get, OnChange: s.Set, Style: core.Style{FlexGrow: 1, FlexShrink: 1}},
			ui.Tabular(value),
		}}
	})}
}

// options are a choiceKnob's options, each labeled by the value's name.
func options[T comparable](values []T, names ...string) []ui.Option[T] {
	out := make([]ui.Option[T], len(values))
	for i, v := range values {
		out[i] = ui.Option[T]{Value: v, Label: names[i]}
	}
	return out
}

// Playground is a card of a component above the knobs that set its props.
// Stage builds the component, again whenever a signal it reads changes;
// Fill stretches it across the stage, for what has no width of its own.
type Playground struct {
	Title string
	Note  string
	Fill  bool
	Stage func() core.Node
	Knobs []core.Node
}

// Render implements core.Node.
func (p Playground) Render(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	align := core.AlignCenter
	if p.Fill {
		align = core.AlignStretch
	}
	stage := core.View{TestID: "stage-" + p.Title, Style: core.Style{MinHeight: core.Pt(96), AlignItems: align, JustifyContent: core.JustifyCenter, Gap: 8,
		Padding: core.All(16), CornerRadius: ui.RadiusInset, BorderCurve: core.BorderCurveContinuous}, StyleFunc: ui.Filled(colors.Inset),
		Children: []core.Node{core.Dynamic(func(*core.Scope) core.Node { return p.Stage() })}}
	children := []core.Node{stage}
	if len(p.Knobs) > 0 {
		children = append(children, core.View{Style: core.Style{Gap: 4}, Children: p.Knobs})
	}
	return ui.Card{Title: p.Title, Description: p.Note, Children: children}
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

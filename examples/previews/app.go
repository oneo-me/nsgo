// Command previews shows the docs' component examples live: each demo is
// the code of a block in docs/public/*/ui/, with what the block leaves
// out defined around it. The website mounts it once per block marked
// preview=<id>, naming the demo in the launch parameter demo; run alone,
// it lists the demos to pick from.
package main

import (
	"slices"
	"strings"

	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/ui"
)

// demos are the previews by ID, a docs page's name and the block's.
var demos = map[string]func(cx *core.Scope) core.Node{}

// fill marks a demo that fills the stage rather than sitting in its
// middle, such as a list.
var fill = map[string]bool{}

func main() { nsgo.Run(Root) }

// Root shows the demo the host names, or every demo to pick from.
func Root(cx *core.Scope) core.Node {
	if id, _ := cx.HostParams()["demo"].(string); id != "" {
		return core.Func(stage(id))
	}
	return picker(cx)
}

// stage shows the demo id on the gallery's grid: in its middle, or
// filling it.
func stage(id string) func(cx *core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		colors := ui.ColorsOf(cx)
		demo, ok := demos[id]
		if !ok {
			demo = func(*core.Scope) core.Node {
				return ui.Text("No preview "+id, 13, ui.Regular, colors.Muted)
			}
		}
		style := core.Style{FlexGrow: 1, FlexShrink: 1, Padding: core.All(24), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}
		if fill[id] {
			style.AlignItems, style.JustifyContent, style.Padding = core.AlignStretch, core.JustifyStart, core.Edges{}
		}
		return grid{style: style, children: []core.Node{core.Func(demo)}}
	}
}

// picker lists the demos above the one picked.
func picker(cx *core.Scope) core.Node {
	ids := make([]string, 0, len(demos))
	for id := range demos {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	options := make([]ui.Option[string], len(ids))
	for i, id := range ids {
		options[i] = ui.Option[string]{Value: id, Label: id}
	}
	picked := core.NewSignal(cx, ids[0])
	colors := ui.ColorsOf(cx)
	return core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, PaddingEnv: core.EnvAll(core.EnvSafeArea)},
		StyleFunc: func() core.Style { return core.Style{Background: colors.Background()} },
		Children: []core.Node{
			core.View{Style: core.Style{Padding: core.All(12), Borders: core.Edges{Bottom: 1}},
				StyleFunc: func() core.Style { return core.Style{BorderColor: colors.Border()} },
				Children: []core.Node{
					ui.Select[string]{Options: options, Value: picked.Get, OnChange: picked.Set, Searchable: true, Label: "Preview"},
				}},
			core.Dynamic(func(*core.Scope) core.Node { return core.Func(stage(picked.Get())) }),
		},
	}
}

// add registers the demo id; a demo of a list or a scroll view fills the
// stage.
func add(id string, demo func(cx *core.Scope) core.Node) {
	demos[id] = demo
	if strings.HasPrefix(id, "list/") || strings.HasPrefix(id, "scroll/") {
		fill[id] = true
	}
}

// row and column lay out a demo's controls.
func row(children ...core.Node) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 8, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: children}
}

func column(children ...core.Node) core.Node {
	return core.View{Style: core.Style{Gap: 12, AlignItems: core.AlignCenter}, Children: children}
}

// form is a column of controls of one width.
func form(children ...core.Node) core.Node {
	return core.View{Style: core.Style{Gap: 12, Width: core.Pt(280), MaxWidth: core.Pct(100)}, Children: children}
}

// log shows what a demo's controls last did.
type log struct{ text *core.Signal[string] }

func newLog(cx *core.Scope) log { return log{core.NewSignal(cx, "")} }

// says returns a press that shows text.
func (l log) says(text string) func() { return func() { l.text.Set(text) } }

func (l log) Render(cx *core.Scope) core.Node {
	return ui.Textf(l.text.Get, 13, ui.Regular, ui.ColorsOf(cx).Muted)
}

// words returns what picks a demo's words in the app's language: Chinese
// where its locale is, as the docs' Chinese pages write them, and English
// elsewhere.
func words(cx *core.Scope) func(en, zh string) string {
	chinese := strings.HasPrefix(cx.Env().Locale(), "zh")
	return func(en, zh string) string {
		if chinese {
			return zh
		}
		return en
	}
}

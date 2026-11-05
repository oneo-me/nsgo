package ui

import (
	"testing"

	"github.com/oneo-me/nsgo/core"
)

// TestCollapsible opens a collapsible and an accordion's sections, one at
// a time.
func TestCollapsible(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			open := core.NewSignal(cx, false)
			section := core.NewSignal(cx, -1)
			return core.View{Style: core.Style{Gap: 16}, Children: []core.Node{
				Collapsible{Text: "Details", Open: open.Get, OnOpenChange: open.Set,
					Content: core.Text{Content: "Hidden details"}},
				Accordion{TestID: "accordion",
					Open: func(i int) bool { return section.Get() == i },
					OnOpenChange: func(i int, open bool) {
						if open {
							section.Set(i)
						} else {
							section.Set(-1)
						}
					},
					Items: []AccordionItem{{Text: "First", Content: core.Text{Content: "First body"}},
						{Text: "Second", Content: core.Text{Content: "Second body"}}}},
			}}
		})
		if app.Exists("Hidden details") {
			t.Fatal("closed content shows")
		}
		app.Press("role:button Details")
		app.Find("Hidden details")
		if !app.Find("role:button Details").Props.Expanded {
			t.Fatal("the open trigger is not expanded")
		}
		app.Press("role:button First")
		app.Find("First body")
		app.Press("role:button Second")
		app.Find("Second body")
		if app.Exists("First body") {
			t.Fatal("opening the second section left the first open")
		}
	})
}

package ui

import (
	"math"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
)

// TestTabs selects tabs of both variants and skips a disabled one.
func TestTabs(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			var lists []core.Node
			for v, name := range []string{"underline", "segmented"} {
				selected := core.NewSignal(cx, 0)
				lists = append(lists, Tabs{Variant: TabsVariant(v), Label: name, TestID: name, Selected: selected.Get, OnChange: selected.Set,
					Items: []TabItem{{Text: "One", TestID: name + "-0"}, {Text: "Two", TestID: name + "-1"},
						{Text: "Three", TestID: name + "-2", Disabled: func() bool { return true }}}})
			}
			return core.View{Style: core.Style{Gap: 16, AlignItems: core.AlignStart}, Children: lists}
		})
		for _, name := range []string{"underline", "segmented"} {
			app.Press(name + "-1")
			if !app.Find(name+"-1").Props.Selected || app.Find(name+"-0").Props.Selected {
				t.Fatalf("%s: pressing the second tab did not select it alone", name)
			}
			if !app.Find(name + "-2").Props.Disabled {
				t.Fatalf("%s: the third tab is not disabled", name)
			}
		}
		if got := app.Find("underline-1").Style.BorderColors.Bottom; got != p.Accent {
			t.Fatalf("underline = %x, want the accent", got)
		}
		if got := app.Find("segmented-1").Style.Background; got != p.Raised {
			t.Fatalf("segment = %x, want raised", got)
		}
	})
}

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

// TestProgress fills a determinate bar to its value and sweeps an
// indeterminate one.
func TestProgress(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			return core.View{Style: core.Style{Gap: 16}, Children: []core.Node{
				Progress{Label: "Upload", TestID: "upload", Value: func() float32 { return 40 }},
				Progress{Label: "Sync", TestID: "sync", Indeterminate: func() bool { return true }},
			}}
		})
		bar := app.Find("upload")
		track, fill := bar.Children[0], bar.Children[0].Children[0]
		if r := fill.Frame.Size.Width / track.Frame.Size.Width; math.Abs(float64(r)-0.4) > 0.01 {
			t.Fatalf("fill = %v of the track, want 0.4", r)
		}
		if bar.Props.Value.Now != 40 {
			t.Fatalf("value = %v, want 40", bar.Props.Value.Now)
		}
		sweep := func() core.Points { return app.Find("sync").Children[0].Children[0].Style.Transform.TranslateX }
		before := sweep()
		app.FastForward(time.Second)
		if sweep() == before {
			t.Fatal("the indeterminate bar does not sweep")
		}
	})
}

package ui

import (
	"testing"

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

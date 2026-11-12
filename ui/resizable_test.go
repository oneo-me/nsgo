package ui

import (
	"testing"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestResizablePanels draws the handle's hairline, with a thumb on
// phones, and keeps the app's style over it.
func TestResizablePanels(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			return core.View{Style: core.Style{FlexGrow: 1, Gap: 12}, Children: []core.Node{
				ResizablePanelGroup{HandleLabel: "Resize", TestID: "group", Style: core.Style{Height: core.Pt(200)},
					Panels: []base.ResizablePanel{
						{Size: func() core.Points { return 150 }, Children: []core.Node{core.Text{Content: "Start"}}},
						{Children: []core.Node{core.Text{Content: "End"}}},
					}},
				ResizablePanelGroup{HandleLabel: "Custom", Style: core.Style{Height: core.Pt(100)},
					HandleStyle: core.Style{Background: core.RGB(0x3366cc)},
					Panels:      []base.ResizablePanel{{MinSize: -1}, {MinSize: -1}}},
			}}
		})
		line := func(name string) *nsgotest.View { return app.Find("role:adjustable " + name).Children[0] }
		if got := line("Resize").Style.Background; got != p.Border {
			t.Fatalf("line = %x, want the border color", got)
		}
		if got := line("Custom").Style.Background; got != core.RGB(0x3366cc) {
			t.Fatalf("the app's line = %x, want its own color", got)
		}
		if e.Idiom != core.IdiomDesktop && len(app.Find("role:adjustable Resize").Children) < 2 {
			t.Fatal("the phone's handle has no thumb")
		}
	})
}

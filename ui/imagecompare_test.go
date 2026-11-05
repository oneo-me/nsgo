package ui

import (
	"testing"

	"github.com/oneo-me/nsgo/core"
)

// TestImageCompare moves the divider where it is pressed, along a drag
// and by a screen reader's steps.
func TestImageCompare(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var position *core.Signal[float32]
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			position = core.NewSignal(cx, float32(50))
			return core.View{Style: core.Style{Width: core.Pt(300)}, Children: []core.Node{
				ImageCompare{TestID: "compare", Label: "Divider", Position: position.Get, OnChange: position.Set,
					BeforeLabel: "Before", AfterLabel: "After",
					Before: core.View{Style: core.Style{FlexGrow: 1, Background: core.RGB(0x336699)}},
					After:  core.View{Style: core.Style{FlexGrow: 1, Background: core.RGB(0x999999)}}},
			}}
		})
		reveal := app.Find("compare").Children[1]
		if w := reveal.Frame.Size.Width; w != 150 {
			t.Fatalf("before shows %v of 300, want 150", w)
		}
		if w := reveal.Children[0].Frame.Size.Width; w != 300 {
			t.Fatalf("before is laid out %v wide, want the frame's 300", w)
		}
		app.Pan("compare", core.Point{X: 30}, core.Point{X: 60})
		if got := position.GetUntracked(); got != 70 {
			t.Fatalf("after a drag of 60 position = %v, want 70", got)
		}
		app.AccessibilityAction("compare", core.ActionDecrement)
		if got := position.GetUntracked(); got != 65 {
			t.Fatalf("after a step down position = %v, want 65", got)
		}
		if v := app.Find("role:adjustable Divider").Props.Value; v.Now != 65 || v.Max != 100 {
			t.Fatalf("value = %+v, want 65 of 100", v)
		}
	})
}

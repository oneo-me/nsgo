package ui

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
)

// TestImage keeps its well's ratio and shows the placeholder until the
// picture loads.
func TestImage(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			return core.View{Style: core.Style{Width: core.Pt(300)}, Children: []core.Node{
				Image{Source: core.Src("https://example.com/coast.jpg"), Label: "Coast", TestID: "coast"},
			}}
		})
		well := app.Find("role:image Coast").Parent
		if s := well.Frame.Size; s.Width != 300 || s.Height != 200 || well.Style.Background != p.Inset {
			t.Fatalf("well = %v, %x; want 300 by 200 on the inset color", s, well.Style.Background)
		}
		if len(well.Children) != 2 {
			t.Fatalf("well holds %d views while loading, want the placeholder and the picture", len(well.Children))
		}
		app.Load("coast", core.Size{Width: 1200, Height: 800})
		if len(well.Children) != 1 {
			t.Fatal("the placeholder stays over the loaded picture")
		}
	})
}

package ui

import (
	"testing"

	"github.com/oneo-me/nsgo/core"
)

// TestSlider moves a knob with a press on the track, a drag, the keys on
// desktops and assistive technology, snapped to its steps; a range's
// knobs never cross.
func TestSlider(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var value, low, high *core.Signal[float64]
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			value, low, high = core.NewSignal(cx, 20.0), core.NewSignal(cx, 20.0), core.NewSignal(cx, 60.0)
			return core.View{Style: core.Style{Gap: 16, Width: core.Pt(220)}, Children: []core.Node{
				Slider{TestID: "volume", Label: "Volume", Value: value.Get, OnChange: value.Set, Step: 5},
				Slider{TestID: "price", Label: "Price", LowLabel: "minimum", HighLabel: "maximum",
					Value: low.Get, OnChange: low.Set, High: high.Get, OnHighChange: high.Set},
			}}
		})
		app.Touch(viewOf(app.Find("volume").Children[0]), core.Point{X: 110, Y: 10}).Lift()
		if value.Get() != 50 {
			t.Fatalf("a press in the middle gave %v, want 50", value.Get())
		}
		app.Drag("volume-thumb", core.Point{X: 40}, core.Point{X: 51})
		if v := value.Get(); v < 70 || v > 80 || v != float64(int(v/5)*5) {
			t.Fatalf("a drag of 51 pt across 200 gave %v, want 75 in steps of 5", v)
		}
		app.AccessibilityAction("volume-thumb", core.ActionDecrement)
		if v := app.Find("label:Volume"); value.Get() != 70 || v.Props.Role != core.RoleAdjustable {
			t.Fatalf("value = %v, role %v; want 70 from an adjustable knob", value.Get(), v.Props.Role)
		}
		if e.Idiom == core.IdiomDesktop {
			app.Key("tab")
			app.Key("end")
			app.Key("left")
			if value.Get() != 95 {
				t.Fatalf("End then Left gave %v, want 95", value.Get())
			}
		}
		// The middle is nearer the high knob, which takes the press.
		app.Touch(viewOf(app.Find("price").Children[0]), core.Point{X: 110, Y: 10}).Lift()
		if low.Get() != 20 || high.Get() != 50 {
			t.Fatalf("range = %v–%v, want 20–50", low.Get(), high.Get())
		}
		for range 40 {
			app.AccessibilityAction("label:Price minimum", core.ActionIncrement)
		}
		if low.Get() != 50 || app.Find("price-low").Props.Value.Max != 50 {
			t.Fatalf("the low knob reached %v, want it stopped at the high one's 50", low.Get())
		}
	})
}

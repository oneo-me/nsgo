package ui

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
)

// TestRating scores by a press, clears with a press on the score, and
// reads as one adjustable element.
func TestRating(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var value *core.Signal[int]
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			value = core.NewSignal(cx, 2)
			return Rating{TestID: "rating", Label: "Rating", Value: value.Get, OnChange: value.Set}
		})
		app.Press("rating-4")
		if value.Get() != 4 {
			t.Fatalf("value = %d, want 4", value.Get())
		}
		app.Press("rating-4")
		if value.Get() != 0 {
			t.Fatalf("a press on the score left %d, want 0", value.Get())
		}
		app.AccessibilityAction("label:Rating", core.ActionIncrement)
		if v := app.Find("label:Rating"); value.Get() != 1 || v.Props.Role != core.RoleAdjustable {
			t.Fatalf("value = %d, role %v; want 1 from an adjustable element", value.Get(), v.Props.Role)
		}
	})
}

// TestRatingRTL trades the arrows right to left, on every press.
func TestRatingRTL(t *testing.T) {
	e := desktopEnv
	e.RTL = true
	var value *core.Signal[int]
	app, _ := start(t, e, func(cx *core.Scope) core.Node {
		value = core.NewSignal(cx, 2)
		return Rating{TestID: "rating", Label: "Rating", Value: value.Get, OnChange: value.Set}
	})
	app.Key("tab")
	app.Key("left")
	app.Key("left")
	if value.Get() != 4 {
		t.Fatalf("value = %d after Left twice, want 4", value.Get())
	}
}

package ui

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
)

// TestNumberInput steps the value with its buttons up to the range's
// end, reads what is typed to the step, gives back text that is no
// number, and scrubs with a drag across its handle.
func TestNumberInput(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var value *core.Signal[float64]
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			value = core.NewSignal(cx, 1.5)
			return NumberInput{TestID: "number", Label: "Opacity", Value: value.Get, OnChange: value.Set, Min: 0, Max: 2, Step: 0.5,
				Leading: lucide.SlidersHorizontal, IncrementLabel: "Increase", DecrementLabel: "Decrease"}
		})
		if got := app.Find("label:Opacity").Text; got != "1.5" {
			t.Fatalf("the field shows %q, want 1.5", got)
		}
		app.Press("label:Increase")
		if value.Get() != 2 || !app.Find("label:Increase").Props.Disabled {
			t.Fatalf("value = %v; want 2 with the increase button disabled at the end", value.Get())
		}
		app.Clear("label:Opacity")
		app.Type("label:Opacity", "0.7")
		app.Submit("label:Opacity")
		if value.Get() != 0.5 {
			t.Fatalf("typed 0.7 gave %v, want 0.5 on the step", value.Get())
		}
		app.Clear("label:Opacity")
		app.Type("label:Opacity", "lots")
		app.Submit("label:Opacity")
		if got := app.Find("label:Opacity").Text; value.Get() != 0.5 || got != "0.5" {
			t.Fatalf("text that is no number left %v showing %q", value.Get(), got)
		}
		app.Pan("number-scrub", core.Point{X: 4}, core.Point{X: 9})
		if value.Get() != 1.5 {
			t.Fatalf("a scrub of two steps gave %v, want 1.5", value.Get())
		}
	})
}

// TestNumberInputKeys steps the focused field with the arrow keys.
func TestNumberInputKeys(t *testing.T) {
	var value *core.Signal[float64]
	app, _ := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		value = core.NewSignal(cx, 3.0)
		return NumberInput{Label: "Count", Value: value.Get, OnChange: value.Set}
	})
	app.Focus("label:Count")
	app.Key("up")
	app.Key("up")
	app.Key("down")
	if value.Get() != 4 || app.Find("label:Count").Text != "4" {
		t.Fatalf("value = %v, want 4", value.Get())
	}
}

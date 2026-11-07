package ui

import (
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

// TestHoldToConfirm confirms only once held for its duration, rewinding
// a hold let go early; assistive technology starts and cancels a hold
// with a press; and the app's reset brings the button back.
func TestHoldToConfirm(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var done *core.Signal[bool]
		confirms := 0
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			done = core.NewSignal(cx, false)
			return HoldToConfirm{TestID: "hold", Text: "Hold to delete", ConfirmedText: "Deleted", Variant: HoldDanger,
				Confirmed: done.Get, OnConfirm: func() { confirms++; done.Set(true) }}
		}, nsgotest.ManualFrames())
		app.PressDown("hold")
		app.Advance(600 * time.Millisecond)
		app.PressUp("hold")
		app.Advance(2 * time.Second)
		if confirms != 0 {
			t.Fatal("a hold let go halfway confirmed")
		}
		app.PressDown("hold")
		app.Advance(1300 * time.Millisecond)
		app.PressUp("hold")
		if confirms != 1 || app.Find("hold").Props.Label != "Deleted" {
			t.Fatalf("a full hold confirmed %d times, reading %q", confirms, app.Find("hold").Props.Label)
		}
		done.Set(false)
		app.Advance(time.Second)
		app.AccessibilityAction("hold", core.ActionActivate)
		app.Advance(300 * time.Millisecond)
		app.AccessibilityAction("hold", core.ActionActivate)
		app.Advance(2 * time.Second)
		if confirms != 1 {
			t.Fatal("a hold cancelled by a second press confirmed")
		}
		app.AccessibilityAction("hold", core.ActionActivate)
		app.Advance(1300 * time.Millisecond)
		if confirms != 2 || app.Find("hold").Props.Label != "Deleted" {
			t.Fatalf("assistive technology's hold confirmed %d times in all, want 2", confirms)
		}
	})
}

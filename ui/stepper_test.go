package ui

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
)

// TestStepper marks completed steps with the accent and reads each
// step's status, returns to a completed step with a press but not to
// one ahead, and shows a failed step's error in place of its
// description.
func TestStepper(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var current *core.Signal[int]
		words := map[StepStatus]string{StepUpcoming: "upcoming", StepCurrent: "current", StepComplete: "completed", StepFailed: "failed"}
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			current = core.NewSignal(cx, 2)
			return Stepper{TestID: "steps", Label: "Progress", Current: current.Get, OnSelect: current.Set,
				StatusText: func(s StepStatus) string { return words[s] },
				Steps: []StepperStep{{Label: "Account"}, {Label: "Profile", Description: "Your name"},
					{Label: "Plan"}, {Label: "Pay", Description: "Card", Error: "Card declined"}}}
		})
		disc := func(i int) core.Color {
			return app.Find("steps-" + string(rune('0'+i))).Children[0].Children[1].Style.Background
		}
		if disc(0) != p.AccentFill || disc(2) == p.AccentFill {
			t.Fatal("only the completed steps are filled with the accent")
		}
		if got := app.Find("steps-1").Props.Label; got != "Profile, completed" {
			t.Fatalf("a completed step reads %q", got)
		}
		app.Find("Card declined")
		if app.Exists("Card") {
			t.Fatal("a failed step shows its description beside its error")
		}
		if !app.Find("steps-3").Props.Disabled {
			t.Fatal("a step ahead can be pressed")
		}
		app.Press("steps-0")
		app.Settle()
		if current.Get() != 0 || app.Find("steps-0").Props.Label != "Account, current" {
			t.Fatalf("pressing a completed step went to %d", current.Get())
		}
	})
}

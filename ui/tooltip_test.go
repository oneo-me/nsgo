package ui

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

// TestTooltip shows a tooltip as the pointer rests on its trigger.
func TestTooltip(t *testing.T) {
	app, _ := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{AlignItems: core.AlignStart, Padding: core.All(40)}, Children: []core.Node{
			Tooltip{Text: "Shared with 3 people", Trigger: core.Text{Content: "Shared"}, TestID: "shared"},
		}}
	})
	app.Hover("shared")
	app.Advance(600 * time.Millisecond)
	app.Find("Shared with 3 people")
}

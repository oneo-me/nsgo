package ui

import (
	"testing"

	"github.com/oneo-me/nsgo/core"
)

// TestContextMenu opens a context menu at a secondary click.
func TestContextMenu(t *testing.T) {
	var chosen string
	app, _ := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		return ContextMenu{TestID: "file", Trigger: core.Text{Content: "report.pdf"}, Children: []core.Node{
			MenuItem{Text: "Rename", OnSelect: func() { chosen = "rename" }},
		}}
	})
	app.RightClick("file")
	app.Press("role:menuitem Rename")
	if chosen != "rename" {
		t.Fatalf("chose %q, want rename", chosen)
	}
}

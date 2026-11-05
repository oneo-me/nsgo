package ui

import (
	"testing"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// TestInput sizes a field and a multiline one, and checks a disabled
// field.
func TestInput(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			return core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
				Input{Input: base.Input{Label: "Email", TestID: "email", Placeholder: "name@example.com"}},
				Input{Input: base.Input{Label: "Notes", TestID: "notes", Multiline: true}},
				Input{Input: base.Input{Label: "Off", TestID: "off", Disabled: func() bool { return true }}},
			}}
		})
		if f := app.Find("email"); f.Frame.Size.Height != ButtonHeight || f.Style.BorderColor != p.FieldBorder {
			t.Fatalf("field = %v tall, edge %x; want %v and the field border", f.Frame.Size.Height, f.Style.BorderColor, ButtonHeight)
		}
		if h := app.Find("notes").Frame.Size.Height; h <= ButtonHeight {
			t.Fatalf("multiline = %v tall, want more than a line", h)
		}
		if !app.Find("off").Input.Disabled {
			t.Fatal("the disabled field takes input")
		}
	})
}

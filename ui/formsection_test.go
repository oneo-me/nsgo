package ui

import (
	"testing"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
)

// TestFormSection divides the rows from the start of the titles above, has a
// pressable row press and a field edit its text.
func TestFormSection(t *testing.T) {
	headless(t)
	pressed := 0
	var text *core.Signal[string]
	app, _ := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		text = core.NewSignal(cx, "")
		return FormSection{Title: "Account", Footer: "Kept on this device.", TestID: "section", Children: []core.Node{
			FormField{Title: "Name", Input: base.Input{Value: text.Get, OnChange: text.Set, Placeholder: "Ada"}},
			FormRow{Icon: lucide.Bell, Title: "Alerts", Value: func() string { return "On" }, OnPress: func() { pressed++ }, TestID: "alerts"},
			FormRow{Title: "Sign out", Variant: FormRowDanger, OnPress: func() { pressed++ }},
		}}
	})
	card := app.Find("id:section").Children[1]
	if len(card.Children) != 5 {
		t.Fatalf("card holds %d views, want 3 rows and 2 hairlines", len(card.Children))
	}
	if x := card.Children[1].Frame.Origin.X; x != formRowPad {
		t.Errorf("hairline below a field starts at %v, want %v", x, formRowPad)
	}
	if x, want := card.Children[3].Frame.Origin.X, formRowPad+formIconTile+formRowGap; x != want {
		t.Errorf("hairline below a row with an icon starts at %v, want %v", x, want)
	}
	app.Press("id:alerts")
	app.Press("Sign out")
	if pressed != 2 {
		t.Fatalf("pressed %d rows, want 2", pressed)
	}
	app.Type("label:Name", "Grace")
	if text.GetUntracked() != "Grace" {
		t.Fatalf("field holds %q", text.GetUntracked())
	}
}

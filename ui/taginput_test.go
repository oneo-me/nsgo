package ui

import (
	"slices"
	"testing"

	"github.com/oneo-me/nsgo/core"
)

// TestTagInput adds tags on Return and at commas, drops repeats, removes
// one from its button and the last with Backspace in the empty field.
func TestTagInput(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var tags *core.Signal[[]string]
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			tags = core.NewSignalFunc(cx, []string{"design"}, slices.Equal)
			return TagInput{TestID: "tags", Label: "Tags", Tags: tags.Get, OnChange: tags.Set,
				Accept:      func(s string) bool { return s != "bad" },
				RemoveLabel: func(s string) string { return "Remove " + s }}
		})
		if got := app.Find("label:Tags").Input.CaretColor; got != p.Accent {
			t.Fatalf("caret = %x, want the accent", got)
		}
		if got := app.Find("tags >> design").Parent.Style.Cursor; got != core.CursorDefault {
			t.Fatalf("a tag's cursor = %v, want the arrow", got)
		}
		app.Type("label:Tags", "go")
		app.Submit("label:Tags")
		app.Type("label:Tags", "ui, design, bad, docs")
		if got := tags.Get(); !slices.Equal(got, []string{"design", "go", "ui"}) {
			t.Fatalf("tags = %v, want design, go and ui", got)
		}
		if got := app.Find("label:Tags").Text; got != "bad, docs" {
			t.Fatalf("the field holds %q, want the rejected and the unfinished tags", got)
		}
		app.Press("label:Remove go")
		if got := tags.Get(); !slices.Equal(got, []string{"design", "ui"}) {
			t.Fatalf("after removing go, tags = %v", got)
		}
		app.Clear("label:Tags")
		app.Key("Backspace")
		if got := tags.Get(); !slices.Equal(got, []string{"design"}) {
			t.Fatalf("backspace in the empty field left %v", got)
		}
		app.Find("tags >> design")
	})
}

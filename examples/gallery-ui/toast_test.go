package ui

import (
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
)

// TestToasts shows toasts read as alerts, runs an action and closes its
// toast, closes one with its close button, and lets one time out.
func TestToasts(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var toaster *Toaster
		undone := false
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			toaster = NewToaster(cx)
			return Toasts{Toaster: toaster, TestID: "toasts", CloseLabel: "Close"}
		})
		toaster.Show(toastOf("saved", ToastSuccess, "Saved", "To your library", "Undo", func() { undone = true }))
		toaster.Show(toastOf("sent", ToastPlain, "Sent", "", "", nil))
		app.Advance(time.Second)
		if v := app.Find("saved"); v.Props.Role != core.RoleAlert || v.Props.Label != "Saved. To your library" {
			t.Fatalf("a toast reads %q as %v", v.Props.Label, v.Props.Role)
		}
		app.Press("saved-action")
		app.Advance(time.Second)
		if !undone || app.Exists("saved") {
			t.Fatal("the action did not run and close its toast")
		}
		app.Press("sent-close")
		app.Advance(time.Second)
		if app.Exists("sent") {
			t.Fatal("the close button left its toast")
		}
		toaster.Show(ToastContent{ID: "brief", Title: "Brief", Duration: 2 * time.Second})
		app.Advance(time.Second)
		app.Find("brief")
		app.Advance(2 * time.Second)
		if app.Exists("brief") {
			t.Fatal("a toast stayed past its duration")
		}
	})
}

// toastOf is a toast's content, short.
func toastOf(id string, kind ToastType, title, description, action string, onAction func()) ToastContent {
	return ToastContent{ID: id, Type: kind, Title: title, Description: description, Action: action, OnAction: onAction}
}

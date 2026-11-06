package ui

import (
	"testing"
	"time"

	"github.com/oneo-me/nsgo/base/toast"
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

// TestToastPlacements places the stack at each edge, swipes toward it by
// default and turns it around with Reverse.
func TestToastPlacements(t *testing.T) {
	for _, c := range []struct {
		p    ToastPlacement
		want toast.SwipeDirection
	}{{ToastTop, toast.SwipeUp}, {ToastBottomEnd, toast.SwipeDown}, {ToastStart, toast.SwipeLeft}, {ToastEnd, toast.SwipeRight}} {
		if got := ToastSwipeEdge.direction(c.p, false); got != c.want {
			t.Fatalf("placement %d swipes %v, want %v", c.p, got, c.want)
		}
	}
	if got := ToastSwipeEdge.direction(ToastStart, true); got != toast.SwipeRight {
		t.Fatalf("the start side swipes %v right to left", got)
	}
	if got := ToastSwipeOff.direction(ToastTop, false); got != toast.SwipeOff {
		t.Fatalf("ToastSwipeOff swipes %v", got)
	}
	each(t, func(t *testing.T, e core.Environment) {
		var toaster *Toaster
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			toaster = NewToaster(cx)
			return Toasts{Toaster: toaster, Placement: ToastEnd, Reverse: true, CloseLabel: "Close"}
		})
		toaster.Show(toastOf("old", ToastPlain, "Old", "", "", nil))
		app.Advance(time.Second)
		toaster.Show(toastOf("new", ToastPlain, "New", "", "", nil))
		app.Advance(time.Second)
		if !(app.Find("old").Frame.Origin.Y <= app.Find("new").Frame.Origin.Y) {
			t.Fatal("Reverse left the newest toast above the oldest")
		}
	})
}

// TestToastsExpandedTakePresses opens the stack and closes an older toast
// where it shows, not only the one in front.
func TestToastsExpandedTakePresses(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var toaster *Toaster
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			toaster = NewToaster(cx)
			return Toasts{Toaster: toaster, Placement: ToastBottom, TestID: "toasts", CloseLabel: "Close"}
		})
		toaster.Show(ToastContent{ID: "old", Title: "Old", Duration: -1})
		app.Advance(time.Second)
		toaster.Show(ToastContent{ID: "new", Title: "New", Duration: -1})
		app.Advance(time.Second)
		app.Hover("toasts")
		app.Advance(time.Second)
		if !(app.Find("old").Parent.Frame.Origin.Y < app.Find("new").Parent.Frame.Origin.Y) {
			t.Fatal("the expanded toasts lie one over the other")
		}
		app.Press("old-close")
		app.Advance(time.Second)
		if app.Exists("old") || !app.Exists("new") {
			t.Fatal("the close button of the older toast did not close it")
		}
	})
}

// TestToastLimit keeps Limit toasts showing and tells a toast's OnClose
// when it has gone, replaced or timed out.
func TestToastLimit(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var toaster *Toaster
		closed := 0
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			toaster = NewToaster(cx)
			toaster.Limit = 1
			return Toasts{Toaster: toaster, CloseLabel: "Close"}
		})
		toaster.Show(toastOf("a", ToastPlain, "A", "", "", nil))
		toaster.Show(toastOf("b", ToastPlain, "B", "", "", nil))
		app.Advance(100 * time.Millisecond)
		if app.Exists("a") || !app.Exists("b") {
			t.Fatal("Limit 1 did not show the newest alone")
		}
		onClose := func() { closed++ }
		toaster.Show(ToastContent{ID: "c", Title: "C", Duration: time.Second, OnClose: onClose})
		toaster.Show(ToastContent{ID: "c", Title: "C again", Duration: time.Second, OnClose: onClose})
		if closed != 1 {
			t.Fatalf("replacing a toast closed %d", closed)
		}
		app.Advance(3 * time.Second)
		if closed != 2 {
			t.Fatalf("OnClose ran %d times once it timed out", closed)
		}
	})
}

// toastOf is a toast's content, short.
func toastOf(id string, kind ToastType, title, description, action string, onAction func()) ToastContent {
	return ToastContent{ID: id, Type: kind, Title: title, Description: description, Action: action, OnAction: onAction}
}

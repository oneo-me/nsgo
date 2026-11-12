package ui

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/base/toast"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
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

// TestToastFromClosingScope shows a toast from a scope that then goes,
// as a dialog's button does as the dialog closes, and still closes it
// with its close button and lets the next one time out.
func TestToastFromClosingScope(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var toaster *Toaster
		var dialog *core.Scope
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			toaster = NewToaster(cx)
			dialog = cx.Child()
			return Toasts{Toaster: toaster, TestID: "toasts", CloseLabel: "Close"}
		})
		app.Post(func() {
			dialog.Post(func() {
				toaster.Show(ToastContent{ID: "accepted", Title: "Accepted"})
				dialog.Dispose()
			})
		})
		app.Advance(time.Second)
		app.Press("accepted-close")
		app.Advance(time.Second)
		if app.Exists("accepted") {
			t.Fatal("a toast shown from a scope that went did not close")
		}
		toaster.Show(ToastContent{ID: "next", Title: "Next", Duration: time.Second})
		app.Advance(2 * time.Second)
		if app.Exists("next") {
			t.Fatal("the next toast stayed past its duration")
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
		if !(toastY(app, "old") <= toastY(app, "new")) {
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
		if !(toastY(app, "old") < toastY(app, "new")) {
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

// TestToastClosing closes the newest toast: it stays where it shows and
// takes no presses, and the others close up at once, at every line a stack
// grows from.
func TestToastClosing(t *testing.T) {
	for _, c := range []struct {
		name    string
		p       ToastPlacement
		reverse bool
	}{{"top", ToastTop, false}, {"bottom", ToastBottomEnd, false}, {"top reversed", ToastTop, true},
		{"bottom reversed", ToastBottom, true}, {"side", ToastEnd, false}, {"side reversed", ToastStart, true}} {
		t.Run(c.name, func(t *testing.T) {
			var toaster *Toaster
			app, _ := start(t, desktopEnv, func(cx *core.Scope) core.Node {
				toaster = NewToaster(cx)
				return Toasts{Toaster: toaster, Placement: c.p, Reverse: c.reverse, TestID: "toasts", CloseLabel: "Close"}
			})
			toaster.Show(ToastContent{ID: "old", Title: "Old", Duration: -1})
			app.Advance(time.Second)
			toaster.Show(ToastContent{ID: "new", Title: "New", Action: "Undo", Duration: -1})
			app.Advance(time.Second)
			if app.Find("new").Animated.Set != 0 {
				t.Fatal("the toast's root binds animated values, which backends draw without its Transition")
			}
			height := app.Find("toasts").Frame.Size.Height
			y := toastY(app, "new")
			toaster.Dismiss("new")
			app.Advance(16 * time.Millisecond)
			if !app.Exists("new") {
				t.Fatal("the toast left before its exit")
			}
			if got := toastY(app, "new"); got != y {
				t.Fatalf("the toast going moved from %v to %v", y, got)
			}
			if got := app.Find("toasts").Frame.Size.Height; got >= height {
				t.Fatalf("the stack is %v tall going, %v before: the toast going takes room", got, height)
			}
			settled := toastY(app, "old")
			going := app.Find("new")
			if top := going.Children[len(going.Children)-1]; top.Frame.Size.Width < going.Frame.Size.Width-2 {
				t.Fatalf("nothing covers the toast going, whose buttons would take presses: %v", top.Frame)
			}
			app.Advance(time.Second)
			if app.Exists("new") {
				t.Fatal("the toast stayed after its exit")
			}
			if got := toastY(app, "old"); got != settled {
				t.Fatalf("the older toast's place went from %v to %v once the newest had gone", settled, got)
			}
		})
	}
}

// toastY is where the stack lays a toast out, in the window.
func toastY(app *nsgotest.App, id string) core.Points {
	var y core.Points
	for v := app.Find(id).Parent; v != nil; v = v.Parent {
		y += v.Frame.Origin.Y
	}
	return y
}

// toastOf is a toast's content, short.
func toastOf(id string, kind ToastType, title, description, action string, onAction func()) ToastContent {
	return ToastContent{ID: id, Type: kind, Title: title, Description: description, Action: action, OnAction: onAction}
}

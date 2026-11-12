package main

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/mobile"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestMain hands the main thread to the platform under nsgo test -target.
func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestGreets(t *testing.T) {
	app := nsgotest.Start(t, Root, nsgotest.Size(420, 320))
	if got := app.Find("id:greeting").Text; got != "Hello, world!" {
		t.Errorf("greeting = %q", got)
	}
	app.Type("id:name", "Ada")
	if got := app.Find("id:greeting").Text; got != "Hello, Ada!" {
		t.Errorf("greeting after typing = %q", got)
	}
	app.Press("id:say")
	app.Press("id:say")
	if got := app.Find("id:said").Text; got != "Said 2 times" {
		t.Errorf("said = %q", got)
	}
}

// TestNotificationServicesAnswer asks whether the app may show
// notifications, which every platform answers without asking the user,
// sets and clears the badge and plays a haptic, where each does nothing
// or what the platform does. Where the app may show notifications it
// shows one, which the system takes, and closes it. Presses ask, as the
// services are the engine's to call.
func TestNotificationServicesAnswer(t *testing.T) {
	permission := make(chan core.NotificationPermission, 1)
	shown := make(chan bool, 1)
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		button := func(id string, press func()) core.Node {
			return core.View{TestID: id, Role: core.RoleButton, Label: id, Style: core.Style{Width: core.Pt(40), Height: core.Pt(20)},
				OnPress: func(core.PressEvent) { press() }}
		}
		return core.View{Children: []core.Node{
			Root(cx),
			button("ask", func() {
				cx.NotificationPermission(func(p core.NotificationPermission) { permission <- p })
				cx.SetBadge(3)
				mobile.PlayHaptic(cx, mobile.HapticSelection)
			}),
			button("show", func() {
				_ = cx.ShowNotification(core.Notification{ID: "hello", Title: "Hello", Body: "From NSGo's tests", Silent: true},
					func(ok bool) { shown <- ok })
			}),
			button("close", func() {
				cx.CloseNotification("hello")
				cx.SetBadge(0)
			}),
		}}
	}, nsgotest.Size(420, 400))
	receive := func(ch any) any {
		t.Helper()
		var v any
		app.WaitUntil(5*time.Second, func() bool {
			switch c := ch.(type) {
			case chan core.NotificationPermission:
				select {
				case v = <-c:
					return true
				default:
				}
			case chan bool:
				select {
				case v = <-c:
					return true
				default:
				}
			}
			return false
		})
		return v
	}
	app.Press("id:ask")
	p := receive(permission).(core.NotificationPermission)
	if p != core.NotificationsGranted {
		app.Press("id:close")
		t.Logf("notification permission: %v; nothing shown", p)
		return
	}
	app.Press("id:show")
	if !receive(shown).(bool) {
		t.Error("the system did not take a notification the app may show")
	}
	app.Press("id:close")
}

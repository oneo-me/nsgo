package core_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestNotificationsAskShowCloseAndReportPresses(t *testing.T) {
	var scope *core.Scope
	var asked []core.NotificationPermission
	var shown []bool
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		scope = cx
		return core.View{}
	})
	answer := func(p core.NotificationPermission) { asked = append(asked, p) }
	show := func(n core.Notification) {
		if err := scope.ShowNotification(n, func(ok bool) { shown = append(shown, ok) }); err != nil {
			t.Fatal(err)
		}
	}

	show(core.Notification{ID: "early", Title: "Too early"})
	scope.NotificationPermission(answer)
	scope.RequestNotificationPermission(answer)
	scope.NotificationPermission(answer)
	app.Settle()
	if want := []core.NotificationPermission{core.NotificationsNotAsked, core.NotificationsGranted, core.NotificationsGranted}; !slices.Equal(asked, want) {
		t.Errorf("permissions %v, want %v", asked, want)
	}
	show(core.Notification{ID: "chat:1", Title: "Ada", Body: "Hi"})
	show(core.Notification{ID: "chat:2", Title: "Bo", Silent: true})
	show(core.Notification{ID: "chat:1", Title: "Ada", Body: "Hi again"})
	scope.CloseNotification("chat:2")
	scope.SetBadge(3)
	app.Settle()
	if !slices.Equal(shown, []bool{false, true, true, true}) {
		t.Errorf("shown %v", shown)
	}
	if len(app.Notifications) != 1 || app.Notifications[0].Body != "Hi again" || app.Badge != 3 {
		t.Errorf("notifications %+v, badge %d", app.Notifications, app.Badge)
	}
	if err := scope.ShowNotification(core.Notification{ID: "x"}, nil); !errors.Is(err, core.ErrNotification) {
		t.Errorf("a notification without a title: %v", err)
	}

	// A press before any handler waits for the first.
	app.PressNotification("chat:1")
	var pressed []string
	scope.OnNotificationPress(func(id string) { pressed = append(pressed, id) })
	app.Settle()
	show(core.Notification{ID: "chat:3", Title: "Cy"})
	app.Settle()
	app.PressNotification("chat:3")
	if !slices.Equal(pressed, []string{"chat:1", "chat:3"}) {
		t.Errorf("pressed %v", pressed)
	}
}

func TestNotificationsHeldToAccess(t *testing.T) {
	var got core.NotificationPermission
	var shown = true
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		cx.ProvideAccess(denyAll{})
		cx.RequestNotificationPermission(func(p core.NotificationPermission) { got = p })
		_ = cx.ShowNotification(core.Notification{ID: "a", Title: "A"}, func(ok bool) { shown = ok })
		cx.SetBadge(2)
		return core.View{}
	})
	app.Settle()
	if got != core.NotificationsUnavailable || shown || app.Badge != 0 {
		t.Errorf("refused code: permission %v, shown %v, badge %d", got, shown, app.Badge)
	}
}

type denyAll struct{}

func (denyAll) Allows(core.AccessRequest) bool { return false }
func (denyAll) WebHosts() []string             { return nil }

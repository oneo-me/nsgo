package app

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestHostChannel plays the host: launch parameters, a message, a call it
// answers and a post it hears.
func TestHostChannel(t *testing.T) {
	host := core.NewHost()
	host.SetParams(map[string]any{"user": "Ada"})
	var mu sync.Mutex
	var taps []any
	host.Handle("time", func(c *core.HostCall) { c.Resolve("12:00") })
	host.Handle("tapped", func(c *core.HostCall) {
		mu.Lock()
		taps = append(taps, c.Data.(map[string]any)["count"])
		mu.Unlock()
	})
	app := nsgotest.Start(t, Root, nsgotest.Host(host))
	app.Find("text:Hello, Ada")

	host.Send("message", map[string]any{"text": "Order filled"})
	app.WaitFor("text:1 messages · Order filled", time.Second)

	app.Press("id:ask")
	app.WaitFor("text:The host says 12:00", time.Second)

	app.Press("id:tell")
	app.Press("id:tell")
	app.WaitUntil(time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(taps) == 2
	})
	if taps[0] != float64(1) || taps[1] != float64(2) {
		t.Fatalf("the host heard %v", taps)
	}
}

// TestHostRoutesLinks has the host take the order link before the
// platform.
func TestHostRoutesLinks(t *testing.T) {
	host := core.NewHost()
	var routed string
	host.HandleOpenURL(func(url string, answer func(bool)) {
		routed = url
		answer(strings.HasPrefix(url, "https://example.com/orders/"))
	})
	app := nsgotest.Start(t, Root, nsgotest.Host(host))
	app.Press("id:open")
	app.WaitFor("text:opened: true", time.Second)
	if routed != OrderURL {
		t.Fatalf("the host saw %q", routed)
	}
	if target := nsgotest.Target(); target == "" && len(app.OpenedURLs) != 0 {
		t.Fatalf("the platform opened %v", app.OpenedURLs)
	}
}

// TestNoHost runs the app on its own: it greets nobody in particular and
// calls fail.
func TestNoHost(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.Find("text:Hello, there")
	app.Press("id:ask")
	app.WaitFor("text:The host failed", time.Second)
}

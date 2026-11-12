// Command embed-desktop is a desktop app of its own that embeds NSGo: its
// window, built with the platform's toolkit and run by its own event
// loop, shows a status line, a button and two NSGo views of
// examples/embedded/app side by side, each an engine of its own. The
// host greets each view's user through its launch parameters, sends both
// a message from the button, answers their "time" calls, hears their
// "tapped" posts, routes their order links and gives each a badge of its
// own as a native component.
package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/examples/embedded/app"
)

func main() { runHost() }

// host is what the host keeps of the views it embeds.
type host struct {
	views  []*nsgo.View
	sent   int
	status func(string) // sets the status line, on the main thread
	onMain func(func()) // runs fn on the main thread, from any goroutine
}

// newViews makes the two views, for users and with tokens of their own.
func (h *host) newViews(opts ...nsgo.Option) []*nsgo.View {
	tokens := []core.Tokens{{}, {Values: map[string]string{"accent": "#cf222e"}, Dark: map[string]string{"accent": "#ff7b72"}}}
	for i, user := range []string{"Ada", "Grace"} {
		ch := core.NewHost()
		ch.SetParams(map[string]any{"user": user})
		ch.Handle("time", func(c *core.HostCall) { c.Resolve(time.Now().Format("15:04:05")) })
		ch.Handle("tapped", func(c *core.HostCall) {
			h.onMain(func() { h.status(fmt.Sprintf("%s tapped %v times", user, c.Data.(map[string]any)["count"])) })
		})
		ch.HandleOpenURL(func(url string, answer func(bool)) {
			// The host's own router takes its order pages.
			ours := strings.HasPrefix(url, "https://example.com/orders/")
			if ours {
				h.onMain(func() { h.status(user + " opened " + url[strings.LastIndex(url, "/")+1:] + " in the host") })
			}
			answer(ours)
		})
		v := nsgo.NewView(app.Root, append([]nsgo.Option{nsgo.HostChannel(ch),
			nsgo.Component(app.Badge, newBadge)}, opts...)...)
		v.SetTokens(tokens[i])
		h.views = append(h.views, v)
	}
	return h.views
}

// send sends every view a message, as the host's button does.
func (h *host) send() {
	h.sent++
	for _, v := range h.views {
		v.Host().Send("message", map[string]any{"text": fmt.Sprintf("Message %d from the host", h.sent)})
	}
}

// close closes the views as the host's window goes.
func (h *host) close() {
	for _, v := range h.views {
		v.Close()
	}
	h.views = nil
}

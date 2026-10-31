//go:build nsgodev

package nsgo

import (
	"bufio"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/oneo-me/nsgo/core"
)

// devServer is nsgo dev's server, set with -ldflags -X for targets that
// pass the app no environment; NSGO_DEV_SERVER overrides it.
var devServer = ""

// devApp runs app under nsgo dev: build failures the server reports and
// the app's own render failures show in an overlay over it, Command or
// Control+R asks for a new build, and with -trace the server prints each
// commit. On the web the page is served from the server and shows build
// failures itself.
func devApp(app App) App {
	server := devServer
	if s := os.Getenv("NSGO_DEV_SERVER"); s != "" {
		server = s
	}
	page := server == ""
	if page {
		server = pageOrigin()
	}
	if server == "" {
		return app
	}
	if devTrace == "1" {
		traceTo(server)
	}
	if page {
		return app
	}
	return func(cx *core.Scope) core.Node {
		failure := core.NewSignal(cx, devFailure{})
		go listenDev(server, func(f devFailure) { cx.Post(func() { failure.Set(f) }) })
		cx.OnKeyWith("mod+r", core.KeyOptions{When: core.KeyAlways}, func() {
			go http.Post(server+"/_nsgo/reload", "text/plain", nil)
		})
		return core.Group{
			core.Catch{
				Try: app,
				Fallback: func(cx *core.Scope, err error, reset func()) core.Node {
					return devOverlay(cx, devFailure{Title: "The app failed", Text: err.Error()}, reset)
				},
			},
			core.Show{When: func() bool { return failure.Get().Text != "" }, Then: func(cx *core.Scope) core.Node {
				return devOverlay(cx, failure.Get(), func() { failure.Set(devFailure{}) })
			}},
		}
	}
}

// devFailure is what the overlay shows.
type devFailure struct{ Title, Text string }

// listenDev follows the server's events, reconnecting while it runs: a
// failed build's output, and its clearing.
func listenDev(server string, report func(devFailure)) {
	for {
		resp, err := http.Get(server + "/_nsgo/events")
		if err == nil {
			sc := bufio.NewScanner(resp.Body)
			sc.Buffer(make([]byte, 1<<16), 16<<20)
			for sc.Scan() {
				data, ok := strings.CutPrefix(sc.Text(), "data: ")
				if !ok {
					continue
				}
				switch kind, rest, _ := strings.Cut(data, " "); kind {
				case "error":
					if text, err := strconv.Unquote(rest); err == nil {
						report(devFailure{Title: "Build failed", Text: text})
					}
				case "ok":
					report(devFailure{})
				}
			}
			resp.Body.Close()
		}
		time.Sleep(time.Second)
	}
}

// devOverlay covers the app with what failed: a press anywhere dismisses
// it, Copy copies the text for a report.
func devOverlay(cx *core.Scope, f devFailure, dismiss func()) core.Node {
	text := core.Style{Color: core.RGB(0xf2f2f2), FontSize: 13}
	button := core.Style{Padding: core.Symmetric(6, 12), CornerRadius: 6, Background: core.RGBA(0xffffff, 0.15)}
	return core.Modal{Animation: core.ModalNone, OnRequestClose: func(string) { dismiss() }, TestID: "nsgo-dev-overlay",
		Children: []core.Node{core.View{
			Style:   core.Style{FlexGrow: 1, Padding: core.Edges{Top: 56, Right: 20, Bottom: 32, Left: 20}, Gap: 12, Background: core.RGBA(0x111111, 0.94)},
			OnPress: func(core.PressEvent) { dismiss() },
			Children: []core.Node{
				core.Text{Content: f.Title, Style: core.Style{Color: core.RGB(0xff5c5c), FontSize: 18, FontWeight: core.FontWeightBold}},
				core.Scroll{Style: core.Style{FlexGrow: 1, FlexShrink: 1}, Children: []core.Node{
					core.Text{Content: f.Text, Style: core.Style{Color: text.Color, FontSize: text.FontSize, FontFamily: core.FontMonospace}},
				}},
				core.View{Style: core.Style{Direction: core.Row, Gap: 12, AlignItems: core.AlignCenter}, Children: []core.Node{
					core.View{Style: button, Role: core.RoleButton, OnPress: func(core.PressEvent) { cx.SetClipboard(f.Text) },
						Children: []core.Node{core.Text{Content: "Copy", Style: text}}},
					core.Text{Content: "Press anywhere to dismiss; save a fix or press " + cx.KeyLabel("mod+r") + " to build again.",
						Style: core.Style{Color: core.RGBA(0xf2f2f2, 0.6), FontSize: 12, FlexShrink: 1}},
				}},
			},
		}}}
}

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

// devLocale is the locale nsgo dev -lang runs the app in, set with
// -ldflags -X.
var devLocale = ""

// devServer is nsgo dev's server, set with -ldflags -X for targets that
// pass the app no environment; NSGO_DEV_SERVER overrides it.
var devServer = ""

// devApp runs app under nsgo dev: build failures the server reports and
// the app's own render failures show in an overlay over it, and with
// -trace the server prints each commit. On the web the page is served from the server and shows build
// failures itself.
func devApp(app App) App {
	if devLocale != "" {
		inner := app
		app = func(cx *core.Scope) core.Node {
			cx.SetLocale(devLocale)
			return inner(cx)
		}
	}
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
		reload := func() { go http.Post(server+"/_nsgo/reload", "text/plain", nil) }
		return core.Group{
			core.Catch{
				Try: app,
				Fallback: func(cx *core.Scope, err error, reset func()) core.Node {
					return devOverlay(cx, devFailure{Title: "The app failed", Text: err.Error()}, reset, reload)
				},
			},
			core.Show{When: func() bool { return failure.Get().Text != "" }, Then: func(cx *core.Scope) core.Node {
				return devOverlay(cx, failure.Get(), func() { failure.Set(devFailure{}) }, reload)
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

// devOverlay covers the app with what failed: a centered panel of its
// title, the text in a box that scrolls, and buttons to copy the text and
// to build again.
func devOverlay(cx *core.Scope, f devFailure, dismiss, reload func()) core.Node {
	label := core.Style{Color: core.RGB(0xf2f2f2), FontSize: 13}
	button := core.Style{Padding: core.Symmetric(6, 14), CornerRadius: 6, Background: core.RGBA(0xffffff, 0.12)}
	return core.Modal{Animation: core.ModalNone, OnRequestClose: func(string) { dismiss() }, TestID: "nsgo-dev-overlay",
		Children: []core.Node{core.View{
			Style: core.Style{FlexGrow: 1, Padding: core.Symmetric(48, 24), JustifyContent: core.JustifyCenter, AlignItems: core.AlignCenter,
				Background: core.RGBA(0x111111, 0.6)},
			Children: []core.Node{core.View{
				Style: core.Style{Width: core.Pct(100), MaxWidth: core.Pt(720), MaxHeight: core.Pct(100), FlexShrink: 1, Padding: core.All(20), Gap: 16,
					CornerRadius: 12, Background: core.RGB(0x1c1c1c), BorderWidth: 1, BorderColor: core.RGBA(0xffffff, 0.1)},
				Children: []core.Node{
					core.Text{Content: f.Title, Style: core.Style{Color: core.RGB(0xff5c5c), FontSize: 18, FontWeight: core.FontWeightBold}},
					core.View{Style: core.Style{MaxHeight: core.Pt(360), FlexShrink: 1, CornerRadius: 8, Background: core.RGBA(0x000000, 0.35)},
						Children: []core.Node{core.Scroll{Style: core.Style{FlexShrink: 1}, Children: []core.Node{
							core.View{Style: core.Style{Padding: core.All(12)}, Children: []core.Node{
								core.Text{Content: f.Text, Style: core.Style{Color: label.Color, FontSize: 12, FontFamily: core.FontMonospace}},
							}},
						}}}},
					core.View{Style: core.Style{Direction: core.Row, Gap: 8, JustifyContent: core.JustifyEnd}, Children: []core.Node{
						core.View{Style: button, Role: core.RoleButton, OnPress: func(core.PressEvent) { cx.SetClipboard(f.Text) },
							Children: []core.Node{core.Text{Content: "Copy", Style: label}}},
						core.View{Style: button, Role: core.RoleButton, OnPress: func(core.PressEvent) { reload() },
							Children: []core.Node{core.Text{Content: "Reload", Style: label}}},
					}},
				},
			}},
		}}}
}

// Package app is an NSGo app made to be embedded: it greets the user its
// host names in its launch parameters, shows the host's messages, calls
// and posts to the host, shows the host's own badge as a native
// component, opens a link the host routes, and themes itself with the
// host's tokens, locale and text size. The Go desktop host in
// examples/embed-desktop and the page in examples/embed-web show it
// twice; examples/embedded builds it for the iOS and Android hosts and,
// with host.js as its page's host, for the web.
package app

import (
	"fmt"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// Badge is the native component a host registers for the app: a view of
// the host's own showing the "count" prop, which emits "tap".
const Badge = "host.badge"

// OrderURL is the link the app opens, which a host routes to its own
// order page.
const OrderURL = "https://example.com/orders/42"

// Root is the app.
func Root(cx *core.Scope) core.Node {
	env := cx.Env()
	user, _ := cx.HostParams()["user"].(string)
	if user == "" {
		user = "there"
	}
	messages := core.NewSignal(cx, 0)
	last := core.NewSignal(cx, "No message from the host yet.")
	answer := core.NewSignal(cx, "")
	taps := core.NewSignal(cx, 0)
	opened := core.NewSignal(cx, "")
	cx.OnHostEvent("message", func(data any) {
		messages.Set(messages.Get() + 1)
		if m, ok := data.(map[string]any); ok {
			if text, ok := m["text"].(string); ok {
				last.Set(text)
			}
		}
	})

	// The app's colors; a host's tokens go over them by the same names.
	theme := core.DefaultTheme()
	theme.Light.Background, theme.Dark.Background = core.RGB(0xffffff), core.RGB(0x0d1117)
	theme.Light.Text, theme.Dark.Text = core.RGB(0x1f2328), core.RGB(0xe6edf3)
	theme.Light.TextMuted, theme.Dark.TextMuted = core.RGB(0x656d76), core.RGB(0x8d96a0)
	theme.Light.Accent, theme.Dark.Accent = core.RGB(0x0969da), core.RGB(0x4493f8)
	cx.SetTheme(theme)
	ink := func() core.Color { return cx.Palette().Text }
	muted := func() core.Color { return cx.Palette().TextMuted }
	accent := func() core.Color { return cx.Palette().Accent }
	text := func(content func() string, size core.Points, tone func() core.Color, id string) core.Node {
		return core.Text{ContentFunc: content, TestID: id,
			StyleFunc: func() core.Style { return core.Style{FontSize: size, Color: tone()} }}
	}
	button := func(label, id string, press func()) core.Node {
		return base.Button{Label: label, TestID: id, OnPress: func(core.PressEvent) { press() },
			StyleFunc: func() core.Style {
				return core.Style{Padding: core.Symmetric(8, 14), CornerRadius: 8, Background: accent(),
					AlignSelf: core.AlignStart}
			},
			Children: []core.Node{core.Text{Content: label, Style: core.Style{Color: core.RGB(0xffffff), FontSize: 14}}}}
	}

	return core.View{
		StyleFunc: func() core.Style {
			return core.Style{FlexGrow: 1, Padding: core.All(20), Gap: 12,
				Background: cx.Palette().Background}
		},
		Children: []core.Node{
			text(func() string { return "Hello, " + user }, 22, ink, "greeting"),
			text(func() string {
				return fmt.Sprintf("Locale %q, text size %.2fx", env.Locale(), env.FontScale())
			}, 13, muted, "environment"),
			text(func() string { return fmt.Sprintf("%d messages · %s", messages.Get(), last.Get()) }, 15, ink, "messages"),
			button("Ask the host the time", "ask", func() {
				cx.CallHost("time", nil, func(result any, err error) {
					if err != nil {
						answer.Set("The host failed: " + err.Error())
						return
					}
					answer.Set(fmt.Sprintf("The host says %v", result))
				})
			}),
			core.Show{When: func() bool { return answer.Get() != "" }, Then: func(cx *core.Scope) core.Node {
				return text(answer.Get, 14, muted, "answer")
			}},
			button("Tell the host", "tell", func() {
				taps.Set(taps.Get() + 1)
				cx.PostHost("tapped", map[string]any{"count": taps.Get()})
			}),
			core.Native{Component: Badge, TestID: "badge", Label: "The host's badge",
				PropsFunc: func() map[string]any { return map[string]any{"count": taps.Get()} },
				OnEvent: func(name string, data any) {
					if name == "tap" {
						taps.Set(0)
					}
				},
				Style: core.Style{AlignSelf: core.AlignStart, MinWidth: core.Pt(120), MinHeight: core.Pt(28)}},
			button("Open order 42", "open", func() {
				cx.OpenURL(OrderURL, func(ok bool) { opened.Set(fmt.Sprintf("opened: %v", ok)) })
			}),
			text(opened.Get, 13, muted, "opened"),
		},
	}
}

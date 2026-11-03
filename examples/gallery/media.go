package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/oneo-me/nsgo/core"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
	"github.com/oneo-me/nsgo/examples/gallery/locales"
)

// inlinePage is an inline page in the gallery's colors, for the
// appearance shown; as in NSH, a page source that reads it loads again
// when they change.
func inlinePage(cx *core.Scope) string {
	colors := ui.ColorsOf(cx).Now()
	isDark := cx.Dark()
	hex := func(c core.Color) string {
		r, g, b, a := c.Components()
		return fmt.Sprintf("rgba(%d,%d,%d,%.3f)", int(r*255+0.5), int(g*255+0.5), int(b*255+0.5), a)
	}
	shine := 0.6
	if isDark {
		shine = 0.45
	}
	return `<!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1">
<style>
  html { -webkit-text-size-adjust: 100%; }
  body { margin: 0; padding: 16px; background: ` + hex(colors.Inset) + `; color: ` + hex(colors.Text) + `; font-family: -apple-system, system-ui, sans-serif; font-size: 15px; line-height: 1.4; }
  h3 { margin: 0 0 4px; font-size: 17px; font-weight: 600; }
  p { margin: 0 0 12px; color: ` + hex(colors.Muted) + `; font-size: 15px; }
  button { height: 44px; padding: 0 18px; border: 0; border-radius: 10px; background: linear-gradient(180deg, #d6f78a 0%, #c1ea5c 100%); color: ` + hex(colors.OnAccent) + `; font-family: inherit; font-size: 15px; font-weight: 600; box-shadow: inset 0 1px 0 rgba(255,255,255,` + fmt.Sprint(shine) + `); }
  #got { margin-top: 12px; padding: 10px 12px; border-radius: 10px; border: 1px solid ` + hex(colors.Border) + `; color: ` + hex(colors.Muted) + `; font-size: 15px; }
</style></head>
<body>
<h3>An inline page</h3>
<p>HTML from a string. It sends messages to the app and receives them.</p>
<button onclick="window.nsgo.postMessage('hello from the page')">Post to the app</button>
<div id="got">No message from the app yet.</div>
<script>window.addEventListener('message', (e) => { document.getElementById('got').textContent = 'From the app: ' + e.data; });</script>
</body></html>`
}

// mediaFrame is a media view's frame, for a StyleFunc.
func mediaFrame(cx *core.Scope, height core.Points, bg core.Color) core.Style {
	return core.Style{Height: core.Pt(height), CornerRadius: ui.RadiusControl, Overflow: core.Hidden, BorderWidth: 1, BorderColor: ui.ColorsOf(cx).Border(), Background: bg}
}

func messaging(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	message := core.NewSignal(cx, "")
	sent := core.NewSignal(cx, 0)
	web := &core.Ref{}
	or := func(s string) string {
		if s == "" {
			return "—"
		}
		return s
	}
	return ui.Card{Title: tr.MediaWebView(), Description: tr.MediaInlineHtmlWithMessagesBoth(), Children: []core.Node{
		core.WebView{Ref: web, SourceFunc: func() core.WebSource { return core.WebHTML(inlinePage(cx), "") }, TestID: "bridge", StyleFunc: func() core.Style { return mediaFrame(cx, 210, colors.Inset()) }, OnMessage: message.Set},
		ui.Caption{Label: tr.MediaFromThePage(), Note: func() string { return or(message.Get()) }},
		ui.Caption{Label: tr.MediaToThePage(), Note: func() string {
			if sent.Get() == 0 {
				return "—"
			}
			return fmt.Sprintf("hi #%d", sent.Get())
		}},
		ui.Button{Text: tr.MediaPostToThePage(), OnPress: func() {
			sent.Set(int(time.Now().UnixMilli() % 1000))
			web.PostMessage(fmt.Sprintf("hi #%d", sent.GetUntracked()))
		}},
	}}
}

// navButton is a round icon button, 44 pt.
func navButton(cx *core.Scope, flip bool, enabled func() bool, label string, onPress func()) core.Node {
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	s := func() core.Style {
		s := fin.Raised().Over(core.Style{Width: core.Pt(44), Height: core.Pt(44), CornerRadius: 22, Background: colors.Raised(), BorderWidth: 1, BorderColor: colors.Border(),
			AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter})
		if flip {
			s.Transform.Rotate = 180
		}
		return s
	}
	return ui.Press{OnPress: onPress, Disabled: func() bool { return !enabled() }, Label: label, HitSlop: 4, Children: []core.Node{
		core.View{StyleFunc: s, Children: []core.Node{ui.Icon(ui.IconChevron, 18, colors.Text)}},
	}}
}

func site(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	state := core.NewSignal(cx, core.WebNavigation{})
	ref := &core.Ref{}
	host := func() string {
		u := state.Get().URL
		u = strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
		if i := strings.Index(u, "/"); i >= 0 {
			u = u[:i]
		}
		if u == "" {
			return "…"
		}
		return u
	}
	return ui.Card{Title: tr.MediaAWebSite(), Description: tr.MediaNavigationStateBackAndForward(), Actions: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
		if state.Get().Loading {
			return ui.Badge{Text: tr.MediaLoading(), Tone: ui.ToneWarn}
		}
		return ui.Badge{Text: tr.MediaLoaded(), Tone: ui.ToneUp}
	})}, Children: []core.Node{
		core.View{StyleFunc: func() core.Style {
			return fin.Track().Over(core.Style{Height: core.Pt(40), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Padding: core.Symmetric(0, 14),
				CornerRadius: 20, Background: colors.Inset(), BorderWidth: 1, BorderColor: colors.Border()})
		}, Children: []core.Node{
			core.View{StyleFunc: func() core.Style {
				c := colors.Up
				if state.Get().Loading {
					c = colors.Warn
				}
				return core.Style{Width: core.Pt(7), Height: core.Pt(7), CornerRadius: 3.5, Background: c()}
			}},
			core.Text{ContentFunc: host, MaxLines: 1, EllipsizeMode: core.EllipsizeMiddle, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 15}, StyleFunc: ui.Colored(colors.Muted)},
		}},
		core.WebView{Ref: ref, Source: core.WebSrc("https://go.dev"), TestID: "site", StyleFunc: func() core.Style { return mediaFrame(cx, 340, colors.Inset()) }, OnNavigationStateChange: state.Set},
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10}, Children: []core.Node{
			navButton(cx, true, func() bool { return state.Get().CanGoBack }, tr.MediaBack(), ref.GoBack),
			navButton(cx, false, func() bool { return state.Get().CanGoForward }, tr.MediaForward(), ref.GoForward),
			ui.Spacer{},
			ui.Button{Text: tr.MediaReload(), Variant: ui.ButtonSecondary, OnPress: ref.Reload},
		}},
	}}
}

// clockText is a duration as minutes and seconds.
func clockText(d time.Duration) string {
	s := int(d.Seconds())
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// flowerClip is a five-second CC0 clip from MDN's examples.
const flowerClip = "https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.mp4"

func player(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	controls := core.NewSignal(cx, "own")
	paused, buffering, failed := core.NewSignal(cx, true), core.NewSignal(cx, false), core.NewSignal(cx, false)
	pos, dur := core.NewSignal(cx, time.Duration(0)), core.NewSignal(cx, time.Duration(0))
	fit := core.NewSignal(cx, core.Contain)
	ref := &core.Ref{}
	return ui.Card{Title: tr.MediaVideo(), Description: tr.MediaThePlatformSPlayer(), Actions: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
		switch {
		case failed.Get():
			return ui.Badge{Text: tr.MediaError(), Tone: ui.ToneDanger}
		case buffering.Get():
			return ui.Badge{Text: tr.MediaBuffering(), Tone: ui.ToneWarn}
		case paused.Get():
			return ui.Badge{Text: tr.MediaPaused()}
		}
		return ui.Badge{Text: tr.MediaPlaying(), Tone: ui.ToneAccent}
	})}, Children: []core.Node{
		core.Dynamic(func(*core.Scope) core.Node {
			// The platform's controls are fixed as the player mounts.
			sys := controls.Get() == "system"
			return core.Video{Ref: ref, Source: flowerClip, PausedFunc: paused.Get, Muted: true, Repeat: true, Controls: sys, ResizeModeFunc: fit.Get, TestID: "player",
				StyleFunc:  func() core.Style { return mediaFrame(cx, 200, core.RGB(0x000000)) },
				OnLoad:     func(i core.VideoInfo) { dur.Set(i.Duration) },
				OnError:    func(error) { failed.Set(true) },
				OnBuffer:   buffering.Set,
				OnProgress: func(p, d time.Duration) { pos.Set(p); dur.Set(d) }}
		}),
		core.Show{When: func() bool { return controls.Get() == "own" }, Then: func(*core.Scope) core.Node {
			return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12}, Children: []core.Node{
				core.Dynamic(func(*core.Scope) core.Node {
					if paused.Get() {
						return ui.RoundButton{Icon: ui.IconPlay, Label: tr.MediaPlay(), OnPress: func() { paused.Set(false) }}
					}
					return ui.RoundButton{Icon: ui.IconPause, Label: tr.MediaPause(), OnPress: func() { paused.Set(true) }}
				}),
				core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 8}, Children: []core.Node{
					ui.Meter{Value: func() float64 {
						if d := dur.Get(); d > 0 {
							return float64(pos.Get()) / float64(d)
						}
						return 0
					}, Tone: ui.ToneAccent},
					core.View{Style: core.Style{Direction: core.Row}, Children: []core.Node{
						core.Text{ContentFunc: func() string { return clockText(pos.Get()) }, Style: core.Style{FlexGrow: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
						ui.Textf(func() string { return clockText(dur.Get()) }, 13, ui.Regular, colors.Faint),
					}},
				}},
				ui.Button{Text: tr.MediaRestart(), Variant: ui.ButtonSecondary, OnPress: func() { ref.Seek(0) }},
			}}
		}},
		ui.Segmented[string]{Options: []ui.Option[string]{{Value: "own", Label: tr.MediaOwnControls()}, {Value: "system", Label: tr.MediaSystemControls()}}, Value: controls.Get, OnChange: controls.Set},
		ui.Segmented[core.ResizeMode]{Options: []ui.Option[core.ResizeMode]{{Value: core.Contain, Label: "Contain"}, {Value: core.Cover, Label: "Cover"}, {Value: core.Stretch, Label: "Stretch"}}, Value: fit.Get, OnChange: fit.Set, Small: true},
	}}
}

// nativeView is a platform view of the app's own, registered with
// nsgo.Component and shown by a Native element: the platform's slider.
func nativeView(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	value := core.NewSignal(cx, 0.4)
	events := core.NewSignal(cx, tr.MediaNoneYet())
	slider := &core.Ref{}
	_, ok := components[sliderComponent]
	noteText := tr.MediaThisPlatformRegistersNoSlider()
	if ok {
		noteText = tr.MediaDraggingThePlatformSOwn()
	}
	return ui.Card{Title: tr.MediaNativeViews(), Description: tr.MediaTheAppSOwnPlatform(), Actions: []core.Node{ui.Badge{Text: "NSGo"}}, Children: []core.Node{
		core.Native{Component: sliderComponent, Ref: slider, TestID: "native-slider", Label: tr.MediaNativeSlider(),
			PropsFunc: func() map[string]any { return map[string]any{"value": value.Get()} },
			OnEvent: func(name string, data any) {
				if v, ok := data.(float64); ok && name == "change" {
					value.Set(v)
				}
				events.Set(fmt.Sprintf("%s %v", name, data))
			},
			Style: core.Style{AlignSelf: core.AlignStretch, MinWidth: core.Pt(200), MinHeight: core.Pt(28)}},
		ui.Note{Text: noteText},
		ui.Caption{Label: tr.MediaValue(), Note: func() string { return fmt.Sprintf("%.2f", value.Get()) }},
		core.Text{TestID: "native-value", ContentFunc: func() string { return tr.MediaLastEvent() + events.Get() }, Style: core.Style{FontSize: 12, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Muted)},
		core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
			ui.Cell{Child: ui.Button{Text: tr.MediaSet08(), Variant: ui.ButtonSecondary, OnPress: func() { value.Set(0.8) }}},
			ui.Cell{Child: ui.Button{Text: tr.MediaResetCommand(), Variant: ui.ButtonSecondary, OnPress: func() { slider.Command("reset", nil) }}},
		}},
	}}
}

// mediaDemo is the platform's hosted views: a web view with messages both
// ways, a site with its history, a video player and the app's own views.
func mediaDemo(cx *core.Scope) core.Node {
	return demoPage(cx, pageProps{}, messaging, site, player, nativeView)
}

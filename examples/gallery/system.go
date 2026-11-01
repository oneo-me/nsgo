package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/desktop"
)

// safeAreaDiagram is the window drawn to scale, with the safe area and
// the keyboard shaded.
func safeAreaDiagram(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	tints := tintsOf(cx)
	fin := finOf(cx)
	env := cx.Env()
	const width = 76
	scale := func() core.Points { return width / max(1, env.Window().Width) }
	band := func(pt core.Points) core.Length { return core.Pt(max(0, core.Points(int(pt*scale()+0.5)))) }
	return core.View{StyleFunc: func() core.Style {
		hl := inner(0, 1, 0, core.RGB(0xffffff))
		if cx.Dark() {
			hl = inner(0, 1, 0, white(0.06))
		}
		return core.Style{Width: core.Pt(width), Height: core.Pt(core.Points(int(env.Window().Height*scale() + 0.5))), CornerRadius: 14, BorderCurve: core.BorderCurveContinuous,
			BorderWidth: 1.5, BorderColor: colors.border(), Background: colors.surface(), Gradients: fin.card().gradients, Shadows: []core.BoxShadow{hl}, Overflow: core.Hidden}
	}, Children: []core.Node{
		core.View{StyleFunc: func() core.Style { return core.Style{Height: band(env.SafeArea().Top), Background: tints.info()} }},
		core.View{StyleFunc: func() core.Style {
			return core.Style{FlexGrow: 1, Margin: core.All(6), CornerRadius: 6, BorderWidth: 1, BorderColor: colors.line(), BorderStyle: core.BorderDashed}
		}},
		core.View{StyleFunc: func() core.Style { return core.Style{Height: band(env.KeyboardHeight()), Background: tints.warn()} }},
		core.View{StyleFunc: func() core.Style { return core.Style{Height: band(env.SafeArea().Bottom), Background: tints.info()} }},
	}}
}

func environmentCard(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	env := cx.Env()
	inset := func(f func(core.Edges) core.Points) func() string {
		return func() string { return itoa(int(f(env.SafeArea()) + 0.5)) }
	}
	return card(cx, cardProps{icon: iconSystem, title: "Environment", subtitle: "Updates when the window, insets or keyboard change", right: badge(cx, "Live", toneUp)},
		core.View{Style: core.Style{Direction: core.Row, Gap: 16, AlignItems: core.AlignCenter}, Children: []core.Node{
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 12}, Children: []core.Node{
				core.View{Style: core.Style{Gap: 2}, Children: []core.Node{
					text("Window", 12, regular, colors.muted),
					figure(cx, func() string { w := env.Window(); return fmt.Sprintf("%d × %d", int(w.Width+0.5), int(w.Height+0.5)) }, "pt", nil, toneNeutral),
				}},
				core.View{Style: core.Style{Direction: core.Row}, Children: []core.Node{
					core.Dynamic(func(*core.Scope) core.Node {
						t := toneNeutral
						if env.KeyboardHeight() > 0 {
							t = toneWarn
						}
						return stat(cx, "Keyboard", func() string { return fmt.Sprintf("%d pt", int(env.KeyboardHeight()+0.5)) }, t, core.Pct(50))
					}),
					stat(cx, "Font scale", func() string {
						s := env.FontScale()
						if s == 0 {
							s = 1
						}
						return fmt.Sprintf("%.2f×", s)
					}, toneNeutral, core.Pct(50)),
				}},
			}},
			safeAreaDiagram(cx),
		}},
		core.View{Style: core.Style{Height: core.Pt(1)}, StyleFunc: filled(colors.line)},
		core.View{Style: core.Style{Gap: 4}, Children: []core.Node{
			caption(cx, "Safe area insets", label("pt")),
			core.View{Style: core.Style{Direction: core.Row}, Children: []core.Node{
				stat(cx, "Top", inset(func(e core.Edges) core.Points { return e.Top }), toneNeutral, core.Pct(25)),
				stat(cx, "Right", inset(func(e core.Edges) core.Points { return e.Right }), toneNeutral, core.Pct(25)),
				stat(cx, "Bottom", inset(func(e core.Edges) core.Points { return e.Bottom }), toneNeutral, core.Pct(25)),
				stat(cx, "Left", inset(func(e core.Edges) core.Points { return e.Left }), toneNeutral, core.Pct(25)),
			}},
		}},
		core.View{Style: core.Style{Height: core.Pt(1)}, StyleFunc: filled(colors.line)},
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 16}, Children: []core.Node{
			legend(cx, colors.info, "Safe area", nil), legend(cx, colors.warn, "Keyboard", nil), spacer(),
			toneBadge(cx, func() string {
				if env.Dark() {
					return "Dark scheme"
				}
				return "Light scheme"
			}, func() tone {
				if env.Dark() {
					return toneInfo
				}
				return toneWarn
			}, true),
		}},
	)
}

func statusBarCard(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	// Until one is picked, the content follows the appearance: light
	// content on the dark page.
	picked, chosen := core.NewSignal(cx, false), core.NewSignal(cx, core.StatusBarDark)
	foreground := func() core.StatusBarStyle {
		switch {
		case picked.Get():
			return chosen.Get()
		case cx.Dark():
			return core.StatusBarLight
		}
		return core.StatusBarDark
	}
	pick := func(s core.StatusBarStyle) { picked.Set(true); chosen.Set(s) }
	hidden, open := core.NewSignal(cx, false), core.NewSignal(cx, false)
	names := map[core.StatusBarStyle]string{core.StatusBarLight: "light", core.StatusBarDark: "dark", core.StatusBarAuto: "auto"}
	return card(cx, cardProps{icon: iconMobile, title: "Status bar", subtitle: "A dialog's StatusBar: over the page's while it is open"},
		segmented([]option[core.StatusBarStyle]{{value: core.StatusBarLight, label: "Light"}, {value: core.StatusBarDark, label: "Dark"}, {value: core.StatusBarAuto, label: "Auto"}},
			foreground, pick, false),
		listRow(cx, false, nil,
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: []core.Node{
				text("Hidden", 15, regular, colors.text),
				core.Text{ContentFunc: func() string { return fmt.Sprintf("Foreground: %s", names[foreground()]) }, Style: core.Style{FontSize: 12, FontFamily: mono}, StyleFunc: colored(colors.muted)},
			}},
			toggle(hidden.Get, hidden.Set, "Hide the status bar"),
		),
		button(buttonProps{label: "Preview", icon: iconMobile}, func() { open.Set(true) }),
		base.Dialog{Open: open.Get, Animation: core.ModalFade, OnOpenChange: func(o bool, _ base.Reason) { open.Set(o) }, Label: "Status bar preview",
			StatusBarFunc: func() core.StatusBarSetting {
				h := core.ToggleOff
				if hidden.Get() {
					h = core.ToggleOn
				}
				return core.StatusBarSetting{Foreground: foreground(), Hidden: h, Animated: core.ToggleOn}
			},
			PopupStyle: core.Style{FlexGrow: 1, AlignSelf: core.AlignStretch},
			Children: []core.Node{core.View{StyleFunc: func() core.Style {
				// The preview's background: the one its content is for.
				bg := colors.background()
				switch foreground() {
				case core.StatusBarLight:
					bg = galleryTheme().Dark.Background
				case core.StatusBarDark:
					bg = galleryTheme().Light.Background
				}
				return core.Style{FlexGrow: 1, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 16, Background: bg}
			}, Children: []core.Node{
				core.Text{ContentFunc: func() string {
					return fmt.Sprintf("StatusBar: {Foreground: %s, Hidden: %v}", names[foreground()], hidden.Get())
				}, StyleFunc: func() core.Style {
					c := galleryTheme().Light.Text
					if foreground() == core.StatusBarLight {
						c = core.RGB(0xffffff)
					}
					return core.Style{FontSize: 13, Color: c, FontFamily: mono}
				}},
				button(buttonProps{label: "Close"}, func() { open.Set(false) }),
			}}}},
	)
}

// actionRow is a pressable row with an icon tile, a title, a detail and
// a chevron.
func actionRow(cx *core.Scope, first bool, name iconName, tint func() core.Color, title, detail string, onPress func()) core.Node {
	colors := colorsOf(cx)
	fin := finOf(cx)
	return press(pressProps{onPress: onPress, scale: 0.98, label: title},
		listRow(cx, first, nil,
			core.View{StyleFunc: func() core.Style {
				return fin.raised().over(core.Style{Width: core.Pt(34), Height: core.Pt(34), CornerRadius: 10, BorderWidth: 1, BorderColor: colors.border(), Background: colors.raised(),
					AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter})
			}, Children: []core.Node{icon(name, 17, tint)}},
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 1, Padding: core.Symmetric(8, 0)}, Children: []core.Node{
				text(title, 15, regular, colors.text), text(detail, 12, regular, colors.muted)}},
			icon(iconChevron, 14, colors.faint),
		))
}

func dialogsCard(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	type answer struct {
		text string
		tone tone
	}
	ans := core.NewSignal(cx, answer{})
	// The answer pops in each time it changes.
	pop := core.NewAnimatedValue(cx, 1)
	show := func(text string, t tone) {
		ans.Set(answer{text, t})
		pop.Set(0.85)
		spring(cx, pop, 1, core.SpringConfig{Stiffness: 380, Damping: 18})
	}
	or := func(l string) string {
		if l == "" {
			return "dismissed"
		}
		return l
	}
	return card(cx, cardProps{icon: iconBell, title: "Alerts and sheets", subtitle: "AlertDialog · Toast", flush: true},
		inset(cx, insetProps{padding: 12, exact: true},
			actionRow(cx, true, iconCheck, colors.up, "Alert", "One button", func() {
				base.ShowAlertDialog(cx, base.AlertDialog{Title: "Saved", Description: "Your changes were saved.", Actions: []core.AlertAction{{Label: "OK"}}},
					func(string) { show("OK", toneUp) })
			}),
			actionRow(cx, false, iconBell, colors.danger, "Confirm", "Cancel and destructive buttons", func() {
				base.ShowAlertDialog(cx, base.AlertDialog{Title: "Delete the photo?", Description: "This cannot be undone.",
					Actions:       []core.AlertAction{{Label: "Cancel", Role: core.AlertCancel}, {Label: "Delete", Role: core.AlertDestructive}},
					CloseOnEscape: core.ToggleOn, CloseOnBackdropPress: core.ToggleOn},
					func(l string) {
						t := toneNeutral
						if l == "Delete" {
							t = toneDanger
						}
						show(or(l), t)
					})
			}),
			actionRow(cx, false, iconStar, colors.info, "Three buttons", "Reports the label of the action pressed", func() {
				base.ShowAlertDialog(cx, base.AlertDialog{Title: "Update available", Description: "Install it now?",
					Actions: []core.AlertAction{{Label: "Later"}, {Label: "Never", Role: core.AlertCancel}, {Label: "Install"}}},
					func(l string) {
						t := toneNeutral
						if l == "Install" {
							t = toneInfo
						}
						show(or(l), t)
					})
			}),
			actionRow(cx, false, iconLists, colors.accent, "Action sheet", "Title, cancel and destructive options", func() {
				base.ShowAlertDialog(cx, base.AlertDialog{Title: "Share the photo", Presentation: base.AlertPresentActionSheet,
					Actions: []core.AlertAction{{Label: "Cancel", Role: core.AlertCancel}, {Label: "Copy link"}, {Label: "Save"}, {Label: "Delete", Role: core.AlertDestructive}}},
					func(l string) {
						t := toneAccent
						if l == "Delete" {
							t = toneDanger
						}
						show(or(l), t)
					})
			}),
			actionRow(cx, false, iconSpark, colors.warn, "Toast", "base, every platform", func() {
				toast(toastContent{title: "Hello from a toast", description: "base's ToastManager, styled by the gallery"})
			}),
		),
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Padding: core.Edges{Top: 12, Right: 11, Bottom: 11, Left: 11}, MinHeight: core.Pt(44)}, Children: []core.Node{
			core.Text{Content: "Answer", Style: core.Style{FlexGrow: 1, FontSize: 13}, StyleFunc: colored(colors.muted)},
			core.Dynamic(func(*core.Scope) core.Node {
				a := ans.Get()
				if a.text == "" {
					return text("None yet", 13, regular, colors.faint)
				}
				return core.View{Style: core.Style{Animated: &core.AnimatedStyle{Scale: pop}}, Children: []core.Node{badge(cx, a.text, a.tone)}}
			}),
		}},
	)
}

func framesAndTimers(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	fps, running, ticks := core.NewSignal(cx, 0), core.NewSignal(cx, false), core.NewSignal(cx, 0)
	// The beat dot flashes once per tick; the fade runs on the platform.
	beat := core.NewAnimatedValue(cx, 0)
	// Frames per second, counted with RequestAnimationFrame.
	var stop func()
	frames := 0
	var since time.Time
	var loop func(at time.Time)
	loop = func(at time.Time) {
		frames++
		if d := at.Sub(since); d >= time.Second {
			fps.Set(int(float64(frames)/d.Seconds() + 0.5))
			frames, since = 0, at
		}
		stop = cx.RequestAnimationFrame(loop)
	}
	toggle := func() {
		if running.GetUntracked() {
			if stop != nil {
				stop()
			}
		} else {
			frames, since = 0, time.Now()
			stop = cx.RequestAnimationFrame(loop)
		}
		toggled(running)
	}
	cx.Cleanup(func() {
		if stop != nil {
			stop()
		}
	})
	every(cx, time.Second, func() {
		ticks.Update(func(n int) int { return n + 1 })
		beat.Set(1)
		timing(cx, beat, 0, core.TimingConfig{Duration: ms(700), Easing: ease(core.EasingQuad, core.EaseModeOut)})
	})
	return card(cx, cardProps{icon: iconAnimation, title: "Frames and timers", subtitle: "RequestAnimationFrame · AfterFunc"},
		core.View{Style: core.Style{Direction: core.Row, Gap: 12}, Children: []core.Node{
			core.View{Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), Gap: 2}, Children: []core.Node{
				text("Frame rate", 12, regular, colors.muted),
				core.Dynamic(func(*core.Scope) core.Node {
					if running.Get() {
						return figure(cx, func() string { return itoa(fps.Get()) }, "fps", label("live"), toneUp)
					}
					return figure(cx, label("–"), "fps", label("stopped"), toneNeutral)
				}),
			}},
			core.View{Style: core.Style{Width: core.Pt(1)}, StyleFunc: filled(colors.line)},
			core.View{Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), Gap: 2}, Children: []core.Node{
				core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6}, Children: []core.Node{
					text("Interval ticks", 12, regular, colors.muted),
					core.View{Style: core.Style{Width: core.Pt(7), Height: core.Pt(7), CornerRadius: 3.5,
						Animated: &core.AnimatedStyle{Opacity: lerp(beat, 0.25, 1), Scale: lerp(beat, 1, 1.6)}}, StyleFunc: filled(colors.accent)},
				}},
				figure(cx, func() string { return itoa(ticks.Get()) }, "", label("every 1 s"), toneAccent),
			}},
		}},
		core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
			caption(cx, "Share of 120 fps", func() string {
				if !running.Get() {
					return "–"
				}
				return fmt.Sprintf("%d%%", fps.Get()*100/120)
			}),
			meter(cx, func() float64 {
				if !running.Get() {
					return 0
				}
				return float64(fps.Get()) / 120
			}, toneUp, 0.5),
		}},
		core.Dynamic(func(*core.Scope) core.Node {
			if running.Get() {
				return button(buttonProps{label: "Stop counting frames", icon: iconPause}, toggle)
			}
			return button(buttonProps{label: "Count frames", icon: iconPlay}, toggle)
		}),
	)
}

// shortcuts are app-level shortcuts, bound while the demo shows.
func shortcuts(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	n, on, last := core.NewSignal(cx, 0), core.NewSignal(cx, false), core.NewSignal(cx, "")
	cx.OnKey("mod+k", func() { n.Update(func(c int) int { return c + 1 }); last.Set("mod+k") })
	cx.OnKey("mod+shift+l", func() { toggled(on); last.Set("mod+shift+l") })
	cx.OnKeyWith("mod+backspace", core.KeyOptions{When: core.KeyNotEditing}, func() { n.Set(0); last.Set("mod+backspace") })
	keys := [][2]string{{cx.KeyLabel("mod+k"), "Counts"}, {cx.KeyLabel("mod+shift+l"), "Switches the light"}, {cx.KeyLabel("mod+backspace"), "Resets (not while typing)"}}
	var rows []core.Node
	for i, k := range keys {
		rows = append(rows, listRow(cx, i == 0, nil,
			core.Text{Content: k[0], Style: core.Style{Width: core.Pt(120), FontSize: 14, FontFamily: mono}, StyleFunc: colored(colors.accent)},
			core.Text{Content: k[1], Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 15}, StyleFunc: colored(colors.text)}))
	}
	return card(cx, cardProps{icon: iconBell, title: "Keyboard shortcuts", subtitle: "Scope.OnKey · desktops and hardware keyboards"},
		core.View{Children: rows},
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
			core.Dynamic(func(*core.Scope) core.Node {
				if on.Get() {
					return badge(cx, "Light on", toneUp)
				}
				return badge(cx, "Light off", toneNeutral)
			}),
			core.View{Style: fill, Children: []core.Node{core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Gap: 8}, Children: []core.Node{
				textf(func() string { return fmt.Sprintf("Counted %d", n.Get()) }, 14, regular, colors.muted),
				spacer(),
				textf(func() string {
					if l := last.Get(); l != "" {
						return cx.KeyLabel(l)
					}
					return "–"
				}, 13, regular, colors.faint),
			}}}},
		}},
	)
}

type jsonPost struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

func networkCard(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	type result struct {
		posts []jsonPost
		line  string
		t     tone
	}
	res := core.NewSignalFunc(cx, result{}, func(a, b result) bool { return false })
	loading := core.NewSignal(cx, false)
	load := func() {
		loading.Set(true)
		started := time.Now()
		// The request runs on a goroutine of its own; Post brings its
		// result back to the engine.
		go func() {
			r := result{t: toneUp}
			resp, err := http.Get("https://jsonplaceholder.typicode.com/posts?_limit=3")
			if err != nil {
				r = result{line: err.Error(), t: toneDanger}
			} else {
				defer resp.Body.Close()
				if err := json.NewDecoder(resp.Body).Decode(&r.posts); err != nil {
					r = result{line: err.Error(), t: toneDanger}
				} else {
					r.line = fmt.Sprintf("HTTP %d · %d ms", resp.StatusCode, time.Since(started).Milliseconds())
					if resp.StatusCode >= 400 {
						r.t = toneDanger
					}
				}
			}
			cx.Post(func() {
				// A failure keeps the posts loaded before.
				if r.posts == nil {
					r.posts = res.GetUntracked().posts
				}
				res.Set(r)
				loading.Set(false)
			})
		}()
	}
	return card(cx, cardProps{icon: iconSearch, title: "Network and logs", subtitle: "net/http on a goroutine · log"},
		core.Dynamic(func(*core.Scope) core.Node {
			ps := res.Get().posts
			if len(ps) == 0 {
				return caption(cx, "Three posts from jsonplaceholder.typicode.com", nil)
			}
			var rows []core.Node
			for i, p := range ps {
				rows = append(rows, appear(i, false, listRow(cx, i == 0, nil,
					text(fmt.Sprintf("#%d", p.ID), 13, regular, colors.accent),
					core.Text{Content: p.Title, MaxLines: 1, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 15}, StyleFunc: colored(colors.text)})))
			}
			return core.View{Children: rows}
		}),
		core.Dynamic(func(*core.Scope) core.Node {
			line, t := res.Get().line, res.Get().t
			if loading.Get() {
				line, t = "loading…", toneNeutral
			}
			if line == "" {
				return core.Group{}
			}
			return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
				core.View{Style: core.Style{Width: core.Pt(7), Height: core.Pt(7), CornerRadius: 3.5}, StyleFunc: filled(t.shade(cx))},
				core.Text{Content: line, MaxLines: 2, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 13}, StyleFunc: colored(t.shade(cx))},
			}}
		}),
		core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
			cell(core.Dynamic(func(*core.Scope) core.Node {
				l := "http.Get"
				if loading.Get() {
					l = "Loading…"
				}
				return button(buttonProps{label: l, icon: iconArrowDown, disabled: loading.Get}, load)
			})),
			cell(button(buttonProps{label: "log.Printf", variant: secondary}, func() {
				log.Printf("hello from the gallery at %s", time.Now().Format(time.RFC3339))
			})),
		}},
		text("log.Printf shows in the terminal of nsgo dev.", 12, regular, colors.faint),
	)
}

// windowCard is the desktop window's title bar, commands, frame and
// level; phones and the web have no window to command.
func windowCard(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	env := cx.Env()
	small := func(l string, fn func()) core.Node {
		return button(buttonProps{label: l, variant: secondary, small: true}, fn)
	}
	if !desktop.IsAvailable(cx) {
		return card(cx, cardProps{icon: iconDock, title: "Window", subtitle: "Title bar, commands, frame and level", right: badge(cx, "Desktop", toneNeutral)},
			note(cx, "Window commands need a desktop window: run the gallery on macOS, Windows or Linux to try them."))
	}
	return card(cx, cardProps{icon: iconDock, title: "Window", subtitle: "Title bar, commands, frame and level", right: badge(cx, "NSGo", toneNeutral)},
		caption(cx, "Title bar", func() string {
			tb := desktop.TitleBar(cx)
			if tb == (core.Edges{}) {
				return "the system's"
			}
			return fmt.Sprintf("%.0f pt, controls keep %.0f / %.0f pt", tb.Top, tb.Left, tb.Right)
		}),
		wrap(
			small("Hide title bar", func() { desktop.SetTitleBar(cx, desktop.TitleBarHidden, 40) }),
			small("Show title bar", func() { desktop.SetTitleBar(cx, desktop.TitleBarDefault, 0) }),
		),
		caption(cx, "Window controls", nil),
		wrap(
			small("Default", func() { desktop.SetWindowControls(cx, desktop.WindowControls{}) }),
			small("Traffic lights at 24, 10", func() {
				desktop.SetWindowControls(cx, desktop.WindowControls{TrafficLights: core.Point{X: 24, Y: 10}})
			}),
			small("Hidden", func() { desktop.SetWindowControls(cx, desktop.WindowControls{Hidden: true}) }),
		),
		caption(cx, "Material", func() string {
			if desktop.ShowsMaterial(cx) {
				return "shows behind the sidebar"
			}
			return "none"
		}),
		wrap(
			small("None", func() { desktop.SetWindowMaterial(cx, desktop.WindowMaterialNone) }),
			small("Sidebar", func() { desktop.SetWindowMaterial(cx, desktop.WindowMaterialSidebar) }),
			small("Under window", func() { desktop.SetWindowMaterial(cx, desktop.WindowMaterialUnderWindow) }),
			small("HUD", func() { desktop.SetWindowMaterial(cx, desktop.WindowMaterialHUD) }),
		),
		caption(cx, "Commands", nil),
		wrap(small("Minimize", cx.MinimizeWindow), small("Zoom", cx.ToggleMaximize), small("Full screen", cx.ToggleFullScreen),
			small("Away for 2 s", func() { desktop.MinimizeWindow(cx); cx.AfterFunc(2*time.Second, func() { desktop.OrderFront(cx) }) })),
		caption(cx, "Frame and level", nil),
		wrap(
			small("640 × 480", func() { desktop.SetWindowFrame(cx, desktop.WindowFrame{Width: 640, Height: 480}) }),
			small("Taller, animated", func() {
				desktop.SetWindowFrame(cx, desktop.WindowFrame{Height: env.Window().Height + 160, Animated: true})
			}),
			small("Move to 80, 80", func() { desktop.SetWindowFrame(cx, desktop.WindowFrame{X: 80, Y: 80, Move: true}) }),
		),
		wrap(
			small("Normal", func() { desktop.SetWindowLevel(cx, desktop.WindowLevelNormal) }),
			small("Floating", func() { desktop.SetWindowLevel(cx, desktop.WindowLevelFloating) }),
			small("Desktop", func() { desktop.SetWindowLevel(cx, desktop.WindowLevelDesktop) }),
		),
		core.View{Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOn}, StyleFunc: func() core.Style {
			return core.Style{Height: core.Pt(52), CornerRadius: radiusControl, Background: colors.raised(), BorderWidth: 1, BorderColor: colors.border(),
				Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifySpaceBetween, Padding: core.Symmetric(0, 14)}
		}, Children: []core.Node{
			text("Drag the window from here", 14, regular, colors.muted),
			core.View{Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOff}, Children: []core.Node{small("Still pressable", func() {})}},
		}},
		button(buttonProps{label: "Quit", variant: secondary, color: colors.danger}, cx.Quit),
	)
}

// clipboardCard is text, images and files on the clipboard.
func clipboardCard(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	clip := core.NewSignal(cx, "Hello from NSGo")
	status := core.NewSignal(cx, "Nothing read yet")
	image := core.NewSignalFunc(cx, (*core.ClipboardImage)(nil), func(a, b *core.ClipboardImage) bool { return a == b })
	small := func(l string, fn func()) core.Node {
		return button(buttonProps{label: l, variant: secondary, small: true}, fn)
	}
	return card(cx, cardProps{icon: iconCopy, title: "Clipboard", subtitle: "Text, images and the files a file manager copied", right: badge(cx, "NSGo", toneNeutral)},
		darkField(base.Input{Value: clip.Get, OnChange: clip.Set, Label: "Clipboard text"}),
		wrap(
			button(buttonProps{label: "Copy text", small: true}, func() { cx.SetClipboard(clip.GetUntracked()) }),
			small("Paste text", func() { cx.ReadClipboard(func(s string) { status.Set("Text: " + s) }) }),
			small("Copy logo", func() { cx.SetClipboardImage(logoPNG) }),
			small("Paste image", func() {
				cx.ReadClipboardImage(func(img core.ClipboardImage, ok bool) {
					if !ok {
						image.Set(nil)
						status.Set("The clipboard holds no image")
						return
					}
					image.Set(&img)
					status.Set(fmt.Sprintf("Image: %.0f × %.0f px, %d bytes of PNG", img.Size.Width, img.Size.Height, len(img.PNG)))
				})
			}),
			small("Paste files", func() {
				cx.ReadClipboardFiles(func(uris []string) {
					if len(uris) == 0 {
						status.Set("The clipboard holds no files")
						return
					}
					status.Set("Files: " + strings.Join(uris, ", "))
				})
			}),
			small("What is there?", func() {
				cx.HasClipboardText(func(t bool) {
					cx.HasClipboardImage(func(i bool) { status.Set(fmt.Sprintf("Text: %v, image: %v", t, i)) })
				})
			}),
		),
		core.Text{ContentFunc: status.Get, TestID: "clipboard-status", Style: core.Style{FontSize: 13}, StyleFunc: colored(colors.muted)},
		core.Show{When: func() bool { return image.Get() != nil }, Then: func(*core.Scope) core.Node {
			return core.Image{SourceFunc: func() core.ImageSource { return core.ImageSource{Data: image.Get().PNG} }, ResizeMode: core.Contain, Label: "Pasted image", TestID: "pasted-image",
				Style: imageFrame(cx, core.Style{Width: core.Pt(120), Height: core.Pt(120)}), StyleFunc: frameInk(cx)}
		}},
	)
}

// servicesCard is opening links, the photo picker and the app's
// appearance.
func servicesCard(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	env := cx.Env()
	opened := core.NewSignal(cx, "Nothing opened yet")
	open := func(u string) func() {
		return func() {
			cx.OpenURL(u, func(ok bool) {
				if ok {
					opened.Set("Opened " + u)
				} else {
					opened.Set("Nothing opened " + u)
				}
			})
		}
	}
	picked := core.NewSignalFunc(cx, []core.PickedFile(nil), func(a, b []core.PickedFile) bool { return false })
	asked := core.NewSignal(cx, false)
	pick := func(o core.PickOptions) func() {
		return func() { cx.PickImages(o, func(files []core.PickedFile) { asked.Set(true); picked.Set(files) }) }
	}
	small := func(l string, fn func()) core.Node {
		return button(buttonProps{label: l, variant: secondary, small: true}, fn)
	}
	names := map[core.Appearance]string{core.AppearanceSystem: "the system's", core.AppearanceLight: "light", core.AppearanceDark: "dark"}
	return card(cx, cardProps{icon: iconSpark, title: "Links, photos and appearance", subtitle: "OpenURL · PickImages · SetAppearance", right: badge(cx, "NSGo", toneNeutral)},
		caption(cx, "OpenURL", nil),
		wrap(small("The website", open("https://github.com/oneo-me/nsgo")), small("An email", open("mailto:hello@example.com?subject=Hello%20from%20NSGo")),
			small("Refused URL", open("javascript:alert(1)"))),
		core.Text{ContentFunc: opened.Get, Style: core.Style{FontSize: 13}, StyleFunc: colored(colors.muted)},
		core.View{Style: core.Style{Height: core.Pt(1)}, StyleFunc: filled(colors.line)},
		caption(cx, "PickImages", nil),
		wrap(small("Pick a photo", pick(core.PickOptions{})), small("Up to 4, 800 px", pick(core.PickOptions{Multiple: true, Limit: 4, MaxDimension: 800}))),
		core.Dynamic(func(cx *core.Scope) core.Node {
			colors := colorsOf(cx)
			files := picked.Get()
			if len(files) == 0 {
				if asked.Get() {
					return note(cx, "No photo picked")
				}
				return note(cx, "HEIC arrives as JPEG, large photos scaled down when asked")
			}
			var tiles []core.Node
			for i, f := range files {
				tiles = append(tiles, core.View{Style: core.Style{Gap: 4, Width: core.Pt(100)}, Children: []core.Node{
					core.Image{Source: core.ImageSource{Data: f.Data}, ResizeMode: core.Cover, Label: f.Name, TestID: fmt.Sprintf("picked-%d", i),
						Style: imageFrame(cx, core.Style{Width: core.Pt(100), Height: core.Pt(76)}), StyleFunc: frameInk(cx)},
					core.Text{Content: f.Name, MaxLines: 1, Style: core.Style{FontSize: 12}, StyleFunc: colored(colors.text)},
					text(fmt.Sprintf("%.0f × %.0f, %d KB", f.Size.Width, f.Size.Height, (len(f.Data)+512)/1024), 12, regular, colors.muted),
				}})
			}
			return wrap(tiles...)
		}),
		core.View{Style: core.Style{Height: core.Pt(1)}, StyleFunc: filled(colors.line)},
		caption(cx, "SetAppearance", func() string { return "forced: " + names[env.Appearance()] }),
		wrap(small("System", func() { cx.SetAppearance(core.AppearanceSystem) }), small("Light", func() { cx.SetAppearance(core.AppearanceLight) }),
			small("Dark", func() { cx.SetAppearance(core.AppearanceDark) })),
	)
}

// filesCard is the system's file picker and save dialog.
func filesCard(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	picked := core.NewSignalFunc(cx, []core.File(nil), func(a, b []core.File) bool { return false })
	status := core.NewSignal(cx, "Files arrive on disk, copied into the app on iOS and Android")
	pick := func(o core.PickFilesOptions) func() {
		return func() {
			cx.PickFiles(o, func(files []core.File, err error) {
				switch {
				case err != nil:
					status.Set("The pick failed: " + err.Error())
				case len(files) == 0:
					status.Set("No file picked")
				default:
					status.Set(fmt.Sprintf("Picked %d", len(files)))
				}
				picked.Set(files)
			})
		}
	}
	save := func(o core.SaveOptions) {
		cx.SaveFile(o, func(saved bool, err error) {
			switch {
			case err != nil:
				status.Set("The save failed: " + err.Error())
			case saved:
				status.Set("Saved " + o.Name)
			default:
				status.Set("Nothing saved")
			}
		})
	}
	small := func(l string, fn func()) core.Node {
		return button(buttonProps{label: l, variant: secondary, small: true}, fn)
	}
	return card(cx, cardProps{icon: iconAttach, title: "Files", subtitle: "PickFiles · SaveFile", right: badge(cx, "NSGo", toneNeutral)},
		caption(cx, "PickFiles", nil),
		wrap(small("Pick a file", pick(core.PickFilesOptions{})),
			small("PDFs and images", pick(core.PickFilesOptions{Multiple: true, Types: []string{"application/pdf", "image/*"}}))),
		core.Dynamic(func(cx *core.Scope) core.Node {
			colors := colorsOf(cx)
			var rows []core.Node
			for i, f := range picked.Get() {
				rows = append(rows, core.Text{Content: fmt.Sprintf("%s · %s · %d KB", f.Name, f.MIME, (f.Size+512)/1024),
					TestID: fmt.Sprintf("picked-file-%d", i), MaxLines: 1, Style: core.Style{FontSize: 13}, StyleFunc: colored(colors.text)})
			}
			return core.View{Style: core.Style{Gap: 4}, Children: rows}
		}),
		core.View{Style: core.Style{Height: core.Pt(1)}, StyleFunc: filled(colors.line)},
		caption(cx, "SaveFile", nil),
		wrap(small("Save a text file", func() {
			save(core.SaveOptions{Name: "Hello from NSGo.txt", Data: []byte("Hello from NSGo\n")})
		}), small("Save the file picked", func() {
			if files := picked.Get(); len(files) > 0 && files[0].Path != "" {
				save(core.SaveOptions{Name: files[0].Name, Path: files[0].Path})
			} else {
				status.Set("Pick a file first")
			}
		})),
		core.Text{ContentFunc: status.Get, TestID: "files-status", Style: core.Style{FontSize: 13}, StyleFunc: colored(colors.muted)},
	)
}

// systemDemo is what the platform does for the app: the environment, the
// status bar, alerts, frames and timers, shortcuts, the network, the
// window, the clipboard, links, photos, the appearance and files.
func systemDemo(cx *core.Scope) core.Node {
	return demoPage(cx, pageProps{}, environmentCard, statusBarCard, dialogsCard, framesAndTimers, shortcuts, networkCard, windowCard, clipboardCard, servicesCard, filesCard)
}

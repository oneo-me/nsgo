package main

import (
	"encoding/json"
	"fmt"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/mobile"

	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/ui"
)

// safeAreaDiagram is the window drawn to scale, with the safe area and
// the keyboard shaded.
func safeAreaDiagram(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	tints := ui.TintsOf(cx)
	fin := ui.FinOf(cx)
	env := cx.Env()
	const width = 76
	scale := func() core.Points { return width / max(1, env.Window().Width) }
	band := func(pt core.Points) core.Length { return core.Pt(max(0, core.Points(int(pt*scale()+0.5)))) }
	return core.View{StyleFunc: func() core.Style {
		hl := ui.Inner(0, 1, 0, core.RGB(0xffffff))
		if cx.Dark() {
			hl = ui.Inner(0, 1, 0, ui.White(0.06))
		}
		return core.Style{Width: core.Pt(width), Height: core.Pt(core.Points(int(env.Window().Height*scale() + 0.5))), CornerRadius: 14, BorderCurve: core.BorderCurveContinuous,
			BorderWidth: 1.5, BorderColor: colors.Border(), Background: colors.Surface(), Gradients: fin.Card().Gradients, Shadows: []core.BoxShadow{hl}, Overflow: core.Hidden}
	}, Children: []core.Node{
		core.View{StyleFunc: func() core.Style { return core.Style{Height: band(env.SafeArea().Top), Background: tints.Info()} }},
		core.View{StyleFunc: func() core.Style {
			return core.Style{FlexGrow: 1, Margin: core.All(6), CornerRadius: 6, BorderWidth: 1, BorderColor: colors.Line(), BorderStyle: core.BorderDashed}
		}},
		core.View{StyleFunc: func() core.Style { return core.Style{Height: band(env.KeyboardHeight()), Background: tints.Warn()} }},
		core.View{StyleFunc: func() core.Style { return core.Style{Height: band(env.SafeArea().Bottom), Background: tints.Info()} }},
	}}
}

func environmentCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	env := cx.Env()
	inset := func(f func(core.Edges) core.Points) func() string {
		return func() string { return itoa(int(f(env.SafeArea()) + 0.5)) }
	}
	return Card{Title: tr.SystemEnvironment(), Description: tr.SystemUpdatesWhenTheWindowInsets(), Actions: []core.Node{ui.Badge{Text: tr.SystemLive(), Variant: ui.BadgeUp}}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, Gap: 16, AlignItems: core.AlignCenter}, Children: []core.Node{
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 12}, Children: []core.Node{
				core.View{Style: core.Style{Gap: 2}, Children: []core.Node{
					ui.Text(tr.SystemWindow(), 12, ui.Regular, colors.Muted),
					Figure{Value: func() string { w := env.Window(); return fmt.Sprintf("%d × %d", int(w.Width+0.5), int(w.Height+0.5)) }, Unit: "pt"},
				}},
				core.View{Style: core.Style{Direction: core.Row}, Children: []core.Node{
					core.Dynamic(func(*core.Scope) core.Node {
						t := ToneNeutral
						if env.KeyboardHeight() > 0 {
							t = ToneWarn
						}
						return Stat{Label: tr.SystemKeyboard(), Value: func() string { return fmt.Sprintf("%d pt", int(env.KeyboardHeight()+0.5)) }, Tone: t, Width: core.Pct(50)}
					}),
					Stat{Label: tr.SystemFontScale(), Value: func() string {
						s := env.FontScale()
						if s == 0 {
							s = 1
						}
						return fmt.Sprintf("%.2f×", s)
					}, Width: core.Pct(50)},
				}},
			}},
			safeAreaDiagram(cx),
		}},
		core.View{Style: core.Style{Height: core.Pt(1)}, StyleFunc: ui.Filled(colors.Line)},
		core.View{Style: core.Style{Gap: 4}, Children: []core.Node{
			Caption{Label: tr.SystemSafeAreaInsets(), Note: label("pt")},
			core.View{Style: core.Style{Direction: core.Row}, Children: []core.Node{
				Stat{Label: tr.SystemTop(), Value: inset(func(e core.Edges) core.Points { return e.Top }), Width: core.Pct(25)},
				Stat{Label: tr.SystemRight(), Value: inset(func(e core.Edges) core.Points { return e.Right }), Width: core.Pct(25)},
				Stat{Label: tr.SystemBottom(), Value: inset(func(e core.Edges) core.Points { return e.Bottom }), Width: core.Pct(25)},
				Stat{Label: tr.SystemLeft(), Value: inset(func(e core.Edges) core.Points { return e.Left }), Width: core.Pct(25)},
			}},
		}},
		core.View{Style: core.Style{Height: core.Pt(1)}, StyleFunc: ui.Filled(colors.Line)},
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 16}, Children: []core.Node{
			Legend{Color: colors.Info, Label: tr.SystemSafeArea()}, Legend{Color: colors.Warn, Label: tr.SystemKeyboard()}, ui.Spacer{},
			ui.Badge{TextFunc: func() string {
				if env.Dark() {
					return tr.SystemDarkScheme()
				}
				return tr.SystemLightScheme()
			}, VariantFunc: func() ui.BadgeVariant {
				if env.Dark() {
					return ui.BadgeInfo
				}
				return ui.BadgeWarn
			}},
		}},
	}}
}

func statusBarCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
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
	return Card{Title: tr.SystemStatusBar(), Description: tr.SystemADialogSStatusbarOver(), Children: []core.Node{
		ui.Tabs[core.StatusBarStyle]{Equal: true, Options: []ui.Option[core.StatusBarStyle]{{Value: core.StatusBarLight, Label: tr.SystemLight()}, {Value: core.StatusBarDark, Label: tr.SystemDark()}, {Value: core.StatusBarAuto, Label: tr.SystemAuto()}}, Value: foreground, OnChange: pick},
		Row{Children: []core.Node{
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: []core.Node{
				ui.Text(tr.SystemHidden(), 13, ui.Regular, colors.Text),
				core.Text{ContentFunc: func() string { return fmt.Sprintf("Foreground: %s", names[foreground()]) }, Style: core.Style{FontSize: 12, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Muted)},
			}},
			ui.Switch{Checked: hidden.Get, OnChange: hidden.Set, Label: tr.SystemHideTheStatusBar()},
		}},
		ui.Button{Text: tr.SystemPreview(), Icon: lucide.Smartphone, OnPress: func() { open.Set(true) }},
		base.Dialog{Open: open.Get, Animation: core.ModalFade, OnOpenChange: func(o bool, _ base.Reason) { open.Set(o) }, Label: tr.SystemStatusBarPreview(),
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
				bg := colors.Background()
				switch foreground() {
				case core.StatusBarLight:
					bg = cx.Theme().Dark.Background
				case core.StatusBarDark:
					bg = cx.Theme().Light.Background
				}
				return core.Style{FlexGrow: 1, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 16, Background: bg}
			}, Children: []core.Node{
				core.Text{ContentFunc: func() string {
					return fmt.Sprintf("StatusBar: {Foreground: %s, Hidden: %v}", names[foreground()], hidden.Get())
				}, StyleFunc: func() core.Style {
					c := cx.Theme().Light.Text
					if foreground() == core.StatusBarLight {
						c = core.RGB(0xffffff)
					}
					return core.Style{FontSize: 13, Color: c, FontFamily: ui.Mono}
				}},
				ui.Button{Text: tr.SystemClose(), OnPress: func() { open.Set(false) }},
			}}}},
	}}
}

// actionRow is a pressable row with an icon tile, a title, a detail and
// a chevron.
func actionRow(cx *core.Scope, first bool, name func() *core.SVG, tint func() core.Color, title, detail string, onPress func()) core.Node {
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	return core.View{OnPress: func(core.PressEvent) { onPress() }, Role: core.RoleButton, Label: title, Children: []core.Node{
		Row{First: first, Children: []core.Node{
			core.View{StyleFunc: func() core.Style {
				return fin.Raised().Over(core.Style{Width: core.Pt(34), Height: core.Pt(34), CornerRadius: 10, BorderWidth: 1, BorderColor: colors.Border(), Background: colors.Raised(),
					AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter})
			}, Children: []core.Node{ui.Icon(name, 17, tint)}},
			core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 1, Padding: core.Symmetric(8, 0)}, Children: []core.Node{
				ui.Text(title, 13, ui.Regular, colors.Text), ui.Text(detail, 12, ui.Regular, colors.Muted)}},
			ui.Icon(lucide.ChevronRight, 14, colors.Faint),
		}},
	}}
}

func dialogsCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	type answer struct {
		text string
		tone Tone
	}
	ans := core.NewSignal(cx, answer{})
	// The answer pops in each time it changes.
	pop := core.NewAnimatedValue(cx, 1)
	show := func(text string, t Tone) {
		ans.Set(answer{text, t})
		pop.Set(0.85)
		ui.Spring(cx, pop, 1, core.SpringConfig{Stiffness: 380, Damping: 18})
	}
	or := func(l string) string {
		if l == "" {
			return "dismissed"
		}
		return l
	}
	return Card{Title: tr.SystemAlertsAndSheets(), Description: tr.SystemAlertdialogToast(), Flush: true, Children: []core.Node{
		Inset{Children: []core.Node{
			actionRow(cx, true, lucide.Check, colors.Up, tr.SystemAlert(), tr.SystemOneButton(), func() {
				base.ShowAlertDialog(cx, base.AlertDialog{Title: tr.SystemSaved(), Description: tr.SystemYourChangesWereSaved(), Actions: []core.AlertAction{{Label: tr.SystemOk()}}},
					func(string) { show(tr.SystemOk(), ToneUp) })
			}),
			actionRow(cx, false, lucide.Bell, colors.Danger, tr.SystemConfirm(), tr.SystemCancelAndDestructiveButtons(), func() {
				base.ShowAlertDialog(cx, base.AlertDialog{Title: tr.SystemDeleteThePhoto(), Description: tr.SystemThisCannotBeUndone(),
					Actions:       []core.AlertAction{{Label: tr.SystemCancel(), Role: core.AlertCancel}, {Label: tr.SystemDelete(), Role: core.AlertDestructive}},
					CloseOnEscape: core.ToggleOn, CloseOnBackdropPress: core.ToggleOn},
					func(l string) {
						t := ToneNeutral
						if l == tr.SystemDelete() {
							t = ToneDanger
						}
						show(or(l), t)
					})
			}),
			actionRow(cx, false, lucide.Star, colors.Info, tr.SystemThreeButtons(), tr.SystemReportsTheLabelOfThe(), func() {
				base.ShowAlertDialog(cx, base.AlertDialog{Title: tr.SystemUpdateAvailable(), Description: tr.SystemInstallItNow(),
					Actions: []core.AlertAction{{Label: tr.SystemLater()}, {Label: tr.SystemNever(), Role: core.AlertCancel}, {Label: tr.SystemInstall()}}},
					func(l string) {
						t := ToneNeutral
						if l == tr.SystemInstall() {
							t = ToneInfo
						}
						show(or(l), t)
					})
			}),
			actionRow(cx, false, lucide.List, colors.Accent, tr.SystemActionSheet(), tr.SystemTitleCancelAndDestructiveOptions(), func() {
				base.ShowAlertDialog(cx, base.AlertDialog{Title: tr.SystemShareThePhoto(), Presentation: base.AlertPresentActionSheet,
					Actions: []core.AlertAction{{Label: tr.SystemCancel(), Role: core.AlertCancel}, {Label: tr.SystemCopyLink()}, {Label: tr.SystemSave()}, {Label: tr.SystemDelete(), Role: core.AlertDestructive}}},
					func(l string) {
						t := ToneAccent
						if l == tr.SystemDelete() {
							t = ToneDanger
						}
						show(or(l), t)
					})
			}),
			actionRow(cx, false, lucide.Sparkles, colors.Warn, tr.SystemToast(), tr.SystemBaseEveryPlatform(), func() {
				shell.toasts.Show(ui.ToastContent{Title: tr.SystemHelloFromAToast(), Description: tr.SystemBaseSToastmanagerStyledBy()})
			}),
		}},
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Padding: core.Edges{Top: 12, Right: 11, Bottom: 11, Left: 11}, MinHeight: core.Pt(44)}, Children: []core.Node{
			core.Text{Content: tr.SystemAnswer(), Style: core.Style{FlexGrow: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
			core.Dynamic(func(*core.Scope) core.Node {
				a := ans.Get()
				if a.text == "" {
					return ui.Text(tr.SystemNoneYet(), 13, ui.Regular, colors.Faint)
				}
				return core.View{Style: core.Style{Animated: &core.AnimatedStyle{Scale: pop}}, Children: []core.Node{ui.Badge{Text: a.text, Variant: a.tone.Badge()}}}
			}),
		}},
	}}
}

func framesAndTimers(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
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
		ui.Timing(cx, beat, 0, core.TimingConfig{Duration: ms(700), Easing: ease(core.EasingQuad, core.EaseModeOut)})
	})
	return Card{Title: tr.SystemFramesAndTimers(), Description: tr.SystemRequestanimationframeAfterfunc(), Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, Gap: 12}, Children: []core.Node{
			core.View{Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), Gap: 2}, Children: []core.Node{
				ui.Text(tr.SystemFrameRate(), 12, ui.Regular, colors.Muted),
				core.Dynamic(func(*core.Scope) core.Node {
					if running.Get() {
						return Figure{Value: func() string { return itoa(fps.Get()) }, Unit: "fps", Note: label("live"), Tone: ToneUp}
					}
					return Figure{Value: label("–"), Unit: "fps", Note: label("stopped")}
				}),
			}},
			core.View{Style: core.Style{Width: core.Pt(1)}, StyleFunc: ui.Filled(colors.Line)},
			core.View{Style: core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), Gap: 2}, Children: []core.Node{
				core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6}, Children: []core.Node{
					ui.Text(tr.SystemIntervalTicks(), 12, ui.Regular, colors.Muted),
					core.View{Style: core.Style{Width: core.Pt(7), Height: core.Pt(7), CornerRadius: 3.5,
						Animated: &core.AnimatedStyle{Opacity: ui.Lerp(beat, 0.25, 1), Scale: ui.Lerp(beat, 1, 1.6)}}, StyleFunc: ui.Filled(colors.Accent)},
				}},
				Figure{Value: func() string { return itoa(ticks.Get()) }, Note: label(tr.SystemEvery1S()), Tone: ToneAccent},
			}},
		}},
		core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
			Caption{Label: tr.SystemShareOf120Fps(), Note: func() string {
				if !running.Get() {
					return "–"
				}
				return fmt.Sprintf("%d%%", fps.Get()*100/120)
			}},
			ui.Meter{Value: func() float64 {
				if !running.Get() {
					return 0
				}
				return float64(fps.Get()) / 120
			}, Variant: ui.MeterUp, Marker: 0.5},
		}},
		core.Dynamic(func(*core.Scope) core.Node {
			if running.Get() {
				return ui.Button{Text: tr.SystemStopCountingFrames(), Icon: lucide.Pause, OnPress: toggle}
			}
			return ui.Button{Text: tr.SystemCountFrames(), Icon: lucide.Play, OnPress: toggle}
		}),
	}}
}

// shortcuts are app-level shortcuts, bound while the demo shows.
func shortcuts(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	n, on, last := core.NewSignal(cx, 0), core.NewSignal(cx, false), core.NewSignal(cx, "")
	cx.OnKey("mod+k", func() { n.Update(func(c int) int { return c + 1 }); last.Set("mod+k") })
	cx.OnKey("mod+shift+l", func() { toggled(on); last.Set("mod+shift+l") })
	cx.OnKeyWith("mod+backspace", core.KeyOptions{When: core.KeyNotEditing}, func() { n.Set(0); last.Set("mod+backspace") })
	keys := [][2]string{{cx.KeyLabel("mod+k"), tr.SystemCounts()}, {cx.KeyLabel("mod+shift+l"), tr.SystemSwitchesTheLight()}, {cx.KeyLabel("mod+backspace"), tr.SystemResetsNotWhileTyping()}}
	var rows []core.Node
	for i, k := range keys {
		rows = append(rows, Row{First: i == 0, Children: []core.Node{
			core.Text{Content: k[0], Style: core.Style{Width: core.Pt(120), FontSize: 13, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Accent)},
			core.Text{Content: k[1], Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Text)},
		}})
	}
	return Card{Title: tr.SystemKeyboardShortcuts(), Description: tr.SystemScopeOnkeyDesktopsAndHardware(), Children: []core.Node{
		core.View{Children: rows},
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
			core.Dynamic(func(*core.Scope) core.Node {
				if on.Get() {
					return ui.Badge{Text: tr.SystemLightOn(), Variant: ui.BadgeUp}
				}
				return ui.Badge{Text: tr.SystemLightOff()}
			}),
			core.View{Style: fill, Children: []core.Node{core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Gap: 8}, Children: []core.Node{
				ui.Textf(func() string { return tr.SystemCounted(n.Get()) }, 13, ui.Regular, colors.Muted),
				ui.Spacer{},
				ui.Textf(func() string {
					if l := last.Get(); l != "" {
						return cx.KeyLabel(l)
					}
					return "–"
				}, 13, ui.Regular, colors.Faint),
			}}}},
		}},
	}}
}

type jsonPost struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

func networkCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	type result struct {
		posts []jsonPost
		line  string
		t     Tone
	}
	res := core.NewSignalFunc(cx, result{}, func(a, b result) bool { return false })
	loading := core.NewSignal(cx, false)
	load := func() {
		loading.Set(true)
		started := time.Now()
		// The request runs on a goroutine of its own; Post brings its
		// result back to the engine.
		go func() {
			r := result{t: ToneUp}
			resp, err := http.Get("https://jsonplaceholder.typicode.com/posts?_limit=3")
			if err != nil {
				r = result{line: err.Error(), t: ToneDanger}
			} else {
				defer resp.Body.Close()
				if err := json.NewDecoder(resp.Body).Decode(&r.posts); err != nil {
					r = result{line: err.Error(), t: ToneDanger}
				} else {
					r.line = fmt.Sprintf("HTTP %d · %d ms", resp.StatusCode, time.Since(started).Milliseconds())
					if resp.StatusCode >= 400 {
						r.t = ToneDanger
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
	return Card{Title: tr.SystemNetworkAndLogs(), Description: tr.SystemNetHttpOnAGoroutine(), Children: []core.Node{
		core.Dynamic(func(*core.Scope) core.Node {
			ps := res.Get().posts
			if len(ps) == 0 {
				return Caption{Label: tr.SystemThreePostsFromJsonplaceholderTypicode()}
			}
			var rows []core.Node
			for i, p := range ps {
				rows = append(rows, Appear{Index: i, Children: []core.Node{Row{First: i == 0, Children: []core.Node{
					ui.Text(fmt.Sprintf("#%d", p.ID), 13, ui.Regular, colors.Accent),
					core.Text{Content: p.Title, MaxLines: 1, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Text)},
				}}}})
			}
			return core.View{Children: rows}
		}),
		core.Dynamic(func(*core.Scope) core.Node {
			line, t := res.Get().line, res.Get().t
			if loading.Get() {
				line, t = "loading…", ToneNeutral
			}
			if line == "" {
				return core.Group{}
			}
			return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
				core.View{Style: core.Style{Width: core.Pt(7), Height: core.Pt(7), CornerRadius: 3.5}, StyleFunc: ui.Filled(t.Shade(cx))},
				core.Text{Content: line, MaxLines: 2, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 13}, StyleFunc: ui.Colored(t.Shade(cx))},
			}}
		}),
		core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
			ui.Cell{Child: core.Dynamic(func(*core.Scope) core.Node {
				l := "http.Get"
				if loading.Get() {
					l = tr.SystemLoading()
				}
				return ui.Button{Text: l, Icon: lucide.ArrowDown, Disabled: loading.Get, OnPress: load}
			})},
			ui.Cell{Child: ui.Button{Text: "log.Printf", Variant: ui.ButtonSecondary, OnPress: func() {
				log.Printf("hello from the gallery at %s", time.Now().Format(time.RFC3339))
			}}},
		}},
		ui.Text(tr.SystemLogPrintfShowsInThe(), 12, ui.Regular, colors.Faint),
	}}
}

// windowCard is the desktop window's title bar, commands, frame and
// level; phones and the web have no window to command.
func windowCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	env := cx.Env()
	small := func(l string, fn func()) core.Node {
		return ui.Button{Text: l, Variant: ui.ButtonSecondary, Size: ui.SizeSmall, OnPress: fn}
	}
	if !desktop.IsAvailable(cx) {
		return Card{Title: tr.SystemWindow(), Description: tr.SystemTitleBarCommandsFrameAnd(), Actions: []core.Node{ui.Badge{Text: tr.SystemDesktop()}}, Children: []core.Node{
			Note{Text: tr.SystemWindowCommandsNeedADesktop()},
		}}
	}
	return Card{Title: tr.SystemWindow(), Description: tr.SystemTitleBarCommandsFrameAnd(), Actions: []core.Node{ui.Badge{Text: tr.SystemNsgo()}}, Children: []core.Node{
		Caption{Label: tr.SystemTitleBar(), Note: func() string {
			tb := desktop.TitleBar(cx)
			if tb == (core.Edges{}) {
				return tr.SystemTheSystemS()
			}
			return tr.SystemTitleBarInsets(float64(tb.Top), float64(tb.Left), float64(tb.Right))
		}},
		ui.Wrap{Children: []core.Node{
			small(tr.SystemHideTitleBar(), func() { desktop.SetTitleBar(cx, desktop.TitleBarHidden, ui.HeaderHeight) }),
			small(tr.SystemShowTitleBar(), func() { desktop.SetTitleBar(cx, desktop.TitleBarDefault, 0) }),
		}},
		Caption{Label: tr.SystemWindowControls()},
		ui.Wrap{Children: []core.Node{
			small(tr.SystemDefault(), func() { desktop.SetWindowControls(cx, desktop.WindowControls{}) }),
			small(tr.SystemTrafficLightsAt2410(), func() {
				desktop.SetWindowControls(cx, desktop.WindowControls{TrafficLights: core.Point{X: 24, Y: 10}})
			}),
			small(tr.SystemHidden(), func() { desktop.SetWindowControls(cx, desktop.WindowControls{Hidden: true}) }),
		}},
		Caption{Label: tr.SystemMaterial(), Note: func() string {
			if desktop.ShowsMaterial(cx) {
				return tr.SystemShowsBehindTheSidebar()
			}
			return "none"
		}},
		ui.Wrap{Children: []core.Node{
			small(tr.SystemNone(), func() { desktop.SetWindowMaterial(cx, desktop.WindowMaterialNone) }),
			small(tr.SystemSidebar(), func() { desktop.SetWindowMaterial(cx, desktop.WindowMaterialSidebar) }),
			small(tr.SystemUnderWindow(), func() { desktop.SetWindowMaterial(cx, desktop.WindowMaterialUnderWindow) }),
			small(tr.SystemHud(), func() { desktop.SetWindowMaterial(cx, desktop.WindowMaterialHUD) }),
		}},
		Caption{Label: tr.SystemCommands()},
		ui.Wrap{Children: []core.Node{small(tr.SystemMinimize(), cx.MinimizeWindow), small(tr.SystemZoom(), cx.ToggleMaximize), small(tr.SystemFullScreen(), cx.ToggleFullScreen),
			small(tr.SystemAwayFor2S(), func() { desktop.MinimizeWindow(cx); cx.AfterFunc(2*time.Second, func() { desktop.OrderFront(cx) }) })}},
		Caption{Label: tr.SystemFrameAndLevel()},
		ui.Wrap{Children: []core.Node{
			small("640 × 480", func() { desktop.SetWindowFrame(cx, desktop.WindowFrame{Width: 640, Height: 480}) }),
			small(tr.SystemTallerAnimated(), func() {
				desktop.SetWindowFrame(cx, desktop.WindowFrame{Height: env.Window().Height + 160, Animated: true})
			}),
			small(tr.SystemMoveTo8080(), func() { desktop.SetWindowFrame(cx, desktop.WindowFrame{X: 80, Y: 80, Move: true}) }),
		}},
		Caption{Label: tr.SystemConstraints()},
		ui.Wrap{Children: []core.Node{
			small(tr.SystemNoConstraints(), func() { desktop.SetWindowConstraints(cx, desktop.WindowConstraints{}) }),
			small("400 × 300 – 800 × 600", func() {
				desktop.SetWindowConstraints(cx, desktop.WindowConstraints{MinWidth: 400, MinHeight: 300, MaxWidth: 800, MaxHeight: 600})
			}),
			small(tr.SystemNoResize(), func() { desktop.SetWindowConstraints(cx, desktop.WindowConstraints{NoResize: true}) }),
			small(tr.SystemNoMinimize(), func() { desktop.SetWindowConstraints(cx, desktop.WindowConstraints{NoMinimize: true}) }),
			small(tr.SystemNoMaximize(), func() { desktop.SetWindowConstraints(cx, desktop.WindowConstraints{NoMaximize: true}) }),
		}},
		ui.Wrap{Children: []core.Node{
			small(tr.SystemNormal(), func() { desktop.SetWindowLevel(cx, desktop.WindowLevelNormal) }),
			small(tr.SystemFloating(), func() { desktop.SetWindowLevel(cx, desktop.WindowLevelFloating) }),
			small(tr.SystemDesktop(), func() { desktop.SetWindowLevel(cx, desktop.WindowLevelDesktop) }),
		}},
		core.View{Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOn}, StyleFunc: func() core.Style {
			return core.Style{Height: core.Pt(52), CornerRadius: ui.RadiusControl, Background: colors.Raised(), BorderWidth: 1, BorderColor: colors.Border(),
				Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifySpaceBetween, Padding: core.Symmetric(0, 14)}
		}, Children: []core.Node{
			ui.Text(tr.SystemDragTheWindowFromHere(), 13, ui.Regular, colors.Muted),
			core.View{Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOff}, Children: []core.Node{small(tr.SystemStillPressable(), func() {})}},
		}},
		ui.Button{Text: tr.SystemQuit(), Variant: ui.ButtonSecondary, Color: colors.Danger, OnPress: cx.Quit},
	}}
}

// clipboardCard is text, images and files on the clipboard.
func clipboardCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	clip := core.NewSignal(cx, tr.SystemHelloFromNsgo())
	status := core.NewSignal(cx, tr.SystemNothingReadYet())
	image := core.NewSignalFunc(cx, (*core.ClipboardImage)(nil), func(a, b *core.ClipboardImage) bool { return a == b })
	small := func(l string, fn func()) core.Node {
		return ui.Button{Text: l, Variant: ui.ButtonSecondary, Size: ui.SizeSmall, OnPress: fn}
	}
	return Card{Title: tr.SystemClipboard(), Description: tr.SystemTextImagesAndTheFiles(), Actions: []core.Node{ui.Badge{Text: tr.SystemNsgo()}}, Children: []core.Node{
		ui.Input{Value: clip.Get, OnChange: clip.Set, Label: tr.SystemClipboardText()},
		ui.Wrap{Children: []core.Node{
			ui.Button{Text: tr.SystemCopyText(), Size: ui.SizeSmall, OnPress: func() { cx.SetClipboard(clip.GetUntracked()) }},
			small(tr.SystemPasteText(), func() { cx.ReadClipboard(func(s string) { status.Set(tr.SystemText() + s) }) }),
			small(tr.SystemCopyLogo(), func() { cx.SetClipboardImage(logoPNG) }),
			small(tr.SystemPasteImage(), func() {
				cx.ReadClipboardImage(func(img core.ClipboardImage, ok bool, err error) {
					if err != nil {
						image.Set(nil)
						status.Set(err.Error())
						return
					}
					if !ok {
						image.Set(nil)
						status.Set(tr.SystemTheClipboardHoldsNoImage())
						return
					}
					image.Set(&img)
					status.Set(tr.SystemClipboardImage(float64(img.Size.Width), float64(img.Size.Height), len(img.PNG)))
				})
			}),
			small(tr.SystemPasteFiles(), func() {
				cx.ReadClipboardFiles(func(uris []string) {
					if len(uris) == 0 {
						status.Set(tr.SystemTheClipboardHoldsNoFiles())
						return
					}
					status.Set(tr.SystemFiles() + strings.Join(uris, ", "))
				})
			}),
			small(tr.SystemWhatIsThere(), func() {
				cx.HasClipboardText(func(t bool) {
					cx.HasClipboardImage(func(i bool) { status.Set(tr.SystemClipboardHolds(yesNo(tr, t), yesNo(tr, i))) })
				})
			}),
		}},
		core.Text{ContentFunc: status.Get, TestID: "clipboard-status", Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
		core.Show{When: func() bool { return image.Get() != nil }, Then: func(*core.Scope) core.Node {
			return core.Image{SourceFunc: func() core.ImageSource { return core.ImageSource{Data: image.Get().PNG} }, ResizeMode: core.Contain, Label: tr.SystemPastedImage(), TestID: "pasted-image",
				Style: imageFrame(cx, core.Style{Width: core.Pt(120), Height: core.Pt(120)}), StyleFunc: frameInk(cx)}
		}},
	}}
}

// servicesCard is opening links, the photo picker and the app's
// appearance.
func servicesCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	env := cx.Env()
	opened := core.NewSignal(cx, tr.SystemNothingOpenedYet())
	open := func(u string) func() {
		return func() {
			cx.OpenURL(u, func(ok bool) {
				if ok {
					opened.Set(tr.SystemOpened() + u)
				} else {
					opened.Set(tr.SystemNothingOpened() + u)
				}
			})
		}
	}
	picked := core.NewSignalFunc(cx, []core.PickedFile(nil), func(a, b []core.PickedFile) bool { return false })
	asked := core.NewSignal(cx, false)
	// failed says why the last photo was not taken, "" when none failed.
	failed := core.NewSignal(cx, "")
	pick := func(o core.PickOptions) func() {
		return func() {
			cx.PickImages(o, func(files []core.PickedFile) { asked.Set(true); failed.Set(""); picked.Set(files) })
		}
	}
	takePhoto := func() {
		cx.TakePhoto(core.PhotoOptions{MaxDimension: 2048}, func(photo *core.PickedFile, err error) {
			asked.Set(true)
			failed.Set("")
			picked.Set(nil)
			switch {
			case err != nil:
				failed.Set(err.Error())
			case photo != nil:
				picked.Set([]core.PickedFile{*photo})
			}
		})
	}
	small := func(l string, fn func()) core.Node {
		return ui.Button{Text: l, Variant: ui.ButtonSecondary, Size: ui.SizeSmall, OnPress: fn}
	}
	names := map[core.Appearance]string{core.AppearanceSystem: tr.SystemTheSystemS(), core.AppearanceLight: "light", core.AppearanceDark: "dark"}
	return Card{Title: tr.SystemLinksPhotosAndAppearance(), Description: tr.SystemOpenurlPickimagesSetappearance(), Actions: []core.Node{ui.Badge{Text: tr.SystemNsgo()}}, Children: []core.Node{
		Caption{Label: "OpenURL"},
		ui.Wrap{Children: []core.Node{small(tr.SystemTheWebsite(), open("https://github.com/nsgo-dev/nsgo")), small(tr.SystemAnEmail(), open("mailto:hello@example.com?subject=Hello%20from%20NSGo")),
			small(tr.SystemRefusedUrl(), open("javascript:alert(1)"))}},
		core.Text{ContentFunc: opened.Get, Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
		core.View{Style: core.Style{Height: core.Pt(1)}, StyleFunc: ui.Filled(colors.Line)},
		Caption{Label: "PickImages"},
		ui.Wrap{Children: []core.Node{small(tr.SystemPickAPhoto(), pick(core.PickOptions{})), small(tr.SystemUpTo4800Px(), pick(core.PickOptions{Multiple: true, Limit: 4, MaxDimension: 800})),
			ui.Button{Text: tr.SystemTakeAPhoto(), Variant: ui.ButtonSecondary, Size: ui.SizeSmall, OnPress: takePhoto, TestID: "take-photo",
				Disabled: func() bool { return !env.CanTakePhoto() }}}},
		core.Dynamic(func(cx *core.Scope) core.Node {
			tr := locales.For(cx)
			colors := ui.ColorsOf(cx)
			files := picked.Get()
			if len(files) == 0 {
				if why := failed.Get(); why != "" {
					return Note{Text: tr.SystemPhotoFailed() + why}
				}
				if asked.Get() {
					return Note{Text: tr.SystemNoPhotoPicked()}
				}
				return Note{Text: tr.SystemHeicArrivesAsJpegLarge()}
			}
			var tiles []core.Node
			for i, f := range files {
				tiles = append(tiles, core.View{Style: core.Style{Gap: 4, Width: core.Pt(100)}, Children: []core.Node{
					core.Image{Source: core.ImageSource{Data: f.Data}, ResizeMode: core.Cover, Label: f.Name, TestID: fmt.Sprintf("picked-%d", i),
						Style: imageFrame(cx, core.Style{Width: core.Pt(100), Height: core.Pt(76)}), StyleFunc: frameInk(cx)},
					core.Text{Content: f.Name, MaxLines: 1, Style: core.Style{FontSize: 12}, StyleFunc: ui.Colored(colors.Text)},
					ui.Text(fmt.Sprintf("%.0f × %.0f, %d KB", f.Size.Width, f.Size.Height, (len(f.Data)+512)/1024), 12, ui.Regular, colors.Muted),
				}})
			}
			return ui.Wrap{Children: tiles}
		}),
		core.View{Style: core.Style{Height: core.Pt(1)}, StyleFunc: ui.Filled(colors.Line)},
		Caption{Label: tr.SystemSetappearance(), Note: func() string { return tr.SystemForced() + names[env.Appearance()] }},
		ui.Wrap{Children: []core.Node{small(tr.SystemSystem(), func() { cx.SetAppearance(core.AppearanceSystem) }), small(tr.SystemLight(), func() { cx.SetAppearance(core.AppearanceLight) }),
			small(tr.SystemDark(), func() { cx.SetAppearance(core.AppearanceDark) })}},
	}}
}

// speechCard is dictation and whether the app is in use.
func speechCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	env := cx.Env()
	heard := core.NewSignal(cx, "")
	status := core.NewSignal(cx, tr.SystemSayAnything())
	var session *core.SpeechSession
	start := func() {
		heard.Set("")
		status.Set(tr.SystemListening())
		session, _ = cx.StartSpeech(core.SpeechOptions{Interim: true,
			OnResult: func(text string, _ bool) { heard.Set(text) },
			OnEnd: func(text string, err error) {
				heard.Set(text)
				reason := "—"
				if code, ok := err.(core.SpeechError); ok {
					reason = string(code)
				}
				status.Set(tr.SystemSpeechEnded(reason))
			}})
	}
	stop := func() {
		if session != nil {
			session.Stop()
		}
	}
	return Card{Title: tr.SystemSpeechAndLifecycle(), Description: tr.SystemStartspeechLifecycle(), Actions: []core.Node{ui.Badge{Text: tr.SystemNsgo()}}, Children: []core.Node{
		Caption{Label: tr.SystemLifecycle(), Note: func() string { return env.Lifecycle().String() }},
		core.View{Style: core.Style{Height: core.Pt(1)}, StyleFunc: ui.Filled(colors.Line)},
		Caption{Label: "StartSpeech"},
		core.Dynamic(func(cx *core.Scope) core.Node {
			if !env.CanRecognizeSpeech() {
				return Note{Text: locales.For(cx).SystemNoSpeechHere()}
			}
			return ui.Wrap{Children: []core.Node{
				ui.Button{Text: tr.SystemDictate(), Size: ui.SizeSmall, OnPress: start, TestID: "dictate"},
				ui.Button{Text: tr.SystemStopDictating(), Variant: ui.ButtonSecondary, Size: ui.SizeSmall, OnPress: stop, TestID: "stop-dictating"},
			}}
		}),
		core.Text{ContentFunc: heard.Get, TestID: "heard", Style: core.Style{FontSize: 15}, StyleFunc: ui.Colored(colors.Text)},
		core.Text{ContentFunc: status.Get, TestID: "speech-status", Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
	}}
}

// filesCard is the system's file picker and save dialog.
func filesCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	picked := core.NewSignalFunc(cx, []core.File(nil), func(a, b []core.File) bool { return false })
	status := core.NewSignal(cx, tr.SystemFilesArriveOnDiskCopied())
	pick := func(o core.PickFilesOptions) func() {
		return func() {
			cx.PickFiles(o, func(files []core.File, err error) {
				switch {
				case err != nil:
					status.Set(tr.SystemThePickFailed() + err.Error())
				case len(files) == 0:
					status.Set(tr.SystemNoFilePicked())
				default:
					status.Set(tr.SystemPicked(len(files)))
				}
				picked.Set(files)
			})
		}
	}
	// folders are the folders picked, each with what it holds once read.
	folders := core.NewSignalFunc(cx, []string(nil), func(a, b []string) bool { return false })
	pickFolders := func() {
		cx.PickDirectories(core.PickDirectoriesOptions{}, func(dirs []core.Directory, err error) {
			switch {
			case err != nil:
				status.Set(tr.SystemThePickFailed() + err.Error())
			case len(dirs) == 0:
				status.Set(tr.SystemNoFolderPicked())
			default:
				status.Set(tr.SystemPicked(len(dirs)))
			}
			rows := make([]string, len(dirs))
			for i, d := range dirs {
				rows[i] = d.Name
			}
			folders.Set(rows)
			// A folder without a path waits for the platform to list it,
			// so it is read off the engine's goroutine.
			for i, d := range dirs {
				go func() {
					entries, err := d.ReadDir(".")
					row := d.Name + " · " + tr.SystemFolderItems(len(entries))
					if err != nil {
						row = d.Name + " · " + err.Error()
					}
					cx.Post(func() {
						if rows := folders.Get(); i < len(rows) && rows[i] == d.Name {
							rows[i] = row
							folders.Set(rows)
						}
					})
				}()
			}
		})
	}
	save := func(o core.SaveOptions) {
		cx.SaveFile(o, func(saved bool, _ string, err error) {
			switch {
			case err != nil:
				status.Set(tr.SystemTheSaveFailed() + err.Error())
			case saved:
				status.Set(tr.SystemSaved2() + o.Name)
			default:
				status.Set(tr.SystemNothingSaved())
			}
		})
	}
	// note writes a text file to the cache, for another app to open or
	// receive.
	note := func() (core.SharedFile, bool) {
		dir := cx.CacheDir()
		if dir == "" {
			dir = os.TempDir()
		}
		path := filepath.Join(dir, "Hello from NSGo.txt")
		if err := os.WriteFile(path, []byte("Hello from NSGo\n"), 0o644); err != nil {
			status.Set(tr.SystemTheOpenFailed() + err.Error())
			return core.SharedFile{}, false
		}
		return core.SharedFile{Path: path}, true
	}
	open := func(f core.SharedFile) {
		cx.OpenFile(f, func(opened bool, err error) {
			switch {
			case err != nil:
				status.Set(tr.SystemTheOpenFailed() + err.Error())
			case opened:
				status.Set(tr.SystemOpenedInAnotherApp())
			default:
				status.Set(tr.SystemNoAppOpensIt())
			}
		})
	}
	var shareButton core.Ref
	share := func(files ...core.SharedFile) {
		o := core.ShareOptions{Files: files, Text: "Hello from NSGo", Anchor: &shareButton}
		cx.Share(o, func(shown bool, err error) {
			switch {
			case err != nil:
				status.Set(tr.SystemTheShareFailed() + err.Error())
			case shown:
				status.Set(tr.SystemShareSheetShown())
			}
		})
	}
	small := func(l string, fn func()) core.Node {
		return ui.Button{Text: l, Variant: ui.ButtonSecondary, Size: ui.SizeSmall, OnPress: fn}
	}
	return Card{Title: tr.SystemFiles2(), Description: tr.SystemPickfilesSavefile(), Actions: []core.Node{ui.Badge{Text: tr.SystemNsgo()}}, Children: []core.Node{
		Caption{Label: "PickFiles"},
		ui.Wrap{Children: []core.Node{small(tr.SystemPickAFile(), pick(core.PickFilesOptions{})),
			small(tr.SystemPdfsAndImages(), pick(core.PickFilesOptions{Multiple: true, Types: []string{"application/pdf", "image/*"}}))}},
		core.Dynamic(func(cx *core.Scope) core.Node {
			colors := ui.ColorsOf(cx)
			var rows []core.Node
			for i, f := range picked.Get() {
				rows = append(rows, core.Text{Content: fmt.Sprintf("%s · %s · %d KB", f.Name, f.MIME, (f.Size+512)/1024),
					TestID: fmt.Sprintf("picked-file-%d", i), MaxLines: 1, Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Text)})
			}
			return core.View{Style: core.Style{Gap: 4}, Children: rows}
		}),
		Caption{Label: "PickDirectories"},
		ui.Wrap{Children: []core.Node{small(tr.SystemPickAFolder(), pickFolders)}},
		core.Dynamic(func(cx *core.Scope) core.Node {
			colors := ui.ColorsOf(cx)
			var rows []core.Node
			for i, row := range folders.Get() {
				rows = append(rows, core.Text{Content: row,
					TestID: fmt.Sprintf("picked-folder-%d", i), MaxLines: 1, Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Text)})
			}
			return core.View{Style: core.Style{Gap: 4}, Children: rows}
		}),
		core.View{Style: core.Style{Height: core.Pt(1)}, StyleFunc: ui.Filled(colors.Line)},
		Caption{Label: "SaveFile"},
		ui.Wrap{Children: []core.Node{small(tr.SystemSaveATextFile(), func() {
			save(core.SaveOptions{Name: "Hello from NSGo.txt", Data: []byte("Hello from NSGo\n")})
		}), small(tr.SystemSaveTheFilePicked(), func() {
			if files := picked.Get(); len(files) > 0 && files[0].Path != "" {
				save(core.SaveOptions{Name: files[0].Name, Path: files[0].Path})
			} else {
				status.Set(tr.SystemPickAFileFirst())
			}
		})}},
		core.View{Style: core.Style{Height: core.Pt(1)}, StyleFunc: ui.Filled(colors.Line)},
		Caption{Label: "OpenFile · Share"},
		ui.Wrap{Children: []core.Node{small(tr.SystemOpenATextFile(), func() {
			if f, ok := note(); ok {
				open(f)
			}
		}), core.View{Ref: &shareButton, Children: []core.Node{small(tr.SystemShareATextFile(), func() {
			if f, ok := note(); ok {
				share(f)
			}
		})}}, small(tr.SystemShareTheFilePicked(), func() {
			if files := picked.Get(); len(files) > 0 && files[0].Path != "" {
				share(core.SharedFile{Path: files[0].Path, Name: files[0].Name, MIME: files[0].MIME})
			} else {
				status.Set(tr.SystemPickAFileFirst())
			}
		})}},
		core.Text{ContentFunc: status.Get, TestID: "files-status", Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
	}}
}

// systemDemo is what the platform does for the app: the environment, the
// status bar, alerts, frames and timers, shortcuts, the network, the
// window, the clipboard, links, photos, the appearance, dictation, the
// lifecycle, notifications, the badge, haptics and files.
func systemDemo(cx *core.Scope) core.Node {
	return demoPage(cx, pageProps{}, environmentCard, statusBarCard, dialogsCard, framesAndTimers, shortcuts, networkCard, windowCard, clipboardCard, servicesCard, speechCard, notificationsCard, biometricsCard, filesCard)
}

// yesNo says whether in tr's language.
func yesNo(tr locales.Messages, yes bool) string {
	if yes {
		return tr.CommonYes()
	}
	return tr.CommonNo()
}

// notificationsCard is system notifications, the app's badge and haptics.
func notificationsCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	permission := core.NewSignal(cx, "—")
	shown := core.NewSignal(cx, "")
	pressed := core.NewSignal(cx, tr.SystemNothingPressed())
	badge := core.NewSignal(cx, 0)
	answer := func(p core.NotificationPermission) { permission.Set(p.String()) }
	cx.NotificationPermission(answer)
	cx.OnNotificationPress(func(id string) { pressed.Set(tr.SystemNotificationPressed(id)) })
	show := func(silent bool) {
		_ = cx.ShowNotification(core.Notification{ID: "gallery", Title: tr.SystemNotificationTitle(), Body: tr.SystemNotificationBody(),
			Silent: silent}, func(ok bool) { shown.Set(tr.SystemNotificationShown(yesNo(tr, ok))) })
	}
	setBadge := func(n int) {
		badge.Set(n)
		cx.SetBadge(n)
	}
	small := func(l, id string, fn func()) core.Node {
		return ui.Button{Text: l, Variant: ui.ButtonSecondary, Size: ui.SizeSmall, OnPress: fn, TestID: id}
	}
	haptic := func(l string, h mobile.Haptic) core.Node {
		return small(l, "", func() { mobile.PlayHaptic(cx, h) })
	}
	line := func() core.Node {
		return core.View{Style: core.Style{Height: core.Pt(1)}, StyleFunc: ui.Filled(colors.Line)}
	}
	return Card{Title: tr.SystemNotificationsAndHaptics(), Description: tr.SystemNotificationsHapticsSummary(),
		Actions: []core.Node{ui.Badge{Text: tr.SystemNsgo()}}, Children: []core.Node{
			Caption{Label: tr.SystemPermission(), Note: permission.Get},
			ui.Wrap{Children: []core.Node{
				ui.Button{Text: tr.SystemAskPermission(), Size: ui.SizeSmall, TestID: "ask-notifications",
					OnPress: func() { cx.RequestNotificationPermission(answer) }},
				small(tr.SystemShowNotification(), "show-notification", func() { show(false) }),
				small(tr.SystemShowSilently(), "", func() { show(true) }),
				small(tr.SystemCloseNotification(), "close-notification", func() { cx.CloseNotification("gallery") }),
			}},
			core.Text{ContentFunc: shown.Get, TestID: "notification-shown", Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
			core.Text{ContentFunc: pressed.Get, TestID: "notification-pressed", Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
			line(),
			Caption{Label: tr.SystemBadge(), Note: func() string { return fmt.Sprint(badge.Get()) }},
			ui.Wrap{Children: []core.Node{
				small("+1", "badge-up", func() { setBadge(badge.GetUntracked() + 1) }),
				small(tr.SystemClearBadge(), "badge-clear", func() { setBadge(0) }),
			}},
			line(),
			Caption{Label: tr.SystemHaptics()},
			core.Dynamic(func(cx *core.Scope) core.Node {
				if !mobile.IsAvailable(cx) {
					return Note{Text: locales.For(cx).SystemNoHapticsHere()}
				}
				return ui.Wrap{Children: []core.Node{
					haptic(tr.SystemSelection(), mobile.HapticSelection), haptic(tr.SystemLightImpact(), mobile.HapticLight),
					haptic(tr.SystemHeavyImpact(), mobile.HapticHeavy), haptic(tr.SystemSuccess(), mobile.HapticSuccess),
					haptic(tr.SystemWarning(), mobile.HapticWarning), haptic(tr.SystemError(), mobile.HapticError),
				}}
			}),
		}}
}

// biometricsCard is the system's enrolled biometric and authentication prompt.
func biometricsCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	kind := core.NewSignal(cx, tr.SystemNone())
	result := core.NewSignal(cx, tr.SystemNoneYet())
	availability := func() {
		cx.BiometricAvailability(func(k core.BiometricKind) {
			switch k {
			case core.BiometricFace:
				kind.Set(tr.SystemFaceId())
			case core.BiometricFingerprint:
				kind.Set(tr.SystemFingerprint())
			default:
				kind.Set(tr.SystemNone())
			}
		})
	}
	return Card{Title: tr.SystemConfirm(), Description: tr.SystemCancelAndDestructiveButtons(), Actions: []core.Node{ui.Badge{Text: tr.SystemNsgo()}}, Children: []core.Node{
		Caption{Label: tr.SystemBiometric(), Note: kind.Get},
		ui.Wrap{Children: []core.Node{
			ui.Button{Text: tr.SystemPreview(), Size: ui.SizeSmall, TestID: "biometric-availability", OnPress: availability},
			ui.Button{Text: tr.SystemAuthenticate(), Variant: ui.ButtonSecondary, Size: ui.SizeSmall, TestID: "biometric-authenticate", OnPress: func() {
				cx.AuthenticateBiometric(tr.SystemConfirm(), func(r core.BiometricResult) {
					switch r {
					case core.BiometricSuccess:
						result.Set(tr.SystemSuccess())
					case core.BiometricCancelled:
						result.Set(tr.SystemCancel())
					case core.BiometricFailure:
						result.Set(tr.SystemDelete())
					default:
						result.Set(tr.SystemNone())
					}
				})
			}},
		}},
		core.Text{ContentFunc: result.Get, TestID: "biometric-result", Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
	}}
}

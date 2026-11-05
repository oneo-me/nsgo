package main

import (
	"fmt"
	"github.com/oneo-me/nsgo/icons/lucide"
	"math"
	"math/rand/v2"
	"slices"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/mobile"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
	"github.com/oneo-me/nsgo/examples/gallery/locales"
)

// hairline is one device pixel, in points.
func hairline(env *core.Env) core.Points {
	r := env.PixelRatio()
	if r == 0 {
		r = 1
	}
	return core.Points(1 / r)
}

var tileTones = []ui.Tone{ui.ToneAccent, ui.ToneInfo, ui.ToneWarn, ui.ToneUp, ui.ToneDanger}

func layoutAnimations(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	items := core.NewSignalFunc(cx, []int{1, 2, 3}, slices.Equal)
	preset := core.NewSignal(cx, "spring")
	next := 4
	presets := map[string]core.LayoutAnimation{"spring": core.LayoutSpringPreset, "ease": core.LayoutEaseInEaseOutPreset, "linear": core.LayoutLinearPreset}
	animate := func(change func()) {
		cx.AnimateLayout(presets[preset.GetUntracked()])
		change()
	}
	return ui.Card{Title: tr.CompLayoutAnimation(), Description: tr.CompAnimatelayoutThenChangeTheTree(), Actions: []core.Node{ui.Badge{TextFunc: func() string { return itoa(len(items.Get())) }}}, Children: []core.Node{
		ui.Segmented[string]{Options: []ui.Option[string]{{Value: "spring", Label: tr.CompSpring()}, {Value: "ease", Label: tr.CompEase()}, {Value: "linear", Label: tr.CompLinear()}}, Value: preset.Get, OnChange: preset.Set, Small: true},
		core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 8, MinHeight: core.Pt(56)}, Children: []core.Node{
			core.Each(items.Get, func(n int) int { return n }, func(cx *core.Scope, n func() int, _ func() int) core.Node {
				t := tileTones[core.Untracked(cx, n)%len(tileTones)]
				return core.View{Style: core.Style{Width: core.Pt(56), Height: core.Pt(56), CornerRadius: 14, BorderCurve: core.BorderCurveContinuous,
					AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, StyleFunc: func() core.Style {
					highlight := ui.Inner(0, 1, 0, core.RGB(0xffffff))
					if cx.Dark() {
						highlight = ui.Inner(0, 1, 0, ui.White(0.06))
					}
					return core.Style{Background: t.Tint(cx), Shadows: []core.BoxShadow{highlight}}
				},
					Children: []core.Node{core.Text{ContentFunc: func() string { return itoa(n()) }, Style: core.Style{FontSize: 15, FontWeight: ui.Semibold}, StyleFunc: ui.Colored(t.Shade(cx))}}}
			}),
		}},
		core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
			ui.Cell{Child: ui.Button{Text: tr.CompAdd(), Icon: lucide.Plus, OnPress: func() {
				animate(func() { items.Update(func(s []int) []int { return append([]int{next}, s...) }); next++ })
			}}},
			ui.Cell{Child: ui.Button{Text: tr.CompShuffle(), Variant: ui.ButtonSecondary, OnPress: func() {
				animate(func() {
					items.Update(func(s []int) []int {
						s = slices.Clone(s)
						rand.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
						return s
					})
				})
			}}},
			ui.Cell{Child: ui.Button{Text: tr.CompRemove(), Variant: ui.ButtonSecondary, Disabled: func() bool { return len(items.Get()) == 0 }, OnPress: func() {
				animate(func() { items.Update(func(s []int) []int { return slices.Clone(s[1:]) }) })
			}}},
		}},
	}}
}

func pressableSheet(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	open := core.NewSignal(cx, false)
	env := cx.Env()
	return ui.Card{Title: tr.CompSheet(), Description: tr.CompAPressedStateStyleOpens(), Children: []core.Node{
		core.View{OnPress: func(core.PressEvent) { open.Set(true) }, Role: core.RoleButton, Label: tr.CompOpenASheet(),
			StyleFunc: func() core.Style {
				pressedFill := core.RGB(0xb84a15)
				if cx.Dark() {
					pressedFill = core.RGB(0xd8825a)
				}
				return core.Style{Shadows: fin.Accent().Shadows, Height: core.Pt(48), Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
					Gap: 8, CornerRadius: 12, Background: colors.AccentFill(), Transition: core.Transition{Duration: 150e6},
					Active: &core.Style{Transform: core.Transform{Scale: 0.97}, Background: pressedFill}}
			},
			Children: []core.Node{ui.Text(tr.CompOpenASheet(), 13, ui.Semibold, colors.OnAccent)}},
		ui.Caption{Label: tr.CompScalesAndDarkensWhilePressed(), Note: func() string { return "Active: scale 0.97" }},
		base.Sheet{Open: open.Get, OnOpenChange: func(o bool, _ base.Reason) { open.Set(o) }, Label: tr.CompASheet(),
			BackdropStyle: core.Style{Background: ui.Black(0.5)},
			PopupStyleFunc: func() core.Style {
				return core.Style{Gap: 16, Padding: core.Edges{Top: 20, Right: 20, Bottom: env.SafeArea().Bottom + 20, Left: 20},
					CornerRadii: core.Corners{TopLeft: 24, TopRight: 24}, Background: colors.Background()}
			},
			Children: []core.Node{
				core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
					core.View{Style: core.Style{Direction: core.Row}, Children: []core.Node{ui.Badge{Text: tr.CompSheet(), Tone: ui.ToneAccent}}},
					core.Text{Content: tr.CompASheet(), Style: core.Style{FontSize: 28, FontWeight: ui.Bold, LetterSpacing: -0.5}, StyleFunc: ui.Colored(colors.Text)},
					core.Text{Content: ui.Hint(cx, tr.CompTapOutsideItOrPress(), tr.CompPressEscapeOrClickOutside()),
						Style: core.Style{FontSize: 13, LineHeight: 21}, StyleFunc: ui.Colored(colors.Muted)},
				}},
				ui.Button{Text: tr.CompClose(), OnPress: func() { open.Set(false) }},
			}},
	}}
}

// pressTile is a tile to press, with the style it shows.
func pressTile(cx *core.Scope, title func() string, noteText string, right core.Node) core.Node {
	colors := ui.ColorsOf(cx)
	children := []core.Node{core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: []core.Node{
		ui.Textf(title, 13, ui.Regular, colors.Text),
		ui.API{Text: noteText},
	}}}
	if right != nil {
		children = append(children, right)
	}
	return core.View{Style: core.Style{MinHeight: core.Pt(60), Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Padding: core.Symmetric(8, 12)}, Children: children}
}

// tileStyle is the raised highlight without its gradient, so an underlay
// color shows. Read it in a StyleFunc.
func tileStyle(cx *core.Scope) core.Style {
	colors := ui.ColorsOf(cx)
	return core.Style{CornerRadius: ui.RadiusControl, BorderCurve: core.BorderCurveContinuous, BorderWidth: 1, BorderColor: colors.Border(),
		Background: colors.Surface(), Shadows: ui.FinOf(cx).Raised().Shadows}
}

// pressFeedback is press feedback drawn by the platform: opacity, an
// underlay; Disabled takes no presses.
func pressFeedback(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	tints := ui.TintsOf(cx)
	last := core.NewSignal(cx, tr.CompNoneYet())
	disabled := core.NewSignal(cx, false)
	fade := func() core.Style { s := tileStyle(cx); s.Active = &core.Style{Opacity: core.Alpha(0.4)}; return s }
	under := func() core.Style { s := tileStyle(cx); s.Active = &core.Style{Background: colors.Raised()}; return s }
	third := func() core.Style {
		s := tileStyle(cx)
		s.Active, s.Disabled = &core.Style{Background: tints.Danger()}, &core.Style{Opacity: core.Alpha(0.4)}
		return s
	}
	return ui.Card{Title: tr.CompPressFeedback(), Description: ui.Hint(cx, tr.CompDrawnNativelyWhileTheFinger(), tr.CompDrawnNativelyWhileTheButton()), Children: []core.Node{
		core.View{StyleFunc: fade, Role: core.RoleButton, OnPress: func(core.PressEvent) { last.Set(tr.CompFadingTile()) },
			Children: []core.Node{pressTile(cx, label(tr.CompFadesWhilePressed()), "Active: Opacity 0.4", nil)}},
		core.View{StyleFunc: under, Role: core.RoleButton, OnPress: func(core.PressEvent) { last.Set(tr.CompUnderlayTile()) },
			Children: []core.Node{pressTile(cx, label(tr.CompLightsUpWhilePressed()), "Active: Background", nil)}},
		core.View{StyleFunc: third, Role: core.RoleButton, Disabled: disabled.Get, OnPress: func(core.PressEvent) { last.Set(tr.CompEnabledTile()) },
			Children: []core.Node{pressTile(cx, func() string {
				if disabled.Get() {
					return tr.CompTakesNoPresses()
				}
				return tr.CompEnabled()
			}, "Disabled + Disabled: Opacity 0.4", core.Dynamic(func(*core.Scope) core.Node {
				if disabled.Get() {
					return ui.Badge{Text: tr.CompOff()}
				}
				return ui.Badge{Text: tr.CompOn(), Tone: ui.ToneUp}
			}))}},
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12}, Children: []core.Node{
			core.View{Style: fill, Children: []core.Node{ui.Caption{Label: tr.CompLastPressed(), Note: last.Get}}},
			ui.Switch{Checked: disabled.Get, OnChange: disabled.Set, Label: tr.CompDisableTheThirdTile()},
		}},
	}}
}

func accessory(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	value := core.NewSignal(cx, "")
	input := &core.Ref{}
	env := cx.Env()
	captionText := tr.CompTheBarCountsWhatYou()
	switch {
	case env.Desktop():
		captionText = tr.CompDesktopsHaveNoSoftwareKeyboard()
	}
	return ui.Card{Title: tr.CompInputAccessory(), Description: tr.CompABarOverTheSoftware(), Actions: []core.Node{ui.Badge{Text: "iOS · Android"}}, Children: []core.Node{
		ui.Input{Input: base.Input{Ref: input, Value: value.Get, OnChange: value.Set, Placeholder: tr.CompFocusMe(), InputAccessoryViewID: "counter", TestID: "accessory-field"}},
		ui.Caption{Label: captionText, Note: func() string { return `InputAccessoryViewID: "counter"` }},
		mobile.InputAccessoryView{ID: "counter", TestID: "accessory-bar", StyleFunc: ui.Filled(colors.Surface), Children: []core.Node{
			core.View{StyleFunc: func() core.Style {
				return core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Padding: core.Symmetric(8, 16),
					Borders: core.Edges{Top: hairline(env)}, BorderColors: core.EdgeColors{Top: colors.Border()}}
			}, Children: []core.Node{
				core.Text{ContentFunc: func() string { return tr.CompCharacters(len([]rune(value.Get()))) }, Style: core.Style{FlexGrow: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
				ui.Button{Text: tr.CompClear(), Variant: ui.ButtonSecondary, OnPress: func() { value.Set("") }},
				ui.Button{Text: tr.CompDone(), OnPress: input.Blur},
			}},
		}},
	}}
}

// avoidingPage is a screen whose field at the bottom stays above the
// keyboard.
func avoidingPage(cx *core.Scope) base.NavPage {
	tr := locales.For(cx)
	margin := core.NewSignal(cx, false)
	return screen(cx, tr.CompKeyboardInset(), nil, core.Dynamic(func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		tints := ui.TintsOf(cx)
		m := margin.Get()
		style := core.Style{FlexGrow: 1, PaddingEnv: core.EnvEdges{Bottom: core.EnvKeyboard}}
		title, note := "padding", tr.CompPaddingenvEnvkeyboardTheTintedBackground()
		if m {
			style.PaddingEnv, style.MarginEnv = core.EnvEdges{}, core.EnvEdges{Bottom: core.EnvKeyboard}
			title, note = "margin", tr.CompMarginenvEnvkeyboardTheViewShrinks()
		}
		return core.View{Style: style, StyleFunc: ui.Filled(tints.Info), Children: []core.Node{
			core.View{Style: core.Style{FlexGrow: 1, Padding: core.All(16), Gap: 16}, Children: []core.Node{
				ui.Card{Title: tr.CompRoom() + title, Description: tr.CompFocusTheFieldAtThe(), Children: []core.Node{
					ui.Segmented[bool]{Options: []ui.Option[bool]{{Value: false, Label: "padding"}, {Value: true, Label: "margin"}}, Value: margin.Get, OnChange: margin.Set},
					core.Text{Content: note, Style: core.Style{FontSize: 13, LineHeight: 20}, StyleFunc: ui.Colored(colors.Muted)},
				}},
				ui.Spacer{},
				ui.Input{Input: base.Input{Placeholder: tr.CompTypeHere()}},
			}},
		}}
	}))
}

func modalOptions(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	visible, translucent := core.NewSignal(cx, false), core.NewSignal(cx, false)
	bg := func() core.Color {
		if cx.Dark() {
			return core.RGB(0x2a1d15)
		}
		return core.RGB(0xf8ece3)
	}
	return ui.Card{Title: tr.CompDialogOptions(), Description: tr.CompStatusbartranslucentOrientations(), Actions: []core.Node{ui.Switch{Checked: translucent.Get, OnChange: translucent.Set, Label: "StatusBarTranslucent"}}, Children: []core.Node{
		core.Text{Content: tr.CompAndroidTranslucentContentReachesUnder(),
			Style: core.Style{FontSize: 13, LineHeight: 20}, StyleFunc: ui.Colored(colors.Muted)},
		ui.Caption{Label: "StatusBarTranslucent", Note: func() string { return fmt.Sprint(translucent.Get()) }},
		ui.Button{Text: tr.CompOpenTheDialog(), Variant: ui.ButtonSecondary, OnPress: func() { visible.Set(true) }},
		core.Dynamic(func(*core.Scope) core.Node {
			t := translucent.Get()
			return base.Dialog{Open: visible.Get, Animation: core.ModalSlide, Label: tr.CompADialog(),
				Android:      core.AndroidModalProps{StatusBarTranslucent: t},
				IOS:          core.IOSModalProps{SupportedOrientations: core.OrientationPortrait | core.OrientationLandscape},
				OnOpenChange: func(o bool, _ base.Reason) { visible.Set(o) },
				PopupStyle:   core.Style{FlexGrow: 1, AlignSelf: core.AlignStretch},
				Children: []core.Node{core.View{Style: core.Style{FlexGrow: 1}, StyleFunc: ui.Filled(bg), Children: []core.Node{
					core.View{Style: core.Style{PaddingEnv: core.EnvAll(core.EnvSafeArea), FlexGrow: 1, Padding: core.All(16), Gap: 12}, Children: []core.Node{
						core.View{Style: core.Style{Direction: core.Row, Padding: core.Edges{Top: 8}}, Children: []core.Node{
							ui.Badge{Text: map[bool]string{true: "translucent", false: tr.CompOpaqueBar()}[t], Tone: ui.ToneAccent}}},
						core.Text{Content: tr.CompADialog(), Style: core.Style{FontSize: 28, FontWeight: ui.Bold, LetterSpacing: -0.5}, StyleFunc: ui.Colored(colors.Text)},
						core.Text{Content: map[bool]string{true: tr.CompOliveUnder(), false: tr.CompOliveBelow()}[t],
							Style: core.Style{FontSize: 13, LineHeight: 21}, StyleFunc: ui.Colored(colors.Muted)},
						ui.Spacer{},
						ui.Button{Text: tr.CompClose(), OnPress: func() { visible.Set(false) }},
					}},
				}}}}
		}),
	}}
}

func platformInfo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	env := cx.Env()
	rows := func() [][2]string {
		r := env.PixelRatio()
		if r == 0 {
			r = 1
		}
		return [][2]string{
			{"Env.OS", osName(env.OS())},
			{"Env.OSVersion", env.OSVersion().String()},
			{"Env.Idiom", idiomName(env.Idiom())},
			{"Env.PixelRatio", fmt.Sprint(r)},
			{tr.Comp103OnWholePixels(), fmt.Sprint(math.Round(10.3*float64(r)) / float64(r))},
			{"Env.ScreenReader", fmt.Sprint(env.ScreenReader())},
			{"Env.ReduceMotion", fmt.Sprint(env.ReduceMotion())},
			{"Env.Locale", env.Locale()},
		}
	}
	announced := 0
	target := &core.Ref{}
	return ui.Card{Title: tr.CompPlatform(), Description: tr.CompTheEnvironmentPixelratioAnnounce(), Actions: []core.Node{ui.Badge{Text: osName(env.OS()), Tone: ui.ToneAccent}}, Children: []core.Node{
		core.Dynamic(func(*core.Scope) core.Node {
			var nodes []core.Node
			for i, r := range rows() {
				nodes = append(nodes, ui.Row{First: i == 0, Children: []core.Node{
					core.Text{Content: r[0], MaxLines: 1, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 12, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Muted)},
					core.Text{Content: r[1], MaxLines: 1, Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Text)},
				}})
			}
			return core.View{Children: nodes}
		}),
		core.View{StyleFunc: func() core.Style { return core.Style{Height: core.Pt(hairline(env)), Background: colors.Accent()} }},
		ui.Caption{Label: tr.CompTheLineAboveIsOne(), Note: func() string { return fmt.Sprintf("%.3f pt", hairline(env)) }},
		core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
			ui.Cell{Child: ui.Button{Text: tr.CompAnnounce(), OnPress: func() {
				announced++
				cx.Announce(tr.CompAnnouncement(announced))
			}}},
			ui.Cell{Child: ui.Button{Text: tr.CompFocusTheNote(), Variant: ui.ButtonSecondary, OnPress: target.SetAccessibilityFocus}},
		}},
		core.Text{Ref: target, Content: tr.CompWithVoiceoverOrTalkbackOn(),
			Style: core.Style{FontSize: 13, LineHeight: 18}, StyleFunc: ui.Colored(colors.Faint)},
	}}
}

// idiomName is a device idiom in words.
func idiomName(i core.Idiom) string {
	switch i {
	case core.IdiomPhone:
		return "phone"
	case core.IdiomTablet:
		return "tablet"
	case core.IdiomDesktop:
		return "desktop"
	}
	return "unknown"
}

// componentsDemo is press feedback, a sheet, layout animation, an input
// accessory, keyboard avoidance, a dialog's options and the platform.
func componentsDemo(cx *core.Scope) core.Node {
	return demoPage(cx, pageProps{behavior: core.ScrollBehavior{KeyboardDismissMode: core.KeyboardDismissInteractive}},
		pressFeedback, pressableSheet, layoutAnimations, accessory,
		func(cx *core.Scope) core.Node {
			tr := locales.For(cx)
			return ui.Card{Title: tr.CompKeyboardAvoidance(), Description: tr.CompContentOutsideScrollViews(), Flush: true, Children: []core.Node{
				ui.LinkRow{Icon: lucide.TextCursorInput, Title: tr.CompKeyboardInset(), Note: "EnvKeyboard", OnPress: func() { shell.push(avoidingPage) }},
			}}
		},
		modalOptions, platformInfo)
}

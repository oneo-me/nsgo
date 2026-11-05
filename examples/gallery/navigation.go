package main

import (
	"fmt"
	"github.com/oneo-me/nsgo/icons/lucide"
	"strings"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
	"github.com/oneo-me/nsgo/examples/gallery/locales"
)

// insetsRow is the safe-area insets where it renders, one figure per
// edge.
func insetsRow(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	env := cx.Env()
	edge := func(f func(core.Edges) core.Points) func() string {
		return func() string { return fmt.Sprintf("%d pt", int(f(core.Insets(cx))+0.5)) }
	}
	_ = env
	return core.View{Style: core.Style{Direction: core.Row}, Children: []core.Node{
		ui.Stat{Label: tr.NavTop(), Value: edge(func(e core.Edges) core.Points { return e.Top }), Width: core.Pct(25)},
		ui.Stat{Label: tr.NavBottom(), Value: edge(func(e core.Edges) core.Points { return e.Bottom }), Width: core.Pct(25)},
		ui.Stat{Label: tr.NavLeft(), Value: edge(func(e core.Edges) core.Points { return e.Left }), Width: core.Pct(25)},
		ui.Stat{Label: tr.NavRight(), Value: edge(func(e core.Edges) core.Points { return e.Right }), Width: core.Pct(25)},
	}}
}

// navPage is the body of a pushed screen: what it shows, then where its
// safe area is.
func navPage(cx *core.Scope, intro string, children ...core.Node) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	return core.Scroll{Style: core.Style{FlexGrow: 1}, StyleFunc: ui.Filled(colors.Background), Children: []core.Node{
		core.View{Style: core.Style{Padding: core.Edges{Top: 16, Right: 16, Bottom: 40, Left: 16}, Gap: 16}, Children: []core.Node{
			ui.Appear{Children: []core.Node{core.Text{Content: intro, Style: core.Style{FontSize: 13, LineHeight: 21, Padding: core.Symmetric(0, 4)}, StyleFunc: ui.Colored(colors.Muted)}}},
			ui.Appear{Index: 1, Children: []core.Node{ui.Section{Title: tr.NavSafeArea(), Note: tr.NavInsets(), Children: []core.Node{insetsRow(cx)}}}},
			ui.Appear{Index: 2, Children: []core.Node{core.View{Style: core.Style{Gap: 16}, Children: children}}},
		}},
	}}
}

func largeTitlePage(cx *core.Scope) base.NavPage {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	var rows []core.Node
	for i := 1; i <= 20; i++ {
		s := core.Style{Height: core.Pt(48), Direction: core.Row, AlignItems: core.AlignCenter, Padding: core.Symmetric(0, 10)}
		if i > 1 {
			s.Borders = core.Edges{Top: 1}
		}
		rows = append(rows, core.View{Style: s, StyleFunc: func() core.Style { return core.Style{BorderColors: core.EdgeColors{Top: colors.Line()}} }, Children: []core.Node{
			core.Text{Content: tr.NavRow(i), Style: core.Style{FlexGrow: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Text)},
			ui.Text(fmt.Sprintf("%d pt", i*48), 13, ui.Regular, colors.Faint),
		}})
	}
	s := screen(cx, tr.NavLargeTitle(), nil, navPage(cx, tr.NavIosTheTitleShrinksInto(),
		ui.Card{Flush: true, Children: rows}))
	s.HeaderLargeTitle = true
	return s
}

func transparentPage(cx *core.Scope) base.NavPage {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	tints := ui.TintsOf(cx)
	bg := func() core.Color {
		if cx.Dark() {
			return core.RGB(0x21160f)
		}
		return core.RGB(0xf8ece3)
	}
	return base.NavPage{Title: tr.NavTransparent(), HeaderTransparent: true, HeaderStyleFunc: func() core.Style { return core.Style{TintColor: colors.Text(), Color: colors.Text()} },
		Header: pageHeader(cx, pageHeaderProps{title: tr.NavTransparent(), transparent: true, tint: colors.Text}),
		Children: []core.Node{core.View{Style: core.Style{FlexGrow: 1, Overflow: core.Hidden, Padding: core.Edges{Top: 16, Right: 16, Bottom: 56, Left: 16},
			JustifyContent: core.JustifyEnd, Gap: 16}, StyleFunc: ui.Filled(bg), Children: []core.Node{
			core.View{StyleFunc: func() core.Style {
				return core.Style{Position: core.Absolute, Top: core.Pt(-90), Right: core.Pt(-70), Width: core.Pt(320), Height: core.Pt(320), CornerRadius: 160, Background: tints.Accent()}
			}},
			core.View{StyleFunc: func() core.Style {
				return core.Style{Position: core.Absolute, Top: core.Pt(150), Left: core.Pt(-110), Width: core.Pt(260), Height: core.Pt(260), CornerRadius: 130, Background: tints.Info()}
			}},
			ui.Appear{Children: []core.Node{core.View{Style: core.Style{Gap: 12}, Children: []core.Node{
				core.View{Style: core.Style{Direction: core.Row}, Children: []core.Node{ui.Badge{Text: "HeaderTransparent", Tone: ui.ToneAccent}}},
				core.Text{Content: tr.NavContentUnderTheHeader(), Style: core.Style{FontSize: 30, FontWeight: ui.Bold, LetterSpacing: -0.5}, StyleFunc: ui.Colored(colors.Text)},
				core.Text{Content: tr.NavTheCirclesStartAtThe(), Style: core.Style{FontSize: 13, LineHeight: 21}, StyleFunc: ui.Colored(colors.Muted)},
			}}}},
			ui.Appear{Index: 1, Children: []core.Node{ui.Inset{Children: []core.Node{insetsRow(cx)}}}},
		}}}}
}

func customHeaderPage(cx *core.Scope) base.NavPage {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	likes := core.NewSignal(cx, 0)
	// Each call makes the elements anew: the system header (phones) and
	// the app's (desktops, the web) each mount their own.
	left := func() core.Node {
		return core.Text{Content: tr.NavClose(), OnPress: func(core.PressEvent) { shell.nav.Pop(1) }, Role: core.RoleButton,
			Style: core.Style{FontSize: 15, FontWeight: ui.Medium}, StyleFunc: ui.Colored(colors.OnAccent)}
	}
	center := func() core.Node {
		return ui.HStack{Gap: 7, Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pt(10), Height: core.Pt(10), CornerRadius: 5}, StyleFunc: ui.Filled(colors.OnAccent)},
			ui.Text(tr.NavCustomTitle(), 15, ui.Bold, colors.OnAccent),
		}}
	}
	right := func() core.Node {
		return core.View{OnPress: func(core.PressEvent) { likes.Update(func(n int) int { return n + 1 }) }, Role: core.RoleButton, Label: tr.NavStar(), HitSlop: core.All(10),
			Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 5}, Children: []core.Node{
				ui.Icon(lucide.Star, 18, colors.OnAccent),
				ui.Textf(func() string { return itoa(likes.Get()) }, 15, ui.Semibold, colors.OnAccent),
			}}
	}
	return base.NavPage{Title: tr.NavCustom(), HeaderStyleFunc: func() core.Style {
		return core.Style{TintColor: colors.OnAccent(), Background: colors.AccentFill(), Color: colors.OnAccent()}
	},
		HeaderBackHidden: true, HeaderLeft: left(), HeaderTitle: center(), HeaderRight: right(),
		Header: pageHeader(cx, pageHeaderProps{title: tr.NavCustom(), background: colors.AccentFill, left: left(), center: center(), right: right()}),
		Children: []core.Node{navPage(cx, tr.NavTheHeaderSItemsAre(),
			ui.Section{Title: tr.NavHeader(), Note: "HeaderRight", Children: []core.Node{ui.Figure{Value: func() string { return itoa(likes.Get()) }, Note: func() string {
				if likes.Get() == 1 {
					return "star"
				}
				return "stars"
			}}}})}}
}

func lockedPage(cx *core.Scope) base.NavPage {
	tr := locales.For(cx)
	s := screen(cx, tr.NavNoSwipeBack(), nil, navPage(cx, ui.Hint(cx,
		tr.NavTheBackGestureIsOff(),
		tr.NavTheBackGestureIsOff2()),
		ui.Section{Title: tr.NavLeaving(), Note: "NoPopOnSwipe", Children: []core.Node{core.View{Style: core.Style{Direction: core.Row}, Children: []core.Node{
			ui.Stat{Label: tr.NavSwipeBack(), Value: label(tr.NavOff()), Tone: ui.ToneDanger, Width: core.Pct(50)},
			ui.Stat{Label: tr.NavBackButtons(), Value: label(tr.NavOn()), Tone: ui.ToneUp, Width: core.Pct(50)},
		}}}}))
	s.NoPopOnSwipe = true
	return s
}

// unsavedPage asks before leaving with unsaved text.
func unsavedPage(cx *core.Scope) base.NavPage {
	tr := locales.For(cx)
	draft := core.NewSignal(cx, "")
	confirm := func() {
		base.ShowAlertDialog(cx, base.AlertDialog{Title: tr.NavDiscardYourChanges(), Actions: []core.AlertAction{
			{Label: tr.NavKeepEditing(), Role: core.AlertCancel},
			{Label: tr.NavDiscard(), Role: core.AlertDestructive},
		}}, func(label string) {
			if label == tr.NavDiscard() {
				shell.nav.Pop(1)
			}
		})
	}
	back := func() {
		if draft.GetUntracked() != "" {
			confirm()
		} else {
			shell.nav.Pop(1)
		}
	}
	s := base.NavPage{Title: tr.NavUnsavedChanges(), HeaderStyleFunc: headerStyle(cx), NoPopOnBack: true, OnBackPress: back,
		Header: pageHeader(cx, pageHeaderProps{title: tr.NavUnsavedChanges(), onBack: back}),
		Children: []core.Node{navPage(cx, ui.Hint(cx,
			tr.NavTypeSomethingThenGoBack(),
			tr.NavTypeSomethingThenGoBack2()),
			ui.Card{Title: tr.NavDraft(), Description: "NoPopOnBack + OnBackPress", Actions: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
				if draft.Get() != "" {
					return ui.Badge{Text: tr.NavGuarded(), Tone: ui.ToneWarn}
				}
				return ui.Badge{Text: tr.NavEmpty()}
			})}, Children: []core.Node{
				ui.Input{Input: base.Input{Value: draft.Get, OnChange: draft.Set, Placeholder: tr.NavADraft(), TestID: "draft"}},
				ui.Note{TextFunc: func() string {
					if n := len([]rune(draft.Get())); n > 0 {
						return tr.NavCharactersLost(n)
					}
					return tr.NavNothingToLoseBackPops()
				}},
			}})}}
	return s
}

func headerlessPage(cx *core.Scope) base.NavPage {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	return base.NavPage{HeaderHidden: true, Children: []core.Node{core.View{Style: core.Style{PaddingEnv: core.EnvAll(core.EnvSafeArea), FlexGrow: 1}, StyleFunc: ui.Filled(colors.Background), Children: []core.Node{
		core.View{Style: core.Style{FlexGrow: 1, Padding: core.All(16), Gap: 16}, Children: []core.Node{
			ui.Appear{Children: []core.Node{core.View{Style: core.Style{Gap: 8, Padding: core.Edges{Top: 12, Right: 4, Left: 4}}, Children: []core.Node{
				core.View{Style: core.Style{Direction: core.Row}, Children: []core.Node{ui.Badge{Text: "HeaderHidden", Tone: ui.ToneAccent}}},
				core.Text{Content: tr.NavNoHeader(), Style: core.Style{FontSize: 30, FontWeight: ui.Bold, LetterSpacing: -0.5}, StyleFunc: ui.Colored(colors.Text)},
				core.Text{Content: tr.NavTheScreenOwnsTheWhole(),
					Style: core.Style{FontSize: 13, LineHeight: 21}, StyleFunc: ui.Colored(colors.Muted)},
			}}}},
			ui.Appear{Index: 1, Children: []core.Node{ui.Section{Title: tr.NavSafeArea(), Note: "PaddingEnv: EnvSafeArea", Children: []core.Node{insetsRow(cx)}}}},
			ui.Spacer{},
			ui.Button{Text: tr.NavBack(), OnPress: func() { shell.nav.Pop(1) }},
		}},
	}}}}
}

func deepPage(depth int) func(cx *core.Scope) base.NavPage {
	return func(cx *core.Scope) base.NavPage {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		fin := ui.FinOf(cx)
		var levels []core.Node
		for level := 1; level <= depth; level++ {
			on := level == depth
			fg := colors.Muted
			if on {
				fg = colors.OnAccent
			}
			face := func() core.Style {
				s := core.Style{Width: core.Pt(32), Height: core.Pt(32), CornerRadius: 8, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}
				if on {
					s = fin.Accent().Over(s)
					s.Background = colors.AccentFill()
				} else {
					s = fin.Raised().Over(s)
					s.BorderWidth, s.BorderColor, s.Background = 1, colors.Border(), colors.Raised()
				}
				return s
			}
			levels = append(levels, core.View{StyleFunc: face, Children: []core.Node{ui.Text(itoa(level), 13, ui.Semibold, fg)}})
		}
		title := tr.NavLevel(depth)
		return screen(cx, title, nil, navPage(cx, tr.NavEveryPushIsANative(),
			ui.Card{Title: title, Description: tr.NavDeep(depth), Children: []core.Node{
				core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 6}, Children: levels},
				core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
					ui.Button{Text: tr.NavDeeper(), Icon: lucide.Plus, Flex: 1, OnPress: func() { shell.push(deepPage(depth + 1)) }},
					ui.Button{Text: tr.NavToTheIndex(), Variant: ui.ButtonSecondary, Flex: 1, OnPress: func() { shell.nav.PopToRoot() }},
				}},
			}}))
	}
}

type modalKind struct {
	key, title, note string
	icon             func() *core.SVG
}

// modalBody is a modal's content: what it is, its safe area and a close
// button.
func modalBody(cx *core.Scope, title, noteText func() string, body func() string, onClose func()) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	return core.View{Style: core.Style{PaddingEnv: core.EnvAll(core.EnvSafeArea), FlexGrow: 1}, StyleFunc: ui.Filled(colors.Background), Children: []core.Node{
		core.View{Style: core.Style{FlexGrow: 1, Padding: core.All(16), Gap: 16}, Children: []core.Node{
			core.View{Style: core.Style{Gap: 8, Padding: core.Edges{Top: 8, Right: 4, Left: 4}}, Children: []core.Node{
				core.View{Style: core.Style{Direction: core.Row}, Children: []core.Node{ui.Badge{TextFunc: noteText, Tone: ui.ToneAccent}}},
				core.Text{ContentFunc: title, Style: core.Style{FontSize: 28, FontWeight: ui.Bold, LetterSpacing: -0.5}, StyleFunc: ui.Colored(colors.Text)},
				core.Text{ContentFunc: body, Style: core.Style{FontSize: 13, LineHeight: 21}, StyleFunc: ui.Colored(colors.Muted)},
			}},
			ui.Section{Title: tr.NavSafeArea(), Note: tr.NavTheModalS(), Children: []core.Node{insetsRow(cx)}},
			ui.Spacer{},
			ui.Button{Text: tr.NavClose(), OnPress: onClose},
		}},
	}}
}

// navigationDemo is base.Nav's screens and their options, core.Modal's
// presentations through base.Dialog, and the back button.
func navigationDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	open := core.NewSignal(cx, "")
	intercept := core.NewSignal(cx, false)
	backs, cancels := core.NewSignal(cx, 0), core.NewSignal(cx, 0)
	closeModal := func() { open.Set("") }
	modals := []modalKind{
		{"fullScreen", tr.NavFullScreen(), tr.NavSlidesUp(), lucide.Smartphone},
		{"pageSheet", tr.NavPageSheet(), ui.Hint(cx, tr.NavIosSheet(), tr.NavATallCenteredCard()), lucide.Component},
		{"formSheet", tr.NavFormSheet(), ui.Hint(cx, tr.NavIosCentered(), tr.NavASmallerCenteredCard()), lucide.TextCursorInput},
		{"fade", tr.NavFade(), tr.NavFadesIn(), lucide.Sparkles},
		{"dialog", tr.NavDialog(), tr.NavSeeThrough(), lucide.Bell},
		{"kept", tr.NavKeptOpen(), ui.Hint(cx, tr.NavNoSwipeToClose(), tr.NavEscapeDoesNotClose()), lucide.ArrowDown},
	}
	// A handler hears back before the stack pops; it lives only while
	// intercepting, in a scope of its own.
	var handler *core.Scope
	cx.Effect(func() {
		on := intercept.Get()
		if handler != nil {
			handler.Dispose()
			handler = nil
		}
		if on {
			handler = cx.Child()
			handler.OnBackPress(func() bool {
				backs.Update(func(n int) int { return n + 1 })
				return true
			})
		}
	})
	// The dialog springs up from a little smaller each time it opens.
	pop := core.NewAnimatedValue(cx, 0)
	cx.Effect(func() {
		if open.Get() != "dialog" {
			return
		}
		pop.Set(0)
		ui.Spring(cx, pop, 1, ui.SettleSpring)
	})
	current := func() modalKind {
		for _, m := range modals {
			if m.key == open.Get() {
				return m
			}
		}
		return modalKind{}
	}
	platformName := ui.Hint(cx, "UIKit / View", tr.NavNativeViews())
	if cx.Env().OS() == core.OSLinux {
		platformName = "GTK 4"
	}

	stack := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		return ui.Card{Title: tr.NavNativeStack(), Description: tr.NavHeadersTransitionsTheSafeArea(), Actions: []core.Node{ui.Badge{Text: platformName}}, Children: []core.Node{insetsRow(cx)}}
	}
	pages := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		return ui.Card{Title: tr.NavPages(), Description: tr.NavScreenOptionsPushedWithNav(), Flush: true, Children: []core.Node{
			ui.LinkRow{Icon: lucide.Type, Title: tr.NavLargeTitle(), Note: "HeaderLargeTitle", OnPress: func() { shell.push(largeTitlePage) }},
			ui.LinkRow{Divided: true, Icon: lucide.Palette, Title: tr.NavTransparentHeader(), Note: "HeaderTransparent", OnPress: func() { shell.push(transparentPage) }},
			ui.LinkRow{Divided: true, Icon: lucide.Star, Title: tr.NavCustomHeaderItems(), Note: "HeaderLeft / Title / Right", OnPress: func() { shell.push(customHeaderPage) }},
			ui.LinkRow{Divided: true, Icon: lucide.Smartphone, Title: tr.NavNoHeader(), Note: "HeaderHidden", OnPress: func() { shell.push(headerlessPage) }},
			ui.LinkRow{Divided: true, Icon: lucide.Hand, Title: tr.NavNoSwipeBack(), Note: "NoPopOnSwipe", OnPress: func() { shell.push(lockedPage) }},
			ui.LinkRow{Divided: true, Icon: lucide.TextCursorInput, Title: tr.NavUnsavedChanges(), Note: "NoPopOnBack + OnBackPress", OnPress: func() { shell.push(unsavedPage) }},
			ui.LinkRow{Divided: true, Icon: lucide.List, Title: tr.NavADeepStack(), Note: "Push / PopToRoot", OnPress: func() { shell.push(deepPage(1)) }},
		}}
	}
	modalsCard := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		var tiles []core.Node
		for _, m := range modals {
			tiles = append(tiles, ui.Press{OnPress: func() { open.Set(m.key) }, Label: m.title, Style: core.Style{Width: core.Pct(48.5)}, Children: []core.Node{
				core.View{StyleFunc: func() core.Style {
					return core.Style{MinHeight: core.Pt(100), Padding: core.All(12), Gap: 12, CornerRadius: ui.RadiusInset, BorderCurve: core.BorderCurveContinuous,
						Background: colors.Inset(), BorderWidth: 1, BorderColor: colors.Line()}
				}, Children: []core.Node{
					ui.IconTile{Icon: m.icon, Size: 32},
					core.View{Style: core.Style{Gap: 2}, Children: []core.Node{ui.Text(m.title, 13, ui.Medium, colors.Text), ui.Text(m.note, 13, ui.Regular, colors.Faint)}},
				}},
			}})
		}
		return ui.Card{Title: tr.NavModals(), Description: tr.NavPresentationStylesAndTransitions(), Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, JustifyContent: core.JustifySpaceBetween, RowGap: 8}, Children: tiles},
		}}
	}
	backCard := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		return ui.Card{Title: tr.NavBackButton(), Description: ui.Hint(cx, tr.NavOnbackpressTheBackButtonAnd(), tr.NavOnbackpressEscapeTheMouseS()), Actions: []core.Node{ui.Switch{Checked: intercept.Get, OnChange: intercept.Set, Label: tr.NavInterceptTheBackButton()}}, Children: []core.Node{
			ui.Figure{Value: func() string { return itoa(backs.Get()) }, Note: func() string {
				unit := "presses"
				if backs.Get() == 1 {
					unit = "press"
				}
				if intercept.Get() {
					return unit + tr.NavCaught()
				}
				return unit + tr.NavPopped()
			}, Tone: ui.ToneAccent},
			ui.Note{TextFunc: func() string {
				if intercept.Get() {
					return tr.NavTheHandlerReturnsTrueBack()
				}
				return ui.Hint(cx, tr.NavOffTheBackButtonPops(), tr.NavOffEscapePopsThisScreen())
			}},
		}}
	}

	isShown := func(keys ...string) func() bool {
		return func() bool {
			return strings.Contains(" "+strings.Join(keys, " ")+" ", " "+open.Get()+" ") && open.Get() != ""
		}
	}
	modalsNodes := []core.Node{
		core.Dynamic(func(cx *core.Scope) core.Node {
			tr := locales.For(cx)
			k := open.Get()
			presentation, animation := core.ModalFullScreen, core.ModalSlide
			switch k {
			case "pageSheet":
				presentation = core.ModalPageSheet
			case "formSheet":
				presentation = core.ModalFormSheet
			case "fade":
				animation = core.ModalFade
			}
			if !isShown("fullScreen", "pageSheet", "formSheet", "fade")() {
				return core.Group{}
			}
			return base.Dialog{Open: func() bool { return true }, Presentation: presentation, Animation: animation, Label: current().title,
				OnOpenChange: func(o bool, _ base.Reason) {
					if !o {
						closeModal()
					}
				},
				PopupStyle: core.Style{FlexGrow: 1, AlignSelf: core.AlignStretch},
				Children: []core.Node{modalBody(cx, func() string { return current().title }, func() string { return k }, func() string {
					if k == "pageSheet" || k == "formSheet" {
						return ui.Hint(cx, tr.NavCloseWithTheButtonThe(),
							tr.NavCloseWithTheButtonEscape())
					}
					return ui.Hint(cx, tr.NavCloseWithTheButtonOr(), tr.NavCloseWithTheButtonOr2())
				}, closeModal)}}
		}),
		base.Dialog{Open: isShown("kept"), Presentation: core.ModalPageSheet, NoCloseOnEscape: true, NoCloseOnBackdropPress: true, NoCloseOnSwipe: true,
			OnCancel: func() { cancels.Update(func(n int) int { return n + 1 }) }, Label: tr.NavKeptOpen(),
			PopupStyle: core.Style{FlexGrow: 1, AlignSelf: core.AlignStretch},
			Children: []core.Node{modalBody(cx, label(tr.NavKeptOpen()), label("NoCloseOn…"), func() string {
				return tr.NavOnlyCallOnCancel(ui.Hint(cx, tr.NavSwipingItDownIosAnd(), tr.NavEscapeAndClicksOutside()), cancels.Get())
			}, closeModal)}},
		base.Dialog{Open: isShown("dialog"), Animation: core.ModalFade, OnOpenChange: func(o bool, _ base.Reason) {
			if !o {
				closeModal()
			}
		}, Label: tr.NavADialog(), PopupStyle: core.Style{FlexGrow: 1, AlignSelf: core.AlignStretch},
			Children: []core.Node{core.View{Style: core.Style{FlexGrow: 1, Background: ui.Black(0.62), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Padding: core.All(16)},
				Children: []core.Node{core.View{Style: core.Style{AlignSelf: core.AlignStretch, MaxWidth: core.Pt(480), Animated: &core.AnimatedStyle{Opacity: pop, Scale: ui.Lerp(pop, 0.9, 1)}},
					Children: []core.Node{ui.Card{Title: tr.NavADialog(), Description: "Animation: ModalFade", Children: []core.Node{
						core.Text{Content: tr.NavADialogOwnsNoLook(),
							Style: core.Style{FontSize: 13, LineHeight: 21}, StyleFunc: ui.Colored(colors.Muted)},
						core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
							ui.Button{Text: tr.NavCancel(), Variant: ui.ButtonSecondary, Flex: 1, OnPress: closeModal},
							ui.Button{Text: tr.NavOk(), Flex: 1, OnPress: closeModal},
						}},
					}}}}}}}},
	}

	return demoPage(cx, pageProps{after: modalsNodes}, stack, pages, modalsCard, backCard)
}

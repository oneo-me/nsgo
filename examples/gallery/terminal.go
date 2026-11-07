package main

import (
	"github.com/oneo-me/nsgo/icons/lucide"
	"runtime"

	"github.com/oneo-me/nsgo/base/terminal"
	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// base/terminal's View runs the user's shell with Ghostty's emulator, or
// on phones, which run no local programs, a demo session standing in for
// a remote one (terminal_session.go). The panel beside it sets what the
// view and the terminal take, each under its API's name, and reads back
// what the shell reports. The context menu around the view is the app's,
// calling the view's Ref; on touch screens a long press selects a word and
// opens it, and a bar of keys shows over the keyboard. Where no terminal
// runs (the web) the demo says why.

// terminalThemes are Ghostty's themes the panel offers, after following
// the appearance.
var terminalThemes = []string{"Catppuccin Mocha", "Catppuccin Latte", "Dracula", "Nord", "GitHub Light Default"}

func terminalDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	theme := core.NewSignal(cx, "")
	family := core.NewSignal(cx, "")
	size := core.NewSignal(cx, 13.0)
	cursor := core.NewSignal(cx, terminal.CursorBlock)
	padX, padY := core.NewSignal(cx, 6.0), core.NewSignal(cx, 4.0)
	title, dir, cells := core.NewSignal(cx, "—"), core.NewSignal(cx, "—"), core.NewSignal(cx, [2]int{})
	generation := core.NewSignal(cx, 0)
	// Phones run no local programs: their session is the demo's, as is a
	// phone's under a desktop's tests.
	local := runtime.GOOS != "ios" && runtime.GOOS != "android" && core.Untracked(cx, cx.Env().Desktop)
	demo := core.NewSignal(cx, !local)
	keyBar := core.NewSignal(cx, true)
	ref := &terminal.Ref{}
	// The terminal running, for the cursor's setter.
	var term *terminal.Terminal

	screen := core.Dynamic(func(cx *core.Scope) core.Node {
		generation.Get()
		var conn *demoSession
		if demo.Get() {
			conn = newDemoSession()
		}
		var t *terminal.Terminal
		opts := terminal.Options{
			Cursor: core.Untracked(cx, cursor.Get),
			OnTitle: func(s string) {
				cx.Post(func() {
					title.Set(s)
					if d := t.Dir(); d != "" {
						dir.Set(d)
					}
				})
			},
		}
		if conn != nil {
			opts.Conn = conn
		}
		t, err := terminal.New(opts)
		if err != nil {
			return core.View{Style: core.Style{FlexGrow: 1, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Padding: core.All(24)},
				Children: []core.Node{ui.Text(tr.TerminalUnavailable(err.Error()), 13, ui.Regular, colors.Muted)}}
		}
		term = t
		cx.Cleanup(func() { t.Close() })
		// The view follows the options; the terminal and its shell stay.
		return core.Dynamic(func(cx *core.Scope) core.Node {
			var th *terminal.Theme
			if name := theme.Get(); name != "" {
				th, _ = terminal.GhosttyTheme(name)
			}
			view := terminal.View{Terminal: t, Label: tr.DemoTerminal(), TestID: "terminal", AutoFocus: true, Ref: ref,
				Theme: th, DarkTheme: th,
				Font:         core.Font{Family: family.Get(), Size: core.Points(size.Get())},
				Padding:      terminal.Padding{X: core.Points(padX.Get()), Y: core.Points(padY.Get())},
				OnSizeChange: func(cols, rows int) { cells.Set([2]int{cols, rows}) },
				Style:        core.Style{FlexGrow: 1},
				NoKeyBar:     !keyBar.Get(),
				KeyBarStyle:  core.Style{Borders: core.Edges{Top: 1}, Padding: core.Symmetric(6, 6), FlexShrink: 0},
				KeyBarStyleFunc: func() core.Style {
					return core.Style{Background: colors.Raised(), BorderColors: core.EdgeColors{Top: colors.Line()}}
				},
				KeyStyle: core.Style{MinWidth: core.Pt(40), Height: core.Pt(34), Padding: core.Symmetric(0, 10), Margin: core.Edges{Right: 6},
					CornerRadius: 6, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, BorderWidth: 1, FontSize: 15},
				KeyStyleFunc: func() core.Style {
					return core.Style{Background: colors.Surface(), BorderColor: colors.Border(), Color: colors.Text(),
						Active: &core.Style{Background: colors.Hover()}}
				},
				KeyOnStyleFunc: func() core.Style {
					return core.Style{Background: colors.AccentFill(), BorderColor: colors.AccentFill(), Color: colors.OnAccent()}
				},
			}
			return ui.ContextMenu{TestID: "terminal-menu", Style: core.Style{FlexGrow: 1}, Trigger: view, Children: []core.Node{
				ui.MenuItem{Text: tr.TerminalCopy(), Icon: lucide.Copy, Disabled: func() bool { return !ref.Selected() }, OnSelect: ref.Copy},
				ui.MenuItem{Text: tr.TerminalPaste(), OnSelect: ref.Paste},
				ui.MenuItem{Text: tr.TerminalSelectAll(), OnSelect: ref.SelectAll},
			}}
		})
	})

	option := func(label, api string, control core.Node) core.Node {
		return core.View{Style: core.Style{Gap: 6, Padding: core.Symmetric(6, 0)}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Gap: 8}, Children: []core.Node{
				ui.Text(label, 13, ui.Medium, colors.Text), API{Text: api},
			}},
			control,
		}}
	}
	number := func(label string, s *core.Signal[float64], lo, hi float64) core.Node {
		return ui.NumberInput{Label: label, Value: s.Get, OnChange: s.Set, Min: lo, Max: hi, Step: 1, Style: core.Style{FlexGrow: 1}}
	}
	themeRows := []core.Node{}
	for i, name := range append([]string{""}, terminalThemes...) {
		label := name
		if name == "" {
			label = tr.TerminalAutomatic()
		}
		themeRows = append(themeRows, Row{First: i == 0, OnPress: func() { theme.Set(name) }, Children: []core.Node{
			core.Text{Content: label, Style: core.Style{FlexGrow: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Text)},
			core.Text{ContentFunc: func() string {
				if theme.Get() == name {
					return "✓"
				}
				return ""
			}, Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Accent)},
		}})
	}
	// Phones have the demo's session alone.
	var sessionOption core.Node = core.Group{}
	if local {
		sessionOption = option(tr.TerminalSession(), "Options.Conn", ui.Tabs[bool]{Equal: true, Value: demo.Get,
			OnChange: func(d bool) {
				if d || local {
					demo.Set(d)
					title.Set("—")
					dir.Set("—")
				}
			},
			Options: []ui.Option[bool]{{Value: false, Label: tr.TerminalLocalShell()}, {Value: true, Label: tr.TerminalDemoSession()}}})
	}
	return stagePage(cx, core.View{StyleFunc: func() core.Style {
		return fin.Card().Over(core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0), CornerRadius: ui.RadiusInset,
			BorderWidth: 1, BorderColor: colors.Border(), Overflow: core.Hidden})
	}, Children: []core.Node{screen}},
		Section{Title: tr.TerminalOptions(), Note: "terminal.View", Children: []core.Node{
			sessionOption,
			option(tr.TerminalTheme(), "Theme · DarkTheme", core.View{Children: themeRows}),
			option(tr.TerminalFont(), "Font.Family", ui.Tabs[string]{Equal: true, Value: family.Get, OnChange: family.Set,
				Options: []ui.Option[string]{{Value: "", Label: tr.TerminalMonospace()}, {Value: "Menlo", Label: "Menlo"}, {Value: "Courier New", Label: "Courier"}}}),
			option(tr.TerminalFontSize(), "Font.Size", number(tr.TerminalFontSize(), size, 8, 32)),
			option(tr.TerminalPadding()+" · "+tr.TerminalAcross(), "Padding.X", number(tr.TerminalAcross(), padX, 0, 40)),
			option(tr.TerminalPadding()+" · "+tr.TerminalDown(), "Padding.Y", number(tr.TerminalDown(), padY, 0, 40)),
			option(tr.TerminalCursor(), "Terminal.SetCursor", ui.Tabs[terminal.CursorStyle]{Equal: true, Value: cursor.Get,
				OnChange: func(c terminal.CursorStyle) {
					cursor.Set(c)
					if term != nil {
						term.SetCursor(c)
					}
				},
				Options: []ui.Option[terminal.CursorStyle]{{Value: terminal.CursorBlock, Label: tr.TerminalBlock()},
					{Value: terminal.CursorBar, Label: tr.TerminalBar()}, {Value: terminal.CursorUnderline, Label: tr.TerminalUnderline()}}}),
		}},
		Section{Title: tr.TerminalKeyBar(), Note: "KeyBarKeys · NoKeyBar", Children: []core.Node{
			SwitchRow{Label: tr.TerminalKeyBar(), Note: tr.TerminalKeyBarNote(), Checked: keyBar.Get, OnChange: keyBar.Set},
		}},
		Section{Title: tr.TerminalStatus(), Note: "OnTitle · OnSizeChange · Dir", Children: []core.Node{
			statusLine(cx, tr.TerminalTitle(), "terminal-title", title.Get),
			statusLine(cx, tr.TerminalSize(), "terminal-size", func() string { c := cells.Get(); return tr.TerminalSizeCells(c[0], c[1]) }),
			statusLine(cx, tr.TerminalDirectory(), "terminal-dir", dir.Get),
		}},
		ui.Button{Text: tr.TerminalRestart(), Variant: ui.ButtonSecondary, OnPress: func() {
			title.Set("—")
			dir.Set("—")
			generation.Update(func(n int) int { return n + 1 })
		}})
}

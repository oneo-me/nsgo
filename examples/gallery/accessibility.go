package main

import (
	"fmt"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
	"github.com/oneo-me/nsgo/examples/gallery/locales"
)

func always() bool { return true }

// accessibilityDemo is what screen readers read beyond roles and labels:
// values and hints, expanded states, actions, groups, hidden elements,
// live regions and focus. Turn on VoiceOver or TalkBack to hear it. NSH's
// gallery shows accessibility within its other demos; NSGo gives it a page.
func accessibilityDemo(cx *core.Scope) core.Node {
	values := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		n := core.NewSignal(cx, 2)
		adjust := func(by int) { n.Update(func(v int) int { return min(max(v+by, 0), 10) }) }
		step := func(name ui.IconName, by int, label string) core.Node {
			// The buttons are for the pointer; the screen reader adjusts
			// the quantity itself.
			return core.View{AccessibilityElementsHidden: always, Children: []core.Node{ui.RoundButton{Icon: name, Label: label, Variant: ui.ButtonSecondary, Size: 36, OnPress: func() { adjust(by) }}}}
		}
		return ui.Card{Title: tr.A11yValuesAndHints(), Description: tr.A11yAnAdjustableElementSwipeUp(), Children: []core.Node{
			core.View{Role: core.RoleAdjustable, Label: tr.A11yQuantity(), Hint: tr.A11ySwipeUpOrDownTo(), TestID: "a11y-quantity",
				AccessibilityValue: func() core.AccessibilityValue {
					return core.AccessibilityValue{Max: 10, Now: float32(n.Get()), Text: tr.A11yItems(n.Get())}
				},
				AccessibilityActions: []core.AccessibilityAction{{Name: core.ActionIncrement}, {Name: core.ActionDecrement}},
				OnAccessibilityAction: func(name string) {
					if name == core.ActionIncrement {
						adjust(1)
					} else {
						adjust(-1)
					}
				},
				Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 16},
				Children: []core.Node{
					step(ui.IconMinus, -1, tr.A11yFewer()),
					ui.Figure{Value: func() string { return itoa(n.Get()) }, Unit: "items"},
					step(ui.IconPlus, 1, tr.A11yMore()),
				}},
			ui.Meter{Value: func() float64 { return float64(n.Get()) / 10 }, Tone: ui.ToneAccent},
			ui.APILine{Label: tr.A11yARangeValueAndThe(), Names: "RoleAdjustable"},
		}}
	}

	expanded := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		open := core.NewSignal(cx, false)
		return ui.Card{Title: tr.A11yExpandedState(), Description: tr.A11yScreenReadersSayWhetherIt(), Children: []core.Node{
			base.Button{Label: tr.A11yShippingDetails(), Expanded: open.Get, TestID: "a11y-disclosure", OnPress: func(core.PressEvent) { toggled(open) },
				Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, MinHeight: core.Pt(44), Active: &core.Style{Opacity: core.Alpha(0.7)}},
				Children: []core.Node{
					core.Text{Content: tr.A11yShippingDetails(), Style: core.Style{FlexGrow: 1, FontSize: 15}, StyleFunc: ui.Colored(colors.Text)},
					core.View{StyleFunc: func() core.Style {
						s := core.Style{Transition: core.Transition{Duration: ms(150), Properties: core.TransitionTransform}}
						if open.Get() {
							s.Transform.Rotate = 90
						}
						return s
					}, Children: []core.Node{ui.Icon(ui.IconChevron, 16, colors.Muted)}},
				}},
			core.Show{When: open.Get, Then: func(*core.Scope) core.Node { return ui.Note{Text: tr.A11yShipsInTwoDaysFrom()} }},
			ui.APILine{Label: tr.A11yADisclosure(), Names: "Expanded"},
		}}
	}

	actions := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		fin := ui.FinOf(cx)
		last := core.NewSignal(cx, tr.A11yNoneYet())
		return ui.Card{Title: tr.A11yActions(), Description: tr.A11yInTheScreenReaderS(), Children: []core.Node{
			core.View{Accessible: true, TestID: "a11y-message",
				AccessibilityActions: []core.AccessibilityAction{
					{Name: "archive", Label: tr.A11yArchive()}, {Name: "flag", Label: tr.A11yFlag()},
					{Name: core.ActionActivate}, {Name: core.ActionEscape}, {Name: core.ActionMagicTap},
				},
				OnAccessibilityAction: last.Set,
				StyleFunc: func() core.Style {
					return fin.Raised().Over(core.Style{Background: colors.Raised(), CornerRadius: ui.RadiusControl, BorderWidth: 1, BorderColor: colors.Border(), Padding: core.All(12), Gap: 4})
				},
				Children: []core.Node{ui.Text("Ana", 15, ui.Semibold, colors.Text), ui.Text(tr.A11yLunchOnFriday(), 15, ui.Regular, colors.Muted)}},
			ui.Caption{Label: tr.A11yLastAction(), Note: last.Get},
			ui.APILine{Label: tr.A11yCustomOnesActivateEscapeAnd(), Names: "AccessibilityActions"},
		}}
	}

	grouped := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		fin := ui.FinOf(cx)
		price := core.NewSignal(cx, 120)
		return ui.Card{Title: tr.A11yGroupedElements(), Description: tr.A11yOneElementLabelledWithThe(), Children: []core.Node{
			core.View{Accessible: true, TestID: "a11y-card",
				StyleFunc: func() core.Style {
					return fin.Raised().Over(core.Style{Background: colors.Raised(), CornerRadius: ui.RadiusControl, BorderWidth: 1, BorderColor: colors.Border(), Padding: core.All(12), Gap: 4})
				},
				Children: []core.Node{
					ui.Text(tr.A11yApples(), 15, ui.Semibold, colors.Text),
					ui.Textf(func() string { return tr.A11yAKilo(fmt.Sprintf("$%d.%02d", price.Get()/100, price.Get()%100)) }, 15, ui.Regular, colors.Muted),
				}},
			ui.Button{Text: tr.A11yChangeThePrice(), Variant: ui.ButtonSecondary, OnPress: func() { price.Update(func(p int) int { return p + 15 }) }},
		}}
	}

	hidden := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		h := core.NewSignal(cx, true)
		return ui.Card{Title: tr.A11yHiddenElements(), Description: tr.A11yDecorationScreenReadersSkip(), Children: []core.Node{
			core.View{AccessibilityElementsHidden: h.Get, TestID: "a11y-hidden", Style: core.Style{AlignItems: core.AlignCenter},
				Children: []core.Node{ui.Text(tr.A11yDecoration(), 15, ui.Regular, colors.Faint)}},
			ui.SwitchRow{Label: tr.A11yHideTheDecoration(), Code: "AccessibilityElementsHidden", Checked: h.Get, OnChange: h.Set},
		}}
	}

	live := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		added := core.NewSignal(cx, 0)
		return ui.Card{Title: tr.A11yLiveRegions(), Description: tr.A11yChangesAnnouncedWithoutMovingFocus(), Children: []core.Node{
			core.Text{LiveRegion: core.LiveRegionPolite, TestID: "a11y-live", ContentFunc: func() string { return tr.A11yInTheBasket(added.Get()) },
				Style: core.Style{FontSize: 17, FontWeight: ui.Medium}, StyleFunc: ui.Colored(colors.Text)},
			ui.Button{Text: tr.A11yAddToBasket(), Icon: ui.IconPlus, OnPress: func() { added.Update(func(n int) int { return n + 1 }) }},
			ui.APILine{Label: tr.A11yAnnouncedAsItChanges(), Names: "LiveRegionPolite"},
		}}
	}

	focus := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		result := &core.Ref{}
		return ui.Card{Title: tr.A11yFocus(), Description: tr.A11yMovingTheScreenReaderAs(), Children: []core.Node{
			ui.Button{Text: tr.A11ySearchApples(), Variant: ui.ButtonSecondary, TestID: "a11y-search", OnPress: result.SetAccessibilityFocus},
			core.Text{Ref: result, Content: tr.A11y3ResultsForApples(), TestID: "a11y-result", Style: core.Style{FontSize: 15}, StyleFunc: ui.Colored(colors.Text)},
			ui.APILine{Label: tr.A11yMovesTheReaderHere(), Names: "Ref.SetAccessibilityFocus"},
		}}
	}

	return demoPage(cx, pageProps{}, values, expanded, actions, grouped, hidden, live, focus)
}

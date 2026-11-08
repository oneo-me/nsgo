package main

import (
	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// popupStyle is the popups' look: the gallery's, as base draws none.
func popupStyle(cx *core.Scope) func() core.Style {
	colors := ui.ColorsOf(cx)
	return func() core.Style {
		return ui.FinOf(cx).Raised().Over(core.Style{Background: colors.Raised(), CornerRadius: ui.RadiusInset, BorderWidth: 1, BorderColor: colors.Border(),
			Padding: core.All(6), MinWidth: core.Pt(180)})
	}
}

// trigger is a pressable chip that opens a popup.
func trigger(cx *core.Scope, label func() string) core.Node {
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	return core.View{StyleFunc: func() core.Style {
		return fin.Raised().Over(core.Style{AlignSelf: core.AlignStart, Padding: core.Symmetric(0, 14), Height: core.Pt(36),
			JustifyContent: core.JustifyCenter, CornerRadius: ui.RadiusControl, Background: colors.Raised(), Active: &core.Style{Background: colors.Border()}})
	},
		Children: []core.Node{ui.Textf(label, 13, ui.Medium, colors.Text)}}
}

// cardsDemo is a demo page of cards.
func cardsDemo(cards ...cardFunc) func(cx *core.Scope) core.Node {
	return func(cx *core.Scope) core.Node { return demoPage(cx, pageProps{}, cards...) }
}

// overlayCard is core.Overlay: its children on top of the screen whatever
// clips them, anchored to an element; base's popups are built on it.
func overlayCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	open := core.NewSignal(cx, false)
	anchor := &core.Ref{}
	return Card{Title: tr.OverlayOverlay(), Description: tr.OverlayABoxThatClipsIts(), Actions: []core.Node{ui.Badge{Text: "core"}}, Children: []core.Node{
		core.View{Style: core.Style{Height: core.Pt(56), Overflow: core.Hidden, CornerRadius: ui.RadiusInset,
			JustifyContent: core.JustifyCenter, Padding: core.Symmetric(0, 12)}, StyleFunc: ui.Filled(colors.Inset), Children: []core.Node{
			core.View{Ref: anchor, OnPress: func(core.PressEvent) { toggled(open) }, Role: core.RoleButton, Style: core.Style{AlignSelf: core.AlignStart},
				Children: []core.Node{trigger(cx, func() string {
					if open.Get() {
						return tr.OverlayClose()
					}
					return tr.OverlayOpenTheOverlay()
				})}},
			core.Show{When: open.Get, Then: func(*core.Scope) core.Node {
				return core.Overlay{Anchor: anchor, Placement: core.PlaceBottom, Align: core.AlignStart, Offset: 6, Margin: 8,
					OnPressOutside: func(core.Point) { open.Set(false) }, OnCancel: func() { open.Set(false) },
					StyleFunc: popupStyle(cx), TestID: "overlay",
					Children: []core.Node{core.Text{Content: tr.OverlayOutsideTheClippedBoxOn(),
						Style: core.Style{FontSize: 13, Padding: core.All(6)}, StyleFunc: ui.Colored(colors.Text)}}}
			}},
		}},
		Caption{Label: tr.OverlayScrollThePageItFollows()},
	}}
}

// sheetCard is ui.Sheet, a panel at the bottom over a backdrop.
func sheetCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	open := core.NewSignal(cx, false)
	side := core.NewSignal(cx, core.PlaceBottom)
	swipe := core.NewSignal(cx, false)
	close := func() { open.Set(false) }
	return Playground{Title: "Sheet", Note: tr.OverlayAPanelAtTheBottom(), Base: baseSheetForm(cx, side, swipe), Stage: func() core.Node {
		return core.Group{
			ui.Button{Text: tr.OverlayShowTheSheet(), Variant: ui.ButtonSecondary, OnPress: func() { open.Set(true) }},
			ui.Sheet{Open: open.Get, OnClose: close, Children: []core.Node{
				ui.Text(tr.OverlaySheet(), 15, ui.Semibold, colors.Text),
				ui.Text(tr.OverlayTheBackdropEscapeAndBack(), 13, ui.Regular, colors.Muted),
				ui.Button{Text: tr.OverlayDone(), OnPress: close},
			}},
		}
	}, BaseKnobs: []core.Node{choiceKnob("Side", side, placements...), boolKnob("CloseOnSwipe", swipe)}}
}

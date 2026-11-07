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

// placementName is a placement as the demos report it.
func placementName(p core.Placement) string {
	return [...]string{"default", "bottom", "top", "left", "right"}[p]
}

// eventBar is the last event a page's cards reported, along its bottom
// once there is one.
func eventBar(cx *core.Scope, last *core.Signal[string]) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	return core.Show{When: func() bool { return last.Get() != "" }, Then: func(*core.Scope) core.Node {
		return core.View{StyleFunc: func() core.Style {
			return core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, MinHeight: core.Pt(32), Padding: core.Symmetric(4, 16),
				PaddingEnv: core.EnvEdges{Bottom: core.EnvSafeArea}, FlexShrink: 0, Borders: core.Edges{Top: 1}, BorderColors: core.EdgeColors{Top: colors.Line()},
				Background: colors.Background()}
		}, Children: []core.Node{
			ui.Text(tr.OverlayLastEvent(), 12, ui.Regular, colors.Muted),
			core.Text{ContentFunc: last.Get, TestID: "last-event", MaxLines: 1, Style: core.Style{FlexShrink: 1, FontSize: 12, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Text)},
		}}
	}}
}

// eventsDemo is a demo of cards that report what happened, over the last
// event they reported.
func eventsDemo(cards ...func(last *core.Signal[string]) cardFunc) func(cx *core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		last := core.NewSignal(cx, "")
		var fns []cardFunc
		for _, card := range cards {
			fns = append(fns, card(last))
		}
		return core.View{Style: fillPage, Children: []core.Node{demoPage(cx, pageProps{}, fns...), eventBar(cx, last)}}
	}
}

// plainCard makes a card that reports nothing a card of an eventsDemo.
func plainCard(card cardFunc) func(*core.Signal[string]) cardFunc {
	return func(*core.Signal[string]) cardFunc { return card }
}

// overlayCard is core.Overlay: its children on top of the screen whatever
// clips them, anchored to an element; base's popups are built on it.
func overlayCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
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
						OnPosition: func(p core.OverlayPosition) { last.Set(tr.OverlayOverlayAt() + placementName(p.Placement)) },
						StyleFunc:  popupStyle(cx), TestID: "overlay",
						Children: []core.Node{core.Text{Content: tr.OverlayOutsideTheClippedBoxOn(),
							Style: core.Style{FontSize: 13, Padding: core.All(6)}, StyleFunc: ui.Colored(colors.Text)}}}
				}},
			}},
			Caption{Label: tr.OverlayScrollThePageItFollows()},
		}}
	}
}

// sheetCard is ui.Sheet, a panel at the bottom over a backdrop.
func sheetCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		open := core.NewSignal(cx, false)
		side := core.NewSignal(cx, core.PlaceBottom)
		swipe := core.NewSignal(cx, false)
		close := func() { open.Set(false); last.Set(tr.OverlaySheet2() + "closed") }
		return Playground{Title: "Sheet", Note: tr.OverlayAPanelAtTheBottom(), Base: baseSheetForm(cx, last, side, swipe), Stage: func() core.Node {
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
}

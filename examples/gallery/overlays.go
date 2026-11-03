package main

import (
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
	"github.com/oneo-me/nsgo/examples/gallery/locales"
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

// eventsDemo is a demo of cards that report what happened, above the last
// event they reported.
func eventsDemo(cards ...func(last *core.Signal[string]) cardFunc) func(cx *core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		last := core.NewSignal(cx, "—")
		var fns []cardFunc
		for _, card := range cards {
			fns = append(fns, card(last))
		}
		return demoPage(cx, pageProps{}, append(fns, lastEventCard(last))...)
	}
}

// overlayCard is core.Overlay: its children on top of the screen whatever
// clips them, anchored to an element; base's popups are built on it.
func overlayCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		open := core.NewSignal(cx, false)
		anchor := &core.Ref{}
		return ui.Card{Title: tr.OverlayOverlay(), Description: tr.OverlayABoxThatClipsIts(), Actions: []core.Node{ui.Badge{Text: "core"}}, Children: []core.Node{
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
			ui.Caption{Label: tr.OverlayScrollThePageItFollows()},
		}}
	}
}

// popoverCard is base.Popover, a panel anchored to its trigger.
func popoverCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		return ui.Card{Title: tr.OverlayPopover(), Description: tr.OverlayAnchoredToItsTriggerOn(), Children: []core.Node{
			base.Popover{Offset: 6, Margin: 8, PopupStyleFunc: popupStyle(cx), Label: tr.OverlayShowThePopover(),
				OnOpenChange: func(open bool, reason base.Reason) {
					state := "closed"
					if open {
						state = "opened"
					}
					last.Set(tr.OverlayPopover2() + state + " (" + reason.String() + ")")
				},
				Trigger: trigger(cx, func() string { return tr.OverlayShowThePopover() }),
				Children: []core.Node{core.View{Style: core.Style{Padding: core.All(8), Gap: 4, MaxWidth: core.Pt(260)}, Children: []core.Node{
					ui.Text(tr.OverlayPopover(), 13, ui.Semibold, colors.Text),
					ui.Text(tr.OverlayAPressOutsideEscapeOr(), 13, ui.Regular, colors.Muted),
				}}}},
		}}
	}
}

// tooltipCard is a view's Tooltip, the platform's bubble, and
// base.Tooltip, the app's, which reports.
func tooltipCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		hoveredOpen, pinned := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return ui.Card{Title: tr.OverlayTooltip(), Description: ui.Hint(cx, tr.OverlayTooltipsFollowAPointerPin(), tr.OverlayRestThePointerOnA()), Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 8}, Children: []core.Node{
				// A view's Tooltip: the platform's bubble with the defaults.
				core.View{Tooltip: core.Text{Content: tr.OverlaySavesTheDocument(), StyleFunc: func() core.Style {
					return core.Style{FontSize: 12, Color: colors.Background(),
						Background: colors.Text(), CornerRadius: 6, Padding: core.Symmetric(4, 8)}
				}},
					Children: []core.Node{ui.Button{Text: tr.OverlaySave(), Variant: ui.ButtonSecondary, OnPress: func() { last.Set(tr.OverlayPressedSave()) }}}},
				// base.Tooltip: the bubble is the app's, and it reports.
				base.Tooltip{Open: func() bool { return pinned.Get() || hoveredOpen.Get() },
					OnOpenChange: func(open bool, reason base.Reason) {
						hoveredOpen.Set(open)
						state := "hidden"
						if open {
							state = "shown"
						}
						last.Set(tr.OverlayTooltip2() + state + " (" + reason.String() + ")")
					},
					Placement: core.PlaceBottom, Delay: 300e6,
					PopupStyle: core.Style{CornerRadius: 6, Padding: core.Symmetric(4, 8)}, PopupStyleFunc: ui.Filled(colors.Text),
					Trigger: core.Dynamic(func(*core.Scope) core.Node {
						label := tr.OverlayPinTheTooltip()
						if pinned.Get() {
							label = tr.OverlayUnpinTheTooltip()
						}
						return ui.Button{Text: label, Variant: ui.ButtonSecondary, OnPress: func() { toggled(pinned) }}
					}),
					Children: []core.Node{ui.Text(tr.OverlayShownWhilePinnedOrHovered(), 12, ui.Regular, colors.Background)}},
			}},
		}}
	}
}

// sheetCard is base.Sheet, a panel at the bottom over a backdrop.
func sheetCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		open := core.NewSignal(cx, false)
		return ui.Card{Title: tr.OverlaySheet(), Description: tr.OverlayAPanelAtTheBottom(), Children: []core.Node{
			ui.Button{Text: tr.OverlayShowTheSheet(), Variant: ui.ButtonSecondary, OnPress: func() { open.Set(true) }},
			base.Sheet{Open: open.Get, Label: tr.OverlaySheet(),
				OnOpenChange: func(next bool, reason base.Reason) {
					open.Set(next)
					state := "closed"
					if next {
						state = "opened"
					}
					last.Set(tr.OverlaySheet2() + state + " (" + reason.String() + ")")
				},
				PopupStyle: core.Style{CornerRadii: core.Corners{TopLeft: 20, TopRight: 20},
					Padding: core.Edges{Top: 12, Right: 12, Bottom: 32, Left: 12}, Gap: 2}, PopupStyleFunc: ui.Filled(colors.Surface),
				BackdropStyle: core.Style{Background: ui.Black(0.45)},
				Children: []core.Node{core.View{Style: core.Style{Padding: core.All(12), Gap: 12}, Children: []core.Node{
					ui.Text(tr.OverlaySheet(), 15, ui.Semibold, colors.Text),
					ui.Text(tr.OverlayTheBackdropEscapeAndBack(), 13, ui.Regular, colors.Muted),
					ui.Button{Text: tr.OverlayDone(), OnPress: func() { open.Set(false) }},
				}}}},
		}}
	}
}

// toastCard is the gallery's toasts, on base's ToastManager, ToastStack
// and Toast.
func toastCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		count := 0
		return ui.Card{Title: tr.OverlayToast(), Description: tr.OverlayTheGallerySToastsOn(), Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 8}, Children: []core.Node{
				ui.Button{Text: tr.OverlayCopyTheLink(), Variant: ui.ButtonSecondary, OnPress: func() {
					count++
					toast(toastContent{title: tr.OverlayCopied(), description: tr.OverlayLinkOnClipboard(count)})
				}},
				ui.Button{Text: tr.OverlayDeleteTheDraft(), Variant: ui.ButtonSecondary, OnPress: func() {
					toast(toastContent{id: "draft", title: tr.OverlayDraftDeleted(), action: tr.OverlayUndo(), close: "×", duration: 8 * time.Second,
						done: func(answer string) {
							if answer == tr.OverlayUndo() {
								last.Set(tr.OverlayToastUndone())
							} else {
								last.Set(tr.OverlayToastGone())
							}
						}})
				}},
				ui.Button{Text: tr.OverlayDismissAll(), Variant: ui.ButtonSecondary, OnPress: func() { shell.toasts.dismiss("") }},
			}},
			ui.Text(tr.OverlaySwipeOneAwayThePointer(), 13, ui.Regular, colors.Muted),
		}}
	}
}

// lastEventCard shows the last thing a demo reported.
func lastEventCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		return ui.Card{Title: tr.OverlayLastEvent(), Children: []core.Node{
			core.Text{ContentFunc: last.Get, TestID: "last-event", Style: core.Style{FontSize: 13, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Text)},
		}}
	}
}

package main

import (
	"fmt"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
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
		Children: []core.Node{ui.Textf(label, 14, ui.Medium, colors.Text)}}
}

// placementName is a placement as the demos report it.
func placementName(p core.Placement) string {
	return [...]string{"default", "bottom", "top", "left", "right"}[p]
}

// overlaysDemo is core.Overlay, which shows its children on top of the
// screen whatever clips them, and base's popups built on it: a popover, a
// tooltip (and the Tooltip every view has) and a sheet. Menus have a demo
// of their own.
func overlaysDemo(cx *core.Scope) core.Node {
	last := core.NewSignal(cx, "—")

	primitive := func(cx *core.Scope) core.Node {
		colors := ui.ColorsOf(cx)
		open := core.NewSignal(cx, false)
		anchor := &core.Ref{}
		return ui.Card{Icon: ui.IconOverlays, Title: "Overlay", Description: "A box that clips its content, with an overlay anchored inside it", Actions: []core.Node{ui.Badge{Text: "core"}}, Children: []core.Node{
			core.View{Style: core.Style{Height: core.Pt(56), Overflow: core.Hidden, CornerRadius: ui.RadiusInset,
				JustifyContent: core.JustifyCenter, Padding: core.Symmetric(0, 12)}, StyleFunc: ui.Filled(colors.Inset), Children: []core.Node{
				core.View{Ref: anchor, OnPress: func(core.PressEvent) { toggled(open) }, Role: core.RoleButton, Style: core.Style{AlignSelf: core.AlignStart},
					Children: []core.Node{trigger(cx, func() string {
						if open.Get() {
							return "Close"
						}
						return "Open the overlay"
					})}},
				core.Show{When: open.Get, Then: func(*core.Scope) core.Node {
					return core.Overlay{Anchor: anchor, Placement: core.PlaceBottom, Align: core.AlignStart, Offset: 6, Margin: 8,
						OnPressOutside: func(core.Point) { open.Set(false) }, OnCancel: func() { open.Set(false) },
						OnPosition: func(p core.OverlayPosition) { last.Set("overlay at " + placementName(p.Placement)) },
						StyleFunc:  popupStyle(cx), TestID: "overlay",
						Children: []core.Node{core.Text{Content: "Outside the clipped box, on top of the page.",
							Style: core.Style{FontSize: 14, Padding: core.All(6)}, StyleFunc: ui.Colored(colors.Text)}}}
				}},
			}},
			ui.Caption{Label: "Scroll the page: it follows its anchor"},
		}}
	}

	popovers := func(cx *core.Scope) core.Node {
		colors := ui.ColorsOf(cx)
		return ui.Card{Icon: ui.IconOverlays, Title: "Popover", Description: "Anchored to its trigger on every device", Children: []core.Node{
			base.Popover{Offset: 6, Margin: 8, PopupStyleFunc: popupStyle(cx), Label: "Show the popover",
				OnOpenChange: func(open bool, reason base.Reason) {
					state := "closed"
					if open {
						state = "opened"
					}
					last.Set("popover " + state + " (" + reason.String() + ")")
				},
				Trigger: trigger(cx, func() string { return "Show the popover" }),
				Children: []core.Node{core.View{Style: core.Style{Padding: core.All(8), Gap: 4, MaxWidth: core.Pt(260)}, Children: []core.Node{
					ui.Text("Popover", 15, ui.Semibold, colors.Text),
					ui.Text("A press outside, Escape or back closes it.", 13, ui.Regular, colors.Muted),
				}}}},
		}}
	}

	tooltips := func(cx *core.Scope) core.Node {
		colors := ui.ColorsOf(cx)
		hoveredOpen, pinned := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return ui.Card{Icon: ui.IconText, Title: "Tooltip", Description: ui.Hint(cx, "Tooltips follow a pointer: pin one to see it", "Rest the pointer on a button"), Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 8}, Children: []core.Node{
				// A view's Tooltip: the platform's bubble with the defaults.
				core.View{Tooltip: core.Text{Content: "Saves the document", StyleFunc: func() core.Style {
					return core.Style{FontSize: 12, Color: colors.Background(),
						Background: colors.Text(), CornerRadius: 6, Padding: core.Symmetric(4, 8)}
				}},
					Children: []core.Node{ui.Button{Text: "Save", Variant: ui.ButtonSecondary, OnPress: func() { last.Set("pressed Save") }}}},
				// base.Tooltip: the bubble is the app's, and it reports.
				base.Tooltip{Open: func() bool { return pinned.Get() || hoveredOpen.Get() },
					OnOpenChange: func(open bool, reason base.Reason) {
						hoveredOpen.Set(open)
						state := "hidden"
						if open {
							state = "shown"
						}
						last.Set("tooltip " + state + " (" + reason.String() + ")")
					},
					Placement: core.PlaceBottom, Delay: 300e6,
					PopupStyle: core.Style{CornerRadius: 6, Padding: core.Symmetric(4, 8)}, PopupStyleFunc: ui.Filled(colors.Text),
					Trigger: core.Dynamic(func(*core.Scope) core.Node {
						label := "Pin the tooltip"
						if pinned.Get() {
							label = "Unpin the tooltip"
						}
						return ui.Button{Text: label, Variant: ui.ButtonSecondary, OnPress: func() { toggled(pinned) }}
					}),
					Children: []core.Node{ui.Text("Shown while pinned or hovered", 12, ui.Regular, colors.Background)}},
			}},
		}}
	}

	sheets := func(cx *core.Scope) core.Node {
		colors := ui.ColorsOf(cx)
		open := core.NewSignal(cx, false)
		return ui.Card{Icon: ui.IconOverlays, Title: "Sheet", Description: "A panel at the bottom, over a backdrop", Children: []core.Node{
			ui.Button{Text: "Show the sheet", Variant: ui.ButtonSecondary, OnPress: func() { open.Set(true) }},
			base.Sheet{Open: open.Get, Label: "Sheet",
				OnOpenChange: func(next bool, reason base.Reason) {
					open.Set(next)
					state := "closed"
					if next {
						state = "opened"
					}
					last.Set("sheet " + state + " (" + reason.String() + ")")
				},
				PopupStyle: core.Style{CornerRadii: core.Corners{TopLeft: 20, TopRight: 20},
					Padding: core.Edges{Top: 12, Right: 12, Bottom: 32, Left: 12}, Gap: 2}, PopupStyleFunc: ui.Filled(colors.Surface),
				BackdropStyle: core.Style{Background: ui.Black(0.45)},
				Children: []core.Node{core.View{Style: core.Style{Padding: core.All(12), Gap: 12}, Children: []core.Node{
					ui.Text("Sheet", 17, ui.Semibold, colors.Text),
					ui.Text("The backdrop, Escape and back close it.", 14, ui.Regular, colors.Muted),
					ui.Button{Text: "Done", OnPress: func() { open.Set(false) }},
				}}}},
		}}
	}

	toasts := func(cx *core.Scope) core.Node {
		colors := ui.ColorsOf(cx)
		count := 0
		return ui.Card{Icon: ui.IconSystem, Title: "Toast", Description: "The gallery's toasts, on ToastManager, ToastStack and Toast", Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 8}, Children: []core.Node{
				ui.Button{Text: "Copy the link", Variant: ui.ButtonSecondary, OnPress: func() {
					count++
					toast(toastContent{title: "Copied", description: fmt.Sprintf("The link is on the clipboard (%d)", count)})
				}},
				ui.Button{Text: "Delete the draft", Variant: ui.ButtonSecondary, OnPress: func() {
					toast(toastContent{id: "draft", title: "Draft deleted", action: "Undo", close: "×", duration: 8 * time.Second,
						done: func(answer string) {
							if answer == "Undo" {
								last.Set("toast: undone")
							} else {
								last.Set("toast: gone")
							}
						}})
				}},
				ui.Button{Text: "Dismiss all", Variant: ui.ButtonSecondary, OnPress: func() { shell.toasts.dismiss("") }},
			}},
			ui.Text("Swipe one away; the pointer on one stops its time", 13, ui.Regular, colors.Muted),
		}}
	}

	return demoPage(cx, pageProps{}, primitive, popovers, tooltips, sheets, toasts, lastEventCard(last))
}

// lastEventCard shows the last thing a demo reported.
func lastEventCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		colors := ui.ColorsOf(cx)
		return ui.Card{Icon: ui.IconSystem, Title: "Last event", Children: []core.Node{
			core.Text{ContentFunc: last.Get, TestID: "last-event", Style: core.Style{FontSize: 13, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Text)},
		}}
	}
}

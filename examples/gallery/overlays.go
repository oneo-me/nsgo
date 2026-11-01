package main

import (
	"fmt"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// popupStyle is the popups' look: the gallery's, as base draws none.
func popupStyle(cx *core.Scope) func() core.Style {
	colors := colorsOf(cx)
	return func() core.Style {
		return finOf(cx).raised().over(core.Style{Background: colors.raised(), CornerRadius: radiusInset, BorderWidth: 1, BorderColor: colors.border(),
			Padding: core.All(6), MinWidth: core.Pt(180)})
	}
}

// trigger is a pressable chip that opens a popup.
func trigger(cx *core.Scope, label func() string) core.Node {
	colors := colorsOf(cx)
	fin := finOf(cx)
	return core.View{StyleFunc: func() core.Style {
		return fin.raised().over(core.Style{AlignSelf: core.AlignStart, Padding: core.Symmetric(0, 14), Height: core.Pt(36),
			JustifyContent: core.JustifyCenter, CornerRadius: radiusControl, Background: colors.raised(), Active: &core.Style{Background: colors.border()}})
	},
		Children: []core.Node{textf(label, 14, medium, colors.text)}}
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
		colors := colorsOf(cx)
		open := core.NewSignal(cx, false)
		anchor := &core.Ref{}
		return card(cx, cardProps{icon: iconOverlays, title: "Overlay", subtitle: "A box that clips its content, with an overlay anchored inside it", right: badge(cx, "core", toneNeutral)},
			core.View{Style: core.Style{Height: core.Pt(56), Overflow: core.Hidden, CornerRadius: radiusInset,
				JustifyContent: core.JustifyCenter, Padding: core.Symmetric(0, 12)}, StyleFunc: filled(colors.inset), Children: []core.Node{
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
							Style: core.Style{FontSize: 14, Padding: core.All(6)}, StyleFunc: colored(colors.text)}}}
				}},
			}},
			caption(cx, "Scroll the page: it follows its anchor", nil),
		)
	}

	popovers := func(cx *core.Scope) core.Node {
		colors := colorsOf(cx)
		return card(cx, cardProps{icon: iconOverlays, title: "Popover", subtitle: "Anchored to its trigger on every device"},
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
					text("Popover", 15, semibold, colors.text),
					text("A press outside, Escape or back closes it.", 13, regular, colors.muted),
				}}}},
		)
	}

	tooltips := func(cx *core.Scope) core.Node {
		colors := colorsOf(cx)
		hoveredOpen, pinned := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return card(cx, cardProps{icon: iconText, title: "Tooltip", subtitle: hint(cx, "Tooltips follow a pointer: pin one to see it", "Rest the pointer on a button")},
			core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 8}, Children: []core.Node{
				// A view's Tooltip: the platform's bubble with the defaults.
				core.View{Tooltip: core.Text{Content: "Saves the document", StyleFunc: func() core.Style {
					return core.Style{FontSize: 12, Color: colors.background(),
						Background: colors.text(), CornerRadius: 6, Padding: core.Symmetric(4, 8)}
				}},
					Children: []core.Node{button(buttonProps{label: "Save", variant: secondary}, func() { last.Set("pressed Save") })}},
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
					PopupStyle: core.Style{CornerRadius: 6, Padding: core.Symmetric(4, 8)}, PopupStyleFunc: filled(colors.text),
					Trigger: core.Dynamic(func(*core.Scope) core.Node {
						label := "Pin the tooltip"
						if pinned.Get() {
							label = "Unpin the tooltip"
						}
						return button(buttonProps{label: label, variant: secondary}, func() { toggled(pinned) })
					}),
					Children: []core.Node{text("Shown while pinned or hovered", 12, regular, colors.background)}},
			}},
		)
	}

	sheets := func(cx *core.Scope) core.Node {
		colors := colorsOf(cx)
		open := core.NewSignal(cx, false)
		return card(cx, cardProps{icon: iconOverlays, title: "Sheet", subtitle: "A panel at the bottom, over a backdrop"},
			button(buttonProps{label: "Show the sheet", variant: secondary}, func() { open.Set(true) }),
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
					Padding: core.Edges{Top: 12, Right: 12, Bottom: 32, Left: 12}, Gap: 2}, PopupStyleFunc: filled(colors.surface),
				BackdropStyle: core.Style{Background: black(0.45)},
				Children: []core.Node{core.View{Style: core.Style{Padding: core.All(12), Gap: 12}, Children: []core.Node{
					text("Sheet", 17, semibold, colors.text),
					text("The backdrop, Escape and back close it.", 14, regular, colors.muted),
					button(buttonProps{label: "Done"}, func() { open.Set(false) }),
				}}}},
		)
	}

	toasts := func(cx *core.Scope) core.Node {
		colors := colorsOf(cx)
		count := 0
		return card(cx, cardProps{icon: iconSystem, title: "Toast", subtitle: "The gallery's toasts, on ToastManager, ToastStack and Toast"},
			core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 8}, Children: []core.Node{
				button(buttonProps{label: "Copy the link", variant: secondary}, func() {
					count++
					toast(toastContent{title: "Copied", description: fmt.Sprintf("The link is on the clipboard (%d)", count)})
				}),
				button(buttonProps{label: "Delete the draft", variant: secondary}, func() {
					toast(toastContent{id: "draft", title: "Draft deleted", action: "Undo", close: "×", duration: 8 * time.Second,
						done: func(answer string) {
							if answer == "Undo" {
								last.Set("toast: undone")
							} else {
								last.Set("toast: gone")
							}
						}})
				}),
				button(buttonProps{label: "Dismiss all", variant: secondary}, func() { shell.toasts.dismiss("") }),
			}},
			text("Swipe one away; the pointer on one stops its time", 13, regular, colors.muted),
		)
	}

	return demoPage(cx, pageProps{}, primitive, popovers, tooltips, sheets, toasts, lastEventCard(last))
}

// lastEventCard shows the last thing a demo reported.
func lastEventCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		colors := colorsOf(cx)
		return card(cx, cardProps{icon: iconSystem, title: "Last event"},
			core.Text{ContentFunc: last.Get, TestID: "last-event", Style: core.Style{FontSize: 13, FontFamily: mono}, StyleFunc: colored(colors.text)})
	}
}

package main

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// The popups: each ui form under its base form, with their props to set.

var placements = options([]core.Placement{core.PlaceBottom, core.PlaceTop, core.PlaceLeft, core.PlaceRight}, "Bottom", "Top", "Left", "Right")

var aligns = options([]core.Align{core.AlignStart, core.AlignCenter, core.AlignEnd}, "Start", "Center", "End")

func dialogCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	open := core.NewSignal(cx, false)
	noEscape, noBackdrop, footer := core.NewSignal(cx, false), core.NewSignal(cx, false), core.NewSignal(cx, true)
	close := func() { open.Set(false) }
	return Playground{Title: "Dialog", Base: baseDialogForm(cx, noEscape, noBackdrop), Stage: func() core.Node {
		d := ui.Dialog{Open: open.Get, Title: tr.KitDialogTitle(), Description: tr.KitDialogNote(), Label: tr.KitDialogTitle(), TestID: "ui-dialog",
			NoCloseOnEscape: noEscape.Get(), NoCloseOnBackdropPress: noBackdrop.Get(),
			OnOpenChange: func(o bool, _ base.Reason) { open.Set(o) },
			Children:     []core.Node{ui.Text(tr.ControlsDeployedToProduction(), 13, ui.Regular, colors.Text)}}
		if footer.Get() {
			d.Footer = []core.Node{ui.Spacer{}, ui.Button{Text: tr.OverlayDone(), OnPress: close}}
		} else {
			d.Children = append(d.Children, ui.Button{Text: tr.OverlayDone(), OnPress: close})
		}
		return core.Group{ui.Button{Text: tr.KitOpen(), Variant: ui.ButtonSecondary, OnPress: func() { open.Set(true) }}, d}
	}, BaseKnobs: []core.Node{boolKnob("NoCloseOnEscape", noEscape), boolKnob("NoCloseOnBackdropPress", noBackdrop)}, Knobs: []core.Node{boolKnob("Footer", footer)}}
}

func alertDialogCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	open := core.NewSignal(cx, false)
	danger := core.NewSignal(cx, true)
	return Playground{Title: "AlertDialog", Base: baseAlertDialogForm(cx), Stage: func() core.Node {
		confirm := ui.AlertAction{Text: tr.OverlayDone(), OnPress: func() { open.Set(false) }}
		if danger.Get() {
			confirm.Color = colors.Danger
		}
		return core.Group{
			ui.Button{Text: tr.KitOpen(), Variant: ui.ButtonSecondary, OnPress: func() { open.Set(true) }},
			ui.AlertDialog{Open: open.Get, Title: tr.KitDialogTitle(), Description: tr.KitDialogNote(),
				OnOpenChange: func(o bool, _ base.Reason) { open.Set(o) },
				Actions:      []ui.AlertAction{{Text: tr.KitCancel(), Variant: ui.ButtonOutline, OnPress: func() { open.Set(false) }}, confirm}},
		}
	}, Knobs: []core.Node{boolKnob("Color", danger)}}
}

func popoverPlayground(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	placement := core.NewSignal(cx, core.PlaceBottom)
	align := core.NewSignal(cx, core.AlignCenter)
	return Playground{Title: "Popover", Base: basePopoverForm(cx, placement, align), Stage: func() core.Node {
		return ui.Popover{Trigger: ui.Button{Text: tr.OverlayShowThePopover(), Variant: ui.ButtonSecondary}, Placement: placement.Get(), Align: align.Get(),
			Label:    tr.OverlayPopover(),
			Children: []core.Node{ui.Text(tr.OverlayAPressOutsideEscapeOr(), 13, ui.Regular, colors.Muted)}}
	}, BaseKnobs: []core.Node{choiceKnob("Placement", placement, placements...), choiceKnob("Align", align, aligns...)}}
}

func tooltipPlayground(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	placement := core.NewSignal(cx, core.PlaceTop)
	return Playground{Title: "Tooltip", Base: baseTooltipForm(cx, placement), Stage: func() core.Node {
		return ui.Tooltip{Text: tr.OverlaySavesTheDocument(), Placement: placement.Get(), Trigger: ui.Button{Text: tr.OverlaySave(), Variant: ui.ButtonSecondary, OnPress: func() {}}}
	}, BaseKnobs: []core.Node{choiceKnob("Placement", placement, placements...)}}
}

// menuItems are the items of the Menu and ContextMenu playgrounds.
func menuItems(cx *core.Scope, last *core.Signal[string], checked, disabled *core.Signal[bool]) []core.Node {
	tr := locales.For(cx)
	choose := func(name string) func() { return func() { last.Set("Menu: " + name) } }
	return []core.Node{
		ui.MenuItem{Text: tr.TerminalCopy(), Icon: lucide.Copy, Shortcut: "mod+c", OnSelect: choose(tr.TerminalCopy())},
		ui.MenuItem{Text: tr.TerminalPaste(), Disabled: disabled.Get, OnSelect: choose(tr.TerminalPaste())},
		ui.MenuItem{Text: tr.LibStarred(), Checked: checked.Get, NoCloseOnSelect: true, OnSelect: func() { toggled(checked) }},
		ui.Submenu{Text: tr.KitMore(), Back: tr.LibBack(), Children: []core.Node{
			ui.MenuItem{Text: tr.LibApple(), OnSelect: choose(tr.LibApple())},
			ui.MenuItem{Text: tr.LibBanana(), OnSelect: choose(tr.LibBanana())},
		}},
		ui.MenuSeparator{},
		ui.MenuItem{Text: tr.OverlayDeleteTheDraft(), Variant: ui.MenuItemDanger, OnSelect: choose(tr.OverlayDeleteTheDraft())},
	}
}

func menuPlayground(cx *core.Scope) core.Node {
	last := core.NewSignal(cx, "")
	tr := locales.For(cx)
	placement := core.NewSignal(cx, core.PlaceBottom)
	align := core.NewSignal(cx, core.AlignStart)
	checked, disabled := core.NewSignal(cx, true), core.NewSignal(cx, true)
	return Playground{Title: "Menu · Submenu", Event: last.Get, Base: baseMenuForm(cx, last, placement, align), Stage: func() core.Node {
		return ui.Menu{Trigger: ui.Button{Text: tr.KitOpen(), Variant: ui.ButtonSecondary}, Placement: placement.Get(), Align: align.Get(), Label: tr.DemoMenu(), TestID: "ui-menu",
			Children: menuItems(cx, last, checked, disabled)}
	}, BaseKnobs: []core.Node{choiceKnob("Placement", placement, placements...), choiceKnob("Align", align, aligns...)},
		Knobs: []core.Node{boolKnob("Checked", checked), boolKnob("Disabled", disabled)}}
}

func contextMenuCard(cx *core.Scope) core.Node {
	last := core.NewSignal(cx, "")
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	checked, off, disabled := core.NewSignal(cx, true), core.NewSignal(cx, false), core.NewSignal(cx, true)
	return Playground{Title: "ContextMenu", Event: last.Get, Fill: true, Base: baseContextMenuForm(cx, last, off), Stage: func() core.Node {
		return ui.ContextMenu{Disabled: off.Get(), Children: menuItems(cx, last, checked, disabled),
			Trigger: core.View{Style: core.Style{Height: core.Pt(64), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
				Children: []core.Node{ui.Text(Hint(cx, tr.KitLongPressHere(), tr.KitRightClickHere()), 13, ui.Regular, colors.Muted)}}}
	}, BaseKnobs: []core.Node{boolKnob("Disabled", off)}}
}

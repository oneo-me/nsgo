package main

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// What a press acts on, Pagination and OtpInput: each ui form under its base
// form, with their props to set.

var buttonVariants = []ui.ButtonVariant{ui.ButtonPrimary, ui.ButtonSecondary, ui.ButtonAccent, ui.ButtonGhost}

func buttonCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		selected, icon, iconOnly, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false), core.NewSignal(cx, false), core.NewSignal(cx, false)
		button := func() ui.Button {
			b := ui.Button{Text: tr.OverlaySave(), Disabled: disabled.Get, Label: tr.OverlaySave(), OnPress: pressed(cx, last, "Button")}
			if icon.Get() || iconOnly.Get() {
				b.Icon = lucide.Send
			}
			if iconOnly.Get() {
				b.Text = ""
			}
			return b
		}
		return Playground{Title: "Button", Base: baseButtonForm(cx, last, selected, disabled), Stage: func() core.Node {
			var buttons []core.Node
			for _, v := range buttonVariants {
				b := button()
				b.Variant = v
				if v == ui.ButtonPrimary {
					b.TestID = "button"
				}
				buttons = append(buttons, b)
			}
			return stageRow(buttons...)
		}, Sizes: func() core.Node {
			var buttons []core.Node
			for _, size := range sizes {
				b := button()
				b.Size = size
				buttons = append(buttons, b)
			}
			return stageRow(buttons...)
		}, BaseKnobs: []core.Node{boolKnob("Selected", selected), boolKnob("Disabled", disabled)}, Knobs: []core.Node{boolKnob("Icon", icon), boolKnob("Text = \"\"", iconOnly)}}
	}
}

func iconButtonCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		size := core.NewSignal(cx, 32.0)
		selected, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "IconButton · RoundButton", Stage: func() core.Node {
			s := core.Points(size.Get())
			return stageRow(
				ui.IconButton{Icon: lucide.Star, Label: tr.LibStarred(), Size: s, Selected: selected.Get, Disabled: disabled.Get, OnPress: pressed(cx, last, "IconButton")},
				ui.RoundButton{Icon: lucide.Plus, Label: tr.LibMore(), Size: s, Variant: ui.RoundButtonPrimary, OnPress: pressed(cx, last, "RoundButton")},
				ui.RoundButton{Icon: lucide.Plus, Label: tr.LibMore(), Size: s, Variant: ui.RoundButtonRaised, OnPress: pressed(cx, last, "RoundButton")},
			)
		}, Knobs: []core.Node{
			numberKnob("Size", size, 24, 56, 4), boolKnob("Selected", selected), boolKnob("Disabled", disabled),
		}}
	}
}

func toggleButtonCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		on, icon, disabled := core.NewSignal(cx, true), core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "ToggleButton", Base: baseToggleForm(cx, on, disabled), Stage: func() core.Node {
			b := ui.ToggleButton{Text: tr.LibBold(), Label: tr.LibBold(), On: on.Get, OnChange: on.Set, Disabled: disabled.Get}
			if icon.Get() {
				b.Text, b.Icon = "", lucide.Star
			}
			return b
		}, BaseKnobs: []core.Node{boolKnob("Pressed", on), boolKnob("Disabled", disabled)}, Knobs: []core.Node{boolKnob("Icon", icon)}}
	}
}

func toggleGroupCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		bold, italic, disabled := core.NewSignal(cx, true), core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "ToggleGroup", Base: baseToggleGroupForm(cx, bold, italic, disabled), Stage: func() core.Node {
			return ui.ToggleGroup{Label: tr.LibFormatting(), Disabled: disabled.Get, Children: []core.Node{
				ui.ToggleButton{Text: tr.LibBold(), On: bold.Get, OnChange: bold.Set},
				ui.ToggleButton{Text: tr.LibItalic(), On: italic.Get, OnChange: italic.Set},
			}}
		}, BaseKnobs: []core.Node{boolKnob("Disabled", disabled)}}
	}
}

func toolbarCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		axis := core.NewSignal(cx, base.Horizontal)
		bold, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "Toolbar", Base: baseToolbarForm(cx, last, axis, bold, disabled), Stage: func() core.Node {
			return ui.Toolbar{Axis: axis.Get(), Label: tr.LibFormatting(), Disabled: disabled.Get, Children: []core.Node{
				ui.ToolbarGroup{Label: tr.LibHistory(), Children: []core.Node{
					ui.IconButton{Icon: lucide.Undo2, Label: tr.LibUndo(), OnPress: pressed(cx, last, tr.LibUndo())},
					ui.IconButton{Icon: lucide.Redo2, Label: tr.LibRedo(), OnPress: pressed(cx, last, tr.LibRedo())},
				}},
				ui.ToolbarSeparator{},
				ui.ToggleButton{Icon: lucide.Bold, Label: tr.LibBold(), On: bold.Get, OnChange: bold.Set},
			}}
		}, BaseKnobs: []core.Node{choiceKnob("Axis", axis, options([]base.Axis{base.Horizontal, base.Vertical}, "Horizontal", "Vertical")...), boolKnob("Disabled", disabled)}}
	}
}

func linkCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		disabled := core.NewSignal(cx, false)
		return Playground{Title: "Link", Base: baseLinkForm(cx, last, disabled), Stage: func() core.Node {
			return ui.Link{Text: tr.LibReadDocs(), Href: "/docs", OpenWith: base.LinkOpenApp, Disabled: disabled.Get, OnOpen: last.Set}
		}, BaseKnobs: []core.Node{boolKnob("Disabled", disabled)}}
	}
}

func paginationCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		page, total := core.NewSignal(cx, 5.0), core.NewSignal(cx, 20.0)
		compact, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "Pagination", Base: basePaginationForm(cx, page, total, compact, disabled), Stage: func() core.Node {
			return ui.Pagination{Page: func() int { return int(page.Get()) }, TotalPages: func() int { return int(total.Get()) }, VisiblePages: 7,
				Compact: compact.Get, Disabled: disabled.Get, OnChange: func(p int) { page.Set(float64(p)) },
				Label: tr.LibPages(), PreviousLabel: tr.LibPreviousPage(), NextLabel: tr.LibNextPage()}
		}, BaseKnobs: []core.Node{numberKnob("TotalPages", total, 1, 100, 1), boolKnob("Compact", compact), boolKnob("Disabled", disabled)}}
	}
}

func otpInputCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		masked, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "OtpInput", Base: baseOtpInputForm(cx, last, masked, disabled), Stage: func() core.Node {
			return ui.OtpInput{Length: 6, Groups: 2, Masked: masked.Get(), Disabled: disabled.Get, Label: tr.LibOneTimeCode(), OnComplete: last.Set}
		}, BaseKnobs: []core.Node{boolKnob("Masked", masked), boolKnob("Disabled", disabled)}}
	}
}

func holdPlayground(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		seconds := core.NewSignal(cx, 1.2)
		confirmed, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "HoldToConfirm", UIOnly: true, Stage: func() core.Node {
			var holds []core.Node
			for _, v := range []ui.HoldVariant{ui.HoldAccent, ui.HoldDanger, ui.HoldNeutral} {
				holds = append(holds, ui.HoldToConfirm{Text: tr.LibHoldToDelete(), ConfirmedText: tr.LibDeleted(), Variant: v, Hint: tr.LibHold(),
					Duration: secondsOf(seconds.Get()), Disabled: disabled.Get, Confirmed: confirmed.Get,
					OnConfirm: func() { confirmed.Set(true); last.Set(tr.LibDeleted()) }})
			}
			return stageRow(holds...)
		}, Knobs: []core.Node{numberKnob("Duration", seconds, 0.4, 3, 0.2), boolKnob("Confirmed", confirmed), boolKnob("Disabled", disabled)}}
	}
}

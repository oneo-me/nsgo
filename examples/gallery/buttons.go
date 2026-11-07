package main

import (
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// The Buttons demo: what a press acts on, each with its props to set.

var buttonVariants = options([]ui.ButtonVariant{ui.ButtonPrimary, ui.ButtonSecondary, ui.ButtonAccent, ui.ButtonGhost},
	"Primary", "Secondary", "Accent", "Ghost")

func buttonCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		variant := core.NewSignal(cx, ui.ButtonPrimary)
		small, icon, iconOnly, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false), core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "Button", Stage: func() core.Node {
			b := ui.Button{Text: tr.OverlaySave(), Variant: variant.Get(), Small: small.Get(), Disabled: disabled.Get, Label: tr.OverlaySave(),
				TestID: "button", OnPress: pressed(cx, last, "Button")}
			if icon.Get() || iconOnly.Get() {
				b.Icon = lucide.Send
			}
			if iconOnly.Get() {
				b.Text = ""
			}
			return b
		}, Knobs: []core.Node{
			choiceKnob("Variant", variant, buttonVariants...),
			boolKnob("Small", small), boolKnob("Icon", icon), boolKnob("Text = \"\"", iconOnly), boolKnob("Disabled", disabled),
		}}
	}
}

func iconButtonCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		variant := core.NewSignal(cx, ui.ButtonSecondary)
		size := core.NewSignal(cx, 32.0)
		selected, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "IconButton · RoundButton", Stage: func() core.Node {
			s := core.Points(size.Get())
			return ui.HStack{Gap: 16, Children: []core.Node{
				ui.IconButton{Icon: lucide.Star, Label: tr.LibStarred(), Size: s, Selected: selected.Get, Disabled: disabled.Get, OnPress: pressed(cx, last, "IconButton")},
				ui.RoundButton{Icon: lucide.Plus, Label: tr.LibMore(), Size: s, Variant: variant.Get(), OnPress: pressed(cx, last, "RoundButton")},
			}}
		}, Knobs: []core.Node{
			numberKnob("Size", size, 24, 56, 4), choiceKnob("Variant", variant, buttonVariants...),
			boolKnob("Selected", selected), boolKnob("Disabled", disabled),
		}}
	}
}

func toggleButtonCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		on, icon, disabled := core.NewSignal(cx, true), core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "ToggleButton", Stage: func() core.Node {
			b := ui.ToggleButton{Text: tr.LibBold(), Label: tr.LibBold(), On: on.Get, OnChange: on.Set, Disabled: disabled.Get}
			if icon.Get() {
				b.Text, b.Icon = "", lucide.Star
			}
			return b
		}, Knobs: []core.Node{boolKnob("On", on), boolKnob("Icon", icon), boolKnob("Disabled", disabled)}}
	}
}

func toggleGroupCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		bold, italic, disabled := core.NewSignal(cx, true), core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "ToggleGroup", Stage: func() core.Node {
			return ui.ToggleGroup{Label: tr.LibFormatting(), Disabled: disabled.Get, Children: []core.Node{
				ui.ToggleButton{Text: tr.LibBold(), On: bold.Get, OnChange: bold.Set},
				ui.ToggleButton{Text: tr.LibItalic(), On: italic.Get, OnChange: italic.Set},
			}}
		}, Knobs: []core.Node{boolKnob("Disabled", disabled)}}
	}
}

func toolbarCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		bold, vertical, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "Toolbar", Stage: func() core.Node {
			axis := base.Horizontal
			if vertical.Get() {
				axis = base.Vertical
			}
			return ui.Toolbar{Axis: axis, Label: tr.LibFormatting(), Disabled: disabled.Get, Children: []core.Node{
				ui.ToolbarGroup{Label: tr.LibHistory(), Children: []core.Node{
					ui.IconButton{Icon: lucide.Undo2, Label: tr.LibUndo(), OnPress: pressed(cx, last, tr.LibUndo())},
					ui.IconButton{Icon: lucide.Redo2, Label: tr.LibRedo(), OnPress: pressed(cx, last, tr.LibRedo())},
				}},
				ui.ToolbarSeparator{},
				ui.ToggleButton{Icon: lucide.Bold, Label: tr.LibBold(), On: bold.Get, OnChange: bold.Set},
			}}
		}, Knobs: []core.Node{boolKnob("Vertical", vertical), boolKnob("Disabled", disabled)}}
	}
}

func linkCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		disabled := core.NewSignal(cx, false)
		return Playground{Title: "Link", Stage: func() core.Node {
			return ui.Link{Text: tr.LibReadDocs(), Href: "/docs", OpenWith: base.LinkOpenApp, Disabled: disabled.Get, OnOpen: last.Set}
		}, Knobs: []core.Node{boolKnob("Disabled", disabled)}}
	}
}

func paginationCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		page, total := core.NewSignal(cx, 5.0), core.NewSignal(cx, 20.0)
		compact, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "Pagination", Stage: func() core.Node {
			return ui.Pagination{Page: func() int { return int(page.Get()) }, TotalPages: func() int { return int(total.Get()) }, VisiblePages: 7,
				Compact: compact.Get, Disabled: disabled.Get, OnChange: func(p int) { page.Set(float64(p)) },
				Label: tr.LibPages(), PreviousLabel: tr.LibPreviousPage(), NextLabel: tr.LibNextPage()}
		}, Knobs: []core.Node{numberKnob("TotalPages", total, 1, 100, 1), boolKnob("Compact", compact), boolKnob("Disabled", disabled)}}
	}
}

func otpInputCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		masked, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "OtpInput", Stage: func() core.Node {
			return ui.OtpInput{Length: 6, Groups: 2, Masked: masked.Get(), Disabled: disabled.Get, Label: tr.LibOneTimeCode(), OnComplete: last.Set}
		}, Knobs: []core.Node{boolKnob("Masked", masked), boolKnob("Disabled", disabled)}}
	}
}

func holdPlayground(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		tone := core.NewSignal(cx, ui.HoldDanger)
		seconds := core.NewSignal(cx, 1.2)
		confirmed, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "HoldToConfirm", Stage: func() core.Node {
			return ui.HoldToConfirm{Text: tr.LibHoldToDelete(), ConfirmedText: tr.LibDeleted(), Tone: tone.Get(), Hint: tr.LibHold(),
				Duration: secondsOf(seconds.Get()), Disabled: disabled.Get, Confirmed: confirmed.Get,
				OnConfirm: func() { confirmed.Set(true); last.Set(tr.LibDeleted()) }}
		}, Knobs: []core.Node{
			choiceKnob("Tone", tone, options([]ui.HoldTone{ui.HoldAccent, ui.HoldDanger, ui.HoldNeutral}, "Accent", "Danger", "Neutral")...),
			numberKnob("Duration", seconds, 0.4, 3, 0.2), boolKnob("Confirmed", confirmed), boolKnob("Disabled", disabled),
		}}
	}
}

func floatingBarCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		count := core.NewSignal(cx, 3.0)
		icons := []func() *core.SVG{lucide.Copy, lucide.Send, lucide.Star, lucide.Trash, lucide.Menu}
		return Playground{Title: "FloatingBar", Stage: func() core.Node {
			var children []core.Node
			for _, icon := range icons[:int(count.Get())] {
				children = append(children, ui.IconButton{Icon: icon, Label: tr.KitAction(), Size: 30, OnPress: pressed(cx, last, "FloatingBar")})
			}
			return ui.FloatingBar{Label: tr.KitAction(), Children: children}
		}, Knobs: []core.Node{numberKnob("Children", count, 1, 5, 1)}}
	}
}

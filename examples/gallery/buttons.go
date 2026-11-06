package main

import (
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

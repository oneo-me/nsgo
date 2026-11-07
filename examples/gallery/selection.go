package main

import (
	"fmt"

	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// What picks a value: each ui form under its base form, with their props to
// set.

func checkboxCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		checked, partly, disabled := core.NewSignal(cx, true), core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "Checkbox", Base: baseCheckboxForm(cx, checked, partly, disabled), Stage: func() core.Node {
			return ui.Checkbox{Text: tr.LibStarred(), Checked: checked.Get, Indeterminate: partly.Get, Disabled: disabled.Get,
				OnChange: func(v bool) { checked.Set(v); last.Set(fmt.Sprintf("Checkbox: %s", onOff(tr, v))) }}
		}, BaseKnobs: []core.Node{boolKnob("Checked", checked), boolKnob("Indeterminate", partly), boolKnob("Disabled", disabled)}}
	}
}

func radioCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		picked := core.NewSignal(cx, 0)
		horizontal, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "RadioGroup · Radio", Base: baseRadioForm(cx, picked, disabled), Stage: func() core.Node {
			var radios []core.Node
			for i, name := range []string{tr.LibApple(), tr.LibBanana(), tr.LibCherry()} {
				radios = append(radios, ui.Radio{Text: name, Disabled: disabled.Get, Checked: func() bool { return picked.Get() == i },
					OnChange: func(bool) { picked.Set(i); last.Set("Radio: " + name) }})
			}
			return ui.RadioGroup{Label: tr.LibFruit(), Horizontal: horizontal.Get(), Children: radios}
		}, BaseKnobs: []core.Node{boolKnob("Disabled", disabled)}, Knobs: []core.Node{boolKnob("Horizontal", horizontal)}}
	}
}

func switchCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		checked, disabled := core.NewSignal(cx, true), core.NewSignal(cx, false)
		return Playground{Title: "Switch", Base: baseSwitchForm(cx, checked, disabled), Stage: func() core.Node {
			var switches []core.Node
			for _, size := range sizes {
				switches = append(switches, ui.Switch{Label: tr.LibStarred(), Checked: checked.Get, Size: size,
					OnChange: func(v bool) { checked.Set(v); last.Set(fmt.Sprintf("Switch: %s", onOff(tr, v))) }})
			}
			return stageRow(switches...)
		}, BaseKnobs: []core.Node{boolKnob("Checked", checked), boolKnob("Disabled", disabled)}}
	}
}

func selectPlayground(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		value := core.NewSignal(cx, "")
		searchable, cleanable := core.NewSignal(cx, false), core.NewSignal(cx, false)
		choices := []ui.Option[string]{{Value: "apple", Label: tr.LibApple()}, {Value: "banana", Label: tr.LibBanana()}, {Value: "cherry", Label: tr.LibCherry()}}
		return Playground{Title: "Select", Base: baseSelectForm(cx, last, value, searchable, cleanable), Stage: func() core.Node {
			return core.Dynamic(func(*core.Scope) core.Node {
				s := ui.Select[string]{Label: tr.LibFruit(), Placeholder: tr.LibPickFruit(), Options: choices, Value: value.Get, Searchable: searchable.Get(),
					OnChange: func(v string) { value.Set(v); last.Set("Select: " + v) }}
				if cleanable.Get() {
					s.OnClear = func() { value.Set(""); last.Set("Select: \"\"") }
				}
				return s
			})
		}, BaseKnobs: []core.Node{choiceKnob("Value", value, append([]ui.Option[string]{{Value: "", Label: "\"\""}}, choices...)...),
			boolKnob("Searchable", searchable), boolKnob("Cleanable", cleanable)}}
	}
}

func sliderPlayground(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		value, high, step := core.NewSignal(cx, 30.0), core.NewSignal(cx, 70.0), core.NewSignal(cx, 1.0)
		ranged, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "Slider", Fill: true, Base: baseSliderForm(cx, value, high, step, ranged, disabled), Stage: func() core.Node {
			s := ui.Slider{Label: tr.LibVolume(), LowLabel: tr.LibMinimum(), HighLabel: tr.LibMaximum(), Step: step.Get(), Disabled: disabled.Get,
				Value: value.Get, OnChange: func(v float64) { value.Set(v); last.Set(fmt.Sprintf("Slider: %v", v)) }}
			if ranged.Get() {
				s.High, s.OnHighChange = high.Get, high.Set
			}
			return s
		}, BaseKnobs: []core.Node{numberKnob("Step", step, 1, 25, 1), boolKnob("Range", ranged), boolKnob("Disabled", disabled)}}
	}
}

func ratingCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		value := core.NewSignal(cx, 3)
		most := core.NewSignal(cx, 5.0)
		readonly, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "Rating", UIOnly: true, Stage: func() core.Node {
			return ui.Rating{Label: tr.KitRating(), Value: value.Get, Max: int(most.Get()), Readonly: readonly.Get(), Disabled: disabled.Get,
				OnChange: func(v int) { value.Set(v); last.Set(fmt.Sprintf("Rating: %d", v)) }}
		}, Knobs: []core.Node{numberKnob("Max", most, 3, 10, 1), boolKnob("Readonly", readonly), boolKnob("Disabled", disabled)}}
	}
}

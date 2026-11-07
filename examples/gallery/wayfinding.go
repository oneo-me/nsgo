package main

import (
	"fmt"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// The ui forms of what finds and folds content, with their props to set.

func tabsCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		selected := core.NewSignal(cx, 0)
		variant := core.NewSignal(cx, ui.TabsSegmented)
		equal, icons, content, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false), core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "Tabs", Fill: true, Stage: func() core.Node {
			choices := []ui.Option[int]{{Value: 0, Label: tr.ControlsOverview()}, {Value: 1, Label: tr.ControlsActivity()}, {Value: 2, Label: tr.ControlsBilling(), Disabled: disabled.Get}}
			if icons.Get() {
				choices[0].Icon, choices[1].Icon, choices[2].Icon = lucide.LayoutGrid, lucide.List, lucide.File
			}
			if content.Get() {
				for i := range choices {
					choices[i].Content = countedTab(cx, choices[i].Label, func() int { return (i + 1) * 4 })
				}
			}
			return ui.Tabs[int]{Variant: variant.Get(), Equal: equal.Get(), Options: choices, Value: selected.Get, Label: tr.DemoTabs(),
				OnChange: func(i int) { selected.Set(i); last.Set(fmt.Sprintf("Tabs: %d", i)) }}
		}, Knobs: []core.Node{
			choiceKnob("Variant", variant, options([]ui.TabsVariant{ui.TabsSegmented, ui.TabsUnderline}, "Segmented", "Underline")...),
			boolKnob("Equal", equal), boolKnob("Icon", icons), boolKnob("Content", content), boolKnob("Disabled", disabled),
		}}
	}
}

// countedTab is a tab's content: its label and a quiet count.
func countedTab(cx *core.Scope, label string, count func() int) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 5}, Children: []core.Node{
		core.Text{Content: label, MaxLines: 1},
		ui.Textf(func() string { return fmt.Sprint(count()) }, 12, ui.Regular, ui.ColorsOf(cx).Faint),
	}}
}

func stepperPlayground(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		current := core.NewSignal(cx, 1.0)
		vertical, compact, describe, failed := core.NewSignal(cx, false), core.NewSignal(cx, false), core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "Stepper", Fill: true, Stage: func() core.Node {
			steps := checkoutSteps(tr)
			if failed.Get() {
				steps[1].Error = tr.LibInstallError()
			}
			return ui.Stepper{Label: tr.LibCheckout(), Steps: steps, Current: func() int { return int(current.Get()) }, Vertical: vertical.Get(), Compact: compact.Get(),
				CurrentDescription: describe.Get(), CompleteLabel: tr.LibOrderPlaced(), StatusText: stepStatus(tr),
				OnSelect: func(i int) { current.Set(float64(i)); last.Set(steps[i].Label) }}
		}, Knobs: []core.Node{
			numberKnob("Current", current, 0, 4, 1), boolKnob("Vertical", vertical), boolKnob("Compact", compact),
			boolKnob("CurrentDescription", describe), boolKnob("Error", failed),
		}}
	}
}

func breadcrumbsCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		most := core.NewSignal(cx, 0.0)
		path := []string{tr.ControlsApps(), tr.ControlsPhotos(), "2026", "Lisbon", "IMG_0042.heic"}
		return Playground{Title: "Breadcrumbs", Fill: true, Stage: func() core.Node {
			return ui.Breadcrumbs{Items: path, MaxItems: int(most.Get()), MoreLabel: tr.KitMore(),
				OnSelect: func(i int) { last.Set("Breadcrumbs: " + path[i]) }}
		}, Knobs: []core.Node{numberKnob("MaxItems", most, 0, 5, 1)}}
	}
}

func accordionCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		open := core.NewSignalFunc(cx, map[int]bool{0: true}, func(a, b map[int]bool) bool { return false })
		single, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false)
		section := func(text, body string, off func() bool) ui.AccordionItem {
			return ui.AccordionItem{Text: text, Disabled: off, Content: ui.Text(body, 13, ui.Regular, colors.Muted)}
		}
		return Playground{Title: "Accordion", Fill: true, Stage: func() core.Node {
			return ui.Accordion{Open: func(i int) bool { return open.Get()[i] },
				OnOpenChange: func(i int, on bool) {
					open.Update(func(m map[int]bool) map[int]bool {
						if single.GetUntracked() {
							clear(m)
						}
						m[i] = on
						return m
					})
					last.Set(fmt.Sprintf("Accordion: %d %s", i, onOff(tr, on)))
				},
				Items: []ui.AccordionItem{
					section(tr.ControlsOverview(), tr.ControlsDeployedToProduction(), nil),
					section(tr.ControlsActivity(), tr.ControlsInvoicePaid(), nil),
					section(tr.ControlsBilling(), tr.ControlsAMemberJoined(), disabled.Get),
				}}
		}, Knobs: []core.Node{boolKnob("One open", single), boolKnob("Disabled", disabled)}}
	}
}

func collapsiblePlayground(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		open, disabled := core.NewSignal(cx, true), core.NewSignal(cx, false)
		return Playground{Title: "Collapsible", Fill: true, Stage: func() core.Node {
			return ui.Collapsible{Text: tr.ControlsActivity(), Open: open.Get, Disabled: disabled.Get,
				OnOpenChange: func(on bool) { open.Set(on); last.Set(fmt.Sprintf("Collapsible: %s", onOff(tr, on))) },
				Content:      ui.Text(tr.ControlsDeployedToProduction(), 13, ui.Regular, colors.Muted)}
		}, Knobs: []core.Node{boolKnob("Open", open), boolKnob("Disabled", disabled)}}
	}
}

package main

import (
	"fmt"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
)

func always() bool { return true }

// accessibilityDemo is what screen readers read beyond roles and labels:
// values and hints, expanded states, actions, groups, hidden elements,
// live regions and focus. Turn on VoiceOver or TalkBack to hear it. NSH's
// gallery shows accessibility within its other demos; NSGo gives it a page.
func accessibilityDemo(cx *core.Scope) core.Node {
	values := func(cx *core.Scope) core.Node {
		n := core.NewSignal(cx, 2)
		adjust := func(by int) { n.Update(func(v int) int { return min(max(v+by, 0), 10) }) }
		step := func(name ui.IconName, by int, label string) core.Node {
			// The buttons are for the pointer; the screen reader adjusts
			// the quantity itself.
			return core.View{AccessibilityElementsHidden: always, Children: []core.Node{ui.RoundButton{Icon: name, Label: label, Variant: ui.ButtonSecondary, Size: 36, OnPress: func() { adjust(by) }}}}
		}
		return ui.Card{Icon: ui.IconAccess, Title: "Values and hints", Description: "An adjustable element: swipe up or down", Children: []core.Node{
			core.View{Role: core.RoleAdjustable, Label: "Quantity", Hint: "Swipe up or down to adjust", TestID: "a11y-quantity",
				AccessibilityValue: func() core.AccessibilityValue {
					return core.AccessibilityValue{Max: 10, Now: float32(n.Get()), Text: fmt.Sprintf("%d items", n.Get())}
				},
				AccessibilityActions: []core.AccessibilityAction{{Name: core.ActionIncrement}, {Name: core.ActionDecrement}},
				OnAccessibilityAction: func(name string) {
					if name == core.ActionIncrement {
						adjust(1)
					} else {
						adjust(-1)
					}
				},
				Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 16},
				Children: []core.Node{
					step(ui.IconMinus, -1, "Fewer"),
					ui.Figure{Value: func() string { return itoa(n.Get()) }, Unit: "items"},
					step(ui.IconPlus, 1, "More"),
				}},
			ui.Meter{Value: func() float64 { return float64(n.Get()) / 10 }, Tone: ui.ToneAccent},
			ui.APILine{Label: "A range value and the increment and decrement actions", Names: "RoleAdjustable"},
		}}
	}

	expanded := func(cx *core.Scope) core.Node {
		colors := ui.ColorsOf(cx)
		open := core.NewSignal(cx, false)
		return ui.Card{Icon: ui.IconAccess, Title: "Expanded state", Description: "Screen readers say whether it is expanded", Children: []core.Node{
			base.Button{Label: "Shipping details", Expanded: open.Get, TestID: "a11y-disclosure", OnPress: func(core.PressEvent) { toggled(open) },
				Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, MinHeight: core.Pt(44), Active: &core.Style{Opacity: core.Alpha(0.7)}},
				Children: []core.Node{
					core.Text{Content: "Shipping details", Style: core.Style{FlexGrow: 1, FontSize: 15}, StyleFunc: ui.Colored(colors.Text)},
					core.View{StyleFunc: func() core.Style {
						s := core.Style{Transition: core.Transition{Duration: ms(150), Properties: core.TransitionTransform}}
						if open.Get() {
							s.Transform.Rotate = 90
						}
						return s
					}, Children: []core.Node{ui.Icon(ui.IconChevron, 16, colors.Muted)}},
				}},
			core.Show{When: open.Get, Then: func(*core.Scope) core.Node { return ui.Note{Text: "Ships in two days from Lisbon."} }},
			ui.APILine{Label: "A disclosure", Names: "Expanded"},
		}}
	}

	actions := func(cx *core.Scope) core.Node {
		colors := ui.ColorsOf(cx)
		fin := ui.FinOf(cx)
		last := core.NewSignal(cx, "none yet")
		return ui.Card{Icon: ui.IconAccess, Title: "Actions", Description: "In the screen reader's actions menu", Children: []core.Node{
			core.View{Accessible: true, TestID: "a11y-message",
				AccessibilityActions: []core.AccessibilityAction{
					{Name: "archive", Label: "Archive"}, {Name: "flag", Label: "Flag"},
					{Name: core.ActionActivate}, {Name: core.ActionEscape}, {Name: core.ActionMagicTap},
				},
				OnAccessibilityAction: last.Set,
				StyleFunc: func() core.Style {
					return fin.Raised().Over(core.Style{Background: colors.Raised(), CornerRadius: ui.RadiusControl, BorderWidth: 1, BorderColor: colors.Border(), Padding: core.All(12), Gap: 4})
				},
				Children: []core.Node{ui.Text("Ana", 15, ui.Semibold, colors.Text), ui.Text("Lunch on Friday?", 15, ui.Regular, colors.Muted)}},
			ui.Caption{Label: "Last action", Note: last.Get},
			ui.APILine{Label: "Custom ones, activate, escape and magic tap", Names: "AccessibilityActions"},
		}}
	}

	grouped := func(cx *core.Scope) core.Node {
		colors := ui.ColorsOf(cx)
		fin := ui.FinOf(cx)
		price := core.NewSignal(cx, 120)
		return ui.Card{Icon: ui.IconAccess, Title: "Grouped elements", Description: "One element, labelled with the text inside", Children: []core.Node{
			core.View{Accessible: true, TestID: "a11y-card",
				StyleFunc: func() core.Style {
					return fin.Raised().Over(core.Style{Background: colors.Raised(), CornerRadius: ui.RadiusControl, BorderWidth: 1, BorderColor: colors.Border(), Padding: core.All(12), Gap: 4})
				},
				Children: []core.Node{
					ui.Text("Apples", 15, ui.Semibold, colors.Text),
					ui.Textf(func() string { return fmt.Sprintf("$%d.%02d a kilo", price.Get()/100, price.Get()%100) }, 15, ui.Regular, colors.Muted),
				}},
			ui.Button{Text: "Change the price", Variant: ui.ButtonSecondary, OnPress: func() { price.Update(func(p int) int { return p + 15 }) }},
		}}
	}

	hidden := func(cx *core.Scope) core.Node {
		colors := ui.ColorsOf(cx)
		h := core.NewSignal(cx, true)
		return ui.Card{Icon: ui.IconAccess, Title: "Hidden elements", Description: "Decoration screen readers skip", Children: []core.Node{
			core.View{AccessibilityElementsHidden: h.Get, TestID: "a11y-hidden", Style: core.Style{AlignItems: core.AlignCenter},
				Children: []core.Node{ui.Text("✦ ✦ ✦  decoration  ✦ ✦ ✦", 15, ui.Regular, colors.Faint)}},
			ui.SwitchRow{Label: "Hide the decoration", Code: "AccessibilityElementsHidden", Checked: h.Get, OnChange: h.Set},
		}}
	}

	live := func(cx *core.Scope) core.Node {
		colors := ui.ColorsOf(cx)
		added := core.NewSignal(cx, 0)
		return ui.Card{Icon: ui.IconAccess, Title: "Live regions", Description: "Changes announced without moving focus", Children: []core.Node{
			core.Text{LiveRegion: core.LiveRegionPolite, TestID: "a11y-live", ContentFunc: func() string { return fmt.Sprintf("%d in the basket", added.Get()) },
				Style: core.Style{FontSize: 17, FontWeight: ui.Medium}, StyleFunc: ui.Colored(colors.Text)},
			ui.Button{Text: "Add to basket", Icon: ui.IconPlus, OnPress: func() { added.Update(func(n int) int { return n + 1 }) }},
			ui.APILine{Label: "Announced as it changes", Names: "LiveRegionPolite"},
		}}
	}

	focus := func(cx *core.Scope) core.Node {
		colors := ui.ColorsOf(cx)
		result := &core.Ref{}
		return ui.Card{Icon: ui.IconAccess, Title: "Focus", Description: "Moving the screen reader, as after a search", Children: []core.Node{
			ui.Button{Text: "Search apples", Variant: ui.ButtonSecondary, TestID: "a11y-search", OnPress: result.SetAccessibilityFocus},
			core.Text{Ref: result, Content: "3 results for “apples”", TestID: "a11y-result", Style: core.Style{FontSize: 15}, StyleFunc: ui.Colored(colors.Text)},
			ui.APILine{Label: "Moves the reader here", Names: "Ref.SetAccessibilityFocus"},
		}}
	}

	return demoPage(cx, pageProps{}, values, expanded, actions, grouped, hidden, live, focus)
}

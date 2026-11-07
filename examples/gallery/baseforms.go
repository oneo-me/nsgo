package main

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/base/toast"
	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// The base forms: each component of package base as an app without package
// ui writes it, behavior and accessibility from base and every part's style
// spelled out here, under the same page as its ui form.

// bareBox is a plain pressable's shape.
var bareBox = core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 6,
	MinHeight: core.Pt(30), Padding: core.Symmetric(0, 12), CornerRadius: 6, BorderWidth: 1, Disabled: &core.Style{Opacity: core.Alpha(0.4)}}

// bareRow is a row of a list or a tree, which has no hover or press.
var bareRow = core.Style{Direction: core.Row, AlignItems: core.AlignCenter, MinHeight: core.Pt(30), Padding: core.Symmetric(0, 8), CornerRadius: 6}

// bareInk is what a plain pressable shows in each of its states.
func bareInk(cx *core.Scope) func() core.Style {
	colors := ui.ColorsOf(cx)
	return func() core.Style {
		on := &core.Style{Background: colors.Pressed()}
		return core.Style{Color: colors.Text(), BorderColor: colors.Line(), Hover: &core.Style{Background: colors.Hover()}, Active: on,
			Selected: on, Pressed: on, Expanded: on, Focus: &core.Style{BorderColor: colors.Accent()}}
	}
}

// bareRowInk is what a row shows: default, selected and focused.
func bareRowInk(cx *core.Scope) func() core.Style {
	colors := ui.ColorsOf(cx)
	return func() core.Style {
		return core.Style{Selected: &core.Style{Background: colors.Pressed()}, Focus: &core.Style{Background: colors.Hover()}}
	}
}

// bareField is a plain text field's look.
func bareField(cx *core.Scope) func() core.Style {
	colors := ui.ColorsOf(cx)
	return func() core.Style {
		return core.Style{Color: colors.Text(), BorderColor: colors.Line(), Focus: &core.Style{BorderColor: colors.Accent()}}
	}
}

// barePopup is a plain popup's fill, over the page in both appearances.
func barePopup(cx *core.Scope) func() core.Style {
	colors := ui.ColorsOf(cx)
	return func() core.Style {
		return core.Style{Background: colors.Surface(), BorderColor: colors.Line(), Color: colors.Text()}
	}
}

// barePanel is a popup's shape around its content.
var barePanel = core.Style{Padding: core.All(12), Gap: 8, CornerRadius: 8, BorderWidth: 1, MinWidth: core.Pt(200), MaxWidth: core.Pt(320)}

// bareBackdrop dims what is under a modal.
var bareBackdrop = core.Style{Background: ui.Black(0.45)}

// bareAccent marks the selected tab: a Tabs' indicator takes one style,
// which an appearance does not restyle.
var bareAccent = core.RGB(0x3478f6)

// bareText is a base form's words.
func bareText(cx *core.Scope, s string) core.Text {
	return ui.Text(s, 13, ui.Regular, ui.ColorsOf(cx).Text)
}

// bareTextf is a base form's words that change.
func bareTextf(cx *core.Scope, s func() string) core.Text {
	return ui.Textf(s, 13, ui.Regular, ui.ColorsOf(cx).Text)
}

// bareButton is a base.Button with words, to open what a stage shows.
func bareButton(cx *core.Scope, text string, press func()) core.Node {
	return base.Button{Label: text, Style: bareBox, StyleFunc: bareInk(cx), OnPress: func(core.PressEvent) { press() },
		Children: []core.Node{bareText(cx, text)}}
}

func baseButtonCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		selected, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "base.Button", Stage: func() core.Node {
			return base.Button{Label: tr.OverlaySave(), Selected: selected.Get, Disabled: disabled.Get, TestID: "base-button", Style: bareBox, StyleFunc: bareInk(cx),
				OnPress: func(core.PressEvent) { pressed(cx, last, "base.Button")() }, Children: []core.Node{bareText(cx, tr.OverlaySave())}}
		}, Knobs: []core.Node{boolKnob("Selected", selected), boolKnob("Disabled", disabled)}}
	}
}

func baseToggleCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		on, disabled := core.NewSignal(cx, true), core.NewSignal(cx, false)
		return Playground{Title: "base.Toggle", Stage: func() core.Node {
			return base.Toggle{Label: tr.LibBold(), Pressed: on.Get, OnChange: on.Set, Disabled: disabled.Get, Style: bareBox, StyleFunc: bareInk(cx),
				Children: []core.Node{bareText(cx, tr.LibBold())}}
		}, Knobs: []core.Node{boolKnob("Pressed", on), boolKnob("Disabled", disabled)}}
	}
}

func baseToggleGroupCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		bold, italic, disabled := core.NewSignal(cx, true), core.NewSignal(cx, false), core.NewSignal(cx, false)
		toggle := func(text string, on *core.Signal[bool]) core.Node {
			return base.Toggle{Label: text, Pressed: on.Get, OnChange: on.Set, Style: bareBox, StyleFunc: bareInk(cx), Children: []core.Node{bareText(cx, text)}}
		}
		return Playground{Title: "base.ToggleGroup", Stage: func() core.Node {
			return base.ToggleGroup{Label: tr.LibFormatting(), Disabled: disabled.Get, Style: core.Style{Direction: core.Row, Gap: 4},
				Children: []core.Node{toggle(tr.LibBold(), bold), toggle(tr.LibItalic(), italic)}}
		}, Knobs: []core.Node{boolKnob("Disabled", disabled)}}
	}
}

func baseLinkCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		disabled := core.NewSignal(cx, false)
		return Playground{Title: "base.Link", Stage: func() core.Node {
			return base.Link{Href: "/docs", OpenWith: base.LinkOpenApp, Disabled: disabled.Get, OnOpen: last.Set, Label: tr.LibReadDocs(),
				Style:    core.Style{Disabled: &core.Style{Opacity: core.Alpha(0.4)}},
				Children: []core.Node{ui.Text(tr.LibReadDocs(), 13, ui.Regular, colors.Accent)}}
		}, Knobs: []core.Node{boolKnob("Disabled", disabled)}}
	}
}

func baseHoldCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		seconds := core.NewSignal(cx, 1.2)
		confirmed, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "base.HoldToConfirm", Stage: func() core.Node {
			return base.HoldToConfirm{Label: tr.LibHoldToDelete(), ConfirmedLabel: tr.LibDeleted(), Duration: secondsOf(seconds.Get()), Disabled: disabled.Get, Confirmed: confirmed.Get,
				OnConfirm: func() { confirmed.Set(true); last.Set(tr.LibDeleted()) }, Style: bareBox.Merge(core.Style{Overflow: core.Hidden}), StyleFunc: bareInk(cx),
				Content: func(cx *core.Scope, fill *core.AnimatedValue) []core.Node {
					return []core.Node{
						core.View{PointerEvents: core.PointerEventsNone, Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Bottom: core.Pt(0), Width: core.Pct(100),
							Animated: &core.AnimatedStyle{Opacity: fill}}, StyleFunc: ui.Filled(colors.Pressed)},
						bareTextf(cx, func() string {
							if confirmed.Get() {
								return tr.LibDeleted()
							}
							return tr.LibHoldToDelete()
						}),
					}
				}}
		}, Knobs: []core.Node{numberKnob("Duration", seconds, 0.4, 3, 0.2), boolKnob("Confirmed", confirmed), boolKnob("Disabled", disabled)}}
	}
}

func baseToolbarCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		bold, vertical, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "base.Toolbar", Stage: func() core.Node {
			axis, direction, rule := base.Horizontal, core.Row, core.Style{Width: core.Pt(1), Height: core.Pt(20)}
			if vertical.Get() {
				axis, direction, rule = base.Vertical, core.Column, core.Style{Width: core.Pt(20), Height: core.Pt(1)}
			}
			return base.Toolbar{Axis: axis, Label: tr.LibFormatting(), Disabled: disabled.Get, Style: core.Style{Direction: direction, AlignItems: core.AlignCenter, Gap: 4},
				Children: []core.Node{
					base.ToolbarGroup{Label: tr.LibHistory(), Style: core.Style{Direction: direction, Gap: 4}, Children: []core.Node{
						bareButton(cx, tr.LibUndo(), pressed(cx, last, tr.LibUndo())),
						bareButton(cx, tr.LibRedo(), pressed(cx, last, tr.LibRedo())),
					}},
					base.ToolbarSeparator{Style: rule, StyleFunc: ui.Filled(colors.Line)},
					base.Toggle{Label: tr.LibBold(), Pressed: bold.Get, OnChange: bold.Set, Style: bareBox, StyleFunc: bareInk(cx), Children: []core.Node{bareText(cx, tr.LibBold())}},
				}}
		}, Knobs: []core.Node{boolKnob("Axis", vertical), boolKnob("Disabled", disabled)}}
	}
}

func baseInputCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	value := core.NewSignal(cx, "")
	masked, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false)
	return Playground{Title: "base.Input", Fill: true, Stage: func() core.Node {
		return base.Input{Value: value.Get, OnChange: value.Set, Placeholder: tr.CompTypeHere(), Masked: masked.Get(), Disabled: disabled.Get,
			Style: core.Style{Height: core.Pt(32), Padding: core.Symmetric(0, 8), FontSize: 13, CornerRadius: 6, BorderWidth: 1}, StyleFunc: bareField(cx)}
	}, Knobs: []core.Node{boolKnob("Masked", masked), boolKnob("Disabled", disabled)}}
}

func baseNumberInputCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		value := core.NewSignal(cx, 12.0)
		step := core.NewSignal(cx, 1.0)
		disabled := core.NewSignal(cx, false)
		button := func(s string) core.Node { return bareText(cx, s) }
		return Playground{Title: "base.NumberInput", Stage: func() core.Node {
			return base.NumberInput{Label: tr.LibFontSize(), Step: step.Get, Disabled: disabled.Get, IncrementLabel: "+1", DecrementLabel: "-1",
				Value: func() *float64 { v := value.Get(); return &v }, OnChange: func(v *float64) {
					if v != nil {
						value.Set(*v)
					}
				},
				Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 4}, ButtonLook: func() core.Style { return bareInk(cx)() },
				InputStyle:     core.Style{Width: core.Pt(72), Height: core.Pt(30), Padding: core.Symmetric(0, 8), FontSize: 13, TextAlign: core.TextAlignCenter, CornerRadius: 6, BorderWidth: 1},
				InputStyleFunc: bareField(cx), DecrementButton: button("-"), IncrementButton: button("+"),
				DecrementButtonStyle: bareBox, IncrementButtonStyle: bareBox}
		}, Knobs: []core.Node{numberKnob("Step", step, 1, 10, 1), boolKnob("Disabled", disabled)}}
	}
}

func baseTagInputCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		tags := core.NewSignalFunc(cx, []string{tr.LibApple(), tr.LibBanana()}, slices.Equal)
		disabled := core.NewSignal(cx, false)
		return Playground{Title: "base.TagInput", Fill: true, Stage: func() core.Node {
			return base.TagInput{Label: tr.LibFruit(), Tags: tags.Get, OnChange: tags.Set, Placeholder: tr.LibPickFruit(), Disabled: disabled.Get,
				RemoveIcon: bareText(cx, "×"),
				Style:      core.Style{Direction: core.Row, Wrap: core.Wrap, AlignItems: core.AlignCenter, Gap: 4, MinHeight: core.Pt(32), Padding: core.All(4), CornerRadius: 6, BorderWidth: 1},
				StyleFunc:  bareField(cx),
				TagStyle:   core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 4, Padding: core.Symmetric(2, 6), FontSize: 13, CornerRadius: 4, BorderWidth: 1}, TagStyleFunc: bareInk(cx),
				RemoveStyle: core.Style{Padding: core.Symmetric(0, 2)},
				InputStyle:  core.Style{FlexGrow: 1, MinWidth: core.Pt(80), Height: core.Pt(22), FontSize: 13}, InputStyleFunc: bareField(cx)}
		}, Knobs: []core.Node{boolKnob("Disabled", disabled)}}
	}
}

func baseOtpInputCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		masked, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "base.OtpInput", Stage: func() core.Node {
			return base.OtpInput{Length: 6, Masked: masked.Get(), Disabled: disabled.Get, Label: tr.LibOneTimeCode(), OnComplete: last.Set,
				Style: core.Style{Direction: core.Row, Gap: 4}, GroupStyle: core.Style{Direction: core.Row, Gap: 4},
				CellStyle: core.Style{Width: core.Pt(32), Height: core.Pt(36), FontSize: 14, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, CornerRadius: 6, BorderWidth: 1}, CellStyleFunc: bareField(cx)}
		}, Knobs: []core.Node{boolKnob("Masked", masked), boolKnob("Disabled", disabled)}}
	}
}

func baseCheckboxCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		checked, partly, disabled := core.NewSignal(cx, true), core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "base.Checkbox", Stage: func() core.Node {
			return base.Checkbox{Label: tr.LibStarred(), Checked: checked.Get, Indeterminate: partly.Get, Disabled: disabled.Get, OnChange: checked.Set,
				Style:          core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Disabled: &core.Style{Opacity: core.Alpha(0.4)}},
				IndicatorStyle: core.Style{Width: core.Pt(16), Height: core.Pt(16), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, CornerRadius: 4, BorderWidth: 1},
				IndicatorStyleFunc: func() core.Style {
					on := &core.Style{Background: colors.Accent(), BorderColor: colors.Accent()}
					return core.Style{BorderColor: colors.Line(), Checked: on, Indeterminate: &core.Style{Background: colors.Line()}}
				},
				Indicator: ui.Text("✓", 11, ui.Semibold, colors.OnAccent), Children: []core.Node{bareText(cx, tr.LibStarred())}}
		}, Knobs: []core.Node{boolKnob("Checked", checked), boolKnob("Indeterminate", partly), boolKnob("Disabled", disabled)}}
	}
}

func baseRadioCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		picked := core.NewSignal(cx, 0)
		disabled := core.NewSignal(cx, false)
		return Playground{Title: "base.RadioGroup · base.Radio", Stage: func() core.Node {
			var radios []core.Node
			for i, name := range []string{tr.LibApple(), tr.LibBanana(), tr.LibCherry()} {
				radios = append(radios, base.Radio{Label: name, Disabled: disabled.Get, Checked: func() bool { return picked.Get() == i }, OnChange: func(bool) { picked.Set(i) },
					Style: bareBox, StyleFunc: func() core.Style {
						return bareInk(cx)().Merge(core.Style{Checked: &core.Style{BorderColor: colors.Accent()}})
					}, Children: []core.Node{bareText(cx, name)}})
			}
			return base.RadioGroup{Label: tr.LibFruit(), Style: core.Style{Direction: core.Row, Gap: 4}, Children: radios}
		}, Knobs: []core.Node{boolKnob("Disabled", disabled)}}
	}
}

func baseSwitchCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		checked, disabled := core.NewSignal(cx, true), core.NewSignal(cx, false)
		return Playground{Title: "base.Switch", Stage: func() core.Node {
			return base.Switch{Label: tr.LibStarred(), Checked: checked.Get, Disabled: disabled.Get, OnChange: checked.Set,
				Style:      core.Style{Disabled: &core.Style{Opacity: core.Alpha(0.4)}},
				TrackStyle: core.Style{Width: core.Pt(36), Height: core.Pt(20), Padding: core.All(2), CornerRadius: 10},
				TrackStyleFunc: func() core.Style {
					return core.Style{Background: colors.Line(), Checked: &core.Style{Background: colors.Accent(), AlignItems: core.AlignEnd}}
				},
				ThumbStyle: core.Style{Width: core.Pt(16), Height: core.Pt(16), CornerRadius: 8}, ThumbStyleFunc: ui.Filled(colors.Surface)}
		}, Knobs: []core.Node{boolKnob("Checked", checked), boolKnob("Disabled", disabled)}}
	}
}

func baseSliderCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		start, end, step := core.NewSignal(cx, 30.0), core.NewSignal(cx, 70.0), core.NewSignal(cx, 1.0)
		ranged, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "base.Slider", Fill: true, Stage: func() core.Node {
			return base.Slider{Label: tr.LibVolume(), StartLabel: tr.LibMinimum(), Step: step.Get, Disabled: disabled.Get,
				Value: func() base.SliderValue {
					if ranged.Get() {
						return base.RangeValue(start.Get(), end.Get())
					}
					return base.SingleValue(start.Get())
				},
				OnChange: func(v base.SliderValue) {
					start.Set(v.Start)
					if v.Range {
						end.Set(v.End)
					}
				},
				Style:          core.Style{Height: core.Pt(20), Disabled: &core.Style{Opacity: core.Alpha(0.4)}},
				TrackStyle:     core.Style{Height: core.Pt(20)},
				IndicatorStyle: core.Style{Height: core.Pt(4), CornerRadius: 2, Margin: core.Edges{Top: 8}}, IndicatorStyleFunc: ui.Filled(colors.Line),
				RangeStyle: core.Style{CornerRadius: 2}, RangeStyleFunc: ui.Filled(colors.Accent),
				ThumbStyle: core.Style{Width: core.Pt(14), Height: core.Pt(14), CornerRadius: 7}, ThumbStyleFunc: ui.Filled(colors.Accent)}
		}, Knobs: []core.Node{numberKnob("Step", step, 1, 25, 1), boolKnob("Range", ranged), boolKnob("Disabled", disabled)}}
	}
}

func baseRatingCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		value := core.NewSignal(cx, 3)
		readonly, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "base.Rating", Stage: func() core.Node {
			return base.Rating{Label: tr.KitRating(), Max: 5, Value: value.Get, OnChange: value.Set, Readonly: readonly.Get(), Disabled: disabled.Get,
				Style: core.Style{Direction: core.Row, Gap: 2, Disabled: &core.Style{Opacity: core.Alpha(0.4)}}, StarStyle: core.Style{Padding: core.All(2)},
				Star: func(_ int, filled func() bool) core.Node {
					return ui.Textf(func() string {
						if filled() {
							return "●"
						}
						return "○"
					}, 16, ui.Regular, colors.Text)
				}}
		}, Knobs: []core.Node{boolKnob("Readonly", readonly), boolKnob("Disabled", disabled)}}
	}
}

func baseAvatarCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		colors := ui.ColorsOf(cx)
		picture := core.NewSignal(cx, false)
		return Playground{Title: "base.Avatar", Stage: func() core.Node {
			a := base.Avatar{Name: demoPeople[0], Style: core.Style{Width: core.Pt(48), Height: core.Pt(48), CornerRadius: 24, Overflow: core.Hidden},
				FallbackStyle: core.Style{AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, StyleFunc: ui.Filled(colors.Pressed),
				ImageStyle: core.Style{Width: core.Pt(48), Height: core.Pt(48)}, Fallback: bareText(cx, "AL")}
			if picture.Get() {
				a.Src = demoImage()
			}
			return a
		}, Knobs: []core.Node{boolKnob("Src", picture)}}
	}
}

func baseAvatarGroupCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		limit := core.NewSignal(cx, 3.0)
		shape := core.Style{Width: core.Pt(36), Height: core.Pt(36), CornerRadius: 18, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Overflow: core.Hidden}
		return Playground{Title: "base.AvatarGroup", Stage: func() core.Node {
			var avatars []core.Node
			for _, name := range demoPeople {
				avatars = append(avatars, base.Avatar{Name: name, Style: shape, StyleFunc: ui.Filled(colors.Pressed), Fallback: bareText(cx, name[:1]),
					FallbackStyle: core.Style{AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}})
			}
			return base.AvatarGroup{Label: tr.ControlsMembers(), Limit: int(limit.Get()), Style: core.Style{Direction: core.Row, Gap: 4},
				OverflowStyle: shape, Overflow: bareText(cx, fmt.Sprintf("+%d", len(demoPeople)-int(limit.Get()))), OverflowLabel: tr.KitMore(), Children: avatars}
		}, Knobs: []core.Node{numberKnob("Limit", limit, 1, 5, 1)}}
	}
}

func baseCardStackCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		index := core.NewSignal(cx, 0)
		var items []base.CardStackItem
		for _, name := range demoPeople[:4] {
			items = append(items, base.CardStackItem{ID: name, Label: name, Content: core.View{Style: core.Style{FlexGrow: 1, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
				Children: []core.Node{bareText(cx, name)}}})
		}
		return ui.Card{Title: "base.CardStack", Children: []core.Node{
			base.CardStack{Items: items, Index: index.Get, Label: tr.ControlsMembers(), LeftLabel: tr.KitPass(), RightLabel: tr.KitKeep(), Style: core.Style{Height: core.Pt(160)},
				OnDecide: func(i int, _ base.CardDecision) { index.Set(i + 1) },
				CardLook: func(int) core.Style {
					return core.Style{Background: colors.Surface(), BorderColor: colors.Line(), BorderWidth: 1, CornerRadius: 8}
				},
				Empty: bareButton(cx, tr.ControlsReset(), func() { index.Set(0) })},
		}}
	}
}

func baseImageCompareCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		position := core.NewSignal(cx, 50.0)
		vertical := core.NewSignal(cx, false)
		return Playground{Title: "base.ImageCompare", Fill: true, Stage: func() core.Node {
			return base.ImageCompare{Before: swatch(cx, ui.ToneInfo, tr.KitBefore()), After: swatch(cx, ui.ToneAccent, tr.KitAfter()), Vertical: vertical.Get(), Label: tr.DemoImage(),
				Position: func() float32 { return float32(position.Get()) }, OnChange: func(p float32) { position.Set(float64(p)) },
				Divider: core.View{Style: core.Style{Width: core.Pt(2), Height: core.Pct(100)}, StyleFunc: ui.Filled(colors.Text)}, Style: core.Style{Height: core.Pt(120)}}
		}, Knobs: []core.Node{numberKnob("Position", position, 0, 100, 1), boolKnob("Vertical", vertical)}}
	}
}

func baseCarouselCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		count := core.NewSignal(cx, 5.0)
		looping := core.NewSignal(cx, false)
		return Playground{Title: "base.Carousel", Fill: true, Stage: func() core.Node {
			var slides []core.Node
			for i := range int(count.Get()) {
				slides = append(slides, base.CarouselItem{Label: fmt.Sprintf("%d / %d", i+1, int(count.Get())), Style: core.Style{Width: core.Pt(200), Margin: core.Edges{Right: 8}},
					Children: []core.Node{swatch(cx, tileTones[i%len(tileTones)], itoa(i+1))}})
			}
			return base.Carousel{Label: tr.DemoImage(), Looping: looping.Get(), Style: core.Style{Gap: 8}, OnChange: func(i int) { last.Set(fmt.Sprintf("Carousel: %d", i+1)) },
				Children: []core.Node{
					base.CarouselContent{Style: core.Style{AlignItems: core.AlignStretch}, Children: slides},
					core.View{Style: core.Style{Direction: core.Row, JustifyContent: core.JustifyCenter, Gap: 4}, Children: []core.Node{
						base.CarouselPrevious{Label: tr.LibBack(), Style: bareBox, StyleFunc: bareInk(cx), Children: []core.Node{bareText(cx, "‹")}},
						base.CarouselNext{Label: tr.LibNext(), Style: bareBox, StyleFunc: bareInk(cx), Children: []core.Node{bareText(cx, "›")}},
					}},
				}}
		}, Knobs: []core.Node{numberKnob("Count", count, 2, 8, 1), boolKnob("Looping", looping)}}
	}
}

func baseListCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		multiple := core.NewSignal(cx, false)
		return Playground{Title: "base.List", Fill: true, Stage: func() core.Node {
			many := multiple.Get()
			return core.Func(func(cx *core.Scope) core.Node {
				return base.List{Label: tr.ControlsMembers(), Selection: base.NewSelection(cx, many), Count: func() int { return len(demoPeople) }, Key: func(i int) any { return demoPeople[i] },
					RowLabel: func(i int) string { return demoPeople[i] }, OnActivate: func(i int) { last.Set("base.List: " + demoPeople[i]) }, EstimatedSize: 30,
					Row: func(cx *core.Scope, row core.ListRow) core.Node {
						return bareTextf(cx, func() string { return demoPeople[row.Index()] })
					},
					Style: core.Style{Height: core.Pt(160), MinHeight: core.Pt(0)}, ListStyle: core.Style{FlexGrow: 1, MinHeight: core.Pt(0)},
					RowStyle: bareRow, RowStyleFunc: bareRowInk(cx)}
			})
		}, Knobs: []core.Node{boolKnob("Multiple", multiple)}}
	}
}

func baseTreeCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		items := []base.TreeItem{
			{ID: "source", Label: "source", Children: []base.TreeItem{{ID: "main.go", Label: "main.go"}, {ID: "view.go", Label: "view.go"}}},
			{ID: "docs", Label: "docs", Children: []base.TreeItem{{ID: "guide.md", Label: "guide.md"}}},
			{ID: "README.md", Label: "README.md"},
		}
		return ui.Card{Title: "base.Tree", Children: []core.Node{
			base.Tree{Label: tr.LibFiles(), Items: func() []base.TreeItem { return items }, DefaultExpanded: []string{"source"}, EstimatedSize: 30,
				OnAction: func(id string) { last.Set("base.Tree: " + id) }, Style: core.Style{Height: core.Pt(180), MinHeight: core.Pt(0)}, ListStyle: core.Style{FlexGrow: 1, MinHeight: core.Pt(0)},
				ItemStyle: bareRow, ItemStyleFunc: bareRowInk(cx),
				RenderItem: func(e base.TreeEntry) core.Node {
					return core.View{Style: core.Style{Padding: core.Edges{Left: core.Points(e.Depth) * 14}}, Children: []core.Node{bareText(cx, e.Label)}}
				}},
		}}
	}
}

func baseProgressCard(*core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		value := core.NewSignal(cx, 40.0)
		indeterminate := core.NewSignal(cx, false)
		return Playground{Title: "base.Progress", Fill: true, Stage: func() core.Node {
			return base.Progress{Label: tr.LibProgress(), Value: func() float32 { return float32(value.Get()) }, Indeterminate: indeterminate.Get,
				TrackStyle: core.Style{Height: core.Pt(6), CornerRadius: 3, Overflow: core.Hidden}, TrackStyleFunc: ui.Filled(colors.Line),
				IndicatorStyle: core.Style{Height: core.Pct(100)}, IndicatorStyleFunc: ui.Filled(colors.Accent)}
		}, Knobs: []core.Node{numberKnob("Value", value, 0, 100, 1), boolKnob("Indeterminate", indeterminate)}}
	}
}

func baseToastCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		shown := core.NewSignal(cx, false)
		swipe := core.NewSignal(cx, toast.SwipeRight)
		return Playground{Title: "toast.Toast", Fill: true, Stage: func() core.Node {
			direction := swipe.Get()
			return core.View{Style: core.Style{Gap: 8, AlignItems: core.AlignStart}, Children: []core.Node{
				bareButton(cx, tr.KitShow(), func() { shown.Set(true) }),
				core.Show{When: shown.Get, Then: func(cx *core.Scope) core.Node {
					return toast.Toast{Label: tr.LibChangesSaved(), SwipeDirection: direction, OnDismissRequest: func() { shown.Set(false); last.Set(tr.OverlayToastGone()) },
						Style: barePanel, StyleFunc: barePopup(cx), Children: []core.Node{bareText(cx, tr.LibChangesSaved())}}
				}},
			}}
		}, Knobs: []core.Node{choiceKnob("SwipeDirection", swipe, options([]toast.SwipeDirection{toast.SwipeRight, toast.SwipeLeft, toast.SwipeOff}, "Right", "Left", "Off")...)}}
	}
}

func baseDialogCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		open := core.NewSignal(cx, false)
		noEscape, noBackdrop := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "base.Dialog", Stage: func() core.Node {
			return core.Group{bareButton(cx, tr.KitOpen(), func() { open.Set(true) }),
				base.Dialog{Open: open.Get, Label: tr.KitDialogTitle(), TestID: "base-dialog", NoCloseOnEscape: noEscape.Get(), NoCloseOnBackdropPress: noBackdrop.Get(),
					OnOpenChange:  func(o bool, reason base.Reason) { open.Set(o); last.Set("base.Dialog: " + reason.String()) },
					BackdropStyle: bareBackdrop, PopupStyle: barePanel, PopupStyleFunc: barePopup(cx),
					Children: []core.Node{bareText(cx, tr.KitDialogTitle()), bareButton(cx, tr.OverlayDone(), func() { open.Set(false) })}}}
		}, Knobs: []core.Node{boolKnob("NoCloseOnEscape", noEscape), boolKnob("NoCloseOnBackdropPress", noBackdrop)}}
	}
}

func baseAlertDialogCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		return Playground{Title: "base.ShowAlertDialog", Stage: func() core.Node {
			return bareButton(cx, tr.KitOpen(), func() {
				base.ShowAlertDialog(cx, base.AlertDialog{Title: tr.KitDialogTitle(), Description: tr.KitDialogNote(),
					Actions: []core.AlertAction{{Label: tr.KitCancel()}, {Label: tr.OverlayDone()}}}, func(label string) { last.Set("AlertDialog: " + label) })
			})
		}}
	}
}

func baseSheetCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		open := core.NewSignal(cx, false)
		side := core.NewSignal(cx, core.PlaceBottom)
		swipe := core.NewSignal(cx, false)
		return Playground{Title: "base.Sheet", Stage: func() core.Node {
			return core.Group{bareButton(cx, tr.OverlayShowTheSheet(), func() { open.Set(true) }),
				base.Sheet{Open: open.Get, Side: side.Get(), CloseOnSwipe: swipe.Get(), Label: tr.OverlaySheet(),
					OnOpenChange:  func(next bool, reason base.Reason) { open.Set(next); last.Set("base.Sheet: " + reason.String()) },
					BackdropStyle: bareBackdrop, PopupStyle: core.Style{Padding: core.All(16), Gap: 12}, PopupStyleFunc: ui.Filled(colors.Surface),
					Children: []core.Node{bareText(cx, tr.OverlayTheBackdropEscapeAndBack()), bareButton(cx, tr.OverlayDone(), func() { open.Set(false) })}}}
		}, Knobs: []core.Node{choiceKnob("Side", side, placements...), boolKnob("CloseOnSwipe", swipe)}}
	}
}

func basePopoverCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		placement := core.NewSignal(cx, core.PlaceBottom)
		align := core.NewSignal(cx, core.AlignCenter)
		return Playground{Title: "base.Popover", Stage: func() core.Node {
			return base.Popover{Placement: placement.Get(), Align: align.Get(), Offset: 4, Margin: 8, Label: tr.OverlayPopover(), Style: bareBox, StyleFunc: bareInk(cx),
				Trigger:      bareText(cx, tr.OverlayShowThePopover()),
				OnOpenChange: func(_ bool, reason base.Reason) { last.Set("base.Popover: " + reason.String()) },
				PopupStyle:   barePanel, PopupStyleFunc: barePopup(cx), Children: []core.Node{bareText(cx, tr.OverlayAPressOutsideEscapeOr())}}
		}, Knobs: []core.Node{choiceKnob("Placement", placement, placements...), choiceKnob("Align", align, aligns...)}}
	}
}

func baseTooltipCard(*core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		placement := core.NewSignal(cx, core.PlaceTop)
		return Playground{Title: "base.Tooltip", Stage: func() core.Node {
			return base.Tooltip{Placement: placement.Get(), Offset: 4, Margin: 8, Trigger: bareText(cx, tr.OverlaySave()), Style: bareBox, StyleFunc: bareInk(cx),
				PopupStyle: core.Style{Padding: core.Symmetric(4, 8), CornerRadius: 6, BorderWidth: 1}, PopupStyleFunc: barePopup(cx),
				Children: []core.Node{bareText(cx, tr.OverlaySavesTheDocument())}}
		}, Knobs: []core.Node{choiceKnob("Placement", placement, placements...)}}
	}
}

// bareMenuItems are the items of the base forms of Menu and ContextMenu.
func bareMenuItems(cx *core.Scope, last *core.Signal[string]) []core.Node {
	tr := locales.For(cx)
	item := func(text string) core.Node {
		return base.MenuItem{Label: text, OnSelect: func() { last.Set("base.Menu: " + text) }, Style: bareRow, StyleFunc: bareInk(cx),
			Children: []core.Node{bareText(cx, text)}}
	}
	colors := ui.ColorsOf(cx)
	return []core.Node{item(tr.TerminalCopy()), item(tr.TerminalPaste()),
		base.MenuSeparator{Style: core.Style{Height: core.Pt(1), Margin: core.Symmetric(4, 0)}, StyleFunc: ui.Filled(colors.Line)},
		base.Submenu{Label: tr.KitMore(), Trigger: bareText(cx, tr.KitMore()), Back: bareText(cx, tr.LibBack()), Style: bareRow, StyleFunc: bareInk(cx),
			PopupStyle: bareMenuPanel, PopupStyleFunc: barePopup(cx), Children: []core.Node{item(tr.LibApple()), item(tr.LibBanana())}}}
}

// bareMenuPanel is a menu's popup: its rows reach its edges.
var bareMenuPanel = core.Style{Padding: core.All(4), Gap: 2, CornerRadius: 8, BorderWidth: 1, MinWidth: core.Pt(160)}

func baseMenuCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		placement := core.NewSignal(cx, core.PlaceBottom)
		return Playground{Title: "base.Menu", Stage: func() core.Node {
			return base.Menu{Placement: placement.Get(), Offset: 4, Margin: 8, Label: tr.DemoMenu(), Trigger: bareText(cx, tr.KitOpen()), Style: bareBox, StyleFunc: bareInk(cx),
				PopupStyle: bareMenuPanel, PopupStyleFunc: barePopup(cx), BackdropStyle: bareBackdrop, Children: bareMenuItems(cx, last)}
		}, Knobs: []core.Node{choiceKnob("Placement", placement, placements...)}}
	}
}

func baseContextMenuCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		off := core.NewSignal(cx, false)
		return Playground{Title: "base.ContextMenu", Fill: true, Stage: func() core.Node {
			return base.ContextMenu{Disabled: off.Get(), PopupStyle: bareMenuPanel, PopupStyleFunc: barePopup(cx), BackdropStyle: bareBackdrop, Children: bareMenuItems(cx, last),
				Trigger: core.View{Style: core.Style{Height: core.Pt(64), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
					Children: []core.Node{bareText(cx, Hint(cx, tr.KitLongPressHere(), tr.KitRightClickHere()))}}}
		}, Knobs: []core.Node{boolKnob("Disabled", off)}}
	}
}

func baseTabsCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		selected := core.NewSignal(cx, 0)
		disabled := core.NewSignal(cx, false)
		return Playground{Title: "base.Tabs · base.Tab", Fill: true, Stage: func() core.Node {
			var tabs []core.Node
			for i, name := range []string{tr.ControlsOverview(), tr.ControlsActivity(), tr.ControlsBilling()} {
				tab := base.Tab{Label: name, Selected: func() bool { return selected.Get() == i }, OnPress: func(core.PressEvent) { selected.Set(i); last.Set(fmt.Sprintf("base.Tabs: %d", i)) },
					Style: bareBox.Merge(core.Style{BorderWidth: 0}), StyleFunc: bareInk(cx), Children: []core.Node{bareText(cx, name)}}
				if i == 2 {
					tab.Disabled = disabled.Get
				}
				tabs = append(tabs, tab)
			}
			return base.Tabs{Label: tr.DemoTabs(), Style: core.Style{Direction: core.Row, Gap: 4, Padding: core.Edges{Bottom: 2}}, Children: tabs,
				IndicatorStyle: core.Style{Position: core.Absolute, Bottom: core.Pt(0), Height: core.Pt(2), Background: bareAccent}}
		}, Knobs: []core.Node{boolKnob("Disabled", disabled)}}
	}
}

func baseStepperCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		current := core.NewSignal(cx, 1.0)
		steps := checkoutSteps(tr)
		failed := core.NewSignal(cx, false)
		return Playground{Title: "base.Stepper", Fill: true, Stage: func() core.Node {
			failing := failed.Get()
			return base.Stepper{Label: tr.LibCheckout(), Count: len(steps), Current: func() int { return int(current.Get()) }, Failed: func(i int) bool { return failing && i == 1 },
				OnSelect: func(i int) { current.Set(float64(i)); last.Set(steps[i].Label) }, StepLabel: func(i int, _ base.StepStatus) string { return steps[i].Label },
				ListStyle: core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 4}, StepStyle: bareBox, StepStyleFunc: bareInk(cx),
				Head: func(cx *core.Scope, step base.StepperStep) []core.Node {
					return []core.Node{bareTextf(cx, func() string { return stepMark(step.Status()) + steps[step.Index].Label })}
				}}
		}, Knobs: []core.Node{numberKnob("Current", current, 0, 4, 1), boolKnob("Failed", failed)}}
	}
}

// stepMark is a step's status in a glyph before its label.
func stepMark(s base.StepStatus) string {
	switch s {
	case base.StepCurrent:
		return "● "
	case base.StepComplete:
		return "✓ "
	case base.StepFailed:
		return "✕ "
	}
	return "○ "
}

func basePaginationCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		page, total := core.NewSignal(cx, 5.0), core.NewSignal(cx, 20.0)
		compact, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "base.Pagination", Stage: func() core.Node {
			return base.Pagination{Label: tr.LibPages(), PreviousLabel: tr.LibPreviousPage(), NextLabel: tr.LibNextPage(), VisiblePages: 7, Compact: compact.Get, Disabled: disabled.Get,
				Page: func() int { return int(page.Get()) }, TotalPages: func() int { return int(total.Get()) }, OnChange: func(p int) { page.Set(float64(p)) },
				Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 2}, ItemStyle: bareBox.Merge(core.Style{Padding: core.Symmetric(0, 6), MinWidth: core.Pt(30)}), ItemStyleFunc: bareInk(cx),
				RenderItem: func(i base.PaginationItem) core.Node {
					switch i.Kind {
					case base.PaginationPrevious:
						return bareText(cx, "‹")
					case base.PaginationNext:
						return bareText(cx, "›")
					case base.PaginationEllipsis:
						return bareText(cx, "…")
					}
					return bareText(cx, strconv.Itoa(i.Page))
				}}
		}, Knobs: []core.Node{numberKnob("TotalPages", total, 1, 100, 1), boolKnob("Compact", compact), boolKnob("Disabled", disabled)}}
	}
}

// demoSections are the Sidebar demo's sections.
func demoSections(tr locales.Messages) []string {
	return []string{tr.ControlsOverview(), tr.ControlsActivity(), tr.ControlsBilling()}
}

func sidebarCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		selected := core.NewSignal(cx, tr.ControlsOverview())
		var items []ui.SidebarItem
		for _, name := range demoSections(tr) {
			items = append(items, ui.SidebarItem{ID: name, Text: name})
		}
		return ui.Card{Title: "Sidebar", Children: []core.Node{
			core.View{Style: core.Style{Height: core.Pt(150), Width: core.Pt(220), AlignSelf: core.AlignCenter}, Children: []core.Node{
				ui.Sidebar{Selected: selected.Get, OnSelect: func(id string) { selected.Set(id); last.Set("Sidebar: " + id) },
					Sections: []ui.SidebarSection{{Title: tr.ControlsApps(), Items: items}}},
			}},
		}}
	}
}

func baseSidebarCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		selected := core.NewSignal(cx, tr.ControlsOverview())
		var items []base.SidebarItem
		for _, name := range demoSections(tr) {
			items = append(items, base.SidebarItem{ID: name, Label: name, Children: []core.Node{bareText(cx, name)}})
		}
		return ui.Card{Title: "base.Sidebar", Children: []core.Node{
			core.View{Style: core.Style{Height: core.Pt(150), Width: core.Pt(220), AlignSelf: core.AlignCenter}, Children: []core.Node{
				base.Sidebar{Selected: selected.Get, OnSelect: func(id string) { selected.Set(id); last.Set("base.Sidebar: " + id) },
					Sections: []base.SidebarSection{{Header: bareText(cx, tr.ControlsApps()), Items: items, Style: core.Style{Gap: 2}}},
					Style:    core.Style{FlexGrow: 1, MinHeight: core.Pt(0)}, ItemStyle: bareRow, ItemStyleFunc: bareRowInk(cx)},
			}},
		}}
	}
}

func baseAccordionCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		open := core.NewSignalFunc(cx, []string{"overview"}, slices.Equal)
		multiple, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false)
		item := func(value, title, body string) core.Node {
			return base.AccordionItem{Value: value, Style: core.Style{Gap: 4}, Children: []core.Node{
				base.AccordionHeader{Children: []core.Node{base.AccordionTrigger{Label: title, Style: bareBox.Merge(core.Style{JustifyContent: core.JustifyStart}), StyleFunc: bareInk(cx),
					Children: []core.Node{bareText(cx, title)}}}},
				base.AccordionPanel{Style: core.Style{Padding: core.Symmetric(4, 12)}, Children: []core.Node{bareText(cx, body)}},
			}}
		}
		return Playground{Title: "base.Accordion", Fill: true, Stage: func() core.Node {
			return base.Accordion{Value: open.Get, Multiple: multiple.Get, Disabled: disabled.Get, OnChange: open.Set, Style: core.Style{Gap: 4}, Children: []core.Node{
				item("overview", tr.ControlsOverview(), tr.ControlsDeployedToProduction()),
				item("activity", tr.ControlsActivity(), tr.ControlsInvoicePaid()),
			}}
		}, Knobs: []core.Node{boolKnob("Multiple", multiple), boolKnob("Disabled", disabled)}}
	}
}

func baseCollapsibleCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		open := core.NewSignal(cx, true)
		return Playground{Title: "base.Collapsible", Fill: true, Stage: func() core.Node {
			return base.Collapsible{Open: open.Get, Style: core.Style{Gap: 4}, Content: core.View{Style: core.Style{Padding: core.Symmetric(4, 12)}, Children: []core.Node{bareText(cx, tr.ControlsDeployedToProduction())}},
				Children: []core.Node{base.Button{Label: tr.ControlsActivity(), Expanded: open.Get, OnPress: func(core.PressEvent) { toggled(open) }, Style: bareBox, StyleFunc: bareInk(cx),
					Children: []core.Node{bareText(cx, tr.ControlsActivity())}}}}
		}, Knobs: []core.Node{boolKnob("Open", open)}}
	}
}

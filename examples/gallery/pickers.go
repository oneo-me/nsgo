package main

import (
	"fmt"
	"slices"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// What picks a value from a popup: ui's forms, and base's, which take their
// look part by part as a bare form does.

func comboboxPlayground(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		multiple := core.NewSignal(cx, false)
		value := core.NewSignalFunc(cx, []string(nil), slices.Equal[[]string])
		choices := []ui.Option[string]{{Value: "apple", Label: tr.LibApple()}, {Value: "banana", Label: tr.LibBanana()}, {Value: "cherry", Label: tr.LibCherry()}}
		return Playground{Title: "Combobox", Stage: func() core.Node {
			return core.Dynamic(func(*core.Scope) core.Node {
				return ui.Combobox[string]{Options: choices, Multiple: multiple.Get(), Value: value.Get, Placeholder: tr.LibPickFruit(), Label: tr.LibFruit(),
					Style:    core.Style{Width: core.Pt(240)},
					OnChange: func(v []string) { value.Set(v); last.Set(fmt.Sprintf("Combobox: %v", v)) }}
			})
		}, Knobs: []core.Node{boolKnob("Multiple", multiple)}}
	}
}

func commandPlayground(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		items := commandItems(tr)
		return Playground{Title: "Command", Fill: true, Stage: func() core.Node {
			return ui.Command{Items: func() []base.CommandItem { return items }, Placeholder: tr.PickerFindCommand(), Label: tr.DemoCommand(),
				Style: core.Style{Height: core.Pt(280), MaxWidth: core.Pt(420)}, OnSelect: func(id string) { last.Set("Command: " + id) }}
		}}
	}
}

// commandItems are what a card's palette runs.
func commandItems(tr locales.Messages) []base.CommandItem {
	return []base.CommandItem{
		{ID: "new", Label: tr.DemoDialog(), Shortcut: "mod+n", Group: tr.DemoMenu()},
		{ID: "sheet", Label: tr.DemoSheet(), Keywords: []string{"panel"}, Group: tr.DemoMenu()},
		{ID: "calendar", Label: tr.DemoDatePicker(), Group: tr.DemoSelect()},
		{ID: "color", Label: tr.DemoColorPicker(), Group: tr.DemoSelect(), Disabled: true},
	}
}

func hoverCardPlayground(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		placement := core.NewSignal(cx, core.PlaceBottom)
		return Playground{Title: "HoverCard", Stage: func() core.Node {
			return ui.HoverCard{Placement: placement.Get(), OnOpenChange: func(_ bool, reason base.Reason) { last.Set("HoverCard: " + reason.String()) },
				Trigger:  ui.Button{Text: tr.DemoHoverCard()},
				Children: []core.Node{ui.Text(tr.DemoHoverCardDetail(), 13, ui.Regular, ui.ColorsOf(cx).Text)}}
		}, Knobs: []core.Node{choiceKnob("Placement", placement, placements...)}}
	}
}

func datePickerPlayground(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		ranged, timed, cleanable := core.NewSignal(cx, false), core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "DatePicker", Stage: func() core.Node {
			return core.Dynamic(func(*core.Scope) core.Node {
				precision := base.TimeNone
				if timed.Get() {
					precision = base.TimeMinute
				}
				return ui.DatePicker{Range: ranged.Get(), TimePrecision: precision, TimeLabel: tr.PickerTime(), Cleanable: cleanable.Get(),
					Placeholder: tr.PickerPickDate(), OnChange: func(v base.DateValue) { last.Set("DatePicker: " + v.Format("%Y-%m-%d %H:%M")) }}
			})
		}, Knobs: []core.Node{boolKnob("Range", ranged), boolKnob("TimePrecision", timed), boolKnob("Cleanable", cleanable)}}
	}
}

func calendarPlayground(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		ranged, weekends := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "Calendar", Stage: func() core.Node {
			return core.Dynamic(func(*core.Scope) core.Node {
				c := ui.Calendar{Range: ranged.Get(), OnChange: func(v base.DateValue) { last.Set("Calendar: " + v.Format("%Y-%m-%d")) }}
				if weekends.Get() {
					c.DisabledMatcher = base.DaysOfWeek(0, 6)
				}
				return c
			})
		}, Knobs: []core.Node{boolKnob("Range", ranged), boolKnob("No weekends", weekends)}}
	}
}

func timeFieldPlayground(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		twelve := core.NewSignal(cx, false)
		return Playground{Title: "TimeField", Stage: func() core.Node {
			return core.Dynamic(func(*core.Scope) core.Node {
				cycle := base.HourCycle23
				if twelve.Get() {
					cycle = base.HourCycle12
				}
				return ui.TimeField{HourCycle: cycle, Label: tr.PickerTime(), DefaultValue: base.TimeOfDay{Hour: 9, Minute: 30},
					OnChange: func(t base.TimeOfDay) { last.Set("TimeField: " + t.String()) }}
			})
		}, Knobs: []core.Node{boolKnob("HourCycle12", twelve)}}
	}
}

var colorPresets = []core.Color{0xef4444ff, 0xf59e0bff, 0x22c55eff, 0x3b82f6ff, 0x8b5cf6ff, 0x3b82f680}

func colorPickerPlayground(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		noAlpha, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false)
		color := core.NewSignal[core.Color](cx, 0xc2410cff)
		return Playground{Title: "ColorPicker", Stage: func() core.Node {
			return core.Dynamic(func(*core.Scope) core.Node {
				return ui.ColorPicker{Value: color.Get, Presets: colorPresets, NoAlpha: noAlpha.Get(), Disabled: disabled.Get,
					OnChange: func(c core.Color, _ string) { color.Set(c) }, OnChangeEnd: func(_ core.Color, formatted string) { last.Set("ColorPicker: " + formatted) }}
			})
		}, Knobs: []core.Node{boolKnob("NoAlpha", noAlpha), boolKnob("Disabled", disabled)}}
	}
}

func colorWellPlayground(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		color := core.NewSignal[core.Color](cx, 0x3b82f6ff)
		return Playground{Title: "ColorWell", Stage: func() core.Node {
			return ui.ColorWell{Value: color.Get, Label: tr.DemoColorPicker(), Picker: ui.ColorPicker{Presets: colorPresets},
				OnChange: color.Set, OnOpenChange: func(_ bool, reason base.Reason) { last.Set("ColorWell: " + reason.String()) }}
		}}
	}
}

// fruits are what a list of a card offers.
func fruits(tr locales.Messages) func() []base.SelectItem {
	items := []base.SelectItem{{Value: "apple", Title: tr.LibApple()}, {Value: "banana", Title: tr.LibBanana()}, {Value: "cherry", Title: tr.LibCherry()}}
	return func() []base.SelectItem { return items }
}

// bareRows is a list's rows in a card's look.
const bareRows = "group h-[32px] px-2 justify-center rounded-md focus:bg-black/10 dark:focus:bg-white/10 selected:font-semibold disabled:opacity-40"

func baseSelectCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		searchable, cleanable := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "base.Select", Stage: func() core.Node {
			return base.Select{Items: fruits(tr), Searchable: searchable.Get, Cleanable: cleanable.Get(), SearchPlaceholder: tr.LibPickFruit(),
				Label: tr.LibFruit(), Offset: 4, Margin: 8, Style: bareBox.Merge(core.Style{Width: core.Pt(200)}), StyleFunc: bareInk(cx),
				PopupStyle: barePanel, PopupStyleFunc: barePopup(cx), ItemClass: bareRows, BackdropStyle: bareBackdrop,
				Trigger: bareText(cx, tr.LibPickFruit()), ClearButton: bareText(cx, "×"), ClearButtonLabel: tr.LibFruit(),
				OnChange: func(v *string) {
					if v == nil {
						last.Set("base.Select: null")
						return
					}
					last.Set("base.Select: " + *v)
				}}
		}, Knobs: []core.Node{boolKnob("Searchable", searchable), boolKnob("Cleanable", cleanable)}}
	}
}

func baseComboboxCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		multiple := core.NewSignal(cx, false)
		return Playground{Title: "base.Combobox", Fill: true, Stage: func() core.Node {
			return core.Dynamic(func(*core.Scope) core.Node {
				return base.Combobox{Items: fruits(tr), Multiple: multiple.Get(), Placeholder: tr.LibPickFruit(), Label: tr.LibFruit(), Offset: 4, Margin: 8,
					Style: bareBox, StyleFunc: bareField(cx), InputStyle: core.Style{FlexGrow: 1},
					PopupStyle: barePanel, PopupStyleFunc: barePopup(cx), ItemClass: bareRows, BackdropStyle: bareBackdrop,
					Empty:    bareText(cx, tr.PickerNothingFound()),
					OnChange: func(v []string) { last.Set(fmt.Sprintf("base.Combobox: %v", v)) }}
			})
		}, Knobs: []core.Node{boolKnob("Multiple", multiple)}}
	}
}

func baseCommandCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		items := commandItems(tr)
		return Playground{Title: "base.Command", Fill: true, Stage: func() core.Node {
			return base.Command{Items: func() []base.CommandItem { return items }, Placeholder: tr.PickerFindCommand(), Label: tr.DemoCommand(),
				Style: core.Style{Height: core.Pt(240), Gap: 4}, SearchStyle: core.Style{Height: core.Pt(32)}, ItemClass: bareRows,
				GroupStyle: core.Style{Padding: core.Symmetric(4, 8), Opacity: core.Alpha(0.6)}, SeparatorStyle: core.Style{Height: core.Pt(8)},
				Empty: bareText(cx, tr.PickerNothingFound()),
				RenderItem: func(e base.CommandEntry) core.Node {
					return core.View{Style: core.Style{Direction: core.Row, JustifyContent: core.JustifySpaceBetween}, Children: []core.Node{
						bareText(cx, e.Label), bareText(cx, e.ShortcutLabel)}}
				},
				OnSelect: func(id string) { last.Set("base.Command: " + id) }}
		}}
	}
}

func baseHoverCardCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		placement := core.NewSignal(cx, core.PlaceBottom)
		return Playground{Title: "base.HoverCard", Stage: func() core.Node {
			return base.HoverCard{Placement: placement.Get(), Offset: 4, Margin: 8, PopupStyle: barePanel, PopupStyleFunc: barePopup(cx),
				OnOpenChange: func(_ bool, reason base.Reason) { last.Set("base.HoverCard: " + reason.String()) },
				Trigger:      core.View{Style: bareBox, StyleFunc: bareInk(cx), Children: []core.Node{bareText(cx, tr.DemoHoverCard())}},
				Children:     []core.Node{bareText(cx, tr.DemoHoverCardDetail())}}
		}, Knobs: []core.Node{choiceKnob("Placement", placement, placements...)}}
	}
}

// bareCalendar is a calendar's parts in a card's look.
func bareCalendar() base.CalendarConfig {
	cell := core.Style{MinHeight: core.Pt(30), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}
	return base.CalendarConfig{ItemStyle: cell,
		ItemClass: "rounded-md selected:bg-blue-500 selected:text-white disabled:opacity-40 data-outside:opacity-40 data-today:font-semibold data-range-middle:bg-blue-500/20"}
}

func baseCalendarCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		ranged, months := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "base.Calendar", Fill: true, Stage: func() core.Node {
			return core.Dynamic(func(*core.Scope) core.Node {
				config := bareCalendar()
				if months.Get() {
					config.NumberOfMonths = 2
				}
				return base.Calendar{Range: ranged.Get(), CalendarConfig: config,
					OnChange: func(v base.DateValue) { last.Set("base.Calendar: " + v.Format("%Y-%m-%d")) }}
			})
		}, Knobs: []core.Node{boolKnob("Range", ranged), boolKnob("Two months", months)}}
	}
}

func baseTimeFieldCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		twelve := core.NewSignal(cx, false)
		return Playground{Title: "base.TimeField", Stage: func() core.Node {
			return core.Dynamic(func(*core.Scope) core.Node {
				cycle := base.HourCycle23
				if twelve.Get() {
					cycle = base.HourCycle12
				}
				return base.TimeField{HourCycle: cycle, Label: tr.PickerTime(), DefaultValue: base.TimeOfDay{Hour: 9, Minute: 30},
					Style: bareBox, StyleFunc: bareField(cx), SegmentClass: "px-1 rounded focus:bg-blue-500 focus:text-white",
					OnChange: func(t base.TimeOfDay) { last.Set("base.TimeField: " + t.String()) }}
			})
		}, Knobs: []core.Node{boolKnob("h12", twelve)}}
	}
}

func baseDatePickerCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		timed, cleanable := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "base.DatePicker", Stage: func() core.Node {
			return core.Dynamic(func(*core.Scope) core.Node {
				precision := base.TimeNone
				if timed.Get() {
					precision = base.TimeMinute
				}
				today := cx.WallTime()
				return base.DatePicker{TimePrecision: precision, Cleanable: cleanable.Get(), CalendarConfig: bareCalendar(), Label: tr.PickerPickDate(),
					Presets: []base.DatePreset{{Label: tr.PickerPickDate(), Value: base.DateValue{Start: base.NewDate(today.Year(), today.Month(), today.Day(), base.TimeOfDay{})}}},
					Offset:  4, Margin: 8, Style: bareBox, StyleFunc: bareInk(cx), PopupStyle: barePanel, PopupStyleFunc: barePopup(cx), BackdropStyle: bareBackdrop,
					SegmentClass: "px-1 rounded focus:bg-blue-500 focus:text-white", TimeFieldStyle: core.Style{Position: core.Relative}, TimeLabel: bareText(cx, tr.PickerTime()),
					Trigger: bareText(cx, tr.PickerPickDate()), ClearButton: bareText(cx, "×"), ClearButtonLabel: tr.PickerPickDate(),
					OnChange: func(v base.DateValue) { last.Set("base.DatePicker: " + v.Format("%Y-%m-%d %H:%M")) }}
			})
		}, Knobs: []core.Node{boolKnob("TimePrecision", timed), boolKnob("Cleanable", cleanable)}}
	}
}

func baseColorPickerCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		disabled := core.NewSignal(cx, false)
		thumb := "w-[14px] h-[14px] rounded-full border-2 border-white shadow"
		track := "h-[10px] rounded-full"
		slider := "h-[20px] justify-center"
		swatches := []core.Color{0xef4444ff, 0x22c55eff, 0x3b82f6ff, 0x3b82f680}
		presets := make([]core.Node, len(swatches))
		for i, c := range swatches {
			presets[i] = base.ColorSwatch{Color: func() core.Color { return c }, Class: "w-[24px] h-[24px] rounded-md selected:border-2",
				CheckerboardColor: 0xccccccff, CheckerboardSize: 6}
		}
		return Playground{Title: "base.ColorPicker", Fill: true, Stage: func() core.Node {
			return base.ColorPicker{DefaultValue: 0x3b82f6ff, Format: base.ColorFormatHex, Disabled: disabled.Get, Style: core.Style{Gap: 8, MaxWidth: core.Pt(280)},
				OnChangeEnd: func(_ core.Color, formatted string) { last.Set("base.ColorPicker: " + formatted) },
				Children: []core.Node{
					base.ColorArea{Class: "h-[140px]", PanelClass: "rounded-lg", ThumbClass: thumb, FillThumb: true, Label: tr.DemoColorPicker()},
					base.ColorSlider{Class: slider, Channel: base.ChannelHue, TrackClass: track, ThumbClass: thumb, FillThumb: true, Label: "Hue"},
					base.ColorSlider{Class: slider, Channel: base.ChannelAlpha, TrackClass: track, ThumbClass: thumb, Label: "Alpha", CheckerboardColor: 0xccccccff, CheckerboardSize: 5},
					core.View{Style: core.Style{Direction: core.Row, Gap: 8, AlignItems: core.AlignCenter}, Children: []core.Node{
						base.ColorSwatch{Class: "w-[32px] h-[32px] rounded-md", CheckerboardColor: 0xccccccff, CheckerboardSize: 6},
						base.ColorField{Label: "Hex", Style: core.Style{FlexGrow: 1, Height: core.Pt(32)}},
						base.ColorEyeDropper{Label: tr.DemoColorPicker(), Style: bareBox, StyleFunc: bareInk(cx), Children: []core.Node{bareText(cx, "⌖")}},
					}},
					core.View{Style: core.Style{Direction: core.Row, Gap: 6}, Children: presets},
				}}
		}, Knobs: []core.Node{boolKnob("Disabled", disabled)}}
	}
}

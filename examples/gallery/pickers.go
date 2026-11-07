package main

import (
	"fmt"
	"slices"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// What picks a value from a popup: ui's forms, and above each base's, which
// takes its look part by part as a bare form does.

func comboboxPlayground(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		multiple := core.NewSignal(cx, false)
		value := core.NewSignalFunc(cx, []string(nil), slices.Equal[[]string])
		choices := []ui.Option[string]{{Value: "apple", Label: tr.LibApple()}, {Value: "banana", Label: tr.LibBanana()}, {Value: "cherry", Label: tr.LibCherry()}}
		return Playground{Title: "Combobox", Base: baseComboboxForm(cx, last, value, multiple), Stage: func() core.Node {
			return core.Dynamic(func(*core.Scope) core.Node {
				return ui.Combobox[string]{Options: choices, Multiple: multiple.Get(), Value: value.Get, Placeholder: tr.LibPickFruit(), Label: tr.LibFruit(),
					Style:    core.Style{Width: core.Pt(240)},
					OnChange: func(v []string) { value.Set(v); last.Set(fmt.Sprintf("Combobox: %v", v)) }}
			})
		}, BaseKnobs: []core.Node{boolKnob("Multiple", multiple)}}
	}
}

func commandPlayground(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		items := commandItems(tr)
		expand, loading, failing, open := core.NewSignal(cx, false), core.NewSignal(cx, false), core.NewSignal(cx, false), core.NewSignal(cx, false)
		failure := func() string {
			if failing.Get() {
				return tr.PickerCommandFailed()
			}
			return ""
		}
		palette := func(expanded bool) ui.Command {
			return ui.Command{Items: func() []base.CommandItem { return items }, Placeholder: tr.PickerFindCommand(), Label: tr.DemoCommand(),
				ExpandOnSearch: expanded, Loading: loading.Get, Error: failure}
		}
		return Playground{Title: "Command", Start: true, Base: baseCommandForm(cx, last), Stage: func() core.Node {
			return core.Dynamic(func(*core.Scope) core.Node {
				inline, dialog := palette(expand.Get()), palette(expand.Get())
				inline.Style = core.Style{Height: core.Pt(280)}
				inline.OnSelect = func(id string) { last.Set("Command: " + id) }
				dialog.InputSource = commandInputSource
				dialog.OnSelect = func(id string) {
					open.Set(false)
					last.Set("CommandDialog: " + id)
				}
				return core.View{Style: core.Style{Gap: 12, AlignItems: core.AlignStart}, Children: []core.Node{
					core.View{Style: core.Style{AlignSelf: core.AlignStretch}, Children: []core.Node{inline}},
					ui.Button{Text: tr.PickerOpenCommandDialog(), Variant: ui.ButtonSecondary, TestID: "open-command-dialog", OnPress: func() { open.Set(true) }},
					ui.CommandDialog{TestID: "command-dialog", Open: open.Get, OnOpenChange: func(o bool, _ base.Reason) { open.Set(o) }, Command: dialog}}}
			})
		}, Knobs: []core.Node{boolKnob("ExpandOnSearch", expand), boolKnob("Loading", loading), boolKnob("Error", failing)}}
	}
}

// commandItems are what a card's palette runs.
func commandItems(tr locales.Messages) []base.CommandItem {
	return []base.CommandItem{
		{ID: "new", Label: tr.DemoDialog(), Description: tr.DemoDialogDetail(), Shortcut: "mod+n", Group: tr.DemoMenu(), Icon: lucide.AppWindow()},
		{ID: "sheet", Label: tr.DemoSheet(), Keywords: []string{"panel"}, Group: tr.DemoMenu(), Icon: lucide.Layers()},
		{ID: "calendar", Label: tr.DemoDatePicker(), Group: tr.DemoSelect(), Icon: lucide.Calendar(), Trailing: ui.Badge{Text: tr.DemoSelect()}},
		{ID: "color", Label: tr.DemoColorPicker(), Group: tr.DemoSelect(), Icon: lucide.Palette(), Disabled: true},
	}
}

func hoverCardPlayground(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		placement := core.NewSignal(cx, core.PlaceBottom)
		return Playground{Title: "HoverCard", Base: baseHoverCardForm(cx, last, placement), Stage: func() core.Node {
			return ui.HoverCard{Placement: placement.Get(), OnOpenChange: func(_ bool, reason base.Reason) { last.Set("HoverCard: " + reason.String()) },
				Trigger:  ui.Button{Text: tr.DemoHoverCard()},
				Children: []core.Node{ui.Text(tr.DemoHoverCardDetail(), 13, ui.Regular, ui.ColorsOf(cx).Text)}}
		}, BaseKnobs: []core.Node{choiceKnob("Placement", placement, placements...)}}
	}
}

func datePickerPlayground(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		ranged, timed, cleanable := core.NewSignal(cx, false), core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "DatePicker", Base: baseDatePickerForm(cx, last, ranged, timed, cleanable), Stage: func() core.Node {
			return core.Dynamic(func(*core.Scope) core.Node {
				precision := base.TimeNone
				if timed.Get() {
					precision = base.TimeMinute
				}
				return ui.DatePicker{Range: ranged.Get(), TimePrecision: precision, TimeLabel: tr.PickerTime(), Cleanable: cleanable.Get(),
					Placeholder: tr.PickerPickDate(), OnChange: func(v base.DateValue) { last.Set("DatePicker: " + v.Format("%Y-%m-%d %H:%M")) }}
			})
		}, BaseKnobs: []core.Node{boolKnob("Range", ranged), boolKnob("TimePrecision", timed), boolKnob("Cleanable", cleanable)}}
	}
}

func calendarPlayground(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		ranged, months, weekends := core.NewSignal(cx, false), core.NewSignal(cx, false), core.NewSignal(cx, false)
		return Playground{Title: "Calendar", Base: baseCalendarForm(cx, last, ranged, months, weekends), Stage: func() core.Node {
			return core.Dynamic(func(*core.Scope) core.Node {
				c := ui.Calendar{Range: ranged.Get(), OnChange: func(v base.DateValue) { last.Set("Calendar: " + v.Format("%Y-%m-%d")) }}
				if months.Get() {
					c.NumberOfMonths = 2
				}
				if weekends.Get() {
					c.DisabledMatcher = base.DaysOfWeek(0, 6)
				}
				return c
			})
		}, BaseKnobs: []core.Node{boolKnob("Range", ranged), boolKnob("Two months", months), boolKnob("No weekends", weekends)}}
	}
}

func timeFieldPlayground(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		twelve := core.NewSignal(cx, false)
		return Playground{Title: "TimeField", Base: baseTimeFieldForm(cx, last, twelve), Stage: func() core.Node {
			return core.Dynamic(func(*core.Scope) core.Node {
				cycle := base.HourCycle23
				if twelve.Get() {
					cycle = base.HourCycle12
				}
				return ui.TimeField{HourCycle: cycle, Label: tr.PickerTime(), DefaultValue: base.TimeOfDay{Hour: 9, Minute: 30},
					OnChange: func(t base.TimeOfDay) { last.Set("TimeField: " + t.String()) }}
			})
		}, BaseKnobs: []core.Node{boolKnob("HourCycle12", twelve)}}
	}
}

var colorPresets = []core.Color{0xef4444ff, 0xf59e0bff, 0x22c55eff, 0x3b82f6ff, 0x8b5cf6ff, 0x3b82f680}

func colorPickerPlayground(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		noAlpha, disabled := core.NewSignal(cx, false), core.NewSignal(cx, false)
		color := core.NewSignal[core.Color](cx, 0xc2410cff)
		return Playground{Title: "ColorPicker", Base: baseColorPickerForm(cx, last, color, disabled), Stage: func() core.Node {
			return core.Dynamic(func(*core.Scope) core.Node {
				return ui.ColorPicker{Value: color.Get, Presets: colorPresets, NoAlpha: noAlpha.Get(), Disabled: disabled.Get,
					OnChange: func(c core.Color, _ string) { color.Set(c) }, OnChangeEnd: func(_ core.Color, formatted string) { last.Set("ColorPicker: " + formatted) }}
			})
		}, BaseKnobs: []core.Node{boolKnob("Disabled", disabled)}, Knobs: []core.Node{boolKnob("NoAlpha", noAlpha)}}
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

func baseSelectForm(cx *core.Scope, last, value *core.Signal[string], searchable, cleanable *core.Signal[bool]) Form {
	tr := locales.For(cx)
	return Form{Name: "base.Select", Stage: func() core.Node {
		return base.Select{Items: fruits(tr), Value: func() *string {
			if v := value.Get(); v != "" {
				return &v
			}
			return nil
		}, Searchable: searchable.Get, Cleanable: cleanable.Get(), SearchPlaceholder: tr.LibPickFruit(),
			Label: tr.LibFruit(), Offset: 4, Margin: 8, Style: bareBox.Merge(core.Style{Width: core.Pt(200)}), StyleFunc: bareInk(cx),
			PopupStyle: barePanel, PopupStyleFunc: barePopup(cx), ItemClass: bareRows, BackdropStyle: bareBackdrop,
			Trigger: bareText(cx, tr.LibPickFruit()), ClearButton: bareText(cx, "×"), ClearButtonLabel: tr.LibFruit(),
			OnChange: func(v *string) {
				if v == nil {
					value.Set("")
					last.Set("base.Select: null")
					return
				}
				value.Set(*v)
				last.Set("base.Select: " + *v)
			}}
	}}
}

func baseComboboxForm(cx *core.Scope, last *core.Signal[string], value *core.Signal[[]string], multiple *core.Signal[bool]) Form {
	tr := locales.For(cx)
	return Form{Name: "base.Combobox", Fill: true, Stage: func() core.Node {
		return core.Dynamic(func(*core.Scope) core.Node {
			return base.Combobox{Items: fruits(tr), Multiple: multiple.Get(), Value: value.Get, Placeholder: tr.LibPickFruit(), Label: tr.LibFruit(), Offset: 4, Margin: 8,
				Style: bareBox, StyleFunc: bareField(cx), InputStyle: core.Style{FlexGrow: 1},
				PopupStyle: barePanel, PopupStyleFunc: barePopup(cx), ItemClass: bareRows, BackdropStyle: bareBackdrop,
				Empty:    bareText(cx, tr.PickerNothingFound()),
				OnChange: func(v []string) { value.Set(v); last.Set(fmt.Sprintf("base.Combobox: %v", v)) }}
		})
	}}
}

func baseCommandForm(cx *core.Scope, last *core.Signal[string]) Form {
	tr := locales.For(cx)
	items := commandItems(tr)
	return Form{Name: "base.Command", Start: true, Stage: func() core.Node {
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

func baseHoverCardForm(cx *core.Scope, last *core.Signal[string], placement *core.Signal[core.Placement]) Form {
	tr := locales.For(cx)
	return Form{Name: "base.HoverCard", Stage: func() core.Node {
		return base.HoverCard{Placement: placement.Get(), Offset: 4, Margin: 8, PopupStyle: barePanel, PopupStyleFunc: barePopup(cx),
			OnOpenChange: func(_ bool, reason base.Reason) { last.Set("base.HoverCard: " + reason.String()) },
			Trigger:      core.View{Style: bareBox, StyleFunc: bareInk(cx), Children: []core.Node{bareText(cx, tr.DemoHoverCard())}},
			Children:     []core.Node{bareText(cx, tr.DemoHoverCardDetail())}}
	}}
}

// bareCalendar is a calendar's parts in a card's look.
func bareCalendar() base.CalendarConfig {
	cell := core.Style{MinHeight: core.Pt(30), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}
	return base.CalendarConfig{ItemStyle: cell,
		ItemClass: "rounded-md selected:bg-blue-500 selected:text-white disabled:opacity-40 data-outside:opacity-40 data-today:font-semibold data-range-middle:bg-blue-500/20"}
}

func baseCalendarForm(cx *core.Scope, last *core.Signal[string], ranged, months, weekends *core.Signal[bool]) Form {
	return Form{Name: "base.Calendar", Fill: true, Stage: func() core.Node {
		return core.Dynamic(func(*core.Scope) core.Node {
			config := bareCalendar()
			if months.Get() {
				config.NumberOfMonths = 2
			}
			if weekends.Get() {
				config.DisabledMatcher = base.DaysOfWeek(0, 6)
			}
			return base.Calendar{Range: ranged.Get(), CalendarConfig: config,
				OnChange: func(v base.DateValue) { last.Set("base.Calendar: " + v.Format("%Y-%m-%d")) }}
		})
	}}
}

func baseTimeFieldForm(cx *core.Scope, last *core.Signal[string], twelve *core.Signal[bool]) Form {
	tr := locales.For(cx)
	return Form{Name: "base.TimeField", Stage: func() core.Node {
		return core.Dynamic(func(*core.Scope) core.Node {
			cycle := base.HourCycle23
			if twelve.Get() {
				cycle = base.HourCycle12
			}
			return base.TimeField{HourCycle: cycle, Label: tr.PickerTime(), DefaultValue: base.TimeOfDay{Hour: 9, Minute: 30},
				Style: bareBox, StyleFunc: bareField(cx), SegmentClass: "px-1 rounded focus:bg-blue-500 focus:text-white",
				OnChange: func(t base.TimeOfDay) { last.Set("base.TimeField: " + t.String()) }}
		})
	}}
}

func baseDatePickerForm(cx *core.Scope, last *core.Signal[string], ranged, timed, cleanable *core.Signal[bool]) Form {
	tr := locales.For(cx)
	return Form{Name: "base.DatePicker", Stage: func() core.Node {
		return core.Dynamic(func(*core.Scope) core.Node {
			precision := base.TimeNone
			if timed.Get() {
				precision = base.TimeMinute
			}
			today := cx.WallTime()
			return base.DatePicker{Range: ranged.Get(), TimePrecision: precision, Cleanable: cleanable.Get(), CalendarConfig: bareCalendar(), Label: tr.PickerPickDate(),
				Presets: []base.DatePreset{{Label: tr.PickerPickDate(), Value: base.DateValue{Start: base.NewDate(today.Year(), today.Month(), today.Day(), base.TimeOfDay{})}}},
				Offset:  4, Margin: 8, Style: bareBox, StyleFunc: bareInk(cx), PopupStyle: barePanel, PopupStyleFunc: barePopup(cx), BackdropStyle: bareBackdrop,
				SegmentClass: "px-1 rounded focus:bg-blue-500 focus:text-white", TimeFieldStyle: core.Style{Position: core.Relative}, TimeLabel: bareText(cx, tr.PickerTime()),
				Trigger: bareText(cx, tr.PickerPickDate()), ClearButton: bareText(cx, "×"), ClearButtonLabel: tr.PickerPickDate(),
				OnChange: func(v base.DateValue) { last.Set("base.DatePicker: " + v.Format("%Y-%m-%d %H:%M")) }}
		})
	}}
}

func baseColorPickerForm(cx *core.Scope, last *core.Signal[string], color *core.Signal[core.Color], disabled *core.Signal[bool]) Form {
	tr := locales.For(cx)
	thumb := "w-[14px] h-[14px] rounded-full border-2 border-white shadow"
	track := "h-[10px] rounded-full"
	slider := "h-[20px] justify-center"
	swatches := []core.Color{0xef4444ff, 0x22c55eff, 0x3b82f6ff, 0x3b82f680}
	presets := make([]core.Node, len(swatches))
	for i, c := range swatches {
		presets[i] = base.ColorSwatch{Color: func() core.Color { return c }, Class: "w-[24px] h-[24px] rounded-md selected:border-2",
			CheckerboardColor: 0xccccccff, CheckerboardSize: 6}
	}
	return Form{Name: "base.ColorPicker", Fill: true, Stage: func() core.Node {
		return base.ColorPicker{Value: color.Get, Format: base.ColorFormatHex, Disabled: disabled.Get, Style: core.Style{Gap: 8, MaxWidth: core.Pt(280)},
			OnChange:    func(c core.Color, _ string) { color.Set(c) },
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
	}}
}

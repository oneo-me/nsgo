package tsx

import (
	"math"
	"reflect"
	"strings"
	"time"

	"github.com/dop251/goja"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

// The popups of nsgo/base that NSH 0.4.0 added, as Go components.

func init() {
	Optional[base.SelectItem]("Disabled", "Group")
	Fields[base.SelectItem](map[string]string{})
	Fields[base.SelectEntry](map[string]string{})
	Fields[base.SelectGroup](map[string]string{})
	Optional[base.CommandItem]("Description", "Keywords", "Group", "Shortcut", "Icon", "Leading", "Trailing", "Disabled", "Checked")
	Fields[base.CommandItem](map[string]string{"ID": "id"})
	Fields[base.CommandEntry](map[string]string{"ID": "id"})
	Fields[base.CommandGroup](map[string]string{})
	Fields[base.CalendarEntry](map[string]string{})
	Fields[base.TimeFieldSegmentEntry](map[string]string{})
	options := func(extra ...Option) []Option {
		return append(append(append([]Option(nil), enumOptions...), dateEnums...), extra...)
	}
	openChange := Event("OnOpenChange", "open", "reason")
	Register("nsgo/base", map[string]core.Node{"MenuItem": base.MenuItem{}}, options(
		custom("Checked", func(p *props, v reflect.Value) {
			if p.has("checked") {
				v.Set(reflect.ValueOf(func() bool { return p.bool("checked") }))
			}
		}, "checked?: boolean | null"),
		custom("Role", func(p *props, v reflect.Value) {
			if p.has("role") {
				v.Set(reflect.ValueOf(roles[p.str("role")]))
			}
		}, `role?: "menuitem" | "menuitemcheckbox" | "menuitemradio"`))...)
	looks := []Option{Rename("StartingClass", "popupStartingClass"), Rename("StartingStyle", "popupStartingStyle"),
		Rename("EndingClass", "popupEndingClass"), Rename("EndingStyle", "popupEndingStyle")}
	Register("nsgo/base", map[string]core.Node{"Popover": base.Popover{}, "Menu": base.Menu{}, "ContextMenu": base.ContextMenu{}},
		options(append(looks, openChange)...)...)
	Register("nsgo/base", map[string]core.Node{"HoverCard": base.HoverCard{}}, options(append(looks, openChange,
		delayProp("OpenDelay", "openDelay"), delayProp("CloseDelay", "closeDelay"))...)...)
	Register("nsgo/base", map[string]core.Node{"Select": base.Select{}}, options(openChange,
		Rename("SearchLabel", "searchAccessibilityLabel"), Rename("ClearButtonLabel", "clearButtonAccessibilityLabel"), Omit("SearchField"),
		custom("Value", optionalString("value"), "value?: string | null"),
		custom("OnChange", func(p *props, v reflect.Value) {
			if h := p.handler("onChange"); h != nil {
				v.Set(reflect.ValueOf(func(value *string) {
					e := p.r.vm.NewObject()
					_ = e.Set("value", stringOrNull(value))
					h(e)
				}))
			}
		}, "onChange?: (e: { value: string | null }) => void"))...)
	Register("nsgo/base", map[string]core.Node{"Calendar": base.Calendar{}}, options(append(calendarOptions(),
		typed("Value", custom("Value", dateValueProp("value"), "value?: DatePickerValue"), reflect.TypeFor[base.DateValue]()),
		custom("DefaultValue", defaultDateProp, "defaultValue?: DatePickerValue"),
		custom("OnChange", dateEvent(func(*props) base.TimePrecision { return base.TimeNone }), "onChange?: (e: { value: DatePickerValue }) => void"))...)...)
	Register("nsgo/base", map[string]core.Node{"TimeField": base.TimeField{}}, options(
		custom("Value", func(p *props, v reflect.Value) {
			if p.has("value") {
				v.Set(reflect.ValueOf(func() base.TimeOfDay { t, _ := base.ParseTimeOfDay(p.str("value")); return t }))
			}
		}, "value?: string"),
		custom("DefaultValue", func(p *props, v reflect.Value) {
			t, _ := base.ParseTimeOfDay(p.str("defaultValue"))
			v.Set(reflect.ValueOf(t))
		}, "defaultValue?: string"),
		custom("OnChange", func(p *props, v reflect.Value) {
			if h := p.handler("onChange"); h != nil {
				v.Set(reflect.ValueOf(func(t base.TimeOfDay) {
					e := p.r.vm.NewObject()
					_ = e.Set("value", t.String())
					h(e)
				}))
			}
		}, "onChange?: (e: { value: string }) => void"))...)
	Register("nsgo/base", map[string]core.Node{"DatePicker": base.DatePicker{}}, options(append(calendarOptions(), openChange,
		Rename("ClearButtonLabel", "clearButtonAccessibilityLabel"), Rename("TimeFieldLabel", "timeAccessibilityLabel"), Rename("TimeLabel", "timeLabel"),
		typed("Value", custom("Value", dateValueProp("value"), "value?: DatePickerValue"), reflect.TypeFor[base.DateValue]()),
		custom("DefaultValue", defaultDateProp, "defaultValue?: DatePickerValue"),
		custom("OnChange", dateEvent(func(p *props) base.TimePrecision { return timePrecisions[p.str("timePrecision")] }),
			"onChange?: (e: { value: DatePickerValue }) => void"),
		custom("DefaultTime", func(p *props, v reflect.Value) {
			t, _ := base.ParseTimeOfDay(p.str("defaultTime"))
			v.Set(reflect.ValueOf(t))
		}, "defaultTime?: string"),
		custom("Presets", func(p *props, v reflect.Value) {
			var presets []base.DatePreset
			for _, item := range p.r.arrayItems(p.get("presets")) {
				o := p.r.options(item)
				presets = append(presets, base.DatePreset{Label: o.str("label", ""), Value: p.r.dateValue(o.o.Get("value"))})
			}
			v.Set(reflect.ValueOf(presets))
		}, "presets?: readonly { label: string; value: DatePickerValue }[]"))...)...)
	Shape("DatePickerValue", `export type DatePickerValue = string | [string | null, string | null] | null;`,
		func(base.DateValue) map[string]any { return nil }, nil)
	colorOptions := func(extra ...Option) []Option {
		return options(append([]Option{
			Enum(map[string]base.ColorSpace{"hsb": base.ColorHSB, "rgb": base.ColorRGB, "hsl": base.ColorHSL, "oklch": base.ColorOKLCH}),
			Enum(map[string]base.ColorChannel{"red": base.ChannelRed, "green": base.ChannelGreen, "blue": base.ChannelBlue,
				"hue": base.ChannelHue, "saturation": base.ChannelSaturation, "lightness": base.ChannelLightness,
				"brightness": base.ChannelBrightness, "chroma": base.ChannelChroma, "alpha": base.ChannelAlpha}),
			Enum(map[string]base.ColorFormat{"hex": base.ColorFormatHex, "rgb": base.ColorFormatRGB, "hsl": base.ColorFormatHSL,
				"hsb": base.ColorFormatHSB, "oklch": base.ColorFormatOKLCH}),
		}, extra...)...)
	}
	Register("nsgo/base", map[string]core.Node{"ColorPicker": base.ColorPicker{}}, colorOptions(
		Event("OnChange", "value", "formatted"), Event("OnChangeEnd", "value", "formatted"))...)
	Register("nsgo/base", map[string]core.Node{
		"ColorArea": base.ColorArea{}, "ColorSlider": base.ColorSlider{}, "ColorField": base.ColorField{},
	}, colorOptions(Omit("Field"))...)
	Register("nsgo/base", map[string]core.Node{"ColorSwatch": base.ColorSwatch{}}, colorOptions(Event("OnPress", "color"))...)
	Register("nsgo/base", map[string]core.Node{"ColorEyeDropper": base.ColorEyeDropper{}}, colorOptions(Event("OnPick", "color"))...)
	Register("nsgo/base", map[string]core.Node{"Command": base.Command{}}, options(
		Rename("NoSearch", "searchable"), Rename("NoFilter", "filterable"), Rename("NoSeparator", "separator"),
		Rename("SearchLabel", "searchAccessibilityLabel"), Omit("SearchField"), Omit("Filter"),
		Event("OnSelect", "id"), Event("OnQueryChange", "query"),
		custom("Query", optionalString("query"), "query?: string"),
		custom("OnHighlightChange", func(p *props, v reflect.Value) {
			if h := p.handler("onHighlightChange"); h != nil {
				v.Set(reflect.ValueOf(func(highlighted *string) {
					e := p.r.vm.NewObject()
					_ = e.Set("highlighted", stringOrNull(highlighted))
					h(e)
				}))
			}
		}, "onHighlightChange?: (e: { highlighted: string | null }) => void"))...)
	Register("nsgo/base", map[string]core.Node{"Combobox": base.Combobox{}}, options(openChange,
		Rename("ClearButtonLabel", "clearButtonAccessibilityLabel"), Omit("Field"),
		Event("OnChange", "value"), Event("OnConfirm", "value"), Event("OnQueryChange", "query"),
		custom("Value", func(p *props, v reflect.Value) {
			if p.has("value") {
				v.Set(reflect.ValueOf(func() []string { return p.r.stringList(p.get("value")) }))
			}
		}, "value?: readonly string[]"),
		custom("Query", optionalString("query"), "query?: string"))...)
}

// optionalString sets a func() *string from prop, a string or null, while
// it is set.
func optionalString(prop string) func(p *props, v reflect.Value) {
	return func(p *props, v reflect.Value) {
		if !p.has(prop) {
			return
		}
		v.Set(reflect.ValueOf(func() *string {
			value := p.get(prop)
			if !defined(value) {
				return nil
			}
			s := value.String()
			return &s
		}))
	}
}

// stringOrNull is s in JS, null for nil.
func stringOrNull(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

// stringList reads an array of strings.
func (r *runtime) stringList(v goja.Value) []string {
	var out []string
	for _, item := range r.arrayItems(v) {
		out = append(out, item.String())
	}
	return out
}

// delayProp sets a delay from prop, in milliseconds: 0 goes at once, and
// Infinity or one no timer waits is never.
func delayProp(field, prop string) Option {
	return custom(field, func(p *props, v reflect.Value) {
		if !p.has(prop) {
			return
		}
		ms := p.get(prop).ToFloat()
		switch {
		case ms > math.MaxInt32:
			v.Set(reflect.ValueOf(base.HoverNever))
		case ms > 0:
			v.Set(reflect.ValueOf(time.Duration(ms * float64(time.Millisecond))))
		default:
			v.Set(reflect.ValueOf(time.Duration(-1)))
		}
	}, prop+"?: number")
}

var timePrecisions = map[string]base.TimePrecision{"minute": base.TimeMinute, "second": base.TimeSecond}

var dateEnums = []Option{
	Enum(map[string]base.TimePrecision{"minute": base.TimeMinute, "second": base.TimeSecond}),
	Enum(map[string]base.HourCycle{"h23": base.HourCycle23, "h12": base.HourCycle12}),
	Enum(map[string]base.TimeSegment{"hour": base.SegmentHour, "minute": base.SegmentMinute, "second": base.SegmentSecond,
		"period": base.SegmentPeriod}),
	Enum(map[string]base.CalendarItemKind{"day": base.CalendarDay, "previous": base.CalendarPrevious, "monthToggle": base.CalendarMonthToggle,
		"yearToggle": base.CalendarYearToggle, "next": base.CalendarNext, "weekday": base.CalendarWeekday, "month": base.CalendarMonth,
		"year": base.CalendarYear}),
}

// calendarOptions are the props a Calendar and a DatePicker share.
func calendarOptions() []Option {
	return []Option{
		Rename("PreviousLabel", "previousAccessibilityLabel"), Rename("NextLabel", "nextAccessibilityLabel"),
		Omit("ItemStyleOf"), custom("DisabledMatcher", dateMatcher, "disabledMatcher?: number[] | { before?: string | null; after?: string | null } | { from?: string | null; to?: string | null } | ((date: string) => boolean)"),
		custom("YearRange", func(p *props, v reflect.Value) {
			if r := p.r.arrayItems(p.get("yearRange")); len(r) == 2 {
				v.Set(reflect.ValueOf([2]int{int(r[0].ToInteger()), int(r[1].ToInteger())}))
			}
		}, "yearRange?: [number, number]"),
	}
}

// dateTime reads an ISO date, or a date and a time after T or a space.
func dateTime(v goja.Value) time.Time {
	if !defined(v) {
		return time.Time{}
	}
	text := strings.TrimSpace(v.String())
	date, clock, timed := strings.Cut(strings.Replace(text, " ", "T", 1), "T")
	day, ok := base.ParseISODate(date)
	if !ok {
		return time.Time{}
	}
	var t base.TimeOfDay
	if timed {
		t, _ = base.ParseTimeOfDay(clock)
	}
	return base.NewDate(day.Year(), day.Month(), day.Day(), t)
}

// dateValue reads a date, [start, end], or null.
func (r *runtime) dateValue(v goja.Value) base.DateValue {
	if o, ok := v.(*goja.Object); ok && o.ClassName() == "Array" {
		items := r.arrayItems(v)
		out := base.DateValue{Range: true}
		if len(items) == 2 {
			out.Start, out.End = dateTime(items[0]), dateTime(items[1])
		}
		return out
	}
	return base.DateValue{Start: dateTime(v)}
}

func dateValueProp(prop string) func(p *props, v reflect.Value) {
	return func(p *props, v reflect.Value) {
		if p.has(prop) {
			v.Set(reflect.ValueOf(func() base.DateValue { return p.r.dateValue(p.get(prop)) }))
		}
	}
}

func defaultDateProp(p *props, v reflect.Value) {
	if p.has("defaultValue") {
		v.Set(reflect.ValueOf(p.r.dateValue(p.get("defaultValue"))))
	}
}

// dateEvent reports a value in the HTML inputs' forms, at precision's.
func dateEvent(precision func(*props) base.TimePrecision) func(p *props, v reflect.Value) {
	return func(p *props, v reflect.Value) {
		h := p.handler("onChange")
		if h == nil {
			return
		}
		v.Set(reflect.ValueOf(func(d base.DateValue) {
			e := p.r.vm.NewObject()
			at := precision(p)
			text := func(t time.Time) any {
				switch {
				case t.IsZero():
					return nil
				case at == base.TimeNone:
					return base.ISODate(t)
				}
				return base.ISODate(t) + "T" + base.TimeOfDay{Hour: t.Hour(), Minute: t.Minute(), Second: t.Second()}.Format(at)
			}
			if d.Range {
				_ = e.Set("value", p.r.vm.NewArray(text(d.Start), text(d.End)))
			} else {
				_ = e.Set("value", text(d.Start))
			}
			h(e)
		}))
	}
}

// dateMatcher sets a DisabledMatcher from days of the week, { before,
// after }, { from, to } or a function, asked once per date: the same date,
// the same answer.
func dateMatcher(p *props, v reflect.Value) {
	if !p.has("disabledMatcher") {
		return
	}
	r := p.r
	var fn goja.Value
	var answers map[time.Time]bool
	var match func(time.Time) bool
	var last goja.Value
	read := func() func(time.Time) bool {
		m := p.get("disabledMatcher")
		if m == last {
			return match
		}
		last, match = m, nil
		switch {
		case !defined(m):
		case isFunction(m):
			fn, answers = m, map[time.Time]bool{}
			match = func(t time.Time) bool {
				answer, known := answers[t]
				if !known {
					answer = r.call(fn, r.vm.ToValue(base.ISODate(t))).ToBoolean()
					answers[t] = answer
				}
				return answer
			}
		case m.(*goja.Object).ClassName() == "Array":
			var days []int
			for _, d := range r.arrayItems(m) {
				days = append(days, int(d.ToInteger()))
			}
			match = base.DaysOfWeek(days...)
		default:
			o := r.options(m)
			if f, to := dateTime(o.o.Get("from")), dateTime(o.o.Get("to")); !f.IsZero() || !to.IsZero() {
				match = base.DatesBetween(f, to)
			} else {
				match = base.DatesOutside(dateTime(o.o.Get("before")), dateTime(o.o.Get("after")))
			}
		}
		return match
	}
	v.Set(reflect.ValueOf(func(t time.Time) bool {
		m := read()
		return m != nil && m(t)
	}))
}

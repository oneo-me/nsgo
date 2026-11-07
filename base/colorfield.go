package base

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/oneo-me/nsgo/base/locales"
	"github.com/oneo-me/nsgo/core"
)

// colorNumberDigits are the decimals a channel keeps as a number, so what
// converting between spaces adds does not read as a change.
const colorNumberDigits = 6

// colorScrubDistance is how many points of a drag across a letter make a
// step.
const colorScrubDistance = 1

// ColorField is the color as text in any CSS form, read as it is submitted
// or left, or with Channel that channel in a NumberInput: what is typed
// shows at once, the keys step it, and the change ends as the field is
// left. Label says the channel's name when empty.
type ColorField struct {
	Channel    ColorChannel
	ColorSpace ColorSpace
	Format     ColorFormat
	// Digits writes the color as six hex digits in capitals, with no # or
	// alpha, and takes six or eight as they are typed; six keep the alpha.
	Digits      bool
	Disabled    func() bool
	Placeholder string
	Class       string
	Style       core.Style
	// Letter is a Channel's scrub handle, a step a point, such as "R".
	Letter string
	// Field draws the input, and Number a Channel's, as a styled form does.
	Field  func(core.Input) core.Node
	Number func(NumberInput) core.Node
	Label  string
	TestID string
}

// Render implements core.Node.
func (f ColorField) Render(cx *core.Scope) core.Node {
	picker := colorStateOf(cx)
	draft := core.NewSignal[*string](cx, nil)
	channel := f.Channel
	if channel != ChannelNone {
		channel = resolveChannel(f.ColorSpace, channel)
	}
	written := func() string {
		m := picker.model.Get()
		if f.Digits {
			return strings.ToUpper(fmt.Sprintf("%06x", m.packed()>>8))
		}
		return m.format(f.Format)
	}
	read := func(text string) (colorModel, bool) {
		now := picker.now()
		next, ok := parseColorText(now, text)
		if digits := strings.TrimPrefix(strings.TrimSpace(text), "#"); ok && f.Digits && (len(digits) == 3 || len(digits) == 6) {
			next.alpha = now.alpha
		}
		return next, ok
	}
	commit := func() {
		text := draft.GetUntracked()
		if text == nil {
			return
		}
		draft.Set(nil)
		if next, ok := read(*text); ok {
			picker.set(next)
		}
		picker.end()
	}
	disabled := func() bool { return is(f.Disabled) || picker.isDisabled() }
	if channel != ChannelNone {
		return f.channel(cx, picker, channel, disabled)
	}
	return drawnField(f.Field, core.Input{Placeholder: f.Placeholder, NoAutoCorrect: true, AutoCapitalize: core.AutoCapitalizeNone,
		ReadonlyFunc: disabled, Disabled: disabled, Label: f.Label, TestID: f.TestID, Class: f.Class, Style: f.Style,
		Value: func() string {
			if d := draft.Get(); d != nil {
				return *d
			}
			return written()
		},
		OnChange: func(text string) {
			draft.Set(&text)
			digits := strings.TrimPrefix(strings.TrimSpace(text), "#")
			if next, ok := read(text); ok && f.Digits && (len(digits) == 6 || len(digits) == 8) {
				picker.set(next)
			}
		},
		OnBlur: commit, OnSubmit: func(string) { commit() }})
}

// channel is the field of a channel, in percents, degrees or bytes as it
// reads, to a decimal where it has them.
func (f ColorField) channel(cx *core.Scope, picker *colorState, channel ColorChannel, disabled func() bool) core.Node {
	spec := colorSpecOf(f.ColorSpace, channel)
	step, precision := 1.0, 0.1
	switch f.ColorSpace {
	case ColorRGB:
		if channel != ChannelAlpha {
			precision = 1
		}
	case ColorOKLCH:
		step, precision = spec.step, spec.step
	}
	suffix := ""
	if channel == ChannelAlpha || f.ColorSpace != ColorOKLCH && (channel == ChannelSaturation || channel == ChannelLightness || channel == ChannelBrightness) {
		suffix = "%"
	}
	decimals := 1
	if precision < 0.1 || precision == 1 {
		decimals = spec.decimals
	}
	zero, top := 0.0, spec.max
	label := f.Label
	if label == "" {
		label = colorChannelLabel(cx, channel)
	}
	in := NumberInput{Class: f.Class, Style: f.Style, TestID: f.TestID, Label: label, Disabled: disabled, Placeholder: func() string { return f.Placeholder },
		Min: func() *float64 { return &zero }, Max: func() *float64 { return &top }, Step: func() float64 { return step },
		OnCommit: true, Live: true, Snap: true, Precision: precision, OnChangeEnd: picker.end,
		Format: func(v float64) string { return colorNumber(v, decimals) + suffix },
		Parse: func(text string) (float64, bool) {
			v, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(text), "%"), 64)
			return v, err == nil
		},
		Value: func() *float64 {
			v := roundTo(picker.model.Get().channel(f.ColorSpace, channel), colorNumberDigits)
			return &v
		},
		OnChange: func(v *float64) {
			if v != nil {
				picker.set(picker.now().withChannel(f.ColorSpace, channel, *v))
			}
		}}
	if f.Letter != "" {
		in.Scrub, in.ScrubDistance = core.Text{Content: f.Letter, MaxLines: 1}, colorScrubDistance
	}
	if f.Number != nil {
		return f.Number(in)
	}
	return in
}

// colorChannelLabel is what assistive technology says for a channel's
// field.
func colorChannelLabel(cx *core.Scope, c ColorChannel) string {
	switch c {
	case ChannelRed:
		return locales.ColorRed(cx)
	case ChannelGreen:
		return locales.ColorGreen(cx)
	case ChannelBlue:
		return locales.ColorBlue(cx)
	case ChannelHue:
		return locales.ColorHueDegrees(cx)
	case ChannelSaturation:
		return locales.ColorSaturationPercent(cx)
	case ChannelBrightness:
		return locales.ColorBrightnessPercent(cx)
	case ChannelLightness:
		return locales.ColorLightnessPercent(cx)
	case ChannelAlpha:
		return locales.ColorOpacityPercent(cx)
	}
	return ""
}

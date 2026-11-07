package base

import (
	"strconv"
	"strings"

	"github.com/oneo-me/nsgo/core"
)

// ColorField is the color as text in any CSS form, read as it is submitted
// or left, or with Channel that channel's number, read as it is typed.
type ColorField struct {
	Channel     ColorChannel
	ColorSpace  ColorSpace
	Format      ColorFormat
	Disabled    func() bool
	Placeholder string
	Class       string
	Style       core.Style
	// Field draws the input, as a styled form does.
	Field  func(core.Input) core.Node
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
	spec := colorSpecOf(f.ColorSpace, channel)
	written := func() string {
		m := picker.model.Get()
		if channel != ChannelNone {
			return spec.format(m.channel(f.ColorSpace, channel))
		}
		return m.format(f.Format)
	}
	commit := func() {
		text := draft.GetUntracked()
		if text == nil {
			return
		}
		draft.Set(nil)
		if channel == ChannelNone {
			if next, ok := parseColorText(picker.now(), *text); ok {
				picker.set(next)
			}
		}
		picker.end()
	}
	disabled := func() bool { return is(f.Disabled) || picker.isDisabled() }
	keyboard := core.KeyboardDefault
	if channel != ChannelNone {
		keyboard = core.KeyboardDecimal
	}
	return drawnField(f.Field, core.Input{Placeholder: f.Placeholder, KeyboardType: keyboard, NoAutoCorrect: true, AutoCapitalize: core.AutoCapitalizeNone,
		ReadonlyFunc: disabled, Disabled: disabled, Label: f.Label, TestID: f.TestID, Class: f.Class, Style: f.Style,
		Value: func() string {
			if d := draft.Get(); d != nil {
				return *d
			}
			return written()
		},
		OnChange: func(text string) {
			draft.Set(&text)
			if channel == ChannelNone {
				return
			}
			if v, err := strconv.ParseFloat(strings.TrimSpace(text), 64); err == nil {
				picker.set(picker.now().withChannel(f.ColorSpace, channel, v))
			}
		},
		OnBlur: commit, OnSubmit: func(string) { commit() }})
}

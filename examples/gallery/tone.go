package main

import (
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/ui"
)

// Tone names a color role for the demos' dots, tiles and figures.
type Tone uint8

const (
	ToneNeutral Tone = iota
	ToneAccent
	ToneInfo
	ToneWarn
	ToneDanger
	ToneUp
	ToneDown
)

// Color is the tone's foreground, read where ColorsOf's colors are.
func (t Tone) Color(cx *core.Scope) core.Color {
	colors := ui.ColorsOf(cx)
	switch t {
	case ToneAccent:
		return colors.Accent()
	case ToneInfo:
		return colors.Info()
	case ToneWarn:
		return colors.Warn()
	case ToneDanger:
		return colors.Danger()
	case ToneUp:
		return colors.Up()
	case ToneDown:
		return colors.Down()
	}
	return colors.Muted()
}

// Shade is the tone's color, for what takes one that may change.
func (t Tone) Shade(cx *core.Scope) func() core.Color {
	return func() core.Color { return t.Color(cx) }
}

// Foreground is the tone's color for text and icons: the text's when
// neutral.
func (t Tone) Foreground(cx *core.Scope) func() core.Color {
	return func() core.Color {
		if t == ToneNeutral {
			return ui.ColorsOf(cx).Text()
		}
		return t.Color(cx)
	}
}

// Tint is the tone's translucent fill, read where ColorsOf's colors are.
func (t Tone) Tint(cx *core.Scope) core.Color {
	tints := ui.TintsOf(cx)
	switch t {
	case ToneAccent:
		return tints.Accent()
	case ToneInfo:
		return tints.Info()
	case ToneWarn:
		return tints.Warn()
	case ToneDanger:
		return tints.Danger()
	case ToneUp:
		return tints.Up()
	case ToneDown:
		return tints.Down()
	}
	return ui.ColorsOf(cx).Raised()
}

// Badge is the badge in the tone's color.
func (t Tone) Badge() ui.BadgeVariant {
	switch t {
	case ToneAccent:
		return ui.BadgeAccent
	case ToneInfo:
		return ui.BadgeInfo
	case ToneWarn:
		return ui.BadgeWarn
	case ToneDanger:
		return ui.BadgeDanger
	case ToneUp:
		return ui.BadgeUp
	case ToneDown:
		return ui.BadgeDown
	}
	return ui.BadgeNeutral
}

// Meter is the meter in the tone's color.
func (t Tone) Meter() ui.MeterVariant {
	switch t {
	case ToneAccent:
		return ui.MeterAccent
	case ToneInfo:
		return ui.MeterInfo
	case ToneWarn:
		return ui.MeterWarn
	case ToneDanger:
		return ui.MeterDanger
	case ToneUp:
		return ui.MeterUp
	case ToneDown:
		return ui.MeterDown
	}
	return ui.MeterNeutral
}

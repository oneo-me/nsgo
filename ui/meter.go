package ui

import "github.com/nsgo-dev/nsgo/core"

// MeterVariant is a Meter's look: the color it fills with.
type MeterVariant uint8

const (
	// MeterNeutral fills with the muted text's color. It is the default.
	MeterNeutral MeterVariant = iota
	// MeterAccent fills with the accent.
	MeterAccent
	// MeterInfo fills with the info color.
	MeterInfo
	// MeterWarn fills with the warning color.
	MeterWarn
	// MeterDanger fills with the danger color.
	MeterDanger
	// MeterUp fills with the color of a rise.
	MeterUp
	// MeterDown fills with the color of a fall.
	MeterDown
)

// fill is the color a variant fills with.
func (v MeterVariant) fill(colors Colors) core.Color {
	switch v {
	case MeterAccent:
		return colors.Accent()
	case MeterInfo:
		return colors.Info()
	case MeterWarn:
		return colors.Warn()
	case MeterDanger:
		return colors.Danger()
	case MeterUp:
		return colors.Up()
	case MeterDown:
		return colors.Down()
	}
	return colors.Muted()
}

// Meter is a horizontal meter of Value, from 0 to 1, in its Variant, with a
// Marker at a limit when it is above 0.
type Meter struct {
	Value   func() float64
	Variant MeterVariant
	Marker  float64
}

// Render implements core.Node.
func (m Meter) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	children := []core.Node{core.View{StyleFunc: func() core.Style {
		return core.Style{Width: core.Pct(float32(min(max(m.Value(), 0), 1) * 100)), Height: core.Pt(6), CornerRadius: 3, Background: m.Variant.fill(colors)}
	}}}
	if m.Marker > 0 {
		children = append(children, core.View{Style: core.Style{Position: core.Absolute, Left: core.Pct(float32(m.Marker * 100)), Top: core.Pt(-3),
			Width: core.Pt(2), Height: core.Pt(12), CornerRadius: 1}, StyleFunc: Filled(colors.Text)})
	}
	return core.View{Style: core.Style{Height: core.Pt(6), CornerRadius: 3}, StyleFunc: Filled(colors.Border), Children: children}
}

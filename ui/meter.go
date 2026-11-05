package ui

import "github.com/oneo-me/nsgo/core"

// Meter is a horizontal meter of Value, from 0 to 1, in its Tone, with a
// Marker at a limit when it is above 0.
type Meter struct {
	Value  func() float64
	Tone   Tone
	Marker float64
}

// Render implements core.Node.
func (m Meter) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	children := []core.Node{core.View{StyleFunc: func() core.Style {
		return core.Style{Width: core.Pct(float32(min(max(m.Value(), 0), 1) * 100)), Height: core.Pt(6), CornerRadius: 3, Background: m.Tone.Color(cx)}
	}}}
	if m.Marker > 0 {
		children = append(children, core.View{Style: core.Style{Position: core.Absolute, Left: core.Pct(float32(m.Marker * 100)), Top: core.Pt(-3),
			Width: core.Pt(2), Height: core.Pt(12), CornerRadius: 1}, StyleFunc: Filled(colors.Text)})
	}
	return core.View{Style: core.Style{Height: core.Pt(6), CornerRadius: 3}, StyleFunc: Filled(colors.Border), Children: children}
}

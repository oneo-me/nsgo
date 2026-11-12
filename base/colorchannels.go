package base

import "github.com/nsgo-dev/nsgo/core"

// colorNotation is a way the picker's channels spell the color, one of
// colorNotations.
type colorNotation struct {
	name     string
	space    ColorSpace
	format   ColorFormat
	channels [3]ColorChannel
	letters  [3]string
}

var colorNotations = []colorNotation{
	{"RGB", ColorRGB, ColorFormatRGB, [3]ColorChannel{ChannelRed, ChannelGreen, ChannelBlue}, [3]string{"R", "G", "B"}},
	{"HSB", ColorHSB, ColorFormatHSB, [3]ColorChannel{ChannelHue, ChannelSaturation, ChannelBrightness}, [3]string{"H", "S", "B"}},
	{"HSL", ColorHSL, ColorFormatHSL, [3]ColorChannel{ChannelHue, ChannelSaturation, ChannelLightness}, [3]string{"H", "S", "L"}},
}

// colorNotationOf is the place of the notation f writes in, the first's
// for one with none.
func colorNotationOf(f ColorFormat) int {
	for i, n := range colorNotations {
		if n.format == f {
			return i
		}
	}
	return 0
}

// ColorChannels is a ColorField of each of the three channels of the
// notation the picker around it spells the color in, which
// ColorNotationMenu chooses, each with its letter as a scrub handle.
type ColorChannels struct {
	// Number draws each field, as ColorField's.
	Number func(NumberInput) core.Node
	Class  string
	Style  core.Style
	TestID string
}

// Render implements core.Node.
func (c ColorChannels) Render(cx *core.Scope) core.Node {
	picker := colorStateOf(cx)
	return core.View{Class: c.Class, Style: c.Style, TestID: c.TestID, Children: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
		n := colorNotations[picker.notation.Get()]
		fields := make(core.Group, len(n.channels))
		for i, channel := range n.channels {
			fields[i] = ColorField{ColorSpace: n.space, Channel: channel, Letter: n.letters[i], Number: c.Number}
		}
		return fields
	})}}
}

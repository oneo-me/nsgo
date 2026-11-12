package base

import (
	"math"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
)

func TestColorsRoundTripThroughEverySpace(t *testing.T) {
	for _, c := range []core.Color{0xff0000ff, 0x00ff00ff, 0x123456ff, 0x808080ff, 0xffffffff, 0x000000ff} {
		m := colorModelOf(c)
		for _, space := range []ColorSpace{ColorRGB, ColorHSL, ColorHSB, ColorOKLCH} {
			if got := m.withTriple(space, m.triple(space)).color(); got != c {
				t.Errorf("%#x through space %d = %#x", uint32(c), space, uint32(got))
			}
		}
	}
}

func TestColorsWriteInEveryFormat(t *testing.T) {
	m := colorModelOf(0x0000ff80)
	for f, want := range map[ColorFormat]string{
		ColorFormatHex: "#0000ff80", ColorFormatRGB: "rgb(0 0 255 / 0.5)", ColorFormatHSL: "hsl(240 100% 50% / 0.5)",
		ColorFormatHSB: "hsb(240 100% 100% / 0.5)",
	} {
		if got := m.format(f); got != want {
			t.Errorf("format %d = %q, want %q", f, got, want)
		}
	}
	if got := colorModelOf(0x00ff00ff).format(ColorFormatOKLCH); got != "oklch(86.6% 0.295 142.5)" {
		t.Errorf("oklch = %q", got)
	}
}

// A gray has no hue and black no saturation: following one keeps them.
func TestGraysKeepTheirHue(t *testing.T) {
	m := colorModelOf(0xff8000ff)
	hue := m.channel(ColorHSB, ChannelHue)
	gray := m.follow(0x808080ff)
	if got := gray.channel(ColorHSB, ChannelHue); math.Abs(got-hue) > 1e-9 {
		t.Errorf("hue = %v, want %v", got, hue)
	}
	black := m.follow(0x000000ff)
	if got := black.channel(ColorHSB, ChannelSaturation); got != 100 {
		t.Errorf("saturation = %v", got)
	}
}

func TestParseColorText(t *testing.T) {
	m := colorModelOf(0x000000ff)
	for text, want := range map[string]core.Color{
		"lime": 0x00ff00ff, "ff0000": 0xff0000ff, "#00f": 0x0000ffff, "hsb(0 100% 100%)": 0xff0000ff, "hsb(0 100 100 / 50%)": 0xff000080,
	} {
		next, ok := parseColorText(m, text)
		if !ok || next.color() != want {
			t.Errorf("%q = %#x, %v, want %#x", text, uint32(next.color()), ok, uint32(want))
		}
	}
	if _, ok := parseColorText(m, "nonsense"); ok {
		t.Error("nonsense parsed")
	}
}

func TestOKLCHPastSRGBKeepsItsLightnessAndHue(t *testing.T) {
	rgb := oklchToRGB([3]float64{70, 0.4, 150})
	for _, c := range rgb {
		if c < 0 || c > 1 {
			t.Fatalf("out of gamut: %v", rgb)
		}
	}
}

func TestSpecsSnapToTheirSteps(t *testing.T) {
	s := colorSpecOf(ColorOKLCH, ChannelChroma)
	if got := s.snap(0.1234); got != 0.123 {
		t.Errorf("snap = %v", got)
	}
	if got := s.snap(9); got != 0.4 {
		t.Errorf("past the end: %v", got)
	}
	if got := colorNumber(1.50, 3); got != "1.5" {
		t.Errorf("number = %q", got)
	}
}

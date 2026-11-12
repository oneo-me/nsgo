package base

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/nsgo-dev/nsgo/core"
)

// ColorSpace is a way of writing a color by channels.
type ColorSpace uint8

const (
	ColorHSB ColorSpace = iota
	ColorRGB
	ColorHSL
	ColorOKLCH
)

// ColorChannel is a channel of a color. A ColorSlider's zero is its space's
// hue, or its first channel; a ColorField's is the whole color.
type ColorChannel uint8

const (
	ChannelNone ColorChannel = iota
	ChannelRed
	ChannelGreen
	ChannelBlue
	ChannelHue
	ChannelSaturation
	ChannelLightness
	ChannelBrightness
	ChannelChroma
	ChannelAlpha
)

// ColorFormat is how a picker writes a color as text.
type ColorFormat uint8

const (
	ColorFormatHex ColorFormat = iota
	ColorFormatRGB
	ColorFormatHSL
	ColorFormatHSB
	ColorFormatOKLCH
)

// colorSpec is a channel's range, step and decimals.
type colorSpec struct {
	max, step float64
	decimals  int
}

func (s colorSpec) snap(v float64) float64 {
	v = min(max(v, 0), s.max)
	return roundTo(min(math.Round(v/s.step)*s.step, s.max), s.decimals)
}

func (s colorSpec) fraction(v float64) float64 { return min(max(v/s.max, 0), 1) }

func (s colorSpec) at(t float64) float64 { return min(max(t, 0), 1) * s.max }

func (s colorSpec) format(v float64) string { return colorNumber(v, s.decimals) }

// colorChannelIndex is a channel's place in its space's triple, -1 for
// alpha or one the space lacks.
func colorChannelIndex(space ColorSpace, c ColorChannel) int {
	switch space {
	case ColorRGB:
		switch c {
		case ChannelRed:
			return 0
		case ChannelGreen:
			return 1
		case ChannelBlue:
			return 2
		}
	case ColorHSL, ColorHSB:
		switch {
		case c == ChannelHue:
			return 0
		case c == ChannelSaturation:
			return 1
		case c == ChannelLightness && space == ColorHSL, c == ChannelBrightness && space == ColorHSB:
			return 2
		}
	case ColorOKLCH:
		switch c {
		case ChannelLightness:
			return 0
		case ChannelChroma:
			return 1
		case ChannelHue:
			return 2
		}
	}
	return -1
}

// resolveChannel is c in space: its first when the space lacks it.
func resolveChannel(space ColorSpace, c ColorChannel) ColorChannel {
	if c == ChannelAlpha || colorChannelIndex(space, c) >= 0 {
		return c
	}
	switch space {
	case ColorRGB:
		return ChannelRed
	case ColorOKLCH:
		return ChannelLightness
	}
	return ChannelHue
}

func colorSpecOf(space ColorSpace, c ColorChannel) colorSpec {
	switch {
	case c == ChannelAlpha:
		return colorSpec{100, 1, 0}
	case space == ColorRGB:
		return colorSpec{255, 1, 0}
	case space == ColorOKLCH && c == ChannelLightness:
		return colorSpec{100, 0.1, 1}
	case space == ColorOKLCH && c == ChannelChroma:
		return colorSpec{0.4, 0.001, 3}
	case space == ColorOKLCH:
		return colorSpec{360, 0.1, 1}
	case c == ChannelHue:
		return colorSpec{360, 1, 0}
	}
	return colorSpec{100, 1, 0}
}

// colorSegments is how many gradients a channel's track takes.
func colorSegments(space ColorSpace, c ColorChannel) int {
	switch {
	case space == ColorOKLCH && c != ChannelAlpha:
		return 12
	case (space == ColorHSL || space == ColorHSB) && c == ChannelHue:
		return 6
	case space == ColorHSL && c == ChannelLightness:
		return 2
	}
	return 1
}

const (
	colorEpsilon = 1e-6
	// colorAchromatic is the OKLCH chroma below which a color has no hue.
	colorAchromatic = 2e-4
)

// colorModel is a color in every space, each keeping what its channels
// said where the color alone cannot: a gray's hue, black's saturation.
type colorModel struct {
	rgb             [3]float64
	alpha           float64
	hsl, hsb, oklch [3]float64
}

func colorComponents(c core.Color) ([3]float64, float64) {
	r, g, b, a := c.Components()
	return [3]float64{r, g, b}, a
}

func colorModelOf(c core.Color) colorModel {
	rgb, a := colorComponents(c)
	return deriveColor(rgb, a, nil)
}

// follow is c, keeping what m said.
func (m colorModel) follow(c core.Color) colorModel {
	rgb, a := colorComponents(c)
	return deriveColor(rgb, a, &m)
}

func deriveColor(rgb [3]float64, alpha float64, prev *colorModel) colorModel {
	hsb, hsl, oklch := rgbToHSB(rgb), rgbToHSL(rgb), rgbToOKLCH(rgb)
	if prev != nil {
		if hsb[1] < colorEpsilon || hsb[2] < colorEpsilon {
			hsb[0] = prev.hsb[0]
		}
		if hsb[2] < colorEpsilon {
			hsb[1] = prev.hsb[1]
		}
		if hsl[1] < colorEpsilon || hsl[2] < colorEpsilon || hsl[2] > 100-colorEpsilon {
			hsl[0] = prev.hsl[0]
		}
		if hsl[2] < colorEpsilon || hsl[2] > 100-colorEpsilon {
			hsl[1] = prev.hsl[1]
		}
		if oklch[1] < colorAchromatic {
			oklch[2] = prev.oklch[2]
		}
	}
	return colorModel{rgb: rgb, alpha: min(max(alpha, 0), 1), hsl: hsl, hsb: hsb, oklch: oklch}
}

func (m colorModel) triple(space ColorSpace) [3]float64 {
	switch space {
	case ColorRGB:
		return [3]float64{m.rgb[0] * 255, m.rgb[1] * 255, m.rgb[2] * 255}
	case ColorHSL:
		return m.hsl
	case ColorOKLCH:
		return m.oklch
	}
	return m.hsb
}

func (m colorModel) channel(space ColorSpace, c ColorChannel) float64 {
	if i := colorChannelIndex(space, c); i >= 0 {
		return m.triple(space)[i]
	}
	return m.alpha * 100
}

func (m colorModel) withChannel(space ColorSpace, c ColorChannel, v float64) colorModel {
	v = min(max(v, 0), colorSpecOf(space, c).max)
	if i := colorChannelIndex(space, c); i >= 0 {
		t := m.triple(space)
		t[i] = v
		return m.withTriple(space, t)
	}
	m.alpha = v / 100
	return m
}

func (m colorModel) withTriple(space ColorSpace, t [3]float64) colorModel {
	var rgb [3]float64
	switch space {
	case ColorRGB:
		rgb = [3]float64{t[0] / 255, t[1] / 255, t[2] / 255}
	case ColorHSL:
		rgb = hslToRGB(t)
	case ColorOKLCH:
		rgb = oklchToRGB(t)
	default:
		rgb = hsbToRGB(t)
	}
	for i := range rgb {
		rgb[i] = min(max(rgb[i], 0), 1)
	}
	next := deriveColor(rgb, m.alpha, &m)
	switch space {
	case ColorHSL:
		next.hsl = t
	case ColorHSB:
		next.hsb = t
	case ColorOKLCH:
		next.oklch = t
	}
	return next
}

// color is m as a core.Color; fully clear black stays a set color.
func (m colorModel) color() core.Color { return core.SetColor(core.Color(m.packed())) }

func (m colorModel) packed() uint32 {
	byteOf := func(v float64) uint32 { return uint32(math.Round(min(max(v, 0), 1) * 255)) }
	return byteOf(m.rgb[0])<<24 | byteOf(m.rgb[1])<<16 | byteOf(m.rgb[2])<<8 | byteOf(m.alpha)
}

func (m colorModel) opaque() core.Color {
	m.alpha = 1
	return m.color()
}

// track is the color at t along a channel's track.
func (m colorModel) track(space ColorSpace, c ColorChannel, t float64) core.Color {
	v := colorSpecOf(space, c).at(t)
	switch {
	case space == ColorHSB && c == ChannelHue:
		return m.withTriple(space, [3]float64{v, 100, 100}).opaque()
	case space == ColorHSL && c == ChannelHue:
		return m.withTriple(space, [3]float64{v, 100, 50}).opaque()
	case c == ChannelAlpha:
		return m.withChannel(space, c, v).color()
	}
	return m.withChannel(space, c, v).opaque()
}

// hueColor is the pure color of m's hue, an area's panel.
func (m colorModel) hueColor() core.Color {
	return colorModelOf(0).withTriple(ColorHSB, [3]float64{m.hsb[0], 100, 100}).opaque()
}

func (m colorModel) format(f ColorFormat) string {
	packed := m.packed()
	alpha := ""
	if packed&0xff != 0xff {
		alpha = " / " + colorNumber(m.alpha, 2)
	}
	switch f {
	case ColorFormatRGB:
		ch := func(v float64) string { return strconv.FormatFloat(math.Round(v*255), 'f', -1, 64) }
		return fmt.Sprintf("rgb(%s %s %s%s)", ch(m.rgb[0]), ch(m.rgb[1]), ch(m.rgb[2]), alpha)
	case ColorFormatHSL:
		return fmt.Sprintf("hsl(%s %s%% %s%%%s)", colorNumber(m.hsl[0], 1), colorNumber(m.hsl[1], 1), colorNumber(m.hsl[2], 1), alpha)
	case ColorFormatHSB:
		return fmt.Sprintf("hsb(%s %s%% %s%%%s)", colorNumber(m.hsb[0], 1), colorNumber(m.hsb[1], 1), colorNumber(m.hsb[2], 1), alpha)
	case ColorFormatOKLCH:
		return fmt.Sprintf("oklch(%s%% %s %s%s)", colorNumber(m.oklch[0], 1), colorNumber(m.oklch[1], 3), colorNumber(m.oklch[2], 1), alpha)
	}
	if packed&0xff == 0xff {
		return fmt.Sprintf("#%06x", packed>>8)
	}
	return fmt.Sprintf("#%08x", packed)
}

// parseColorText reads text in any CSS form, as bare hex digits or as
// hsb(), and applies it to m.
func parseColorText(m colorModel, text string) (colorModel, bool) {
	text = strings.TrimSpace(text)
	if c, err := core.ParseColor(text); err == nil {
		return m.follow(c), true
	}
	if n := len(text); n == 3 || n == 4 || n == 6 || n == 8 {
		if c, err := core.ParseColor("#" + text); err == nil {
			return m.follow(c), true
		}
	}
	args, ok := strings.CutPrefix(text, "hsb(")
	if !ok {
		return m, false
	}
	if args, ok = strings.CutSuffix(args, ")"); !ok {
		return m, false
	}
	channels, alphaText, hasAlpha := strings.Cut(args, "/")
	fields := strings.FieldsFunc(channels, func(r rune) bool { return r == ' ' || r == ',' })
	if len(fields) != 3 {
		return m, false
	}
	var t [3]float64
	for i, f := range fields {
		v, err := strconv.ParseFloat(strings.TrimSuffix(f, "%"), 64)
		if err != nil {
			return m, false
		}
		t[i] = v
	}
	alpha := 1.0
	if hasAlpha {
		a := strings.TrimSpace(alphaText)
		percent := strings.HasSuffix(a, "%")
		v, err := strconv.ParseFloat(strings.TrimSuffix(a, "%"), 64)
		if err != nil {
			return m, false
		}
		if percent {
			v /= 100
		}
		alpha = v
	}
	m.alpha = min(max(alpha, 0), 1)
	return m.withTriple(ColorHSB, [3]float64{remEuclid(t[0], 360), min(max(t[1], 0), 100), min(max(t[2], 0), 100)}), true
}

// colorNumber is v at most decimals places, without trailing zeros.
func colorNumber(v float64, decimals int) string {
	text := strconv.FormatFloat(roundTo(v, decimals), 'f', decimals, 64)
	if strings.Contains(text, ".") {
		text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	}
	if text == "-0" {
		return "0"
	}
	return text
}

func roundTo(v float64, decimals int) float64 {
	scale := math.Pow10(decimals)
	return math.Round(v*scale) / scale
}

func remEuclid(v, m float64) float64 {
	r := math.Mod(v, m)
	if r < 0 {
		r += m
	}
	return r
}

func maxOf3(v [3]float64) float64 { return max(v[0], v[1], v[2]) }
func minOf3(v [3]float64) float64 { return min(v[0], v[1], v[2]) }

func hueOf(rgb [3]float64, high, d float64) float64 {
	r, g, b := rgb[0], rgb[1], rgb[2]
	switch {
	case d <= 0:
		return 0
	case high == r:
		return 60 * remEuclid((g-b)/d, 6)
	case high == g:
		return 60 * ((b-r)/d + 2)
	}
	return 60 * ((r-g)/d + 4)
}

func rgbToHSB(rgb [3]float64) [3]float64 {
	high, d := maxOf3(rgb), maxOf3(rgb)-minOf3(rgb)
	s := 0.0
	if high > 0 {
		s = d / high
	}
	return [3]float64{hueOf(rgb, high, d), s * 100, high * 100}
}

func hsbToRGB(t [3]float64) [3]float64 {
	h, s, v := t[0], t[1]/100, t[2]/100
	f := func(n float64) float64 {
		k := remEuclid(n+h/60, 6)
		return v - v*s*min(max(min(k, 4-k), 0), 1)
	}
	return [3]float64{f(5), f(3), f(1)}
}

func rgbToHSL(rgb [3]float64) [3]float64 {
	high, low := maxOf3(rgb), minOf3(rgb)
	d, l := high-low, (high+low)/2
	s := 0.0
	if d > 0 {
		s = d / (1 - math.Abs(2*l-1))
	}
	return [3]float64{hueOf(rgb, high, d), s * 100, l * 100}
}

func hslToRGB(t [3]float64) [3]float64 {
	h, s, l := t[0], t[1]/100, t[2]/100
	a := s * min(l, 1-l)
	f := func(n float64) float64 {
		k := remEuclid(n+h/30, 12)
		return l - a*min(max(min(k-3, 9-k), -1), 1)
	}
	return [3]float64{f(0), f(8), f(4)}
}

func toLinear(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func fromLinear(c float64) float64 {
	if c <= 0.0031308 {
		return 12.92 * c
	}
	return 1.055*math.Pow(c, 1/2.4) - 0.055
}

func rgbToOKLCH(rgb [3]float64) [3]float64 {
	r, g, b := toLinear(rgb[0]), toLinear(rgb[1]), toLinear(rgb[2])
	l := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	m := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	s := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
	lightness := 0.2104542553*l + 0.793617785*m - 0.0040720468*s
	a := 1.9779984951*l - 2.428592205*m + 0.4505937099*s
	bb := 0.0259040371*l + 0.7827717662*m - 0.808675766*s
	chroma := math.Hypot(a, bb)
	hue := 0.0
	if chroma >= colorAchromatic {
		hue = remEuclid(math.Atan2(bb, a)*180/math.Pi, 360)
	}
	return [3]float64{lightness * 100, chroma, hue}
}

// oklchToLinear is OKLCH in linear sRGB, unclamped.
func oklchToLinear(t [3]float64) [3]float64 {
	l, h := t[0]/100, t[2]*math.Pi/180
	a, b := t[1]*math.Cos(h), t[1]*math.Sin(h)
	cube := func(v float64) float64 { return v * v * v }
	l3 := cube(l + 0.3963377774*a + 0.2158037573*b)
	m3 := cube(l - 0.1055613458*a - 0.0638541728*b)
	s3 := cube(l - 0.0894841775*a - 1.291485548*b)
	return [3]float64{4.0767416621*l3 - 3.3077115913*m3 + 0.2309699292*s3,
		-1.2684380046*l3 + 2.6097574011*m3 - 0.3413193965*s3,
		-0.0041960863*l3 - 0.7034186147*m3 + 1.707614701*s3}
}

// oklchToRGB is OKLCH in sRGB: a color past sRGB is the one of its
// lightness and hue with the most chroma sRGB has, CSS Color 4's mapping.
func oklchToRGB(t [3]float64) [3]float64 {
	inside := func(v [3]float64) bool {
		for _, c := range v {
			if c < -1e-6 || c > 1+1e-6 {
				return false
			}
		}
		return true
	}
	linear := oklchToLinear(t)
	if !inside(linear) {
		low, high := 0.0, t[1]
		for range 20 {
			mid := (low + high) / 2
			if inside(oklchToLinear([3]float64{t[0], mid, t[2]})) {
				low = mid
			} else {
				high = mid
			}
		}
		linear = oklchToLinear([3]float64{t[0], low, t[2]})
	}
	var out [3]float64
	for i, c := range linear {
		out[i] = fromLinear(min(max(c, 0), 1))
	}
	return out
}

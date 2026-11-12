package textview

import (
	"math"
	"strconv"
	"strings"

	"github.com/nsgo-dev/nsgo/core"
)

// cssColors are CSS's basic named colors, what HTML in documents uses.
var cssColors = map[string]uint32{
	"black": 0x000000, "silver": 0xc0c0c0, "gray": 0x808080, "grey": 0x808080, "white": 0xffffff,
	"maroon": 0x800000, "red": 0xff0000, "purple": 0x800080, "fuchsia": 0xff00ff, "magenta": 0xff00ff,
	"green": 0x008000, "lime": 0x00ff00, "olive": 0x808000, "yellow": 0xffff00, "navy": 0x000080,
	"blue": 0x0000ff, "teal": 0x008080, "aqua": 0x00ffff, "cyan": 0x00ffff, "orange": 0xffa500,
	"pink": 0xffc0cb, "gold": 0xffd700, "brown": 0xa52a2a,
}

// parseCSSColor reads a CSS color: #rgb, #rgba, #rrggbb, #rrggbbaa,
// rgb() and rgba(), hsl() and hsla() in either syntax, or a basic name.
func parseCSSColor(s string) (core.Color, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if hex, ok := strings.CutPrefix(s, "#"); ok {
		return parseHexColor(hex)
	}
	if v, ok := cssColors[s]; ok {
		return core.RGB(v), true
	}
	open := strings.IndexByte(s, '(')
	if open < 0 || !strings.HasSuffix(s, ")") {
		return 0, false
	}
	fn, args := s[:open], s[open+1:len(s)-1]
	var alpha string
	if a, b, ok := strings.Cut(args, "/"); ok {
		args, alpha = a, b
	}
	parts := strings.FieldsFunc(args, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
	if len(parts) == 4 && alpha == "" {
		parts, alpha = parts[:3], parts[3]
	}
	if len(parts) != 3 {
		return 0, false
	}
	a := 1.0
	if alpha = strings.TrimSpace(alpha); alpha != "" {
		v, ok := cssNumber(alpha, 1)
		if !ok {
			return 0, false
		}
		a = v
	}
	var r, g, b float64
	switch fn {
	case "rgb", "rgba":
		var c [3]float64
		for i, p := range parts {
			v, ok := cssNumber(p, 255)
			if !ok {
				return 0, false
			}
			c[i] = v / 255
		}
		r, g, b = c[0], c[1], c[2]
	case "hsl", "hsla":
		h, err := strconv.ParseFloat(strings.TrimSuffix(parts[0], "deg"), 64)
		sat, ok1 := cssNumber(parts[1], 1)
		light, ok2 := cssNumber(parts[2], 1)
		if err != nil || !ok1 || !ok2 {
			return 0, false
		}
		r, g, b = hslToRGB(h, sat, light)
	default:
		return 0, false
	}
	clamp := func(v float64) uint32 { return uint32(math.Round(min(max(v, 0), 1) * 255)) }
	return core.Color(clamp(r)<<24 | clamp(g)<<16 | clamp(b)<<8 | clamp(a)), true
}

// cssNumber reads a number, or a percentage of full.
func cssNumber(s string, full float64) (float64, bool) {
	s = strings.TrimSpace(s)
	pct := strings.HasSuffix(s, "%")
	v, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	if pct {
		v = v / 100 * full
	}
	return v, true
}

func parseHexColor(hex string) (core.Color, bool) {
	switch len(hex) {
	case 3, 4:
		var b strings.Builder
		for _, c := range hex {
			b.WriteRune(c)
			b.WriteRune(c)
		}
		hex = b.String()
	case 6, 8:
	default:
		return 0, false
	}
	if len(hex) == 6 {
		hex += "ff"
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 0, false
	}
	return core.Color(v), true
}

func hslToRGB(h, s, l float64) (r, g, b float64) {
	h = math.Mod(math.Mod(h, 360)+360, 360) / 360
	s, l = min(max(s, 0), 1), min(max(l, 0), 1)
	f := func(n float64) float64 {
		k := math.Mod(n+h*12, 12)
		a := s * min(l, 1-l)
		return l - a*max(-1, min(k-3, 9-k, 1))
	}
	return f(0), f(8), f(4)
}

package tsx

import (
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/oneo-me/nsgo/core"
)

// CSS values in style objects (NSH's css.rs): lengths, angles, times and
// timing functions; core parses the lists of box-shadow, filter and
// gradients. Lengths are points: px and bare numbers are points, rem is
// 16 points.

const rem = 16

// splitTop splits s at sep outside parentheses and brackets.
func splitTop(s string, sep rune) []string {
	var parts []string
	depth, start := 0, 0
	for i, c := range s {
		switch {
		case c == '(' || c == '[':
			depth++
		case c == ')' || c == ']':
			depth--
		case c == sep && depth == 0:
			parts = append(parts, strings.TrimSpace(s[start:i]))
			start = i + len(string(c))
		}
	}
	return append(parts, strings.TrimSpace(s[start:]))
}

// words are the whitespace-separated words of s, keeping functions whole.
func words(s string) []string {
	var out []string
	depth, start := 0, -1
	for i, c := range s {
		switch {
		case c == '(' || c == '[':
			depth++
			if start < 0 {
				start = i
			}
		case c == ')' || c == ']':
			depth--
		case unicode.IsSpace(c) && depth == 0:
			if start >= 0 {
				out = append(out, s[start:i])
				start = -1
			}
		default:
			if start < 0 {
				start = i
			}
		}
	}
	if start >= 0 {
		out = append(out, s[start:])
	}
	return out
}

func cssNumber(s string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f, err == nil && !math.IsInf(f, 0) && !math.IsNaN(f)
}

// cssLength reads a length in points: 4, 4px, 0.5rem, 1pt.
func cssLength(s string) (core.Points, bool) {
	s = strings.TrimSpace(s)
	scale := 1.0
	switch {
	case strings.HasSuffix(s, "px"):
		s = s[:len(s)-2]
	case strings.HasSuffix(s, "pt"):
		s, scale = s[:len(s)-2], 4.0/3
	case strings.HasSuffix(s, "rem"):
		s, scale = s[:len(s)-3], rem
	}
	f, ok := cssNumber(s)
	return core.Points(f * scale), ok
}

// cssLengthOrPercent reads a length or a percentage.
func cssLengthOrPercent(s string) (core.Length, bool) {
	if p, ok := strings.CutSuffix(strings.TrimSpace(s), "%"); ok {
		f, ok := cssNumber(p)
		return core.Pct(float32(f)), ok
	}
	l, ok := cssLength(s)
	return core.Pt(l), ok
}

// cssAngle reads an angle in degrees: 45deg, 0.5turn, 1rad, 100grad, 0.
func cssAngle(s string) (float32, bool) {
	s = strings.TrimSpace(s)
	for _, u := range []struct {
		suffix string
		scale  float64
	}{{"deg", 1}, {"grad", 0.9}, {"rad", 180 / math.Pi}, {"turn", 360}} {
		if n, ok := strings.CutSuffix(s, u.suffix); ok {
			f, ok := cssNumber(n)
			return float32(f * u.scale), ok
		}
	}
	f, ok := cssNumber(s)
	return 0, ok && f == 0
}

// cssAmount reads a number or a percentage as a factor: 1.2, 120%.
func cssAmount(s string) (float32, bool) {
	if p, ok := strings.CutSuffix(strings.TrimSpace(s), "%"); ok {
		f, ok := cssNumber(p)
		return float32(f / 100), ok
	}
	f, ok := cssNumber(s)
	return float32(f), ok
}

func cssColor(s string) (core.Color, bool) {
	c, err := core.ParseColor(s)
	return c, err == nil
}

// cssDuration reads a time in milliseconds: 150, "150ms", "0.2s".
func cssDuration(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if n, ok := strings.CutSuffix(s, "ms"); ok {
		return cssNumber(n)
	}
	if n, ok := strings.CutSuffix(s, "s"); ok {
		f, ok := cssNumber(n)
		return f * 1000, ok
	}
	return cssNumber(s)
}

// cssTimingFunction reads linear, ease, ease-in, ease-out, ease-in-out
// or cubic-bezier(...).
func cssTimingFunction(s string) (core.Curve, bool) {
	switch s = strings.TrimSpace(s); s {
	case "linear":
		return core.Curve{}, true
	case "ease":
		return core.Curve{X1: 0.25, Y1: 0.1, X2: 0.25, Y2: 1}, true
	case "ease-in":
		return core.EaseIn, true
	case "ease-out":
		return core.EaseOut, true
	case "ease-in-out":
		return core.EaseInOut, true
	}
	if args, ok := strings.CutPrefix(s, "cubic-bezier("); ok {
		parts := splitTop(strings.TrimSuffix(args, ")"), ',')
		if len(parts) != 4 {
			return core.Curve{}, false
		}
		var v [4]float32
		for i, p := range parts {
			f, ok := cssNumber(p)
			if !ok {
				return core.Curve{}, false
			}
			v[i] = float32(f)
		}
		return core.Curve{X1: v[0], Y1: v[1], X2: v[2], Y2: v[3]}, true
	}
	return core.Curve{}, false
}

func cssShadows(s string) ([]core.BoxShadow, bool) {
	v, err := core.ParseBoxShadows(s)
	return v, err == nil
}

func cssGradients(s string) ([]core.Gradient, bool) {
	v, err := core.ParseGradients(s)
	return v, err == nil
}

func cssFilters(s string) ([]core.Filter, bool) {
	v, err := core.ParseFilters(s)
	return v, err == nil
}

func cssObjectPosition(s string) (core.ObjectPosition, bool) {
	v, err := core.ParseObjectPosition(s)
	return v, err == nil
}

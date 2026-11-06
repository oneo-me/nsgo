package core

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// CSS values (NSH's css.rs): box-shadow, background-image's gradients,
// filter and object-position lists, which style strings and Tailwind's
// arbitrary values write. Lengths are points: px and bare numbers are
// points, rem is 16 points.

// ParseBoxShadows reads a CSS box-shadow list; none is no shadows.
func ParseBoxShadows(s string) ([]BoxShadow, error) {
	sh, ok := cssShadows(s)
	if !ok {
		return nil, fmt.Errorf("core: %q is no box-shadow", s)
	}
	return sh, nil
}

// ParseGradients reads a CSS background-image list of linear-, radial-
// and conic-gradient() and their repeating forms; none is no gradients.
func ParseGradients(s string) ([]Gradient, error) {
	g, ok := cssGradients(s)
	if !ok {
		return nil, fmt.Errorf("core: %q is no gradient list", s)
	}
	return g, nil
}

// ParseFilters reads a CSS filter list; none is no filters.
func ParseFilters(s string) ([]Filter, error) {
	f, ok := cssFilters(s)
	if !ok {
		return nil, fmt.Errorf("core: %q is no filter list", s)
	}
	return f, nil
}

// ParseObjectPosition reads CSS's <position>: one value, two, or four
// (edges with offsets).
func ParseObjectPosition(s string) (ObjectPosition, error) {
	p, ok := cssObjectPosition(s)
	if !ok {
		return ObjectPosition{}, fmt.Errorf("core: %q is no position", s)
	}
	return p, nil
}

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
func cssLength(s string) (Points, bool) {
	s = strings.TrimSpace(s)
	scale := 1.0
	switch {
	case strings.HasSuffix(s, "px"):
		s = s[:len(s)-2]
	case strings.HasSuffix(s, "pt"):
		s, scale = s[:len(s)-2], 4.0/3
	case strings.HasSuffix(s, "rem"):
		s, scale = s[:len(s)-3], 16
	}
	f, ok := cssNumber(s)
	return Points(f * scale), ok
}

// cssLengthOrPercent reads a length or a percentage.
func cssLengthOrPercent(s string) (Length, bool) {
	if p, ok := strings.CutSuffix(strings.TrimSpace(s), "%"); ok {
		f, ok := cssNumber(p)
		return Pct(float32(f)), ok
	}
	l, ok := cssLength(s)
	return Pt(l), ok
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

func cssColor(s string) (Color, bool) {
	c, err := ParseColor(s)
	return c, err == nil
}

// cssShadows reads a box-shadow list; none is empty.
func cssShadows(s string) ([]BoxShadow, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "none" {
		return nil, true
	}
	var out []BoxShadow
	for _, part := range splitTop(s, ',') {
		sh, ok := cssShadow(part)
		if !ok {
			return nil, false
		}
		out = append(out, sh)
	}
	return out, true
}

// cssShadow reads [inset] <x> <y> [<blur> [<spread>]] [<color>].
func cssShadow(s string) (BoxShadow, bool) {
	sh := BoxShadow{Color: CurrentColor}
	var lengths []Points
	colorSeen := false
	for _, w := range words(s) {
		if w == "inset" {
			sh.Inset = true
		} else if l, ok := cssLength(w); ok {
			lengths = append(lengths, l)
		} else if c, ok := cssColor(w); ok && !colorSeen {
			sh.Color, colorSeen = c, true
		} else {
			return sh, false
		}
	}
	switch len(lengths) {
	case 4:
		sh.Spread = lengths[3]
		fallthrough
	case 3:
		sh.Blur = max(lengths[2], 0)
		fallthrough
	case 2:
		sh.Offset = Point{X: lengths[0], Y: lengths[1]}
	default:
		return sh, false
	}
	return sh, true
}

// cssGradients reads a background-image list of gradients; none is
// empty.
func cssGradients(s string) ([]Gradient, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "none" {
		return nil, true
	}
	var out []Gradient
	for _, part := range splitTop(s, ',') {
		g, ok := cssGradient(part)
		if !ok {
			return nil, false
		}
		out = append(out, g)
	}
	return out, true
}

var colorSpaces = map[string]ColorSpace{"srgb": ColorSpaceSRGB, "srgb-linear": ColorSpaceLinearSRGB,
	"oklab": ColorSpaceOklab, "oklch": ColorSpaceOklch}

// cssGradient reads linear-gradient(), radial-gradient(),
// conic-gradient() and their repeating- forms.
func cssGradient(s string) (Gradient, bool) {
	var g Gradient
	s = strings.TrimSpace(s)
	open := strings.IndexByte(s, '(')
	if open < 0 || !strings.HasSuffix(s, ")") {
		return g, false
	}
	name, args := strings.TrimSpace(s[:open]), s[open+1:len(s)-1]
	name, g.Repeating = strings.CutPrefix(name, "repeating-")
	switch name {
	case "linear-gradient":
		g.Kind, g.Angle = GradientLinear, 180
	case "radial-gradient":
		g.Kind, g.Extent = GradientRadial, ExtentFarthestCorner
	case "conic-gradient":
		g.Kind = GradientConic
	default:
		return g, false
	}
	parts := splitTop(args, ',')
	explicit := false
	if len(parts) > 0 {
		first := words(parts[0])
		var geometry []string
		for i := 0; i < len(first); i++ {
			if first[i] == "in" && i+1 < len(first) {
				space, ok := colorSpaces[first[i+1]]
				if !ok {
					return g, false
				}
				g.Space, explicit = space, true
				i++
				// A hue method (shorter hue, ...) goes with polar spaces.
				if i+2 < len(first) && first[i+2] == "hue" {
					i += 2
				}
				continue
			}
			geometry = append(geometry, first[i])
		}
		configured := configureGradient(&g, geometry)
		if explicit && !configured {
			return g, false
		}
		if configured {
			parts = parts[1:]
		}
	}
	modern := false
	for _, part := range parts {
		w := words(part)
		if len(w) == 0 || len(w) > 3 {
			return g, false
		}
		c, ok := cssColor(w[0])
		if !ok {
			return g, false
		}
		modern = modern || !legacyColor(w[0])
		if len(w) == 1 {
			g.Stops = append(g.Stops, ColorStop{Color: c})
		}
		for _, p := range w[1:] {
			at, ok := stopPosition(p, g.Kind == GradientConic)
			if !ok {
				return g, false
			}
			g.Stops = append(g.Stops, ColorStop{Color: c, At: at})
		}
	}
	switch len(g.Stops) {
	case 0:
		return g, false
	case 1:
		g.Stops = append(g.Stops, g.Stops[0])
	}
	// Without a method, CSS mixes in Oklab unless every color is a legacy
	// sRGB one.
	if !explicit && modern {
		g.Space = ColorSpaceOklab
	}
	return g, true
}

// legacyColor reports whether s is a legacy sRGB color: hex, rgb(),
// hsl(), hwb() or a name.
func legacyColor(s string) bool {
	s = strings.ToLower(s)
	for _, p := range []string{"#", "rgb", "hsl", "hwb"} {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return !strings.Contains(s, "(")
}

func stopPosition(s string, conic bool) (Length, bool) {
	s = strings.TrimSpace(s)
	if p, ok := strings.CutSuffix(s, "%"); ok {
		f, ok := cssNumber(p)
		return Pct(float32(f)), ok
	}
	if conic {
		a, ok := cssAngle(s)
		return Pct(a / 360 * 100), ok
	}
	l, ok := cssLength(s)
	if l == 0 {
		return Pct(0), ok
	}
	return Pt(l), ok
}

// configureGradient reads a gradient's geometry argument; false when it
// is a color stop.
func configureGradient(g *Gradient, w []string) bool {
	if len(w) == 0 {
		return true
	}
	switch g.Kind {
	case GradientLinear:
		if w[0] == "to" {
			h, v := 0, 0
			for _, side := range w[1:] {
				switch side {
				case "top":
					v = -1
				case "bottom":
					v = 1
				case "left":
					h = -1
				case "right":
					h = 1
				default:
					return false
				}
			}
			switch {
			case h == 0 && v == -1:
				g.Angle = 0
			case h == 1 && v == 0:
				g.Angle = 90
			case h == 0 && v == 1:
				g.Angle = 180
			case h == -1 && v == 0:
				g.Angle = 270
			case h == 0 && v == 0:
				return false
			default:
				g.ToCorner = map[[2]int]GradientCorner{{-1, -1}: ToTopLeft, {1, -1}: ToTopRight,
					{-1, 1}: ToBottomLeft, {1, 1}: ToBottomRight}[[2]int{h, v}]
			}
			return true
		}
		a, ok := cssAngle(w[0])
		if ok {
			g.Angle = a
		}
		return ok
	case GradientRadial:
		any := false
		for i := 0; i < len(w); i++ {
			switch w[i] {
			case "circle":
				g.Circle = true
			case "ellipse":
				g.Circle = false
			case "closest-side":
				g.Extent = ExtentClosestSide
			case "closest-corner":
				g.Extent = ExtentClosestCorner
			case "farthest-side":
				g.Extent = ExtentFarthestSide
			case "farthest-corner":
				g.Extent = ExtentFarthestCorner
			case "at":
				p, ok := cssObjectPosition(strings.Join(w[i+1:], " "))
				g.Center = p
				return ok
			default:
				return any
			}
			any = true
		}
		return any
	default:
		any := false
		for i := 0; i < len(w); i++ {
			switch w[i] {
			case "from":
				if i+1 >= len(w) {
					return false
				}
				a, ok := cssAngle(w[i+1])
				if !ok {
					return false
				}
				g.Angle = a
				i++
			case "at":
				p, ok := cssObjectPosition(strings.Join(w[i+1:], " "))
				g.Center = p
				return ok
			default:
				return any
			}
			any = true
		}
		return any
	}
}

// cssObjectPosition reads CSS's <position>: one value, two, or four
// (edges with offsets).
func cssObjectPosition(s string) (ObjectPosition, bool) {
	type word struct {
		kw  string
		len Length
	}
	read := func(w string) (word, bool) {
		switch w {
		case "left", "right", "top", "bottom", "center":
			return word{kw: w}, true
		}
		l, ok := cssLengthOrPercent(w)
		return word{len: l}, ok
	}
	start, end, center := PositionAxis{Offset: Pct(0)}, PositionAxis{Offset: Pct(100)}, PositionAxis{}
	horizontal := func(w word) (PositionAxis, bool) {
		switch w.kw {
		case "left":
			return start, true
		case "center":
			return center, true
		case "right":
			return end, true
		case "":
			return PositionAxis{Offset: w.len}, true
		}
		return center, false
	}
	vertical := func(w word) (PositionAxis, bool) {
		switch w.kw {
		case "top":
			return start, true
		case "center":
			return center, true
		case "bottom":
			return end, true
		case "":
			return PositionAxis{Offset: w.len}, true
		}
		return center, false
	}
	f := strings.Fields(s)
	var p ObjectPosition
	switch len(f) {
	case 1:
		w, ok := read(f[0])
		if !ok {
			return p, false
		}
		if w.kw == "top" || w.kw == "bottom" {
			p.Y, ok = vertical(w)
			return p, ok
		}
		p.X, ok = horizontal(w)
		return p, ok
	case 2:
		a, ok1 := read(f[0])
		b, ok2 := read(f[1])
		if !ok1 || !ok2 {
			return p, false
		}
		if a.kw != "" && b.kw != "" && (a.kw == "top" || a.kw == "bottom" || b.kw == "left" || b.kw == "right") {
			a, b = b, a
		}
		var okx, oky bool
		p.X, okx = horizontal(a)
		p.Y, oky = vertical(b)
		return p, okx && oky
	case 4:
		var hasX, hasY bool
		for i := 0; i < 4; i += 2 {
			o, ok := cssLengthOrPercent(f[i+1])
			if !ok {
				return p, false
			}
			switch f[i] {
			case "left":
				p.X, hasX = PositionAxis{Offset: o}, !hasX
			case "right":
				p.X, hasX = PositionAxis{Offset: o, FromEnd: true}, !hasX
			case "top":
				p.Y, hasY = PositionAxis{Offset: o}, !hasY
			case "bottom":
				p.Y, hasY = PositionAxis{Offset: o, FromEnd: true}, !hasY
			default:
				return p, false
			}
		}
		return p, hasX && hasY
	}
	return p, false
}

// cssFilters reads a filter list; none is empty.
func cssFilters(s string) ([]Filter, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "none" {
		return nil, true
	}
	var out []Filter
	for _, w := range words(s) {
		f, ok := cssFilter(w)
		if !ok {
			return nil, false
		}
		out = append(out, f)
	}
	return out, true
}

func cssFilter(s string) (Filter, bool) {
	open := strings.IndexByte(s, '(')
	if open < 0 || !strings.HasSuffix(s, ")") {
		return Filter{}, false
	}
	name, a := s[:open], strings.TrimSpace(s[open+1:len(s)-1])
	factor := func(clamp bool) (float32, bool) {
		if a == "" {
			return 1, true
		}
		f, ok := cssAmount(a)
		if clamp {
			f = min(max(f, 0), 1)
		}
		return f, ok
	}
	switch name {
	case "blur":
		if a == "" {
			return Blur(0), true
		}
		l, ok := cssLength(a)
		return Blur(l), ok
	case "brightness":
		f, ok := factor(false)
		return Brightness(f), ok
	case "contrast":
		f, ok := factor(false)
		return Contrast(f), ok
	case "saturate":
		f, ok := factor(false)
		return Saturate(f), ok
	case "grayscale":
		f, ok := factor(true)
		return Grayscale(f), ok
	case "invert":
		f, ok := factor(true)
		return Invert(f), ok
	case "opacity":
		f, ok := factor(true)
		return Fade(f), ok
	case "sepia":
		f, ok := factor(true)
		return Sepia(f), ok
	case "hue-rotate":
		if a == "" {
			return HueRotate(0), true
		}
		d, ok := cssAngle(a)
		return HueRotate(d), ok
	case "drop-shadow":
		sh, ok := cssShadow(a)
		return DropShadow(sh.Offset, sh.Blur, sh.Color), ok
	}
	return Filter{}, false
}

// ParseGridLines reads grid-row or grid-column: a placement, or a start
// and an end apart by a slash ("1 / 3", "span 2", "header / footer").
func ParseGridLines(s string) (GridLines, error) {
	start, end, hasEnd := strings.Cut(s, "/")
	var g GridLines
	var err error
	if g.Start, err = ParseGridPlacement(start); err == nil && hasEnd {
		g.End, err = ParseGridPlacement(end)
	}
	return g, err
}

// ParseGridPlacement reads one end of a grid item: auto, a line, "span
// n", a line's name, or a name and which of its lines ("a 2").
func ParseGridPlacement(s string) (GridPlacement, error) {
	f := strings.Fields(s)
	bad := fmt.Errorf("core: %q is no grid placement", s)
	switch {
	case len(f) == 1 && f[0] == "auto":
		return GridPlacement{}, nil
	case len(f) == 2 && f[0] == "span":
		n, err := strconv.Atoi(f[1])
		if err != nil || n < 1 {
			// A span to a named line is not supported.
			return GridPlacement{}, bad
		}
		return GridSpan(n), nil
	case len(f) == 1:
		if n, err := strconv.Atoi(f[0]); err == nil {
			if n == 0 {
				return GridPlacement{}, bad
			}
			return GridLine(n), nil
		}
		if !gridName(f[0]) {
			return GridPlacement{}, bad
		}
		return GridNamed(f[0], 0), nil
	case len(f) == 2 && gridName(f[0]):
		n, err := strconv.Atoi(f[1])
		if err != nil || n == 0 {
			return GridPlacement{}, bad
		}
		return GridNamed(f[0], n), nil
	}
	return GridPlacement{}, bad
}

// gridName reports whether s is a CSS identifier a grid line may have:
// no digit first, and not a keyword.
func gridName(s string) bool {
	if s == "" || s == "span" || s == "auto" || unicode.IsDigit(rune(s[0])) ||
		len(s) > 1 && s[0] == '-' && unicode.IsDigit(rune(s[1])) {
		return false
	}
	for _, c := range s {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '-' && c != '_' {
			return false
		}
	}
	return true
}

// ParseEnvLength reads a length of CSS's environment variables:
// env(safe-area-inset-*) or env(keyboard-inset-height), alone or in a
// calc() that adds lengths to it, as an inset and the points added. A
// variable's fallback is ignored, as every platform has its value.
func ParseEnvLength(s string) (EnvInset, Points, error) {
	bad := fmt.Errorf("core: %q is no env() length", s)
	inner := strings.TrimSpace(s)
	if c, ok := strings.CutPrefix(inner, "calc("); ok {
		// Spaced around its operators, as Tailwind writes calc().
		inner = strings.TrimSuffix(c, ")")
		inner = strings.ReplaceAll(strings.ReplaceAll(inner, "+", " + "), ")-", ") - ")
	}
	env, amount, sign := EnvNone, Points(0), Points(1)
	for _, w := range words(inner) {
		switch {
		case w == "+":
			sign = 1
		case w == "-":
			sign = -1
		case strings.HasPrefix(w, "env("):
			name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(w, "env("), ")"))
			if i := strings.IndexByte(name, ','); i >= 0 {
				name = strings.TrimSpace(name[:i])
			}
			if env != EnvNone || sign < 0 {
				return EnvNone, 0, bad
			}
			switch name {
			case "safe-area-inset-top", "safe-area-inset-right", "safe-area-inset-bottom", "safe-area-inset-left":
				env = EnvSafeArea
			case "keyboard-inset-height":
				env = EnvKeyboard
			default:
				return EnvNone, 0, bad
			}
		default:
			l, ok := cssLength(w)
			if !ok {
				return EnvNone, 0, bad
			}
			amount += sign * l
		}
	}
	if env == EnvNone {
		return EnvNone, 0, bad
	}
	return env, amount, nil
}

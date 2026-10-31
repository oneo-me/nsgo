package core

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// Tailwind's utilities (NSH's tailwind/utilities.rs), each as what it
// sets of a Style: what Tailwind's generated CSS sets, mapped to core's
// fields.

// utility parses a utility without variants; nil when it is unknown. env
// reports that what it sets depends on the environment.
func (t *tailwindTheme) utility(u string) (decls []classDecl, env bool) {
	if d, ok := t.keyword(u); ok {
		return []classDecl{d}, false
	}
	neg := false
	if rest, ok := strings.CutPrefix(u, "-"); ok {
		u, neg = rest, true
	}
	for _, f := range utilityFamilies {
		v, ok := strings.CutPrefix(u, f.prefix)
		if !ok || (neg && !f.negative) {
			continue
		}
		if d, env := f.parse(t, v, neg); d != nil {
			return []classDecl{d}, env
		}
	}
	return nil, false
}

// set1 makes a decl of a function of the style alone.
func set1(f func(s *Style)) classDecl { return func(s *Style, _ *classProps, _ *classEnv) { f(s) } }

// keyword parses the utilities that take no value.
func (t *tailwindTheme) keyword(u string) (classDecl, bool) {
	if f, ok := keywords[u]; ok {
		return set1(f), true
	}
	switch u {
	case "truncate":
		return func(s *Style, p *classProps, _ *classEnv) {
			// One line, ellipsized: core's no-wrap would lay the text out
			// past its box instead.
			p.lines, p.ellipsize = 1, EllipsizeTail
			s.Overflow = Hidden
		}, true
	case "text-ellipsis":
		return func(_ *Style, p *classProps, _ *classEnv) { p.ellipsize = EllipsizeTail }, true
	case "text-clip":
		return func(_ *Style, p *classProps, _ *classEnv) { p.ellipsize = EllipsizeClip }, true
	case "line-clamp-none":
		return func(_ *Style, p *classProps, _ *classEnv) { p.lines = 0 }, true
	case "pointer-events-none":
		return func(_ *Style, p *classProps, _ *classEnv) { p.pointerEvents = PointerEventsNone }, true
	case "pointer-events-auto":
		return func(_ *Style, p *classProps, _ *classEnv) { p.pointerEvents = PointerEventsAuto }, true
	case "space-x-reverse":
		return func(_ *Style, p *classProps, _ *classEnv) { p.reverseX = true }, true
	case "space-y-reverse":
		return func(_ *Style, p *classProps, _ *classEnv) { p.reverseY = true }, true
	case "transition", "transition-all", "transition-colors", "transition-opacity", "transition-shadow",
		"transition-transform":
		props := map[string]TransitionProperty{
			"transition":           TransitionOpacity | TransitionBackground | TransitionTransform | TransitionBorder | TransitionShadow,
			"transition-all":       TransitionOpacity | TransitionBackground | TransitionTransform | TransitionBorder | TransitionShadow,
			"transition-colors":    TransitionBackground | TransitionBorder,
			"transition-opacity":   TransitionOpacity,
			"transition-shadow":    TransitionShadow,
			"transition-transform": TransitionTransform,
		}[u]
		return set1(func(s *Style) {
			s.Transition.Properties = props
			if s.Transition.Duration == 0 {
				s.Transition.Duration = t.duration
			}
			if s.Transition.Easing == (Curve{}) {
				s.Transition.Easing = t.curve
			}
		}), true
	case "transition-none":
		return set1(func(s *Style) { s.Transition = Transition{} }), true
	case "border":
		return set1(func(s *Style) { s.BorderWidth = 1 }), true
	case "outline":
		return set1(func(s *Style) { s.OutlineWidth, s.OutlineStyle = 1, BorderSolid }), true
	case "ring":
		return t.ring("1")
	case "ring-inset":
		return func(_ *Style, p *classProps, _ *classEnv) { p.ringInset = true }, true
	case "shadow-none":
		return func(s *Style, p *classProps, _ *classEnv) { p.shadows, p.hasShadow = nil, true }, true
	case "blur-none":
		return func(_ *Style, p *classProps, _ *classEnv) { p.filter(FilterBlur, 0, false) }, true
	case "grayscale", "invert", "sepia":
		kind := map[string]FilterKind{"grayscale": FilterGrayscale, "invert": FilterInvert, "sepia": FilterSepia}[u]
		return func(_ *Style, p *classProps, _ *classEnv) { p.filter(kind, 1, false) }, true
	case "bg-none":
		return func(_ *Style, p *classProps, _ *classEnv) { p.gradient = gradientParts{none: true} }, true
	case "bg-radial":
		return func(_ *Style, p *classProps, _ *classEnv) { p.gradient.kind, p.gradient.set = GradientRadial, true }, true
	case "bg-conic":
		return func(_ *Style, p *classProps, _ *classEnv) { p.gradient.kind, p.gradient.set = GradientConic, true }, true
	}
	if corner, ok := strings.CutPrefix(u, "bg-linear-to-"); ok || strings.HasPrefix(u, "bg-gradient-to-") {
		if !ok {
			corner = strings.TrimPrefix(u, "bg-gradient-to-")
		}
		angle, to, ok := gradientDirection(corner)
		if !ok {
			return nil, false
		}
		return func(_ *Style, p *classProps, _ *classEnv) {
			p.gradient.kind, p.gradient.angle, p.gradient.corner, p.gradient.set = GradientLinear, angle, to, true
		}, true
	}
	return nil, false
}

func gradientDirection(d string) (float32, GradientCorner, bool) {
	switch d {
	case "t":
		return 0, 0, true
	case "r":
		return 90, 0, true
	case "b":
		return 180, 0, true
	case "l":
		return 270, 0, true
	case "tr":
		return 0, ToTopRight, true
	case "br":
		return 0, ToBottomRight, true
	case "bl":
		return 0, ToBottomLeft, true
	case "tl":
		return 0, ToTopLeft, true
	}
	return 0, 0, false
}

// keywords are the utilities that set fixed values.
var keywords = map[string]func(s *Style){
	"flex":                  func(s *Style) { s.Display, s.Direction = DisplayFlex, Row },
	"inline-flex":           func(s *Style) { s.Display, s.Direction = DisplayFlex, Row },
	"block":                 func(s *Style) { s.Display = DisplayBlock },
	"inline-block":          func(s *Style) { s.Display = DisplayBlock },
	"inline":                func(s *Style) { s.Display = DisplayBlock },
	"flow-root":             func(s *Style) { s.Display = DisplayBlock },
	"grid":                  func(s *Style) { s.Display = DisplayGrid },
	"inline-grid":           func(s *Style) { s.Display = DisplayGrid },
	"hidden":                func(s *Style) { s.Display = DisplayNone },
	"box-content":           func(s *Style) { s.BoxSizing = ContentBox },
	"static":                func(s *Style) { s.Position = Static },
	"relative":              func(s *Style) { s.Position = Relative },
	"absolute":              func(s *Style) { s.Position = Absolute },
	"fixed":                 func(s *Style) { s.Position = Fixed },
	"sticky":                func(s *Style) { s.Position = Sticky },
	"visible":               func(s *Style) { s.Visibility = VisibilityVisible },
	"invisible":             func(s *Style) { s.Visibility = VisibilityHidden },
	"collapse":              func(s *Style) { s.Visibility = VisibilityHidden },
	"overflow-hidden":       func(s *Style) { s.Overflow = Hidden },
	"overflow-clip":         func(s *Style) { s.Overflow = Hidden },
	"overflow-visible":      func(s *Style) { s.Overflow = Visible },
	"overflow-scroll":       func(s *Style) { s.Overflow = OverflowScroll },
	"overflow-auto":         func(s *Style) { s.Overflow = OverflowScroll },
	"overflow-x-hidden":     func(s *Style) { s.OverflowX = Hidden },
	"overflow-y-hidden":     func(s *Style) { s.OverflowY = Hidden },
	"overflow-x-auto":       func(s *Style) { s.OverflowX = OverflowScroll },
	"overflow-y-auto":       func(s *Style) { s.OverflowY = OverflowScroll },
	"overflow-x-scroll":     func(s *Style) { s.OverflowX = OverflowScroll },
	"overflow-y-scroll":     func(s *Style) { s.OverflowY = OverflowScroll },
	"flex-row":              func(s *Style) { s.Direction = Row },
	"flex-row-reverse":      func(s *Style) { s.Direction = RowReverse },
	"flex-col":              func(s *Style) { s.Direction = Column },
	"flex-col-reverse":      func(s *Style) { s.Direction = ColumnReverse },
	"flex-wrap":             func(s *Style) { s.Wrap = Wrap },
	"flex-wrap-reverse":     func(s *Style) { s.Wrap = WrapReverse },
	"flex-nowrap":           func(s *Style) { s.Wrap = NoWrap },
	"flex-auto":             func(s *Style) { s.FlexGrow, s.FlexShrink, s.FlexBasis = 1, 1, Length{} },
	"flex-initial":          func(s *Style) { s.FlexGrow, s.FlexShrink, s.FlexBasis = 0, 1, Length{} },
	"flex-none":             func(s *Style) { s.FlexGrow, s.FlexShrink, s.FlexBasis = 0, 0, Length{} },
	"grow":                  func(s *Style) { s.FlexGrow = 1 },
	"shrink":                func(s *Style) { s.FlexShrink = 1 },
	"justify-start":         func(s *Style) { s.JustifyContent = JustifyStart },
	"justify-normal":        func(s *Style) { s.JustifyContent = JustifyStart },
	"justify-end":           func(s *Style) { s.JustifyContent = JustifyEnd },
	"justify-center":        func(s *Style) { s.JustifyContent = JustifyCenter },
	"justify-between":       func(s *Style) { s.JustifyContent = JustifySpaceBetween },
	"justify-around":        func(s *Style) { s.JustifyContent = JustifySpaceAround },
	"justify-evenly":        func(s *Style) { s.JustifyContent = JustifySpaceEvenly },
	"justify-stretch":       func(s *Style) { s.JustifyContent = JustifyStretch },
	"justify-items-start":   func(s *Style) { s.JustifyItems = AlignStart },
	"justify-items-end":     func(s *Style) { s.JustifyItems = AlignEnd },
	"justify-items-center":  func(s *Style) { s.JustifyItems = AlignCenter },
	"justify-items-stretch": func(s *Style) { s.JustifyItems = AlignStretch },
	"justify-self-start":    func(s *Style) { s.JustifySelf = AlignStart },
	"justify-self-end":      func(s *Style) { s.JustifySelf = AlignEnd },
	"justify-self-center":   func(s *Style) { s.JustifySelf = AlignCenter },
	"justify-self-stretch":  func(s *Style) { s.JustifySelf = AlignStretch },
	"content-normal":        func(s *Style) { s.AlignContent = AlignStretch },
	"content-start":         func(s *Style) { s.AlignContent = AlignStart },
	"content-end":           func(s *Style) { s.AlignContent = AlignEnd },
	"content-center":        func(s *Style) { s.AlignContent = AlignCenter },
	"content-between":       func(s *Style) { s.AlignContent = AlignSpaceBetween },
	"content-around":        func(s *Style) { s.AlignContent = AlignSpaceAround },
	"content-evenly":        func(s *Style) { s.AlignContent = AlignSpaceEvenly },
	"content-stretch":       func(s *Style) { s.AlignContent = AlignStretch },
	"items-start":           func(s *Style) { s.AlignItems = AlignStart },
	"items-end":             func(s *Style) { s.AlignItems = AlignEnd },
	"items-center":          func(s *Style) { s.AlignItems = AlignCenter },
	"items-baseline":        func(s *Style) { s.AlignItems = AlignBaseline },
	"items-stretch":         func(s *Style) { s.AlignItems = AlignStretch },
	"self-start":            func(s *Style) { s.AlignSelf = AlignStart },
	"self-end":              func(s *Style) { s.AlignSelf = AlignEnd },
	"self-center":           func(s *Style) { s.AlignSelf = AlignCenter },
	"self-stretch":          func(s *Style) { s.AlignSelf = AlignStretch },
	"self-baseline":         func(s *Style) { s.AlignSelf = AlignBaseline },
	"place-content-center":  func(s *Style) { s.AlignContent, s.JustifyContent = AlignCenter, JustifyCenter },
	"place-content-start":   func(s *Style) { s.AlignContent, s.JustifyContent = AlignStart, JustifyStart },
	"place-content-end":     func(s *Style) { s.AlignContent, s.JustifyContent = AlignEnd, JustifyEnd },
	"place-content-between": func(s *Style) {
		s.AlignContent, s.JustifyContent = AlignSpaceBetween, JustifySpaceBetween
	},
	"place-content-stretch": func(s *Style) { s.AlignContent, s.JustifyContent = AlignStretch, JustifyStretch },
	"place-items-center":    func(s *Style) { s.AlignItems, s.JustifyItems = AlignCenter, AlignCenter },
	"place-items-start":     func(s *Style) { s.AlignItems, s.JustifyItems = AlignStart, AlignStart },
	"place-items-end":       func(s *Style) { s.AlignItems, s.JustifyItems = AlignEnd, AlignEnd },
	"place-items-stretch":   func(s *Style) { s.AlignItems, s.JustifyItems = AlignStretch, AlignStretch },
	"place-self-center":     func(s *Style) { s.AlignSelf, s.JustifySelf = AlignCenter, AlignCenter },
	"place-self-start":      func(s *Style) { s.AlignSelf, s.JustifySelf = AlignStart, AlignStart },
	"place-self-end":        func(s *Style) { s.AlignSelf, s.JustifySelf = AlignEnd, AlignEnd },
	"place-self-stretch":    func(s *Style) { s.AlignSelf, s.JustifySelf = AlignStretch, AlignStretch },
	"grid-flow-row":         func(s *Style) { s.GridAutoFlow = FlowRow },
	"grid-flow-col":         func(s *Style) { s.GridAutoFlow = FlowColumn },
	"grid-flow-dense":       func(s *Style) { s.GridAutoFlow = FlowRowDense },
	"grid-flow-row-dense":   func(s *Style) { s.GridAutoFlow = FlowRowDense },
	"grid-flow-col-dense":   func(s *Style) { s.GridAutoFlow = FlowColumnDense },
	"col-span-full":         func(s *Style) { s.GridColumn = GridLines{Start: GridLine(1), End: GridLine(-1)} },
	"row-span-full":         func(s *Style) { s.GridRow = GridLines{Start: GridLine(1), End: GridLine(-1)} },
	"auto-cols-auto":        func(s *Style) { s.GridAutoColumns = "auto" },
	"auto-cols-min":         func(s *Style) { s.GridAutoColumns = "min-content" },
	"auto-cols-max":         func(s *Style) { s.GridAutoColumns = "max-content" },
	"auto-cols-fr":          func(s *Style) { s.GridAutoColumns = "minmax(0, 1fr)" },
	"auto-rows-auto":        func(s *Style) { s.GridAutoRows = "auto" },
	"auto-rows-min":         func(s *Style) { s.GridAutoRows = "min-content" },
	"auto-rows-max":         func(s *Style) { s.GridAutoRows = "max-content" },
	"auto-rows-fr":          func(s *Style) { s.GridAutoRows = "minmax(0, 1fr)" },
	"italic":                func(s *Style) { s.FontStyle = FontStyleItalic },
	"not-italic":            func(s *Style) { s.FontStyle = FontStyleNormal },
	"underline":             func(s *Style) { s.TextDecoration = TextDecorationUnderline },
	"line-through":          func(s *Style) { s.TextDecoration = TextDecorationLineThrough },
	"no-underline":          func(s *Style) { s.TextDecoration = TextDecorationNone },
	"decoration-solid":      func(s *Style) { s.TextDecorationStyle = DecorationSolid },
	"decoration-double":     func(s *Style) { s.TextDecorationStyle = DecorationDouble },
	"decoration-dotted":     func(s *Style) { s.TextDecorationStyle = DecorationDotted },
	"decoration-dashed":     func(s *Style) { s.TextDecorationStyle = DecorationDashed },
	"decoration-wavy":       func(s *Style) { s.TextDecorationStyle = DecorationWavy },
	// Auto is the font's thickness and place, which unset lengths are.
	"decoration-auto":       func(s *Style) { s.TextDecorationThickness = Length{} },
	"decoration-from-font":  func(s *Style) { s.TextDecorationThickness = Length{} },
	"underline-offset-auto": func(s *Style) { s.TextUnderlineOffset = Length{} },
	// font-variant-numeric and font-variant-caps: each sets its own
	// features and clears those it excludes.
	"normal-nums":       variant(0, FontOldstyleNums|FontLiningNums|FontTabularNums|FontProportionalNums),
	"small-caps":        variant(FontSmallCaps, 0),
	"oldstyle-nums":     variant(FontOldstyleNums, FontLiningNums),
	"lining-nums":       variant(FontLiningNums, FontOldstyleNums),
	"tabular-nums":      variant(FontTabularNums, FontProportionalNums),
	"proportional-nums": variant(FontProportionalNums, FontTabularNums),
	"uppercase":         func(s *Style) { s.TextTransform = TextTransformUppercase },
	"lowercase":         func(s *Style) { s.TextTransform = TextTransformLowercase },
	"capitalize":        func(s *Style) { s.TextTransform = TextTransformCapitalize },
	"normal-case":       func(s *Style) { s.TextTransform = TextTransformNone },
	"text-left":         func(s *Style) { s.TextAlign = TextAlignStart },
	"text-start":        func(s *Style) { s.TextAlign = TextAlignStart },
	"text-center":       func(s *Style) { s.TextAlign = TextAlignCenter },
	"text-right":        func(s *Style) { s.TextAlign = TextAlignEnd },
	"text-end":          func(s *Style) { s.TextAlign = TextAlignEnd },
	"text-justify":      func(s *Style) { s.TextAlign = TextAlignJustify },
	"align-baseline":    func(s *Style) { s.VerticalAlign = VerticalAlignBaseline },
	"align-text-top":    func(s *Style) { s.VerticalAlign = VerticalAlignTextTop },
	"align-text-bottom": func(s *Style) { s.VerticalAlign = VerticalAlignTextBottom },
	"align-middle":      func(s *Style) { s.VerticalAlign = VerticalAlignMiddle },
	"whitespace-nowrap": func(s *Style) { s.WhiteSpace = WhiteSpaceNoWrap },
	"text-nowrap":       func(s *Style) { s.WhiteSpace = WhiteSpaceNoWrap },
	"whitespace-normal": func(s *Style) { s.WhiteSpace = WhiteSpaceNormal },
	"text-wrap":         func(s *Style) { s.WhiteSpace = WhiteSpaceNormal },
	"text-balance":      func(s *Style) { s.WhiteSpace = WhiteSpaceNormal },
	"text-pretty":       func(s *Style) { s.WhiteSpace = WhiteSpaceNormal },
	"select-none":       func(s *Style) { s.UserSelect = UserSelectNone },
	"select-text":       func(s *Style) { s.UserSelect = UserSelectText },
	"select-auto":       func(s *Style) { s.UserSelect = UserSelectNone },
	"border-solid":      func(s *Style) { s.BorderStyle = BorderSolid },
	"border-dashed":     func(s *Style) { s.BorderStyle = BorderDashed },
	"border-dotted":     func(s *Style) { s.BorderStyle = BorderDotted },
	"border-hidden":     func(s *Style) { s.BorderStyle = BorderHidden },
	"border-none":       func(s *Style) { s.BorderStyle = BorderNone },
	"outline-none":      func(s *Style) { s.OutlineStyle = BorderNone },
	"outline-hidden":    func(s *Style) { s.OutlineStyle = BorderNone },
	"outline-solid":     func(s *Style) { s.OutlineStyle = BorderSolid },
	"outline-dashed":    func(s *Style) { s.OutlineStyle = BorderDashed },
	"outline-dotted":    func(s *Style) { s.OutlineStyle = BorderDotted },
	"rounded-full":      func(s *Style) { s.CornerRadius = 9999 },
	"aspect-auto":       func(s *Style) { s.AspectRatio = 0 },
	"backface-hidden":   func(s *Style) { s.BackfaceHidden = true },
	"animate-spin":      func(s *Style) { s.Animation = KeyframeAnimation{Keyframes: KeyframesSpin} },
	"animate-ping":      func(s *Style) { s.Animation = KeyframeAnimation{Keyframes: KeyframesPing} },
	"animate-pulse":     func(s *Style) { s.Animation = KeyframeAnimation{Keyframes: KeyframesPulse} },
	"animate-bounce":    func(s *Style) { s.Animation = KeyframeAnimation{Keyframes: KeyframesBounce} },
	"animate-none":      func(s *Style) { s.Animation = KeyframeAnimation{} },
	"fill-none":         func(s *Style) { s.Fill = PaintColor(clearColor) },
	"stroke-none":       func(s *Style) { s.Stroke = PaintColor(clearColor) },
}

// family is the utilities that start with prefix and take a value.
type family struct {
	prefix   string
	negative bool // takes a leading -
	parse    func(t *tailwindTheme, v string, neg bool) (classDecl, bool)
}

// utilityFamilies are tried in order, so a longer prefix comes before
// one it starts with.
var utilityFamilies []family

func init() {
	edges := func(prefix string, neg bool, set func(s *Style, v Points)) family {
		return family{prefix, neg, func(t *tailwindTheme, v string, n bool) (classDecl, bool) {
			p, ok := t.spacingValue(v)
			if !ok {
				return nil, false
			}
			if n {
				p = -p
			}
			return set1(func(s *Style) { set(s, p) }), false
		}}
	}
	margin := func(prefix string, sides Sides, set func(s *Style, v Points)) family {
		f := edges(prefix, true, set)
		parse := f.parse
		f.parse = func(t *tailwindTheme, v string, n bool) (classDecl, bool) {
			if v == "auto" && !n {
				return set1(func(s *Style) { s.AutoMargin |= sides }), false
			}
			return parse(t, v, n)
		}
		return f
	}
	sized := func(prefix string, set func(s *Style, v Length), containers bool) family {
		return family{prefix, false, func(t *tailwindTheme, v string, _ bool) (classDecl, bool) {
			if containers {
				if w, ok := t.containers[v]; ok {
					return set1(func(s *Style) { set(s, Pt(w)) }), false
				}
				if v == "prose" {
					return set1(func(s *Style) { set(s, Pt(65*8)) }), false
				}
			}
			return t.sizeDecl(v, false, func(s *Style, l Length) { set(s, l) })
		}}
	}
	inset := func(prefix string, set func(s *Style, v Length, env *classEnv)) family {
		return family{prefix, true, func(t *tailwindTheme, v string, neg bool) (classDecl, bool) {
			l, ok := t.size(v, neg)
			if !ok {
				return nil, false
			}
			return func(s *Style, _ *classProps, env *classEnv) { set(s, l, env) }, false
		}}
	}
	start := func(s *Style, l Length, env *classEnv) {
		if env.rtl {
			s.Right = l
		} else {
			s.Left = l
		}
	}
	end := func(s *Style, l Length, env *classEnv) {
		if env.rtl {
			s.Left = l
		} else {
			s.Right = l
		}
	}
	utilityFamilies = []family{
		// Spacing.
		edges("px-", false, func(s *Style, v Points) { s.Padding.Left, s.Padding.Right = v, v }),
		edges("py-", false, func(s *Style, v Points) { s.Padding.Top, s.Padding.Bottom = v, v }),
		edges("pt-", false, func(s *Style, v Points) { s.Padding.Top = v }),
		edges("pr-", false, func(s *Style, v Points) { s.Padding.Right = v }),
		edges("pb-", false, func(s *Style, v Points) { s.Padding.Bottom = v }),
		edges("pl-", false, func(s *Style, v Points) { s.Padding.Left = v }),
		edges("ps-", false, func(s *Style, v Points) { s.Padding.Left = v }),
		edges("pe-", false, func(s *Style, v Points) { s.Padding.Right = v }),
		edges("p-", false, func(s *Style, v Points) { s.Padding = Edges{Top: v, Right: v, Bottom: v, Left: v} }),
		margin("mx-", SidesHorizontal, func(s *Style, v Points) { s.Margin.Left, s.Margin.Right = v, v }),
		margin("my-", SidesVertical, func(s *Style, v Points) { s.Margin.Top, s.Margin.Bottom = v, v }),
		margin("mt-", SideTop, func(s *Style, v Points) { s.Margin.Top = v }),
		margin("mr-", SideRight, func(s *Style, v Points) { s.Margin.Right = v }),
		margin("mb-", SideBottom, func(s *Style, v Points) { s.Margin.Bottom = v }),
		margin("ml-", SideLeft, func(s *Style, v Points) { s.Margin.Left = v }),
		margin("ms-", SideLeft, func(s *Style, v Points) { s.Margin.Left = v }),
		margin("me-", SideRight, func(s *Style, v Points) { s.Margin.Right = v }),
		margin("m-", SidesAll, func(s *Style, v Points) { s.Margin = Edges{Top: v, Right: v, Bottom: v, Left: v} }),
		{"space-x-", true, func(t *tailwindTheme, v string, neg bool) (classDecl, bool) {
			p, ok := t.spacingValue(v)
			if neg {
				p = -p
			}
			if !ok {
				return nil, false
			}
			return func(_ *Style, c *classProps, _ *classEnv) { c.spaceX, c.hasSpaceX = p, true }, false
		}},
		{"space-y-", true, func(t *tailwindTheme, v string, neg bool) (classDecl, bool) {
			p, ok := t.spacingValue(v)
			if neg {
				p = -p
			}
			if !ok {
				return nil, false
			}
			return func(_ *Style, c *classProps, _ *classEnv) { c.spaceY, c.hasSpaceY = p, true }, false
		}},
		edges("gap-x-", false, func(s *Style, v Points) { s.ColumnGap = v }),
		edges("gap-y-", false, func(s *Style, v Points) { s.RowGap = v }),
		edges("gap-", false, func(s *Style, v Points) { s.Gap = v }),
		// Sizing.
		sized("size-", func(s *Style, l Length) { s.Width, s.Height = l, l }, false),
		sized("min-w-", func(s *Style, l Length) { s.MinWidth = l }, true),
		sized("min-h-", func(s *Style, l Length) { s.MinHeight = l }, false),
		sized("max-w-", func(s *Style, l Length) { s.MaxWidth = l }, true),
		sized("max-h-", func(s *Style, l Length) { s.MaxHeight = l }, false),
		sized("w-", func(s *Style, l Length) { s.Width = l }, true),
		sized("h-", func(s *Style, l Length) { s.Height = l }, false),
		sized("basis-", func(s *Style, l Length) { s.FlexBasis = l }, true),
		// Placement.
		inset("inset-x-", func(s *Style, l Length, _ *classEnv) { s.Left, s.Right = l, l }),
		inset("inset-y-", func(s *Style, l Length, _ *classEnv) { s.Top, s.Bottom = l, l }),
		inset("inset-", func(s *Style, l Length, _ *classEnv) { s.Top, s.Right, s.Bottom, s.Left = l, l, l, l }),
		inset("top-", func(s *Style, l Length, _ *classEnv) { s.Top = l }),
		inset("right-", func(s *Style, l Length, _ *classEnv) { s.Right = l }),
		inset("bottom-", func(s *Style, l Length, _ *classEnv) { s.Bottom = l }),
		inset("left-", func(s *Style, l Length, _ *classEnv) { s.Left = l }),
		inset("start-", start),
		inset("end-", end),
		{"z-", true, func(t *tailwindTheme, v string, neg bool) (classDecl, bool) {
			n, ok := integer(v)
			if !ok {
				return nil, false
			}
			if neg {
				n = -n
			}
			return set1(func(s *Style) { s.ZIndex = n }), false
		}},
		{"aspect-", false, func(t *tailwindTheme, v string, _ bool) (classDecl, bool) {
			r, ok := t.aspect[v]
			if !ok {
				r, ok = ratio(v)
			}
			if !ok {
				return nil, false
			}
			return set1(func(s *Style) { s.AspectRatio = r }), false
		}},
		// Flexbox and grid.
		{"flex-", false, func(t *tailwindTheme, v string, _ bool) (classDecl, bool) {
			n, ok := number(v)
			if !ok {
				return nil, false
			}
			return set1(func(s *Style) { s.FlexGrow, s.FlexShrink, s.FlexBasis = n, 1, Pct(0) }), false
		}},
		{"grow-", false, numeric(func(s *Style, n float32) { s.FlexGrow = n })},
		{"shrink-", false, numeric(func(s *Style, n float32) { s.FlexShrink = n })},
		{"grid-cols-", false, tracks(func(s *Style, g GridTracks) { s.GridTemplateColumns = g })},
		{"grid-rows-", false, tracks(func(s *Style, g GridTracks) { s.GridTemplateRows = g })},
		{"auto-cols-", false, arbitraryTracks(func(s *Style, g GridTracks) { s.GridAutoColumns = g })},
		{"auto-rows-", false, arbitraryTracks(func(s *Style, g GridTracks) { s.GridAutoRows = g })},
		{"col-span-", false, span(func(s *Style, n int) { s.GridColumn = GridLines{Start: GridSpan(n)} })},
		{"row-span-", false, span(func(s *Style, n int) { s.GridRow = GridLines{Start: GridSpan(n)} })},
		{"col-start-", true, line(func(s *Style, n int) { s.GridColumn.Start = GridLine(n) })},
		{"col-end-", true, line(func(s *Style, n int) { s.GridColumn.End = GridLine(n) })},
		{"row-start-", true, line(func(s *Style, n int) { s.GridRow.Start = GridLine(n) })},
		{"row-end-", true, line(func(s *Style, n int) { s.GridRow.End = GridLine(n) })},
		// Typography.
		{"font-", false, (*tailwindTheme).font},
		{"text-", false, (*tailwindTheme).textDecl},
		{"leading-", false, (*tailwindTheme).leadingDecl},
		{"tracking-", true, (*tailwindTheme).trackingDecl},
		{"line-clamp-", false, func(t *tailwindTheme, v string, _ bool) (classDecl, bool) {
			n, ok := integer(v)
			if !ok || n < 0 {
				return nil, false
			}
			return func(_ *Style, p *classProps, _ *classEnv) { p.lines = n }, false
		}},
		{"decoration-", false, func(t *tailwindTheme, v string, _ bool) (classDecl, bool) {
			if l, ok := fontLength(v, false); ok {
				return set1(func(s *Style) { s.TextDecorationThickness = l }), false
			}
			return t.colorDecl(v, func(s *Style, c Color) { s.TextDecorationColor = c })
		}},
		{"underline-offset-", true, func(t *tailwindTheme, v string, neg bool) (classDecl, bool) {
			l, ok := fontLength(v, neg)
			if !ok {
				return nil, false
			}
			return set1(func(s *Style) { s.TextUnderlineOffset = l }), false
		}},
		// Backgrounds.
		{"bg-linear-", true, func(t *tailwindTheme, v string, neg bool) (classDecl, bool) {
			a, ok := angle(v)
			if !ok {
				return nil, false
			}
			if neg {
				a = -a
			}
			return func(_ *Style, p *classProps, _ *classEnv) {
				p.gradient.kind, p.gradient.angle, p.gradient.corner, p.gradient.set = GradientLinear, a, 0, true
			}, false
		}},
		{"bg-", false, func(t *tailwindTheme, v string, _ bool) (classDecl, bool) {
			return t.colorDecl(v, func(s *Style, c Color) { s.Background = c })
		}},
		{"from-", false, stop(func(g *gradientParts) *gradientStop { return &g.from })},
		{"via-", false, stop(func(g *gradientParts) *gradientStop { return &g.via })},
		{"to-", false, stop(func(g *gradientParts) *gradientStop { return &g.to })},
		// Borders.
		{"rounded", false, (*tailwindTheme).rounded},
		{"border-x-", false, border(func(s *Style, w Points) { s.Borders.Left, s.Borders.Right = w, w },
			func(s *Style, c Color) { s.BorderColors.Left, s.BorderColors.Right = c, c })},
		{"border-y-", false, border(func(s *Style, w Points) { s.Borders.Top, s.Borders.Bottom = w, w },
			func(s *Style, c Color) { s.BorderColors.Top, s.BorderColors.Bottom = c, c })},
		{"border-t-", false, border(func(s *Style, w Points) { s.Borders.Top = w },
			func(s *Style, c Color) { s.BorderColors.Top = c })},
		{"border-r-", false, border(func(s *Style, w Points) { s.Borders.Right = w },
			func(s *Style, c Color) { s.BorderColors.Right = c })},
		{"border-b-", false, border(func(s *Style, w Points) { s.Borders.Bottom = w },
			func(s *Style, c Color) { s.BorderColors.Bottom = c })},
		{"border-l-", false, border(func(s *Style, w Points) { s.Borders.Left = w },
			func(s *Style, c Color) { s.BorderColors.Left = c })},
		{"border-", false, border(func(s *Style, w Points) { s.BorderWidth = w },
			func(s *Style, c Color) { s.BorderColor = c })},
		{"border", false, func(t *tailwindTheme, v string, _ bool) (classDecl, bool) {
			// border-t, border-x: one point on those sides.
			sides := map[string]func(s *Style){
				"-x": func(s *Style) { s.Borders.Left, s.Borders.Right = 1, 1 },
				"-y": func(s *Style) { s.Borders.Top, s.Borders.Bottom = 1, 1 },
				"-t": func(s *Style) { s.Borders.Top = 1 }, "-r": func(s *Style) { s.Borders.Right = 1 },
				"-b": func(s *Style) { s.Borders.Bottom = 1 }, "-l": func(s *Style) { s.Borders.Left = 1 },
			}
			if f, ok := sides[v]; ok {
				return set1(f), false
			}
			return nil, false
		}},
		{"outline-offset-", false, edgesWidth(func(s *Style, w Points) { s.OutlineOffset = w })},
		{"outline-", false, func(t *tailwindTheme, v string, _ bool) (classDecl, bool) {
			if w, ok := width(v); ok {
				return set1(func(s *Style) { s.OutlineWidth, s.OutlineStyle = w, cmpOr(s.OutlineStyle, BorderSolid) }), false
			}
			return t.colorDecl(v, func(s *Style, c Color) { s.OutlineColor = c })
		}},
		{"ring-", false, func(t *tailwindTheme, v string, _ bool) (classDecl, bool) {
			if d, ok := t.ring(v); ok {
				return d, false
			}
			return t.colorDecl(v, nil, func(p *classProps, c Color) { p.ringColor = c })
		}},
		// Effects.
		{"inset-shadow-", false, func(t *tailwindTheme, v string, _ bool) (classDecl, bool) {
			sh, ok := t.insetShadow[v]
			if !ok {
				return nil, false
			}
			return func(_ *Style, p *classProps, _ *classEnv) { p.insetShadows = sh }, false
		}},
		{"shadow", false, func(t *tailwindTheme, v string, _ bool) (classDecl, bool) {
			if sh, ok := t.shadows[strings.TrimPrefix(v, "-")]; ok && (v == "" || v[0] == '-') {
				return func(_ *Style, p *classProps, _ *classEnv) { p.shadows, p.hasShadow = sh, true }, false
			}
			if c, ok := strings.CutPrefix(v, "-"); ok {
				return t.colorDecl(c, nil, func(p *classProps, c Color) { p.shadowColor = c })
			}
			return nil, false
		}},
		{"opacity-", false, func(t *tailwindTheme, v string, _ bool) (classDecl, bool) {
			o, ok := percent(v)
			if !ok {
				return nil, false
			}
			return set1(func(s *Style) { s.Opacity = Alpha(o) }), false
		}},
		{"mix-blend-", false, func(t *tailwindTheme, v string, _ bool) (classDecl, bool) {
			b, ok := blendModes[v]
			if !ok {
				return nil, false
			}
			return set1(func(s *Style) { s.BlendMode = b }), false
		}},
		{"backdrop-", false, func(t *tailwindTheme, v string, _ bool) (classDecl, bool) {
			return t.filterDecl(v, true)
		}},
		{"", true, func(t *tailwindTheme, v string, neg bool) (classDecl, bool) {
			// Filters: blur-md, brightness-50, hue-rotate-90 (negative).
			if neg && !strings.HasPrefix(v, "hue-rotate-") {
				return nil, false
			}
			d, ok := t.filterDecl(v, false)
			if ok && neg {
				inner := d
				d = func(s *Style, p *classProps, env *classEnv) {
					inner(s, p, env)
					f := &p.filters[len(p.filters)-1]
					f.amount = -f.amount
				}
			}
			return d, ok
		}},
		// Transforms.
		{"scale-x-", true, scale(func(t *Transform, f float32) { t.ScaleX = f })},
		{"scale-y-", true, scale(func(t *Transform, f float32) { t.ScaleY = f })},
		{"scale-", true, scale(func(t *Transform, f float32) { t.Scale = f })},
		{"rotate-x-", true, degrees(func(t *Transform, d float32) { t.RotateX = d })},
		{"rotate-y-", true, degrees(func(t *Transform, d float32) { t.RotateY = d })},
		{"rotate-", true, degrees(func(t *Transform, d float32) { t.Rotate = d })},
		{"skew-x-", true, degrees(func(t *Transform, d float32) { t.SkewX = d })},
		{"skew-y-", true, degrees(func(t *Transform, d float32) { t.SkewY = d })},
		{"translate-x-", true, translate(func(t *Transform, p Points, pct float32) { t.TranslateX, t.TranslateXPercent = p, pct })},
		{"translate-y-", true, translate(func(t *Transform, p Points, pct float32) { t.TranslateY, t.TranslateYPercent = p, pct })},
		{"origin-", false, func(t *tailwindTheme, v string, _ bool) (classDecl, bool) {
			o, ok := origins[v]
			if !ok {
				return nil, false
			}
			return set1(func(s *Style) { s.TransformOrigin = o }), false
		}},
		{"perspective-", false, func(t *tailwindTheme, v string, _ bool) (classDecl, bool) {
			p, ok := t.perspective[v]
			if !ok {
				if a, isArb := arbitrary(v); isArb {
					p, ok = t.length(a)
				}
			}
			if !ok {
				return nil, false
			}
			return set1(func(s *Style) { s.Perspective = p }), false
		}},
		// Transitions and animation.
		{"duration-", false, millis(func(tr *Transition, d time.Duration) { tr.Duration = d })},
		{"delay-", false, millis(func(tr *Transition, d time.Duration) { tr.Delay = d })},
		{"ease-", false, func(t *tailwindTheme, v string, _ bool) (classDecl, bool) {
			c, ok := t.ease[v]
			if !ok {
				return nil, false
			}
			return set1(func(s *Style) { s.Transition.Easing = c }), false
		}},
		// Interactivity and SVG.
		{"cursor-", false, func(t *tailwindTheme, v string, _ bool) (classDecl, bool) {
			for c := CursorAuto; c <= CursorNone; c++ {
				if c.Name() == v {
					return set1(func(s *Style) { s.Cursor = c }), false
				}
			}
			return nil, false
		}},
		{"fill-", false, func(t *tailwindTheme, v string, _ bool) (classDecl, bool) {
			return t.colorDecl(v, func(s *Style, c Color) { s.Fill = PaintColor(c) })
		}},
		{"stroke-", false, func(t *tailwindTheme, v string, _ bool) (classDecl, bool) {
			if n, ok := number(v); ok {
				return set1(func(s *Style) { s.StrokeWidth = Points(n) }), false
			}
			return t.colorDecl(v, func(s *Style, c Color) { s.Stroke = PaintColor(c) })
		}},
	}
}

func cmpOr[T comparable](a, b T) T {
	var zero T
	if a != zero {
		return a
	}
	return b
}

// spacingValue reads a spacing value: a number of steps, px, or an
// arbitrary length.
func (t *tailwindTheme) spacingValue(v string) (Points, bool) {
	switch v {
	case "px":
		return 1, true
	case "0":
		return 0, true
	}
	if a, ok := arbitrary(v); ok {
		return t.length(a)
	}
	n, ok := number(v)
	if !ok || n < 0 || math.Mod(float64(n)*4, 1) != 0 {
		return 0, false
	}
	return Points(n) * t.spacing, true
}

// size reads a size value: spacing, a fraction, full, auto or an
// arbitrary length or percentage.
func (t *tailwindTheme) size(v string, neg bool) (Length, bool) {
	sign := float32(1)
	if neg {
		sign = -1
	}
	switch v {
	case "full":
		return Pct(100 * sign), true
	case "auto":
		return Length{}, !neg
	}
	if a, b, ok := strings.Cut(v, "/"); ok {
		n, err1 := strconv.ParseFloat(a, 32)
		d, err2 := strconv.ParseFloat(b, 32)
		if err1 == nil && err2 == nil && d != 0 {
			return Pct(sign * float32(n/d*100)), true
		}
		return Length{}, false
	}
	if a, ok := arbitrary(v); ok {
		if p, ok := strings.CutSuffix(a, "%"); ok {
			f, err := strconv.ParseFloat(p, 32)
			return Pct(sign * float32(f)), err == nil
		}
	}
	p, ok := t.spacingValue(v)
	return Pt(p * Points(sign)), ok
}

// sizeDecl is size, and the window's for screen, svw, dvh and the like.
func (t *tailwindTheme) sizeDecl(v string, neg bool, set func(s *Style, l Length)) (classDecl, bool) {
	switch v {
	case "screen", "svw", "lvw", "dvw":
		return func(s *Style, _ *classProps, env *classEnv) { set(s, Pt(env.window.Width)) }, true
	case "svh", "lvh", "dvh":
		return func(s *Style, _ *classProps, env *classEnv) { set(s, Pt(env.window.Height)) }, true
	}
	l, ok := t.size(v, neg)
	if !ok {
		return nil, false
	}
	return set1(func(s *Style) { set(s, l) }), false
}

func number(v string) (float32, bool) {
	if a, ok := arbitrary(v); ok {
		v = a
	}
	f, err := strconv.ParseFloat(v, 32)
	return float32(f), err == nil && !math.IsInf(f, 0) && !math.IsNaN(f)
}

func integer(v string) (int, bool) {
	if a, ok := arbitrary(v); ok {
		v = a
	}
	n, err := strconv.Atoi(v)
	return n, err == nil
}

// percent reads 50 or [0.5] as 0.5.
func percent(v string) (float32, bool) {
	if a, ok := arbitrary(v); ok {
		if p, ok := strings.CutSuffix(a, "%"); ok {
			f, err := strconv.ParseFloat(p, 32)
			return float32(f / 100), err == nil
		}
		f, err := strconv.ParseFloat(a, 32)
		return float32(f), err == nil
	}
	n, err := strconv.Atoi(v)
	return float32(n) / 100, err == nil && n >= 0
}

func ratio(v string) (float32, bool) {
	if a, ok := arbitrary(v); ok {
		v = strings.ReplaceAll(a, " ", "")
	}
	a, b, ok := strings.Cut(v, "/")
	if !ok {
		return number(v)
	}
	n, err1 := strconv.ParseFloat(a, 32)
	d, err2 := strconv.ParseFloat(b, 32)
	return float32(n / d), err1 == nil && err2 == nil && d != 0
}

func angle(v string) (float32, bool) {
	if a, ok := arbitrary(v); ok {
		v = strings.TrimSuffix(a, "deg")
	}
	return number(v)
}

// width reads a border's or outline's width: a number of points.
func width(v string) (Points, bool) {
	if a, ok := arbitrary(v); ok {
		p, ok := defaultTailwind.length(a)
		return p, ok
	}
	n, err := strconv.Atoi(v)
	return Points(n), err == nil && n >= 0
}

// fontLength reads a decoration's length, negated when neg: a whole
// number of points, or an arbitrary length, percentages and em of the
// font size.
func fontLength(v string, neg bool) (Length, bool) {
	sign := float32(1)
	if neg {
		sign = -1
	}
	a, ok := arbitrary(v)
	if !ok {
		n, err := strconv.Atoi(v)
		return Pt(Points(sign * float32(n))), err == nil && n >= 0
	}
	a = strings.TrimSpace(a)
	if n, ok := strings.CutSuffix(a, "%"); ok {
		f, err := strconv.ParseFloat(n, 32)
		return Pct(sign * float32(f)), err == nil
	}
	if n, ok := strings.CutSuffix(a, "em"); ok && !strings.HasSuffix(n, "r") {
		f, err := strconv.ParseFloat(n, 32)
		return Pct(sign * float32(f) * 100), err == nil
	}
	p, ok := defaultTailwind.length(a)
	return Pt(Points(sign) * p), ok
}

// variant returns a utility that turns on the features on and off those
// in off, keeping the rest of the node's variant.
func variant(on, off FontVariant) func(s *Style) {
	return func(s *Style) { s.FontVariant = s.FontVariant&^off | on | FontVariantNormal }
}

func numeric(set func(s *Style, n float32)) func(*tailwindTheme, string, bool) (classDecl, bool) {
	return func(_ *tailwindTheme, v string, _ bool) (classDecl, bool) {
		n, ok := number(v)
		if !ok {
			return nil, false
		}
		return set1(func(s *Style) { set(s, n) }), false
	}
}

func edgesWidth(set func(s *Style, w Points)) func(*tailwindTheme, string, bool) (classDecl, bool) {
	return func(_ *tailwindTheme, v string, _ bool) (classDecl, bool) {
		w, ok := width(v)
		if !ok {
			return nil, false
		}
		return set1(func(s *Style) { set(s, w) }), false
	}
}

func tracks(set func(s *Style, g GridTracks)) func(*tailwindTheme, string, bool) (classDecl, bool) {
	return func(_ *tailwindTheme, v string, _ bool) (classDecl, bool) {
		var g GridTracks
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			g = GridTracks("repeat(" + v + ", minmax(0, 1fr))")
		} else if v == "none" {
			g = "none"
		} else if a, ok := arbitrary(v); ok {
			g = GridTracks(a)
		}
		if g == "" || !g.Valid() {
			return nil, false
		}
		return set1(func(s *Style) { set(s, g) }), false
	}
}

func arbitraryTracks(set func(s *Style, g GridTracks)) func(*tailwindTheme, string, bool) (classDecl, bool) {
	return func(_ *tailwindTheme, v string, _ bool) (classDecl, bool) {
		a, ok := arbitrary(v)
		if g := GridTracks(a); ok && g.Valid() {
			return set1(func(s *Style) { set(s, g) }), false
		}
		return nil, false
	}
}

func span(set func(s *Style, n int)) func(*tailwindTheme, string, bool) (classDecl, bool) {
	return func(_ *tailwindTheme, v string, _ bool) (classDecl, bool) {
		n, ok := integer(v)
		if !ok || n < 1 {
			return nil, false
		}
		return set1(func(s *Style) { set(s, n) }), false
	}
}

func line(set func(s *Style, n int)) func(*tailwindTheme, string, bool) (classDecl, bool) {
	return func(_ *tailwindTheme, v string, neg bool) (classDecl, bool) {
		n, ok := integer(v)
		if !ok || n == 0 {
			return nil, false
		}
		if neg {
			n = -n
		}
		return set1(func(s *Style) { set(s, n) }), false
	}
}

func scale(set func(t *Transform, f float32)) func(*tailwindTheme, string, bool) (classDecl, bool) {
	return func(_ *tailwindTheme, v string, neg bool) (classDecl, bool) {
		f, ok := percent(v)
		if !ok {
			return nil, false
		}
		if neg {
			f = -f
		}
		return set1(func(s *Style) { set(&s.Transform, f) }), false
	}
}

func degrees(set func(t *Transform, d float32)) func(*tailwindTheme, string, bool) (classDecl, bool) {
	return func(_ *tailwindTheme, v string, neg bool) (classDecl, bool) {
		d, ok := angle(v)
		if !ok {
			return nil, false
		}
		if neg {
			d = -d
		}
		return set1(func(s *Style) { set(&s.Transform, d) }), false
	}
}

func translate(set func(t *Transform, p Points, pct float32)) func(*tailwindTheme, string, bool) (classDecl, bool) {
	return func(t *tailwindTheme, v string, neg bool) (classDecl, bool) {
		l, ok := t.size(v, neg)
		if !ok || l == (Length{}) && v != "0" {
			return nil, false
		}
		pct, isPct := l.Percent()
		p, isPt := l.Points()
		if !isPct {
			pct = 0
		}
		if !isPt {
			p = 0
		}
		return set1(func(s *Style) { set(&s.Transform, p, pct) }), false
	}
}

func millis(set func(tr *Transition, d time.Duration)) func(*tailwindTheme, string, bool) (classDecl, bool) {
	return func(_ *tailwindTheme, v string, _ bool) (classDecl, bool) {
		if a, ok := arbitrary(v); ok {
			d, err := time.ParseDuration(a)
			if err != nil {
				return nil, false
			}
			return set1(func(s *Style) { set(&s.Transition, d) }), false
		}
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return nil, false
		}
		d := time.Duration(n) * time.Millisecond
		return set1(func(s *Style) { set(&s.Transition, d) }), false
	}
}

var origins = map[string]ObjectPosition{
	"center":       {X: PositionAxis{Offset: Pct(50)}, Y: PositionAxis{Offset: Pct(50)}},
	"top":          {X: PositionAxis{Offset: Pct(50)}, Y: PositionAxis{Offset: Pct(0)}},
	"top-right":    {X: PositionAxis{Offset: Pct(100)}, Y: PositionAxis{Offset: Pct(0)}},
	"right":        {X: PositionAxis{Offset: Pct(100)}, Y: PositionAxis{Offset: Pct(50)}},
	"bottom-right": {X: PositionAxis{Offset: Pct(100)}, Y: PositionAxis{Offset: Pct(100)}},
	"bottom":       {X: PositionAxis{Offset: Pct(50)}, Y: PositionAxis{Offset: Pct(100)}},
	"bottom-left":  {X: PositionAxis{Offset: Pct(0)}, Y: PositionAxis{Offset: Pct(100)}},
	"left":         {X: PositionAxis{Offset: Pct(0)}, Y: PositionAxis{Offset: Pct(50)}},
	"top-left":     {X: PositionAxis{Offset: Pct(0)}, Y: PositionAxis{Offset: Pct(0)}},
}

var blendModes = map[string]BlendMode{"normal": BlendNormal, "multiply": BlendMultiply, "screen": BlendScreen,
	"overlay": BlendOverlay, "darken": BlendDarken, "lighten": BlendLighten, "color-dodge": BlendColorDodge,
	"color-burn": BlendColorBurn, "hard-light": BlendHardLight, "soft-light": BlendSoftLight,
	"difference": BlendDifference, "exclusion": BlendExclusion, "hue": BlendHue, "saturation": BlendSaturation,
	"color": BlendColor, "luminosity": BlendLuminosity, "plus-lighter": BlendPlusLighter}

// font parses font-*: a weight, a family, or an arbitrary one of them.
func (t *tailwindTheme) font(v string, _ bool) (classDecl, bool) {
	if w, ok := t.weights[v]; ok {
		return set1(func(s *Style) { s.FontWeight = w }), false
	}
	if f, ok := t.fonts[v]; ok {
		return set1(func(s *Style) { s.FontFamily = f }), false
	}
	if a, ok := arbitrary(v); ok {
		if n, err := strconv.Atoi(a); err == nil {
			return set1(func(s *Style) { s.FontWeight = FontWeight(n) }), false
		}
		return set1(func(s *Style) { s.FontFamily = strings.Trim(a, `'"`) }), false
	}
	return nil, false
}

// text parses text-*: a size (text-lg, text-lg/7), a color, or an
// arbitrary size or color.
func (t *tailwindTheme) textDecl(v string, _ bool) (classDecl, bool) {
	name, lead, hasLead := cutModifier(v)
	if sz, ok := t.text[name]; ok {
		multiple := sz.leading
		var exact Points
		if hasLead {
			if l, ok := t.leading[lead]; ok {
				multiple = l
			} else if p, ok := t.spacingValue(lead); ok {
				exact, multiple = p, 0
			} else {
				return nil, false
			}
		}
		return set1(func(s *Style) {
			s.FontSize = sz.size
			s.LineHeight, s.LineHeightMultiple, s.LineHeightEm = exact, multiple, 0
		}), false
	}
	if a, ok := arbitrary(v); ok {
		if p, ok := t.length(a); ok {
			return set1(func(s *Style) { s.FontSize = p }), false
		}
	}
	return t.colorDecl(v, func(s *Style, c Color) { s.Color = c })
}

// cutModifier splits v at its last slash outside brackets: text-lg/7,
// bg-red-500/50.
func cutModifier(v string) (string, string, bool) {
	depth := 0
	for i := len(v) - 1; i >= 0; i-- {
		switch v[i] {
		case ']', ')':
			depth++
		case '[', '(':
			depth--
		case '/':
			if depth == 0 {
				return v[:i], v[i+1:], true
			}
		}
	}
	return v, "", false
}

func (t *tailwindTheme) leadingDecl(v string, _ bool) (classDecl, bool) {
	if l, ok := t.leading[v]; ok {
		return set1(func(s *Style) { s.LineHeight, s.LineHeightMultiple, s.LineHeightEm = 0, l, 0 }), false
	}
	if a, ok := arbitrary(v); ok {
		if n, err := strconv.ParseFloat(a, 32); err == nil {
			return set1(func(s *Style) { s.LineHeight, s.LineHeightMultiple, s.LineHeightEm = 0, float32(n), 0 }), false
		}
	}
	p, ok := t.spacingValue(v)
	if !ok {
		return nil, false
	}
	return set1(func(s *Style) { s.LineHeight, s.LineHeightMultiple, s.LineHeightEm = p, 0, 0 }), false
}

func (t *tailwindTheme) trackingDecl(v string, neg bool) (classDecl, bool) {
	em, ok := t.tracking[v]
	if !ok {
		if a, isArb := arbitrary(v); isArb {
			if e, isEm := strings.CutSuffix(a, "em"); isEm {
				f, err := strconv.ParseFloat(e, 32)
				em, ok = float32(f), err == nil
			} else if p, isLen := t.length(a); isLen {
				if neg {
					p = -p
				}
				return set1(func(s *Style) { s.LetterSpacing = p }), false
			}
		}
	}
	if !ok {
		return nil, false
	}
	if neg {
		em = -em
	}
	return func(_ *Style, p *classProps, _ *classEnv) { p.trackingEm, p.hasTracking = em, true }, false
}

// rounded parses rounded, rounded-lg, rounded-t-lg, rounded-tl-[6px].
func (t *tailwindTheme) rounded(v string, _ bool) (classDecl, bool) {
	corners := map[string][]int{"": {0, 1, 2, 3}, "-t": {0, 1}, "-r": {1, 2}, "-b": {2, 3}, "-l": {0, 3},
		"-s": {0, 3}, "-e": {1, 2}, "-tl": {0}, "-tr": {1}, "-br": {2}, "-bl": {3}, "-ss": {0}, "-se": {1},
		"-ee": {2}, "-es": {3}}
	side, size := v, ""
	for s := range corners {
		if s == "" {
			continue
		}
		if v == s || strings.HasPrefix(v, s+"-") {
			if len(s) > len(side) || side == v {
				side, size = s, strings.TrimPrefix(strings.TrimPrefix(v, s), "-")
			}
		}
	}
	if side == v {
		side, size = "", strings.TrimPrefix(v, "-")
		if v != "" && v[0] != '-' {
			return nil, false
		}
	}
	var r Points
	switch size {
	case "":
		r = t.radius["sm"]
	case "none":
		r = 0
	case "full":
		r = 9999
	default:
		var ok bool
		if r, ok = t.radius[size]; !ok {
			a, isArb := arbitrary(size)
			if !isArb {
				return nil, false
			}
			if r, ok = t.length(a); !ok {
				return nil, false
			}
		}
	}
	which := corners[side]
	if len(which) == 4 {
		return set1(func(s *Style) { s.CornerRadius, s.CornerRadii = r, Corners{} }), false
	}
	return set1(func(s *Style) {
		c := &s.CornerRadii
		if *c == (Corners{}) {
			*c = Corners{TopLeft: s.CornerRadius, TopRight: s.CornerRadius, BottomRight: s.CornerRadius,
				BottomLeft: s.CornerRadius}
		}
		for _, i := range which {
			*[]*Points{&c.TopLeft, &c.TopRight, &c.BottomRight, &c.BottomLeft}[i] = r
		}
	}), false
}

func border(setWidth func(s *Style, w Points), setColor func(s *Style, c Color)) func(*tailwindTheme, string, bool) (classDecl, bool) {
	return func(t *tailwindTheme, v string, _ bool) (classDecl, bool) {
		if w, ok := width(v); ok {
			return set1(func(s *Style) { setWidth(s, w) }), false
		}
		return t.colorDecl(v, setColor)
	}
}

// ring parses ring widths: ring (1), ring-2, ring-[3px].
func (t *tailwindTheme) ring(v string) (classDecl, bool) {
	w, ok := width(v)
	if !ok {
		return nil, false
	}
	return func(_ *Style, p *classProps, _ *classEnv) { p.ringWidth, p.hasRing = w, true }, true
}

// colorDecl parses a color value into a decl setting it with style, or
// with props: a palette color, a theme's color (bg-accent), black,
// white, transparent or an arbitrary one, with an opacity modifier.
func (t *tailwindTheme) colorDecl(v string, style func(s *Style, c Color), props ...func(p *classProps, c Color)) (classDecl, bool) {
	name, mod, hasMod := cutModifier(v)
	alpha := float32(1)
	if hasMod {
		a, ok := percent(mod)
		if !ok {
			return nil, false
		}
		alpha = a
	}
	apply := func(s *Style, p *classProps, c Color) {
		c = withAlpha(c, alpha)
		if style != nil {
			style(s, c)
		}
		for _, f := range props {
			f(p, c)
		}
	}
	if c, ok := t.colors[name]; ok {
		return func(s *Style, p *classProps, _ *classEnv) { apply(s, p, c) }, false
	}
	if a, ok := arbitrary(name); ok {
		a = strings.TrimPrefix(a, "color:")
		c, err := ParseColor(a)
		if err != nil {
			return nil, false
		}
		if c == 0 {
			c = clearColor
		}
		return func(s *Style, p *classProps, _ *classEnv) { apply(s, p, c) }, false
	}
	// A color of the theme's palette (Palette's roles), by its kebab-case
	// name: bg-accent, text-text-muted.
	role := camel(name)
	if paletteRoles[role] == nil {
		return nil, false
	}
	return func(s *Style, p *classProps, env *classEnv) {
		if env.palette == nil {
			return
		}
		if c, ok := env.palette.Color(role); ok && c != 0 {
			apply(s, p, c)
		}
	}, true
}

// camel is a kebab-case name in camelCase: text-muted as textMuted.
func camel(s string) string {
	parts := strings.Split(s, "-")
	for i := 1; i < len(parts); i++ {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// withAlpha multiplies c's alpha by a.
func withAlpha(c Color, a float32) Color {
	if a >= 1 {
		return c
	}
	alpha := float32(c&0xff) * a
	out := c&^0xff | Color(alpha+0.5)
	if out == 0 {
		return clearColor
	}
	return out
}

// filterDecl parses a filter: blur-md, brightness-50, drop-shadow-lg,
// on the view or, backdrop, behind it.
func (t *tailwindTheme) filterDecl(v string, backdrop bool) (classDecl, bool) {
	add := func(kind FilterKind, amount float32) (classDecl, bool) {
		return func(_ *Style, p *classProps, _ *classEnv) { p.filter(kind, amount, backdrop) }, false
	}
	if size, ok := strings.CutPrefix(v, "blur"); ok {
		size = strings.TrimPrefix(size, "-")
		r, ok := t.blur[size]
		if !ok {
			if a, isArb := arbitrary(size); isArb {
				r, ok = t.length(a)
			}
		}
		if !ok {
			return nil, false
		}
		return add(FilterBlur, float32(r))
	}
	percents := map[string]FilterKind{"brightness-": FilterBrightness, "contrast-": FilterContrast,
		"saturate-": FilterSaturate, "grayscale-": FilterGrayscale, "invert-": FilterInvert,
		"sepia-": FilterSepia, "opacity-": FilterOpacity}
	for prefix, kind := range percents {
		if n, ok := strings.CutPrefix(v, prefix); ok {
			f, ok := percent(n)
			if !ok {
				return nil, false
			}
			return add(kind, f)
		}
	}
	if backdrop {
		switch v {
		case "grayscale":
			return add(FilterGrayscale, 1)
		case "invert":
			return add(FilterInvert, 1)
		case "sepia":
			return add(FilterSepia, 1)
		}
	}
	if n, ok := strings.CutPrefix(v, "hue-rotate-"); ok {
		d, ok := angle(n)
		if !ok {
			return nil, false
		}
		return add(FilterHueRotate, d)
	}
	if size, ok := strings.CutPrefix(v, "drop-shadow"); ok && !backdrop {
		sh, ok := t.dropShadows[strings.TrimPrefix(size, "-")]
		if !ok || size != "" && size[0] != '-' {
			return nil, false
		}
		return func(_ *Style, p *classProps, _ *classEnv) {
			for _, s := range sh {
				p.filters = append(p.filters, filterPart{kind: FilterDropShadow, shadow: s})
			}
		}, false
	}
	return nil, false
}

func stop(which func(g *gradientParts) *gradientStop) func(*tailwindTheme, string, bool) (classDecl, bool) {
	return func(t *tailwindTheme, v string, _ bool) (classDecl, bool) {
		if p, ok := strings.CutSuffix(v, "%"); ok {
			f, err := strconv.ParseFloat(p, 32)
			if err != nil {
				return nil, false
			}
			return func(_ *Style, c *classProps, _ *classEnv) { which(&c.gradient).at = Pct(float32(f)) }, false
		}
		return t.colorDecl(v, nil, func(c *classProps, col Color) {
			st := which(&c.gradient)
			st.color, st.set = col, true
		})
	}
}

package core

import (
	"cmp"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Tailwind CSS classes (`flex-row gap-4 p-4 bg-white dark:bg-zinc-900`),
// ported from NSH's nsh-core tailwind module: each distinct class string
// is parsed once into rules, and a node resolves them for its
// environment and states. Utilities and variants mean what they mean in
// Tailwind CSS 4; what has no native meaning is an unknown class.
//
// Later utilities win over earlier ones; variant rules apply after plain
// ones in Tailwind's variant order; important ones (`!p-4`, `p-4!`) after
// the inline style. A node's style is its classes', then its Style over
// them.

// Class states a variant can need, as NSH's tailwind::state.
const (
	classPressed uint16 = 1 << iota
	classHovered
	classFocused
	classDisabled
	classSelected
	classChecked
	classIndeterminate
	classFocusWithin
	classAriaBusy
	classAriaChecked
	classExpanded
	classAriaPressed
	classAriaSelected
)

// classStates are the class states in the order of StateStyles.slots.
var classStates = [stateCount]uint16{classChecked, classIndeterminate, classSelected, classFocusWithin, classHovered,
	classFocused, classPressed, classAriaBusy, classAriaChecked, classExpanded, classAriaPressed, classAriaSelected,
	classDisabled}

// classVariants are the variants that read one state, group-* too.
var classVariants = map[string]struct {
	bit   uint16
	order uint16
}{"checked": {classChecked, rankChecked}, "indeterminate": {classIndeterminate, rankIndeterminate},
	"selected": {classSelected, rankSelected}, "focus-within": {classFocusWithin, rankFocusWithin},
	"hover": {classHovered, rankHover}, "focus": {classFocused, rankFocus},
	"focus-visible": {classFocused, rankFocusVisible}, "active": {classPressed, rankActive},
	"disabled": {classDisabled, rankDisabled}, "aria-busy": {classAriaBusy, rankAriaBusy},
	"aria-checked": {classAriaChecked, rankAriaChecked}, "aria-expanded": {classExpanded, rankAriaExpanded},
	"aria-pressed": {classAriaPressed, rankAriaPressed}, "aria-selected": {classAriaSelected, rankAriaSelected}}

// Positions among siblings: first:, last:, odd:, even:, only:.
const (
	positionFirst uint8 = 1 << iota
	positionLast
	positionOdd
	positionEven
	positionOnly
)

// tri is a condition on a boolean: either, true or false.
type tri int8

const (
	either tri = iota
	yes
	no
)

func (t tri) holds(v bool) bool { return t == either || (t == yes) == v }

// classCond is when a rule applies: all of it must hold.
type classCond struct {
	dark, landscape, rtl, reduceMotion tri
	minWidth, maxWidth                 Points // 0: none
	platform                           OS     // "": any
	states, notStates                  uint16
	data                               string // data-*:'s name
	group                              *classGroup
	position                           uint8
	placeholder, selection             bool
}

// classGroup is a group-* variant: the states of the nearest group named
// name ("" any).
type classGroup struct {
	name   string
	states uint16
	data   string
}

// classKey is a variant's place in Tailwind's variant order.
type classKey struct {
	order uint16
	value int64 // a breakpoint's, or a group's inner variant
	name  string
}

func (a classKey) compare(b classKey) int {
	if c := cmp.Compare(a.order, b.order); c != 0 {
		return c
	}
	if c := cmp.Compare(a.value, b.value); c != 0 {
		return c
	}
	return strings.Compare(a.name, b.name)
}

// classRank orders rules as Tailwind sorts them by their variants: the
// highest variant decides, then the next; plain utilities come first.
type classRank []classKey // highest first

func (r *classRank) add(k classKey) {
	i, found := slices.BinarySearchFunc(*r, k, func(a, b classKey) int { return b.compare(a) })
	if !found {
		*r = slices.Insert(*r, i, k)
	}
}

func (a classRank) compare(b classRank) int {
	for i := range min(len(a), len(b)) {
		if c := a[i].compare(b[i]); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(a), len(b))
}

// The places of the variants in Tailwind's order.
const (
	rankGroup         = 1
	rankSelection     = 10
	rankPlaceholder   = 11
	rankFirst         = 20
	rankLast          = 21
	rankOnly          = 22
	rankOdd           = 23
	rankEven          = 24
	rankChecked       = 30
	rankIndeterminate = 31
	rankSelected      = 32
	rankFocusWithin   = 33
	rankHover         = 34
	rankFocus         = 35
	rankFocusVisible  = 36
	rankActive        = 37
	rankEnabled       = 38
	rankDisabled      = 39
	rankAriaBusy      = 40
	rankAriaChecked   = 41
	rankAriaExpanded  = 42
	rankAriaPressed   = 43
	rankAriaSelected  = 44
	rankData          = 45
	rankMotionSafe    = 46
	rankMotionReduce  = 47
	rankMax           = 50
	rankMin           = 51
	rankPortrait      = 60
	rankLandscape     = 61
	rankLTR           = 62
	rankRTL           = 63
	rankDark          = 64
	rankIOS           = 70
	rankAndroid       = 71
	rankWeb           = 72
)

// classDecl is what a utility sets.
type classDecl func(s *Style, p *classProps, env *classEnv)

// classProps are what classes set that is not a field of one style: the
// view's props, which an explicit prop wins over, and the parts several
// utilities build one field from.
type classProps struct {
	// spaceX and spaceY are space-x-* and space-y-*: the margins the view
	// gives its children but the last.
	spaceX, spaceY Points
	// spaceImportant is set for space-*!, which wins over margins.
	spaceImportant       bool
	hasSpaceX, hasSpaceY bool
	reverseX, reverseY   bool

	// The parts of one style's shadows, filters, gradient and tracking.
	ringWidth          Points
	ringColor          Color
	hasRing, ringInset bool
	// insetRingWidth is inset-ring-*'s, inside the view's edge.
	insetRingWidth Points
	insetRingColor Color
	hasInsetRing   bool
	// flexRow is set by flex: a row unless a direction is set.
	flexRow bool
	// shadowAlpha is shadow-*/n's: the alpha every layer takes, of its
	// color; dropAlpha and textShadowAlpha the same for drop and text
	// shadows. 0 keeps the layers' own.
	shadowAlpha, dropAlpha, textShadowAlpha float32
	// dropColor is drop-shadow-<color>'s.
	dropColor Color
	// outlineStyle is outline-solid's and the like, whatever their place.
	outlineStyle BorderStyle
	// leading is set by leading-*, which wins over a text size's own
	// line height, as Tailwind's var(--tw-leading) does.
	leading bool
	// state is set on a state's props: what it leaves unset is the
	// view's.
	state            bool
	shadows          []BoxShadow
	hasShadow        bool
	shadowColor      Color
	insetShadows     []BoxShadow
	insetShadowColor Color
	// ringOffset is ring-offset-*'s width, a solid band of
	// ringOffsetColor (white without one) between the view and its ring.
	ringOffset      Points
	ringOffsetColor Color
	// textShadow is text-shadow-*'s, textShadowColor its color.
	textShadow      *BoxShadow
	textShadowColor Color
	filters         []filterPart
	backdrop        []filterPart
	gradient        gradientParts
	trackingEm      float32
	hasTracking     bool
}

// currentMark is the color a -current utility sets, its alpha the
// modifier's: finish replaces it with the view's text color, as CSS's
// currentColor is whatever color the view ends up with. A field that is
// currentColor when unset takes 0 when the text color is inherited.
const currentMark Color = 0x01020300

func isCurrent(c Color) bool { return c&^0xff == currentMark }

// currentIn returns c with CurrentColor, or a -current utility's mark,
// replaced by color, the mark's alpha applied.
func currentIn(c, color Color) Color {
	if isCurrent(c) {
		return withAlpha(color, float32(c&0xff)/255)
	}
	return driver.ResolveCurrent(c, color)
}

// resolveCurrent replaces the -current colors of s and p.
func (p *classProps) resolveCurrent(s *Style) {
	text := s.Color
	cur := func(c *Color, zeroIsCurrent bool) {
		if !isCurrent(*c) {
			return
		}
		alpha := float32(*c&0xff) / 255
		switch {
		case text != 0:
			*c = withAlpha(text, alpha)
		case zeroIsCurrent && alpha == 1:
			*c = 0
		case zeroIsCurrent:
			// Faded, it stays a mark: the view's style resolves it.
		default:
			*c = withAlpha(RGB(0), alpha)
		}
	}
	for _, c := range []*Color{&s.BorderColor, &s.BorderColors.Top, &s.BorderColors.Right, &s.BorderColors.Bottom,
		&s.BorderColors.Left, &s.OutlineColor, &p.ringColor, &p.insetRingColor} {
		cur(c, true)
	}
	if isCurrent(s.TextDecorationColor) && text == 0 {
		// A text's decoration has its text's color, drawn with no mark.
		s.TextDecorationColor = 0
	}
	cur(&s.TextDecorationColor, true)
	// A background or a tint keeps its mark without a text color here:
	// the view's style resolves it in the color the view inherits
	// (currentIn).
	if text != 0 {
		cur(&s.Background, false)
		cur(&s.TintColor, false)
	}
	for _, c := range []*Color{&s.SelectionColor, &s.CaretColor, &s.PlaceholderColor, &p.shadowColor,
		&p.insetShadowColor, &p.ringOffsetColor, &p.textShadowColor, &p.dropColor, &p.gradient.from.color,
		&p.gradient.via.color, &p.gradient.to.color} {
		cur(c, false)
	}
}

// filterPart is one filter function a utility sets.
type filterPart struct {
	kind   FilterKind
	amount float32
	shadow BoxShadow
}

// filter sets a filter function, replacing one of its kind.
func (p *classProps) filter(kind FilterKind, amount float32, backdrop bool) {
	list := &p.filters
	if backdrop {
		list = &p.backdrop
	}
	for i := range *list {
		if (*list)[i].kind == kind {
			(*list)[i].amount = amount
			return
		}
	}
	*list = append(*list, filterPart{kind: kind, amount: amount})
}

// gradientParts are bg-linear-to-*, from-*, via-* and to-*.
type gradientParts struct {
	kind          GradientKind
	angle         float32
	corner        GradientCorner
	set, none     bool
	from, via, to gradientStop
	// arbitrary is set by an arbitrary angle: bg-linear-[25deg].
	arbitrary bool
}

type gradientStop struct {
	color Color
	at    Length
	set   bool
	// legacy is set by an arbitrary color CSS mixes in sRGB: #f00, rgb().
	legacy bool
}

// inherit fills what a state's gradient leaves unset with the view's: a
// state that changes one stop keeps the rest.
func (g *gradientParts) inherit(base gradientParts) {
	if !g.from.set && !g.via.set && !g.to.set || g.set || g.none {
		return
	}
	kind, from, via, to := base, g.from, g.via, g.to
	*g = kind
	for _, st := range []struct{ dst, src *gradientStop }{{&g.from, &from}, {&g.via, &via}, {&g.to, &to}} {
		if st.src.set {
			*st.dst = *st.src
		} else if st.src.at != (Length{}) {
			st.dst.at = st.src.at
		}
	}
}

// filterOrder is the order Tailwind applies filter functions in.
var filterOrder = []FilterKind{FilterBlur, FilterBrightness, FilterContrast, FilterDropShadow, FilterGrayscale,
	FilterHueRotate, FilterInvert, FilterSaturate, FilterSepia, FilterOpacity}

// finish builds the fields of s that several utilities make: its
// shadows (rings under shadows), filters, gradient and letter spacing.
func (p *classProps) finish(s *Style) {
	// tint gives a shadow layer color c, alpha too, as Tailwind's
	// --tw-shadow-color does, at alpha when a size's modifier sets one.
	tint := func(sh BoxShadow, c Color, alpha float32) BoxShadow {
		switch {
		case c != 0 && alpha > 0:
			sh.Color = withAlpha(c, alpha)
		case c != 0:
			sh.Color = c
		case alpha > 0:
			sh.Color = withAlpha(sh.Color|0xff, alpha)
		}
		return sh
	}
	if p.flexRow && s.Direction == 0 {
		s.Direction = Row
	}
	switch {
	case p.outlineStyle == BorderNone:
		s.OutlineWidth, s.OutlineStyle = 0, BorderNone
	case p.outlineStyle != 0:
		s.OutlineStyle = p.outlineStyle
		if s.OutlineWidth == 0 && !p.state {
			// A style alone is CSS's initial medium width.
			s.OutlineWidth = 3
		}
	case s.OutlineWidth != 0 && !p.state:
		s.OutlineStyle = BorderSolid
	}
	p.resolveCurrent(s)
	if p.hasRing || p.hasShadow || p.insetShadows != nil || p.hasInsetRing {
		var out []BoxShadow
		for _, sh := range p.insetShadows {
			out = append(out, tint(sh, p.insetShadowColor, 0))
		}
		if p.hasInsetRing {
			out = append(out, BoxShadow{Spread: p.insetRingWidth, Color: cmpOr(p.insetRingColor, cmpOr(s.Color, RGB(0))),
				Inset: true})
		}
		if p.hasRing {
			c := p.ringColor
			if c == 0 {
				c = cmpOr(s.Color, RGB(0))
			}
			if p.ringOffset > 0 && !p.ringInset {
				oc := p.ringOffsetColor
				if oc == 0 {
					oc = RGB(0xffffff)
				}
				out = append(out, BoxShadow{Spread: p.ringOffset, Color: oc})
			}
			spread := p.ringWidth
			if !p.ringInset {
				spread += p.ringOffset
			}
			out = append(out, BoxShadow{Spread: spread, Color: c, Inset: p.ringInset})
		}
		for _, sh := range p.shadows {
			out = append(out, tint(sh, p.shadowColor, p.shadowAlpha))
		}
		if out == nil {
			out = []BoxShadow{}
		}
		s.Shadows = out
	}
	if sh := p.textShadow; sh != nil {
		t := tint(*sh, p.textShadowColor, p.textShadowAlpha)
		s.TextShadowColor, s.TextShadowOffset, s.TextShadowRadius = t.Color, t.Offset, t.Blur
	}
	build := func(parts []filterPart) []Filter {
		var out []Filter
		for _, k := range filterOrder {
			for _, f := range parts {
				if f.kind == k {
					sh := f.shadow
					if k == FilterDropShadow {
						sh = tint(sh, p.dropColor, p.dropAlpha)
					}
					out = append(out, Filter{Kind: f.kind, Amount: f.amount, Shadow: sh})
				}
			}
		}
		return out
	}
	if p.filters != nil {
		s.Filters = build(p.filters)
	}
	if p.backdrop != nil {
		s.BackdropFilters = build(p.backdrop)
	}
	if g := p.gradient; g.none {
		s.Gradients = []Gradient{}
	} else if g.set && (g.from.set || g.via.set || g.to.set) {
		// Tailwind's stops: from, via, to; a missing end is transparent
		// of the other's color.
		from, to := g.from, g.to
		if !from.set {
			from.color = to.color &^ 0xff
		}
		if !to.set {
			to.color = from.color &^ 0xff
			if g.via.set {
				to.color = g.via.color &^ 0xff
			}
		}
		stops := []ColorStop{{Color: from.color, At: cmpOr(from.at, Pct(0))}}
		if g.via.set {
			stops = append(stops, ColorStop{Color: g.via.color, At: cmpOr(g.via.at, Pct(50))})
		}
		stops = append(stops, ColorStop{Color: to.color, At: cmpOr(to.at, Pct(100))})
		gr := Gradient{Kind: g.kind, Angle: g.angle, ToCorner: g.corner, Stops: stops}
		// Tailwind writes in oklab but at an arbitrary angle, where CSS
		// mixes legacy colors in sRGB.
		if g.arbitrary && (!g.from.set || g.from.legacy) && (!g.via.set || g.via.legacy) && (!g.to.set || g.to.legacy) {
			gr.Space = ColorSpaceSRGB
		}
		s.Gradients = []Gradient{gr}
	}
	if p.hasTracking {
		s.LetterSpacing = Points(p.trackingEm) * cmpOr(s.FontSize, DefaultFontSize)
	}
}

type classRule struct {
	cond      classCond
	decls     []classDecl
	rank      classRank
	important bool
}

// classList is a parsed class string.
type classList struct {
	tw    *tailwindTheme
	class string
	rules []classRule
	// env is set when the rules depend on the environment: the
	// appearance, the window, the direction, the platform, the motion
	// setting or the theme's colors.
	env bool
	// positional is set when they depend on the node's place among its
	// siblings.
	positional bool
	// groups are the groups the view is: "" for `group`, item for
	// `group/item`.
	groups  []string
	unknown []string
	// spacing is set when the view gives its children margins.
	spacing bool
}

// classEnv is what a node's classes are resolved for.
type classEnv struct {
	dark     bool
	window   Size
	rtl      bool
	reduce   bool
	os       OS
	position uint8
	palette  *Palette
	tw       *tailwindTheme
	// theme gives the tokens classes name as variables: rounded-(--radius).
	theme *Theme
	// fontSize is what em-based utilities (tracking-*) measure against.
	fontSize Points
}

var classCache sync.Map // classSource → *classList

// classSource is a class string and the Tailwind theme it is read with.
type classSource struct {
	tw    *tailwindTheme
	class string
}

// parseClasses parses a class string for Tailwind's default theme.
func parseClasses(s string) *classList { return defaultTailwind.classes(s) }

// classes parses a class string, once per distinct string.
func (t *tailwindTheme) classes(s string) *classList {
	key := classSource{t, s}
	if v, ok := classCache.Load(key); ok {
		return v.(*classList)
	}
	l := t.parse(s)
	classCache.Store(key, l)
	return l
}

func (t *tailwindTheme) parse(s string) *classList {
	l := &classList{tw: t, class: s}
	for _, token := range strings.Fields(s) {
		if !t.parseToken(l, token) {
			l.unknown = append(l.unknown, token)
		}
	}
	// Stable: equal ranks keep the order of the string.
	slices.SortStableFunc(l.rules, func(a, b classRule) int {
		if a.important != b.important {
			if a.important {
				return 1
			}
			return -1
		}
		return a.rank.compare(b.rank)
	})
	return l
}

// splitVariants splits a token at the colons outside brackets: the
// variants, then the utility.
func splitVariants(token string) []string {
	var parts []string
	depth, start := 0, 0
	for i, c := range token {
		switch c {
		case '[', '(':
			depth++
		case ']', ')':
			depth--
		case ':':
			if depth == 0 {
				parts = append(parts, token[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, token[start:])
}

func (t *tailwindTheme) parseToken(l *classList, token string) bool {
	parts := splitVariants(token)
	utility, variants := parts[len(parts)-1], parts[:len(parts)-1]
	important := false
	if u, ok := strings.CutPrefix(utility, "!"); ok {
		utility, important = u, true
	} else if u, ok := strings.CutSuffix(utility, "!"); ok {
		utility, important = u, true
	}
	if len(variants) == 0 {
		if utility == "group" {
			l.groups = append(l.groups, "")
			return true
		}
		if name, ok := strings.CutPrefix(utility, "group/"); ok {
			l.groups = append(l.groups, name)
			return true
		}
	}
	var cond classCond
	var rank classRank
	for _, v := range variants {
		k, ok := t.variant(v, &cond)
		if !ok {
			return false
		}
		rank.add(k)
	}
	if ignoredUtility(utility) {
		return true
	}
	decls, env := t.utility(utility)
	if decls == nil {
		return false
	}
	if (cond.placeholder || cond.selection) && !colorOnly(utility, cond.placeholder) {
		return false
	}
	l.rules = append(l.rules, classRule{cond: cond, decls: decls, rank: rank, important: important})
	l.env = l.env || env || cond.dark != either || cond.landscape != either || cond.rtl != either ||
		cond.reduceMotion != either || cond.minWidth != 0 || cond.maxWidth != 0 || cond.platform != ""
	l.positional = l.positional || cond.position != 0
	l.spacing = l.spacing || strings.HasPrefix(strings.TrimPrefix(utility, "-"), "space-")
	return true
}

// colorOnly reports whether a utility under placeholder: or selection:
// sets the one color the pseudo-element has: a placeholder's text color,
// a selection's background.
func colorOnly(u string, placeholder bool) bool {
	if placeholder {
		return strings.HasPrefix(u, "text-")
	}
	return strings.HasPrefix(u, "bg-")
}

// ignoredUtility reports utilities whose CSS is a rendering hint or what
// every native view already is: accepted, doing nothing.
func ignoredUtility(u string) bool {
	switch u {
	case "antialiased", "subpixel-antialiased", "isolation-auto", "appearance-none", "appearance-auto",
		"will-change-auto", "will-change-scroll", "will-change-contents", "will-change-transform", "box-border":
		return true
	}
	_, ok := arbitrary(strings.TrimPrefix(u, "will-change-"))
	return ok && strings.HasPrefix(u, "will-change-")
}

// variant applies variant v to cond and returns its place.
func (t *tailwindTheme) variant(v string, c *classCond) (classKey, bool) {
	key := func(order uint16) (classKey, bool) { return classKey{order: order}, true }
	switch v {
	case "selection":
		c.selection = true
		return key(rankSelection)
	case "placeholder":
		c.placeholder = true
		return key(rankPlaceholder)
	case "first":
		c.position |= positionFirst
		return key(rankFirst)
	case "last":
		c.position |= positionLast
		return key(rankLast)
	case "only":
		c.position |= positionOnly
		return key(rankOnly)
	case "odd":
		c.position |= positionOdd
		return key(rankOdd)
	case "even":
		c.position |= positionEven
		return key(rankEven)
	case "enabled":
		c.notStates |= classDisabled
		return key(rankEnabled)
	case "motion-safe":
		c.reduceMotion = no
		return key(rankMotionSafe)
	case "motion-reduce":
		c.reduceMotion = yes
		return key(rankMotionReduce)
	case "portrait":
		c.landscape = no
		return key(rankPortrait)
	case "landscape":
		c.landscape = yes
		return key(rankLandscape)
	case "ltr":
		c.rtl = no
		return key(rankLTR)
	case "rtl":
		c.rtl = yes
		return key(rankRTL)
	case "dark":
		c.dark = yes
		return key(rankDark)
	case "ios":
		c.platform = OSIOS
		return key(rankIOS)
	case "android":
		c.platform = OSAndroid
		return key(rankAndroid)
	case "web":
		c.platform = OSWeb
		return key(rankWeb)
	}
	if b, ok := classVariants[v]; ok {
		c.states |= b.bit
		return key(b.order)
	}
	if name, ok := dataVariant(v); ok {
		c.data = name
		return classKey{order: rankData, name: name}, true
	}
	if rest, ok := strings.CutPrefix(v, "group-"); ok {
		state, name, _ := strings.Cut(rest, "/")
		b, known := classVariants[state]
		data, isData := dataVariant(state)
		if !known && !isData {
			return classKey{}, false
		}
		if c.group == nil {
			c.group = &classGroup{name: name}
		} else if c.group.name != name {
			return classKey{}, false
		}
		if isData {
			c.group.data = data
			return classKey{order: rankGroup, value: rankData, name: name + "/" + data}, true
		}
		c.group.states |= b.bit
		return classKey{order: rankGroup, value: int64(b.order), name: name}, true
	}
	// Breakpoints: md:, max-md:, min-[600px]:, max-[600px]:.
	name, max := strings.CutPrefix(v, "max-")
	if !max {
		name = strings.TrimPrefix(v, "min-")
	}
	var width Points
	named := int64(0)
	if a, ok := arbitrary(name); ok {
		w, ok := t.length(a)
		if !ok {
			return classKey{}, false
		}
		width = w
	} else {
		// Tailwind sorts a theme's breakpoints, in rem, after arbitrary
		// ones, in px.
		named = 1 << 40
		i := slices.IndexFunc(t.breakpoints, func(b breakpoint) bool { return b.name == name })
		if i < 0 {
			return classKey{}, false
		}
		width = t.breakpoints[i].width
	}
	thousandths := int64(width * 1000)
	if max {
		if c.maxWidth == 0 || width < c.maxWidth {
			c.maxWidth = width
		}
		return classKey{order: rankMax, value: named - thousandths}, true
	}
	c.minWidth = maxPoints(c.minWidth, width)
	return classKey{order: rankMin, value: thousandths}, true
}

// dataVariant reads data-<name>: Tailwind's boolean form, of lower case
// letters, digits and dashes.
func dataVariant(v string) (string, bool) {
	name, ok := strings.CutPrefix(v, "data-")
	if !ok || name == "" {
		return "", false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return "", false
		}
	}
	return name, true
}

func maxPoints(a, b Points) Points {
	if a > b {
		return a
	}
	return b
}

// arbitrary returns the value of `[value]`, underscores as spaces.
func arbitrary(s string) (string, bool) {
	if v, ok := strings.CutPrefix(s, "["); ok {
		if v, ok := strings.CutSuffix(v, "]"); ok && v != "" {
			return strings.ReplaceAll(v, "_", " "), true
		}
	}
	return "", false
}

// length reads a CSS length as points: px, rem and em (of the theme's
// rem), pt, or 0.
func (t *tailwindTheme) length(s string) (Points, bool) {
	s = strings.TrimSpace(s)
	scale := float64(1)
	switch {
	case strings.HasSuffix(s, "px"):
		s = s[:len(s)-2]
	case strings.HasSuffix(s, "rem"):
		s, scale = s[:len(s)-3], float64(t.rem)
	case strings.HasSuffix(s, "em"):
		s, scale = s[:len(s)-2], float64(t.rem)
	case strings.HasSuffix(s, "pt"):
		s, scale = s[:len(s)-2], 4.0/3
	default:
		// A length needs a unit, as in CSS, but for 0.
		if s != "0" {
			return 0, false
		}
	}
	f, err := strconv.ParseFloat(s, 32)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return 0, false
	}
	return Points(f * scale), true
}

// matches reports whether cond's environment holds in env.
func (c *classCond) matches(env *classEnv) bool {
	w := env.window
	return c.dark.holds(env.dark) && c.rtl.holds(env.rtl) && c.reduceMotion.holds(env.reduce) &&
		c.landscape.holds(w.Width > w.Height) && (c.minWidth == 0 || w.Width >= c.minWidth) &&
		(c.maxWidth == 0 || w.Width < c.maxWidth) && (c.platform == "" || c.platform == env.os) &&
		(c.position == 0 || env.position&c.position != 0)
}

// resolve returns the style the classes give a node in env, the styles
// of states in its state fields, then what important classes give, and
// the props they set.
func (l *classList) resolve(env *classEnv) (normal, important Style, p classProps) {
	parts := map[*Style]*classProps{}
	var order []*Style
	propsOf := func(s *Style) *classProps {
		if c := parts[s]; c != nil {
			return c
		}
		c := &classProps{state: s != &normal && s != &important}
		parts[s] = c
		order = append(order, s)
		return c
	}
	propsOf(&normal)
	propsOf(&important)
	for i := range l.rules {
		r := &l.rules[i]
		if !r.cond.matches(env) {
			continue
		}
		base := &normal
		if r.important {
			base = &important
		}
		target := base
		switch c := &r.cond; {
		case c.placeholder || c.selection:
			var s Style
			var sp classProps
			for _, d := range r.decls {
				d(&s, &sp, env)
			}
			if c.placeholder {
				sp.resolveCurrent(&s)
				set(&base.PlaceholderColor, s.Color)
			} else {
				sp.resolveCurrent(&s)
				set(&base.SelectionColor, s.Background)
			}
			continue
		case c.group != nil:
			if base.GroupStyle == nil || base.GroupStyle.Name != c.group.name {
				gs := &GroupStyle{Name: c.group.name}
				if base.GroupStyle != nil {
					*gs = *base.GroupStyle
					gs.Name = c.group.name
				}
				base.GroupStyle = gs
			}
			target = stateStyle(c.group.states, c.group.data, base.GroupStyle.states())
		case c.states != 0 || c.data != "":
			target = stateStyle(c.states, c.data, base.states())
		}
		tp := propsOf(target)
		for _, d := range r.decls {
			d(target, tp, env)
		}
	}
	for _, s := range order {
		if s != &normal && s != &important {
			parts[s].gradient.inherit(parts[&normal].gradient)
		}
		parts[s].finish(s)
	}
	// A state's background shows over the view's gradient, as feedback:
	// the state hides it unless it has one of its own.
	if len(normal.Gradients) > 0 {
		for _, slot := range normal.states().slots {
			if st := *slot; st != nil && st.Background != 0 && st.Gradients == nil {
				st.Gradients = []Gradient{}
			}
		}
	}
	// The view's props: the important classes' over the others'.
	p = *parts[&normal]
	// Important spacing wins over the children's own margins.
	if ip := parts[&important]; ip.hasSpaceX || ip.hasSpaceY {
		if ip.hasSpaceX {
			p.spaceX, p.hasSpaceX, p.reverseX = ip.spaceX, true, ip.reverseX || p.reverseX
		}
		if ip.hasSpaceY {
			p.spaceY, p.hasSpaceY, p.reverseY = ip.spaceY, true, ip.reverseY || p.reverseY
		}
		p.spaceImportant = true
	}
	return normal, important, p
}

// stateStyle returns the style of the state core merges last among
// states, made when missing: a rule needing several states applies in
// the one that decides. Disabled is merged after data, the others before.
func stateStyle(states uint16, data string, styles stateSet) *Style {
	if states&classDisabled == 0 && data != "" {
		return styles.dataStyle(data)
	}
	for i := stateCount - 1; i >= 0; i-- {
		if states&classStates[i] != 0 {
			if *styles.slots[i] == nil {
				*styles.slots[i] = &Style{}
			}
			return *styles.slots[i]
		}
	}
	return styles.dataStyle(data)
}

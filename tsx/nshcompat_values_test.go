package tsx_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/fs"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/nsgotest"
)

// The text forms of NSH's style keys and props (nsh-compat's values.rs),
// written from what NSGo hands its platforms: a script's `style id:card
// opacity 0.5` reads the view's ViewStyle. Where NSGo's platforms receive
// a value in another shape than NSH's (a gradient resolved against the
// box, a transform as one matrix, a shadow's opacity in its color), the
// script's text is read into that shape and compared there instead.

// compatNum writes a number as NSH does: Rust's shortest f32 form.
func compatNum[T ~float32 | ~float64](f T) string {
	return strconv.FormatFloat(float64(float32(f)), 'f', -1, 32)
}

// compatColor writes a color as #rrggbbaa.
func compatColor(c driver.Color) string {
	if c&0xff == 0 {
		c = 0 // core.Transparent
	}
	return fmt.Sprintf("#%08x", uint32(c))
}

func compatMillis(d time.Duration) string { return compatNum(float64(d) / float64(time.Millisecond)) }

func compatCurve(c driver.Curve) string {
	if c == (driver.Curve{}) {
		c = driver.Curve{X1: 0, Y1: 0, X2: 1, Y2: 1}
	}
	return fmt.Sprintf("cubic-bezier(%s, %s, %s, %s)", compatNum(c.X1), compatNum(c.Y1), compatNum(c.X2), compatNum(c.Y2))
}

func compatLength(l driver.Length) string {
	if pct, ok := l.Percent(); ok {
		return compatNum(pct) + "%"
	}
	if p, ok := l.Points(); ok {
		return compatNum(p)
	}
	return "auto"
}

// compatTarget is what a style or prop step reads: the view its selector
// names and, for span:, the characters it starts at.
type compatTarget struct {
	app  *nsgotest.App
	view *nsgotest.View
	// span is set for span:, with chars the span's characters.
	span  *driver.TextSpan
	chars string
	// shadowColor is the shadow color the case last checked the view
	// has, which its shadowOpacity scales.
	shadowColor string
}

func compatTargetOf(app *nsgotest.App, selector string) compatTarget {
	if text, ok := strings.CutPrefix(selector, "span:"); ok {
		text = strings.Trim(text, `"`)
		views := app.QueryMounted("text:" + text)
		if len(views) == 0 {
			panic(compatFailure{"no text shows " + strconv.Quote(text)})
		}
		v := views[len(views)-1]
		at := strings.Index(v.Text, text)
		sp := driver.TextSpan{Start: 0, End: len(v.Text), Style: v.TextStyle}
		for _, s := range v.Spans {
			if at >= s.Start && at < s.End {
				sp = s
			}
		}
		return compatTarget{app: app, view: v, span: &sp, chars: v.Text[sp.Start:sp.End]}
	}
	return compatTarget{app: app, view: compatView(app, selector)}
}

// textStyle is the text style a key reads: a span's, a text's or an
// input's.
func (t compatTarget) textStyle() driver.TextStyle {
	switch {
	case t.span != nil:
		return t.span.Style
	case t.view.Kind == driver.KindInput:
		return t.view.Input.Style
	}
	return t.view.TextStyle
}

func (t compatTarget) text() string {
	if t.span != nil {
		return t.chars
	}
	return t.view.Text
}

// compatStyle returns the text form of key on t, and whether the view has
// it set: a value other than the platform's default.
func compatStyle(t compatTarget, key string) (string, bool) {
	vs := &t.view.Style
	ts := t.textStyle()
	effects := vs.Effects
	if effects == nil {
		effects = &driver.Effects{}
	}
	widths := vs.Borders
	if widths == (driver.Edges{}) {
		widths = driver.All(vs.BorderWidth)
	}
	colors := vs.BorderColors
	if colors == (driver.EdgeColors{}) {
		colors = driver.AllColors(vs.BorderColor)
	}
	corners := vs.Corners
	if corners == (driver.Corners{}) {
		corners = driver.AllCorners(vs.CornerRadius)
	}
	num := func(f float32) (string, bool) { return compatNum(f), f != 0 }
	color := func(c driver.Color) (string, bool) { return compatColor(c), c != 0 }
	switch key {
	case "backgroundColor":
		if t.span != nil {
			return color(t.span.Background.Color)
		}
		return color(vs.Background)
	case "opacity":
		return compatNum(vs.Opacity), vs.Opacity != 1
	case "borderWidth":
		if widths != driver.All(widths.Top) {
			return "", false
		}
		return num(float32(widths.Top))
	case "borderTopWidth":
		return num(float32(widths.Top))
	case "borderRightWidth":
		return num(float32(widths.Right))
	case "borderBottomWidth":
		return num(float32(widths.Bottom))
	case "borderLeftWidth":
		return num(float32(widths.Left))
	case "borderColor":
		if colors != driver.AllColors(colors.Top) {
			return "", false
		}
		return color(colors.Top)
	case "borderTopColor":
		return color(colors.Top)
	case "borderRightColor":
		return color(colors.Right)
	case "borderBottomColor":
		return color(colors.Bottom)
	case "borderLeftColor":
		return color(colors.Left)
	case "borderRadius":
		if !corners.Uniform() {
			return "", false
		}
		return num(float32(corners.TopLeft))
	case "borderTopLeftRadius":
		return num(float32(corners.TopLeft))
	case "borderTopRightRadius":
		return num(float32(corners.TopRight))
	case "borderBottomRightRadius":
		return num(float32(corners.BottomRight))
	case "borderBottomLeftRadius":
		return num(float32(corners.BottomLeft))
	case "borderStyle":
		return map[driver.BorderStyle]string{0: "solid", driver.BorderSolid: "solid", driver.BorderDashed: "dashed",
			driver.BorderDotted: "dotted"}[vs.BorderStyle], vs.BorderStyle > driver.BorderSolid
	case "borderCurve":
		if vs.ContinuousCorners {
			return "continuous", true
		}
		return "circular", false
	case "outlineWidth":
		return num(float32(vs.Outline.Width))
	case "outlineOffset":
		return num(float32(vs.Outline.Offset))
	case "outlineColor":
		return color(vs.Outline.Color)
	case "outlineStyle":
		return map[driver.BorderStyle]string{0: "solid", driver.BorderSolid: "solid", driver.BorderDashed: "dashed",
			driver.BorderDotted: "dotted"}[vs.Outline.Style], vs.Outline.Width > 0
	case "overflow":
		if vs.Clip {
			return "hidden", true
		}
		return "visible", false
	case "visibility":
		if vs.Hidden {
			return "hidden", true
		}
		return "visible", false
	case "backfaceVisibility":
		if vs.BackfaceHidden {
			return "hidden", true
		}
		return "visible", false
	case "cursor":
		return vs.Cursor.Name(), vs.Cursor != driver.CursorAuto
	case "mixBlendMode":
		return vs.BlendMode.Name(), vs.BlendMode != driver.BlendNormal
	case "glassEffect":
		return map[driver.GlassEffect]string{driver.GlassNone: "none", driver.GlassRegular: "regular",
			driver.GlassClear: "clear"}[vs.Glass], vs.Glass != driver.GlassNone
	case "tintColor":
		return color(vs.TintColor)
	case "shadowColor":
		return color(vs.ShadowColor)
	case "shadowOpacity":
		return compatNum(float32(vs.ShadowColor&0xff) / 255), vs.ShadowColor&0xff != 0
	case "shadowRadius":
		return num(float32(vs.ShadowRadius))
	case "shadowOffset":
		return fmt.Sprintf("width=%s height=%s", compatNum(vs.ShadowOffset.X), compatNum(vs.ShadowOffset.Y)),
			vs.ShadowOffset != driver.Point{}
	case "boxShadow":
		var parts []string
		for _, sh := range effects.Shadows {
			s := fmt.Sprintf("%s %s %s %s %s", compatNum(sh.Offset.X), compatNum(sh.Offset.Y), compatNum(sh.Blur),
				compatNum(sh.Spread), compatColor(sh.Color))
			if sh.Inset {
				s = "inset " + s
			}
			parts = append(parts, s)
		}
		return strings.Join(parts, ", "), len(parts) > 0
	case "filter":
		return compatFilters(effects.Filters), len(effects.Filters) > 0
	case "backdropFilter":
		return compatFilters(effects.BackdropFilters), len(effects.BackdropFilters) > 0
	case "backgroundImage":
		return "", len(effects.Gradients) > 0
	case "maskImage":
		return "", effects.Mask != nil
	case "transform":
		return "", vs.Transform != (driver.Transform{})
	case "transformOrigin":
		return "", vs.Origin != (driver.Point{}) || vs.OriginZ != 0
	case "transitionDuration":
		return compatMillis(vs.Transition.Duration), vs.Transition.Duration != 0
	case "transitionDelay":
		return compatMillis(vs.Transition.Delay), vs.Transition.Delay != 0
	case "transitionTimingFunction":
		return compatCurve(vs.Transition.Easing), vs.Transition.Duration != 0
	case "transitionProperty":
		return compatTransitionProperty(vs.Transition.Properties), vs.Transition.Properties != 0
	case "animationName":
		names := map[driver.Keyframes]string{0: "none", driver.KeyframesSpin: "spin", driver.KeyframesPing: "ping",
			driver.KeyframesPulse: "pulse", driver.KeyframesBounce: "bounce"}
		return names[vs.Animation.Keyframes], vs.Animation.Keyframes != 0
	case "animationDuration", "animationTimingFunction":
		d, frames := vs.Animation.Cycle()
		if key == "animationDuration" {
			return compatMillis(d), vs.Animation.Keyframes != 0
		}
		ease := vs.Animation.Easing
		if len(frames) > 0 && ease == (driver.Curve{}) {
			ease = frames[0].Easing
		}
		return compatCurve(ease), vs.Animation.Keyframes != 0
	case "animationDelay":
		return compatMillis(vs.Animation.Delay), vs.Animation.Keyframes != 0
	case "animationIterationCount":
		if vs.Animation.Iterations == 0 {
			return "infinite", vs.Animation.Keyframes != 0
		}
		return strconv.Itoa(vs.Animation.Iterations), true
	case "objectPosition":
		return compatPosition(t.view.Image.Position), true
	case "numberOfLines":
		return strconv.Itoa(t.view.MaxLines), t.view.MaxLines != 0
	case "textOverflow":
		return [...]string{"ellipsis", "ellipsis", "ellipsis-start", "ellipsis-middle", "clip"}[t.view.Ellipsize],
			t.view.Ellipsize > driver.EllipsizeTail
	case "userSelect":
		return map[driver.Selectable]string{driver.SelectableAuto: "auto", driver.SelectableOn: "text",
			driver.SelectableOff: "none"}[t.view.Props.Selectable], t.view.Props.Selectable != driver.SelectableAuto
	case "pointerEvents":
		return [...]string{"auto", "none", "box-none", "box-only"}[t.view.Props.PointerEvents],
			t.view.Props.PointerEvents != driver.PointerEventsAuto

	case "color":
		return color(ts.Color)
	case "fontSize":
		return num(float32(ts.Font.Size))
	case "fontWeight":
		return strconv.Itoa(int(ts.Font.Weight)), ts.Font.Weight != driver.FontWeightNormal
	case "fontFamily":
		return ts.Font.Family, ts.Font.Family != ""
	case "fontStyle":
		if ts.Font.Italic {
			return "italic", true
		}
		return "normal", false
	case "fontVariant":
		var names []string
		for _, f := range []struct {
			bit  driver.FontVariant
			name string
		}{{driver.FontSmallCaps, "small-caps"}, {driver.FontOldstyleNums, "oldstyle-nums"},
			{driver.FontLiningNums, "lining-nums"}, {driver.FontTabularNums, "tabular-nums"},
			{driver.FontProportionalNums, "proportional-nums"}} {
			if ts.Font.Variant&f.bit != 0 {
				names = append(names, f.name)
			}
		}
		return strings.Join(names, ", "), len(names) > 0
	case "lineHeight":
		return num(float32(ts.LineHeight))
	case "letterSpacing":
		return num(float32(ts.LetterSpacing))
	case "textAlign":
		return map[driver.TextAlign]string{0: "auto", driver.TextAlignStart: "auto", driver.TextAlignCenter: "center",
			driver.TextAlignEnd: "right", driver.TextAlignJustify: "justify"}[ts.Align], ts.Align > driver.TextAlignStart
	case "writingDirection":
		if ts.RTL {
			return "rtl", true
		}
		return "ltr", false
	case "textDecorationLine":
		var lines []string
		if ts.Decoration&driver.TextDecorationUnderline != 0 {
			lines = append(lines, "underline")
		}
		if ts.Decoration&driver.TextDecorationLineThrough != 0 {
			lines = append(lines, "line-through")
		}
		if len(lines) == 0 {
			return "none", ts.Decoration != 0
		}
		return strings.Join(lines, " "), true
	case "textDecorationStyle":
		return map[driver.TextDecorationStyle]string{0: "solid", driver.DecorationSolid: "solid",
			driver.DecorationDouble: "double", driver.DecorationDotted: "dotted", driver.DecorationDashed: "dashed",
			driver.DecorationWavy: "wavy"}[ts.DecorationStyle], ts.DecorationStyle != 0
	case "textDecorationColor":
		return color(ts.DecorationColor)
	case "textDecorationThickness":
		return num(float32(ts.DecorationThickness))
	case "textUnderlineOffset":
		return compatLength(ts.UnderlineOffset), !ts.UnderlineOffset.IsAuto()
	case "textTransform":
		names := [...]string{"none", "none", "uppercase", "lowercase", "capitalize"}
		return names[min(int(ts.Transform), len(names)-1)], ts.Transform > 1
	case "textShadowColor":
		return color(t.view.TextShadow.Color)
	case "textShadowOffset":
		o := t.view.TextShadow.Offset
		return fmt.Sprintf("width=%s height=%s", compatNum(o.X), compatNum(o.Y)), o != driver.Point{}
	case "textShadowRadius":
		return num(float32(t.view.TextShadow.Radius))
	case "placeholderColor":
		return color(t.view.Input.PlaceholderColor)
	case "caretColor":
		return color(t.view.Input.CaretColor)
	case "selectionColor":
		if t.view.Kind == driver.KindInput {
			return color(t.view.Input.SelectionColor)
		}
		return color(t.view.Props.SelectionColor)
	}
	return "", false
}

func compatFilters(fs []driver.Filter) string {
	names := map[driver.FilterKind]string{driver.FilterBlur: "blur", driver.FilterBrightness: "brightness",
		driver.FilterContrast: "contrast", driver.FilterGrayscale: "grayscale", driver.FilterHueRotate: "hue-rotate",
		driver.FilterInvert: "invert", driver.FilterOpacity: "opacity", driver.FilterSaturate: "saturate",
		driver.FilterSepia: "sepia", driver.FilterDropShadow: "drop-shadow"}
	var parts []string
	for _, f := range fs {
		arg := compatNum(f.Amount)
		switch f.Kind {
		case driver.FilterHueRotate:
			arg += "deg"
		case driver.FilterDropShadow:
			sh := f.Shadow
			arg = fmt.Sprintf("%s %s %s %s", compatNum(sh.Offset.X), compatNum(sh.Offset.Y), compatNum(sh.Blur),
				compatColor(sh.Color))
		}
		parts = append(parts, names[f.Kind]+"("+arg+")")
	}
	return strings.Join(parts, " ")
}

func compatTransitionProperty(p driver.TransitionProperty) string {
	colors := driver.TransitionBackground | driver.TransitionBorder
	all := colors | driver.TransitionOpacity | driver.TransitionShadow | driver.TransitionTransform | driver.TransitionLayout
	switch p {
	case 0:
		return "none"
	case all:
		return "all"
	case colors:
		return "colors"
	}
	var names []string
	for _, n := range []struct {
		bit  driver.TransitionProperty
		name string
	}{{driver.TransitionOpacity, "opacity"}, {driver.TransitionTransform, "transform"},
		{driver.TransitionShadow, "shadow"}, {driver.TransitionBorder, "shape"}, {driver.TransitionLayout, "layout"},
		{driver.TransitionBackground, "background"}} {
		if p&n.bit != 0 {
			names = append(names, n.name)
		}
	}
	return strings.Join(names, ", ")
}

func compatPosition(p driver.ObjectPosition) string {
	axis := func(a driver.PositionAxis, start, end string) string {
		side := start
		if a.FromEnd {
			side = end
		}
		if a.Offset.IsAuto() {
			return side + " 50%"
		}
		return side + " " + compatLength(a.Offset)
	}
	return axis(p.X, "left", "right") + " " + axis(p.Y, "top", "bottom")
}

// compatStyleIs reports whether key on t has the value want, in NSH's
// text form, and what it has.
func compatStyleIs(t compatTarget, key, want string) (bool, string) {
	got, _ := compatStyle(t, key)
	vs := &t.view.Style
	size := t.view.Frame.Size
	switch key {
	case "backgroundImage", "maskImage":
		gs, err := core.ParseGradients(want)
		if err != nil {
			return false, "an unreadable gradient: " + err.Error()
		}
		var have []driver.GradientPaint
		if vs.Effects != nil {
			have = vs.Effects.Gradients
			if key == "maskImage" {
				have = nil
				if vs.Effects.Mask != nil {
					have = []driver.GradientPaint{*vs.Effects.Mask}
				}
			}
		}
		if len(have) != len(gs) {
			return false, fmt.Sprintf("%d gradients", len(have))
		}
		for i := range gs {
			p, _ := gs[i].Paint(size)
			if !compatPaintsEqual(p, have[i]) {
				return false, fmt.Sprintf("%+v", have[i])
			}
		}
		return true, want
	case "transform":
		m, err := compatTransformMatrix(want, size)
		if err != nil {
			return false, err.Error()
		}
		have := vs.Matrix(size)
		// NSGo keeps a parent's perspective on the parent, where NSH
		// writes it into its children's transforms.
		if p := t.view.Parent; p != nil && p.Style.ChildPerspective > 0 {
			ps, at := p.Frame.Size, t.view.Frame.Origin
			cx, cy := float64(ps.Width/2-at.X), float64(ps.Height/2-at.Y)
			have = driver.Translate3D(cx, cy, 0).Mul(driver.Perspective3D(float64(p.Style.ChildPerspective))).
				Mul(driver.Translate3D(-cx, -cy, 0)).Mul(have)
		}
		for i := range m {
			if math.Abs(m[i]-have[i]) > 1e-3 {
				return false, fmt.Sprintf("%+v", vs.Transform)
			}
		}
		return true, want
	case "transformOrigin":
		f := strings.Fields(want)
		at := func(s string, length driver.Points) driver.Points {
			if p, ok := strings.CutSuffix(s, "%"); ok {
				v, _ := strconv.ParseFloat(p, 32)
				return driver.Points(v) * length / 100
			}
			v, _ := strconv.ParseFloat(s, 32)
			return driver.Points(v)
		}
		x, y := at(f[0], size.Width)-size.Width/2, at(f[1], size.Height)-size.Height/2
		var z driver.Points
		if len(f) > 2 {
			z = at(f[2], 0)
		}
		if math.Abs(float64(x-vs.Origin.X)) > 0.01 || math.Abs(float64(y-vs.Origin.Y)) > 0.01 || z != vs.OriginZ {
			return false, fmt.Sprintf("%v from the center, %v deep", vs.Origin, vs.OriginZ)
		}
		return true, want
	case "borderRadius", "borderWidth", "borderColor":
		// A shorthand the sides override in part sets the others.
		if got == "" {
			c := t.view.Style.Corners
			w, cs := t.view.Style.Borders, t.view.Style.BorderColors
			var sides []string
			switch key {
			case "borderRadius":
				for _, r := range c.Array() {
					sides = append(sides, compatNum(r))
				}
			case "borderWidth":
				sides = []string{compatNum(w.Top), compatNum(w.Right), compatNum(w.Bottom), compatNum(w.Left)}
			default:
				sides = []string{compatColor(cs.Top), compatColor(cs.Right), compatColor(cs.Bottom), compatColor(cs.Left)}
			}
			return slices.Contains(sides, want), strings.Join(sides, " ")
		}
	case "textAlign":
		return got == want || got == "auto" && want == "left", got
	case "writingDirection":
		return got == want || got == "ltr" && want == "auto", got
	case "shadowColor":
		// NSGo's shadow color carries its opacity, which shadowOpacity
		// checks: the color's own channels are checked here.
		return len(got) == 9 && len(want) == 9 && got[:7] == want[:7], got
	case "shadowOpacity":
		// The color's alpha is its own times the opacity.
		o, _ := strconv.ParseFloat(want, 64)
		own := 1.0
		if c, err := core.ParseColor(t.shadowColor); err == nil && t.shadowColor != "" {
			own = float64(c&0xff) / 255
		}
		return math.Abs(float64(vs.ShadowColor&0xff)/255-o*own) <= 1.0/255, got
	}
	return got == want, got
}

// compatFS is the files of the app a case runs.
var compatFS fs.FS

func compatPaintsEqual(a, b driver.GradientPaint) bool {
	close := func(x, y float32) bool { return math.Abs(float64(x-y)) < 1e-3 }
	if a.Kind != b.Kind || len(a.Stops) != len(b.Stops) || !close(a.X0, b.X0) || !close(a.Y0, b.Y0) ||
		!close(a.X1, b.X1) || !close(a.Y1, b.Y1) || !close(a.CX, b.CX) || !close(a.CY, b.CY) ||
		!close(a.RX, b.RX) || !close(a.RY, b.RY) || !close(a.Angle, b.Angle) {
		return false
	}
	for i := range a.Stops {
		if a.Stops[i].Color != b.Stops[i].Color || !close(a.Stops[i].Offset, b.Stops[i].Offset) {
			return false
		}
	}
	return true
}

// compatTransformMatrix reads NSH's text form of a transform, its steps
// in order, into the matrix a view of size draws with around its center.
func compatTransformMatrix(s string, size driver.Size) (driver.Matrix3D, error) {
	m := driver.Translate3D(float64(size.Width/2), float64(size.Height/2), 0)
	for _, step := range strings.Fields(strings.ReplaceAll(s, ") ", ")\n")) {
		name, arg, ok := strings.Cut(strings.TrimSuffix(step, ")"), "(")
		if !ok {
			return m, fmt.Errorf("an unreadable transform step %q", step)
		}
		v, _ := strconv.ParseFloat(strings.TrimSuffix(arg, "deg"), 64)
		var t driver.Transform
		switch name {
		case "translateX":
			t.TranslateX = driver.Points(v)
		case "translateY":
			t.TranslateY = driver.Points(v)
		case "scale":
			t.Scale = float32(v)
		case "scaleX":
			t.ScaleX = float32(v)
		case "scaleY":
			t.ScaleY = float32(v)
		case "rotate", "rotateZ":
			t.Rotate = float32(v)
		case "rotateX":
			t.RotateX = float32(v)
		case "rotateY":
			t.RotateY = float32(v)
		case "skewX":
			t.SkewX = float32(v)
		case "skewY":
			t.SkewY = float32(v)
		case "perspective":
			t.Perspective = driver.Points(v)
		default:
			return m, fmt.Errorf("an unknown transform step %q", name)
		}
		m = m.Mul(t.Matrix(driver.Point{}))
	}
	return m.Mul(driver.Translate3D(-float64(size.Width/2), -float64(size.Height/2), 0)), nil
}

// compatHidden says a prop step reads what NSGo keeps from its platforms:
// a behavior its core or base decides rather than one it hands over.
const compatHidden = "\x00hidden"

var compatRoles = map[driver.Role]string{
	driver.RoleButton: "button", driver.RoleLink: "link", driver.RoleHeader: "header", driver.RoleImage: "image",
	driver.RoleCheckbox: "checkbox", driver.RoleSwitch: "switch", driver.RoleRadio: "radio", driver.RoleTab: "tab",
	driver.RoleText: "text", driver.RoleTextField: "textbox", driver.RoleAdjustable: "adjustable",
	driver.RoleProgressBar: "progressbar", driver.RoleRadioGroup: "radiogroup", driver.RoleTabList: "tablist",
	driver.RoleMenu: "menu", driver.RoleMenuItem: "menuitem", driver.RoleDialog: "dialog", driver.RoleAlert: "alert",
	driver.RoleTable: "table", driver.RoleGrid: "grid", driver.RoleRowGroup: "rowgroup", driver.RoleRow: "row",
	driver.RoleColumnHeader: "columnheader", driver.RoleCell: "cell",
}

// compatPairs writes key=value pairs, those with a value only.
func compatPairs(pairs ...string) string {
	var out []string
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i+1] != "" {
			out = append(out, pairs[i]+"="+pairs[i+1])
		}
	}
	if len(out) == 0 {
		return "{}"
	}
	return strings.Join(out, " ")
}

// compatNonzero writes a number, "" for 0.
func compatNonzero[T ~float32 | ~float64](f T) string {
	if f == 0 {
		return ""
	}
	return compatNum(f)
}

func compatBool(b bool) string { return strconv.FormatBool(b) }

func compatEdges(e driver.Edges) string {
	return compatPairs("top", compatNonzero(e.Top), "right", compatNonzero(e.Right), "bottom", compatNonzero(e.Bottom),
		"left", compatNonzero(e.Left))
}

// compatSource writes an image's source as NSH does: its URI, asset:/
// for an app's assets.
func compatSource(s driver.ImageSource) string { return s.URI }

// compatProp returns the text form of prop key on t, as its platform
// receives it, and whether the view has it set.
func compatProp(t compatTarget, key string) (string, bool) {
	v := t.view
	p := &v.Props
	key = strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(key, "ios."), "android."), "desktop.")
	sc := &v.Scroll
	b := &sc.Behavior
	in := &v.Input
	screen := &v.Screen
	// A dialog's props are its modal's, which holds the view its testID
	// names.
	modal := v
	for modal != nil && modal.Kind != driver.KindModal {
		modal = modal.Parent
	}
	if modal == nil {
		modal = v
	}
	switch key {
	case "role":
		return compatRoles[p.Role], p.Role != driver.RoleNone && !(v.Kind == driver.KindInput && p.Role == driver.RoleTextField)
	case "accessibilityLabel":
		return p.Label, p.Label != ""
	case "accessibilityHint":
		return p.Hint, p.Hint != ""
	case "accessibilityState":
		checked := ""
		switch {
		case p.Indeterminate:
			checked = "mixed"
		case p.Checked:
			checked = "true"
		}
		expanded := ""
		if p.Expandable {
			expanded = compatBool(p.Expanded)
		}
		pick := func(b bool) string {
			if b {
				return "true"
			}
			return ""
		}
		return compatPairs("disabled", pick(p.Disabled), "selected", pick(p.Selected), "checked", checked,
			"expanded", expanded, "pressed", pick(p.Pressed), "busy", pick(p.Busy)), true
	case "accessibilityValue":
		av := p.Value
		now := compatNum(av.Now)
		if av.NoNow {
			now = ""
		}
		s := compatPairs("min", compatNum(av.Min), "max", compatNum(av.Max), "now", now, "text", av.Text)
		if av.Max == 0 && av.Min == 0 {
			s = compatPairs("text", av.Text)
		}
		return s, av != (driver.AccessibilityValue{})
	case "accessible":
		return compatBool(p.Accessible == driver.AccessibleYes), p.Accessible != driver.AccessibleAuto
	case "accessibilityElementsHidden":
		return compatBool(p.ElementsHidden), p.ElementsHidden
	case "accessibilityLiveRegion":
		return [...]string{"none", "polite", "assertive"}[p.LiveRegion], p.LiveRegion != driver.LiveRegionNone
	case "accessibilityActions":
		var names []string
		for _, a := range p.ActionList() {
			n := a.Name
			if a.Label != "" {
				n += ":" + a.Label
			}
			names = append(names, n)
		}
		return strings.Join(names, ", "), len(names) > 0
	case "pointerEvents":
		return [...]string{"auto", "none", "box-none", "box-only"}[p.PointerEvents], p.PointerEvents != driver.PointerEventsAuto
	case "hitSlop":
		return compatEdges(p.HitSlop), p.HitSlop != (driver.Edges{})
	case "delayLongPress":
		return compatMillis(p.DelayLongPress), p.DelayLongPress != 0
	case "focusable":
		return compatBool(!p.NoFocus), p.NoFocus
	case "panActivation":
		a := p.PanActivation
		return compatPairs("activeOffsetX", compatNonzero(a.ActiveOffsetX), "activeOffsetY", compatNonzero(a.ActiveOffsetY),
			"failOffsetX", compatNonzero(a.FailOffsetX), "failOffsetY", compatNonzero(a.FailOffsetY)), a != (driver.PanActivation{})
	case "ripple":
		r := p.Ripple
		if r == nil {
			return "", false
		}
		borderless := ""
		if r.Borderless {
			borderless = "true"
		}
		color := ""
		if r.Color != 0 {
			color = compatColor(r.Color)
		}
		return compatPairs("color", color, "borderless", borderless, "radius", compatNonzero(r.Radius)), true
	case "selectable":
		switch p.Selectable {
		case driver.SelectableOn:
			return "true", true
		case driver.SelectableOff:
			return "false", true
		}
		return "", false
	case "selectableView":
		return compatBool(p.SelectionArea), p.SelectionArea
	case "windowDrag":
		return compatBool(p.WindowDrag == driver.WindowDragOn), p.WindowDrag != driver.WindowDragInherit
	case "pan", "pinch", "rotation", "pullValue", "scrollValue":
		val := func(node uint32) string {
			if node == 0 {
				return ""
			}
			return compatNum(t.app.AnimatedValue(node))
		}
		bd := t.app.Bound(fmt.Sprintf("view:%d", v.ID))
		switch key {
		case "pan":
			return compatPairs("x", val(bd.PanX), "y", val(bd.PanY)), bd.PanX != 0 || bd.PanY != 0
		case "pinch":
			return compatPairs("scale", val(bd.Pinch)), bd.Pinch != 0
		case "rotation":
			return compatPairs("angle", val(bd.Rotation)), bd.Rotation != 0
		case "pullValue":
			return val(bd.Pull), bd.Pull != 0
		}
		node := bd.ScrollY
		if sc.Horizontal {
			node = bd.ScrollX
		}
		return val(node), node != 0
	case "autoFocus":
		// The core focuses the field as it shows.
		return compatBool(v.Focused), v.Focused
	case "elevation":
		return compatNum(v.Style.Elevation), v.Style.Elevation != 0
	case "backdropMaterial":
		return [...]string{"ultraThin", "thin", "regular", "thick", "chrome"}[v.Style.Material], true
	case "numberOfLines":
		return strconv.Itoa(v.MaxLines), v.MaxLines != 0
	case "ellipsizeMode":
		return [...]string{"tail", "tail", "head", "middle", "clip"}[v.Ellipsize], v.Ellipsize > driver.EllipsizeTail

	case "value":
		return v.Text, v.Text != ""
	case "placeholder":
		if v.Kind == driver.KindImage {
			return compatSource(v.Image.Placeholder), v.Image.Placeholder.URI != ""
		}
		return in.Placeholder, in.Placeholder != ""
	case "masked":
		return compatBool(in.Masked), in.Masked
	case "readonly":
		return compatBool(in.Readonly), in.Readonly
	case "maxLength":
		return strconv.Itoa(in.MaxLength), in.MaxLength != 0
	case "keyboardType":
		return [...]string{"default", "email-address", "numeric", "decimal-pad", "phone-pad", "url"}[in.KeyboardType],
			in.KeyboardType != driver.KeyboardDefault
	case "returnKeyType":
		return [...]string{"default", "done", "go", "next", "search", "send"}[in.ReturnKey], in.ReturnKey != driver.ReturnKeyDefault
	case "autoCapitalize":
		return [...]string{"sentences", "none", "words", "characters"}[in.AutoCapitalize],
			in.AutoCapitalize != driver.AutoCapitalizeSentences
	case "autoCorrect":
		return compatBool(!in.NoAutoCorrect), in.NoAutoCorrect
	case "autoComplete", "textContentType":
		return [...]string{"default", "off", "username", "password", "new-password", "one-time-code", "email", "tel",
				"name", "given-name", "family-name", "street-address", "postal-code", "cc-number"}[in.AutoComplete],
			in.AutoComplete != driver.AutoCompleteDefault
	case "submitBehavior":
		return [...]string{"default", "submit", "blurAndSubmit", "newline"}[in.SubmitBehavior],
			in.SubmitBehavior != driver.SubmitBehaviorDefault
	case "pasteBehavior":
		return [...]string{"insert", "report"}[in.PasteBehavior], in.PasteBehavior != driver.PasteInsert
	case "caret":
		return compatBool(!in.HideCaret), in.HideCaret
	case "keyboardAppearance":
		return [...]string{"default", "light", "dark"}[in.KeyboardAppearance], in.KeyboardAppearance != 0
	case "selectedRange":
		return compatPairs("start", strconv.Itoa(v.Selection.Start), "end", strconv.Itoa(v.Selection.End)),
			v.Selection != (driver.TextRange{})
	case "inputAccessoryView":
		return compatBool(in.Accessory != 0), in.Accessory != 0
	case "inputAccessoryViewID":
		return in.AccessoryID, in.AccessoryID != ""

	case "src":
		switch v.Kind {
		case driver.KindVideo:
			return v.Video.Source, v.Video.Source != ""
		case driver.KindWeb:
			return v.Web.Source.Src, v.Web.Source.Src != ""
		}
		return compatSource(v.Image.Source), v.Image.Source.URI != ""
	case "resizeMode":
		mode := v.Image.ResizeMode
		if v.Kind == driver.KindVideo {
			mode = v.Video.ResizeMode
		}
		return [...]string{"cover", "cover", "contain", "stretch", "center", "scale-down"}[mode], mode > driver.Cover
	case "blurRadius":
		return compatNum(v.Image.BlurRadius), v.Image.BlurRadius != 0
	case "fadeDuration":
		return compatMillis(v.Image.FadeDuration), v.Image.FadeDuration != 0
	case "paused":
		return compatBool(v.Video.Paused), v.Video.Paused
	case "muted":
		return compatBool(v.Video.Muted), v.Video.Muted
	case "volume":
		return compatNum(v.Video.Volume), v.Video.Volume != 1
	case "rate":
		return compatNum(v.Video.Rate), v.Video.Rate != 1
	case "repeat":
		return compatBool(v.Video.Repeat), v.Video.Repeat
	case "controls":
		return compatBool(v.Video.Controls), v.Video.Controls
	case "progressUpdateInterval":
		return compatMillis(v.Video.ProgressInterval), true
	case "html":
		return v.Web.Source.HTML, v.Web.Source.HTML != ""
	case "baseUrl":
		return v.Web.Source.BaseURL, v.Web.Source.BaseURL != ""
	case "injectedJavaScript":
		return v.Web.InjectedJavaScript, v.Web.InjectedJavaScript != ""
	case "injectedJavaScriptBeforeContentLoaded":
		return v.Web.InjectedJavaScriptBeforeContent, v.Web.InjectedJavaScriptBeforeContent != ""
	case "userAgent":
		return v.Web.UserAgent, v.Web.UserAgent != ""
	case "javaScriptEnabled":
		return compatBool(!v.Web.NoJavaScript), v.Web.NoJavaScript
	case "allowsBackForwardNavigationGestures":
		return compatBool(v.Web.BackForwardGestures), v.Web.BackForwardGestures

	case "scrollable":
		if v.Kind == driver.KindWeb {
			return compatBool(!v.Web.NoScroll), v.Web.NoScroll
		}
		return compatBool(!b.NoScroll), b.NoScroll
	case "horizontal":
		return compatBool(sc.Horizontal), sc.Horizontal
	case "scrollbar":
		return compatBool(!sc.HideIndicator), sc.HideIndicator
	case "bounces":
		return compatBool(!b.NoBounces), b.NoBounces
	case "decelerationRate":
		return compatNum(b.DecelerationRate), b.DecelerationRate != 0
	case "pagingEnabled":
		return compatBool(b.Paging), b.Paging
	case "snapToInterval":
		return compatNum(b.SnapToInterval), b.SnapToInterval != 0
	case "snapToOffsets":
		var offs []string
		for _, o := range b.SnapToOffsets {
			offs = append(offs, compatNum(o))
		}
		return strings.Join(offs, ", "), len(offs) > 0
	case "snapToAlignment":
		return [...]string{"start", "center", "end"}[b.SnapToAlignment], b.SnapToAlignment != driver.SnapStart
	case "disableIntervalMomentum":
		return compatBool(b.DisableIntervalMomentum), b.DisableIntervalMomentum
	case "contentInset":
		return compatEdges(b.ContentInset), b.ContentInset != (driver.Edges{})
	case "contentOffset":
		return compatPairs("x", compatNonzero(v.Offset.X), "y", compatNonzero(v.Offset.Y)), v.Offset != (driver.Point{})
	case "keyboardDismissMode":
		return [...]string{"none", "on-drag", "interactive"}[b.KeyboardDismissMode], b.KeyboardDismissMode != 0
	case "keyboardShouldPersistTaps":
		return [...]string{"never", "always", "handled"}[b.KeyboardShouldPersistTaps], b.KeyboardShouldPersistTaps != 0
	case "refreshing":
		return compatBool(sc.Refreshing), sc.Refreshing
	case "refreshIndicator":
		return compatPairs("height", compatNonzero(b.RefreshIndicator)), b.RefreshIndicator != 0
	case "zoomScale":
		z := v.ZoomScale
		if z == 0 {
			z = sc.ZoomScale
		}
		return compatNum(z), z != 0
	case "minimumZoomScale":
		return compatNum(sc.MinZoomScale), sc.MinZoomScale != 0
	case "maximumZoomScale":
		return compatNum(sc.MaxZoomScale), sc.MaxZoomScale != 0
	case "bouncesZoom":
		return compatBool(!sc.NoBouncesZoom), sc.NoBouncesZoom
	case "zoomable":
		return compatBool(!sc.NoPinchZoom), sc.NoPinchZoom
	case "centerContent":
		return compatBool(sc.CenterContent), sc.CenterContent
	case "stickyHeaderIndices":
		var cells []string
		for _, c := range v.Sticky {
			end := compatNum(c.End)
			if math.IsInf(float64(c.End), 1) || c.End >= driver.Points(math.MaxFloat32) {
				end = "Infinity"
			}
			cells = append(cells, fmt.Sprintf("start=%s end=%s", compatNum(c.Start), end))
		}
		return strings.Join(cells, ", "), len(cells) > 0

	case "title":
		return screen.Title, screen.Title != ""
	case "headerVisible":
		return compatBool(!screen.HeaderHidden), screen.HeaderHidden
	case "headerTransparent":
		return compatBool(screen.HeaderTransparent), screen.HeaderTransparent
	case "headerLargeTitle":
		return compatBool(screen.LargeTitle), screen.LargeTitle
	case "headerBackVisible":
		return compatBool(!screen.BackHidden), screen.BackHidden
	case "headerBackTitle":
		return screen.OwnBackTitle, screen.OwnBackTitle != ""
	case "headerStyle":
		color := func(c driver.Color) string {
			if c == 0 {
				return ""
			}
			return compatColor(c)
		}
		return compatPairs("backgroundColor", color(screen.Background), "color", color(screen.TitleColor),
			"tintColor", color(screen.TintColor)), screen.Background != 0 || screen.TitleColor != 0 || screen.TintColor != 0
	case "popOnBack":
		return compatBool(!screen.NoPopOnBack), screen.NoPopOnBack
	case "popOnSwipe":
		return compatBool(!screen.NoPopOnSwipe), screen.NoPopOnSwipe
	case "animation":
		if modal.Kind == driver.KindModal {
			return [...]string{"slide", "fade", "none"}[modal.Modal.Animation], modal.Modal.Animation != driver.ModalSlide
		}
		return [...]string{"default", "slide", "fade", "none"}[screen.Animation], screen.Animation != driver.ScreenDefault
	case "presentation":
		return [...]string{"fullScreen", "pageSheet", "formSheet"}[modal.Modal.Presentation], modal.Modal.Presentation != driver.ModalFullScreen
	case "closeOnSwipe":
		return compatBool(!modal.Modal.NoCloseOnSwipe), modal.Modal.NoCloseOnSwipe
	case "statusBarTranslucent":
		return compatBool(modal.Modal.StatusBarTranslucent), modal.Modal.StatusBarTranslucent
	case "closeOnEscape":
		return compatBool(!modal.Modal.NoCloseOnEscape), modal.Modal.NoCloseOnEscape
	case "closeOnBackdropPress":
		return compatBool(!modal.Modal.NoCloseOnBackdropPress), modal.Modal.NoCloseOnBackdropPress
	case "supportedOrientations":
		var names []string
		for _, o := range []struct {
			bit  driver.Orientation
			name string
		}{{driver.OrientationPortrait, "portrait"}, {driver.OrientationPortraitUpsideDown, "portrait-upside-down"},
			{driver.OrientationLandscapeLeft, "landscape-left"}, {driver.OrientationLandscapeRight, "landscape-right"}} {
			if modal.Modal.Orientations&o.bit != 0 {
				names = append(names, o.name)
			}
		}
		return strings.Join(names, ", "), len(names) > 0
	}
	return compatHidden, false
}

// compatPropIs reports whether prop key on t has the value want, and
// what it has.
func compatPropIs(t compatTarget, key, want string) (bool, string) {
	got, _ := compatProp(t, key)
	switch key {
	case "selectedRange":
		// The field holds the range within its text, as a platform does.
		p := compatPayloadOf(strings.Fields(want))
		n := driver.UTF16Len(t.view.Text)
		clamp := func(x float32) int { return min(max(int(x), 0), n) }
		r := driver.TextRange{Start: clamp(p.num("start")), End: clamp(p.num("end"))}
		return r == t.view.Selection, got
	case "accessibilityState", "accessibilityValue", "hitSlop", "contentInset", "contentOffset", "panActivation":
		// What the text leaves out is false or 0 there.
		norm := func(s string) string {
			var keep []string
			for _, f := range strings.Fields(s) {
				if k, v, ok := strings.Cut(f, "="); ok && (v == "false" || v == "0") && k != "checked" && k != "expanded" {
					continue
				}
				keep = append(keep, f)
			}
			if len(keep) == 0 {
				return "{}"
			}
			return strings.Join(keep, " ")
		}
		w := norm(want)
		if key == "accessibilityState" && strings.Contains(w, "checked=false") && !strings.Contains(got, "checked=") {
			w = strings.TrimSpace(strings.ReplaceAll(w, "checked=false", ""))
			if w == "" {
				w = "{}"
			}
		}
		return norm(got) == w, got
	}
	if strings.HasPrefix(want, "asset:/") && strings.HasPrefix(got, "data:") && compatFS != nil {
		// NSGo's bundles carry their assets as data: URLs.
		data, err := fs.ReadFile(compatFS, strings.TrimPrefix(want, "asset:/"))
		_, b64, _ := strings.Cut(got, ";base64,")
		have, _ := base64.StdEncoding.DecodeString(b64)
		return err == nil && bytes.Equal(data, have), got
	}
	return got == want, got
}

// compatDrawing lists what canvas v's drawing does, an operation to a
// line as a script's drawing steps say it, what a slot binds as
// "animated": a path a fill takes with it is in the fill's line, as NSH's
// recording keeps a Path2D.
func compatDrawing(v *nsgotest.View) []string {
	if v.Drawing == nil {
		return nil
	}
	n := func(x float32) string { return strconv.FormatFloat(math.Round(float64(x)*1000)/1000, 'f', -1, 64) }
	var out, path []string
	for i, op := range v.Drawing.Ops {
		arg := func(a driver.DrawArg, x float32) string {
			for _, s := range v.Drawing.Slots {
				if s.Op == i && s.Arg == a {
					return "animated"
				}
			}
			return n(x)
		}
		line := func(name string, args ...string) string { return strings.Join(append([]string{name}, args...), " ") }
		built := ""
		switch op.Kind {
		case driver.DrawBeginPath:
			path = nil
			continue
		case driver.DrawMoveTo:
			built = line("moveTo", arg(driver.ArgX, op.X), arg(driver.ArgY, op.Y))
		case driver.DrawLineTo:
			built = line("lineTo", arg(driver.ArgX, op.X), arg(driver.ArgY, op.Y))
		case driver.DrawClosePath:
			built = "closePath"
		case driver.DrawRect:
			built = line("rect", arg(driver.ArgX, op.X), arg(driver.ArgY, op.Y), arg(driver.ArgW, op.W), arg(driver.ArgH, op.H))
		case driver.DrawRoundRect:
			r := arg(driver.ArgR, op.R)
			built = line("roundRect", arg(driver.ArgX, op.X), arg(driver.ArgY, op.Y), arg(driver.ArgW, op.W), arg(driver.ArgH, op.H), r, r, r, r)
		case driver.DrawArc:
			built = line("arc", arg(driver.ArgX, op.X), arg(driver.ArgY, op.Y), arg(driver.ArgR, op.R), arg(driver.ArgA0, op.A0), arg(driver.ArgA1, op.A1))
		}
		if built != "" {
			out, path = append(out, built), append(path, built)
			continue
		}
		switch op.Kind {
		case driver.DrawFill, driver.DrawStroke, driver.DrawClip:
			name := map[driver.DrawKind]string{driver.DrawFill: "fill", driver.DrawStroke: "stroke", driver.DrawClip: "clip"}[op.Kind]
			out = append(out, name, name+" path("+strings.Join(path, ", ")+")")
			if r, ok := strings.CutPrefix(strings.Join(path, ", "), "rect "); ok && op.Kind == driver.DrawFill && len(path) == 1 {
				out = append(out, "fillRect "+r)
			}
		case driver.DrawFillColor:
			out = append(out, "fillStyle "+compatColor(op.Color))
		case driver.DrawStrokeColor:
			out = append(out, "strokeStyle "+compatColor(op.Color))
		case driver.DrawFillGradient, driver.DrawStrokeGradient:
			g := op.Gradient
			name := map[driver.DrawKind]string{driver.DrawFillGradient: "fillStyle", driver.DrawStrokeGradient: "strokeStyle"}[op.Kind]
			out = append(out, fmt.Sprintf("%s createLinearGradient(%s, %s, %s, %s)...", name, n(g.X1), n(g.Y1), n(g.X2), n(g.Y2)))
		case driver.DrawLineWidth:
			out = append(out, "lineWidth "+arg(driver.ArgW, op.W))
		case driver.DrawTranslate:
			out = append(out, line("translate", n(op.X), n(op.Y)))
		case driver.DrawLineDash:
			if op.Dash != nil && len(*op.Dash) > 0 {
				dash := make([]string, len(*op.Dash))
				for j, d := range *op.Dash {
					dash[j] = n(d)
				}
				out = append(out, "setLineDash "+strings.Join(dash, " "))
			}
		case driver.DrawFillText:
			family := op.Font.Family
			if family == "" {
				family = "sans-serif"
			}
			out = append(out, fmt.Sprintf("font %spx %s", n(float32(op.Font.Size)), family),
				fmt.Sprintf("fillText %q %s %s", op.Text, n(op.X), n(op.Y)))
		}
	}
	return out
}

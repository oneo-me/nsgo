package tsx

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/dop251/goja"

	"github.com/oneo-me/nsgo/core"
)

// Style objects decoded into core.Style (NSH's style.rs style_keys!):
// each of NSH's keys sets the field of the same meaning. Keys taking
// animated nodes (opacity, colors, transform steps) bind them through
// core.AnimatedStyle.

// style decodes a style object, an array of them (later ones winning,
// falsy entries skipped) or nothing.
func (r *runtime) style(v goja.Value) core.Style {
	var s core.Style
	if items := r.arrayItems(v); items != nil {
		for _, it := range items {
			s = s.Merge(r.style(it))
		}
		return s
	}
	o, ok := v.(*goja.Object)
	if !ok {
		return s
	}
	// Shorthands first, so the longhands set over them in any order, as
	// NSH's style does.
	keys := o.Keys()
	for pass := range 3 {
		for _, k := range keys {
			if styleOrder(k) == pass {
				r.styleKey(&s, k, o.Get(k))
			}
		}
	}
	return s
}

// styleOrder is the pass that sets key: shorthands, then the others,
// then those applied over them (shadowOpacity over shadowColor).
func styleOrder(key string) int {
	switch {
	case styleShorthands[key]:
		return 0
	case key == "shadowOpacity":
		return 2
	}
	return 1
}

var styleShorthands = map[string]bool{
	"flex": true, "margin": true, "marginHorizontal": true, "marginVertical": true,
	"padding": true, "paddingHorizontal": true, "paddingVertical": true, "borderWidth": true,
	"borderColor": true, "borderRadius": true, "gridColumn": true, "gridRow": true,
}

// animated returns s's animated bindings, made on first use.
func animated(s *core.Style) *core.AnimatedStyle {
	if s.Animated == nil {
		s.Animated = &core.AnimatedStyle{}
	}
	return s.Animated
}

// styleKey sets one key of NSH's style objects.
func (r *runtime) styleKey(s *core.Style, key string, v goja.Value) {
	if !defined(v) {
		return
	}
	if n := r.animatedNodeOf(v); n != nil {
		r.animatedKey(s, key, n)
		return
	}
	switch key {
	case "display":
		if v.String() == "flow-root" {
			s.Display = core.DisplayBlock
			break
		}
		s.Display = pick(r, key, v, map[string]core.Display{"flex": core.DisplayFlex, "none": core.DisplayNone,
			"grid": core.DisplayGrid, "block": core.DisplayBlock})
	case "boxSizing":
		s.BoxSizing = pick(r, key, v, map[string]core.BoxSizing{"border-box": core.BorderBox, "content-box": core.ContentBox})
	case "position":
		s.Position = pick(r, key, v, map[string]core.Position{"relative": core.Relative, "absolute": core.Absolute,
			"static": core.Static, "fixed": core.Fixed, "sticky": core.Sticky})
	case "flexDirection":
		s.Direction = pick(r, key, v, map[string]core.FlexDirection{"column": core.Column, "row": core.Row,
			"column-reverse": core.ColumnReverse, "row-reverse": core.RowReverse})
	case "flexWrap":
		s.Wrap = pick(r, key, v, map[string]core.FlexWrap{"nowrap": core.NoWrap, "wrap": core.Wrap,
			"wrap-reverse": core.WrapReverse})
	case "flex":
		// React Native's meaning, not CSS's shorthand.
		switch f := float32(finite(v)); {
		case f > 0:
			s.FlexGrow, s.FlexShrink, s.FlexBasis = f, 1, core.Pt(0)
		case f < 0:
			s.FlexGrow, s.FlexShrink = 0, -f
		default:
			s.FlexGrow, s.FlexShrink = 0, 0
		}
	case "flexGrow":
		s.FlexGrow = float32(finite(v))
	case "flexShrink":
		s.FlexShrink = float32(finite(v))
	case "flexBasis":
		s.FlexBasis = r.length(key, v)
	case "justifyContent":
		s.JustifyContent = pick(r, key, v, justifies)
	case "alignItems":
		s.AlignItems = pick(r, key, v, aligns)
	case "alignSelf":
		s.AlignSelf = pick(r, key, v, aligns)
	case "alignContent":
		s.AlignContent = pick(r, key, v, aligns)
	case "justifyItems":
		s.JustifyItems = pick(r, key, v, aligns)
	case "justifySelf":
		s.JustifySelf = pick(r, key, v, aligns)
	case "width":
		s.Width = r.length(key, v)
	case "height":
		s.Height = r.length(key, v)
	case "minWidth":
		s.MinWidth = r.length(key, v)
	case "minHeight":
		s.MinHeight = r.length(key, v)
	case "maxWidth":
		s.MaxWidth = r.length(key, v)
	case "maxHeight":
		s.MaxHeight = r.length(key, v)
	case "aspectRatio":
		s.AspectRatio = float32(finite(v))
	case "top":
		s.Top = r.length(key, v)
	case "right", "end":
		s.Right = r.length(key, v)
	case "bottom":
		s.Bottom = r.length(key, v)
	case "left", "start":
		s.Left = r.length(key, v)
	case "zIndex":
		s.ZIndex = int(v.ToInteger())
	case "direction":
		s.LayoutDirection = map[string]core.LayoutDirection{"ltr": core.DirectionLTR, "rtl": core.DirectionRTL}[v.String()]
	case "margin":
		r.edge(s, key, v, true, core.SidesAll)
	case "marginHorizontal":
		r.edge(s, key, v, true, core.SidesHorizontal)
	case "marginVertical":
		r.edge(s, key, v, true, core.SidesVertical)
	case "marginTop":
		r.edge(s, key, v, true, core.SideTop)
	case "marginRight", "marginEnd":
		r.edge(s, key, v, true, core.SideRight)
	case "marginBottom":
		r.edge(s, key, v, true, core.SideBottom)
	case "marginLeft", "marginStart":
		r.edge(s, key, v, true, core.SideLeft)
	case "padding":
		r.edge(s, key, v, false, core.SidesAll)
	case "paddingHorizontal":
		r.edge(s, key, v, false, core.SidesHorizontal)
	case "paddingVertical":
		r.edge(s, key, v, false, core.SidesVertical)
	case "paddingTop":
		r.edge(s, key, v, false, core.SideTop)
	case "paddingRight", "paddingEnd":
		r.edge(s, key, v, false, core.SideRight)
	case "paddingBottom":
		r.edge(s, key, v, false, core.SideBottom)
	case "paddingLeft", "paddingStart":
		r.edge(s, key, v, false, core.SideLeft)
	case "gap":
		s.Gap = points(v)
	case "rowGap":
		s.RowGap = points(v)
	case "columnGap":
		s.ColumnGap = points(v)
	case "gridTemplateColumns":
		s.GridTemplateColumns = core.GridTracks(v.String())
	case "gridTemplateRows":
		s.GridTemplateRows = core.GridTracks(v.String())
	case "gridAutoColumns":
		s.GridAutoColumns = core.GridTracks(v.String())
	case "gridAutoRows":
		s.GridAutoRows = core.GridTracks(v.String())
	case "gridAutoFlow":
		s.GridAutoFlow = pick(r, key, v, map[string]core.GridFlow{"row": core.FlowRow, "column": core.FlowColumn,
			"dense": core.FlowRowDense, "row dense": core.FlowRowDense, "column dense": core.FlowColumnDense})
	case "gridColumn":
		s.GridColumn = gridLines(v.String())
	case "gridRow":
		s.GridRow = gridLines(v.String())
	case "gridColumnStart":
		s.GridColumn.Start = gridPlacement(v.String())
	case "gridColumnEnd":
		s.GridColumn.End = gridPlacement(v.String())
	case "gridRowStart":
		s.GridRow.Start = gridPlacement(v.String())
	case "gridRowEnd":
		s.GridRow.End = gridPlacement(v.String())

	case "backgroundColor":
		s.Background = r.color(key, v)
	case "backgroundImage":
		g, ok := cssGradients(v.String())
		r.check(key, v, ok)
		s.Gradients = g
	case "borderRadius":
		s.CornerRadius = points(v)
		s.CornerRadii = core.AllCorners(points(v))
	case "borderTopLeftRadius", "borderTopStartRadius":
		s.CornerRadii.TopLeft = points(v)
	case "borderTopRightRadius", "borderTopEndRadius":
		s.CornerRadii.TopRight = points(v)
	case "borderBottomRightRadius", "borderBottomEndRadius":
		s.CornerRadii.BottomRight = points(v)
	case "borderBottomLeftRadius", "borderBottomStartRadius":
		s.CornerRadii.BottomLeft = points(v)
	case "borderCurve":
		s.BorderCurve = pick(r, key, v, map[string]core.BorderCurve{"circular": core.BorderCurveCircular,
			"continuous": core.BorderCurveContinuous})
	case "borderWidth":
		s.BorderWidth = points(v)
		s.Borders = core.All(points(v))
	case "borderTopWidth":
		s.Borders.Top = points(v)
	case "borderRightWidth", "borderEndWidth":
		s.Borders.Right = points(v)
	case "borderBottomWidth":
		s.Borders.Bottom = points(v)
	case "borderLeftWidth", "borderStartWidth":
		s.Borders.Left = points(v)
	case "borderColor":
		s.BorderColor = core.SetColor(r.color(key, v))
		s.BorderColors = core.AllColors(s.BorderColor)
	case "borderTopColor":
		s.BorderColors.Top = core.SetColor(r.color(key, v))
	case "borderRightColor", "borderEndColor":
		s.BorderColors.Right = core.SetColor(r.color(key, v))
	case "borderBottomColor":
		s.BorderColors.Bottom = core.SetColor(r.color(key, v))
	case "borderLeftColor", "borderStartColor":
		s.BorderColors.Left = core.SetColor(r.color(key, v))
	case "borderStyle":
		s.BorderStyle = pick(r, key, v, borderStyles)
	case "outlineWidth":
		s.OutlineWidth = points(v)
	case "outlineColor":
		s.OutlineColor = r.color(key, v)
	case "outlineStyle":
		s.OutlineStyle = pick(r, key, v, borderStyles)
	case "outlineOffset":
		s.OutlineOffset = points(v)
	case "opacity":
		s.Opacity = core.Alpha(float32(finite(v)))
	case "visibility":
		s.Visibility = pick(r, key, v, map[string]core.Visibility{"visible": core.VisibilityVisible,
			"hidden": core.VisibilityHidden, "collapse": core.VisibilityHidden})
	case "overflow":
		s.Overflow = pick(r, key, v, map[string]core.Overflow{"visible": core.Visible, "hidden": core.Hidden,
			"scroll": core.OverflowScroll})
	case "boxShadow":
		sh, ok := cssShadows(v.String())
		r.check(key, v, ok)
		s.Shadows = sh
	case "shadowColor":
		s.ShadowColor = r.color(key, v)
	case "shadowOffset":
		s.ShadowOffset = r.offset(v)
	case "shadowOpacity":
		// After shadowColor (late keys), black when it is unset.
		if s.ShadowColor == 0 {
			s.ShadowColor = core.RGB(0)
		}
		s.ShadowColor = s.ShadowColor.Faded(float32(finite(v)))
	case "shadowRadius":
		s.ShadowRadius = points(v)
	case "filter":
		f, ok := cssFilters(v.String())
		r.check(key, v, ok)
		s.Filters = f
	case "backdropFilter":
		f, ok := cssFilters(v.String())
		r.check(key, v, ok)
		s.BackdropFilters = f
	case "maskImage":
		g, ok := cssGradients(v.String())
		r.check(key, v, ok)
		if len(g) > 0 {
			s.Mask = &g[0]
		}
	case "glassEffect":
		s.GlassEffect = pick(r, key, v, map[string]core.GlassEffect{"none": core.GlassNone,
			"regular": core.GlassRegular, "clear": core.GlassClear})
	case "mixBlendMode":
		s.BlendMode = pick(r, key, v, blendModes)
	case "cursor":
		c, ok := cursors[v.String()]
		if !ok {
			c = core.CursorAuto
		}
		s.Cursor = c
	case "backfaceVisibility":
		s.BackfaceHidden = v.String() == "hidden"
	case "transform":
		r.transform(s, v)
	case "transformOrigin":
		origin := v.String()
		s.TransformOriginZ = 0
		if items := r.arrayItems(v); items != nil {
			if len(items) > 2 {
				s.TransformOriginZ = points(items[2])
			}
			parts := make([]string, len(items))
			for i, it := range items {
				parts[i] = it.String()
				if _, isNum := it.Export().(string); !isNum {
					parts[i] += "px"
				}
			}
			origin = strings.Join(parts[:min(len(parts), 2)], " ")
		}
		if f := strings.Fields(origin); len(f) == 3 {
			// x y z: the depth is a length.
			z, ok := cssLength(f[2])
			r.check(key, v, ok)
			origin, s.TransformOriginZ = f[0]+" "+f[1], z
		}
		p, ok := cssObjectPosition(origin)
		r.check(key, v, ok)
		s.TransformOrigin = p
	case "transitionProperty":
		s.Transition.Properties = transitionProperties(v.String())
	case "transitionDuration":
		s.Transition.Duration = r.duration(key, v)
	case "transitionDelay":
		s.Transition.Delay = r.duration(key, v)
	case "transitionTimingFunction":
		c, ok := cssTimingFunction(v.String())
		r.check(key, v, ok)
		s.Transition.Easing = c
	case "animationName":
		s.Animation.Keyframes = pick(r, key, v, map[string]core.Keyframes{"none": 0, "spin": core.KeyframesSpin,
			"ping": core.KeyframesPing, "pulse": core.KeyframesPulse, "bounce": core.KeyframesBounce})
	case "animationDuration":
		s.Animation.Duration = r.duration(key, v)
	case "animationDelay":
		s.Animation.Delay = r.duration(key, v)
	case "animationTimingFunction":
		c, ok := cssTimingFunction(v.String())
		r.check(key, v, ok)
		s.Animation.Easing = c
	case "animationIterationCount":
		if v.String() == "infinite" {
			s.Animation.Iterations = 0
		} else {
			s.Animation.Iterations = int(v.ToInteger())
		}
	case "fill":
		s.Fill = r.paint(key, v)
	case "stroke":
		s.Stroke = r.paint(key, v)
	case "strokeWidth":
		l, _ := cssLength(v.String())
		s.StrokeWidth = l
	case "strokeDasharray":
		s.StrokeDash = nil
		for _, w := range strings.FieldsFunc(v.String(), func(c rune) bool { return c == ',' || c == ' ' }) {
			if l, ok := cssLength(w); ok {
				s.StrokeDash = append(s.StrokeDash, l)
			}
		}
	case "verticalAlign":
		s.VerticalAlign = pick(r, key, v, map[string]core.VerticalAlign{"baseline": core.VerticalAlignBaseline,
			"text-top": core.VerticalAlignTextTop, "text-bottom": core.VerticalAlignTextBottom,
			"middle": core.VerticalAlignMiddle})
	case "tintColor":
		s.TintColor = r.color(key, v)
	case "objectPosition":
		p, ok := cssObjectPosition(v.String())
		r.check(key, v, ok)
		s.ObjectPosition = p

	case "color":
		s.Color = r.color(key, v)
	case "fontSize":
		s.FontSize = points(v)
	case "fontWeight":
		s.FontWeight = r.fontWeight(v)
	case "fontFamily":
		s.FontFamily = v.String()
	case "fontStyle":
		s.FontStyle = pick(r, key, v, map[string]core.FontStyle{"normal": core.FontStyleNormal,
			"italic": core.FontStyleItalic})
	case "lineHeight":
		if em, ok := emOf(v); ok {
			s.LineHeight, s.LineHeightEm = 0, em
		} else {
			s.LineHeight, s.LineHeightEm = points(v), 0
		}
	case "letterSpacing":
		if em, ok := emOf(v); ok {
			s.LetterSpacing, s.LetterSpacingEm = 0, em
		} else {
			s.LetterSpacing, s.LetterSpacingEm = points(v), 0
		}
	case "textAlign":
		s.TextAlign = pick(r, key, v, map[string]core.TextAlign{"left": core.TextAlignStart, "auto": core.TextAlignStart,
			"center": core.TextAlignCenter, "right": core.TextAlignEnd, "justify": core.TextAlignJustify})
	case "textAlignVertical":
		s.TextAlignVertical = map[string]core.TextAlignVertical{"top": core.TextAlignVerticalTop,
			"center": core.TextAlignVerticalCenter, "bottom": core.TextAlignVerticalBottom}[v.String()]
	case "textDecorationLine":
		var d core.TextDecoration
		for _, w := range strings.Fields(v.String()) {
			switch w {
			case "underline":
				d |= core.TextDecorationUnderline
			case "line-through":
				d |= core.TextDecorationLineThrough
			case "none":
			default:
				r.warn("style key %q has no value %q", key, w)
			}
		}
		if d == 0 {
			d = core.TextDecorationNone
		}
		s.TextDecoration = d
	case "textDecorationStyle":
		s.TextDecorationStyle = pick(r, key, v, map[string]core.TextDecorationStyle{"solid": core.DecorationSolid,
			"double": core.DecorationDouble, "dotted": core.DecorationDotted, "dashed": core.DecorationDashed,
			"wavy": core.DecorationWavy})
	case "textDecorationColor":
		s.TextDecorationColor = r.color(key, v)
	case "textDecorationThickness":
		if str, ok := v.Export().(string); ok && (str == "auto" || str == "from-font") {
			s.TextDecorationThickness = core.Length{}
			break
		}
		s.TextDecorationThickness = r.length(key, v)
	case "textUnderlineOffset":
		s.TextUnderlineOffset = r.length(key, v)
	case "textTransform":
		s.TextTransform = pick(r, key, v, map[string]core.TextTransform{"none": core.TextTransformNone,
			"uppercase": core.TextTransformUppercase, "lowercase": core.TextTransformLowercase,
			"capitalize": core.TextTransformCapitalize})
	case "textShadowColor":
		s.TextShadowColor = r.color(key, v)
	case "textShadowOffset":
		s.TextShadowOffset = r.offset(v)
	case "textShadowRadius":
		s.TextShadowRadius = points(v)
	case "writingDirection":
		s.WritingDirection = map[string]core.WritingDirection{"auto": core.WritingAuto, "ltr": core.WritingLTR,
			"rtl": core.WritingRTL}[v.String()]
	case "placeholderColor":
		s.PlaceholderColor = r.color(key, v)
	case "selectionColor":
		s.SelectionColor = r.color(key, v)
	case "caretColor":
		s.CaretColor = r.color(key, v)
	case "fontVariant":
		s.FontVariant = core.FontVariantNormal
		var names []string
		if err := r.vm.ExportTo(v, &names); err != nil {
			names = strings.Fields(v.String())
		}
		for _, name := range names {
			s.FontVariant |= pick(r, key, r.vm.ToValue(name), fontVariants)
		}
	default:
		r.warn("style key %q is not supported", key)
	}
}

// animatedKey binds a key that takes an animated node.
func (r *runtime) animatedKey(s *core.Style, key string, n core.AnimatedNode) {
	a := animated(s)
	switch key {
	case "opacity":
		a.Opacity = n
	case "backgroundColor":
		a.Background = n
	case "borderColor":
		a.BorderColor = n
	case "shadowColor":
		a.ShadowColor = n
	case "shadowOpacity":
		a.ShadowOpacity = n
	case "shadowRadius":
		a.ShadowRadius = n
	case "color":
		a.TextColor = n
	case "top", "left":
		a.StickyInset = n
	default:
		r.warn("style key %q takes no animated value", key)
	}
}

// transform reads the steps of a transform, animated ones bound.
func (r *runtime) transform(s *core.Style, v goja.Value) {
	var t core.Transform
	for _, step := range r.arrayItems(v) {
		o, ok := step.(*goja.Object)
		if !ok {
			continue
		}
		for _, k := range o.Keys() {
			x := o.Get(k)
			if n := r.animatedNodeOf(x); n != nil {
				a := animated(s)
				switch k {
				case "translateX":
					a.TranslateX = n
				case "translateY":
					a.TranslateY = n
				case "scale":
					a.Scale = n
				case "scaleX":
					a.ScaleX = n
				case "scaleY":
					a.ScaleY = n
				case "rotate", "rotateZ":
					a.Rotate = n
				case "rotateX":
					a.RotateX = n
				case "rotateY":
					a.RotateY = n
				default:
					r.warn("transform step %q takes no animated value", k)
				}
				continue
			}
			switch k {
			case "translateX", "translateY":
				p, pct := float32(0), float32(0)
				if str, ok := x.Export().(string); ok && strings.HasSuffix(str, "%") {
					f, _ := strconv.ParseFloat(strings.TrimSuffix(str, "%"), 32)
					pct = float32(f)
				} else {
					p = float32(finite(x))
				}
				if k == "translateX" {
					t.TranslateX, t.TranslateXPercent = core.Points(p), pct
				} else {
					t.TranslateY, t.TranslateYPercent = core.Points(p), pct
				}
			case "scale":
				t.Scale = float32(finite(x))
			case "scaleX":
				t.ScaleX = float32(finite(x))
			case "scaleY":
				t.ScaleY = float32(finite(x))
			case "rotate", "rotateZ":
				t.Rotate = r.degrees(x)
			case "rotateX":
				t.RotateX = r.degrees(x)
			case "rotateY":
				t.RotateY = r.degrees(x)
			case "skewX":
				t.SkewX = r.degrees(x)
			case "skewY":
				t.SkewY = r.degrees(x)
			case "perspective":
				t.Perspective = points(x)
			default:
				r.warn("transform step %q is not supported", k)
			}
		}
	}
	s.Transform = t
}

// degrees reads an angle: a number of degrees or "45deg".
func (r *runtime) degrees(v goja.Value) float32 {
	if str, ok := v.Export().(string); ok {
		return float32(angleDegrees(str))
	}
	return float32(finite(v))
}

// edge sets a margin's or a padding's sides: a number, a percentage,
// "auto" (margins), env() or calc(env() + length).
func (r *runtime) edge(s *core.Style, key string, v goja.Value, margin bool, sides core.Sides) {
	var amount core.Points
	env := core.EnvNone
	auto, percent := false, false
	if str, ok := v.Export().(string); ok {
		str = strings.TrimSpace(str)
		switch {
		case str == "auto" && margin:
			auto = true
		case strings.HasSuffix(str, "%"):
			f, ok := cssNumber(strings.TrimSuffix(str, "%"))
			r.check(key, v, ok)
			amount, percent = core.Points(f), true
		case strings.Contains(str, "env("):
			var ok bool
			env, amount, ok = envTerm(str)
			r.check(key, v, ok)
		default:
			l, ok := cssLength(str)
			r.check(key, v, ok)
			amount = l
		}
	} else {
		amount = points(v)
	}
	edges, envs, pct := &s.Padding, &s.PaddingEnv, &s.PaddingPercent
	if margin {
		edges, envs, pct = &s.Margin, &s.MarginEnv, &s.MarginPercent
	}
	set := func(side core.Sides, e *core.Points, ev *core.EnvInset) {
		if sides&side == 0 {
			return
		}
		*e, *ev = amount, env
		if percent {
			*pct |= side
		} else {
			*pct &^= side
		}
		if margin {
			if auto {
				s.AutoMargin |= side
			} else {
				s.AutoMargin &^= side
			}
		}
	}
	set(core.SideTop, &edges.Top, &envs.Top)
	set(core.SideRight, &edges.Right, &envs.Right)
	set(core.SideBottom, &edges.Bottom, &envs.Bottom)
	set(core.SideLeft, &edges.Left, &envs.Left)
}

// envTerm reads env(safe-area-inset-*) or env(keyboard-inset-height),
// alone or in calc(env(...) + length).
func envTerm(s string) (core.EnvInset, core.Points, bool) {
	env, amount, err := core.ParseEnvLength(s)
	return env, amount, err == nil
}

// gridLines reads "1 / 3", "span 2", a line or a name.
func gridLines(s string) core.GridLines {
	g, _ := core.ParseGridLines(s)
	return g
}

// gridPlacement reads one end of a grid item.
func gridPlacement(s string) core.GridPlacement {
	p, _ := core.ParseGridPlacement(s)
	return p
}

var transitionNames = map[string]core.TransitionProperty{
	"opacity": core.TransitionOpacity, "transform": core.TransitionTransform, "translate": core.TransitionTransform,
	"scale": core.TransitionTransform, "rotate": core.TransitionTransform, "box-shadow": core.TransitionShadow,
	"boxShadow": core.TransitionShadow, "shadow": core.TransitionShadow, "border-radius": core.TransitionBorder,
	"borderRadius": core.TransitionBorder, "border-width": core.TransitionBorder, "borderWidth": core.TransitionBorder,
	"width": core.TransitionLayout, "height": core.TransitionLayout, "min-width": core.TransitionLayout,
	"minWidth": core.TransitionLayout, "max-width": core.TransitionLayout, "maxWidth": core.TransitionLayout,
	"min-height": core.TransitionLayout, "minHeight": core.TransitionLayout, "max-height": core.TransitionLayout,
	"maxHeight": core.TransitionLayout, "inset": core.TransitionLayout, "top": core.TransitionLayout,
	"right": core.TransitionLayout, "bottom": core.TransitionLayout, "left": core.TransitionLayout,
}

// transitionProperties reads a transition-property list.
func transitionProperties(s string) core.TransitionProperty {
	colors := core.TransitionBackground | core.TransitionBorder
	all := colors | core.TransitionOpacity | core.TransitionShadow | core.TransitionTransform | core.TransitionLayout
	var p core.TransitionProperty
	for _, name := range strings.Split(s, ",") {
		switch name = strings.TrimSpace(name); name {
		case "all":
			p |= all
		case "color", "colors", "background-color", "backgroundColor", "border-color", "borderColor",
			"outline-color", "text-decoration-color", "fill", "stroke":
			p |= colors
		default:
			p |= transitionNames[name]
		}
	}
	return p
}

func (r *runtime) duration(key string, v goja.Value) time.Duration {
	ms, ok := cssDuration(v.String())
	r.check(key, v, ok)
	return millis(ms)
}

// paint reads none, currentColor or a color.
func (r *runtime) paint(key string, v goja.Value) core.Paint {
	switch v.String() {
	case "none":
		return core.PaintNone
	case "currentColor":
		return core.PaintCurrentColor
	}
	return core.PaintColor(r.color(key, v))
}

// offset reads { width, height }.
func (r *runtime) offset(v goja.Value) core.Point {
	o := r.options(v)
	return core.Point{X: core.Points(o.float("width", 0)), Y: core.Points(o.float("height", 0))}
}

// check warns once about a value of key that did not read.
func (r *runtime) check(key string, v goja.Value, ok bool) {
	if !ok {
		r.warn("style key %q has no value %q", key, v.String())
	}
}

var borderStyles = map[string]core.BorderStyle{"solid": core.BorderSolid, "dashed": core.BorderDashed,
	"dotted": core.BorderDotted, "none": core.BorderNone, "hidden": core.BorderHidden}

var blendModes = map[string]core.BlendMode{"normal": core.BlendNormal, "multiply": core.BlendMultiply,
	"screen": core.BlendScreen, "overlay": core.BlendOverlay, "darken": core.BlendDarken,
	"lighten": core.BlendLighten, "color-dodge": core.BlendColorDodge, "color-burn": core.BlendColorBurn,
	"hard-light": core.BlendHardLight, "soft-light": core.BlendSoftLight, "difference": core.BlendDifference,
	"exclusion": core.BlendExclusion, "hue": core.BlendHue, "saturation": core.BlendSaturation,
	"color": core.BlendColor, "luminosity": core.BlendLuminosity, "plus-lighter": core.BlendPlusLighter,
	"plus-darker": core.BlendPlusDarker}

// cursors are the cursors by CSS name; the others are auto.
var cursors = map[string]core.Cursor{"auto": core.CursorAuto, "default": core.CursorDefault,
	"pointer": core.CursorPointer, "text": core.CursorText, "vertical-text": core.CursorText,
	"wait": core.CursorWait, "progress": core.CursorProgress, "help": core.CursorHelp,
	"crosshair": core.CursorCrosshair, "move": core.CursorMove, "all-scroll": core.CursorMove,
	"grab": core.CursorGrab, "grabbing": core.CursorGrabbing, "not-allowed": core.CursorNotAllowed,
	"no-drop": core.CursorNotAllowed, "zoom-in": core.CursorZoomIn, "zoom-out": core.CursorZoomOut,
	"col-resize": core.CursorColResize, "e-resize": core.CursorColResize, "w-resize": core.CursorColResize,
	"ew-resize": core.CursorColResize, "row-resize": core.CursorRowResize, "n-resize": core.CursorRowResize,
	"s-resize": core.CursorRowResize, "ns-resize": core.CursorRowResize, "none": core.CursorNone}

var fontVariants = map[string]core.FontVariant{"small-caps": core.FontSmallCaps,
	"oldstyle-nums": core.FontOldstyleNums, "lining-nums": core.FontLiningNums,
	"tabular-nums": core.FontTabularNums, "proportional-nums": core.FontProportionalNums}

var justifies = map[string]core.Justify{"flex-start": core.JustifyStart, "start": core.JustifyStart,
	"center": core.JustifyCenter, "flex-end": core.JustifyEnd, "end": core.JustifyEnd,
	"space-between": core.JustifySpaceBetween, "space-around": core.JustifySpaceAround,
	"space-evenly": core.JustifySpaceEvenly, "stretch": core.JustifyStretch, "normal": 0}

var aligns = map[string]core.Align{"stretch": core.AlignStretch, "flex-start": core.AlignStart,
	"start": core.AlignStart, "self-start": core.AlignStart, "center": core.AlignCenter,
	"flex-end": core.AlignEnd, "end": core.AlignEnd, "self-end": core.AlignEnd,
	"space-between": core.AlignSpaceBetween, "space-around": core.AlignSpaceAround,
	"baseline": core.AlignBaseline, "space-evenly": core.AlignSpaceEvenly, "normal": 0, "auto": 0}

// pick returns the value a keyword names, warning about one it lacks.
func pick[T any](r *runtime, key string, v goja.Value, values map[string]T) T {
	t, ok := values[v.String()]
	if !ok {
		r.warn("style key %q has no value %q", key, v.String())
	}
	return t
}

// emOf reads a length in em, "0.1em", of the element's font size.
func emOf(v goja.Value) (float32, bool) {
	str, ok := v.Export().(string)
	if !ok {
		return 0, false
	}
	n, ok := strings.CutSuffix(strings.TrimSpace(str), "em")
	if !ok || strings.HasSuffix(n, "r") {
		return 0, false
	}
	f, ok := cssNumber(n)
	return float32(f), ok
}

func points(v goja.Value) core.Points {
	if str, ok := v.Export().(string); ok {
		l, _ := cssLength(str)
		return l
	}
	return core.Points(finite(v))
}

// finite is v as a number, 0 for one that is not finite (NaN from a
// sum over a missing value), which a style leaves unset.
func finite(v goja.Value) float64 {
	f := v.ToFloat()
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return f
}

// length decodes a number (points), a length or percentage string, or
// "auto"; min-content and the like are auto.
func (r *runtime) length(key string, v goja.Value) core.Length {
	s, isString := v.Export().(string)
	if !isString {
		return core.Pt(core.Points(finite(v)))
	}
	switch s {
	case "auto", "min-content", "max-content", "fit-content":
		return core.Length{}
	}
	l, ok := cssLengthOrPercent(s)
	r.check(key, v, ok)
	return l
}

func (r *runtime) color(key string, v goja.Value) core.Color {
	c, err := core.ParseColor(v.String())
	if err != nil {
		r.warn("style key %q: %v", key, err)
	}
	return c
}

// fontWeight decodes "normal", "bold" or a weight from 1 to 1000, as a
// number or a string.
func (r *runtime) fontWeight(v goja.Value) core.FontWeight {
	switch s := v.String(); s {
	case "normal":
		return core.FontWeightNormal
	case "bold":
		return core.FontWeightBold
	default:
		if w, err := strconv.Atoi(s); err == nil && w >= 1 && w <= 1000 {
			return core.FontWeight(w)
		}
		r.warn("style key \"fontWeight\" has no value %q", s)
		return 0
	}
}

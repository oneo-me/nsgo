package tsx

import (
	"strconv"
	"strings"
	"time"

	"github.com/dop251/goja"

	"github.com/oneo-me/nsgo/core"
)

// Props and style objects decoded into core's values (NSH's protocol.rs):
// each NSH prop sets the field of core's element that means the same.

// stateProps are the props styling a view in a state (NSH's activeStyle,
// ...), as core's Style fields of the states.
var stateProps = map[string]func(s *core.Style) **core.Style{
	"activeStyle":        func(s *core.Style) **core.Style { return &s.Active },
	"hoverStyle":         func(s *core.Style) **core.Style { return &s.Hover },
	"focusStyle":         func(s *core.Style) **core.Style { return &s.Focus },
	"disabledStyle":      func(s *core.Style) **core.Style { return &s.Disabled },
	"checkedStyle":       func(s *core.Style) **core.Style { return &s.Checked },
	"indeterminateStyle": func(s *core.Style) **core.Style { return &s.Indeterminate },
	"selectedStyle":      func(s *core.Style) **core.Style { return &s.Selected },
}

// setProp sets prop name of n to v and gives n its props again.
func (r *runtime) setProp(n *node, name string, v goja.Value) {
	if n.kind == kindRaw || n.kind == kindRegion {
		panic(r.vm.NewTypeError("nsgo: a text node has no props"))
	}
	switch name {
	case "children":
		return // a host element's children are nodes
	case "ref":
		r.applyRef(v, n, "")
		return
	case "style":
		n.style = r.style(v)
	case "class":
		n.class = stringOf(v)
	case "classList":
		n.classList = r.classList(v)
	default:
		if _, ok := stateProps[name]; ok {
			if n.states == nil {
				n.states = map[string]core.Style{}
			}
			if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
				delete(n.states, name)
			} else {
				n.states[name] = r.style(v)
			}
			break
		}
		var known bool
		switch n.kind {
		case kindView:
			known = r.viewProp(&n.view, name, v)
		case kindText:
			known = r.textProp(&n.text, name, v)
		case kindScroll:
			known = r.scrollProp(&n.scroll, name, v)
		case kindInput:
			known = r.inputProp(&n.input, name, v)
		case kindImage:
			known = r.imageProp(&n.image, name, v)
		}
		if !known {
			r.warn("<%s> has no prop %q yet", kindNames[n.kind], name)
			return
		}
	}
	r.apply(n)
}

var kindNames = map[kind]string{kindView: "View", kindText: "Text", kindScroll: "Scroll", kindInput: "Input",
	kindImage: "Image"}

// apply gives n's element its props.
func (r *runtime) apply(n *node) {
	style := n.style
	for name, st := range n.states {
		st := st
		*stateProps[name](&style) = &st
	}
	switch n.kind {
	case kindView:
		v := n.view
		v.Style, v.Class = style, n.classes()
		n.elem.Set(v)
	case kindText:
		r.textChanged(n)
	case kindScroll:
		v := n.scroll
		v.Style, v.Class = style, n.classes()
		n.elem.Set(v)
	case kindInput:
		v := n.input
		v.Style, v.Class = style, n.classes()
		n.elem.Set(v)
	case kindImage:
		v := n.image
		v.Style, v.Class = style, n.classes()
		n.elem.Set(v)
	}
}

// inputProp sets a prop of an <Input>, reporting whether it has it.
func (r *runtime) inputProp(p *core.Input, name string, v goja.Value) bool {
	switch name {
	case "value":
		if v == nil || goja.IsUndefined(v) {
			p.Value = nil
			break
		}
		s := stringOf(v)
		p.Value = func() string { return s }
	case "defaultValue":
		p.DefaultValue = stringOf(v)
	case "placeholder":
		p.Placeholder = stringOf(v)
	case "masked":
		p.Masked = v.ToBoolean()
	case "multiline":
		p.Multiline = v.ToBoolean()
	case "maxLength":
		p.MaxLength = int(v.ToInteger())
	case "readonly":
		p.Readonly = v.ToBoolean()
	case "autoFocus":
		p.AutoFocus = v.ToBoolean()
	case "disabled":
		p.Disabled = boolFunc(v.ToBoolean())
	case "keyboardType":
		p.KeyboardType = map[string]core.KeyboardType{"default": core.KeyboardDefault, "email": core.KeyboardEmail,
			"email-address": core.KeyboardEmail, "number": core.KeyboardNumber, "numeric": core.KeyboardNumber,
			"number-pad": core.KeyboardNumber, "decimal": core.KeyboardDecimal, "decimal-pad": core.KeyboardDecimal,
			"phone": core.KeyboardPhone, "phone-pad": core.KeyboardPhone, "url": core.KeyboardURL}[stringOf(v)]
	case "returnKeyType":
		p.ReturnKeyType = map[string]core.ReturnKeyType{"default": core.ReturnKeyDefault, "done": core.ReturnKeyDone,
			"go": core.ReturnKeyGo, "next": core.ReturnKeyNext, "search": core.ReturnKeySearch,
			"send": core.ReturnKeySend}[stringOf(v)]
	case "autoCapitalize":
		p.AutoCapitalize = map[string]core.AutoCapitalize{"sentences": core.AutoCapitalizeSentences,
			"none": core.AutoCapitalizeNone, "words": core.AutoCapitalizeWords,
			"characters": core.AutoCapitalizeCharacters}[stringOf(v)]
	case "testID":
		p.TestID = stringOf(v)
	case "accessibilityLabel":
		p.Label = stringOf(v)
	case "onChange":
		p.OnChange = r.textHandler(v)
	case "onSubmit":
		p.OnSubmit = r.textHandler(v)
	case "onFocus":
		p.OnFocus = r.handler(v)
	case "onBlur":
		p.OnBlur = r.handler(v)
	case "onKeyPress":
		p.OnKeyPress = r.textHandler(v)
	case "onLayout":
		p.OnLayout = r.layoutHandler(v)
	default:
		return false
	}
	return true
}

// imageProp sets a prop of an <Image>, reporting whether it has it.
func (r *runtime) imageProp(p *core.Image, name string, v goja.Value) bool {
	switch name {
	case "src":
		p.Source = r.imageSource(v)
	case "placeholder":
		p.Placeholder = r.imageSource(v)
	case "resizeMode":
		p.ResizeMode = map[string]core.ResizeMode{"cover": core.Cover, "contain": core.Contain,
			"stretch": core.Stretch, "center": core.Center}[stringOf(v)]
	case "blurRadius":
		p.BlurRadius = core.Points(v.ToFloat())
	case "testID":
		p.TestID = stringOf(v)
	case "accessibilityLabel", "alt":
		p.Label = stringOf(v)
	case "onPress":
		p.OnPress = r.pressHandler(v)
	case "onLoad":
		if !isFunction(v) {
			p.OnLoad = nil
			break
		}
		p.OnLoad = func(s core.Size) { r.call(v, r.size(s)) }
	case "onError":
		if !isFunction(v) {
			p.OnError = nil
			break
		}
		p.OnError = func(err error) { r.call(v, r.vm.NewGoError(err)) }
	case "onLoadStart":
		p.OnLoadStart = r.handler(v)
	case "onLoadEnd":
		p.OnLoadEnd = r.handler(v)
	case "onLayout":
		p.OnLayout = r.layoutHandler(v)
	default:
		return false
	}
	return true
}

// imageSource reads a URL, or { uri, width, height }.
func (r *runtime) imageSource(v goja.Value) core.ImageSource {
	if o, ok := v.(*goja.Object); ok {
		src := core.ImageSource{URI: stringOf(o.Get("uri"))}
		if w, h := o.Get("width"), o.Get("height"); w != nil && h != nil && !goja.IsUndefined(w) && !goja.IsUndefined(h) {
			src.Size = core.Size{Width: core.Points(w.ToFloat()), Height: core.Points(h.ToFloat())}
		}
		return src
	}
	return core.ImageSource{URI: stringOf(v)}
}

// textHandler wraps a JS handler of a text: onChange, onSubmit.
func (r *runtime) textHandler(v goja.Value) func(string) {
	if !isFunction(v) {
		return nil
	}
	return func(s string) { r.call(v, r.vm.ToValue(s)) }
}

// classList reads `classList`: the classes whose values are truthy.
func (r *runtime) classList(v goja.Value) string {
	o, ok := v.(*goja.Object)
	if !ok {
		return ""
	}
	var b strings.Builder
	for _, k := range o.Keys() {
		if o.Get(k).ToBoolean() {
			b.WriteString(k)
			b.WriteByte(' ')
		}
	}
	return strings.TrimSpace(b.String())
}

// viewProp sets a prop of a <View>, reporting whether it has it.
func (r *runtime) viewProp(p *core.View, name string, v goja.Value) bool {
	switch name {
	case "testID":
		p.TestID = stringOf(v)
	case "accessibilityLabel":
		p.Label = stringOf(v)
	case "accessibilityHint":
		p.Hint = stringOf(v)
	case "accessible":
		p.Accessible = v.ToBoolean()
	case "role":
		p.Role = roles[stringOf(v)]
	case "disabled":
		p.Disabled = boolFunc(v.ToBoolean())
	case "selected":
		p.Selected = boolFunc(v.ToBoolean())
	case "checked":
		p.Checked = boolFunc(v.ToBoolean())
	case "pointerEvents":
		p.PointerEvents = pointerEvents[stringOf(v)]
	case "hitSlop":
		p.HitSlop = r.edges(v)
	case "delayLongPress":
		p.DelayLongPress = time.Duration(v.ToFloat() * float64(time.Millisecond))
	case "onPress":
		p.OnPress = r.pressHandler(v)
	case "onLongPress":
		p.OnLongPress = r.pressHandler(v)
	case "onPressDown":
		p.OnPressDown = r.pressHandler(v)
	case "onPressUp":
		p.OnPressUp = r.pressHandler(v)
	case "onHoverIn":
		p.OnHoverIn = r.handler(v)
	case "onHoverOut":
		p.OnHoverOut = r.handler(v)
	case "onFocus":
		p.OnFocus = r.handler(v)
	case "onBlur":
		p.OnBlur = r.handler(v)
	case "onLayout":
		p.OnLayout = r.layoutHandler(v)
	default:
		return false
	}
	return true
}

// textProp sets a prop of a <Text>, reporting whether it has it.
func (r *runtime) textProp(p *core.Text, name string, v goja.Value) bool {
	switch name {
	case "testID":
		p.TestID = stringOf(v)
	case "accessibilityLabel":
		p.Label = stringOf(v)
	case "accessibilityHint":
		p.Hint = stringOf(v)
	case "role":
		p.Role = roles[stringOf(v)]
	case "numberOfLines":
		p.MaxLines = int(v.ToInteger())
	case "ellipsizeMode":
		p.EllipsizeMode = map[string]core.EllipsizeMode{"head": core.EllipsizeHead, "middle": core.EllipsizeMiddle,
			"tail": core.EllipsizeTail, "clip": core.EllipsizeClip}[stringOf(v)]
	case "selectable":
		p.Selectable = core.SelectableOff
		if v.ToBoolean() {
			p.Selectable = core.SelectableOn
		}
	case "onPress":
		p.OnPress = r.pressHandler(v)
	case "onLayout":
		p.OnLayout = r.layoutHandler(v)
	default:
		return false
	}
	return true
}

// scrollProp sets a prop of a <Scroll>, reporting whether it has it.
func (r *runtime) scrollProp(p *core.Scroll, name string, v goja.Value) bool {
	switch name {
	case "testID":
		p.TestID = stringOf(v)
	case "horizontal":
		p.Horizontal = v.ToBoolean()
	case "scrollbar":
		p.HideIndicator = !v.ToBoolean()
	case "onScroll":
		if !isFunction(v) {
			p.OnScroll = nil
			break
		}
		p.OnScroll = func(e core.ScrollEvent) {
			o := r.vm.NewObject()
			_ = o.Set("contentOffset", r.point(e.Offset))
			_ = o.Set("contentSize", r.size(e.ContentSize))
			_ = o.Set("layoutMeasurement", r.size(e.Size))
			_ = o.Set("zoomScale", float64(e.ZoomScale))
			r.call(v, o)
		}
	case "scrollEventThrottle":
		p.ScrollEventThrottle = time.Duration(v.ToFloat() * float64(time.Millisecond))
	case "onLayout":
		p.OnLayout = r.layoutHandler(v)
	default:
		return false
	}
	return true
}

var roles = map[string]core.Role{"button": core.RoleButton, "link": core.RoleLink, "header": core.RoleHeader,
	"image": core.RoleImage, "text": core.RoleText, "checkbox": core.RoleCheckbox, "switch": core.RoleSwitch,
	"radio": core.RoleRadio, "radiogroup": core.RoleRadioGroup, "tab": core.RoleTab, "tablist": core.RoleTabList,
	"menu": core.RoleMenu, "menuitem": core.RoleMenuItem, "dialog": core.RoleDialog, "alert": core.RoleAlert,
	"adjustable": core.RoleAdjustable, "progressbar": core.RoleProgressBar, "none": core.RoleNone}

var pointerEvents = map[string]core.PointerEvents{"auto": core.PointerEventsAuto, "none": core.PointerEventsNone,
	"box-none": core.PointerEventsBoxNone, "box-only": core.PointerEventsBoxOnly}

// edges reads a number for every side, or { top, right, bottom, left }.
func (r *runtime) edges(v goja.Value) core.Edges {
	if o, ok := v.(*goja.Object); ok {
		side := func(k string) core.Points {
			if s := o.Get(k); s != nil && !goja.IsUndefined(s) {
				return core.Points(s.ToFloat())
			}
			return 0
		}
		return core.Edges{Top: side("top"), Right: side("right"), Bottom: side("bottom"), Left: side("left")}
	}
	p := core.Points(v.ToFloat())
	return core.Edges{Top: p, Right: p, Bottom: p, Left: p}
}

func (r *runtime) size(s core.Size) *goja.Object {
	o := r.vm.NewObject()
	_ = o.Set("width", float64(s.Width))
	_ = o.Set("height", float64(s.Height))
	return o
}

// handler wraps a JS event handler; nil for anything but a function.
func (r *runtime) handler(v goja.Value) func() {
	if !isFunction(v) {
		return nil
	}
	return func() { r.call(v) }
}

func (r *runtime) pressHandler(v goja.Value) func(core.PressEvent) {
	if !isFunction(v) {
		return nil
	}
	return func(e core.PressEvent) {
		o := r.vm.NewObject()
		_ = o.Set("position", r.point(e.Position))
		_ = o.Set("pagePosition", r.point(e.PagePosition))
		m := r.vm.NewObject()
		_ = m.Set("control", e.Modifiers.Control)
		_ = m.Set("alt", e.Modifiers.Alt)
		_ = m.Set("shift", e.Modifiers.Shift)
		_ = m.Set("platform", e.Modifiers.Platform)
		_ = o.Set("modifiers", m)
		_ = o.Set("clickCount", e.Clicks)
		r.call(v, o)
	}
}

func (r *runtime) layoutHandler(v goja.Value) func(core.Bounds) {
	if !isFunction(v) {
		return nil
	}
	return func(b core.Bounds) {
		o := r.vm.NewObject()
		_ = o.Set("origin", r.point(b.Origin))
		size := r.vm.NewObject()
		_ = size.Set("width", float64(b.Size.Width))
		_ = size.Set("height", float64(b.Size.Height))
		_ = o.Set("size", size)
		r.call(v, o)
	}
}

func (r *runtime) point(p core.Point) *goja.Object {
	o := r.vm.NewObject()
	_ = o.Set("x", float64(p.X))
	_ = o.Set("y", float64(p.Y))
	return o
}

func stringOf(v goja.Value) string {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return ""
	}
	return v.String()
}

func boolFunc(b bool) func() bool {
	if !b {
		return nil
	}
	return func() bool { return true }
}

// style decodes a style object, an array of them (later ones winning) or
// nothing.
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
	for pass := range 2 {
		for _, k := range keys {
			if styleShorthands[k] == (pass == 0) {
				r.styleKey(&s, k, o.Get(k))
			}
		}
	}
	return s
}

var styleShorthands = map[string]bool{
	"flex": true, "margin": true, "marginHorizontal": true, "marginVertical": true,
	"padding": true, "paddingHorizontal": true, "paddingVertical": true,
}

// styleKey sets one key of NSH's style objects.
func (r *runtime) styleKey(s *core.Style, key string, v goja.Value) {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return
	}
	switch key {
	case "display":
		s.Display = pick(r, key, v, map[string]core.Display{"flex": core.DisplayFlex, "none": core.DisplayNone,
			"grid": core.DisplayGrid, "block": core.DisplayBlock})
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
		switch f := float32(v.ToFloat()); {
		case f > 0:
			s.FlexGrow, s.FlexShrink, s.FlexBasis = f, 1, core.Pt(0)
		case f < 0:
			s.FlexGrow, s.FlexShrink = 0, -f
		}
	case "flexGrow":
		s.FlexGrow = float32(v.ToFloat())
	case "flexShrink":
		s.FlexShrink = float32(v.ToFloat())
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
		s.AspectRatio = float32(v.ToFloat())
	case "top":
		s.Top = r.length(key, v)
	case "right":
		s.Right = r.length(key, v)
	case "bottom":
		s.Bottom = r.length(key, v)
	case "left":
		s.Left = r.length(key, v)
	case "zIndex":
		s.ZIndex = int(v.ToInteger())
	case "margin":
		s.Margin = core.Edges{Top: points(v), Right: points(v), Bottom: points(v), Left: points(v)}
	case "marginHorizontal":
		s.Margin.Left, s.Margin.Right = points(v), points(v)
	case "marginVertical":
		s.Margin.Top, s.Margin.Bottom = points(v), points(v)
	case "marginTop":
		s.Margin.Top = points(v)
	case "marginRight":
		s.Margin.Right = points(v)
	case "marginBottom":
		s.Margin.Bottom = points(v)
	case "marginLeft":
		s.Margin.Left = points(v)
	case "padding":
		s.Padding = core.Edges{Top: points(v), Right: points(v), Bottom: points(v), Left: points(v)}
	case "paddingHorizontal":
		s.Padding.Left, s.Padding.Right = points(v), points(v)
	case "paddingVertical":
		s.Padding.Top, s.Padding.Bottom = points(v), points(v)
	case "paddingTop":
		s.Padding.Top = points(v)
	case "paddingRight":
		s.Padding.Right = points(v)
	case "paddingBottom":
		s.Padding.Bottom = points(v)
	case "paddingLeft":
		s.Padding.Left = points(v)
	case "gap":
		s.Gap = points(v)
	case "rowGap":
		s.RowGap = points(v)
	case "columnGap":
		s.ColumnGap = points(v)
	case "backgroundColor":
		s.Background = r.color(key, v)
	case "borderRadius":
		s.CornerRadius = points(v)
	case "borderWidth":
		s.BorderWidth = points(v)
	case "borderColor":
		s.BorderColor = r.color(key, v)
	case "opacity":
		s.Opacity = core.Alpha(float32(v.ToFloat()))
	case "overflow":
		s.Overflow = pick(r, key, v, map[string]core.Overflow{"visible": core.Visible, "hidden": core.Hidden,
			"scroll": core.OverflowScroll})
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
		s.LineHeight = points(v)
	case "letterSpacing":
		s.LetterSpacing = points(v)
	case "textAlign":
		s.TextAlign = pick(r, key, v, map[string]core.TextAlign{"left": core.TextAlignStart, "auto": core.TextAlignStart,
			"center": core.TextAlignCenter, "right": core.TextAlignEnd, "justify": core.TextAlignJustify})
	case "fontVariant":
		s.FontVariant = core.FontVariantNormal
		var names []string
		if err := r.vm.ExportTo(v, &names); err != nil {
			names = strings.Fields(v.String())
		}
		for _, name := range names {
			s.FontVariant |= pick(r, key, r.vm.ToValue(name), fontVariants)
		}
	case "textDecorationThickness":
		if str, ok := v.Export().(string); ok && (str == "auto" || str == "from-font") {
			s.TextDecorationThickness = core.Length{}
			break
		}
		s.TextDecorationThickness = r.length(key, v)
	case "textUnderlineOffset":
		s.TextUnderlineOffset = r.length(key, v)
	default:
		r.warn("style key %q is not supported yet", key)
	}
}

var fontVariants = map[string]core.FontVariant{"small-caps": core.FontSmallCaps,
	"oldstyle-nums": core.FontOldstyleNums, "lining-nums": core.FontLiningNums,
	"tabular-nums": core.FontTabularNums, "proportional-nums": core.FontProportionalNums}

var justifies = map[string]core.Justify{"flex-start": core.JustifyStart, "start": core.JustifyStart,
	"center": core.JustifyCenter, "flex-end": core.JustifyEnd, "end": core.JustifyEnd,
	"space-between": core.JustifySpaceBetween, "space-around": core.JustifySpaceAround,
	"space-evenly": core.JustifySpaceEvenly, "stretch": core.JustifyStretch}

var aligns = map[string]core.Align{"stretch": core.AlignStretch, "flex-start": core.AlignStart,
	"start": core.AlignStart, "center": core.AlignCenter, "flex-end": core.AlignEnd, "end": core.AlignEnd,
	"space-between": core.AlignSpaceBetween, "space-around": core.AlignSpaceAround,
	"baseline": core.AlignBaseline, "space-evenly": core.AlignSpaceEvenly}

// pick returns the value a keyword names, warning about one it lacks.
func pick[T any](r *runtime, key string, v goja.Value, values map[string]T) T {
	t, ok := values[v.String()]
	if !ok {
		r.warn("style key %q has no value %q", key, v.String())
	}
	return t
}

func points(v goja.Value) core.Points { return core.Points(v.ToFloat()) }

// length decodes a number (points), a percentage string ("50%") or
// "auto".
func (r *runtime) length(key string, v goja.Value) core.Length {
	s, isString := v.Export().(string)
	if !isString {
		return core.Pt(points(v))
	}
	if s == "auto" {
		return core.Length{}
	}
	if p, ok := strings.CutSuffix(s, "%"); ok {
		if f, err := strconv.ParseFloat(p, 32); err == nil {
			return core.Pct(float32(f))
		}
	}
	r.warn("style key %q has no length %q", key, s)
	return core.Length{}
}

func (r *runtime) color(key string, v goja.Value) core.Color {
	c, err := core.ParseColor(v.String())
	if err != nil {
		r.warn("style key %q: %v", key, err)
	}
	return c
}

// fontWeight decodes "normal", "bold" or a weight from 100 to 900, as a
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

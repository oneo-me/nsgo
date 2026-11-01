package tsx

import (
	"github.com/dop251/goja"

	"github.com/oneo-me/nsgo/core"
)

// The host elements' props (NSH's HostProps and each element's own):
// each sets the field of core's element that means the same. A prop one
// of core's elements lacks is logged once.

// viewProp sets a prop of a <View>, reporting whether it has it.
func (r *runtime) viewProp(p *core.View, name string, v goja.Value) bool {
	if touchProps[name] {
		return true // made with the others as the props apply (touches)
	}
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
	case "accessibilityState":
		o := r.options(v)
		p.Disabled, p.Selected = boolFunc(o.bool("disabled", false)), boolFunc(o.bool("selected", false))
		p.Expanded = nil
		if o.has("expanded") {
			e := o.bool("expanded", false)
			p.Expanded = func() bool { return e }
		}
		switch c := o.get("checked"); {
		case !defined(c):
			p.Checked, p.Indeterminate = nil, nil
		case c.String() == "mixed":
			p.Checked, p.Indeterminate = nil, boolFunc(true)
		default:
			p.Checked, p.Indeterminate = boolFunc(c.ToBoolean()), nil
		}
	case "accessibilityValue":
		o := r.options(v)
		av := core.AccessibilityValue{Min: float32(o.float("min", 0)), Max: float32(o.float("max", 0)),
			Now: float32(o.float("now", 0)), Text: o.str("text", "")}
		p.AccessibilityValue = func() core.AccessibilityValue { return av }
	case "accessibilityElementsHidden":
		p.AccessibilityElementsHidden = boolFunc(v.ToBoolean())
	case "importantForAccessibility":
		p.ImportantForAccessibility = map[string]core.Importance{"auto": core.ImportantAuto, "yes": core.ImportantYes,
			"no": core.ImportantNo, "no-hide-descendants": core.ImportantNoHideDescendants}[stringOf(v)]
	case "accessibilityActions":
		p.AccessibilityActions = nil
		for _, it := range r.arrayItems(v) {
			o := r.options(it)
			p.AccessibilityActions = append(p.AccessibilityActions,
				core.AccessibilityAction{Name: o.str("name", ""), Label: o.str("label", "")})
		}
	case "onAccessibilityAction":
		p.OnAccessibilityAction = nil
		if isFunction(v) {
			p.OnAccessibilityAction = func(action string) {
				e := r.vm.NewObject()
				_ = e.Set("actionName", action)
				r.call(v, e)
			}
		}
	case "accessibilityLiveRegion":
		p.LiveRegion = liveRegions[stringOf(v)]
	case "tooltip":
		p.Tooltip = r.region(func() goja.Value { return v })
	case "focusable":
		p.NoFocus = !v.ToBoolean()
	case "disabled":
		p.Disabled = boolFunc(v.ToBoolean())
	case "selected":
		p.Selected = boolFunc(v.ToBoolean())
	case "checked":
		p.Checked = boolFunc(v.ToBoolean())
	case "selectable":
		p.Selectable = v.ToBoolean()
	case "pointerEvents":
		p.PointerEvents = pointerEvents[stringOf(v)]
	case "hitSlop":
		p.HitSlop = r.edges(v)
	case "delayLongPress":
		p.DelayLongPress = millis(v.ToFloat())
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
	case "onHoverMove":
		p.OnHoverMove = nil
		if isFunction(v) {
			p.OnHoverMove = func(e core.HoverEvent) { r.call(v, r.pointerEvent(e.Position, e.PagePosition, e.Modifiers)) }
		}
	case "onContextMenu":
		p.OnContextMenu = nil
		if isFunction(v) {
			p.OnContextMenu = func(e core.ContextMenuEvent) {
				r.call(v, r.pointerEvent(e.Position, e.PagePosition, e.Modifiers))
			}
		}
	case "onMouseMove":
		p.OnMouseMove = nil
		if isFunction(v) {
			p.OnMouseMove = func(at core.Point) { r.call(v, r.point(at)) }
		}
	case "onFileDragEnter":
		p.OnFileDragEnter = r.handler(v)
	case "onFileDragLeave":
		p.OnFileDragLeave = r.handler(v)
	case "onFileDrop":
		p.OnFileDrop = nil
		if isFunction(v) {
			p.OnFileDrop = func(uris []string) {
				e := r.vm.NewObject()
				_ = e.Set("uris", uris)
				r.call(v, e)
			}
		}
	case "onFocus":
		p.OnFocus = r.handler(v)
	case "onBlur":
		p.OnBlur = r.handler(v)
	case "onLayout":
		p.OnLayout = r.layoutHandler(v)
	case "pan":
		o := r.options(v)
		p.PanValues = core.AnimatedXY{X: r.animatedValueOf(o.get("x")), Y: r.animatedValueOf(o.get("y"))}
	case "pinch":
		p.PinchValue = r.animatedValueOf(r.options(v).get("scale"))
	case "rotation":
		p.RotationValue = r.animatedValueOf(r.options(v).get("angle"))
	case "panActivation":
		o := r.options(v)
		p.PanActivation = core.PanActivation{
			ActiveOffsetX: core.Points(o.float("activeOffsetX", 0)), ActiveOffsetY: core.Points(o.float("activeOffsetY", 0)),
			FailOffsetX: core.Points(o.float("failOffsetX", 0)), FailOffsetY: core.Points(o.float("failOffsetY", 0))}
	case "onPanStart", "onPanEnd", "onPinchStart", "onPinchEnd", "onRotationStart", "onRotationEnd":
		return true // made with the others as the props apply (gestures)
	case "backdropMaterial":
		p.IOS.BackdropMaterial = map[string]core.BackdropMaterial{"ultraThin": core.BackdropUltraThin,
			"thin": core.BackdropThin, "regular": core.BackdropRegular, "thick": core.BackdropThick,
			"chrome": core.BackdropChrome}[stringOf(v)]
	case "ripple":
		p.Android.Ripple = nil
		if o := r.options(v); o.o != nil {
			p.Android.Ripple = &core.Ripple{Borderless: o.bool("borderless", false),
				Radius: core.Points(o.float("radius", 0)), Foreground: o.bool("foreground", false)}
			if c := o.get("color"); defined(c) {
				p.Android.Ripple.Color = r.color("ripple", c)
			}
		}
	case "windowDrag":
		p.Desktop.WindowDrag = core.WindowDragOff
		if v.ToBoolean() {
			p.Desktop.WindowDrag = core.WindowDragOn
		}
	default:
		return false
	}
	return true
}

// gestures makes v's OnPan, OnPinch and OnRotation from n's gesture
// props: a gesture's start and end, the moves driving animated values.
func (r *runtime) gestures(n *node, v *core.View) {
	get := func(name string) goja.Value {
		if h := n.values[name]; isFunction(h) {
			return h
		}
		return nil
	}
	if start, end := get("onPanStart"), get("onPanEnd"); start != nil || end != nil {
		v.OnPan = func(e core.PanEvent) {
			h := end
			switch e.Phase {
			case core.GestureBegan:
				h = start
			case core.GestureChanged:
				return
			}
			if h == nil {
				return
			}
			o := r.vm.NewObject()
			_ = o.Set("cancelled", e.Phase == core.GestureCancelled)
			_ = o.Set("translationX", float64(e.Translation.X))
			_ = o.Set("translationY", float64(e.Translation.Y))
			_ = o.Set("velocityX", float64(e.Velocity.X))
			_ = o.Set("velocityY", float64(e.Velocity.Y))
			if v.PanValues.X != nil {
				_ = o.Set("valueX", float64(e.Values.X))
			}
			if v.PanValues.Y != nil {
				_ = o.Set("valueY", float64(e.Values.Y))
			}
			r.call(h, o)
		}
	}
	gesture := func(start, end, key string) func(core.GestureEvent) {
		s, en := get(start), get(end)
		if s == nil && en == nil {
			return nil
		}
		return func(e core.GestureEvent) {
			h := en
			switch e.Phase {
			case core.GestureBegan:
				h = s
			case core.GestureChanged:
				return
			}
			if h == nil {
				return
			}
			o := r.vm.NewObject()
			_ = o.Set(key, float64(e.Value))
			_ = o.Set("velocity", float64(e.Velocity))
			r.call(h, o)
		}
	}
	v.OnPinch = gesture("onPinchStart", "onPinchEnd", "scale")
	v.OnRotation = gesture("onRotationStart", "onRotationEnd", "rotation")
}

var liveRegions = map[string]core.LiveRegion{"none": core.LiveRegionNone, "polite": core.LiveRegionPolite,
	"assertive": core.LiveRegionAssertive}

// textProp sets a prop of a <Text>, reporting whether it has it.
func (r *runtime) textProp(p *core.Text, name string, v goja.Value) bool {
	switch name {
	case "testID":
		p.TestID = stringOf(v)
	case "accessibilityLabel":
		p.Label = stringOf(v)
	case "accessibilityHint":
		p.Hint = stringOf(v)
	case "accessibilityLiveRegion":
		p.LiveRegion = liveRegions[stringOf(v)]
	case "role":
		p.Role = roles[stringOf(v)]
	case "numberOfLines":
		p.MaxLines = int(v.ToInteger())
	case "ellipsizeMode":
		p.EllipsizeMode = map[string]core.EllipsizeMode{"head": core.EllipsizeHead, "middle": core.EllipsizeMiddle,
			"tail": core.EllipsizeTail, "clip": core.EllipsizeClip}[stringOf(v)]
	case "allowFontScaling":
		p.NoFontScaling = !v.ToBoolean()
	case "maxFontSizeMultiplier":
		p.MaxFontSizeMultiplier = float32(v.ToFloat())
	case "adjustsFontSizeToFit":
		p.AdjustsFontSizeToFit = v.ToBoolean()
	case "minimumFontScale":
		p.MinimumFontScale = float32(v.ToFloat())
	case "selectable":
		p.Selectable = core.SelectableOff
		if v.ToBoolean() {
			p.Selectable = core.SelectableOn
		}
	case "onPress":
		p.OnPress = r.pressHandler(v)
	case "onLongPress":
		p.OnLongPress = r.pressHandler(v)
	case "onLayout":
		p.OnLayout = r.layoutHandler(v)
	case "onTextLayout":
		p.OnTextLayout = nil
		if isFunction(v) {
			p.OnTextLayout = func(lines []core.TextLine) {
				out := make([]any, len(lines))
				for i, l := range lines {
					o := r.vm.NewObject()
					_ = o.Set("text", l.Text)
					_ = o.Set("bounds", r.bounds(l.Bounds))
					_ = o.Set("ascender", float64(l.Ascender))
					_ = o.Set("descender", float64(l.Descender))
					_ = o.Set("capHeight", float64(l.CapHeight))
					_ = o.Set("xHeight", float64(l.XHeight))
					out[i] = o
				}
				e := r.vm.NewObject()
				_ = e.Set("lines", out)
				r.call(v, e)
			}
		}
	default:
		return false
	}
	return true
}

// scrollProp sets a prop of a <Scroll>, reporting whether it has it.
func (r *runtime) scrollProp(p *core.Scroll, name string, v goja.Value) bool {
	if r.behaviorProp(&p.Behavior, name, v) {
		return true
	}
	switch name {
	case "testID":
		p.TestID = stringOf(v)
	case "horizontal":
		p.Horizontal = v.ToBoolean()
	case "scrollbar":
		p.HideIndicator = !v.ToBoolean()
	case "contentOffset":
		p.ContentOffset = r.pointOf(v.ToObject(r.vm))
	case "minimumZoomScale":
		p.MinZoomScale = float32(v.ToFloat())
	case "maximumZoomScale":
		p.MaxZoomScale = float32(v.ToFloat())
	case "zoomScale":
		z := float32(v.ToFloat())
		p.ZoomScaleFunc = func() float32 { return z }
	case "bouncesZoom":
		p.NoBouncesZoom = !v.ToBoolean()
	case "zoomable":
		p.NoPinchZoom = !v.ToBoolean()
	case "centerContent":
		p.CenterContent = v.ToBoolean()
	case "refreshing":
		p.Refreshing = boolFunc(v.ToBoolean())
		if p.Refreshing == nil {
			p.Refreshing = func() bool { return false }
		}
	case "onRefresh":
		p.OnRefresh = r.handler(v)
	case "pullValue":
		p.PullValue = r.animatedValueOf(v)
	case "scrollValue":
		p.OffsetValues = core.AnimatedXY{}
		if p.Horizontal {
			p.OffsetValues.X = r.animatedValueOf(v)
		} else {
			p.OffsetValues.Y = r.animatedValueOf(v)
		}
	case "onScroll":
		p.OnScroll = r.scrollEventHandler(v)
	case "scrollEventThrottle":
		p.ScrollEventThrottle = millis(v.ToFloat())
	case "onScrollDragStart":
		p.OnScrollDragStart = r.scrollHandler(r.caller(v))
	case "onScrollDragEnd":
		p.OnScrollDragEnd = r.scrollHandler(r.caller(v))
	case "onMomentumStart":
		p.OnMomentumStart = r.scrollHandler(r.caller(v))
	case "onMomentumEnd":
		p.OnMomentumEnd = r.scrollHandler(r.caller(v))
	case "onScrollEnd":
		p.OnScrollEnd = r.scrollHandler(r.caller(v))
	case "onLayout":
		p.OnLayout = r.layoutHandler(v)
	default:
		return false
	}
	return true
}

// behaviorProp sets a prop of how a scroll view scrolls (NSH's
// ScrollingHostProps), reporting whether it is one.
func (r *runtime) behaviorProp(b *core.ScrollBehavior, name string, v goja.Value) bool {
	switch name {
	case "pagingEnabled":
		b.Paging = v.ToBoolean()
	case "snapToInterval":
		b.SnapToInterval = points(v)
	case "snapToOffsets":
		b.SnapToOffsets = nil
		for _, it := range r.arrayItems(v) {
			b.SnapToOffsets = append(b.SnapToOffsets, points(it))
		}
	case "snapToAlignment":
		b.SnapToAlignment = map[string]core.SnapAlignment{"start": core.SnapStart, "center": core.SnapCenter,
			"end": core.SnapEnd}[stringOf(v)]
	case "decelerationRate":
		switch stringOf(v) {
		case "normal":
			b.DecelerationRate = 0.998
		case "fast":
			b.DecelerationRate = 0.99
		default:
			b.DecelerationRate = float32(v.ToFloat())
		}
	case "bounces":
		b.NoBounces = !v.ToBoolean()
	case "contentInset":
		b.ContentInset = r.edges(v)
	case "keyboardShouldPersistTaps":
		switch s := stringOf(v); s {
		case "always", "true":
			b.KeyboardShouldPersistTaps = core.KeyboardTapsAlways
		case "handled":
			b.KeyboardShouldPersistTaps = core.KeyboardTapsHandled
		default:
			b.KeyboardShouldPersistTaps = core.KeyboardTapsNever
		}
	case "keyboardDismissMode":
		b.KeyboardDismissMode = map[string]core.KeyboardDismissMode{"none": core.KeyboardDismissNone,
			"on-drag": core.KeyboardDismissOnDrag, "interactive": core.KeyboardDismissInteractive}[stringOf(v)]
	case "disableIntervalMomentum":
		b.DisableIntervalMomentum = v.ToBoolean()
	case "refreshIndicator":
		b.RefreshIndicator = core.Points(r.options(v).float("height", 0))
	case "scrollable":
		b.NoScroll = !v.ToBoolean()
	default:
		return false
	}
	return true
}

// scrollBehavior reads the props of how a scroll view scrolls.
func (r *runtime) scrollBehavior(get func(string) goja.Value) core.ScrollBehavior {
	var b core.ScrollBehavior
	for _, name := range []string{"pagingEnabled", "snapToInterval", "snapToOffsets", "snapToAlignment",
		"decelerationRate", "bounces", "contentInset", "keyboardShouldPersistTaps", "keyboardDismissMode",
		"disableIntervalMomentum", "refreshIndicator", "scrollable"} {
		if v := get(name); defined(v) {
			r.behaviorProp(&b, name, v)
		}
	}
	return b
}

// caller calls v when it is a function; nil otherwise.
func (r *runtime) caller(v goja.Value) func(args ...goja.Value) goja.Value {
	if !isFunction(v) {
		return nil
	}
	return func(args ...goja.Value) goja.Value { return r.call(v, args...) }
}

// scrollHandler gives h the content offset of scroll events.
func (r *runtime) scrollHandler(h func(args ...goja.Value) goja.Value) func(core.ScrollEvent) {
	if h == nil {
		return nil
	}
	return func(e core.ScrollEvent) { h(r.point(e.Offset)) }
}

// scrollEventHandler gives v whole scroll events.
func (r *runtime) scrollEventHandler(v goja.Value) func(core.ScrollEvent) {
	if !isFunction(v) {
		return nil
	}
	return func(e core.ScrollEvent) { r.call(v, r.scrollEvent(e)) }
}

func (r *runtime) scrollEvent(e core.ScrollEvent) *goja.Object {
	o := r.vm.NewObject()
	_ = o.Set("offset", r.point(e.Offset))
	_ = o.Set("contentSize", r.size(e.ContentSize))
	_ = o.Set("size", r.size(e.Size))
	_ = o.Set("zoomScale", float64(e.ZoomScale))
	return o
}

var keyboardTypes = map[string]core.KeyboardType{"default": core.KeyboardDefault, "email": core.KeyboardEmail,
	"email-address": core.KeyboardEmail, "number": core.KeyboardNumber, "numeric": core.KeyboardNumber,
	"number-pad": core.KeyboardNumber, "numbers-and-punctuation": core.KeyboardNumber,
	"decimal": core.KeyboardDecimal, "decimal-pad": core.KeyboardDecimal, "phone": core.KeyboardPhone,
	"phone-pad": core.KeyboardPhone, "url": core.KeyboardURL}

var autoCompletes = map[string]core.AutoComplete{"off": core.AutoCompleteOff, "none": core.AutoCompleteOff,
	"username": core.AutoCompleteUsername, "password": core.AutoCompletePassword,
	"current-password": core.AutoCompletePassword, "new-password": core.AutoCompleteNewPassword,
	"newPassword": core.AutoCompleteNewPassword, "one-time-code": core.AutoCompleteOneTimeCode,
	"oneTimeCode": core.AutoCompleteOneTimeCode, "sms-otp": core.AutoCompleteOneTimeCode,
	"email": core.AutoCompleteEmail, "emailAddress": core.AutoCompleteEmail, "tel": core.AutoCompleteTel,
	"telephoneNumber": core.AutoCompleteTel, "name": core.AutoCompleteName, "given-name": core.AutoCompleteGivenName,
	"givenName": core.AutoCompleteGivenName, "family-name": core.AutoCompleteFamilyName,
	"familyName": core.AutoCompleteFamilyName, "additional-name": core.AutoCompleteMiddleName,
	"middleName": core.AutoCompleteMiddleName, "nickname": core.AutoCompleteNickname,
	"organization": core.AutoCompleteOrganization, "organizationName": core.AutoCompleteOrganization,
	"street-address": core.AutoCompleteStreetAddress, "fullStreetAddress": core.AutoCompleteStreetAddress,
	"address-line1": core.AutoCompleteAddressLine1, "streetAddressLine1": core.AutoCompleteAddressLine1,
	"address-line2": core.AutoCompleteAddressLine2, "streetAddressLine2": core.AutoCompleteAddressLine2,
	"address-level1": core.AutoCompleteRegion, "addressState": core.AutoCompleteRegion,
	"address-level2": core.AutoCompleteCity, "addressCity": core.AutoCompleteCity,
	"postal-code": core.AutoCompletePostalCode, "postalCode": core.AutoCompletePostalCode,
	"country": core.AutoCompleteCountry, "countryName": core.AutoCompleteCountry,
	"cc-number": core.AutoCompleteCreditCardNumber, "creditCardNumber": core.AutoCompleteCreditCardNumber,
	"cc-exp": core.AutoCompleteCreditCardExpiration, "creditCardExpiration": core.AutoCompleteCreditCardExpiration,
	"cc-csc": core.AutoCompleteCreditCardSecurityCode, "creditCardSecurityCode": core.AutoCompleteCreditCardSecurityCode,
	"cc-name": core.AutoCompleteCreditCardName, "creditCardName": core.AutoCompleteCreditCardName,
	"url": core.AutoCompleteURL, "URL": core.AutoCompleteURL, "birthdate-full": core.AutoCompleteBirthdate,
	"birthdate": core.AutoCompleteBirthdate}

// inputProp sets a prop of an <Input>, reporting whether it has it.
func (r *runtime) inputProp(p *core.Input, name string, v goja.Value) bool {
	switch name {
	case "value":
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
		p.KeyboardType = keyboardTypes[stringOf(v)]
	case "returnKeyType":
		p.ReturnKeyType = map[string]core.ReturnKeyType{"default": core.ReturnKeyDefault, "done": core.ReturnKeyDone,
			"go": core.ReturnKeyGo, "next": core.ReturnKeyNext, "search": core.ReturnKeySearch,
			"send": core.ReturnKeySend}[stringOf(v)]
	case "autoCapitalize":
		p.AutoCapitalize = map[string]core.AutoCapitalize{"sentences": core.AutoCapitalizeSentences,
			"none": core.AutoCapitalizeNone, "words": core.AutoCapitalizeWords,
			"characters": core.AutoCapitalizeCharacters}[stringOf(v)]
	case "autoCorrect":
		p.NoAutoCorrect = !v.ToBoolean()
	case "submitBehavior":
		p.SubmitBehavior = map[string]core.SubmitBehavior{"submit": core.SubmitBehaviorSubmit,
			"blurAndSubmit": core.SubmitBehaviorBlurAndSubmit, "newline": core.SubmitBehaviorNewline}[stringOf(v)]
	case "pasteBehavior":
		p.PasteBehavior = map[string]core.PasteBehavior{"insert": core.PasteInsert, "report": core.PasteReport}[stringOf(v)]
	case "caret":
		p.HideCaret = !v.ToBoolean()
	case "autoComplete", "textContentType":
		p.AutoComplete = autoCompletes[stringOf(v)]
	case "selectedRange":
		o := r.options(v)
		start := int(o.float("start", 0))
		tr := core.TextRange{Start: start, End: int(o.float("end", float64(start)))}
		p.SelectedRange = func() core.TextRange { return tr }
	case "keyboardAppearance":
		p.IOS.KeyboardAppearance = map[string]core.KeyboardAppearance{"default": core.KeyboardAppearanceDefault,
			"light": core.KeyboardAppearanceLight, "dark": core.KeyboardAppearanceDark}[stringOf(v)]
	case "inputAccessoryViewID":
		p.InputAccessoryViewID = stringOf(v)
	case "testID":
		p.TestID = stringOf(v)
	case "accessibilityLabel":
		p.Label = stringOf(v)
	case "onChange":
		p.OnChange = r.textHandler(v)
	case "onSubmit":
		p.OnSubmit = nil
		if isFunction(v) {
			p.OnSubmit = func(text string) {
				e := r.vm.NewObject()
				_ = e.Set("text", text)
				r.call(v, e)
			}
		}
	case "onFocus":
		p.OnFocus = r.handler(v)
	case "onBlur":
		p.OnBlur = r.handler(v)
	case "onKeyPress":
		p.OnKeyPress = nil
		if isFunction(v) {
			p.OnKeyPress = func(key string) {
				e := r.vm.NewObject()
				_ = e.Set("key", key)
				r.call(v, e)
			}
		}
	case "onSelectionChange":
		p.OnSelectionChange = nil
		if isFunction(v) {
			p.OnSelectionChange = func(tr core.TextRange) {
				e := r.vm.NewObject()
				_ = e.Set("start", tr.Start)
				_ = e.Set("end", tr.End)
				r.call(v, e)
			}
		}
	case "onCompositionChange":
		p.OnCompositionChange = nil
		if isFunction(v) {
			p.OnCompositionChange = func(c core.Composition) {
				e := r.vm.NewObject()
				_ = e.Set("composing", c.Composing)
				_ = e.Set("text", c.Text)
				r.call(v, e)
			}
		}
	case "onContentSizeChange":
		p.OnContentSizeChange = nil
		if isFunction(v) {
			p.OnContentSizeChange = func(s core.Size) { r.call(v, r.size(s)) }
		}
	case "onCaretMove":
		p.OnCaretMove = nil
		if isFunction(v) {
			p.OnCaretMove = func(c core.CaretEvent) {
				e := r.vm.NewObject()
				_ = e.Set("bounds", r.bounds(c.Bounds))
				_ = e.Set("pageBounds", r.bounds(c.PageBounds))
				r.call(v, e)
			}
		}
	case "onPaste":
		p.OnPaste = nil
		if isFunction(v) {
			p.OnPaste = func(pe core.PasteEvent) {
				e := r.vm.NewObject()
				_ = e.Set("text", pe.Text)
				_ = e.Set("uris", pe.URIs)
				files := make([]any, len(pe.Files))
				for i, f := range pe.Files {
					files[i] = r.pastedFile(f)
				}
				_ = e.Set("files", files)
				r.call(v, e)
			}
		}
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
		p.ResizeMode = resizeModes[stringOf(v)]
	case "blurRadius":
		p.BlurRadius = core.Points(v.ToFloat())
	case "fadeDuration":
		p.FadeDuration = millis(v.ToFloat())
	case "testID":
		p.TestID = stringOf(v)
	case "accessibilityLabel", "alt":
		p.Label = stringOf(v)
	case "onPress":
		p.OnPress = r.pressHandler(v)
	case "onLoad":
		p.OnLoad = nil
		if isFunction(v) {
			p.OnLoad = func(s core.Size) { r.call(v, r.size(s)) }
		}
	case "onError":
		p.OnError = nil
		if isFunction(v) {
			p.OnError = func(err error) {
				e := r.vm.NewObject()
				_ = e.Set("message", err.Error())
				r.call(v, e)
			}
		}
	case "onProgress":
		p.OnProgress = nil
		if isFunction(v) {
			p.OnProgress = func(loaded, total int64) {
				e := r.vm.NewObject()
				_ = e.Set("loaded", loaded)
				_ = e.Set("total", total)
				r.call(v, e)
			}
		}
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

// imageSource reads a URL, an ImageAsset ({ url, size, scales }) or
// { uri, width, height }.
func (r *runtime) imageSource(v goja.Value) core.ImageSource {
	o, ok := v.(*goja.Object)
	if !ok {
		return core.ImageSource{URI: stringOf(v)}
	}
	op := opts{o}
	src := core.ImageSource{URI: op.str("url", op.str("uri", ""))}
	if s, ok := o.Get("size").(*goja.Object); ok {
		src.Size = r.sizeOf(s)
	} else if op.has("width") && op.has("height") {
		src.Size = core.Size{Width: core.Points(op.float("width", 0)), Height: core.Points(op.float("height", 0))}
	}
	if scales := r.arrayItems(o.Get("scales")); len(scales) > 0 {
		fs := make([]float32, len(scales))
		for i, s := range scales {
			fs[i] = float32(s.ToFloat())
		}
		src.Scales = core.ScalesOf(fs...)
	}
	return src
}

// textHandler wraps a JS handler of a text: onChange.
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
	var out []byte
	for _, k := range o.Keys() {
		if o.Get(k).ToBoolean() {
			if len(out) > 0 {
				out = append(out, ' ')
			}
			out = append(out, k...)
		}
	}
	return string(out)
}

var roles = map[string]core.Role{"button": core.RoleButton, "imagebutton": core.RoleButton,
	"keyboardkey": core.RoleButton, "link": core.RoleLink, "header": core.RoleHeader, "image": core.RoleImage,
	"text": core.RoleText, "summary": core.RoleText, "search": core.RoleTextField, "combobox": core.RoleButton,
	"checkbox": core.RoleCheckbox, "switch": core.RoleSwitch, "radio": core.RoleRadio,
	"radiogroup": core.RoleRadioGroup, "tab": core.RoleTab, "tablist": core.RoleTabList, "menu": core.RoleMenu,
	"menubar": core.RoleMenu, "menuitem": core.RoleMenuItem, "dialog": core.RoleDialog, "alert": core.RoleAlert,
	"adjustable": core.RoleAdjustable, "spinbutton": core.RoleAdjustable, "scrollbar": core.RoleAdjustable,
	"progressbar": core.RoleProgressBar, "timer": core.RoleText, "toolbar": core.RoleNone, "list": core.RoleNone,
	"grid": core.RoleGrid, "table": core.RoleTable, "row": core.RoleRow, "cell": core.RoleCell,
	"columnheader": core.RoleColumnHeader, "rowgroup": core.RoleRowGroup, "none": core.RoleNone}

var pointerEvents = map[string]core.PointerEvents{"auto": core.PointerEventsAuto, "none": core.PointerEventsNone,
	"box-none": core.PointerEventsBoxNone, "box-only": core.PointerEventsBoxOnly}

// edges reads a number for every side, or { top, right, bottom, left }.
func (r *runtime) edges(v goja.Value) core.Edges {
	if o, ok := v.(*goja.Object); ok {
		op := opts{o}
		return core.Edges{Top: core.Points(op.float("top", 0)), Right: core.Points(op.float("right", 0)),
			Bottom: core.Points(op.float("bottom", 0)), Left: core.Points(op.float("left", 0))}
	}
	return core.All(core.Points(v.ToFloat()))
}

func (r *runtime) size(s core.Size) *goja.Object {
	o := r.vm.NewObject()
	_ = o.Set("width", float64(s.Width))
	_ = o.Set("height", float64(s.Height))
	return o
}

func (r *runtime) point(p core.Point) *goja.Object {
	o := r.vm.NewObject()
	_ = o.Set("x", float64(p.X))
	_ = o.Set("y", float64(p.Y))
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
	return func(e core.PressEvent) { r.call(v, r.pressEvent(e)) }
}

// pressEvent is { position, pagePosition, modifiers, clickCount }.
func (r *runtime) pressEvent(e core.PressEvent) *goja.Object {
	o := r.pointerEvent(e.Position, e.PagePosition, e.Modifiers)
	_ = o.Set("clickCount", e.Clicks)
	return o
}

// pointerEvent is { position, pagePosition, modifiers }.
func (r *runtime) pointerEvent(at, page core.Point, mods core.Modifiers) *goja.Object {
	o := r.vm.NewObject()
	_ = o.Set("position", r.point(at))
	_ = o.Set("pagePosition", r.point(page))
	m := r.vm.NewObject()
	_ = m.Set("control", mods.Control)
	_ = m.Set("alt", mods.Alt)
	_ = m.Set("shift", mods.Shift)
	_ = m.Set("platform", mods.Platform)
	_ = o.Set("modifiers", m)
	return o
}

func (r *runtime) layoutHandler(v goja.Value) func(core.Bounds) {
	if !isFunction(v) {
		return nil
	}
	return func(b core.Bounds) { r.call(v, r.bounds(b)) }
}

func stringOf(v goja.Value) string {
	if !defined(v) {
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

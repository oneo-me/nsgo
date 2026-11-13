package tsx

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/tsx/internal/js"

	"github.com/nsgo-dev/nsgo/core"
)

// setProps sets each prop of the JS object js on a new T through set,
// failing on one it does not have.
func setProps[T any](t *testing.T, r *runtime, set func(*runtime, *T, string, js.Value) bool, src string) *T {
	t.Helper()
	p := new(T)
	o := jsValue(t, r, src).ToObject(r.vm)
	for _, name := range o.Keys() {
		if !set(r, p, name, o.Get(name)) {
			t.Fatalf("%s: no prop %q", src, name)
		}
	}
	return p
}

func TestViewPropsSetTheirFields(t *testing.T) {
	cases := map[string]func(*core.View) bool{
		`{testID: "a", accessibilityLabel: "b", accessibilityHint: "c", accessible: true, role: "button"}`: func(p *core.View) bool {
			return p.TestID == "a" && p.Label == "b" && p.Hint == "c" && p.Accessible && p.Role == core.RoleButton
		},
		`{accessibilityState: {disabled: true, selected: true, expanded: false, pressed: "true", busy: true, checked: true}}`: func(p *core.View) bool {
			return p.Disabled() && p.Selected() && !p.Expanded() && p.Pressed() && p.Busy() && p.Checked() && p.Indeterminate == nil
		},
		`{accessibilityDescription: "d", accessibilityState: {invalid: true}, role: "alertdialog"}`: func(p *core.View) bool {
			return p.AccessibilityDescription == "d" && p.Invalid() && p.Role == core.RoleAlertDialog
		},
		`{accessibilityState: {checked: "mixed"}}`: func(p *core.View) bool {
			return p.Checked == nil && p.Indeterminate() && p.Expanded == nil
		},
		`{accessibilityValue: {min: 1, max: 3, now: 2, text: "two"}}`: func(p *core.View) bool {
			return p.AccessibilityValue() == core.AccessibilityValue{Min: 1, Max: 3, Now: 2, Text: "two"}
		},
		`{accessibilityValue: {text: "some"}}`: func(p *core.View) bool { return p.AccessibilityValue().NoNow },
		`{accessibilityElementsHidden: true, importantForAccessibility: "no-hide-descendants"}`: func(p *core.View) bool {
			return p.AccessibilityElementsHidden() && p.ImportantForAccessibility == core.ImportantNoHideDescendants
		},
		`{accessibilityActions: [{name: "open", label: "Open"}, {name: "close"}]}`: func(p *core.View) bool {
			return len(p.AccessibilityActions) == 2 && p.AccessibilityActions[0] == core.AccessibilityAction{Name: "open", Label: "Open"}
		},
		`{accessibilityLiveRegion: "polite"}`: func(p *core.View) bool { return p.LiveRegion != 0 },
		`{focusable: true, disabled: true, selected: true, checked: true, selectable: true}`: func(p *core.View) bool {
			return p.Focusable() && p.Disabled() && p.Selected() && p.Checked() && p.Selectable
		},
		`{hitSlop: 4, delayLongPress: 200}`: func(p *core.View) bool {
			return p.HitSlop == core.All(4) && p.DelayLongPress == 200*time.Millisecond
		},
		`{panActivation: {activeOffsetX: 1, activeOffsetY: 2, failOffsetX: 3, failOffsetY: 4}}`: func(p *core.View) bool {
			return p.PanActivation == core.PanActivation{ActiveOffsetX: 1, ActiveOffsetY: 2, FailOffsetX: 3, FailOffsetY: 4}
		},
		`{backdropMaterial: "chrome"}`: func(p *core.View) bool { return p.IOS.BackdropMaterial == core.BackdropChrome },
		`{ripple: {borderless: true, radius: 8, foreground: true, color: "#000001"}}`: func(p *core.View) bool {
			return *p.Android.Ripple == core.Ripple{Borderless: true, Radius: 8, Foreground: true, Color: core.RGB(1)}
		},
		`{elevation: 3}`:                       func(p *core.View) bool { return p.Android.Elevation == 3 },
		`{windowDrag: true}`:                   func(p *core.View) bool { return p.Desktop.WindowDrag == core.WindowDragOn },
		`{windowDrag: false}`:                  func(p *core.View) bool { return p.Desktop.WindowDrag == core.WindowDragOff },
		`{onPanStart: null, onPinchEnd: null}`: func(p *core.View) bool { return true },
	}
	for src, ok := range cases {
		t.Run(src, func(t *testing.T) {
			if p := setProps(t, decoding(), (*runtime).viewProp, src); !ok(p) {
				t.Errorf("decoded %+v", p)
			}
		})
	}
}

func TestViewHandlersCallTheirFunctions(t *testing.T) {
	r := decoding()
	p := setProps(t, r, (*runtime).viewProp, `{
		onAccessibilityAction: e => { globalThis.action = e.actionName },
		onFileDrop: e => { globalThis.files = e.uris.length },
	}`)
	p.OnAccessibilityAction("open")
	p.OnFileDrop([]string{"a", "b"})
	if got := jsValue(t, r, `action + files`).String(); got != "open2" {
		t.Errorf("heard %q", got)
	}
	if off := setProps(t, r, (*runtime).viewProp, `{onAccessibilityAction: null, onFileDrop: null, onContextMenu: null}`); off.OnAccessibilityAction != nil ||
		off.OnFileDrop != nil || off.OnContextMenu != nil {
		t.Errorf("null left a handler: %+v", off)
	}
}

func TestTextPropsSetTheirFields(t *testing.T) {
	r := decoding()
	p := setProps(t, r, (*runtime).textProp, `{testID: "a", accessibilityLabel: "b", accessibilityHint: "c", role: "header",
		allowFontScaling: false, maxFontSizeMultiplier: 2, adjustsFontSizeToFit: true, minimumFontScale: 0.5,
		onTextLayout: e => { globalThis.lines = e.lines.map(l => l.text + l.ascender).join() }}`)
	if p.TestID != "a" || p.Label != "b" || p.Hint != "c" || p.Role == 0 || !p.NoFontScaling ||
		p.MaxFontSizeMultiplier != 2 || !p.AdjustsFontSizeToFit || p.MinimumFontScale != 0.5 {
		t.Errorf("decoded %+v", p)
	}
	p.OnTextLayout([]core.TextLine{{Text: "one", Ascender: 2}, {Text: "two", Ascender: 3}})
	if got := jsValue(t, r, `lines`).String(); got != "one2,two3" {
		t.Errorf("heard %q", got)
	}
}

func TestScrollPropsSetTheirFields(t *testing.T) {
	r := decoding()
	p := setProps(t, r, (*runtime).scrollProp, `{testID: "a", horizontal: true, contentOffset: {x: 1, y: 2},
		minimumZoomScale: 0.5, maximumZoomScale: 3, zoomScale: 2, bouncesZoom: false, zoomable: false,
		centerContent: true, refreshing: true, scrollEventThrottle: 16,
		onScroll: e => { globalThis.at = e.offset.x }}`)
	if p.TestID != "a" || !p.Horizontal || p.ContentOffset != (core.Point{X: 1, Y: 2}) || p.MinZoomScale != 0.5 ||
		p.MaxZoomScale != 3 || p.ZoomScaleFunc() != 2 || !p.NoBouncesZoom || !p.NoPinchZoom || !p.CenterContent ||
		!p.Refreshing() || p.ScrollEventThrottle != 16*time.Millisecond {
		t.Errorf("decoded %+v", p)
	}
	p.OnScroll(core.ScrollEvent{Offset: core.Point{X: 7}})
	if got := jsValue(t, r, `at`).ToFloat(); got != 7 {
		t.Errorf("heard %v", got)
	}
	if off := setProps(t, r, (*runtime).scrollProp, `{refreshing: false}`); off.Refreshing == nil || off.Refreshing() {
		t.Error("refreshing: false is still refreshing")
	}
}

func TestInputPropsSetTheirFields(t *testing.T) {
	r := decoding()
	p := setProps(t, r, (*runtime).inputProp, `{value: "v", defaultValue: "d", placeholder: "p", allowFontScaling: false,
		maxFontSizeMultiplier: 2, masked: true, multiline: true, maxLength: 9, readonly: true, autoFocus: true,
		disabled: true, autoCorrect: false, caret: false, inputAccessoryViewID: "bar", testID: "a",
		accessibilityLabel: "b", selectedRange: {start: 1, end: 2},
		onChange: e => { globalThis.heard = "change " + e.value },
		onSubmit: e => { globalThis.heard = "submit " + e.text },
		onKeyPress: e => { globalThis.heard = "key " + e.key }}`)
	if p.Value() != "v" || p.DefaultValue != "d" || p.Placeholder != "p" || !p.NoFontScaling || p.MaxFontSizeMultiplier != 2 ||
		!p.Masked || !p.Multiline || p.MaxLength != 9 || !p.Readonly || !p.AutoFocus || !p.Disabled() || !p.NoAutoCorrect ||
		!p.HideCaret || p.InputAccessoryViewID != "bar" || p.TestID != "a" || p.Label != "b" {
		t.Errorf("decoded %+v", p)
	}
	if got := p.SelectedRange(); got.Start != 1 || got.End != 2 {
		t.Errorf("selected range %+v", got)
	}
	for want, fire := range map[string]func(){
		"change x": func() { p.OnChange("x") },
		"submit x": func() { p.OnSubmit("x") },
		"key x":    func() { p.OnKeyPress("x") },
	} {
		fire()
		if got := jsValue(t, r, `heard`).String(); got != want {
			t.Errorf("heard %q, want %q", got, want)
		}
	}
}

func TestImagePropsSetTheirFields(t *testing.T) {
	r := decoding()
	p := setProps(t, r, (*runtime).imageProp, `{src: "https://a.example/b.png", placeholder: "https://a.example/c.png",
		blurRadius: 3, fadeDuration: 100, testID: "a", alt: "b"}`)
	if p.TestID != "a" || p.Label != "b" || p.BlurRadius != 3 || p.FadeDuration != 100*time.Millisecond {
		t.Errorf("decoded %+v", p)
	}
}

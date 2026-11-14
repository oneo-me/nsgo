//go:build linux && !android

package linux

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

func closeTo(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestKeyNamesMatchOnKeys(t *testing.T) {
	for _, c := range []struct {
		keyval uint32
		shift  bool
		want   string
	}{
		{'a', false, "a"}, {'A', false, "a"}, {'Z', false, "z"}, {'0', false, "0"}, {'9', false, "9"},
		{gdkF1, false, "f1"}, {gdkF1 + 23, false, "f24"},
		{gdkEscape, false, "escape"}, {gdkReturn, false, "enter"}, {gdkSpace, false, "space"},
		{gdkPageUp, false, "pageup"}, {gdkDown, false, "down"},
		{'?', true, "/"}, {'+', true, "="}, {'_', true, "-"},
		{0, false, ""},
	} {
		if got := keyName(c.keyval, c.shift); got != c.want {
			t.Errorf("keyName(0x%X, %v) = %q, want %q", c.keyval, c.shift, got, c.want)
		}
	}
}

func TestPortalTriggersUseXDGNames(t *testing.T) {
	for _, test := range []struct {
		key, want string
		ok        bool
	}{
		{"mod+shift+s", "CTRL+SHIFT+s", true},
		{"meta+alt+space", "LOGO+ALT+space", true},
		{"ctrl+f12", "CTRL+F12", true},
		{"ctrl+plus", "CTRL+plus", true},
		{"pageup", "Page_Up", true},
		{"unknown+a", "", false},
		{"shift", "", false},
	} {
		got, ok := portalTrigger(test.key)
		if got != test.want || ok != test.ok {
			t.Errorf("portalTrigger(%q) = (%q, %v), want (%q, %v)", test.key, got, ok, test.want, test.ok)
		}
	}
}

func TestPortalQuoteEscapesGVariantStrings(t *testing.T) {
	if got := portalQuote("a'b\\c"); got != "'a\\'b\\\\c'" {
		t.Fatalf("portalQuote = %q", got)
	}
}

func TestX11TriggerModifiers(t *testing.T) {
	mods, ok := x11TriggerModifiers([]string{"CTRL", "ALT", "SHIFT", "LOGO"})
	if !ok || mods != x11ControlMask|x11Mod1Mask|x11ShiftMask|x11Mod4Mask {
		t.Fatalf("x11TriggerModifiers = (%#x, %v)", mods, ok)
	}
	if _, ok := x11TriggerModifiers([]string{"NOPE"}); ok {
		t.Fatal("unknown X11 modifier was accepted")
	}
}

func TestUnshiftedFindsTheTypingKey(t *testing.T) {
	if got := unshifted('?'); got != '/' {
		t.Fatalf("? types / on a US layout, got %q", got)
	}
	if got := unshifted('A'); got != 'A' {
		t.Fatalf("a letter stayed %q", got)
	}
}

func TestSessionLocaleIsABCP47Tag(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "de_DE.UTF-8@euro")
	t.Setenv("LANG", "fr_FR.UTF-8")
	if got := sessionLocale(); got != "de-DE" {
		t.Fatalf("locale = %q, want de-DE", got)
	}
}

func TestRightToLeftFollowsTheLanguage(t *testing.T) {
	for _, c := range []struct {
		locale string
		want   bool
	}{{"ar-SA", true}, {"he-IL", true}, {"fa", true}, {"en-US", false}, {"zh-CN", false}, {"", false}} {
		if got := rightToLeft(c.locale); got != c.want {
			t.Errorf("rightToLeft(%q) = %v, want %v", c.locale, got, c.want)
		}
	}
}

func TestPanActivation(t *testing.T) {
	if activates(driver.PanActivation{}, driver.Point{X: 5, Y: 5}) {
		t.Fatal("a short drag activated a pan")
	}
	if !activates(driver.PanActivation{}, driver.Point{X: 8, Y: 8}) {
		t.Fatal("a drag past the slop did not activate")
	}
	a := driver.PanActivation{ActiveOffsetX: 20, ActiveOffsetY: 40}
	if !activates(a, driver.Point{X: 25}) || activates(a, driver.Point{X: 10, Y: 10}) {
		t.Fatal("active offsets")
	}
	a.FailOffsetY = 30
	if !fails(a, driver.Point{Y: 50}) || fails(a, driver.Point{Y: 20}) {
		t.Fatal("fail offsets")
	}
	if rest(driver.EventPinch) != 1 || rest(driver.EventRotation) != 0 {
		t.Fatal("gesture rest values")
	}
}

func TestMixMovesOnlyThePropertiesAsked(t *testing.T) {
	from := driver.ViewStyle{Opacity: 0, Background: driver.RGB(0xff0000), BorderWidth: 0,
		Transform: driver.Transform{TranslateX: 0}, ShadowRadius: 0}
	to := driver.ViewStyle{Opacity: 1, Background: driver.RGB(0x0000ff), BorderWidth: 10,
		Transform: driver.Transform{TranslateX: 20}, ShadowRadius: 4}
	mixed := driver.MixStyles(from, to, 0.5, driver.TransitionOpacity|driver.TransitionBackground)
	if mixed.Opacity != 0.5 {
		t.Fatalf("opacity = %v", mixed.Opacity)
	}
	if mixed.Background != driver.RGBA(0x800080, 1) {
		t.Fatalf("background = %08x", mixed.Background)
	}
	if mixed.BorderWidth != 10 || mixed.Transform.TranslateX != 20 {
		t.Fatalf("untouched fields = %+v", mixed)
	}
	full := driver.MixStyles(from, to, 0.5, allProperties)
	if full.Transform.TranslateX != 10 || full.ShadowRadius != 2 {
		t.Fatalf("full mix = %+v", full)
	}
	same := driver.MixStyles(to, to, 0.5, allProperties)
	if same.Opacity != to.Opacity || same.Background != to.Background || same.BorderWidth != to.BorderWidth ||
		same.Transform.TranslateX != to.Transform.TranslateX {
		t.Fatalf("unchanged = %+v, want %+v", same, to)
	}
}

func TestImageDensityFromTheURI(t *testing.T) {
	for _, c := range []struct {
		uri  string
		want float64
	}{
		{"logo.png", 1}, {"logo@2x.png", 2}, {"/images/logo@1.5x.png", 1.5},
		{"logo@3X.png", 1}, {"logo@2.png", 1}, {"logo@x.png", 1}, {"logo@-2x.png", 1}, {"a/b@2x", 2},
	} {
		if got := density(c.uri); got != c.want {
			t.Errorf("density(%q) = %g, want %g", c.uri, got, c.want)
		}
	}
}

func TestImageFitModes(t *testing.T) {
	w := &window{}
	s := &imageSlot{src: driver.ImageSource{}, pic: &picture{width: 400, height: 200}}
	r := rect{x: 5, w: 100, h: 100}
	fit := func(mode driver.ResizeMode, pos driver.ObjectPosition, s *imageSlot) rect {
		return w.fit(&view{image: driver.ImageConfig{ResizeMode: mode, Position: pos}}, s, r)
	}
	if dst := fit(driver.Stretch, driver.ObjectPosition{}, s); dst != r {
		t.Fatalf("stretch = %+v", dst)
	}
	if dst := fit(driver.Contain, driver.ObjectPosition{}, s); dst != (rect{x: 5, y: 25, w: 100, h: 50}) {
		t.Fatalf("contain = %+v", dst)
	}
	if dst := fit(driver.Cover, driver.ObjectPosition{}, s); dst != (rect{x: -45, w: 200, h: 100}) {
		t.Fatalf("cover = %+v", dst)
	}
	if dst := fit(driver.Cover, driver.ObjectPosition{X: core.PositionEnd}, s); dst != (rect{x: -95, w: 200, h: 100}) {
		t.Fatalf("cover at the right = %+v", dst)
	}
	if dst := fit(driver.Center, driver.ObjectPosition{}, s); dst != (rect{x: -145, y: -50, w: 400, h: 200}) {
		t.Fatalf("center of a large picture = %+v", dst)
	}
	if dst := fit(driver.ScaleDown, driver.ObjectPosition{}, s); dst != (rect{x: 5, y: 25, w: 100, h: 50}) {
		t.Fatalf("scale-down of a large picture = %+v", dst)
	}
	small := &imageSlot{src: driver.ImageSource{}, pic: &picture{width: 40, height: 20}}
	if dst := fit(driver.ScaleDown, driver.ObjectPosition{}, small); dst != (rect{x: 35, y: 40, w: 40, h: 20}) {
		t.Fatalf("scale-down = %+v", dst)
	}
	small.src.URI = "logo@2x.png"
	if dst := fit(driver.Center, driver.ObjectPosition{}, small); dst != (rect{x: 45, y: 45, w: 20, h: 10}) {
		t.Fatalf("center @2x = %+v", dst)
	}
	small.src.Size = driver.Size{Width: 80, Height: 40}
	if dst := fit(driver.Center, driver.ObjectPosition{Y: core.PositionStart}, small); dst != (rect{x: 15, w: 80, h: 40}) {
		t.Fatalf("center with size at the top = %+v", dst)
	}
}

func TestImageFadesIn(t *testing.T) {
	s := &imageSlot{}
	now := time.Now()
	if got := s.alpha(now); got != 1 {
		t.Fatalf("alpha without a fade = %v", got)
	}
	s.fadeAt, s.fade = now, 200*time.Millisecond
	if got := s.alpha(now.Add(50 * time.Millisecond)); got != 0.25 {
		t.Fatalf("alpha a quarter in = %v", got)
	}
	if got := s.alpha(now.Add(time.Second)); got != 1 {
		t.Fatalf("alpha after = %v", got)
	}
}

func TestHitFindsTheTopmostMatchAndSkipsHiddenViews(t *testing.T) {
	below := &view{id: 2, kind: driver.KindView, frame: driver.Bounds{Size: driver.Size{Width: 50, Height: 50}}}
	above := &view{id: 3, kind: driver.KindView, frame: driver.Bounds{Origin: driver.Point{X: 20}, Size: driver.Size{Width: 50, Height: 50}}}
	root := &view{id: 1, kind: driver.KindView, frame: driver.Bounds{Size: driver.Size{Width: 100, Height: 100}}, children: []*view{below, above}}
	all := func(*view) bool { return true }
	if got := hit(root, driver.Point{X: 30, Y: 30}, all); got != above {
		t.Fatalf("hit = %p, want the later child %p", got, above)
	}
	above.style.Hidden = true
	if got := hit(root, driver.Point{X: 30, Y: 30}, all); got != below {
		t.Fatalf("hidden child = %p, want %p", got, below)
	}
	if got := hit(root, driver.Point{X: 300, Y: 300}, all); got != nil {
		t.Fatalf("outside = %+v", got)
	}
	scroll := &view{id: 4, kind: driver.KindScroll, frame: driver.Bounds{Size: driver.Size{Width: 100, Height: 100}},
		offset: driver.Point{Y: 40}, children: []*view{{
			id: 5, kind: driver.KindView, frame: driver.Bounds{Origin: driver.Point{Y: 60}, Size: driver.Size{Width: 20, Height: 20}},
		}}}
	if got := hit(scroll, driver.Point{X: 10, Y: 30}, all); got == nil || got.id != 5 {
		t.Fatalf("scrolled hit = %+v", got)
	}
}

func TestScrollsAtReturnsTheScrollViewsAroundAPoint(t *testing.T) {
	visible := driver.ViewStyle{Opacity: 1}
	inner := &view{id: 3, kind: driver.KindScroll, style: visible, frame: driver.Bounds{Origin: driver.Point{X: 10, Y: 10}, Size: driver.Size{Width: 50, Height: 50}}}
	outer := &view{id: 2, kind: driver.KindScroll, style: visible, frame: driver.Bounds{Size: driver.Size{Width: 100, Height: 100}}, children: []*view{inner}}
	root := &view{id: 1, kind: driver.KindView, style: visible, frame: driver.Bounds{Size: driver.Size{Width: 100, Height: 100}}, children: []*view{outer}}
	got := scrollsAt(root, driver.Point{X: 30, Y: 30}, nil)
	if len(got) != 2 || got[0] != outer || got[1] != inner {
		t.Fatalf("scrollsAt = %v", got)
	}
	if got := scrollsAt(root, driver.Point{X: 300}, nil); len(got) != 0 {
		t.Fatalf("outside = %v", got)
	}
}

// TestScrollsAtPassesThroughViewsThatTakeNoPointer turns the wheel over
// views that let the pointer through, as a DataTable's body and overlays
// do: the scroll view they cover is still the one under it.
func TestScrollsAtPassesThroughViewsThatTakeNoPointer(t *testing.T) {
	visible := driver.ViewStyle{Opacity: 1}
	area := driver.Bounds{Size: driver.Size{Width: 100, Height: 100}}
	at := driver.Point{X: 30, Y: 30}
	scroll := &view{id: 2, kind: driver.KindScroll, style: visible, frame: area}
	body := &view{id: 3, kind: driver.KindView, style: visible, frame: area,
		props: driver.ViewProps{PointerEvents: driver.PointerEventsNone}}
	overlay := &view{id: 4, kind: driver.KindView, style: visible, frame: area,
		props: driver.ViewProps{PointerEvents: driver.PointerEventsBoxNone}}
	root := &view{id: 1, kind: driver.KindView, style: visible, frame: area, children: []*view{scroll, body, overlay}}
	if got := scrollsAt(root, at, nil); len(got) != 1 || got[0] != scroll {
		t.Fatalf("scrollsAt through views that take no pointer = %v, want the scroll view", got)
	}
	// A view that takes the pointer keeps the wheel from what it covers.
	overlay.props.PointerEvents = driver.PointerEventsAuto
	if got := scrollsAt(root, at, nil); len(got) != 0 {
		t.Fatalf("scrollsAt under a view that takes the pointer = %v", got)
	}
}

// TestBarAtPassesThroughViewsThatTakeNoPointer grabs the bar of a scroll
// view under the views that let the pointer through.
func TestBarAtPassesThroughViewsThatTakeNoPointer(t *testing.T) {
	visible := driver.ViewStyle{Opacity: 1}
	area := driver.Bounds{Size: driver.Size{Width: 100, Height: 100}}
	scroll := &view{id: 2, kind: driver.KindScroll, style: visible, frame: area,
		size: driver.Size{Width: 100, Height: 300}}
	body := &view{id: 3, kind: driver.KindView, style: visible, frame: area,
		props: driver.ViewProps{PointerEvents: driver.PointerEventsNone}}
	overlay := &view{id: 4, kind: driver.KindView, style: visible, frame: area,
		props: driver.ViewProps{PointerEvents: driver.PointerEventsBoxNone}}
	root := &view{id: 1, kind: driver.KindView, style: visible, frame: area, children: []*view{scroll, body, overlay}}
	at := driver.Point{X: 95, Y: 40}
	if got, local := barAt(root, at); got != scroll || local != at {
		t.Fatalf("barAt = %v at %v, want the scroll view at %v", got, local, at)
	}
	overlay.props.PointerEvents = driver.PointerEventsAuto
	if got, _ := barAt(root, at); got != nil {
		t.Fatalf("barAt under a view that takes the pointer = %v", got)
	}
}

func TestBackgroundIsTheNearestOpaqueAncestor(t *testing.T) {
	root := &view{id: 1, style: driver.ViewStyle{Background: driver.RGB(0x112233)}}
	clear := &view{id: 2, parent: root}
	colored := &view{id: 3, parent: clear, style: driver.ViewStyle{Background: driver.RGB(0x445566)}}
	if got := background(colored); got != driver.RGB(0x445566) {
		t.Fatalf("background = %08x", got)
	}
	if got := background(clear); got != driver.RGB(0x112233) {
		t.Fatalf("inherited background = %08x", got)
	}
	if got := background(nil); got != driver.RGB(0xffffff) {
		t.Fatalf("default background = %08x", got)
	}
}

func TestSurfaceBoundsAddsAncestorsAndScrollOffsets(t *testing.T) {
	root := &view{id: 1, kind: driver.KindView, frame: driver.Bounds{Origin: driver.Point{X: 10, Y: 20},
		Size: driver.Size{Width: 200, Height: 200}}, offset: driver.Point{X: 3, Y: 4}}
	child := &view{id: 2, kind: driver.KindView, parent: root, frame: driver.Bounds{Origin: driver.Point{X: 5, Y: 6},
		Size: driver.Size{Width: 30, Height: 40}}}
	w := &window{root: 1, views: []*view{nil, root, child}}
	b, ok := w.surfaceBounds(child)
	if !ok || b != (driver.Bounds{Origin: driver.Point{X: 12, Y: 22}, Size: driver.Size{Width: 30, Height: 40}}) {
		t.Fatalf("surfaceBounds = %+v, %v", b, ok)
	}
	if _, ok := w.surfaceBounds(&view{id: 9}); ok {
		t.Fatal("a detached view reported bounds")
	}
}

func TestMenuKeysSkipEditingRolesAndKeepTheFirst(t *testing.T) {
	w := &window{chrome: chrome{keys: map[string]menuRef{}}}
	w.menuKeys([]driver.MenuEntry{
		{Role: driver.MenuCopy},
		{Label: "Save", Shortcut: "mod+s"},
		{Label: "Save again", Shortcut: "mod+s"},
		{Label: "Disabled", Shortcut: "mod+d", Disabled: true},
		{Label: "Quit", Role: driver.MenuQuit},
	})
	if len(w.chrome.keys) != 2 {
		t.Fatalf("keys = %v", w.chrome.keys)
	}
	if ref := w.chrome.keys["mod+s"]; ref.id != 0 {
		t.Fatalf("mod+s = %+v", ref)
	}
	if ref, ok := w.chrome.keys["alt+f4"]; !ok || ref.role != driver.MenuQuit {
		t.Fatalf("the role's shortcut = %+v, %v", ref, ok)
	}
	if _, ok := w.chrome.keys["mod+c"]; ok {
		t.Fatal("an editing role bound its shortcut")
	}
}

func TestMenuShortcutsLeaveFieldsTheirKeys(t *testing.T) {
	w := &window{chrome: chrome{keys: map[string]menuRef{}}}
	w.menuKeys([]driver.MenuEntry{{Label: "Run", Shortcut: "mod+r", NotInText: true}})
	if ref := w.chrome.keys["mod+r"]; !ref.notInText {
		t.Fatalf("mod+r = %+v", ref)
	}
	w.focus = &view{}
	if w.menuShortcut("mod+r") {
		t.Fatal("the menu took a focused field's key")
	}
}

func TestMenuHelpers(t *testing.T) {
	for _, r := range []driver.MenuRole{driver.MenuAbout, driver.MenuHide, driver.MenuHideOthers, driver.MenuShowAll} {
		if !unsupported(r) {
			t.Errorf("%v is supported", r)
		}
	}
	if unsupported(driver.MenuQuit) || unsupported(driver.MenuCopy) {
		t.Fatal("a supported role is unsupported")
	}
	if got := shortcutOf(&driver.MenuEntry{Role: driver.MenuQuit}); got != "alt+f4" {
		t.Fatalf("quit shortcut = %q", got)
	}
	if got := shortcutOf(&driver.MenuEntry{Role: driver.MenuQuit, Shortcut: "mod+q"}); got != "mod+q" {
		t.Fatalf("given shortcut = %q", got)
	}
}

func TestEntryStyleCarriesTheTextStyle(t *testing.T) {
	v := &view{id: 4, input: driver.InputConfig{Style: driver.TextStyle{
		Font:  driver.Font{Family: "Noto Sans", Size: 15, Weight: driver.FontWeightBold, Italic: true},
		Color: driver.RGB(0x123456), Align: driver.TextAlignEnd,
	}, PlaceholderColor: driver.RGB(0xff0000), SelectionColor: driver.RGB(0x00ff00)}}
	css := entryStyle(v)
	for _, want := range []string{"transparent", "#nsgo-input-4 *", "placeholder", "rgba(255,0,0,1.000)", "selection { background-color: rgba(0,255,0,0.302)", "caret-color: rgba(0,255,0", `font-family: "Noto Sans"`, "font-size: 15.000pt", "font-weight: 700",
		"font-style: italic", "rgba(18,52,86"} {
		if !strings.Contains(css, want) {
			t.Errorf("CSS lacks %q:\n%s", want, css)
		}
	}
}

func TestHistoryKeysStepThroughAPage(t *testing.T) {
	for _, c := range []struct {
		keyval, state uint32
		want          int
	}{
		{gdkLeft, gdkAlt, -1}, {gdkKPRight, gdkAlt, 1}, {gdkBack, 0, -1}, {gdkForward, 0, 1},
		{gdkLeft, 0, 0}, {gdkLeft, gdkAlt | gdkShift, 0}, {gdkBack, gdkControl, 0},
		// Lock keys do not count.
		{gdkLeft, gdkAlt | 1<<1, -1},
	} {
		if got := historyKey(c.keyval, c.state); got != c.want {
			t.Errorf("historyKey(0x%X, 0x%X) = %d, want %d", c.keyval, c.state, got, c.want)
		}
	}
}

func TestMediaErrorsKeepTheFirstLine(t *testing.T) {
	gst := "Error from element /GstPlayBin3:playbin3/GstDecodebin3:decodebin3-0: " +
		"Your GStreamer installation is missing a plug-in.\n" +
		"Your GStreamer installation is missing a plug-in.\n" +
		"../gstdecodebin3.c(3258): mq_slot_check_reconfiguration (): /GstPlayBin3\n" +
		"No suitable plugins found"
	if got := mediaError(gst); !strings.HasPrefix(got, "Your GStreamer installation is missing a plug-in. No") ||
		strings.Contains(got, "\n") {
		t.Errorf("mediaError = %q", got)
	}
	if got := mediaError("Resource not found."); got != "Resource not found." {
		t.Errorf("mediaError = %q", got)
	}
}

func TestVideoFitModes(t *testing.T) {
	for mode, want := range map[driver.ResizeMode]int32{
		driver.Cover: gtkContentFitCover, driver.Contain: gtkContentFitContain,
		driver.Stretch: gtkContentFitFill, driver.Center: gtkContentFitScaleDown,
	} {
		if got := contentFit(mode); got != want {
			t.Errorf("contentFit(%d) = %d, want %d", mode, got, want)
		}
	}
}

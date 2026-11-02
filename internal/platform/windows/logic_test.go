//go:build windows

package windows

import (
	"math"
	"testing"
	"time"
	"unsafe"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/imagedecode"
)

func closeTo(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

// TestRedrawAllDrawsTheWholeWindow checks that the frame after redrawAll
// draws every area, even with nothing queued: a window that lost its web
// views must redraw what the composition showed over it.
func TestRedrawAllDrawsTheWholeWindow(t *testing.T) {
	w := &window{}
	w.redrawAll()
	_, _, changedAll, drawAll := w.frameDamage(rect{0, 0, 100, 100}, false)
	if !changedAll || !drawAll {
		t.Fatalf("redrawAll's frame drew parts: changedAll %v, drawAll %v", changedAll, drawAll)
	}
	if w.damage.whole {
		t.Fatal("the whole-window mark survived its frame")
	}
}

func TestMatrixComposesAndInverts(t *testing.T) {
	// Points go through the first matrix, then the second.
	m := translation(10, 0).then(scaling(2, 3))
	if x, y := m.apply(1, 1); x != 22 || y != 3 {
		t.Fatalf("apply = %g, %g; want 22, 3", x, y)
	}
	if x, y := m.invert().apply(22, 3); !closeTo(x, 1) || !closeTo(y, 1) {
		t.Fatalf("invert = %g, %g; want 1, 1", x, y)
	}
	if !m.axisAligned() || !closeTo(m.stretch(), math.Sqrt(6)) {
		t.Fatalf("axisAligned %v, stretch %g", m.axisAligned(), m.stretch())
	}
	if b := m.bound(rect{0, 0, 2, 2}); b != (rect{20, 0, 24, 6}) {
		t.Fatalf("bound = %+v", b)
	}
	// A singular matrix inverts to the identity rather than dividing by
	// zero.
	if got := (matrix{}).invert(); got != identity {
		t.Fatalf("singular invert = %+v", got)
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
	// Everything without its property comes from the target.
	if mixed.BorderWidth != 10 || mixed.Transform.TranslateX != 20 {
		t.Fatalf("untouched fields = %+v", mixed)
	}

	full := driver.MixStyles(from, to, 0.5, allProperties)
	if full.Transform.TranslateX != 10 || full.ShadowRadius != 2 {
		t.Fatalf("full mix = %+v", full)
	}
	// A style that does not change keeps its look.
	same := driver.MixStyles(to, to, 0.5, allProperties)
	if same.Opacity != to.Opacity || same.Background != to.Background || same.BorderWidth != to.BorderWidth ||
		same.Transform.TranslateX != to.Transform.TranslateX {
		t.Fatalf("unchanged = %+v, want %+v", same, to)
	}
}

func TestKeyNamesMatchOnKeys(t *testing.T) {
	for _, c := range []struct {
		vk   uintptr
		want string
	}{
		{'A', "a"}, {'Z', "z"}, {'0', "0"}, {'9', "9"},
		{0x60, "0"}, {0x69, "9"}, {0x70, "f1"}, {0x87, "f24"},
		{0x1B, "escape"}, {0x0D, "enter"}, {0x20, "space"},
		{0x10, ""}, {0x11, ""}, {0x12, ""},
	} {
		if got := keyName(c.vk); got != c.want {
			t.Errorf("keyName(0x%X) = %q, want %q", c.vk, got, c.want)
		}
	}
}

func TestShortcutLabelsUseWindowsNames(t *testing.T) {
	for _, c := range []struct{ key, want string }{
		{"mod+shift+n", "Ctrl+Shift+N"},
		{"ctrl+alt+delete", "Ctrl+Alt+Del"},
		{"meta+pageup", "Win+PgUp"},
		{"pagedown", "PgDn"},
		{"escape", "Esc"},
		{"f5", "F5"},
		{"a", "A"},
	} {
		if got := shortcutLabel(c.key); got != c.want {
			t.Errorf("shortcutLabel(%q) = %q, want %q", c.key, got, c.want)
		}
	}
}

func TestBackdropFor(t *testing.T) {
	for _, c := range []struct {
		m    driver.WindowMaterial
		want int32
	}{
		{driver.WindowMaterialNone, backdropNone},
		{driver.WindowMaterialMica, backdropMica},
		{driver.WindowMaterialTabbed, backdropMicaAlt},
		{driver.WindowMaterialAcrylic, backdropAcrylic},
		{driver.WindowMaterialSidebar, backdropMica},
		{driver.WindowMaterialMenu, backdropAcrylic},
		{driver.WindowMaterial(100), backdropMica},
	} {
		if got := backdropFor(c.m); got != c.want {
			t.Errorf("backdropFor(%d) = %d, want %d", c.m, got, c.want)
		}
	}
}

func TestRightToLeftFollowsTheLocale(t *testing.T) {
	for _, c := range []struct {
		locale string
		want   bool
	}{{"ar-SA", true}, {"he-IL", true}, {"en-US", false}, {"zh-CN", false}, {"", false}} {
		if got := rightToLeft(c.locale); got != c.want {
			t.Errorf("rightToLeft(%q) = %v, want %v", c.locale, got, c.want)
		}
	}
}

func TestImageDensityFromTheURI(t *testing.T) {
	for _, c := range []struct {
		uri  string
		want float64
	}{
		{"logo.png", 1},
		{"logo@2x.png", 2},
		{"C:/images/logo@1.5x.png", 1.5},
		{"logo@3X.png", 1}, // the width suffix is lowercase
		{"logo@2.png", 1},
		{"logo@x.png", 1},
		{"logo@-2x.png", 1},
		{"a/b@2x", 2},
	} {
		if got := density(c.uri); got != c.want {
			t.Errorf("density(%q) = %g, want %g", c.uri, got, c.want)
		}
	}
}

func TestImageFitModes(t *testing.T) {
	w := &window{scale: 1}
	s := &imageSlot{src: driver.ImageSource{}, pic: &picture{width: 400, height: 200}}
	r := rectF{5, 0, 105, 100}
	fit := func(mode driver.ResizeMode, pos driver.ObjectPosition, s *imageSlot) rectF {
		return w.fit(&view{image: driver.ImageConfig{ResizeMode: mode, Position: pos}}, s, r)
	}
	if dst := fit(driver.Stretch, driver.ObjectPosition{}, s); dst != r {
		t.Fatalf("stretch = %+v", dst)
	}
	if dst := fit(driver.Contain, driver.ObjectPosition{}, s); dst != (rectF{5, 25, 105, 75}) {
		t.Fatalf("contain = %+v", dst)
	}
	if dst := fit(driver.Cover, driver.ObjectPosition{}, s); dst != (rectF{-45, 0, 155, 100}) {
		t.Fatalf("cover = %+v", dst)
	}
	if dst := fit(driver.Cover, driver.ObjectPosition{X: core.PositionStart}, s); dst != (rectF{5, 0, 205, 100}) {
		t.Fatalf("cover at the left = %+v", dst)
	}
	if dst := fit(driver.Center, driver.ObjectPosition{}, s); dst != (rectF{-145, -50, 255, 150}) {
		t.Fatalf("center of a large picture = %+v", dst)
	}
	if dst := fit(driver.ScaleDown, driver.ObjectPosition{}, s); dst != (rectF{5, 25, 105, 75}) {
		t.Fatalf("scale-down of a large picture = %+v", dst)
	}
	// A picture smaller than the box stays its own size.
	small := &imageSlot{src: driver.ImageSource{}, pic: &picture{width: 40, height: 20}}
	if dst := fit(driver.ScaleDown, driver.ObjectPosition{}, small); dst != (rectF{35, 40, 75, 60}) {
		t.Fatalf("scale-down = %+v", dst)
	}
	small.src.URI = "logo@2x.png"
	if dst := fit(driver.Center, driver.ObjectPosition{}, small); dst != (rectF{45, 45, 65, 55}) {
		t.Fatalf("center @2x = %+v", dst)
	}
	// A declared size wins over the URI's density.
	small.src.Size = driver.Size{Width: 80, Height: 40}
	if dst := fit(driver.Center, driver.ObjectPosition{Y: core.PositionEnd}, small); dst != (rectF{15, 60, 95, 100}) {
		t.Fatalf("center with size at the bottom = %+v", dst)
	}
	// Edges fall on whole pixels.
	w.scale = 2
	odd := &imageSlot{pic: &picture{width: 3, height: 3}}
	if dst := fit(driver.Center, driver.ObjectPosition{}, odd); dst != (rectF{53.5, 48.5, 56.5, 51.5}) {
		t.Fatalf("center at 2x = %+v", dst)
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

func TestColorsConvertForDirect2D(t *testing.T) {
	if got := colorOf(driver.RGBA(0xff0000, 0.5)); got.r != 1 || got.g != 0 || got.b != 0 || got.a < 0.49 || got.a > 0.51 {
		t.Fatalf("colorOf = %+v", got)
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
	// A scroll view's children move by their offset.
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

// TestWheelTurnsTheViewsAlongItsAxis turns a vertical wheel over a
// vertical scroll view holding a horizontal one, as a DataTable's is: the
// page scrolls, the row inside it only on a tilt, and a row alone takes a
// vertical wheel too.
func TestWheelTurnsTheViewsAlongItsAxis(t *testing.T) {
	visible := driver.ViewStyle{Opacity: 1}
	vertical := &view{id: 2, kind: driver.KindScroll, style: visible,
		frame: driver.Bounds{Size: driver.Size{Width: 100, Height: 100}}, size: driver.Size{Width: 100, Height: 300}}
	horizontal := &view{id: 3, kind: driver.KindScroll, style: visible, scroll: driver.ScrollConfig{Horizontal: true},
		frame: driver.Bounds{Origin: driver.Point{X: 10, Y: 10}, Size: driver.Size{Width: 80, Height: 80}},
		size:  driver.Size{Width: 300, Height: 80}}
	under := []*view{vertical, horizontal}
	if v, to := wheelTurn(under, -1, false); v != vertical || to.Y != 48 || to.X != 0 {
		t.Fatalf("vertical wheel = %v at %v, want the vertical view at Y 48", v, to)
	}
	if v, to := wheelTurn(under, 1, true); v != horizontal || to.X != 48 || to.Y != 0 {
		t.Fatalf("tilt = %v at %v, want the horizontal view at X 48", v, to)
	}
	// A horizontal view takes a vertical wheel where none is vertical.
	if v, to := wheelTurn([]*view{horizontal}, -1, false); v != horizontal || to.X != 48 {
		t.Fatalf("vertical wheel over a row = %v at %v, want the horizontal view at X 48", v, to)
	}
	// At the vertical view's end, the wheel does not go to the row inside.
	vertical.offset.Y = 200
	if v, _ := wheelTurn(under, -1, false); v != nil {
		t.Fatalf("vertical wheel at the end = %v, want none", v)
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
	// A view outside the root's tree has no surface bounds.
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
	w.focus = &textInput{}
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
	if boolArg(true) != 1 || boolArg(false) != 0 {
		t.Fatal("boolArg")
	}
	if !repeated(1<<30) || repeated(0) {
		t.Fatal("repeated")
	}
}

func TestPixelConversionsUseTheWindowScale(t *testing.T) {
	w := &window{scale: 1.5}
	if got := w.px(10); got != 15 {
		t.Fatalf("px = %d", got)
	}
	if got := w.point(15, 30); got != (driver.Point{X: 10, Y: 20}) {
		t.Fatalf("point = %+v", got)
	}
}

func testWindow() *window {
	return &window{scale: 1}
}

func TestSpanAttachmentsTakeTheirPlaceAndGrowTheLine(t *testing.T) {
	w := testWindow()
	style := driver.TextStyle{Font: driver.Font{Family: "Segoe UI", Size: 16, Weight: 400}, Color: driver.RGB(0)}
	text := "ab " + string(driver.AttachmentRune) + " cd"
	at := &driver.AttributedText{Text: text, Style: style,
		Spans:       []driver.TextSpan{{Start: 0, End: 3, Style: style}, {Start: 3, End: 6, Style: style}, {Start: 6, End: len(text), Style: style}},
		Attachments: []driver.TextAttachment{{Offset: 3, Size: driver.Size{Width: 40, Height: 60}}}}
	plain := &driver.AttributedText{Text: "ab x cd", Style: style}
	pt, pp := w.Prepare(at), w.Prepare(plain)
	defer pt.Release()
	defer pp.Release()
	size, plainSize := pt.Measure(driver.Auto), pp.Measure(driver.Auto)
	if size.Height < 60 || size.Height <= plainSize.Height {
		t.Fatalf("height with a 60pt attachment = %v, plain %v", size.Height, plainSize.Height)
	}
	l := pt.Layout(size)
	defer l.Release()
	al, ok := l.(core.AttachmentLayout)
	if !ok {
		t.Fatal("the layout reports no attachments")
	}
	box, shown := al.Attachment(0)
	if !shown || box.Size != (driver.Size{Width: 40, Height: 60}) || box.Origin.X <= 0 || math.Abs(float64(box.Origin.Y)) > 0.5 {
		t.Fatalf("attachment = %+v, %v; want 40×60 after the first word, at the top of its line", box, shown)
	}
	// It stands on the baseline.
	baseline := l.(*textLayout).lines[0].baseline
	if bottom := float32(box.Origin.Y + box.Size.Height); math.Abs(float64(bottom-baseline)) > 0.01 {
		t.Errorf("attachment bottom = %v, want the baseline %v", bottom, baseline)
	}
	if _, shown := al.Attachment(1); shown {
		t.Error("a missing attachment is shown")
	}
}

// TestAttachmentsGrowLinesOfAFixedHeight checks that a line height does
// not cut an attachment taller than it: its line grows, and the lines
// after it move down.
func TestAttachmentsGrowLinesOfAFixedHeight(t *testing.T) {
	w := testWindow()
	style := driver.TextStyle{Font: driver.Font{Family: "Segoe UI", Size: 16, Weight: 400}, Color: driver.RGB(0), LineHeight: 24}
	text := "ab " + string(driver.AttachmentRune) + "\ncd\nef"
	at := &driver.AttributedText{Text: text, Style: style,
		Attachments: []driver.TextAttachment{{Offset: 3, Size: driver.Size{Width: 40, Height: 60}}}}
	p := w.Prepare(at)
	defer p.Release()
	size := p.Measure(driver.Auto)
	if want := driver.Points(math.Ceil(float64(60 + 24 + 24 + 24 - p.(*preparedText).baseline))); size.Height != want {
		t.Errorf("height = %v, want the attachment's line, grown, and two of 24", size.Height)
	}
	l := p.Layout(size).(*textLayout)
	defer l.Release()
	// Every line moves down by what the first grew above its baseline.
	b := p.(*preparedText).baseline
	if len(l.lines) != 3 || len(l.groups) != 1 || math.Abs(float64(l.groups[0].shift-(60-b))) > 0.01 {
		t.Fatalf("lines %+v, groups %+v; want all moved down by %v", l.lines, l.groups, 60-b)
	}
	if box, _ := l.Attachment(0); box.Origin.Y != 0 {
		t.Errorf("the attachment is at %v, want the top", box.Origin.Y)
	}
}

func TestSpanAttachmentsPastTheLineLimitAreCut(t *testing.T) {
	w := testWindow()
	style := driver.TextStyle{Font: driver.Font{Family: "Segoe UI", Size: 16, Weight: 400}, Color: driver.RGB(0)}
	text := "first line words\n" + string(driver.AttachmentRune)
	at := &driver.AttributedText{Text: text, Style: style, MaxLines: 1,
		Spans:       []driver.TextSpan{{Start: 0, End: len(text), Style: style}},
		Attachments: []driver.TextAttachment{{Offset: len(text) - 3, Size: driver.Size{Width: 10, Height: 10}}}}
	p := w.Prepare(at)
	defer p.Release()
	l := p.Layout(p.Measure(driver.Auto))
	defer l.Release()
	if _, shown := l.(core.AttachmentLayout).Attachment(0); shown {
		t.Error("an attachment on a line past the limit is shown")
	}
}

func TestSpanBackgroundsBoxEachLineWithoutHangingSpaces(t *testing.T) {
	w := testWindow()
	style := driver.TextStyle{Font: driver.Font{Family: "Segoe UI", Size: 16, Weight: 400}, Color: driver.RGB(0)}
	text := "aa bbbb cccc dd"
	bg := driver.TextBackground{Color: driver.RGB(0xff0000), Radius: 100}
	at := &driver.AttributedText{Text: text, Style: style,
		Spans: []driver.TextSpan{{Start: 0, End: 3, Style: style}, {Start: 3, End: 13, Style: style, Background: bg},
			{Start: 13, End: len(text), Style: style}}}
	p := w.Prepare(at)
	defer p.Release()
	one := p.Measure(driver.Auto)
	l := p.Layout(one).(*textLayout)
	if len(l.backgrounds) != 1 {
		t.Fatalf("%d background boxes on one line, want 1", len(l.backgrounds))
	}
	b := l.backgrounds[0]
	m := metricsOf(style.Font)
	height := (m.ascent + m.descent) * 16
	if math.Abs(float64(b.r.bottom-b.r.top-height)) > 0.01 || b.r.left <= 0 || b.radius != (b.r.bottom-b.r.top)/2 {
		t.Errorf("box = %+v, want the font's height %v after the first word, radius half of it", b, height)
	}
	l.Release()
	// Wrapped after "bbbb ", the first line's box ends at its ink.
	narrow := p.Measure(driver.Points(b.r.left + (float32(one.Width)-b.r.left)/2))
	l = p.Layout(narrow).(*textLayout)
	defer l.Release()
	if len(l.lines) < 2 || len(l.backgrounds) != 2 {
		t.Fatalf("%d lines, %d boxes; want a box on each of 2 lines", len(l.lines), len(l.backgrounds))
	}
	first := l.lines[0]
	_, ink := inkWidth(l.layout, first.start, first.end-first.trailing-first.newline)
	if b := l.backgrounds[0]; b.r.right > ink+0.01 {
		t.Errorf("the first line's box ends at %v, past its last word's end %v", b.r.right, ink)
	}
}

// TestPlainTextMeasuresTheLinesItDraws is a table header's case: bold
// text with a custom line height, wrapped at a narrow width. Measure and
// Layout must break it into the same lines, or the row reserves less than
// the text draws.
func TestPlainTextMeasuresTheLinesItDraws(t *testing.T) {
	w := testWindow()
	style := driver.TextStyle{Font: driver.Font{Family: "Segoe UI", Size: 13, Weight: driver.FontWeightSemibold},
		Color: driver.RGB(0), LineHeight: 21.7}
	p := w.Prepare(&driver.AttributedText{Text: "Reaches the system through", Style: style})
	defer p.Release()
	natural := p.Measure(driver.Auto)
	narrow := driver.Points(float64(natural.Width) * 0.55)
	size := p.Measure(narrow)
	l := p.Layout(size).(*textLayout)
	defer l.Release()
	if len(l.lines) < 2 {
		t.Fatalf("%v wide text drew %d line(s) at %v, want at least 2", natural.Width, len(l.lines), narrow)
	}
	if want := driver.Points(math.Ceil(float64(len(l.lines)) * 21.7)); size.Height != want {
		t.Errorf("measured height %v, want %v for %d lines of 21.7", size.Height, want, len(l.lines))
	}
	for i := 1; i < len(l.lines); i++ {
		if l.lines[i].top < l.lines[i-1].top+l.lines[i-1].height-0.01 {
			t.Errorf("line %d starts at %v, before the last ends", i, l.lines[i].top)
		}
	}
}

// TestFallbackFontsDrawMissingGlyphs checks that DirectWrite sets a rune
// the text's font lacks in a family that has it, so it takes that
// family's width rather than the missing-glyph box's.
func TestFallbackFontsDrawMissingGlyphs(t *testing.T) {
	w := testWindow()
	style := driver.TextStyle{Font: driver.Font{Family: "Segoe UI", Size: 32}, Color: driver.RGB(0)}
	for _, c := range []struct{ text, family string }{
		{"😀", "Segoe UI Emoji"},
		{"中", "Microsoft YaHei UI"},
	} {
		p := w.Prepare(&driver.AttributedText{Text: c.text, Style: style})
		got := p.Measure(driver.Auto).Width
		p.Release()
		fallback := style
		fallback.Font.Family = c.family
		q := w.Prepare(&driver.AttributedText{Text: c.text, Style: fallback})
		want := q.Measure(driver.Auto).Width
		q.Release()
		if got != want || got == 0 {
			t.Errorf("%q is %v wide, the %s glyph is %v", c.text, got, c.family, want)
		}
	}
}

// TestSingleLineTextTruncatesInsteadOfWrapping checks MaxLines 1 keeps
// the text on one line, trimmed to the box's width.
func TestSingleLineTextTruncatesInsteadOfWrapping(t *testing.T) {
	w := testWindow()
	style := driver.TextStyle{Font: driver.Font{Family: "Segoe UI", Size: 16}, Color: driver.RGB(0)}
	at := &driver.AttributedText{Text: "one two three four", Style: style, MaxLines: 1}
	p := w.Prepare(at)
	defer p.Release()
	full := p.Measure(driver.Auto)
	two := w.Prepare(&driver.AttributedText{Text: "one two", Style: style})
	twoSize := two.Measure(driver.Auto)
	two.Release()
	narrow := driver.Points(float64(twoSize.Width) - 1)
	size := p.Measure(narrow)
	if size.Height != full.Height || size.Width > narrow {
		t.Errorf("size %v at %v, want one line's height %v within the width", size, narrow, full.Height)
	}
	l := p.Layout(size).(*textLayout)
	defer l.Release()
	if len(l.lines) != 1 {
		t.Fatalf("%d lines, want one", len(l.lines))
	}
	var m textMetrics
	l.layout.call(layoutGetMetrics, uintptr(unsafe.Pointer(&m)))
	if m.lineCount != 1 || m.width > float32(size.Width)+0.5 {
		t.Errorf("the line is %v wide in %d line(s), want it trimmed to %v", m.width, m.lineCount, size.Width)
	}
}

// TestIndexAtFindsPlainTextOffsets checks that a press on plain text maps
// to the UTF-8 offset under it, as a press on spans does.
func TestIndexAtFindsPlainTextOffsets(t *testing.T) {
	w := testWindow()
	style := driver.TextStyle{Font: driver.Font{Family: "Segoe UI", Size: 16}, Color: driver.RGB(0)}
	p := w.Prepare(&driver.AttributedText{Text: "ab cd", Style: style})
	defer p.Release()
	l := p.Layout(p.Measure(driver.Auto)).(*textLayout)
	defer l.Release()
	y := l.lines[0].baseline - 2
	if got := l.indexAt(1, y); got != 0 {
		t.Errorf("press at the first character = %d, want 0", got)
	}
	_, a := inkWidth(l.layout, 0, 1)
	if got := l.indexAt(a+1, y); got != 1 {
		t.Errorf("press at the second character = %d, want 1", got)
	}
	_, space := inkWidth(l.layout, 0, 3)
	if got := l.indexAt(space+1, y); got != 3 {
		t.Errorf("press at the second word = %d, want 3", got)
	}
	if got := l.indexAt(1, y+100); got != -1 {
		t.Errorf("press below the text = %d, want -1", got)
	}
}

// TestTextEllipsizesJustifiesAndReportsLines checks the head and middle
// cuts, clipping, justified lines, decoration styles and the lines
// OnTextLayout reports.
func TestTextEllipsizesJustifiesAndReportsLines(t *testing.T) {
	w := testWindow()
	style := driver.TextStyle{Font: driver.Font{Family: "Segoe UI", Size: 16}, Color: driver.RGB(0)}
	text := "Lorem ipsum dolor sit amet, consectetur adipiscing elit"
	for _, mode := range []driver.EllipsizeMode{driver.EllipsizeHead, driver.EllipsizeMiddle} {
		p := w.Prepare(&driver.AttributedText{Text: text, Style: style, MaxLines: 1, Ellipsize: mode})
		size := p.Measure(120)
		l := p.Layout(size).(*textLayout)
		lines := l.TextLines()
		if len(lines) != 1 || lines[0].Bounds.Size.Width > 120.5 {
			t.Errorf("mode %v: lines %+v, want one within 120", mode, lines)
		}
		var m textMetrics
		l.layout.call(layoutGetMetrics, uintptr(unsafe.Pointer(&m)))
		if m.width > 120.5 {
			t.Errorf("mode %v: the cut line is %v wide", mode, m.width)
		}
		l.Release()
		p.Release()
	}
	p := w.Prepare(&driver.AttributedText{Text: text, Style: style, MaxLines: 1, Ellipsize: driver.EllipsizeClip})
	l := p.Layout(p.Measure(120)).(*textLayout)
	if l.clip.empty() || l.clip.right != 120 {
		t.Errorf("clip = %v, want the box", l.clip)
	}
	l.Release()
	p.Release()

	justified := style
	justified.Align = driver.TextAlignJustify
	justified.Decoration, justified.DecorationStyle, justified.DecorationColor =
		driver.TextDecorationUnderline, driver.DecorationWavy, driver.RGB(0xff0000)
	p = w.Prepare(&driver.AttributedText{Text: "aa bb cc dd ee ff gg hh ii jj kk ll mm nn oo pp", Style: justified})
	defer p.Release()
	l = p.Layout(driver.Size{Width: 120, Height: p.Measure(120).Height}).(*textLayout)
	defer l.Release()
	lines := l.TextLines()
	if len(lines) < 2 || lines[1].Start != lines[0].End {
		t.Fatalf("lines = %+v, want several in a row", lines)
	}
	if w := lines[0].Bounds.Size.Width; w < 118 {
		t.Errorf("the justified first line is %v wide, want 120", w)
	}
	if lines[0].CapHeight <= lines[0].XHeight || lines[0].XHeight <= 0 {
		t.Errorf("metrics %+v", lines[0])
	}
	if len(l.decorations) == 0 || l.decorations[0].style != driver.DecorationWavy || l.decorations[0].color != driver.RGB(0xff0000) {
		t.Errorf("decorations = %+v, want red and wavy", l.decorations)
	}
}

func editModelOf(text string) *editModel {
	m := &editModel{}
	m.setText(text)
	return m
}

func TestEditModelEditsAsNSHs(t *testing.T) {
	eq := func(got, want any) {
		t.Helper()
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}
	s := "a😀b"
	eq(utf16Len(s), 4)
	eq(utf16ToByte(s, 2), 5)
	eq(utf16ToByte(s, 3), 5)
	eq(utf16ToByte(s, 99), 6)
	// Values keep the caret, at the end or on a boundary.
	m := editModelOf("hello")
	m.setText("hello world")
	eq(m.caret, 11)
	m.selectRange(2, 2)
	m.setText("héllo")
	eq(m.caret, 1)
	// Typing and deleting.
	m = editModelOf("")
	for _, c := range []string{"a", "b", "c"} {
		m.insert(c)
	}
	eq(m.delete(unitChar, false), true)
	eq(m.text, "ab")
	m.moveTo(0, false)
	eq(m.delete(unitChar, false), false)
	eq(m.delete(unitChar, true), true)
	eq(m.text, "b")
	m = editModelOf("one two three")
	m.delete(unitWord, false)
	eq(m.text, "one two ")
	m.moveTo(0, false)
	m.delete(unitWord, true)
	eq(m.text, "two ")
	// Undo merges runs of typing and of deleting.
	m = editModelOf("")
	for _, c := range []string{"h", "i", " ", "y", "o"} {
		m.insert(c)
	}
	m.delete(unitChar, false)
	m.delete(unitChar, false)
	eq(m.undo(), true)
	eq(m.text, "hi yo")
	eq(m.undo(), true)
	eq(m.text, "")
	eq(m.undo(), false)
	eq(m.redo(), true)
	eq(m.redo(), true)
	eq(m.text, "hi ")
	eq(m.redo(), false)
	m = editModelOf("")
	m.insert("a")
	m.moveTo(1, false)
	m.insert("c")
	m.insert("pasted")
	m.undo()
	eq(m.text, "ac")
	// Clusters, but Backspace takes a code point.
	family := "a👨‍👩‍👧b"
	eq(nextCluster(family, 1), len(family)-1)
	eq(prevCluster(family, len(family)-1), 1)
	eq(nextCluster("🇨🇳🇯🇵", 0), 8)
	m = editModelOf("é")
	m.delete(unitChar, false)
	eq(m.text, "e")
	// Words.
	w := "hello, world  foo"
	eq(wordRight(w, 0), 5)
	eq(wordRight(w, 7), 14)
	eq(wordLeft(w, 14), 7)
	eq(wordLeft(w, 5), 0)
	start, end := wordAt(w, 8)
	eq(start, 7)
	eq(end, 12)
	start, end = wordAt("你好world", 0)
	eq(start, 0)
	eq(end, 6)
	// A selection collapses to the end a step goes to.
	m = editModelOf("abcdef")
	m.selectRange(1, 4)
	eq(m.selected(), "bcd")
	m.step(unitChar, false, false)
	eq(m.caret, 1)
	eq(m.anchor, 1)
	// The composition shows at the caret; a mask shows one bullet a
	// character.
	m = editModelOf("ab")
	m.moveTo(1, false)
	m.composition = &imeText{text: "ni", cursor: 1}
	eq(m.display(), "anib")
	eq(m.toDisplay(2, false), uint32(4))
	eq(m.displayCaret(false), uint32(2))
	eq(m.textAt(2, false), 1)
	eq(m.textAt(4, false), 2)
	m = editModelOf("a😀b")
	eq(m.toDisplay(5, true), uint32(2))
	eq(m.textAt(2, true), 5)
	eq(pasted("a\r\nb\r\n"), "a b")
}

func TestInputHelpers(t *testing.T) {
	if s, e, ok := targetClause([]byte{0, 0, 1, 1, 2}); !ok || s != 2 || e != 4 {
		t.Fatalf("clause = %d, %d, %v", s, e, ok)
	}
	if s, e, ok := targetClause([]byte{3, 0}); !ok || s != 0 || e != 1 {
		t.Fatalf("clause = %d, %d, %v", s, e, ok)
	}
	if _, _, ok := targetClause([]byte{0, 2}); ok {
		t.Fatal("a clause without a target")
	}
	if got := revealAxis(0, 100, 150, 30); got != 80 {
		t.Fatalf("reveal below = %v", got)
	}
	if got := revealAxis(100, 100, 50, 30); got != 50 {
		t.Fatalf("reveal above = %v", got)
	}
	if got := revealAxis(40, 100, 50, 30); got != 40 {
		t.Fatalf("reveal shown = %v", got)
	}
	if xAlign(driver.TextAlignCenter) != 0.5 || xAlign(driver.TextAlignEnd) != 1 || xAlign(0) != 0 {
		t.Fatal("xAlign")
	}
}

// TestFocusFollowsModalsAndTab checks the focus without a window: Tab
// goes around the fields of the newest modal, and a modal takes the focus
// from a field under it until it goes.
func TestFocusFollowsModalsAndTab(t *testing.T) {
	w := &window{root: 1}
	visible := driver.ViewStyle{Opacity: 1}
	mk := func(id driver.NodeID, kind driver.ViewKind, parent *view) *view {
		v := &view{id: id, kind: kind, style: visible, parent: parent}
		if kind == driver.KindInput {
			v.field = newTextInput(w, v)
		}
		if parent != nil {
			parent.children = append(parent.children, v)
		}
		for len(w.views) <= int(id) {
			w.views = append(w.views, nil)
		}
		w.views[id] = v
		return v
	}
	root := mk(1, driver.KindView, nil)
	a := mk(2, driver.KindInput, root)
	b := mk(3, driver.KindInput, root)
	if !w.tab(false) || w.focus != a.field || !w.tab(false) || w.focus != b.field || !w.tab(false) || w.focus != a.field {
		t.Fatal("Tab does not go around the fields")
	}
	if !w.tab(true) || w.focus != b.field {
		t.Fatal("Shift+Tab does not go back")
	}
	modal := mk(4, driver.KindModal, root)
	c := mk(5, driver.KindInput, modal)
	w.checkFocus()
	if w.focus != nil || w.restore != b.field {
		t.Fatalf("the modal left the focus at %v", w.focus)
	}
	if !w.tab(false) || w.focus != c.field || !w.tab(false) || w.focus != c.field {
		t.Fatal("Tab left the modal")
	}
	w.setFocus(nil)
	w.restore = b.field
	modal.style.Hidden = true
	w.checkFocus()
	if w.focus != b.field {
		t.Fatal("the field did not get the focus back after the modal")
	}
}

// TestFileURLsEscapeAndKeepHosts checks the URLs of dropped files.
func TestFileURLsEscapeAndKeepHosts(t *testing.T) {
	for path, want := range map[string]string{
		`C:\a b\c#d.txt`:     "file:///C:/a%20b/c%23d.txt",
		`D:\日本`:              "file:///D:/%E6%97%A5%E6%9C%AC",
		`\\server\share\x y`: "file://server/share/x%20y",
		`relative\x`:         "",
	} {
		if got := fileURL(path); got != want {
			t.Errorf("fileURL(%q) = %q, want %q", path, got, want)
		}
	}
}

// TestPictureFramesFollowTheirDelaysAndLoops checks which frame an
// animation shows when.
func TestPictureFramesFollowTheirDelaysAndLoops(t *testing.T) {
	ms := time.Millisecond
	pic := &picture{frames: []imagedecode.Frame{{Delay: 100 * ms}, {Delay: 50 * ms}}, plays: 2}
	for _, c := range []struct {
		at    time.Duration
		frame int
		more  bool
	}{{0, 0, true}, {99 * ms, 0, true}, {100 * ms, 1, true}, {160 * ms, 0, true}, {260 * ms, 1, true},
		{300 * ms, 1, false}, {time.Hour, 1, false}} {
		if frame, more := pic.frameAt(c.at); frame != c.frame || more != c.more {
			t.Errorf("at %v: frame %d, more %v; want %d, %v", c.at, frame, more, c.frame, c.more)
		}
	}
	pic.plays = 0
	if frame, more := pic.frameAt(time.Hour + 120*ms); frame != 1 || !more {
		t.Errorf("a looping animation shows frame %d (more %v) after an hour", frame, more)
	}
}

// TestLayoutAnimationKeepsRemovedViewsDrawnApartFromTheirIDs applies
// batches without a window: a Delete animation keeps a removed subtree
// drawn under its parent while its IDs serve new views, an Update moves
// a frame from the one shown, and the end destroys what went.
func TestLayoutAnimationKeepsRemovedViewsDrawnApartFromTheirIDs(t *testing.T) {
	w := &window{animator: driver.NewAnimator(func(driver.Event) {}), drops: map[*view]bool{}, hosts: map[*component]bool{}}
	frame := func(id driver.NodeID, x, y, width, height driver.Points) driver.MountItem {
		return driver.MountItem{Kind: driver.ItemFrame, ID: id, Frame: driver.Bounds{Origin: driver.Point{X: x, Y: y},
			Size: driver.Size{Width: width, Height: height}}}
	}
	w.apply(&driver.MountBatch{Root: 1, Items: []driver.MountItem{
		{Kind: driver.ItemCreate, ID: 1, View: driver.KindView},
		{Kind: driver.ItemCreate, ID: 2, View: driver.KindView},
		{Kind: driver.ItemCreate, ID: 3, View: driver.KindView},
		{Kind: driver.ItemCreate, ID: 4, View: driver.KindView},
		{Kind: driver.ItemInsert, ID: 2, Parent: 1},
		{Kind: driver.ItemInsert, ID: 3, Parent: 2},
		{Kind: driver.ItemInsert, ID: 4, Parent: 1},
		frame(1, 0, 0, 300, 300), frame(2, 0, 0, 100, 100), frame(3, 10, 10, 20, 20), frame(4, 0, 100, 50, 50),
	}})
	root, old, child, mover := w.views[1], w.views[2], w.views[3], w.views[4]
	tr := &driver.LayoutTransition{Duration: time.Second}
	w.apply(&driver.MountBatch{Root: 1, LayoutAnimation: &driver.LayoutAnimation{Update: tr, Delete: tr, Create: tr},
		Items: []driver.MountItem{
			{Kind: driver.ItemDelete, ID: 2},
			{Kind: driver.ItemDelete, ID: 3},
			{Kind: driver.ItemCreate, ID: 2, View: driver.KindView},
			{Kind: driver.ItemInsert, ID: 2, Parent: 1, Before: 4},
			frame(2, 0, 0, 100, 40), frame(4, 0, 40, 50, 50),
		}})
	fresh := w.views[2]
	switch {
	case fresh == old || fresh == nil || w.views[3] != nil:
		t.Fatal("the deleted IDs do not name the new views")
	case len(root.leaving) != 1 || root.leaving[0].v != old || root.leaving[0].at != 0:
		t.Fatalf("the root keeps %v going, want the old view at 0", root.leaving)
	case !old.retired || !child.retired || len(old.children) != 1 || old.children[0] != child:
		t.Fatal("the subtree going is not kept whole")
	case old.fading == nil || !old.fading.out:
		t.Fatal("the old view does not go out")
	case fresh.fading == nil || fresh.fading.out:
		t.Fatal("the new view does not come in")
	case mover.moving == nil || mover.moving.from.Origin.Y != 100 || frameOf(mover).Origin.Y != 100 || mover.frame.Origin.Y != 40:
		t.Fatalf("the moved view moves from %v to %v", frameOf(mover), mover.frame)
	}
	w.finishLeaving(old)
	if len(root.leaving) != 0 || !old.deleted || old.retired || !child.deleted || w.views[2] != fresh {
		t.Fatal("the end of the animation did not destroy only what went")
	}
	// Without an animation a frame snaps, and a delete is immediate.
	w.apply(&driver.MountBatch{Root: 1, Items: []driver.MountItem{frame(4, 0, 0, 50, 50), {Kind: driver.ItemDelete, ID: 2}}})
	if mover.moving != nil || frameOf(mover).Origin.Y != 0 || len(root.leaving) != 0 || !fresh.deleted {
		t.Fatal("a batch without a layout animation animated")
	}
}

// TestFrameTransitionMovesOnlyItsView moves a view by its frame item's
// own transition, and lets a batch's Update win over it.
func TestFrameTransitionMovesOnlyItsView(t *testing.T) {
	w := &window{animator: driver.NewAnimator(func(driver.Event) {}), drops: map[*view]bool{}, hosts: map[*component]bool{}}
	frame := func(id driver.NodeID, width driver.Points, tr *driver.LayoutTransition) driver.MountItem {
		return driver.MountItem{Kind: driver.ItemFrame, ID: id, Frame: driver.Bounds{Size: driver.Size{Width: width, Height: 10}},
			Transition: tr}
	}
	w.apply(&driver.MountBatch{Root: 1, Items: []driver.MountItem{
		{Kind: driver.ItemCreate, ID: 1, View: driver.KindView},
		{Kind: driver.ItemCreate, ID: 2, View: driver.KindView},
		{Kind: driver.ItemCreate, ID: 3, View: driver.KindView},
		{Kind: driver.ItemInsert, ID: 2, Parent: 1},
		{Kind: driver.ItemInsert, ID: 3, Parent: 1},
		frame(1, 300, nil), frame(2, 20, nil), frame(3, 20, nil),
	}})
	bar, other := w.views[2], w.views[3]
	own := driver.Transition{Duration: time.Second, Easing: driver.EaseOut}.Layout()
	w.apply(&driver.MountBatch{Root: 1, Items: []driver.MountItem{frame(2, 200, own), frame(3, 200, nil)}})
	switch {
	case bar.moving == nil || bar.moving.tr != *own || frameOf(bar).Size.Width != 20 || bar.frame.Size.Width != 200:
		t.Fatalf("the bar moves %+v from %v", bar.moving, frameOf(bar))
	case other.moving != nil || frameOf(other).Size.Width != 200:
		t.Fatal("a frame without a transition animated")
	}
	update := &driver.LayoutTransition{Duration: 2 * time.Second}
	w.apply(&driver.MountBatch{Root: 1, LayoutAnimation: &driver.LayoutAnimation{Update: update},
		Items: []driver.MountItem{frame(2, 100, own)}})
	if bar.moving == nil || bar.moving.tr != *update {
		t.Fatalf("the batch's Update did not win: %+v", bar.moving)
	}
}

func TestTextLayoutsHitTestTheirCharacters(t *testing.T) {
	w := testWindow()
	style := driver.TextStyle{Font: driver.Font{Family: "Segoe UI", Size: 16}, Color: driver.RGB(0)}
	layout := func(text string) *textLayout {
		p := w.Prepare(&driver.AttributedText{Text: text, Style: style})
		t.Cleanup(p.Release)
		l := p.Layout(p.Measure(driver.Auto)).(*textLayout)
		t.Cleanup(l.Release)
		return l
	}
	l := layout("abc def\nghi")
	lines := l.TextLines()
	if len(lines) != 2 {
		t.Fatalf("%d lines, want 2", len(lines))
	}
	first := lines[0].Bounds
	if c := l.HitTest(driver.Point{X: first.Size.Width + 50, Y: first.Origin.Y + 2}); c.Offset != 7 {
		t.Fatalf("past the first line: %+v, want 7", c)
	}
	if c := l.HitTest(driver.Point{X: -5, Y: first.Size.Height + 4}); c.Offset != 8 {
		t.Fatalf("the second line's start: %+v, want 8", c)
	}
	a, ok := l.CaretRect(driver.Caret{Offset: 1})
	b, _ := l.CaretRect(driver.Caret{Offset: 2})
	if !ok || a.Origin.X <= 0 || b.Origin.X <= a.Origin.X {
		t.Fatalf("carets %v and %v should step right", a, b)
	}
	if rects := l.RangeRects(5, 9, nil); len(rects) != 2 || rects[1].Origin.Y <= rects[0].Origin.Y {
		t.Fatalf("a range across the break covers %v", rects)
	}
	if r := layout("ab \u05d0\u05d1\u05d2 cd").RangeRects(1, 5, nil); len(r) != 2 {
		t.Fatalf("a range from b into the Hebrew covers %v, want two boxes", r)
	}
}

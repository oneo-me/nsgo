//go:build linux && !android

package linux

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
)

// The tests below open windows, as native_test.go's do, and keep what
// they read in the window's top left 130 × 130 points.

// boundsOf returns the surface bounds of the view with testID.
func (w *window) boundsOf(t *testing.T, testID string) driver.Bounds {
	t.Helper()
	var b driver.Bounds
	found := false
	w.onLoop(t, func() {
		for _, v := range w.views {
			if v != nil && v.props.TestID == testID {
				b, _ = w.surfaceBounds(v)
				found = true
			}
		}
	})
	if !found {
		t.Fatalf("no view %q", testID)
	}
	return b
}

// pressView presses the middle of the view with testID.
func (w *window) pressView(t *testing.T, testID string) {
	t.Helper()
	b := w.boundsOf(t, testID)
	at := driver.Point{X: b.Origin.X + b.Size.Width/2, Y: b.Origin.Y + b.Size.Height/2}
	w.onLoop(t, func() { w.press(at) })
}

func abs(left, top, width, height driver.Points, bg driver.Color) core.Style {
	return core.Style{Position: core.Absolute, Left: driver.Pt(left), Top: driver.Pt(top),
		Width: driver.Pt(width), Height: driver.Pt(height), Background: bg}
}

func isColor(p []byte, rgb uint32) bool {
	return p[3] == 0xff && p[2] == byte(rgb>>16) && p[1] == byte(rgb>>8) && p[0] == byte(rgb)
}

// firstX returns the first x in row y whose pixel is rgb, or -1.
func firstX(p pixels, y int, rgb uint32) int {
	for x := 0; x < min(p.width, 130); x++ {
		if isColor(p.at(x, y), rgb) {
			return x
		}
	}
	return -1
}

// fileListValue returns a GValue holding a GdkFileList of paths, as a
// drop of files from another app hands the drop target, and frees it.
func fileListValue(paths []string) (*[3]uint64, func()) {
	files := make([]uintptr, len(paths))
	for i, p := range paths {
		files[i] = gFileNewForPath(cstr(p))
	}
	list := gdkFileListNewFromArray(&files[0], uintptr(len(files)))
	for _, f := range files {
		gObjectUnref(f)
	}
	value := new([3]uint64) // GValue: a GType and two words
	gValueInit(uintptr(unsafe.Pointer(value)), gdkFileListGetType())
	gValueTakeBoxed(uintptr(unsafe.Pointer(value)), list)
	return value, func() { gValueUnset(uintptr(unsafe.Pointer(value))) }
}

// TestFileDropFindsTheDestinationUnderThePointer drags files over nested,
// disabled and missing destinations through the surface's drop target
// and drops them: the nearest enabled view taking files above the one
// under the pointer gets them as escaped file: URLs.
func TestFileDropFindsTheDestinationUnderThePointer(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	dir := t.TempDir()
	paths := []string{filepath.Join(dir, "a b.txt"), filepath.Join(dir, "é.txt"), filepath.Join(dir, "folder")}
	for _, p := range paths[:2] {
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(paths[2], 0o755); err != nil {
		t.Fatal(err)
	}
	title := "nsgo-native-" + t.Name()
	app := func(cx *core.Scope) core.Node {
		log := core.NewSignal(cx, "")
		show := core.NewSignal(cx, true)
		add := func(s string) {
			log.Update(func(l string) string {
				if l == "" {
					return s
				}
				return l + "," + s
			})
		}
		cx.Effect(func() { cx.SetTitle(title + "|" + log.Get()) })
		destination := func(name string, style core.Style, disabled bool, children ...core.Node) core.Node {
			return core.View{TestID: name, Style: style, Children: children,
				Disabled:        func() bool { return disabled },
				OnFileDragEnter: func() { add("enter " + name) },
				OnFileDragLeave: func() { add("leave " + name) },
				OnFileDrop:      func(uris []string) { add("drop " + name + " " + strings.Join(uris, " ")) }}
		}
		return core.View{Style: core.Style{Width: driver.Pt(360), Height: driver.Pt(240), Background: driver.RGB(0xffffff)},
			Children: []core.Node{
				core.Show{When: show.Get, Then: func(cx *core.Scope) core.Node {
					return core.View{Style: abs(0, 0, 130, 65, 0), Children: []core.Node{
						destination("a", abs(0, 0, 60, 60, driver.RGB(0xeeeeee)), false,
							core.View{TestID: "inner", Style: abs(10, 10, 20, 20, driver.RGB(0xcccccc))}),
						destination("off", abs(70, 0, 50, 50, driver.RGB(0xdddddd)), true),
					}}
				}},
				core.View{TestID: "remove", OnPress: func(core.PressEvent) { show.Set(false) }, Style: abs(70, 70, 40, 40, driver.RGB(0x000000))},
			}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 360, Height: 240})
	waitForTitle(t, w, title+"|")
	var installed bool
	w.onLoop(t, func() { installed = w.drop.target != 0 })
	if !installed {
		t.Fatal("no drop target while views take files")
	}

	steps := []struct {
		at   driver.Point
		want uint32
	}{
		{driver.Point{X: 20, Y: 20}, gdkActionCopy}, // on the inner view: its parent takes them
		{driver.Point{X: 50, Y: 50}, gdkActionCopy}, // the same destination: no new events
		{driver.Point{X: 90, Y: 20}, 0},             // a disabled destination
		{driver.Point{X: 20, Y: 100}, 0},            // nothing takes files there
		{driver.Point{X: 5, Y: 5}, gdkActionCopy},
	}
	for _, s := range steps {
		var got uint32
		w.onLoop(t, func() { got = dropMotionCallback(0, float64(s.at.X), float64(s.at.Y), w.handle) })
		if got != s.want {
			t.Fatalf("action at %v = %d, want %d", s.at, got, s.want)
		}
	}
	waitForTitle(t, w, title+"|enter a,leave a,enter a")

	var dropped int32
	w.onLoop(t, func() {
		value, free := fileListValue(paths)
		defer free()
		dropped = dropCallback(0, uintptr(unsafe.Pointer(value)), 20, 20, w.handle)
	})
	if dropped == 0 {
		t.Fatal("the drop was refused")
	}
	want := fmt.Sprintf("drop a file://%s/a%%20b.txt file://%s/%%C3%%A9.txt file://%s/folder", dir, dir, dir)
	waitForTitle(t, w, title+"|enter a,leave a,enter a,"+want+",leave a")

	// Nowhere to drop: refused.
	w.onLoop(t, func() {
		value, free := fileListValue(paths)
		defer free()
		dropped = dropCallback(0, uintptr(unsafe.Pointer(value)), 20, 100, w.handle)
	})
	if dropped != 0 {
		t.Fatal("a drop with no destination was taken")
	}

	// The last destination going removes the drop target.
	w.pressView(t, "remove")
	deadline := time.Now().Add(5 * time.Second)
	for installed {
		if time.Now().After(deadline) {
			t.Fatal("the drop target stayed after its views went")
		}
		time.Sleep(20 * time.Millisecond)
		w.onLoop(t, func() { installed = w.drop.target != 0 || w.drop.views != 0 })
	}
}

// TestFileURLsEscapeTheirPaths checks the file: URLs drops deliver.
func TestFileURLsEscapeTheirPaths(t *testing.T) {
	for path, want := range map[string]string{
		"/tmp/a b.txt":      "file:///tmp/a%20b.txt",
		"/tmp/é/ü.png":      "file:///tmp/%C3%A9/%C3%BC.png",
		"/tmp/100%.txt":     "file:///tmp/100%25.txt",
		"/tmp/q?#.txt":      "file:///tmp/q%3F%23.txt",
		"/home/user/folder": "file:///home/user/folder",
	} {
		if got := fileURL(path); got != want {
			t.Errorf("fileURL(%q) = %q, want %q", path, got, want)
		}
	}
}

// swatch is a native component the tests register: a GTK box painted
// green, which records its props and echoes its commands from another
// goroutine.
type swatch struct {
	cx     *driver.NativeContext
	widget uintptr
	props  map[string]any
	closed *atomic.Bool
}

func (s *swatch) View() any                      { return s.widget }
func (s *swatch) SetProp(name string, value any) { s.props[name] = value }
func (s *swatch) Command(name string, args any)  { go s.cx.Emit(name, args) }
func (s *swatch) Close()                         { s.closed.Store(true) }

// TestNativeComponentFillsItsView hosts an app's component in a view:
// its widget follows the view's frame, takes its props and commands,
// talks back from any goroutine and closes with the view. An unknown
// component leaves an empty box.
func TestNativeComponentFillsItsView(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	var made atomic.Pointer[swatch]
	closed := new(atomic.Bool)
	components := driver.NativeComponents{"swatch": func(cx *driver.NativeContext) driver.NativeComponent {
		box := gtkBoxNew(0, 0)
		css := gtkCssProviderNew()
		gtkCssProviderLoadFromString(css, cstr("box { background-color: #00ff00; }"))
		gtkStyleContextAddProvider(gtkWidgetGetStyleContext(box), css, 800)
		gObjectUnref(css)
		s := &swatch{cx: cx, widget: box, props: map[string]any{}, closed: closed}
		made.Store(s)
		cx.SetSize(driver.Size{Width: 1, Height: 1})
		return s
	}}
	title := "nsgo-native-" + t.Name()
	app := func(cx *core.Scope) core.Node {
		wide := core.NewSignal(cx, false)
		show := core.NewSignal(cx, true)
		event := core.NewSignal(cx, "")
		var ref core.Ref
		cx.Effect(func() { cx.SetTitle(title + "|" + event.Get()) })
		return core.View{Style: core.Style{Width: driver.Pt(360), Height: driver.Pt(240), Padding: driver.All(10),
			Background: driver.RGB(0xffffff)},
			Children: []core.Node{
				core.Show{When: show.Get, Then: func(cx *core.Scope) core.Node {
					return core.Native{Component: "swatch", Ref: &ref, TestID: "swatch",
						PropsFunc: func() map[string]any { return map[string]any{"wide": wide.Get()} },
						OnEvent:   func(name string, data any) { event.Set(fmt.Sprint(name, ":", data)) },
						StyleFunc: func() core.Style {
							if wide.Get() {
								return core.Style{Width: driver.Pt(80), Height: driver.Pt(30)}
							}
							return core.Style{Width: driver.Pt(40), Height: driver.Pt(30)}
						}}
				}},
				core.Native{Component: "nsgo-test-unknown", TestID: "unknown", Style: core.Style{Width: driver.Pt(10), Height: driver.Pt(10)}},
				core.View{TestID: "grow", Style: abs(10, 60, 20, 20, driver.RGB(0x0000ff)),
					OnPress: func(core.PressEvent) { wide.Set(true); ref.Command("ping", 7) }},
				core.View{TestID: "remove", Style: abs(40, 60, 20, 20, driver.RGB(0x000000)),
					OnPress: func(core.PressEvent) { show.Set(false) }},
			}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 360, Height: 240, Components: components})
	waitForTitle(t, w, title+"|")
	isSwatch := func(v *view) bool { return v.native != nil && v.native.comp != nil }
	waitHosted(t, w, isSwatch, driver.Bounds{Origin: driver.Point{X: 10, Y: 10}, Size: driver.Size{Width: 40, Height: 30}})
	waitPixels(t, w, "the component's green in its view", func(p pixels) bool {
		return isColor(p.at(30, 25), 0x00ff00) && isColor(p.at(55, 25), 0xffffff)
	})
	var unknownHosts bool
	w.onLoop(t, func() {
		for _, v := range w.views {
			if v != nil && v.props.TestID == "unknown" {
				unknownHosts = v.native == nil || v.native.comp != nil || v.native.child != 0
			}
		}
	})
	if unknownHosts {
		t.Fatal("an unknown component hosts something")
	}

	w.pressView(t, "grow")
	waitForTitle(t, w, title+"|ping:7")
	waitHosted(t, w, isSwatch, driver.Bounds{Origin: driver.Point{X: 10, Y: 10}, Size: driver.Size{Width: 80, Height: 30}})
	var wideProp any
	w.onLoop(t, func() { wideProp = made.Load().props["wide"] })
	if wideProp != true {
		t.Fatalf("prop wide = %v, want true", wideProp)
	}
	waitPixels(t, w, "the component grown with its view", func(p pixels) bool { return isColor(p.at(80, 25), 0x00ff00) })

	w.pressView(t, "remove")
	deadline := time.Now().Add(5 * time.Second)
	for !closed.Load() {
		if time.Now().After(deadline) {
			t.Fatal("the component did not close with its view")
		}
		time.Sleep(20 * time.Millisecond)
	}
	waitPixels(t, w, "the component gone", func(p pixels) bool { return isColor(p.at(30, 25), 0xffffff) })
}

// TestAnimatorDrawsOnTheFrameClock runs a timing animation and a drag
// bound to animated values: the views move on the backend's frame clock
// with no engine work per frame, and the run's end reaches the app.
func TestAnimatorDrawsOnTheFrameClock(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-native-" + t.Name()
	app := func(cx *core.Scope) core.Node {
		x := core.NewAnimatedValue(cx, 0)
		dragged := core.NewAnimatedValue(cx, 0)
		done := core.NewSignal(cx, "")
		cx.Effect(func() { cx.SetTitle(title + "|" + done.Get()) })
		box := abs(0, 0, 20, 20, driver.RGB(0xff0000))
		box.Animated = &core.AnimatedStyle{TranslateX: x}
		handle := abs(0, 40, 20, 20, driver.RGB(0x00ff00))
		handle.Animated = &core.AnimatedStyle{TranslateX: dragged}
		return core.View{Style: core.Style{Width: driver.Pt(360), Height: driver.Pt(240), Background: driver.RGB(0xffffff)},
			Children: []core.Node{
				core.View{TestID: "box", Style: box},
				core.View{TestID: "handle", Style: handle, PanValues: core.AnimatedXY{X: dragged}},
				core.View{TestID: "go", Style: abs(0, 100, 20, 20, driver.RGB(0x0000ff)), OnPress: func(core.PressEvent) {
					cx.StartAnimation(core.Timing(x, 80, driver.TimingConfig{Duration: 1500 * time.Millisecond}),
						func(finished bool) { done.Set(fmt.Sprint(finished)) })
				}},
			}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 360, Height: 240})
	waitForTitle(t, w, title+"|")
	waitPixels(t, w, "the box at its start", func(p pixels) bool { return firstX(p, 10, 0xff0000) == 0 })

	w.pressView(t, "go")
	waitPixels(t, w, "the box on its way", func(p pixels) bool {
		x := firstX(p, 10, 0xff0000)
		return x > 15 && x < 65
	})
	waitForTitle(t, w, title+"|true")
	waitPixels(t, w, "the box at its end", func(p pixels) bool { return firstX(p, 10, 0xff0000) == 80 })
	var running bool
	w.onLoop(t, func() { running = w.animator.Running() })
	if running {
		t.Fatal("the animator runs after its animation ended")
	}

	// A drag moves the values bound to it in the frame it moves.
	w.onLoop(t, func() {
		w.panDown(driver.Point{X: 10, Y: 50})
		dragBeginCallback(0, 10, 50, w.handle)
		dragUpdateCallback(0, 50, 0, w.handle)
	})
	waitPixels(t, w, "the handle dragged 50 points", func(p pixels) bool { return firstX(p, 50, 0x00ff00) == 50 })
	w.onLoop(t, func() { dragEndCallback(0, 50, 0, w.handle) })
}

// TestLayoutTransitionWidensAView widens a bar whose style transitions
// layout and looks half way: its right edge is between its widths.
func TestLayoutTransitionWidensAView(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-native-" + t.Name()
	app := func(cx *core.Scope) core.Node {
		full := core.NewSignal(cx, false)
		cx.SetTitle(title)
		return core.View{Style: core.Style{Width: driver.Pt(360), Height: driver.Pt(240), Background: driver.RGB(0xffffff)},
			Children: []core.Node{
				core.View{StyleFunc: func() core.Style {
					s := abs(0, 0, 40, 20, driver.RGB(0xff0000))
					if full.Get() {
						s.Width = driver.Pt(240)
					}
					s.Transition = driver.Transition{Duration: 1500 * time.Millisecond, Properties: driver.TransitionLayout}
					return s
				}},
				core.View{TestID: "widen", Style: abs(0, 100, 20, 20, driver.RGB(0x000000)),
					OnPress: func(core.PressEvent) { full.Set(true) }},
			}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 360, Height: 240})
	waitForTitle(t, w, title)
	red := func(p pixels, x int) bool { return isColor(p.at(x, 10), 0xff0000) }
	waitPixels(t, w, "the narrow bar", func(p pixels) bool { return red(p, 30) && !red(p, 60) })
	w.pressView(t, "widen")
	waitPixels(t, w, "the bar half way", func(p pixels) bool { return red(p, 100) && !red(p, 220) })
	waitPixels(t, w, "the wide bar", func(p pixels) bool { return red(p, 235) && !red(p, 250) })
}

// TestLayoutAnimationMovesShowsAndRetiresViews animates one commit's
// frame change, insertion and removal, each checked half way.
func TestLayoutAnimationMovesShowsAndRetiresViews(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-native-" + t.Name()
	second := driver.LayoutTransition{Duration: 1500 * time.Millisecond}
	app := func(cx *core.Scope) core.Node {
		x := core.NewSignal(cx, driver.Points(0))
		kept, added := core.NewSignal(cx, true), core.NewSignal(cx, false)
		cx.SetTitle(title)
		return core.View{Style: core.Style{Width: driver.Pt(360), Height: driver.Pt(240), Background: driver.RGB(0xffffff)},
			Children: []core.Node{
				core.View{TestID: "box", StyleFunc: func() core.Style { return abs(x.Get(), 0, 20, 20, driver.RGB(0xff0000)) }},
				core.Show{When: kept.Get, Then: func(*core.Scope) core.Node {
					return core.View{Style: abs(0, 40, 20, 20, driver.RGB(0x0000ff))}
				}},
				core.Show{When: added.Get, Then: func(*core.Scope) core.Node {
					return core.View{Style: abs(80, 40, 40, 40, driver.RGB(0x00ff00))}
				}},
				core.View{TestID: "move", Style: abs(0, 100, 20, 20, driver.RGB(0x000000)), OnPress: func(core.PressEvent) {
					cx.AnimateLayout(driver.LayoutAnimation{Update: &second})
					x.Set(80)
				}},
				core.View{TestID: "remove", Style: abs(30, 100, 20, 20, driver.RGB(0x000000)), OnPress: func(core.PressEvent) {
					tr := second
					cx.AnimateLayout(driver.LayoutAnimation{Delete: &tr})
					kept.Set(false)
				}},
				core.View{TestID: "add", Style: abs(60, 100, 20, 20, driver.RGB(0x000000)), OnPress: func(core.PressEvent) {
					tr := second
					tr.Property = driver.LayoutScale
					cx.AnimateLayout(driver.LayoutAnimation{Create: &tr})
					added.Set(true)
				}},
			}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 360, Height: 240})
	waitForTitle(t, w, title)
	waitPixels(t, w, "the box and the view to remove", func(p pixels) bool {
		return firstX(p, 10, 0xff0000) == 0 && isColor(p.at(10, 50), 0x0000ff)
	})

	w.pressView(t, "move")
	waitPixels(t, w, "the box half way", func(p pixels) bool {
		x := firstX(p, 10, 0xff0000)
		return x > 15 && x < 65
	})
	waitPixels(t, w, "the box at its new frame", func(p pixels) bool { return firstX(p, 10, 0xff0000) == 80 })

	w.pressView(t, "remove")
	deadline := time.Now().Add(time.Second)
	for retired := false; !retired; {
		if time.Now().After(deadline) {
			t.Fatal("the removed view does not leave animated")
		}
		w.onLoop(t, func() {
			for _, v := range w.layouts {
				retired = retired || v.dying
			}
		})
	}
	waitPixels(t, w, "the removed view fading", func(p pixels) bool {
		c := p.at(10, 50)
		return c[0] == 0xff && c[2] > 0x30 && c[2] < 0xd0
	})
	waitPixels(t, w, "the removed view gone", func(p pixels) bool { return isColor(p.at(10, 50), 0xffffff) })

	w.pressView(t, "add")
	waitPixels(t, w, "the new view growing from its center", func(p pixels) bool {
		return isColor(p.at(100, 60), 0x00ff00) && isColor(p.at(83, 43), 0xffffff)
	})
	waitPixels(t, w, "the new view at its size", func(p pixels) bool { return isColor(p.at(83, 43), 0x00ff00) })
	deadline = time.Now().Add(2 * time.Second)
	for ticking := true; ticking; {
		if time.Now().After(deadline) {
			t.Fatal("the frame clock ticks on after the layout animations ended")
		}
		time.Sleep(20 * time.Millisecond)
		w.onLoop(t, func() { ticking = len(w.layouts) > 0 || w.ticking })
	}
}

// TestInputAccessoryViewsAreIgnoredOnTheDesktop mounts an input naming
// an accessory: the accessory is never shown and the input still works.
func TestInputAccessoryViewsAreIgnoredOnTheDesktop(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-native-" + t.Name()
	app := func(cx *core.Scope) core.Node {
		text := core.NewSignal(cx, "")
		cx.Effect(func() { cx.SetTitle(title + "|" + text.Get()) })
		return core.View{Style: core.Style{Width: driver.Pt(360), Height: driver.Pt(240), Background: driver.RGB(0xffffff)},
			Children: []core.Node{
				core.Input{Value: text.Get, OnChange: text.Set, InputAccessoryViewID: "bar",
					Style: core.Style{Width: driver.Pt(100), Height: driver.Pt(30)}},
				core.InputAccessoryView{ID: "bar", Children: []core.Node{
					core.View{Style: core.Style{Height: driver.Pt(40), Background: driver.RGB(0xff0000)}},
				}},
			}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 360, Height: 240})
	waitForTitle(t, w, title+"|")
	w.onLoop(t, func() {
		for entry := range w.entries {
			gtkEditableSetText(entry, cstr("typed"))
		}
	})
	waitForTitle(t, w, title+"|typed")
	p := w.surfacePixels(t)
	for y := 30; y < 130; y += 5 {
		if isColor(p.at(50, y), 0xff0000) {
			t.Fatalf("the accessory shows at y %d", y)
		}
	}
}

// animatedGIF returns a GIF of 20 × 20 frames of colors, each shown for
// delay hundredths of a second, played loops times (0: for ever).
func animatedGIF(t *testing.T, delay, loops int, colors ...color.RGBA) []byte {
	t.Helper()
	anim := &gif.GIF{LoopCount: loops}
	if loops > 0 {
		anim.LoopCount = loops - 1 // GIF counts the repeats
		if loops == 1 {
			anim.LoopCount = -1
		}
	}
	for _, c := range colors {
		frame := image.NewPaletted(image.Rect(0, 0, 20, 20), color.Palette{c})
		anim.Image = append(anim.Image, frame)
		anim.Delay = append(anim.Delay, delay)
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, anim); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// TestAnimatedImagesPlayTheirFrames shows an animated GIF that loops and
// one that plays once: each view plays its own frames on the frame
// clock, and the one that ends stays on its last frame.
func TestAnimatedImagesPlayTheirFrames(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	red, blue := color.RGBA{0xff, 0, 0, 0xff}, color.RGBA{0, 0, 0xff, 0xff}
	green, black := color.RGBA{0, 0xff, 0, 0xff}, color.RGBA{0, 0, 0, 0xff}
	loop := animatedGIF(t, 20, 0, red, blue)
	once := animatedGIF(t, 30, 1, green, black)
	title := "nsgo-native-" + t.Name()
	app := func(cx *core.Scope) core.Node {
		size := core.NewSignal(cx, "")
		cx.Effect(func() { cx.SetTitle(title + "|" + size.Get()) })
		return core.View{Style: core.Style{Width: driver.Pt(360), Height: driver.Pt(240), Background: driver.RGB(0xffffff)},
			Children: []core.Node{
				core.Image{Source: driver.ImageSource{Data: loop}, ResizeMode: driver.Stretch,
					OnLoad: func(s driver.Size) { size.Set(fmt.Sprint(s.Width, "x", s.Height)) },
					Style:  abs(0, 0, 40, 40, 0)},
				core.Image{Source: driver.ImageSource{Data: once}, ResizeMode: driver.Stretch, Style: abs(50, 0, 40, 40, 0)},
			}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 360, Height: 240})
	waitForTitle(t, w, title+"|20x20")
	for range 2 {
		waitPixels(t, w, "the red frame", func(p pixels) bool { return isColor(p.at(20, 20), 0xff0000) })
		waitPixels(t, w, "the blue frame", func(p pixels) bool { return isColor(p.at(20, 20), 0x0000ff) })
	}
	waitPixels(t, w, "the animation played once on its last frame", func(p pixels) bool {
		return isColor(p.at(70, 20), 0x000000)
	})
	var playing int
	w.onLoop(t, func() { playing = len(w.playing) })
	if playing != 1 {
		t.Fatalf("%d animations play, want the looping one", playing)
	}
	time.Sleep(400 * time.Millisecond)
	if p := w.surfacePixels(t); !isColor(p.at(70, 20), 0x000000) {
		t.Fatalf("the finished animation shows %v again", p.at(70, 20))
	}
}

// TestSVGImagesDrawSharpAndReportTheirSize shows an SVG scaled up: it is
// drawn from an oversampled raster, and OnLoad reports its size in
// points.
func TestSVGImagesDrawSharpAndReportTheirSize(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="20.4" height="10" viewBox="0 0 20 10">` +
		`<rect width="10" height="10" fill="#ff0000"/><rect x="10" width="10" height="10" fill="#0000ff"/></svg>`
	title := "nsgo-native-" + t.Name()
	app := func(cx *core.Scope) core.Node {
		size := core.NewSignal(cx, "")
		cx.Effect(func() { cx.SetTitle(title + "|" + size.Get()) })
		return core.View{Style: core.Style{Width: driver.Pt(360), Height: driver.Pt(240), Background: driver.RGB(0xffffff)},
			Children: []core.Node{core.Image{Source: driver.ImageSource{Data: []byte(svg)}, ResizeMode: driver.Stretch,
				OnLoad: func(s driver.Size) { size.Set(fmt.Sprint(s.Width, "x", s.Height)) },
				Style:  abs(0, 0, 80, 40, 0)}}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 360, Height: 240})
	waitForTitle(t, w, title+"|20x10")
	var raster [2]int
	w.onLoop(t, func() {
		for _, v := range w.views {
			if v != nil && v.kind == driver.KindImage && v.source.pic != nil {
				raster = [2]int{v.source.pic.width, v.source.pic.height}
			}
		}
	})
	if raster[0] < 80 || raster[1] < 40 {
		t.Fatalf("raster %v, want it oversampled", raster)
	}
	waitPixels(t, w, "the red and blue halves with a sharp edge", func(p pixels) bool {
		return isColor(p.at(20, 20), 0xff0000) && isColor(p.at(60, 20), 0x0000ff) &&
			isColor(p.at(38, 20), 0xff0000) && isColor(p.at(41, 20), 0x0000ff)
	})
}

// halves returns a PNG of 20 by 10 pixels, red on the left, blue on the
// right.
func halves(t *testing.T) []byte {
	m := image.NewRGBA(image.Rect(0, 0, 20, 10))
	for y := range 10 {
		for x := range 20 {
			c := color.RGBA{0xff, 0, 0, 0xff}
			if x >= 10 {
				c = color.RGBA{0, 0, 0xff, 0xff}
			}
			m.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, m); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestImagesFitAtTheirPositionBlurAndFade checks the core fit with a
// position, NSH's Center at the natural size, a blur that blends the
// halves but keeps the edges opaque, a fade-in and ImageSize.
func TestImagesFitAtTheirPositionBlurAndFade(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	data := halves(t)
	title := "nsgo-native-" + t.Name()
	app := func(cx *core.Scope) core.Node {
		size := core.NewSignal(cx, "")
		cx.Effect(func() { cx.SetTitle(title + "|" + size.Get()) })
		cx.ImageSize(driver.ImageSource{Data: data}, func(px driver.Size, err error) { size.Set(fmt.Sprint(px, err)) })
		at := func(left, top, width, height driver.Points, pos driver.ObjectPosition) core.Style {
			s := abs(left, top, width, height, 0)
			s.ObjectPosition = pos
			return s
		}
		return core.View{Style: core.Style{Width: driver.Pt(360), Height: driver.Pt(240), Background: driver.RGB(0xffffff)},
			Children: []core.Node{
				// Contained at the end: 80 by 40 on the right of 120 by 40.
				core.Image{Source: driver.ImageSource{Data: data}, ResizeMode: driver.Contain,
					Style: at(0, 0, 120, 40, driver.ObjectPosition{X: core.PositionEnd})},
				// Its natural size, 20 by 10, 5 points from the top left.
				core.Image{Source: driver.ImageSource{Data: data}, ResizeMode: driver.Center,
					Style: at(0, 50, 100, 100, driver.ObjectPosition{X: driver.PositionAxis{Offset: driver.Pt(5)},
						Y: driver.PositionAxis{Offset: driver.Pt(5)}})},
				core.Image{Source: driver.ImageSource{Data: data}, ResizeMode: driver.Stretch, BlurRadius: 6,
					Style: abs(150, 0, 80, 40, 0)},
				core.Image{Source: driver.ImageSource{Data: data}, ResizeMode: driver.Stretch, FadeDuration: 300 * time.Millisecond,
					Style: abs(150, 50, 80, 40, 0)},
			}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 360, Height: 240})
	waitForTitle(t, w, title+"|"+fmt.Sprint(driver.Size{Width: 20, Height: 10}, nil))
	waitPixels(t, w, "the contained image at the end", func(p pixels) bool {
		return isColor(p.at(20, 20), 0xffffff) && isColor(p.at(50, 20), 0xff0000) && isColor(p.at(100, 20), 0x0000ff)
	})
	waitPixels(t, w, "the centered image at its natural size and position", func(p pixels) bool {
		return isColor(p.at(3, 57), 0xffffff) && isColor(p.at(8, 60), 0xff0000) && isColor(p.at(22, 60), 0x0000ff) &&
			isColor(p.at(28, 60), 0xffffff) && isColor(p.at(10, 68), 0xffffff)
	})
	waitPixels(t, w, "the blur blending the halves with opaque edges", func(p pixels) bool {
		mid, edge := p.at(190, 20), p.at(151, 1)
		return mid[3] == 0xff && mid[2] > 0x40 && mid[0] > 0x40 && edge[3] == 0xff && edge[2] > 0xc0
	})
	waitPixels(t, w, "the faded-in image", func(p pixels) bool { return isColor(p.at(160, 70), 0xff0000) })
}

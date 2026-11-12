//go:build windows

package windows

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"math"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/engine"
)

var (
	getParent       = user32.NewProc("GetParent")
	mapWindowPoints = user32.NewProc("MapWindowPoints")
)

// startWith runs app in a window with opts and returns it, closed when
// the test ends.
func startWith(t *testing.T, app engine.App, opts Options) uintptr {
	t.Helper()
	if testing.Short() {
		t.Skip("opens a window")
	}
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer close(done)
		Run(app, opts)
	}()
	// The app may retitle the window at once: find it by its class.
	var hwnd uintptr
	deadline := time.Now().Add(5 * time.Second)
	for hwnd == 0 {
		h, _, _ := findWindowW.Call(uintptr(unsafe.Pointer(wide("NsgoWindow"))), 0)
		if h != 0 && strings.HasPrefix(windowText(h), strings.SplitN(opts.Title, "|", 2)[0]+"|") {
			hwnd = h
			break
		}
		select {
		case <-done:
			t.Skip("no desktop could create the test window")
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("no window titled %q", opts.Title)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Cleanup(func() {
		postMessageW.Call(hwnd, wmClose, 0, 0)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("the window did not close")
		}
	})
	return hwnd
}

// px converts points to the system's pixels.
func px(p float64) int32 {
	scale := 1.0
	if dpi, _, _ := getDpiForSystem.Call(); dpi != 0 {
		scale = float64(dpi) / 96
	}
	return int32(math.Round(p * scale))
}

// clientColor reads client pixel x, y as #rrggbb from what the window
// painted.
func clientColor(t *testing.T, hwnd uintptr, x, y int32) uint32 {
	t.Helper()
	pix, w, h, ok := clientPixels(hwnd)
	if !ok || x >= w || y >= h {
		t.Skip("Windows cannot copy the window's pixels here")
	}
	at := (int(y)*int(w) + int(x)) * 4
	return uint32(pix[at+2])<<16 | uint32(pix[at+1])<<8 | uint32(pix[at])
}

// near reports whether two #rrggbb colors are within tolerance per
// channel.
func near(a, b uint32, tolerance int) bool {
	for shift := 0; shift < 24; shift += 8 {
		d := int(a>>shift&0xff) - int(b>>shift&0xff)
		if d < -tolerance || d > tolerance {
			return false
		}
	}
	return true
}

// waitForColor waits until what the window painted at x, y is want.
func waitForColor(t *testing.T, hwnd uintptr, x, y int32, want uint32, timeout time.Duration) uint32 {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		got := clientColor(t, hwnd, x, y)
		if near(got, want, 24) {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatalf("pixel (%d, %d) is #%06x, want #%06x", x, y, got, want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// fakeData is an IDataObject a test drags: CF_HDROP of files when files
// is not nil.
type fakeData struct {
	vtbl  *[12]uintptr
	files []string
}

const (
	eNotImpl       = 0x80004001
	dvEFormatEtc   = 0x80040064
	gmemMoveableT  = 0x0002
	dropFilesBytes = 20
)

var fakeDataVtbl = [12]uintptr{
	syscall.NewCallback(func(d *fakeData, iid *guid, out *unsafe.Pointer) uintptr {
		*out = unsafe.Pointer(d)
		return sOK
	}),
	syscall.NewCallback(func(d *fakeData) uintptr { return 1 }),
	syscall.NewCallback(func(d *fakeData) uintptr { return 1 }),
	// GetData
	syscall.NewCallback(func(d *fakeData, f *formatEtc, m *stgMedium) uintptr {
		if d.files == nil || f.format != cfHDrop || f.tymed&tymedHGlobal == 0 {
			return dvEFormatEtc
		}
		// DROPFILES, then the wide names, each ended, and one more end.
		var names []uint16
		for _, name := range d.files {
			u, _ := syscall.UTF16FromString(name)
			names = append(names, u...)
		}
		names = append(names, 0)
		size := dropFilesBytes + 2*len(names)
		buf := make([]byte, size)
		binary.LittleEndian.PutUint32(buf[0:], dropFilesBytes) // pFiles
		binary.LittleEndian.PutUint32(buf[16:], 1)             // fWide
		for i, u := range names {
			binary.LittleEndian.PutUint16(buf[dropFilesBytes+2*i:], u)
		}
		h, _, _ := globalAlloc.Call(gmemMoveableT, uintptr(size))
		p, _, _ := globalLock.Call(h)
		rtlMoveMemory.Call(p, uintptr(unsafe.Pointer(&buf[0])), uintptr(size))
		globalUnlock.Call(h)
		*m = stgMedium{tymed: tymedHGlobal, handle: h}
		return sOK
	}),
	syscall.NewCallback(func(d *fakeData, f *formatEtc, m *stgMedium) uintptr { return eNotImpl }),
	// QueryGetData
	syscall.NewCallback(func(d *fakeData, f *formatEtc) uintptr {
		if d.files == nil || f.format != cfHDrop {
			return dvEFormatEtc
		}
		return sOK
	}),
	syscall.NewCallback(func(d *fakeData, in, out uintptr) uintptr { return eNotImpl }),
	syscall.NewCallback(func(d *fakeData, f, m, release uintptr) uintptr { return eNotImpl }),
	syscall.NewCallback(func(d *fakeData, dir, out uintptr) uintptr { return eNotImpl }),
	syscall.NewCallback(func(d *fakeData, f, advf, sink, conn uintptr) uintptr { return eNotImpl }),
	syscall.NewCallback(func(d *fakeData, conn uintptr) uintptr { return eNotImpl }),
	syscall.NewCallback(func(d *fakeData, out uintptr) uintptr { return eNotImpl }),
}

// TestFileDropFindsItsDestinationAndDelivers drives the window's OLE drop
// target as OLE would: the innermost enabled view taking files under the
// pointer gets them, as file: URLs; disabled views and text drags get
// nothing, and the destination hears the drag enter and leave.
func TestFileDropFindsItsDestinationAndDelivers(t *testing.T) {
	title := "nsgo-drop"
	app := func(cx *core.Scope) core.Node {
		log := core.NewSignal(cx, "")
		add := func(s string) { log.Update(func(l string) string { return l + s + ";" }) }
		cx.Effect(func() { cx.SetTitle(title + "|" + log.Get()) })
		zone := func(name string, disabled bool, children ...core.Node) core.Node {
			return core.View{Style: core.Style{Width: driver.Pt(150), Height: driver.Pt(200), Padding: driver.All(20)},
				Disabled:        func() bool { return disabled },
				OnFileDrop:      func(uris []string) { add(name + "=" + strings.Join(uris, ",")) },
				OnFileDragEnter: func() { add("enter " + name) },
				OnFileDragLeave: func() { add("leave " + name) },
				Children:        children}
		}
		return core.View{Style: core.Style{Width: driver.Pt(300), Height: driver.Pt(200), Direction: core.Row},
			Children: []core.Node{
				zone("a", false, core.View{Style: core.Style{Width: driver.Pt(50), Height: driver.Pt(50), Background: driver.RGB(0x00ff00)}}),
				zone("b", true),
			}}
	}
	hwnd := startWith(t, app, Options{Title: title + "|", Width: 300, Height: 200})
	waitForTitle(t, hwnd, title+"|", 5*time.Second)

	files := []string{`C:\nsgo dir\a b.txt`, `C:\日本\x`, `\\server\share\y z`}
	data := &fakeData{vtbl: &fakeDataVtbl, files: files}
	text := &fakeData{vtbl: &fakeDataVtbl}
	screen := func(x, y float64) point {
		at := point{px(x), px(y)}
		clientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&at)))
		return at
	}
	// call calls a method of the target with a data object, if any, the
	// point and the effect, and returns the effect it answered.
	call := func(method int, d *fakeData, at point) uint32 {
		effect := uint32(dropEffectCopy | 2) // Copy or Move
		var answer uint32
		onMain(t, hwnd, func(w *window) {
			if w.drop == nil {
				t.Error("the window is no drop target")
				return
			}
			o := (*comObject)(unsafe.Pointer(w.drop))
			args := []uintptr{}
			if d != nil {
				args = append(args, uintptr(unsafe.Pointer(d)))
			}
			args = append(args, 0)
			args = append(args, pointArgs(at.X, at.Y)...)
			args = append(args, uintptr(unsafe.Pointer(&effect)))
			o.call(method, args...)
			answer = effect
		})
		return answer
	}
	const dragEnter, dragOver, dragLeave, drop = 3, 4, 5, 6

	// Text is no file drop.
	if got := call(dragEnter, text, screen(75, 100)); got != dropEffectNone {
		t.Fatalf("a text drag got effect %d", got)
	}
	onMain(t, hwnd, func(w *window) { (*comObject)(unsafe.Pointer(w.drop)).call(dragLeave) })

	// Over the inner view, its ancestor takes the files.
	if got := call(dragEnter, data, screen(40, 40)); got != dropEffectCopy {
		t.Fatalf("over a destination the effect is %d, want Copy", got)
	}
	waitForTitle(t, hwnd, title+"|enter a;", 5*time.Second)
	// The disabled view takes nothing, and a leaves.
	if got := call(dragOver, nil, screen(225, 100)); got != dropEffectNone {
		t.Fatalf("over a disabled view the effect is %d", got)
	}
	waitForTitle(t, hwnd, title+"|enter a;leave a;", 5*time.Second)
	if got := call(drop, data, screen(75, 150)); got != dropEffectCopy {
		t.Fatalf("the drop's effect is %d, want Copy", got)
	}
	want := title + "|enter a;leave a;enter a;a=file:///C:/nsgo%20dir/a%20b.txt," +
		"file:///C:/%E6%97%A5%E6%9C%AC/x,file://server/share/y%20z;leave a;"
	if got := waitForTitle(t, hwnd, want, 5*time.Second); got != want {
		t.Fatalf("title %q, want %q", got, want)
	}
}

// testComponent is a native component with a window of its own: a STATIC
// control. It emits each prop it is given.
type testComponent struct {
	cx       *driver.NativeContext
	hwnd     uintptr
	commands []string
}

func (c *testComponent) View() any { return c.hwnd }

func (c *testComponent) SetProp(name string, value any) {
	c.cx.Emit("prop", fmt.Sprintf("%s=%v", name, value))
}

func (c *testComponent) Command(name string, args any) {
	c.commands = append(c.commands, name)
	c.cx.Emit("command", fmt.Sprintf("%s(%v)", name, args))
}

func (c *testComponent) Close() { destroyWindow.Call(c.hwnd) }

// TestNativeComponentFollowsItsView hosts a component's window: it is a
// child of the window, placed on its view, clipped by the scroll view
// around it, follows scrolling, gets props and commands, and hides with
// its view.
func TestNativeComponentFollowsItsView(t *testing.T) {
	title := "nsgo-component"
	var made []*testComponent
	factory := func(cx *driver.NativeContext) driver.NativeComponent {
		const wsPopupT, wsVisible = 0x80000000, 0x10000000
		hwnd, _, _ := createWindowExW.Call(0, uintptr(unsafe.Pointer(wide("STATIC"))),
			uintptr(unsafe.Pointer(wide("component"))), wsPopupT|wsVisible, 0, 0, 50, 50, 0, 0, 0, 0)
		c := &testComponent{cx: cx, hwnd: hwnd}
		made = append(made, c)
		return c
	}
	app := func(cx *core.Scope) core.Node {
		var ref core.Ref
		step := core.NewSignal(cx, 0)
		log := core.NewSignal(cx, "")
		cx.Effect(func() { cx.SetTitle(fmt.Sprintf("%s|%d|%s", title, step.Get(), log.Get())) })
		return core.View{Style: core.Style{Width: driver.Pt(300), Height: driver.Pt(240), Background: driver.RGB(0xffffff)},
			Children: []core.Node{
				core.View{Style: core.Style{Height: driver.Pt(30), Background: driver.RGB(0x123456)},
					OnPress: func(core.PressEvent) {
						step.Update(func(n int) int { return n + 1 })
						if step.GetUntracked() == 1 {
							ref.Command("ping", 7)
						}
					}},
				core.Scroll{Style: core.Style{Width: driver.Pt(200), Height: driver.Pt(150), Margin: driver.Edges{Left: 10}},
					Children: []core.Node{
						core.View{Style: core.Style{Height: driver.Pt(100)}},
						core.Native{Component: "box", Ref: &ref, TestID: "box",
							PropsFunc: func() map[string]any { return map[string]any{"step": step.Get()} },
							OnEvent: func(name string, data any) {
								log.Update(func(l string) string { return l + name + ":" + fmt.Sprint(data) + ";" })
							},
							StyleFunc: func() core.Style {
								s := core.Style{Width: driver.Pt(120), Height: driver.Pt(100)}
								if step.Get() >= 2 {
									s.Display = core.DisplayNone
								}
								return s
							}},
						core.View{Style: core.Style{Height: driver.Pt(300)}},
					}},
			}}
	}
	hwnd := startWith(t, app, Options{Title: title + "|0|", Width: 300, Height: 240,
		Components: driver.NativeComponents{"box": factory}})
	waitForTitle(t, hwnd, title+"|0|prop:step=0;", 5*time.Second)

	var child uintptr
	placed := func() (r rect, region rect, shown bool) {
		onMain(t, hwnd, func(w *window) {
			if len(made) != 1 {
				t.Errorf("%d components made", len(made))
				return
			}
			child = made[0].hwnd
			if p, _, _ := getParent.Call(child); p != hwnd {
				t.Errorf("the component's window has parent %#x, want the window %#x", p, hwnd)
			}
			getWindowRect.Call(child, uintptr(unsafe.Pointer(&r)))
			mapWindowPoints.Call(0, hwnd, uintptr(unsafe.Pointer(&r)), 2)
			getWindowRgnBox.Call(child, uintptr(unsafe.Pointer(&region)))
			v, _, _ := isWindowVisible.Call(child)
			shown = v != 0
		})
		return
	}
	// The view is at (10, 130) in the window, 120 by 100; the scroll view
	// ends at 180, which cuts it to 50 points.
	wait := func(want rect, wantRegion rect) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			r, region, shown := placed()
			if shown && r == want && region == wantRegion {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("placed at %v with region %v (shown %v), want %v with %v", r, region, shown, want, wantRegion)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	wait(rect{px(10), px(130), px(130), px(230)}, rect{0, 0, px(120), px(50)})

	// Scrolled by 60 points, it moves up and shows 100 - 10 points.
	onMain(t, hwnd, func(w *window) {
		for _, v := range w.views {
			if v != nil && v.kind == driver.KindScroll {
				w.scrollTo(v, driver.Point{Y: 60})
				invalidateRect.Call(w.hwnd, 0, 0)
			}
		}
	})
	// Wholly visible, it has no region.
	wait(rect{px(10), px(70), px(130), px(170)}, rect{})

	// A command and a prop reach it; its events reach the app.
	click(t, hwnd, px(100), px(10))
	waitForTitle(t, hwnd, title+"|1|prop:step=0;prop:step=1;command:ping(7);", 5*time.Second)

	// Hidden with its view.
	click(t, hwnd, px(100), px(10))
	waitForTitle(t, hwnd, title+"|2|", 5*time.Second)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, _, shown := placed(); !shown {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the component's window shows with its view hidden")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestAnimatedValuesDrawWithoutTheEngine runs an animation on the frame
// clock: a view's background and translation follow it to the end, and a
// drag moves a view through values bound to its pans.
func TestAnimatedValuesDrawWithoutTheEngine(t *testing.T) {
	title := "nsgo-animated"
	app := func(cx *core.Scope) core.Node {
		state := core.NewSignal(cx, "running")
		cx.Effect(func() { cx.SetTitle(title + "|" + state.Get()) })
		progress := core.NewAnimatedValue(cx, 0)
		dragX := core.NewAnimatedValue(cx, 0)
		cx.StartAnimation(core.Timing(progress, 1, driver.TimingConfig{Duration: 400 * time.Millisecond}),
			func(finished bool) { state.Set(fmt.Sprint("done ", finished)) })
		return core.View{Style: core.Style{Width: driver.Pt(300), Height: driver.Pt(200), Background: driver.RGB(0xffffff)},
			Children: []core.Node{
				core.View{Style: core.Style{Width: driver.Pt(100), Height: driver.Pt(80), Animated: &core.AnimatedStyle{
					Background: core.Interpolate(progress, driver.Interpolation{Input: []float64{0, 1},
						Colors: []driver.Color{driver.RGB(0xff0000), driver.RGB(0x0000ff)}}),
					TranslateX: core.Interpolate(progress, driver.Interpolation{Input: []float64{0, 1}, Output: []float64{0, 150}}),
				}}},
				core.View{Style: core.Style{Width: driver.Pt(60), Height: driver.Pt(60), Margin: driver.Edges{Top: 20},
					Background: driver.RGB(0x00ff00), Animated: &core.AnimatedStyle{TranslateX: dragX}},
					PanValues: core.AnimatedXY{X: dragX}},
			}}
	}
	hwnd := startWith(t, app, Options{Title: title + "|running", Width: 300, Height: 200})
	waitForTitle(t, hwnd, title+"|done true", 5*time.Second)
	// Moved 150 points right, and blue.
	waitForColor(t, hwnd, px(200), px(40), 0x0000ff, 2*time.Second)
	if got := clientColor(t, hwnd, px(50), px(40)); !near(got, 0xffffff, 8) {
		t.Fatalf("where the view started shows #%06x, want white", got)
	}

	// Dragging the green view 120 points moves it as the pointer moves.
	lParam := func(x, y int32) uintptr { return uintptr(uint32(y)<<16 | uint32(x)&0xffff) }
	y := px(130)
	postMessageW.Call(hwnd, wmMouseMove, 0, lParam(px(30), y))
	postMessageW.Call(hwnd, wmLButtonDown, mkLButton, lParam(px(30), y))
	for x := 40.0; x <= 150; x += 10 {
		postMessageW.Call(hwnd, wmMouseMove, mkLButton, lParam(px(x), y))
		time.Sleep(10 * time.Millisecond)
	}
	waitForColor(t, hwnd, px(150), y, 0x00ff00, 2*time.Second)
	postMessageW.Call(hwnd, wmLButtonUp, 0, lParam(px(150), y))
	if got := clientColor(t, hwnd, px(20), y); !near(got, 0xffffff, 8) {
		t.Fatalf("where the dragged view started shows #%06x, want white", got)
	}
}

// TestLayoutAnimationMovesFadesAndRemoves toggles a layout with a linear
// LayoutAnimation of two seconds and looks midway: the moved view is
// between its frames, the new one half faded in and the removed one half
// faded out, still drawn; at the end all are where they go.
func TestLayoutAnimationMovesFadesAndRemoves(t *testing.T) {
	title := "nsgo-layout"
	app := func(cx *core.Scope) core.Node {
		on := core.NewSignal(cx, false)
		cx.Effect(func() { cx.SetTitle(fmt.Sprint(title, "|", on.Get())) })
		box := func(c driver.Color) core.Node {
			return core.View{Style: core.Style{Width: driver.Pt(50), Height: driver.Pt(40), Background: c}}
		}
		return core.View{Style: core.Style{Width: driver.Pt(300), Height: driver.Pt(200), Background: driver.RGB(0xffffff), Gap: 10},
			Children: []core.Node{
				core.View{Style: core.Style{Height: driver.Pt(30), Background: driver.RGB(0x123456)}, OnPress: func(core.PressEvent) {
					cx.AnimateLayout(driver.LayoutAnimation{
						Create: &driver.LayoutTransition{Duration: 2 * time.Second},
						Update: &driver.LayoutTransition{Duration: 2 * time.Second},
						Delete: &driver.LayoutTransition{Duration: 2 * time.Second},
					})
					on.Set(true)
				}},
				// Moves from x 0 to 200.
				core.View{StyleFunc: func() core.Style {
					s := core.Style{Width: driver.Pt(50), Height: driver.Pt(40), Background: driver.RGB(0xff0000)}
					if on.Get() {
						s.Margin.Left = 200
					}
					return s
				}},
				// Goes, then comes, in the same place.
				core.Show{When: on.Get,
					Then: func(*core.Scope) core.Node { return box(driver.RGB(0x0000ff)) },
					Else: func(*core.Scope) core.Node { return box(driver.RGB(0x000000)) }},
			}}
	}
	hwnd := startWith(t, app, Options{Title: title + "|false", Width: 300, Height: 200})
	// The rows: the button 0-30, the moving box 40-80, the swapped box
	// 90-130.
	rowMove, rowSwap := px(60), px(110)
	waitForColor(t, hwnd, px(25), rowMove, 0xff0000, 5*time.Second)
	waitForColor(t, hwnd, px(25), rowSwap, 0x000000, 5*time.Second)

	clicked := time.Now()
	click(t, hwnd, px(100), px(15))
	waitForTitle(t, hwnd, title+"|true", 5*time.Second)
	time.Sleep(700 * time.Millisecond)
	t.Logf("sampled %v after the click", time.Since(clicked))
	pix, w, _, ok := clientPixels(hwnd)
	if !ok {
		t.Skip("Windows cannot copy the window's pixels here")
	}
	at := func(x, y int32) uint32 {
		i := (int(y)*int(w) + int(x)) * 4
		return uint32(pix[i+2])<<16 | uint32(pix[i+1])<<8 | uint32(pix[i])
	}
	// The moving box's left edge, midway between 0 and 200.
	left := int32(-1)
	for x := int32(0); x < px(300); x++ {
		if near(at(x, rowMove), 0xff0000, 40) {
			left = x
			break
		}
	}
	if left <= px(10) || left >= px(190) {
		t.Fatalf("midway the moving box starts at x %d, want between %d and %d", left, px(10), px(190))
	}
	// The removed black box and the new blue one overlap, both partly
	// shown: neither black, blue nor white.
	mid := at(px(25), rowSwap)
	if near(mid, 0x000000, 30) || near(mid, 0x0000ff, 30) || near(mid, 0xffffff, 30) {
		t.Fatalf("midway the swapped boxes show #%06x, want a mix", mid)
	}
	t.Logf("midway: left %d, swapped #%06x", left, mid)
	// At the end the boxes are where they go: the moved one's right
	// edge reaches 250.
	waitForColor(t, hwnd, px(246), rowMove, 0xff0000, 3*time.Second)
	t.Logf("moved %v after the click", time.Since(clicked))
	time.Sleep(100 * time.Millisecond)
	waitForColor(t, hwnd, px(25), rowMove, 0xffffff, time.Second)
	waitForColor(t, hwnd, px(25), rowSwap, 0x0000ff, time.Second)
	onMain(t, hwnd, func(w *window) {
		for _, v := range w.views {
			if v != nil && len(v.leaving) > 0 {
				t.Error("a removed view is still drawn after its animation")
			}
		}
		if w.ticking {
			t.Error("the frame clock runs with nothing animating")
		}
	})
}

// TestAnimatedGIFAndSVGImagesDraw shows an animated GIF, whose frames
// change on the frame clock, and an SVG drawn sharp, which reports its
// intrinsic size.
func TestAnimatedGIFAndSVGImagesDraw(t *testing.T) {
	title := "nsgo-images"
	var buf bytes.Buffer
	palette := color.Palette{color.RGBA{0xff, 0, 0, 0xff}, color.RGBA{0, 0, 0xff, 0xff}}
	anim := &gif.GIF{LoopCount: 0}
	for i := range 2 {
		frame := image.NewPaletted(image.Rect(0, 0, 8, 8), palette)
		for j := range frame.Pix {
			frame.Pix[j] = uint8(i)
		}
		anim.Image = append(anim.Image, frame)
		anim.Delay = append(anim.Delay, 15) // 150 ms
	}
	if err := gif.EncodeAll(&buf, anim); err != nil {
		t.Fatal(err)
	}
	const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="20" viewBox="0 0 10 20">` +
		`<rect width="10" height="10" fill="#ff0000"/><rect y="10" width="10" height="10" fill="#00ff00"/></svg>`
	app := func(cx *core.Scope) core.Node {
		size := core.NewSignal(cx, "")
		cx.Effect(func() { cx.SetTitle(title + "|" + size.Get()) })
		return core.View{Style: core.Style{Width: driver.Pt(300), Height: driver.Pt(200), Background: driver.RGB(0xffffff),
			Direction: core.Row}, Children: []core.Node{
			core.Image{Source: driver.ImageSource{Data: buf.Bytes()}, ResizeMode: driver.Stretch,
				Style: core.Style{Width: driver.Pt(100), Height: driver.Pt(100)}},
			core.Image{Source: driver.ImageSource{Data: []byte(svg)}, ResizeMode: driver.Stretch,
				Style:  core.Style{Width: driver.Pt(100), Height: driver.Pt(200)},
				OnLoad: func(s driver.Size) { size.Set(fmt.Sprintf("%vx%v", s.Width, s.Height)) }},
		}}
	}
	hwnd := startWith(t, app, Options{Title: title + "|", Width: 300, Height: 200})
	waitForTitle(t, hwnd, title+"|10x20", 5*time.Second)
	// The GIF shows both its frames within a second.
	seen := map[uint32]bool{}
	deadline := time.Now().Add(2 * time.Second)
	for len(seen) < 2 && time.Now().Before(deadline) {
		switch c := clientColor(t, hwnd, px(50), px(50)); {
		case near(c, 0xff0000, 24):
			seen[0xff0000] = true
		case near(c, 0x0000ff, 24):
			seen[0x0000ff] = true
		}
		time.Sleep(30 * time.Millisecond)
	}
	if len(seen) < 2 {
		t.Fatalf("the GIF showed %v, want both its frames", seen)
	}
	// The SVG's halves, and its edge between them sharp: two points
	// either side of it are already their colors.
	waitForColor(t, hwnd, px(150), px(50), 0xff0000, time.Second)
	waitForColor(t, hwnd, px(150), px(150), 0x00ff00, time.Second)
	waitForColor(t, hwnd, px(150), px(98), 0xff0000, time.Second)
	waitForColor(t, hwnd, px(150), px(102), 0x00ff00, time.Second)
}

// TestImagesFitAtTheirPositionBlurAndFade checks the core fit with a
// position, NSH's Center at the natural size, a blur that blends the
// halves but keeps the edges opaque, a fade-in and ImageSize.
func TestImagesFitAtTheirPositionBlurAndFade(t *testing.T) {
	title := "nsgo-image-props"
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
	data := buf.Bytes()
	app := func(cx *core.Scope) core.Node {
		size := core.NewSignal(cx, "")
		cx.Effect(func() { cx.SetTitle(title + "|" + size.Get()) })
		cx.ImageSize(driver.ImageSource{Data: data}, func(px driver.Size, err error) { size.Set(fmt.Sprint(px, err)) })
		at := func(left, top, width, height driver.Points, pos driver.ObjectPosition) core.Style {
			return core.Style{Position: core.Absolute, Left: driver.Pt(left), Top: driver.Pt(top),
				Width: driver.Pt(width), Height: driver.Pt(height), ObjectPosition: pos}
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
					Style: at(150, 0, 80, 40, driver.ObjectPosition{})},
				core.Image{Source: driver.ImageSource{Data: data}, ResizeMode: driver.Stretch, FadeDuration: 300 * time.Millisecond,
					Style: at(150, 50, 80, 40, driver.ObjectPosition{})},
			}}
	}
	hwnd := startWith(t, app, Options{Title: title + "|", Width: 360, Height: 240})
	waitForTitle(t, hwnd, title+"|"+fmt.Sprint(driver.Size{Width: 20, Height: 10}, nil), 5*time.Second)
	waitForColor(t, hwnd, px(20), px(20), 0xffffff, time.Second)
	waitForColor(t, hwnd, px(50), px(20), 0xff0000, time.Second)
	waitForColor(t, hwnd, px(100), px(20), 0x0000ff, time.Second)
	waitForColor(t, hwnd, px(3), px(57), 0xffffff, time.Second)
	waitForColor(t, hwnd, px(8), px(60), 0xff0000, time.Second)
	waitForColor(t, hwnd, px(22), px(60), 0x0000ff, time.Second)
	waitForColor(t, hwnd, px(28), px(60), 0xffffff, time.Second)
	waitForColor(t, hwnd, px(10), px(68), 0xffffff, time.Second)
	// Blurred, the middle mixes red and blue and the edge stays red.
	if c := clientColor(t, hwnd, px(190), px(20)); c>>16&0xff < 0x40 || c&0xff < 0x40 {
		t.Fatalf("blurred middle = #%06x, want red and blue mixed", c)
	}
	if c := clientColor(t, hwnd, px(151), px(1)); c>>16&0xff < 0xc0 {
		t.Fatalf("blurred edge = #%06x, want opaque red", c)
	}
	waitForColor(t, hwnd, px(160), px(70), 0xff0000, time.Second)
}

// TestImageClipsItsRoundedCorners checks an image's content is clipped to
// its rounded corners without Clip, as Apple, Linux and the web do.
func TestImageClipsItsRoundedCorners(t *testing.T) {
	m := image.NewRGBA(image.Rect(0, 0, 20, 10))
	for y := range 10 {
		for x := range 20 {
			m.Set(x, y, color.RGBA{0xff, 0, 0, 0xff})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, m); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	title := "nsgo-image-corners"
	hwnd := startWith(t, func(*core.Scope) core.Node {
		return core.View{Style: core.Style{Width: driver.Pt(120), Height: driver.Pt(120), Background: driver.RGB(0xffffff)},
			Children: []core.Node{
				core.Image{Source: driver.ImageSource{Data: data}, ResizeMode: driver.Stretch,
					Style: core.Style{Width: driver.Pt(100), Height: driver.Pt(100), CornerRadius: 50}},
			}}
	}, Options{Title: title + "|", Width: 120, Height: 120})
	waitForColor(t, hwnd, px(50), px(50), 0xff0000, 5*time.Second)
	// The frame's corner is outside the picture's circle.
	waitForColor(t, hwnd, px(4), px(4), 0xffffff, time.Second)
}

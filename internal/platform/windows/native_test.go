//go:build windows

package windows

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/engine"
)

var (
	getDC                  = user32.NewProc("GetDC")
	releaseDC              = user32.NewProc("ReleaseDC")
	getWindowTextW         = user32.NewProc("GetWindowTextW")
	getWindowTextLength    = user32.NewProc("GetWindowTextLengthW")
	getWindowRgnBox        = user32.NewProc("GetWindowRgnBox")
	createCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	createCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	deleteDC               = gdi32.NewProc("DeleteDC")
	selectObject           = gdi32.NewProc("SelectObject")
	bitBlt                 = gdi32.NewProc("BitBlt")
	getDIBits              = gdi32.NewProc("GetDIBits")

	findWindowW   = user32.NewProc("FindWindowW")
	findWindowExW = user32.NewProc("FindWindowExW")
	printWindow   = user32.NewProc("PrintWindow")
)

// PrintWindow's PW_CLIENTONLY and PW_RENDERFULLCONTENT: the swap chain's
// pixels are only in the window's composed content.
const clientOnly = 1 | 2

const srcCopy = 0x00CC0020

// windowText returns a window's title.
func windowText(hwnd uintptr) string {
	n, _, _ := getWindowTextLength.Call(hwnd)
	buf := make([]uint16, n+1)
	getWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), n+1)
	return syscall.UTF16ToString(buf)
}

// bitmapInfo is BITMAPINFO with no color table.
type bitmapInfo struct {
	Size                         uint32
	Width, Height                int32
	Planes, BitCount             uint16
	Compression, SizeImage       uint32
	XPelsPerMeter, YPelsPerMeter int32
	ClrUsed, ClrImportant        uint32
	Colors                       [1]uint32
}

// nativeApp is the app a native test drives: a button and a field whose
// state the window's title shows, so the test can wait for the engine.
func nativeApp(title string) func(*core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		count := core.NewSignal(cx, 0)
		field := core.NewSignal(cx, "")
		submitted := core.NewSignal(cx, 0)
		cx.Effect(func() {
			cx.SetTitle(fmt.Sprintf("%s|%d|%s|%d", title, count.Get(), field.Get(), submitted.Get()))
		})
		return core.View{
			Style: core.Style{Width: driver.Pt(360), Height: driver.Pt(240), Padding: driver.All(10),
				Direction: core.Row, Gap: 10, Background: driver.RGB(0xffffff)},
			Children: []core.Node{
				core.View{OnPress: func(core.PressEvent) { count.Update(func(n int) int { return n + 1 }) },
					Style: core.Style{Width: driver.Pt(100), Height: driver.Pt(80), Background: driver.RGB(0x123456)}},
				core.Input{Value: field.Get, OnChange: field.Set, AutoFocus: true,
					OnSubmit: func(string) { submitted.Update(func(n int) int { return n + 1 }) },
					Style:    core.Style{Width: driver.Pt(200), Height: driver.Pt(30)}},
			},
		}
	}
}

// TestWindowRunsAnAppAndTakesNativeInput opens a real window, clicks its
// view, types into its drawn field and reads the pixels it painted. It
// skips where no desktop can create a window.
func TestWindowRunsAnAppAndTakesNativeInput(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-native-" + strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer close(done)
		Run(nativeApp(title), Options{Title: title + "|0||0", Width: 360, Height: 240})
	}()

	hwnd := waitForWindow(t, title+"|0||0", done)
	if hwnd == 0 {
		return
	}
	t.Cleanup(func() {
		postMessageW.Call(hwnd, wmClose, 0, 0)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("the window did not close")
		}
	})
	waitForText(t, hwnd, title+"|0||0", done)

	// The button at (10, 10) to (110, 90) in the client area.
	click(t, hwnd, 60, 50)
	waitForText(t, hwnd, title+"|1||0", done)

	// The auto-focused field takes what is typed and reports its changes.
	if findChild(hwnd, "EDIT") != 0 {
		t.Fatal("the input is a child window")
	}
	typeText(hwnd, "hi")
	waitForText(t, hwnd, title+"|1|hi|0", done)

	// Return submits the focused field.
	postMessageW.Call(hwnd, wmKeyDown, vkReturn, 1)
	waitForText(t, hwnd, title+"|1|hi|1", done)

	// What Direct2D drew is in the window, not only in the engine's model.
	if r, g, b, ok := windowPixel(hwnd, 60, 50); ok {
		if r != 0x12 || g != 0x34 || b != 0x56 {
			t.Fatalf("button pixel = #%02x%02x%02x, want #123456", r, g, b)
		}
	}
}

// typeText posts text to the window as a keyboard's characters.
func typeText(hwnd uintptr, text string) {
	for _, u := range utf16.Encode([]rune(text)) {
		postMessageW.Call(hwnd, wmChar, uintptr(u), 1)
	}
}

// pressKey posts a key's down and the character it types, if any.
func pressKey(hwnd uintptr, vk uintptr, char uint16) {
	postMessageW.Call(hwnd, wmKeyDown, vk, 1)
	if char != 0 {
		postMessageW.Call(hwnd, wmChar, uintptr(char), 1)
	}
}

func findChild(parent uintptr, class string) uintptr {
	child, _, _ := findWindowExW.Call(parent, 0, uintptr(unsafe.Pointer(wide(class))), 0)
	return child
}

// fieldApp is a field above a button that opens a modal covering it. The
// title shows the field's text, its focus and blur counts, its submits
// and whether the modal shows.
func fieldApp(title string) func(*core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		text := core.NewSignal(cx, "")
		focused, blurred, submitted := core.NewSignal(cx, 0), core.NewSignal(cx, 0), core.NewSignal(cx, 0)
		open := core.NewSignal(cx, false)
		cx.Effect(func() {
			cx.SetTitle(fmt.Sprintf("%s|%s|%d|%d|%d|%v", title, text.Get(), focused.Get(), blurred.Get(),
				submitted.Get(), open.Get()))
		})
		cx.OnKey("f2", func() { open.Update(func(o bool) bool { return !o }) })
		return core.View{
			Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(200), Padding: driver.All(10),
				Background: driver.RGB(0xffffff)},
			Children: []core.Node{
				core.Input{Value: text.Get, OnChange: text.Set, Placeholder: "Type here",
					OnFocus:        func() { focused.Update(func(n int) int { return n + 1 }) },
					OnBlur:         func() { blurred.Update(func(n int) int { return n + 1 }) },
					SubmitBehavior: driver.SubmitBehaviorSubmit,
					OnSubmit:       func(string) { submitted.Update(func(n int) int { return n + 1 }) },
					Style: core.Style{Width: driver.Pt(300), Height: driver.Pt(40), Background: driver.RGB(0xffffff),
						FontSize: 20, Color: driver.RGB(0x000000)}},
				core.Show{When: open.Get, Then: func(*core.Scope) core.Node {
					return core.Modal{Style: core.Style{Position: core.Absolute, Top: driver.Pt(0), Left: driver.Pt(0),
						Width: driver.Pt(320), Height: driver.Pt(200), Background: driver.RGB(0xff0000)}}
				}},
			},
		}
	}
}

// TestDrawnFieldEditsSelectsAndIsCovered types into the drawn field,
// selects a word with a double click and types over it, deletes, pastes,
// submits, and checks that a modal over the field covers its pixels and
// takes its focus until it goes. The IME is not driven: Windows has no
// way to compose without a user's input method.
func TestDrawnFieldEditsSelectsAndIsCovered(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-field"
	hwnd := startApp(t, title+"||0|0|0|false", fieldApp(title), 320, 200)
	// A click in the field focuses it.
	var px, py int32
	onMain(t, hwnd, func(w *window) { px, py = w.px(100), w.px(30) })
	click(t, hwnd, px, py)
	waitForTitle(t, hwnd, title+"||1|0|0|false", 5*time.Second)

	typeText(hwnd, "hello world")
	waitForTitle(t, hwnd, title+"|hello world|1|0|0|", 5*time.Second)

	// A double click on "world" selects it; typing replaces it.
	var x, y int32
	onMain(t, hwnd, func(w *window) {
		in := w.focusedField()
		if in == nil {
			t.Fatal("no field has the focus")
		}
		layout := in.textLayout()
		ox, oy := in.origin(layout)
		cx, cy, h := caretAt(layout, uint32(len("hello wo")))
		c := in.config.Content.Origin
		gx, gy := in.world.apply(float64(ox+cx)+float64(c.X), float64(oy+cy+h/2)+float64(c.Y))
		x, y = int32(gx*w.scale), int32(gy*w.scale)
	})
	lParam := uintptr(uint32(y)<<16 | uint32(x)&0xffff)
	for range 2 {
		postMessageW.Call(hwnd, wmLButtonDown, 1, lParam)
		postMessageW.Call(hwnd, wmLButtonUp, 0, lParam)
	}
	time.Sleep(100 * time.Millisecond)
	onMain(t, hwnd, func(w *window) {
		if in := w.focusedField(); in == nil || in.model.selected() != "world" {
			t.Errorf("the double click selected %q, want world", in.model.selected())
		}
	})
	typeText(hwnd, "there")
	waitForTitle(t, hwnd, title+"|hello there|", 5*time.Second)

	// Backspace deletes before the caret.
	pressKey(hwnd, vkBack, 0x08)
	waitForTitle(t, hwnd, title+"|hello ther|", 5*time.Second)

	// Paste inserts the clipboard's text at the caret, its lines joined.
	onMain(t, hwnd, func(w *window) {
		w.setClipboard("e!\r\n")
		w.editKey('V', true, false, false)
	})
	waitForTitle(t, hwnd, title+"|hello there!|", 5*time.Second)
	// Undo takes the paste back.
	onMain(t, hwnd, func(w *window) { w.editKey('Z', true, false, false) })
	waitForTitle(t, hwnd, title+"|hello ther|", 5*time.Second)

	postMessageW.Call(hwnd, wmKeyDown, vkReturn, 1)
	waitForTitle(t, hwnd, title+"|hello ther|1|0|1|false", 5*time.Second)

	// The text is drawn: dark pixels in the field.
	setForegroundWindow.Call(hwnd)
	time.Sleep(300 * time.Millisecond)
	dark := func() int {
		pix, w, h, ok := screenPixels(hwnd)
		if !ok {
			t.Skip("cannot read the screen")
		}
		n := 0
		fx0, fy0, fx1, fy1 := int32(10*float64(w)/320), int32(10*float64(h)/200), int32(310*float64(w)/320), int32(50*float64(h)/200)
		for py := fy0; py < fy1; py++ {
			for px := fx0; px < fx1; px++ {
				at := (int(py)*int(w) + int(px)) * 4
				if int(pix[at])+int(pix[at+1])+int(pix[at+2]) < 200 {
					n++
				}
			}
		}
		return n
	}
	if n := dark(); n < 20 {
		t.Fatalf("only %d dark pixels in the field: its text is not drawn", n)
	}

	// The modal covers the field, which gives up the focus.
	postMessageW.Call(hwnd, wmKeyDown, 0x71, 1) // F2
	waitForTitle(t, hwnd, title+"|hello ther|1|1|1|true", 5*time.Second)
	sx, sy := int32(60), int32(30)
	onMain(t, hwnd, func(w *window) { sx, sy = w.px(60), w.px(30) })
	if got, ok := waitForPixel(t, hwnd, sx, sy, 0xff0000, 3*time.Second); !ok {
		t.Fatalf("the field under the modal shows #%06x, want the modal's #ff0000", got)
	}
	if n := dark(); n != 0 {
		t.Fatalf("%d dark pixels show through the modal", n)
	}
	// Typing goes nowhere while the modal shows.
	typeText(hwnd, "x")
	time.Sleep(100 * time.Millisecond)
	if got := windowText(hwnd); !strings.HasPrefix(got, title+"|hello ther|") {
		t.Fatalf("typing under the modal changed the field: %q", got)
	}
	// Closing the modal gives the field its focus back.
	postMessageW.Call(hwnd, wmKeyDown, 0x71, 1)
	waitForTitle(t, hwnd, title+"|hello ther|2|1|1|false", 5*time.Second)
}

func waitForWindow(t *testing.T, title string, done <-chan struct{}) uintptr {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		hwnd, _, _ := findWindowW.Call(0, uintptr(unsafe.Pointer(wide(title))))
		if hwnd != 0 {
			return hwnd
		}
		select {
		case <-done:
			t.Skip("no desktop could create the test window")
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("no window titled %q", title)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitForText(t *testing.T, hwnd uintptr, want string, done <-chan struct{}) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if got := windowText(hwnd); got == want {
			return
		}
		select {
		case <-done:
			t.Fatalf("the window closed with title %q, want %q", windowText(hwnd), want)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("window title = %q, want %q", windowText(hwnd), want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func click(t *testing.T, hwnd uintptr, x, y int32) {
	t.Helper()
	lParam := uintptr(uint32(y)<<16 | uint32(x)&0xffff)
	postMessageW.Call(hwnd, wmMouseMove, 0, lParam)
	postMessageW.Call(hwnd, wmLButtonDown, 1, lParam)
	time.Sleep(50 * time.Millisecond)
	postMessageW.Call(hwnd, wmLButtonUp, 0, lParam)
}

// windowPixel reads the client pixel at x, y as the window painted it,
// and reports whether Windows could copy the window's pixels.
func windowPixel(hwnd uintptr, x, y int32) (r, g, b uint8, ok bool) {
	pix, width, height, ok := clientPixels(hwnd)
	if !ok || width <= x || height <= y {
		return 0, 0, 0, false
	}
	at := (int(y)*int(width) + int(x)) * 4
	return pix[at+2], pix[at+1], pix[at], true
}

// clientPixels reads the window's client area as it painted it: 32-bit
// pixels, top to bottom, in the order B, G, R, A. It reports false when
// Windows cannot copy the window's pixels, as on a session with no
// desktop.
func clientPixels(hwnd uintptr) (pix []byte, width, height int32, ok bool) {
	var client rect
	getClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client)))
	width, height = client.Right, client.Bottom
	if width <= 0 || height <= 0 {
		return nil, 0, 0, false
	}
	dc, _, _ := getDC.Call(hwnd)
	if dc == 0 {
		return nil, 0, 0, false
	}
	defer releaseDC.Call(hwnd, dc)
	mem, _, _ := createCompatibleDC.Call(dc)
	if mem == 0 {
		return nil, 0, 0, false
	}
	defer deleteDC.Call(mem)
	bmp, _, _ := createCompatibleBitmap.Call(dc, uintptr(width), uintptr(height))
	if bmp == 0 {
		return nil, 0, 0, false
	}
	defer deleteObject.Call(bmp)
	old, _, _ := selectObject.Call(mem, bmp)
	if r, _, _ := printWindow.Call(hwnd, mem, clientOnly); r == 0 {
		// A window without WM_PRINT support still has its client on
		// screen; copy it directly.
		if r, _, _ := bitBlt.Call(mem, 0, 0, uintptr(width), uintptr(height), dc, 0, 0, srcCopy); r == 0 {
			selectObject.Call(mem, old)
			return nil, 0, 0, false
		}
	}
	// GetDIBits wants the bitmap out of the DC.
	selectObject.Call(mem, old)
	pix = make([]byte, int(width)*int(height)*4)
	info := bitmapInfo{Width: width, Height: -height, Planes: 1, BitCount: 32}
	info.Size = uint32(unsafe.Sizeof(info))
	if n, _, _ := getDIBits.Call(mem, bmp, 0, uintptr(height), uintptr(unsafe.Pointer(&pix[0])),
		uintptr(unsafe.Pointer(&info)), 0); n == 0 {
		return nil, 0, 0, false
	}
	return pix, width, height, true
}

// TestRoundedCornersBlend draws a circle and checks the pixels along its
// edge: without antialiasing every one of them is black or white.
func TestRoundedCornersBlend(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	testEdgesBlend(t, "nsgo-round", func(*core.Scope) core.Node {
		return core.View{Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(240), Background: driver.RGB(0xffffff),
			JustifyContent: core.JustifyCenter, AlignItems: core.AlignCenter}, Children: []core.Node{
			core.View{Style: core.Style{Width: driver.Pt(200), Height: driver.Pt(200), CornerRadius: 100,
				Background: driver.RGB(0x000000)}},
		}}
	})
}

// TestRoundedClipBlendsItsEdge clips a black child to a circle and checks
// both that the corner of its frame keeps the background and that the
// circle's edge blends.
func TestRoundedClipBlendsItsEdge(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	app := func(*core.Scope) core.Node {
		return core.View{Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(240), Background: driver.RGB(0xffffff),
			Padding: driver.All(20)}, Children: []core.Node{
			core.View{Style: core.Style{Width: driver.Pt(200), Height: driver.Pt(200), CornerRadius: 100,
				Overflow: core.Hidden, Background: driver.RGB(0xffffff)}, Children: []core.Node{
				core.View{Style: core.Style{Width: driver.Pt(300), Height: driver.Pt(300), Background: driver.RGB(0x000000)}},
			}}}}
	}
	hwnd := testEdgesBlend(t, "nsgo-clip", app)
	if hwnd == 0 {
		return
	}
	scale := 1.0
	if dpi, _, _ := getDpiForSystem.Call(); dpi != 0 {
		scale = float64(dpi) / 96
	}
	// The frame's corner is outside the circle, so the child must not have
	// painted there.
	if r, g, b, ok := windowPixel(hwnd, int32(22*scale), int32(22*scale)); ok && (r != 0xff || g != 0xff || b != 0xff) {
		t.Errorf("the frame's corner = #%02x%02x%02x, want the background", r, g, b)
	}
	if r, g, b, ok := windowPixel(hwnd, int32(120*scale), int32(120*scale)); ok && (r != 0 || g != 0 || b != 0) {
		t.Errorf("the circle's centre = #%02x%02x%02x, want the child's black", r, g, b)
	}
}

// TestCanvasCurvesBlend checks that a canvas's curves blend too.
func TestCanvasCurvesBlend(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	draw := func(d *driver.Drawing, size driver.Size) {
		d.SetFillColor(driver.RGB(0x000000)).BeginPath().
			Ellipse(float32(size.Width)/2, float32(size.Height)/2, float32(size.Width)/2-20, float32(size.Height)/2-20).Fill()
	}
	testEdgesBlend(t, "nsgo-curve", func(*core.Scope) core.Node {
		return core.View{Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(240), Background: driver.RGB(0xffffff)},
			Children: []core.Node{core.Canvas{Draw: draw, Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(240)}}}}
	})
}

// testEdgesBlend runs app in a window and checks that what it painted
// holds black, white and the greys between them, which only a blended
// edge makes.
func testEdgesBlend(t *testing.T, title string, app engine.App) uintptr {
	t.Helper()
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer close(done)
		Run(app, Options{Title: title, Width: 320, Height: 240})
	}()
	hwnd := waitForWindow(t, title, done)
	if hwnd == 0 {
		return 0
	}
	t.Cleanup(func() {
		postMessageW.Call(hwnd, wmClose, 0, 0)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("the window did not close")
		}
	})
	// Wait for the frame that has the shape, since the first may come
	// before the engine has laid it out.
	var pix []byte
	deadline := time.Now().Add(5 * time.Second)
	for {
		var ok bool
		if pix, _, _, ok = clientPixels(hwnd); ok && blackAndWhite(pix) {
			break
		}
		if time.Now().After(deadline) {
			t.Skip("the window painted no shape")
		}
		time.Sleep(10 * time.Millisecond)
	}
	var black, white, blends int
	for i := 0; i+3 < len(pix); i += 4 {
		b, g, r := pix[i], pix[i+1], pix[i+2]
		switch {
		case r == 0 && g == 0 && b == 0:
			black++
		case r == 0xff && g == 0xff && b == 0xff:
			white++
		case r == g && g == b:
			blends++
		}
	}
	if black == 0 || white == 0 {
		t.Fatalf("pixels: %d black, %d white; want both", black, white)
	}
	if blends < 20 {
		t.Fatalf("%d pixels blend the shape's edge, want at least 20", blends)
	}
	return hwnd
}

// blackAndWhite reports whether pix holds both colors, which means the
// shape has been painted.
func blackAndWhite(pix []byte) bool {
	var black, white bool
	for i := 0; i+3 < len(pix); i += 4 {
		switch {
		case pix[i] == 0 && pix[i+1] == 0 && pix[i+2] == 0:
			black = true
		case pix[i] == 0xff && pix[i+1] == 0xff && pix[i+2] == 0xff:
			white = true
		}
	}
	return black && white
}

// TestTextDecorationsBackgroundsAndAttachments draws struck-through
// spaces, alone and in a span, a span's background and an attachment, and
// finds each in the window's pixels.
func TestTextDecorationsBackgroundsAndAttachments(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-inline"
	black := driver.RGB(0x000000)
	strike := core.Style{TextDecoration: driver.TextDecorationLineThrough, Color: black}
	spaces := "            "
	app := func(*core.Scope) core.Node {
		box := core.Style{Height: driver.Pt(40), AlignItems: core.AlignStart}
		return core.View{Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(240), Background: driver.RGB(0xffffff),
			Padding: driver.All(10), FontSize: 20, Color: black}, Children: []core.Node{
			core.View{Style: box, Children: []core.Node{core.Text{Content: spaces, Style: strike}}},
			core.View{Style: box, Children: []core.Node{core.Text{Spans: []core.Span{{Content: spaces, Style: strike}}}}},
			core.View{Style: core.Style{Height: driver.Pt(60), AlignItems: core.AlignStart}, Children: []core.Node{
				core.Text{Spans: []core.Span{
					{Content: "ab "},
					{Content: "code", Style: core.Style{Background: driver.RGB(0xff0000), CornerRadius: 4}},
					{Content: " "},
					{Attachment: core.View{Style: core.Style{Width: driver.Pt(30), Height: driver.Pt(30), Background: driver.RGB(0x0000ff)}}},
					{Content: " cd"},
				}},
			}},
		}}
	}
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer close(done)
		Run(app, Options{Title: title, Width: 320, Height: 240})
	}()
	hwnd := waitForWindow(t, title, done)
	if hwnd == 0 {
		return
	}
	t.Cleanup(func() {
		postMessageW.Call(hwnd, wmClose, 0, 0)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("the window did not close")
		}
	})
	scale := 1.0
	if dpi, _, _ := getDpiForSystem.Call(); dpi != 0 {
		scale = float64(dpi) / 96
	}
	px := func(p float64) int { return int(p * scale) }
	type box struct{ left, top, right, bottom, n int }
	find := func(pix []byte, width int, match func(r, g, b byte) bool, top, bottom int) box {
		bx := box{left: 1 << 30, top: 1 << 30}
		for y := top; y < bottom && (y+1)*width*4 <= len(pix); y++ {
			for x := 0; x < width; x++ {
				at := (y*width + x) * 4
				if match(pix[at+2], pix[at+1], pix[at]) {
					bx.n++
					bx.left, bx.right = min(bx.left, x), max(bx.right, x+1)
					bx.top, bx.bottom = min(bx.top, y), max(bx.bottom, y+1)
				}
			}
		}
		return bx
	}
	dark := func(r, g, b byte) bool { return r < 0x80 && g < 0x80 && b < 0x80 }
	red := func(r, g, b byte) bool { return r == 0xff && g == 0 && b == 0 }
	blue := func(r, g, b byte) bool { return r == 0 && g == 0 && b == 0xff }
	var pix []byte
	var width int32
	deadline := time.Now().Add(5 * time.Second)
	for {
		var ok bool
		if pix, width, _, ok = clientPixels(hwnd); ok && find(pix, int(width), blue, 0, px(240)).n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Skip("the window painted no attachment")
		}
		time.Sleep(10 * time.Millisecond)
	}
	w := int(width)
	if s := find(pix, w, dark, px(10), px(50)); s.n == 0 || s.right-s.left < px(30) {
		t.Errorf("a struck-through text's spaces drew %+v, want a line", s)
	}
	if s := find(pix, w, dark, px(50), px(90)); s.n == 0 || s.right-s.left < px(30) {
		t.Errorf("a struck-through span's spaces drew %+v, want a line", s)
	}
	bg := find(pix, w, red, px(90), px(150))
	att := find(pix, w, blue, px(90), px(150))
	if bg.n < px(20)*px(10) {
		t.Errorf("the span's background covers %d pixels, want a box", bg.n)
	}
	if got, want := att.right-att.left, px(30); got < want-1 || got > want+1 {
		t.Errorf("the attachment is %d pixels wide, want %d", got, want)
	}
	if got, want := att.bottom-att.top, px(30); got < want-1 || got > want+1 {
		t.Errorf("the attachment is %d pixels tall, want %d", got, want)
	}
	if att.left <= bg.right {
		t.Errorf("the attachment starts at %d, inside the background ending at %d", att.left, bg.right)
	}
	// It stands on the baseline, which is below the background's top by
	// the font's ascent: its bottom is between the background's top and
	// bottom.
	if att.bottom <= bg.top || att.bottom >= bg.bottom {
		t.Errorf("the attachment's bottom %d is not on the baseline, inside the background %d..%d",
			att.bottom, bg.top, bg.bottom)
	}
}

// TestFallbackGlyphsPaintAsTheirFallback draws a command symbol and an
// emoji in the default font and again in the fallback family that has
// them, and checks the two renderings' ink has the same box: a glyph the
// font lacks would be the missing-glyph box instead.
func TestFallbackGlyphsPaintAsTheirFallback(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-glyphs"
	glyphs := []struct{ text, family string }{
		{"中", "Microsoft YaHei UI"},
		{"😀", "Segoe UI Emoji"},
	}
	var rows []core.Node
	for _, g := range glyphs {
		for _, family := range []string{"", g.family} {
			style := core.Style{FontSize: 40, FontFamily: family, Color: driver.RGB(0)}
			rows = append(rows, core.View{Style: core.Style{Height: driver.Pt(60), AlignItems: core.AlignStart},
				Children: []core.Node{core.Text{Content: g.text, Style: style}}})
		}
	}
	app := func(*core.Scope) core.Node {
		return core.View{Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(400), Background: driver.RGB(0xffffff),
			Padding: driver.All(10)}, Children: rows}
	}
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer close(done)
		Run(app, Options{Title: title, Width: 320, Height: 400})
	}()
	hwnd := waitForWindow(t, title, done)
	if hwnd == 0 {
		return
	}
	t.Cleanup(func() {
		postMessageW.Call(hwnd, wmClose, 0, 0)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("the window did not close")
		}
	})
	scale := 1.0
	if dpi, _, _ := getDpiForSystem.Call(); dpi != 0 {
		scale = float64(dpi) / 96
	}
	px := func(p float64) int { return int(p * scale) }
	// Ink is what is not the background: color emoji are not dark.
	dark := func(pix []byte, width, x, y int) bool {
		at := (y*width + x) * 4
		return int(pix[at])+int(pix[at+1])+int(pix[at+2]) < 3*0xc0
	}
	// ink returns the box of the dark pixels in a band, or a zero box.
	ink := func(pix []byte, width, top, bottom int) (left, right, y0, y1 int) {
		left, right, y0, y1 = 1<<30, -1, 1<<30, -1
		for y := top; y < bottom && (y+1)*width*4 <= len(pix); y++ {
			for x := 0; x < width; x++ {
				if dark(pix, width, x, y) {
					left, right = min(left, x), max(right, x)
					y0, y1 = min(y0, y), max(y1, y)
				}
			}
		}
		if right < 0 {
			return 0, 0, 0, 0
		}
		return
	}
	var pix []byte
	var width int32
	deadline := time.Now().Add(5 * time.Second)
	for {
		var ok bool
		if pix, width, _, ok = clientPixels(hwnd); ok {
			if l, r, top, bottom := ink(pix, int(width), px(10), px(70)); r > l && bottom > top {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Skip("the window painted no text")
		}
		time.Sleep(10 * time.Millisecond)
	}
	for i, g := range glyphs {
		l1, r1, t1, b1 := ink(pix, int(width), px(10+float64(i*2)*60), px(10+float64(i*2+1)*60))
		l2, r2, t2, b2 := ink(pix, int(width), px(10+float64(i*2+1)*60), px(10+float64(i*2+2)*60))
		if r1 <= l1 || r2 <= l2 {
			t.Errorf("%q drew nothing in the default or the %s font", g.text, g.family)
			continue
		}
		if r1-l1 != r2-l2 || b1-t1 != b2-t2 || l1 != l2 {
			t.Errorf("%q's default glyph is %d×%d at x=%d, but %s's is %d×%d at x=%d",
				g.text, r1-l1, b1-t1, l1, g.family, r2-l2, b2-t2, l2)
		}
	}
}

func TestWebViewPostsMessagesBothWays(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	if _, err := findWebView2(); err != nil {
		t.Skip(err)
	}
	title := "nsgo-web"
	const page = `<!doctype html><html><body><script>
addEventListener('message', function (e) { window.nsgo.postMessage('echo:' + e.data); });
window.nsgo.postMessage('ready');
</script></body></html>`
	app := func(cx *core.Scope) core.Node {
		var ref core.Ref
		loaded := core.NewSignal(cx, "waiting")
		echo := core.NewSignal(cx, "")
		cx.Effect(func() { cx.SetTitle(title + "|" + loaded.Get() + "|" + echo.Get()) })
		// The web view starts 20pt into a scroll view 150pt tall, which
		// cuts off its bottom.
		return core.View{Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(240), Padding: driver.All(10),
			Background: driver.RGB(0xffffff)}, Children: []core.Node{
			core.Scroll{Style: core.Style{Width: driver.Pt(300), Height: driver.Pt(150)}, Children: []core.Node{
				core.View{Style: core.Style{Height: driver.Pt(20)}},
				core.WebView{Source: core.WebHTML(page, ""), Ref: &ref, UserAgent: "nsgo-test",
					InjectedJavaScriptBeforeContent: "window.before = 'b';",
					InjectedJavaScript:              "window.nsgo.postMessage('loaded:' + window.before + ':' + navigator.userAgent);",
					Style:                           core.Style{Width: driver.Pt(280), Height: driver.Pt(200)},
					OnMessage: func(data string) {
						switch {
						case data == "ready":
							ref.PostMessage("hello")
						case strings.HasPrefix(data, "loaded:"):
							loaded.Set(data)
						default:
							echo.Set(data)
						}
					},
					OnError: func(err error) { loaded.Set("error " + err.Error()) }},
				core.View{Style: core.Style{Height: driver.Pt(200)}},
			}},
		}}
	}
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer close(done)
		Run(app, Options{Title: title + "|waiting|", Width: 320, Height: 240})
	}()
	hwnd := waitForWindow(t, title+"|waiting|", done)
	if hwnd == 0 {
		return
	}
	t.Cleanup(func() {
		postMessageW.Call(hwnd, wmClose, 0, 0)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("the window did not close")
		}
	})
	// The first start of the browser process can take a while.
	want := title + "|loaded:b:nsgo-test|echo:hello"
	deadline := time.Now().Add(30 * time.Second)
	for {
		got := windowText(hwnd)
		if got == want {
			break
		}
		if strings.HasPrefix(got, title+"|error") {
			t.Fatalf("the web view failed: %q", got)
		}
		select {
		case <-done:
			t.Fatalf("the window closed with title %q", got)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("window title = %q, want %q", got, want)
		}
		time.Sleep(20 * time.Millisecond)
	}
	scale := 1.0
	if dpi, _, _ := getDpiForSystem.Call(); dpi != 0 {
		scale = float64(dpi) / 96
	}
	px := func(p float64) int32 { return int32(math.Round(p * scale)) }
	near := func(a, b int32) bool { return a-b <= 1 && b-a <= 1 }
	// placed returns where the last frame put the page: its top and the
	// top and bottom of what its ancestors leave visible, in client pixels.
	placed := func() (top, from, to int32, ok bool) {
		onMain(t, hwnd, func(w *window) {
			for wv := range w.webs {
				if wv.at != nil {
					top, from, to, ok = wv.at.bounds().Top, int32(wv.at.clip.top), int32(wv.at.clip.bottom), true
				}
			}
		})
		return
	}
	if top, from, to, ok := placed(); !ok || !near(top, px(30)) || !near(from, px(30)) || !near(to, px(160)) {
		t.Fatalf("the page is at %d showing %d to %d (%v), want %d showing %d to %d", top, from, to, ok, px(30), px(30), px(160))
	}
	// One notch scrolls 48pt: the top goes under the scroll view's edge.
	at := point{px(15), px(15)}
	clientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&at)))
	postMessageW.Call(hwnd, wmMouseWheel, uintptr(uint32(uint16(0xff88))<<16), uintptr(uint32(at.Y)<<16|uint32(at.X)&0xffff))
	deadline = time.Now().Add(5 * time.Second)
	for {
		top, from, to, _ := placed()
		if near(top, px(-18)) && near(from, px(10)) && near(to, px(160)) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("after scrolling the page is at %d showing %d to %d, want %d showing %d to %d",
				top, from, to, px(-18), px(10), px(160))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// onMain runs fn on the window's thread and waits for it.
func onMain(t *testing.T, hwnd uintptr, fn func(w *window)) {
	t.Helper()
	w := windows[hwnd]
	if w == nil {
		t.Fatal("no such window")
	}
	done := make(chan struct{})
	w.post(func() {
		fn(w)
		close(done)
	})
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the window's thread did not answer")
	}
}

var getWindowDC = getDC

// screenPixels reads the window's client area from the screen, where the
// system composed it with its DirectComposition content: B, G, R, A
// pixels, top to bottom. The window must be on top.
func screenPixels(hwnd uintptr) (pix []byte, width, height int32, ok bool) {
	var client rect
	getClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client)))
	width, height = client.Right, client.Bottom
	origin := point{}
	clientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&origin)))
	screen, _, _ := getWindowDC.Call(0)
	if screen == 0 || width <= 0 || height <= 0 {
		return nil, 0, 0, false
	}
	defer releaseDC.Call(0, screen)
	mem, _, _ := createCompatibleDC.Call(screen)
	defer deleteDC.Call(mem)
	bmp, _, _ := createCompatibleBitmap.Call(screen, uintptr(width), uintptr(height))
	defer deleteObject.Call(bmp)
	old, _, _ := selectObject.Call(mem, bmp)
	r, _, _ := bitBlt.Call(mem, 0, 0, uintptr(width), uintptr(height), screen, uintptr(origin.X), uintptr(origin.Y), srcCopy)
	selectObject.Call(mem, old)
	if r == 0 {
		return nil, 0, 0, false
	}
	pix = make([]byte, int(width)*int(height)*4)
	info := bitmapInfo{Width: width, Height: -height, Planes: 1, BitCount: 32}
	info.Size = uint32(unsafe.Sizeof(info))
	if n, _, _ := getDIBits.Call(mem, bmp, 0, uintptr(height), uintptr(unsafe.Pointer(&pix[0])),
		uintptr(unsafe.Pointer(&info)), 0); n == 0 {
		return nil, 0, 0, false
	}
	return pix, width, height, true
}

// waitForPixel waits until the screen shows want (#rrggbb) at client
// pixel x, y, within tolerance, and returns what it showed last.
func waitForPixel(t *testing.T, hwnd uintptr, x, y int32, want uint32, timeout time.Duration) (uint32, bool) {
	t.Helper()
	var got uint32
	deadline := time.Now().Add(timeout)
	for {
		setForegroundWindow.Call(hwnd)
		if pix, w, h, ok := screenPixels(hwnd); ok && x < w && y < h {
			at := (int(y)*int(w) + int(x)) * 4
			got = uint32(pix[at+2])<<16 | uint32(pix[at+1])<<8 | uint32(pix[at])
			close := true
			for shift := 0; shift < 24; shift += 8 {
				d := int(got>>shift&0xff) - int(want>>shift&0xff)
				if d < -24 || d > 24 {
					close = false
				}
			}
			if close {
				return got, true
			}
		}
		if time.Now().After(deadline) {
			return got, false
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestClientAreaIsTheAppsSize opens a window whose root ends with a bar:
// its client area is the size asked for, within the monitor's work area,
// and the bar's last row is on screen (#23).
func TestClientAreaIsTheAppsSize(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	app := func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{Direction: core.Column, Height: core.Pct(100)}, Children: []core.Node{
			core.View{Style: core.Style{FlexGrow: 1}},
			core.View{Style: core.Style{Height: core.Pt(58), Background: core.RGB(0xFF00A0)}},
		}}
	}
	hwnd := startApp(t, "nsgo-client-size", app, 400, 300)
	var scale float64
	var client, outer rect
	var mi monitorInfo
	onMain(t, hwnd, func(w *window) {
		scale = w.scale
		getClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client)))
		getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&outer)))
		mi.Size = uint32(unsafe.Sizeof(mi))
		monitor, _, _ := monitorFromWindow.Call(hwnd, monitorDefaultNear)
		getMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&mi)))
	})
	px := func(p float64) int32 { return int32(math.Round(p * scale)) }
	if client.Right != px(400) || client.Bottom != px(300) {
		t.Errorf("client area = %dx%d, want %dx%d", client.Right, client.Bottom, px(400), px(300))
	}
	if w := mi.Work; outer.Top < w.Top || outer.Bottom > w.Bottom {
		t.Errorf("window %+v outside the work area %+v", outer, w)
	}
	if got, ok := waitForPixel(t, hwnd, client.Right/2, client.Bottom-1, 0xFF00A0, 3*time.Second); !ok {
		t.Errorf("the bar's last row shows #%06x", got)
	}
}

// TestWheelSettlesOnPagesWithinInsets turns the wheel over a paging
// scroll view with a content inset: it settles on the next page once the
// wheel rests, reporting the momentum, and the inset widens its offsets.
func TestWheelSettlesOnPagesWithinInsets(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-native-" + t.Name()
	var offset, rested atomic.Int64
	offset.Store(math.MinInt32)
	app := func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{Width: driver.Pt(360), Height: driver.Pt(240), Background: driver.RGB(0xffffff)},
			Children: []core.Node{core.Scroll{HideIndicator: true, ContentOffset: driver.Point{Y: -10},
				Behavior:      driver.ScrollBehavior{Paging: true, ContentInset: driver.Edges{Top: 10}},
				OnScroll:      func(e core.ScrollEvent) { offset.Store(int64(e.Offset.Y)) },
				OnMomentumEnd: func(core.ScrollEvent) { rested.Add(1) },
				Style:         core.Style{Width: driver.Pt(100), Height: driver.Pt(100)},
				Children:      []core.Node{core.View{Style: core.Style{Height: driver.Pt(400), Background: driver.RGB(0x0000ff)}}}}}}
	}
	hwnd := startApp(t, title, app, 360, 240)
	wait := func(wantOffset, wantRested int64) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for offset.Load() != wantOffset || rested.Load() != wantRested {
			if time.Now().After(deadline) {
				t.Fatalf("offset %d after %d rests, want %d after %d", offset.Load(), rested.Load(), wantOffset, wantRested)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	wait(-10, 0)
	at := point{px(20), px(20)}
	clientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&at)))
	postMessageW.Call(hwnd, wmMouseWheel, uintptr(uint32(uint16(0xff88))<<16), uintptr(uint32(at.Y)<<16|uint32(at.X)&0xffff))
	wait(100, 1)
}

// TestWheelScrollsThroughAnOverlay turns the wheel over a vertical scroll
// view holding a horizontal one, under an overlay that lets the pointer
// through, as a DataTable's layers do: the wheel scrolls the vertical view
// and leaves the row inside it to a tilt.
func TestWheelScrollsThroughAnOverlay(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-native-" + t.Name()
	var vertical, horizontal atomic.Int64
	app := func(cx *core.Scope) core.Node {
		row := core.Scroll{
			Horizontal: true, HideIndicator: true,
			OnScroll: func(e core.ScrollEvent) { horizontal.Store(int64(e.Offset.X)) },
			Style:    core.Style{Width: driver.Pt(300), Height: driver.Pt(180)},
			Children: []core.Node{core.View{Style: core.Style{Width: driver.Pt(900), Height: driver.Pt(180),
				Background: driver.RGB(0x0000ff)}}},
		}
		page := core.Scroll{
			HideIndicator: true,
			OnScroll:      func(e core.ScrollEvent) { vertical.Store(int64(e.Offset.Y)) },
			Style:         core.Style{Position: core.Absolute, Left: driver.Pt(0), Top: driver.Pt(0), Width: driver.Pt(300), Height: driver.Pt(180)},
			Children:      []core.Node{core.View{Style: core.Style{Width: driver.Pt(300), Height: driver.Pt(600)}, Children: []core.Node{row}}},
		}
		overlay := core.View{
			PointerEvents: driver.PointerEventsBoxNone,
			Style:         core.Style{Position: core.Absolute, Left: driver.Pt(0), Top: driver.Pt(0), Width: driver.Pt(300), Height: driver.Pt(180)},
		}
		return core.View{Style: core.Style{Width: driver.Pt(300), Height: driver.Pt(200), Background: driver.RGB(0xffffff)},
			Children: []core.Node{page, overlay}}
	}
	hwnd := startApp(t, title, app, 300, 200)
	time.Sleep(300 * time.Millisecond)
	at := point{px(30), px(30)}
	clientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&at)))
	postMessageW.Call(hwnd, wmMouseWheel, uintptr(uint32(uint16(0xff88))<<16), uintptr(uint32(at.Y)<<16|uint32(at.X)&0xffff))
	deadline := time.Now().Add(5 * time.Second)
	for vertical.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("the vertical scroll view did not move: vertical %d, horizontal %d", vertical.Load(), horizontal.Load())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if horizontal.Load() != 0 {
		t.Fatalf("the wheel moved the row inside the page: horizontal %d", horizontal.Load())
	}
}

// startApp runs app in a window titled title and returns it, closed when
// the test ends.
func startApp(t *testing.T, title string, app engine.App, width, height driver.Points) uintptr {
	t.Helper()
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer close(done)
		Run(app, Options{Title: title, Width: width, Height: height})
	}()
	hwnd := waitForWindow(t, title, done)
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

// waitForTitle waits for the window's title to start with want.
func waitForTitle(t *testing.T, hwnd uintptr, want string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		got := windowText(hwnd)
		if strings.HasPrefix(got, want) {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatalf("window title = %q, want %q...", got, want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestWebViewIsCoveredByLaterSiblings draws a view over part of a page:
// the composition puts what is drawn after a web view above it.
func TestWebViewIsCoveredByLaterSiblings(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	if _, err := findWebView2(); err != nil {
		t.Skip(err)
	}
	title := "nsgo-web-cover"
	const page = `<!doctype html><html><body style="margin:0;background:#ff0000"></body></html>`
	app := func(cx *core.Scope) core.Node {
		state := core.NewSignal(cx, "waiting")
		cx.Effect(func() { cx.SetTitle(title + "|" + state.Get()) })
		return core.View{Style: core.Style{Width: driver.Pt(300), Height: driver.Pt(200), Background: driver.RGB(0xffffff)},
			Children: []core.Node{
				core.WebView{Source: core.WebHTML(page, ""), Style: core.Style{Width: driver.Pt(300), Height: driver.Pt(200)},
					OnLoad:  func(driver.WebNavigation) { state.Set("loaded") },
					OnError: func(err error) { state.Set("error " + err.Error()) }},
				core.View{Style: core.Style{Position: core.Absolute, Left: driver.Pt(150), Top: driver.Pt(0),
					Width: driver.Pt(150), Height: driver.Pt(200), Background: driver.RGB(0x0000ff)}},
			}}
	}
	hwnd := startApp(t, title+"|waiting", app, 300, 200)
	if got := waitForTitle(t, hwnd, title+"|loaded", 30*time.Second); got != title+"|loaded" {
		t.Fatalf("the page did not load: %q", got)
	}
	scale := 1.0
	if dpi, _, _ := getDpiForSystem.Call(); dpi != 0 {
		scale = float64(dpi) / 96
	}
	px := func(p float64) int32 { return int32(math.Round(p * scale)) }
	if got, ok := waitForPixel(t, hwnd, px(75), px(100), 0xff0000, 10*time.Second); !ok {
		t.Fatalf("the page's pixel is #%06x, want #ff0000", got)
	}
	if got, ok := waitForPixel(t, hwnd, px(225), px(100), 0x0000ff, 5*time.Second); !ok {
		t.Fatalf("the covering sibling's pixel is #%06x, want #0000ff over the page", got)
	}
}

// testVideo is a small local video: NSGO_TEST_VIDEO, else MDN's flower
// clip downloaded to C:\nsgo\flower.mp4.
func testVideo(t *testing.T) string {
	path := os.Getenv("NSGO_TEST_VIDEO")
	if path == "" {
		path = `C:\nsgo\flower.mp4`
	}
	if _, err := os.Stat(path); err != nil {
		t.Skip("no test video:", err)
	}
	return path
}

// TestVideoLoadsDrawsSeeksAndEnds plays a local file: it loads with its
// duration, a seek near the end ends it soon, and its frame is drawn.
func TestVideoLoadsDrawsSeeksAndEnds(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	source := testVideo(t)
	title := "nsgo-video"
	app := func(cx *core.Scope) core.Node {
		var ref core.Ref
		loaded := core.NewSignal(cx, "waiting")
		ends := core.NewSignal(cx, 0)
		latest := core.NewSignal(cx, time.Duration(0))
		cx.Effect(func() {
			cx.SetTitle(fmt.Sprintf("%s|%s|%d|%d", title, loaded.Get(), ends.Get(), latest.Get().Milliseconds()/1000))
		})
		return core.View{Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(200), Background: driver.RGB(0x000000)},
			Children: []core.Node{
				core.Video{Source: source, Muted: true, Ref: &ref, ResizeMode: driver.Stretch,
					Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(200)},
					OnLoad: func(i core.VideoInfo) {
						loaded.Set(fmt.Sprintf("%d", i.Duration.Milliseconds()/100))
						ref.Seek(4 * time.Second)
					},
					OnProgress: func(p, _ time.Duration) { latest.Set(p) },
					OnEnd:      func() { ends.Update(func(n int) int { return n + 1 }) },
					OnError:    func(err error) { loaded.Set("error " + err.Error()) }},
			}}
	}
	hwnd := startApp(t, title+"|waiting|0|0", app, 320, 200)
	got := waitForTitle(t, hwnd, title+"|", 15*time.Second)
	deadline := time.Now().Add(15 * time.Second)
	for strings.HasPrefix(got, title+"|waiting") && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		got = windowText(hwnd)
	}
	parts := strings.Split(got, "|")
	if len(parts) < 4 || strings.HasPrefix(parts[1], "error") || parts[1] == "waiting" {
		t.Fatalf("the video did not load: %q", got)
	}
	// MDN's clip lasts about five seconds, in tenths here.
	if os.Getenv("NSGO_TEST_VIDEO") == "" && parts[1] != "50" {
		t.Fatalf("duration = %s tenths of a second, want 50", parts[1])
	}
	loadedAt := time.Now()
	// The seek leaves about a second to play.
	end := waitForTitle(t, hwnd, title+"|"+parts[1]+"|1|", 4*time.Second)
	if took := time.Since(loadedAt); took > 3500*time.Millisecond {
		t.Fatalf("the end came %v after loading: the seek did not jump", took)
	}
	if p := strings.Split(end, "|"); p[3] < "4" {
		t.Fatalf("the last progress was at %ss, want 4s or more", p[3])
	}
	// The last frame stays drawn: not the black behind it.
	setForegroundWindow.Call(hwnd)
	time.Sleep(300 * time.Millisecond)
	pix, w, h, ok := screenPixels(hwnd)
	if !ok {
		t.Skip("cannot read the screen")
	}
	lit := 0
	for y := int32(0); y < h; y += 8 {
		for x := int32(0); x < w; x += 8 {
			at := (int(y)*int(w) + int(x)) * 4
			if int(pix[at])+int(pix[at+1])+int(pix[at+2]) > 60 {
				lit++
			}
		}
	}
	if total := int(w/8) * int(h/8); lit < total/4 {
		t.Fatalf("only %d of %d sampled pixels are lit: the frame is not drawn", lit, total)
	}
}

// hoverApp is a view at (10, 10) to (110, 90) that counts the pointer
// entering and leaving it in the title.
func hoverApp(title string) func(*core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		in, out := core.NewSignal(cx, 0), core.NewSignal(cx, 0)
		cx.Effect(func() { cx.SetTitle(fmt.Sprintf("%s|%d|%d", title, in.Get(), out.Get())) })
		return core.View{Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(200), Padding: driver.All(10),
			Background: driver.RGB(0xffffff)}, Children: []core.Node{
			core.View{Style: core.Style{Width: driver.Pt(100), Height: driver.Pt(80), Background: driver.RGB(0x123456)},
				OnMouseEnter: func(core.MouseEvent) { in.Update(func(n int) int { return n + 1 }) },
				OnMouseLeave: func(core.MouseEvent) { out.Update(func(n int) int { return n + 1 }) }},
		}}
	}
}

// TestPressesAndTouchesFollowTheMouse holds the button on a view whose
// long press takes 300 ms, then drags on a view that takes the touches.
func TestPressesAndTouchesFollowTheMouse(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-native-presses"
	app := func(cx *core.Scope) core.Node {
		last := core.NewSignal(cx, "none")
		cx.Effect(func() { cx.SetTitle(title + "|" + last.Get()) })
		set := func(s string) func(core.PressEvent) { return func(core.PressEvent) { last.Set(s) } }
		return core.View{Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(200), Background: driver.RGB(0xffffff)},
			Children: []core.Node{core.View{DelayLongPress: 300 * time.Millisecond,
				Style:       core.Style{Width: driver.Pt(100), Height: driver.Pt(100), Background: driver.RGB(0x123456)},
				OnPressDown: set("down"), OnLongPress: set("long"), OnPress: set("press")}}}
	}
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer close(done)
		Run(app, Options{Title: title + "|start", Width: 360, Height: 240})
	}()
	hwnd := waitForWindow(t, title+"|none", done)
	if hwnd == 0 {
		return
	}
	t.Cleanup(func() {
		postMessageW.Call(hwnd, wmClose, 0, 0)
		<-done
	})
	at := uintptr(uint32(40)<<16 | 40)
	postMessageW.Call(hwnd, wmLButtonDown, mkLButton, at)
	waitForText(t, hwnd, title+"|long", done)
	postMessageW.Call(hwnd, wmLButtonUp, 0, at)
	time.Sleep(100 * time.Millisecond)
	waitForText(t, hwnd, title+"|long", done)
	postMessageW.Call(hwnd, wmLButtonDown, mkLButton, at)
	postMessageW.Call(hwnd, wmLButtonUp, 0, at)
	waitForText(t, hwnd, title+"|press", done)
}

// TestTheResponderFollowsADrag drags on a view that takes the touches as
// they start: it is granted them, follows the move and is released.
func TestTheResponderFollowsADrag(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-native-responder"
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer close(done)
		Run(responderApp(title), Options{Title: title + "|start", Width: 360, Height: 240})
	}()
	hwnd := waitForWindow(t, title+"|", done)
	if hwnd == 0 {
		return
	}
	t.Cleanup(func() {
		postMessageW.Call(hwnd, wmClose, 0, 0)
		<-done
	})
	scale := 1.0
	if dpi, _, _ := getDpiForSystem.Call(); dpi != 0 {
		scale = float64(dpi) / 96
	}
	px := func(x, y float64) uintptr { return uintptr(uint32(y*scale)<<16 | uint32(x*scale)&0xffff) }
	postMessageW.Call(hwnd, wmLButtonDown, mkLButton, px(20, 20))
	waitForText(t, hwnd, title+"|grant 0,0", done)
	postMessageW.Call(hwnd, wmMouseMove, mkLButton, px(50, 30))
	waitForText(t, hwnd, title+"|move 30,10", done)
	postMessageW.Call(hwnd, wmLButtonUp, 0, px(50, 30))
	waitForText(t, hwnd, title+"|release 30,10", done)
}

// responderApp is a view that takes the touches as they start and logs
// its responder's events in the title.
func responderApp(title string) func(*core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		log := core.NewSignal(cx, "")
		cx.Effect(func() { cx.SetTitle(title + "|" + log.Get()) })
		say := func(what string) func(core.ResponderEvent) {
			return func(e core.ResponderEvent) {
				log.Set(fmt.Sprintf("%s %.0f,%.0f", what, e.Gesture.Delta.X, e.Gesture.Delta.Y))
			}
		}
		return core.View{Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(200), Background: driver.RGB(0xffffff)},
			Children: []core.Node{core.View{Style: core.Style{Width: driver.Pt(100), Height: driver.Pt(100)},
				Responder: &core.Responder{Claims: []core.TouchClaim{{}}, OnGrant: say("grant"), OnMove: say("move"),
					OnRelease: say("release")}}}}
	}
}

// TestKeysCaretAndPasteInAField focuses a field that reports its caret
// and pastes: a plain key binding leaves the field its typing, one that
// asks fires there and once while held, the caret is reported, and a
// paste is reported with the clipboard's text, kept out of the field.
func TestKeysCaretAndPasteInAField(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-native-keys"
	app := func(cx *core.Scope) core.Node {
		x, down := core.NewSignal(cx, 0), core.NewSignal(cx, 0)
		caret := core.NewSignal(cx, false)
		pasted := core.NewSignal(cx, "")
		cx.OnKey("x", func() { x.Update(func(n int) int { return n + 1 }) })
		cx.OnKeyWith("down", core.KeyOptions{When: core.KeyAlways}, func() { down.Update(func(n int) int { return n + 1 }) })
		cx.Effect(func() {
			cx.SetTitle(fmt.Sprintf("%s|%d|%d|%v|%s", title, x.Get(), down.Get(), caret.Get(), pasted.Get()))
		})
		return core.View{Style: core.Style{Padding: driver.All(20), Background: driver.RGB(0xffffff)}, Children: []core.Node{
			core.Input{Style: core.Style{Height: driver.Pt(30)}, PasteBehavior: driver.PasteReport,
				OnCaretMove: func(e core.CaretEvent) { caret.Set(e.Bounds.Size.Height > 0) },
				OnPaste:     func(e driver.PasteEvent) { pasted.Set(e.Text) }},
		}}
	}
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer close(done)
		Run(app, Options{Title: title + "|start", Width: 360, Height: 240})
	}()
	hwnd := waitForWindow(t, title+"|0|0|false|", done)
	if hwnd == 0 {
		return
	}
	t.Cleanup(func() {
		postMessageW.Call(hwnd, wmClose, 0, 0)
		<-done
	})
	var field *textInput
	onMain(t, hwnd, func(w *window) {
		for _, v := range w.views {
			if v != nil && v.field != nil {
				field = v.field
			}
		}
		w.setFocus(field)
	})
	waitForText(t, hwnd, title+"|0|0|true|", done)
	const vkX, vkDown, repeat = 0x58, 0x28, 1 << 30
	postMessageW.Call(hwnd, wmKeyDown, vkX, 1)
	postMessageW.Call(hwnd, wmKeyDown, vkDown, 1)
	postMessageW.Call(hwnd, wmKeyDown, vkDown, 1|repeat) // held: once
	waitForText(t, hwnd, title+"|0|1|true|", done)
	onMain(t, hwnd, func(w *window) {
		w.setClipboard("pasted text")
		w.paste(field)
	})
	waitForText(t, hwnd, title+"|0|1|true|pasted text", done)
	onMain(t, hwnd, func(w *window) {
		if got := field.model.text; got != "x" {
			t.Errorf("the field reads %q, want the x typed and the paste kept out", got)
		}
	})
}

// TestTouchAreasDecideWhatTakesAPress clicks a menu outside its parent,
// a small target by its hit slop and a button under a veil that takes no
// pointer. Client pixels are points at the test machine's scale.
func TestTouchAreasDecideWhatTakesAPress(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-native-areas"
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer close(done)
		Run(touchAreasApp(title), Options{Title: title + "|start", Width: 360, Height: 240})
	}()
	hwnd := waitForWindow(t, title+"|none", done)
	if hwnd == 0 {
		return
	}
	t.Cleanup(func() {
		postMessageW.Call(hwnd, wmClose, 0, 0)
		<-done
	})
	scale := 1.0
	if dpi, _, _ := getDpiForSystem.Call(); dpi != 0 {
		scale = float64(dpi) / 96
	}
	for _, c := range []struct {
		x, y float64
		want string
	}{{60, 60, "menu"}, {225, 25, "slop"}, {200, 140, "under"}} {
		at := uintptr(uint32(c.y*scale)<<16 | uint32(c.x*scale)&0xffff)
		postMessageW.Call(hwnd, wmLButtonDown, mkLButton, at)
		postMessageW.Call(hwnd, wmLButtonUp, 0, at)
		waitForText(t, hwnd, title+"|"+c.want, done)
	}
}

func touchAreasApp(title string) func(*core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		last := core.NewSignal(cx, "none")
		cx.Effect(func() { cx.SetTitle(title + "|" + last.Get()) })
		press := func(name string) func(core.PressEvent) { return func(core.PressEvent) { last.Set(name) } }
		return core.View{Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(200), Background: driver.RGB(0xffffff)},
			Children: []core.Node{
				// A menu 30 points below its field, outside it.
				core.View{Style: core.Style{Position: core.Absolute, Left: driver.Pt(10), Top: driver.Pt(10), Width: driver.Pt(100),
					Height: driver.Pt(30)}, Children: []core.Node{
					core.View{OnPress: press("menu"), Style: core.Style{Position: core.Absolute, Top: driver.Pt(30),
						Width: driver.Pt(100), Height: driver.Pt(40), Background: driver.RGB(0x123456)}},
				}},
				// A 10-point target with 20 points of slop.
				core.View{OnPress: press("slop"), HitSlop: driver.All(20), Style: core.Style{Position: core.Absolute,
					Left: driver.Pt(200), Top: driver.Pt(20), Width: driver.Pt(10), Height: driver.Pt(10), Background: driver.RGB(0x123456)}},
				// A veil over a button lets its presses through.
				core.View{OnPress: press("under"), Style: core.Style{Position: core.Absolute, Left: driver.Pt(150),
					Top: driver.Pt(120), Width: driver.Pt(100), Height: driver.Pt(40), Background: driver.RGB(0x123456)}},
				core.View{OnPress: press("veil"), PointerEvents: driver.PointerEventsNone, Style: core.Style{Position: core.Absolute,
					Left: driver.Pt(140), Top: driver.Pt(110), Width: driver.Pt(120), Height: driver.Pt(60)}},
			}}
	}
}

// TestPointerEventsReportPositions moves the mouse over a view and
// clicks it twice: the move and the presses report where they are in the
// view, 10 points from where they are in the surface, and the second
// press counts two clicks.
func TestPointerEventsReportPositions(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-native-pointer"
	app := func(cx *core.Scope) core.Node {
		hover, press := core.NewSignal(cx, core.MouseEvent{}), core.NewSignal(cx, core.PressEvent{})
		cx.Effect(func() {
			h, p := hover.Get(), press.Get()
			cx.SetTitle(fmt.Sprintf("%s|%.0f,%.0f|%.0f,%.0f|%d", title, h.PagePosition.X-h.Position.X,
				h.PagePosition.Y-h.Position.Y, p.PagePosition.X-p.Position.X, p.PagePosition.Y-p.Position.Y, p.Clicks))
		})
		return core.View{Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(200), Padding: driver.All(10),
			Background: driver.RGB(0xffffff)}, Children: []core.Node{
			core.View{Style: core.Style{Width: driver.Pt(100), Height: driver.Pt(80), Background: driver.RGB(0x123456)},
				OnMouseMove: hover.Set, OnPress: press.Set},
		}}
	}
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer close(done)
		Run(app, Options{Title: title + "|start", Width: 360, Height: 240})
	}()
	hwnd := waitForWindow(t, title+"|0,0|0,0|0", done)
	if hwnd == 0 {
		return
	}
	t.Cleanup(func() {
		postMessageW.Call(hwnd, wmClose, 0, 0)
		<-done
	})
	at := uintptr(uint32(50)<<16 | 60)
	postMessageW.Call(hwnd, wmMouseMove, 0, at)
	waitForText(t, hwnd, title+"|10,10|0,0|0", done)
	for range 2 {
		postMessageW.Call(hwnd, wmLButtonDown, mkLButton, at)
		postMessageW.Call(hwnd, wmLButtonUp, 0, at)
	}
	waitForText(t, hwnd, title+"|10,10|10,10|2", done)
}

// TestHoverFollowsThePointer moves the mouse onto a view, off it and
// back, then out of the window. Windows may report the mouse leaving at
// once, as the real cursor is elsewhere, so only the counts the moves
// settle on are checked.
func TestHoverFollowsThePointer(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-native-hover"
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer close(done)
		Run(hoverApp(title), Options{Title: title + "|0|0", Width: 360, Height: 240})
	}()
	hwnd := waitForWindow(t, title+"|0|0", done)
	if hwnd == 0 {
		return
	}
	t.Cleanup(func() {
		postMessageW.Call(hwnd, wmClose, 0, 0)
		<-done
	})
	waitForText(t, hwnd, title+"|0|0", done)
	move := func(x, y int32) { postMessageW.Call(hwnd, wmMouseMove, 0, uintptr(uint32(y)<<16|uint32(x)&0xffff)) }
	move(60, 50)
	move(200, 150)
	waitForText(t, hwnd, title+"|1|1", done)
	move(60, 50)
	postMessageW.Call(hwnd, wmMouseLeave, 0, 0)
	waitForText(t, hwnd, title+"|2|2", done)
}

// TestStickyListRowsStayPinned turns the wheel over a list whose first
// row sticks: the row stays at the top, moved natively from its frame.
func TestStickyListRowsStayPinned(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-native-" + t.Name()
	var offset atomic.Int64
	app := func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{Width: driver.Pt(360), Height: driver.Pt(240), Background: driver.RGB(0xffffff)},
			Children: []core.Node{core.List{HideIndicator: true, OnScroll: func(e core.ScrollEvent) { offset.Store(int64(e.Offset.Y)) },
				Count: func() int { return 40 }, StickyHeaderIndices: []int{0}, EstimatedSize: 20,
				Style: core.Style{Width: driver.Pt(40), Height: driver.Pt(100)},
				Row: func(cx *core.Scope, r core.ListRow) core.Node {
					return core.View{StyleFunc: func() core.Style {
						if r.Index() == 0 {
							return core.Style{Height: driver.Pt(20), Background: driver.RGB(0xff0000)}
						}
						return core.Style{Height: driver.Pt(20), Background: driver.RGB(0x0000ff)}
					}}
				}}}}
	}
	hwnd := startApp(t, title, app, 360, 240)
	at := point{px(20), px(20)}
	clientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&at)))
	time.Sleep(300 * time.Millisecond)
	postMessageW.Call(hwnd, wmMouseWheel, uintptr(uint32(uint16(0xff88))<<16), uintptr(uint32(at.Y)<<16|uint32(at.X)&0xffff))
	deadline := time.Now().Add(5 * time.Second)
	for {
		pix, w, _, ok := clientPixels(hwnd)
		if ok && offset.Load() > 20 {
			bgr := func(x, y float64) []byte { i := (px(y)*w + px(x)) * 4; return pix[i : i+3] }
			top, below := bgr(20, 10), bgr(20, 50)
			if top[2] == 0xff && top[0] == 0 && below[0] == 0xff && below[2] == 0 {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("offset %d: the sticky row is not pinned at the top", offset.Load())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

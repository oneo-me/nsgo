//go:build windows

package windows

import (
	"strings"
	"unsafe"

	"github.com/oneo-me/nsgo/internal/driver"
)

// The window's chrome, as NSH's Windows backend has it:
//
//   - The hidden title bar: WM_NCCALCSIZE gives the client area the
//     window's whole height (the side and bottom resize borders stay
//     non-client), WM_NCHITTEST answers the top resize border, the caption
//     buttons (HTMINBUTTON, HTMAXBUTTON, HTCLOSE, so Windows 11 shows snap
//     layouts) and the views that drag the window (HTCAPTION: the system
//     moves the window, maximizes it on a double click and opens its menu
//     on a right click; with the system's title bar too). The caption
//     buttons are drawn over the app in Windows 11's style, and their
//     non-client clicks handled here, never by DefWindowProc, which would
//     draw Windows 95's.
//   - Full screen: a borderless window the size of its monitor.
//   - The menu bar: a Win32 menu (SetMenu), not shown while the title bar
//     is hidden or in full screen; its shortcuts are matched on key downs
//     after the app's own, the editing roles excepted, which the focused
//     field handles itself.

var (
	setWindowPos           = user32.NewProc("SetWindowPos")
	isZoomed               = user32.NewProc("IsZoomed")
	getDpiForWindow        = user32.NewProc("GetDpiForWindow")
	getSystemMetricsForDpi = user32.NewProc("GetSystemMetricsForDpi")
	trackMouseEvent        = user32.NewProc("TrackMouseEvent")
	getWindowPlacement     = user32.NewProc("GetWindowPlacement")
	setWindowPlacement     = user32.NewProc("SetWindowPlacement")
	monitorFromWindow      = user32.NewProc("MonitorFromWindow")
	monitorFromRect        = user32.NewProc("MonitorFromRect")
	getMonitorInfoW        = user32.NewProc("GetMonitorInfoW")
	createMenu             = user32.NewProc("CreateMenu")
	createPopupMenu        = user32.NewProc("CreatePopupMenu")
	appendMenuW            = user32.NewProc("AppendMenuW")
	setMenu                = user32.NewProc("SetMenu")
	destroyMenu            = user32.NewProc("DestroyMenu")
	drawMenuBar            = user32.NewProc("DrawMenuBar")
	dwmGetWindowAttribute  = dwmapi.NewProc("DwmGetWindowAttribute")
)

const (
	wmClose           = 0x0010
	wmGetMinMaxInfo   = 0x0024
	wmNCCalcSize      = 0x0083
	wmNCHitTest       = 0x0084
	wmNCActivate      = 0x0086
	wmNCMouseMove     = 0x00A0
	wmNCLButtonDown   = 0x00A1
	wmNCLButtonUp     = 0x00A2
	wmNCLButtonDblClk = 0x00A3
	wmNCMouseLeave    = 0x02A2

	htClient    = 1
	htCaption   = 2
	htMinButton = 8
	htMaxButton = 9
	htTop       = 12
	htTopLeft   = 13
	htTopRight  = 14
	htClose     = 20

	smCyFrame                        = 33
	smCxPaddedBorder                 = 92
	dwmwaVisibleFrameBorderThickness = 37

	swpNoSize        = 0x0001
	swpNoMove        = 0x0002
	swpNoZOrder      = 0x0004
	swpNoActivate    = 0x0010
	swpFrameChanged  = 0x0020
	swpNoOwnerZOrder = 0x0200

	swMaximize = 3
	swMinimize = 6
	swRestore  = 9

	wsPopup            = 0x80000000
	monitorDefaultNear = 2
	tmeLeave           = 0x02
	tmeNonClient       = 0x10

	mfString    = 0x0000
	mfGrayed    = 0x0001
	mfChecked   = 0x0008
	mfPopup     = 0x0010
	mfSeparator = 0x0800
	// mfRightOrder lays a menu out right to left: its items' text, and
	// the bar's and its submenus' order.
	mfRightOrder = 0x2000
)

// The caption buttons after Onevra's: 32 point squares 4 apart, their
// glyphs 10, as Windows' own are at 96 DPI, centered in the app's bar
// and 8 points in from the right edge; the bar is 32 tall unless the app
// says otherwise.
const (
	defaultCaptionHeight driver.Points = 32
	captionButtonSize    driver.Points = 32
	captionButtonGap     driver.Points = 4
	captionMargin        driver.Points = 8
	captionGlyphSize     driver.Points = 10
	// captionRight is how far the buttons reach in from the right edge.
	captionRight = captionMargin + 3*captionButtonSize + 2*captionButtonGap
)

type ncCalcSizeParams struct {
	Rects [3]rect
	Pos   uintptr
}

type trackMouse struct {
	Size      uint32
	Flags     uint32
	Track     uintptr
	HoverTime uint32
}

// minMaxInfo is MINMAXINFO.
type minMaxInfo struct {
	Reserved     point
	MaxSize      point
	MaxPosition  point
	MinTrackSize point
	MaxTrackSize point
}

type windowPlacement struct {
	Length, Flags, ShowCmd uint32
	MinPosition            point
	MaxPosition            point
	NormalPosition         rect
}

type monitorInfo struct {
	Size    uint32
	Monitor rect
	Work    rect
	Flags   uint32
}

// chrome is what the window keeps of its chrome.
type chrome struct {
	titleBar driver.TitleBarStyle
	height   driver.Points // the app's title bar height; 0 for the default
	// noButtons is set while the app draws its own caption buttons.
	noButtons bool
	hover     uint32 // the caption button under the mouse (HT*), 0 for none
	pressed   uint32
	tracking  bool // the mouse is tracked for WM_NCMOUSELEAVE
	inactive  bool

	// constraints are the app's limits on the window.
	constraints driver.WindowConstraints

	fullScreen bool
	savedStyle uintptr
	savedPlace windowPlacement

	menu    []driver.MenuEntry // nil: no menu bar, Windows' default
	bar     uintptr
	entries []menuRef          // the bar's commands; a command's id is its index + 1
	keys    map[string]menuRef // the menu's shortcuts
	// menuBar says when the bar shows; with MenuBarAutoHide, revealed is
	// set while Alt alone showed it, menuOpen while a key opened a menu,
	// and altAlone while Alt is down with no other key.
	menuBar                      driver.MenuBarMode
	revealed, menuOpen, altAlone bool
}

// menuRef is a menu item's ID and role.
type menuRef struct {
	id        uint32
	role      driver.MenuRole
	notInText bool
}

// hidden reports whether the title bar is hidden now: full screen has
// none.
func (w *window) hidden() bool {
	return w.chrome.titleBar == driver.TitleBarHidden && !w.chrome.fullScreen
}

// captionHeight is the app's bar height, and at least Windows' own.
func (w *window) captionHeight() driver.Points {
	return max(w.chrome.height, defaultCaptionHeight)
}

// titleBarArea is what the hidden title bar's window controls cover.
func (w *window) titleBarArea() driver.Edges {
	if !w.hidden() {
		return driver.Edges{}
	}
	if w.chrome.noButtons {
		return driver.Edges{Top: w.captionHeight()}
	}
	return driver.Edges{Top: w.captionHeight(), Right: captionRight}
}

// buttons reports whether the caption buttons show: in a hidden title
// bar the app did not take them from.
func (w *window) buttons() bool { return w.hidden() && !w.chrome.noButtons }

func (w *window) zoomed() bool {
	r, _, _ := isZoomed.Call(w.hwnd)
	return r != 0
}

// frameThickness is the resize border in pixels, also what a maximized
// window hangs off its monitor by.
func (w *window) frameThickness() int32 {
	dpi, _, _ := getDpiForWindow.Call(w.hwnd)
	if dpi == 0 {
		dpi = uintptr(w.scale * 96)
	}
	frame, _, _ := getSystemMetricsForDpi.Call(smCyFrame, dpi)
	padded, _, _ := getSystemMetricsForDpi.Call(smCxPaddedBorder, dpi)
	return int32(frame) + int32(padded)
}

// topBorder is the visible border Windows 11 draws along a window's top
// edge, in pixels.
func (w *window) topBorder() int32 {
	if dwmGetWindowAttribute.Find() != nil {
		return 0
	}
	var thickness uint32
	if r, _, _ := dwmGetWindowAttribute.Call(w.hwnd, dwmwaVisibleFrameBorderThickness,
		uintptr(unsafe.Pointer(&thickness)), 4); r != 0 {
		return 0
	}
	return int32(min(thickness, 8))
}

// frameChanged has the window compute its client area again.
func (w *window) frameChanged() {
	setWindowPos.Call(w.hwnd, 0, 0, 0, 0, 0, swpFrameChanged|swpNoMove|swpNoSize|swpNoZOrder|swpNoActivate)
}

// chromeChanged follows a change of the title bar or full screen: the
// menu bar, the area reported to the app and the drawing.
func (w *window) chromeChanged() {
	w.showMenuBar()
	w.updateEnvironment()
	w.redrawAll()
}

// windowCommand carries out an ItemWindow.
func (w *window) windowCommand(c *driver.WindowCommand) {
	switch c.Action {
	case driver.WindowSetTitleBar:
		if c.TitleBar != w.chrome.titleBar || c.Height != w.chrome.height {
			w.chrome.titleBar, w.chrome.height = c.TitleBar, c.Height
			w.frameChanged()
			w.chromeChanged()
		}
	case driver.WindowMinimize:
		w.minimize()
	case driver.WindowToggleMaximize:
		w.toggleMaximize()
	case driver.WindowToggleFullScreen:
		w.toggleFullScreen()
	case driver.WindowClose:
		postMessageW.Call(w.hwnd, wmClose, 0, 0)
	case driver.WindowSetFrame:
		w.setWindowFrame(c.Frame)
	case driver.WindowSetLevel:
		w.setLevel(c.Level)
	case driver.WindowOrderFront:
		w.orderFront()
	case driver.WindowSetMenuBar:
		w.setMenuBar(c.MenuBar)
	case driver.WindowSetControls:
		if c.Controls.Hidden != w.chrome.noButtons {
			w.chrome.noButtons = c.Controls.Hidden
			w.chrome.hover, w.chrome.pressed = 0, 0
			w.chromeChanged()
		}
	case driver.WindowSetMaterial:
		w.setMaterial(c.Material)
	case driver.WindowSetConstraints:
		w.setConstraints(c.Constraints)
	}
}

// constrainedStyle returns style with the frame and the buttons c leaves.
func constrainedStyle(style uintptr, c driver.WindowConstraints) uintptr {
	style |= wsThickFrame | wsMinimizeBox | wsMaximizeBox
	if c.NoResize {
		style &^= wsThickFrame
	}
	if c.NoMinimize {
		style &^= wsMinimizeBox
	}
	if c.NoMaximize {
		style &^= wsMaximizeBox
	}
	return style
}

// setConstraints limits the window and moves it into its new sizes; in
// full screen they wait for the style it goes back to.
func (w *window) setConstraints(c driver.WindowConstraints) {
	w.chrome.constraints = c
	w.chrome.hover, w.chrome.pressed = 0, 0
	if w.chrome.fullScreen {
		w.chrome.savedStyle = constrainedStyle(w.chrome.savedStyle, c)
		return
	}
	style, _, _ := getWindowLongPtrW.Call(w.hwnd, gwlStyle)
	setWindowLongPtrW.Call(w.hwnd, gwlStyle, constrainedStyle(style, c))
	w.frameChanged()
	w.redrawAll()
	if w.zoomed() || isIconicWindow(w.hwnd) {
		return
	}
	var outer, client rect
	getWindowRect.Call(w.hwnd, uintptr(unsafe.Pointer(&outer)))
	getClientRect.Call(w.hwnd, uintptr(unsafe.Pointer(&client)))
	shown := w.point(client.Right, client.Bottom)
	size := driver.Size{Width: shown.X, Height: shown.Y}
	if fit := c.Clamp(size); fit != size {
		setWindowPos.Call(w.hwnd, 0, 0, 0,
			uintptr(outer.Right-outer.Left-client.Right+w.px(fit.Width)),
			uintptr(outer.Bottom-outer.Top-client.Bottom+w.px(fit.Height)),
			swpNoMove|swpNoZOrder|swpNoActivate)
	}
}

// minMaxInfo answers WM_GETMINMAXINFO: the constraints' content sizes
// with what the frame takes. Windows also holds SetWindowPos to these
// sizes, so full screen goes without them, and a minimized window has
// no client area to measure.
func (w *window) minMaxInfo(lParam uintptr) {
	c := &w.chrome.constraints
	if w.chrome.fullScreen || isIconicWindow(w.hwnd) {
		return
	}
	info := *(**minMaxInfo)(unsafe.Pointer(&lParam)) // the message's, valid while it is handled
	var outer, client rect
	getWindowRect.Call(w.hwnd, uintptr(unsafe.Pointer(&outer)))
	getClientRect.Call(w.hwnd, uintptr(unsafe.Pointer(&client)))
	frameX, frameY := outer.Right-outer.Left-client.Right, outer.Bottom-outer.Top-client.Bottom
	if c.MinWidth > 0 {
		info.MinTrackSize.X = w.px(c.MinWidth) + frameX
	}
	if c.MinHeight > 0 {
		info.MinTrackSize.Y = w.px(c.MinHeight) + frameY
	}
	if c.MaxWidth > 0 {
		info.MaxTrackSize.X = w.px(c.MaxWidth) + frameX
		info.MaxSize.X = min(info.MaxSize.X, info.MaxTrackSize.X)
	}
	if c.MaxHeight > 0 {
		info.MaxTrackSize.Y = w.px(c.MaxHeight) + frameY
		info.MaxSize.Y = min(info.MaxSize.Y, info.MaxTrackSize.Y)
	}
}

func (w *window) minimize() {
	if !w.chrome.constraints.NoMinimize {
		showWindow.Call(w.hwnd, swMinimize)
	}
}

func (w *window) toggleMaximize() {
	if w.chrome.fullScreen {
		return
	}
	if w.zoomed() {
		showWindow.Call(w.hwnd, swRestore)
	} else if !w.chrome.constraints.NoMaximize {
		showWindow.Call(w.hwnd, swMaximize)
	}
}

// toggleFullScreen makes the window a borderless one covering its
// monitor, or puts it back as it was.
func (w *window) toggleFullScreen() {
	c := &w.chrome
	if !c.fullScreen {
		var mi monitorInfo
		mi.Size = uint32(unsafe.Sizeof(mi))
		monitor, _, _ := monitorFromWindow.Call(w.hwnd, monitorDefaultNear)
		if r, _, _ := getMonitorInfoW.Call(monitor, uintptr(unsafe.Pointer(&mi))); r == 0 {
			return
		}
		c.savedPlace = windowPlacement{}
		c.savedPlace.Length = uint32(unsafe.Sizeof(c.savedPlace))
		getWindowPlacement.Call(w.hwnd, uintptr(unsafe.Pointer(&c.savedPlace)))
		c.savedStyle, _, _ = getWindowLongPtrW.Call(w.hwnd, gwlStyle)
		c.fullScreen = true
		w.showMenuBar()
		setWindowLongPtrW.Call(w.hwnd, gwlStyle, c.savedStyle&^wsOverlappedWindow|wsPopup)
		m := mi.Monitor
		setWindowPos.Call(w.hwnd, 0, uintptr(m.Left), uintptr(m.Top), uintptr(m.Right-m.Left), uintptr(m.Bottom-m.Top),
			swpNoOwnerZOrder|swpFrameChanged)
	} else {
		c.fullScreen = false
		setWindowLongPtrW.Call(w.hwnd, gwlStyle, c.savedStyle)
		w.showMenuBar()
		setWindowPlacement.Call(w.hwnd, uintptr(unsafe.Pointer(&c.savedPlace)))
		setWindowPos.Call(w.hwnd, 0, 0, 0, 0, 0,
			swpNoMove|swpNoSize|swpNoZOrder|swpNoOwnerZOrder|swpNoActivate|swpFrameChanged)
	}
	w.chromeChanged()
}

// ncCalcSize answers WM_NCCALCSIZE with the title bar hidden: the client
// area reaches the window's top.
func (w *window) ncCalcSize(wParam, lParam uintptr) (uintptr, bool) {
	if wParam == 0 || !w.hidden() {
		return 0, false
	}
	params := *(**ncCalcSizeParams)(unsafe.Pointer(&lParam)) // the message's, valid while it is handled
	top := params.Rects[0].Top
	defWindowProcW.Call(w.hwnd, wmNCCalcSize, wParam, lParam)
	// A maximized window hangs off its monitor by its resize border;
	// another one's top row would be under Windows 11's border.
	if w.zoomed() {
		params.Rects[0].Top = top + w.frameThickness()
	} else {
		params.Rects[0].Top = top + w.topBorder()
	}
	return 0, true
}

// captionAt returns the caption button at p (client points), as its hit
// test code, or 0.
func (w *window) captionAt(p driver.Point) uint32 {
	height := w.captionHeight()
	top := (height - captionButtonSize) / 2
	if !w.buttons() || p.Y < top || p.Y >= top+captionButtonSize {
		return 0
	}
	var client rect
	getClientRect.Call(w.hwnd, uintptr(unsafe.Pointer(&client)))
	width := w.point(client.Right, 0).X
	for i, code := range []uint32{htClose, htMaxButton, htMinButton} {
		right := width - captionMargin - driver.Points(i)*(captionButtonSize+captionButtonGap)
		if p.X >= right-captionButtonSize && p.X < right {
			if !w.captionEnabled(code) {
				return 0
			}
			return code
		}
	}
	return 0
}

// captionEnabled reports whether the app's constraints leave the caption
// button code.
func (w *window) captionEnabled(code uint32) bool {
	c := &w.chrome.constraints
	return !(code == htMinButton && c.NoMinimize || code == htMaxButton && c.NoMaximize)
}

// ncHitTest answers WM_NCHITTEST: the hidden title bar's resize border
// and caption buttons, and the views that drag the window.
func (w *window) ncHitTest(lParam uintptr) uintptr {
	def, _, _ := defWindowProcW.Call(w.hwnd, wmNCHitTest, 0, lParam)
	if def != htClient {
		return def
	}
	at := point{int32(int16(lParam & 0xffff)), int32(int16(lParam >> 16 & 0xffff))}
	screenToClient.Call(w.hwnd, uintptr(unsafe.Pointer(&at)))
	p := w.point(at.X, at.Y)
	if w.hidden() {
		if border := w.frameThickness(); !w.zoomed() && !w.chrome.constraints.NoResize && at.Y < border {
			var client rect
			getClientRect.Call(w.hwnd, uintptr(unsafe.Pointer(&client)))
			switch {
			case at.X < border*2:
				return htTopLeft
			case at.X >= client.Right-border*2:
				return htTopRight
			}
			return htTop
		}
		if code := w.captionAt(p); code != 0 {
			return uintptr(code)
		}
	}
	if w.windowDragAt(p) {
		return htCaption
	}
	return htClient
}

func isButton(code uint32) bool { return code == htMinButton || code == htMaxButton || code == htClose }

func (w *window) setHover(code uint32) {
	if w.chrome.hover != code {
		w.chrome.hover = code
		w.redrawAll()
	}
}

// ncMouse handles the non-client mouse messages over the caption
// buttons, reporting whether it took the message.
func (w *window) ncMouse(message uint32, wParam uintptr) bool {
	c := &w.chrome
	code := uint32(wParam)
	switch message {
	case wmNCMouseMove:
		if !c.tracking {
			c.tracking = true
			t := trackMouse{Flags: tmeLeave | tmeNonClient, Track: w.hwnd}
			t.Size = uint32(unsafe.Sizeof(t))
			trackMouseEvent.Call(uintptr(unsafe.Pointer(&t)))
		}
		if isButton(code) {
			w.setHover(code)
		} else {
			w.setHover(0)
		}
	case wmNCMouseLeave:
		c.tracking = false
		c.pressed = 0
		w.setHover(0)
	case wmNCLButtonDown, wmNCLButtonDblClk:
		if isButton(code) && w.hidden() {
			c.pressed = code
			w.redrawAll()
			return true
		}
	case wmNCLButtonUp:
		if isButton(code) && w.hidden() {
			pressed := c.pressed
			c.pressed = 0
			if pressed == code {
				switch code {
				case htMinButton:
					w.setHover(0)
					showWindow.Call(w.hwnd, swMinimize)
				case htMaxButton:
					w.toggleMaximize()
				default:
					postMessageW.Call(w.hwnd, wmClose, 0, 0)
				}
			}
			w.redrawAll()
			return true
		}
	}
	return false
}

// ncActivate handles WM_NCACTIVATE: the caption buttons dim in an
// inactive window, and with the title bar hidden the default handling
// must not paint the old caption over the app (lParam -1).
func (w *window) ncActivate(wParam, lParam uintptr) uintptr {
	w.chrome.inactive = wParam == 0
	w.redrawAll()
	if w.hidden() {
		lParam = ^uintptr(0)
	}
	r, _, _ := defWindowProcW.Call(w.hwnd, wmNCActivate, wParam, lParam)
	return r
}

// drawCaptionButtons draws the caption buttons over the app, with the
// title bar hidden, at the right of a client area width pixels wide. It
// draws Onevra's: rounded squares in the middle of the bar, the close one
// red under the pointer. It draws in pixels, so the glyphs' lines fall on
// whole ones.
func (p *painter) drawCaptionButtons(width int32) {
	w := p.w
	if !w.buttons() {
		return
	}
	// In the topmost layer, above web views.
	p.realize()
	p.setTransform(scaling(1/w.scale, 1/w.scale))
	c := &w.chrome
	dark := w.shownDark()
	button, gap, margin := w.px(captionButtonSize), w.px(captionButtonGap), w.px(captionMargin)
	height := w.px(w.captionHeight())
	top := (height - button) / 2
	glyph := w.px(captionGlyphSize)
	stroke := float32(max(w.px(1), 1))
	radius := float32(w.px(8))
	for i, code := range []uint32{htClose, htMaxButton, htMinButton} {
		right := width - margin - int32(i)*(button+gap)
		r := rect{right - button, top, right, top + button}
		hovered := c.hover == code
		pressed := hovered && c.pressed == code
		var bg driver.Color
		switch {
		case !hovered:
		case code == htClose && pressed:
			bg = 0xc42b1cff
		case code == htClose:
			bg = 0xe81123ff
		case !dark && pressed:
			bg = 0x33332f24
		case !dark:
			bg = 0x33332f14
		case pressed:
			bg = 0xeeeae224
		default:
			bg = 0xeeeae214
		}
		p.fillRounded(rectF{float32(r.Left), float32(r.Top), float32(r.Right), float32(r.Bottom)}, radius, bg)
		fg := driver.Color(0x72726aff)
		if dark {
			fg = 0xbdb6aaff
		}
		switch {
		case !w.captionEnabled(code):
			fg = fg&^0xff | 0x48
		case code == htClose && hovered:
			fg = 0xffffffff
		case c.inactive && !hovered:
			fg = fg&^0xff | 0x70
		}
		left, glyphTop := float32(r.Left+(button-glyph)/2), float32(top+(button-glyph)/2)
		if code == htMinButton {
			// Onevra sets the minus a little low, as its glyph reads.
			glyphTop += float32(w.px(2))
		}
		size := float32(glyph)
		// box strokes a square's outline inside the pixels from x, y.
		box := func(x, y, side float32) {
			line := rectF{x + stroke/2, y + stroke/2, x + side - stroke/2, y + side - stroke/2}
			p.dc.callF(rtDrawRectangle, 0b100, uintptr(unsafe.Pointer(&line)), p.solid(fg).ptr(), f32(stroke), 0)
		}
		switch {
		case code == htMinButton:
			mid := glyphTop + size/2
			p.fillRect(rectF{left, mid, left + size, mid + stroke}, fg)
		case code == htMaxButton && w.zoomed():
			// Restore: a square with another behind it.
			o := float32(max(w.px(2), 2))
			box(left, glyphTop+o, size-o)
			p.fillRect(rectF{left + o, glyphTop, left + size, glyphTop + stroke}, fg)
			p.fillRect(rectF{left + size - stroke, glyphTop, left + size, glyphTop + size - o}, fg)
			p.fillRect(rectF{left + o, glyphTop, left + o + stroke, glyphTop + o}, fg)
			p.fillRect(rectF{left + size - o, glyphTop + size - o - stroke, left + size, glyphTop + size - o}, fg)
		case code == htMaxButton:
			box(left, glyphTop, size)
		default:
			cross := p.pathGeometry(d2dFillAlternate, func(sink *comObject) {
				for _, l := range [2][2]pointF{{{left, glyphTop}, {left + size, glyphTop + size}}, {{left + size, glyphTop}, {left, glyphTop + size}}} {
					sink.callPoint(sinkBeginFigure, l[0].x, l[0].y, d2dFigureHollow)
					sink.call(sinkAddLines, uintptr(unsafe.Pointer(&l[1])), 1)
					sink.call(sinkEndFigure, d2dFigureOpen)
				}
			})
			if cross != nil {
				p.dc.callF(rtDrawGeometry, 0b100, cross.ptr(), p.solid(fg).ptr(), f32(stroke), 0)
				cross.release()
			}
		}
	}
}

// windowDragAt reports whether a press at p (client points) drags the
// window, as core.Tree.WindowDragAt decides: the nearest view around it
// that sets WindowDrag, unless a view nearer than it takes presses.
func (w *window) windowDragAt(p driver.Point) bool {
	root := w.view(w.root)
	if root == nil {
		return false
	}
	var path []*view
	for v := root; v != nil; {
		self, children := v.reach(p)
		if v.style.Hidden || !self && !children {
			break
		}
		if self {
			path = append(path, v)
		}
		if !children {
			break
		}
		p = driver.Point{X: p.X - v.frame.Origin.X + v.offset.X, Y: p.Y - v.frame.Origin.Y + v.offset.Y}
		var next *view
		for i := len(v.children) - 1; i >= 0; i-- {
			if c := v.children[i]; c.under(p) {
				next = c
				break
			}
		}
		v = next
	}
	for i := len(path) - 1; i >= 0; i-- {
		v := path[i]
		switch v.props.WindowDrag {
		case driver.WindowDragOn:
			return true
		case driver.WindowDragOff:
			return false
		}
		if takesPointer(v) {
			return false
		}
	}
	return false
}

// takesPointer reports whether presses on v are the app's.
func takesPointer(v *view) bool {
	if v.kind == driver.KindInput || v.kind == driver.KindScroll {
		return true
	}
	for _, k := range []driver.EventKind{driver.EventPress, driver.EventPressDown, driver.EventLongPress,
		driver.EventPan, driver.EventPinch, driver.EventRotation, driver.EventContextMenu} {
		if v.events.Has(k) {
			return true
		}
	}
	return false
}

// setMenu replaces the application menu; nil is Windows' default, none.
func (w *window) setMenu(entries []driver.MenuEntry) {
	w.chrome.menu = entries
	w.chrome.keys = map[string]menuRef{}
	w.menuKeys(entries)
	w.showMenuBar()
}

// unsupported reports the roles Windows has no use for.
func unsupported(r driver.MenuRole) bool {
	return r == driver.MenuAbout || r == driver.MenuHide || r == driver.MenuHideOthers || r == driver.MenuShowAll
}

// shortcutOf returns e's shortcut, its role's when it has none.
func shortcutOf(e *driver.MenuEntry) string {
	if e.Shortcut != "" {
		return e.Shortcut
	}
	return e.Role.Shortcut(false)
}

// menuKeys gathers the shortcuts of entries' items, but the editing
// roles': the focused field handles those keys itself.
func (w *window) menuKeys(entries []driver.MenuEntry) {
	for i := range entries {
		e := &entries[i]
		switch {
		case e.Separator, e.Disabled, unsupported(e.Role):
		case len(e.Items) > 0:
			w.menuKeys(e.Items)
		default:
			if k := shortcutOf(e); k != "" && !e.Role.EditsText() {
				if _, taken := w.chrome.keys[k]; !taken {
					w.chrome.keys[k] = menuRef{e.ID, e.Role, e.NotInText}
				}
			}
		}
	}
}

// showMenuBar shows the application menu as the window's menu bar,
// unless the title bar is hidden, the window is in full screen or the
// bar hides until Alt shows it.
func (w *window) showMenuBar() {
	c := &w.chrome
	c.entries = nil
	var bar uintptr
	shown := c.menuBar == driver.MenuBarShown || c.revealed || c.menuOpen
	if c.menu != nil && c.titleBar == driver.TitleBarDefault && !c.fullScreen && shown {
		bar, _, _ = createMenu.Call()
		if bar != 0 {
			w.appendMenu(bar, c.menu)
		}
	}
	if bar == 0 && c.bar == 0 {
		return
	}
	setMenu.Call(w.hwnd, bar)
	if c.bar != 0 {
		destroyMenu.Call(c.bar)
	}
	c.bar = bar
	drawMenuBar.Call(w.hwnd)
}

// menuFlags are the flags of every item of a menu: right to left in an
// app laid out so, whatever the system's direction.
func (w *window) menuFlags() uintptr {
	if w.rtl {
		return mfString | mfRightOrder
	}
	return mfString
}

// popupFlags are TrackPopupMenuEx's flags of a context menu, which opens
// leftward in an app laid out right to left.
func (w *window) popupFlags() uintptr {
	if w.rtl {
		return tpmReturnCmd | tpmRightButton | tpmLayoutRTL | tpmRightAlign
	}
	return tpmReturnCmd | tpmRightButton
}

// appendMenu appends entries to the menu bar's menu, numbering their
// commands.
func (w *window) appendMenu(menu uintptr, entries []driver.MenuEntry) {
	w.appendMenuTo(menu, entries, &w.chrome.entries)
}

// appendMenuTo appends entries to menu, numbering their commands by
// their index in refs plus one.
func (w *window) appendMenuTo(menu uintptr, entries []driver.MenuEntry, refs *[]menuRef) {
	lastSeparator := true
	for i := range entries {
		e := &entries[i]
		switch {
		case e.Separator:
			if !lastSeparator {
				appendMenuW.Call(menu, mfSeparator, 0, 0)
				lastSeparator = true
			}
			continue
		case unsupported(e.Role):
			continue
		}
		label := strings.ReplaceAll(e.Label, "&", "&&")
		flags := w.menuFlags()
		if e.Disabled {
			flags |= mfGrayed
		}
		if len(e.Items) > 0 {
			sub, _, _ := createPopupMenu.Call()
			if sub == 0 {
				continue
			}
			w.appendMenuTo(sub, e.Items, refs)
			appendMenuW.Call(menu, flags|mfPopup, sub, uintptr(unsafe.Pointer(wide(label))))
		} else {
			if k := shortcutOf(e); k != "" {
				label += "\t" + shortcutLabel(k)
			}
			if e.Checked {
				flags |= mfChecked
			}
			*refs = append(*refs, menuRef{e.ID, e.Role, e.NotInText})
			appendMenuW.Call(menu, flags, uintptr(len(*refs)), uintptr(unsafe.Pointer(wide(label))))
		}
		lastSeparator = false
	}
}

// shortcutLabel shows a normalized shortcut as Windows' menus do, such
// as "Ctrl+Shift+N".
func shortcutLabel(key string) string {
	parts := strings.Split(key, "+")
	for i, part := range parts {
		switch part {
		case "mod", "ctrl":
			parts[i] = "Ctrl"
		case "alt":
			parts[i] = "Alt"
		case "shift":
			parts[i] = "Shift"
		case "meta":
			parts[i] = "Win"
		case "pageup":
			parts[i] = "PgUp"
		case "pagedown":
			parts[i] = "PgDn"
		case "delete":
			parts[i] = "Del"
		case "escape":
			parts[i] = "Esc"
		default:
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "+")
}

// menuCommand handles a WM_COMMAND from the menu bar.
func (w *window) menuCommand(command uint16) {
	if i := int(command) - 1; i >= 0 && i < len(w.chrome.entries) {
		w.chooseMenu(w.chrome.entries[i])
	}
}

// menuShortcut chooses the item with shortcut key, reporting whether
// there is one.
func (w *window) menuShortcut(key string) bool {
	ref, ok := w.chrome.keys[key]
	if ok && ref.notInText && w.editing() {
		return false // the field's or the page's key
	}
	if ok {
		w.chooseMenu(ref)
	}
	return ok
}

// chooseMenu carries out an item's role and tells the app about it.
func (w *window) chooseMenu(ref menuRef) {
	switch ref.role {
	case driver.MenuQuit, driver.MenuClose:
		postMessageW.Call(w.hwnd, wmClose, 0, 0)
	case driver.MenuMinimize:
		w.minimize()
	case driver.MenuZoom:
		w.toggleMaximize()
	case driver.MenuToggleFullScreen:
		w.toggleFullScreen()
	case driver.MenuUndo, driver.MenuRedo, driver.MenuCut, driver.MenuCopy, driver.MenuPaste, driver.MenuSelectAll:
		// The focused field's own keys.
		vk := map[driver.MenuRole]uintptr{driver.MenuUndo: 'Z', driver.MenuRedo: 'Y', driver.MenuCut: 'X',
			driver.MenuCopy: 'C', driver.MenuPaste: 'V', driver.MenuSelectAll: 'A'}[ref.role]
		w.editKey(vk, true, false, false)
	}
	if ref.id != 0 {
		w.engine.Dispatch(driver.Event{Kind: driver.EventMenu, Count: ref.id})
	}
}

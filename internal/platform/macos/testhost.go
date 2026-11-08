//go:build darwin && !ios && nsgotest

package macos

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
	"github.com/oneo-me/nsgo/internal/testhost"
)

// The test host: nsgo test -target macos runs each test in a window of
// its own, acting with events posted to the application's queue, so they
// take the path a user's do, its key and mouse monitors included.

func init() { testhost.Register(host{}) }

type host struct{}

func (host) Main(run func() int) {
	nsApp := startApp(Options{Title: "NSGo test"})
	go func() {
		code := run()
		os.Exit(code)
	}()
	nsApp.Send(sel("run"))
}

func (host) Open(app func(cx *core.Scope) core.Node, o testhost.Options) (testhost.Surface, error) {
	var s *surface
	test := &testWindow{options: o}
	onMainSync(func() {
		s = openWindow(app, Options{Title: "NSGo test", Width: o.Size.Width, Height: o.Size.Height, Perf: o.FPS}, test)
		// In front, as a user sees it, but without taking the keyboard
		// from the app the user works in.
		s.window.Send(sel("orderFrontRegardless"))
		s.window.Send(sel("makeKeyWindow"))
	})
	return &testSurface{s: s, test: test}, nil
}

// onMainSync runs fn on the main thread and waits for it. It goes through
// the run loop in its common modes, as batches do, rather than the main
// queue: a context menu tracks in a loop of its own inside a main-queue
// block, which holds the queue until the menu closes.
func onMainSync(fn func()) {
	done := make(chan struct{})
	callsMu.Lock()
	calls = append(calls, func() {
		defer close(done)
		fn()
	})
	callsMu.Unlock()
	if caller == 0 {
		callerOnce.Do(func() {
			class := apple.RegisterClass("NsgoTestCaller", objc.GetClass("NSObject"), nil, []objc.MethodDef{{
				Cmd: sel("call"),
				Fn: func(objc.ID, objc.SEL) {
					callsMu.Lock()
					fns := calls
					calls = nil
					callsMu.Unlock()
					for _, fn := range fns {
						fn()
					}
				},
			}})
			caller = objc.ID(class).Send(selNew)
		})
	}
	caller.Send(sel("performSelectorOnMainThread:withObject:waitUntilDone:"), sel("call"), objc.ID(0), false)
	<-done
}

var (
	caller     objc.ID
	callerOnce sync.Once
	callsMu    sync.Mutex
	calls      []func()
)

type testSurface struct {
	s       *surface
	test    *testWindow
	hovered []objc.ID
}

// Event types and modifier flags, from NSEvent.h.
const (
	evLeftDown, evLeftUp, evRightDown, evRightUp, evMoved, evLeftDragged = 1, 2, 3, 4, 5, 6
	evKeyDown, evKeyUp                                                   = 10, 11
	flagShift, flagControl, flagOption, flagCommand                      = 1 << 17, 1 << 18, 1 << 19, 1 << 20
)

// windowPoint is p, in the surface's points from its top left, in the
// window's base coordinates.
func (t *testSurface) windowPoint(p driver.Point) apple.CGPoint {
	return objc.Send[apple.CGPoint](t.s.native, sel("convertPoint:toView:"), apple.CGPoint{X: float64(p.X), Y: float64(p.Y)}, objc.ID(0))
}

func uptime() float64 {
	return objc.Send[float64](objc.ID(objc.GetClass("NSProcessInfo")).Send(sel("processInfo")), sel("systemUptime"))
}

// mouse posts a mouse event of kind at p to the application's queue.
func (t *testSurface) mouse(kind uint, p driver.Point, clicks int) {
	onMainSync(func() {
		w := t.s.window
		ev := objc.ID(objc.GetClass("NSEvent")).Send(
			sel("mouseEventWithType:location:modifierFlags:timestamp:windowNumber:context:eventNumber:clickCount:pressure:"),
			kind, t.windowPoint(p), uint(0), uptime(), objc.Send[int](w, sel("windowNumber")), objc.ID(0), 0, clicks, float32(1))
		nsApp().Send(sel("postEvent:atStart:"), ev, false)
	})
}

func (t *testSurface) Press(at driver.Point, hold time.Duration) error {
	t.mouse(evMoved, at, 0)
	t.mouse(evLeftDown, at, 1)
	if hold > 0 {
		time.Sleep(hold + 50*time.Millisecond)
	}
	t.mouse(evLeftUp, at, 1)
	return nil
}

func (t *testSurface) RightClick(at driver.Point) error {
	t.mouse(evRightDown, at, 1)
	t.mouse(evRightUp, at, 1)
	return nil
}

// Drag rests where it lands, then moves 10 points a frame, as a hand.
func (t *testSurface) Drag(path []driver.Point) error {
	if len(path) == 0 {
		return errors.New("an empty drag")
	}
	t.mouse(evMoved, path[0], 0)
	t.mouse(evLeftDown, path[0], 1)
	time.Sleep(100 * time.Millisecond)
	at := path[0]
	for _, to := range path[1:] {
		for _, p := range steps(at, to, 10) {
			t.mouse(evLeftDragged, p, 1)
			time.Sleep(16 * time.Millisecond)
		}
		at = to
	}
	t.mouse(evLeftUp, at, 1)
	return nil
}

// steps are the points from a (excluded) to b, at most step apart.
func steps(a, b driver.Point, step driver.Points) []driver.Point {
	dx, dy := b.X-a.X, b.Y-a.Y
	n := int(max(abs(dx), abs(dy))/step) + 1
	out := make([]driver.Point, n)
	for i := range out {
		f := driver.Points(i+1) / driver.Points(n)
		out[i] = driver.Point{X: a.X + dx*f, Y: a.Y + dy*f}
	}
	return out
}

var (
	cgEventCreateScrollWheelEvent2 func(source uintptr, units uint32, count uint32, w1, w2, w3 int32) uintptr
	cgEventSetIntegerValueField    func(event uintptr, field uint32, value int64)
	cgEventSetLocation             func(event uintptr, p apple.CGPoint)
)

func loadCG() {
	if cgEventSetLocation != nil {
		return
	}
	cg, err := purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_NOW)
	if err != nil {
		panic(err)
	}
	purego.RegisterLibFunc(&cgEventCreateScrollWheelEvent2, cg, "CGEventCreateScrollWheelEvent2")
	purego.RegisterLibFunc(&cgEventSetIntegerValueField, cg, "CGEventSetIntegerValueField")
	purego.RegisterLibFunc(&cgEventSetLocation, cg, "CGEventSetLocation")
}

// Scroll is a trackpad gesture: it begins, moves 20 points at most every
// 16 ms, and ends without momentum.
func (t *testSurface) Scroll(at driver.Point, delta driver.Point) error {
	loadCG()
	const pixel, scrollPhase, phaseBegan, phaseChanged, phaseEnded = 0, 99, 1, 2, 4
	// A trackpad's deltas are continuous, in points.
	const continuous = 88
	n := int(max(abs(delta.X), abs(delta.Y))/20) + 1
	// The gesture finds its view with AppKit's geometry, as the window's
	// monitor has it do.
	onMainSync(t.s.commitPending)
	send := func(phase int64, dx, dy driver.Points) {
		onMainSync(func() {
			// The wheel's deltas are the content's way: positive shows what
			// is above or left.
			ev := cgEventCreateScrollWheelEvent2(0, pixel, 2, int32(-dy), int32(-dx), 0)
			defer apple.Release(ev)
			cgEventSetIntegerValueField(ev, scrollPhase, phase)
			cgEventSetIntegerValueField(ev, continuous, 1)
			cgEventSetLocation(ev, t.screenPoint(at))
			ns := objc.ID(objc.GetClass("NSEvent")).Send(sel("eventWithCGEvent:"), ev)
			// AppKit hands wheels to the window under the system's pointer,
			// which a test leaves where the user has it: the view under at
			// takes it, and passes it up as a real one.
			if hit := t.hit(at); hit != 0 {
				hit.Send(sel("scrollWheel:"), ns)
			}
		})
		time.Sleep(16 * time.Millisecond)
	}
	send(phaseBegan, 0, 0)
	for range n {
		send(phaseChanged, delta.X/driver.Points(n), delta.Y/driver.Points(n))
	}
	send(phaseEnded, 0, 0)
	return nil
}

// screenPoint is p in the global display coordinates CGEvents use: from
// the main screen's top left.
func (t *testSurface) screenPoint(p driver.Point) apple.CGPoint {
	wp := t.windowPoint(p)
	sp := objc.Send[apple.CGPoint](t.s.window, sel("convertPointToScreen:"), wp)
	screens := objc.ID(objc.GetClass("NSScreen")).Send(sel("screens"))
	main := screens.Send(sel("objectAtIndex:"), uint(0))
	h := objc.Send[apple.CGRect](main, sel("frame")).Size.Height
	return apple.CGPoint{X: sp.X, Y: h - sp.Y}
}

// hit is the view at at, as the window's hit testing finds it.
func (t *testSurface) hit(at driver.Point) objc.ID {
	root := t.s.native
	wp := t.windowPoint(at)
	return root.Send(sel("hitTest:"), objc.Send[apple.CGPoint](root.Send(sel("superview")), sel("convertPoint:fromView:"), wp, objc.ID(0)))
}

// Hover moves the mouse over at: posted moves reach only the first
// responder, so the views under it hear what their tracking areas would
// report.
func (t *testSurface) Hover(at driver.Point) error {
	onMainSync(func() {
		t.s.commitPending()
		var under []objc.ID
		root := t.s.native
		if at.X >= 0 && at.Y >= 0 {
			for n := t.hit(at); n != 0 && n != root; n = n.Send(selSuperview) {
				if v := views[n]; v != nil && v.native == n {
					under = append(under, n)
				}
			}
		}
		in := func(list []objc.ID, n objc.ID) bool {
			for _, o := range list {
				if o == n {
					return true
				}
			}
			return false
		}
		for _, n := range t.hovered {
			// A view the app removed since is gone with its events.
			if v := views[n]; v != nil && v.native == n && !in(under, n) {
				n.Send(sel("mouseExited:"), objc.ID(0))
			}
		}
		for i := len(under) - 1; i >= 0; i-- {
			if !in(t.hovered, under[i]) {
				under[i].Send(sel("mouseEntered:"), objc.ID(0))
			}
		}
		t.hovered = under
		if len(under) > 0 {
			ev := objc.ID(objc.GetClass("NSEvent")).Send(
				sel("mouseEventWithType:location:modifierFlags:timestamp:windowNumber:context:eventNumber:clickCount:pressure:"),
				uint(evMoved), t.windowPoint(at), uint(0), uptime(), objc.Send[int](t.s.window, sel("windowNumber")), objc.ID(0), 0, 0, float32(0))
			for _, n := range under {
				n.Send(sel("mouseMoved:"), ev)
			}
		}
	})
	return nil
}

// keyCodes are the virtual key codes of the keys tests name, from
// Carbon's Events.h.
var keyCodes = map[string]uint16{
	"enter": 36, "return": 36, "tab": 48, "space": 49, "backspace": 51, "escape": 53, "delete": 117,
	"left": 123, "right": 124, "down": 125, "up": 126, "home": 115, "end": 119, "pageup": 116, "pagedown": 121,
	"a": 0, "s": 1, "d": 2, "f": 3, "h": 4, "g": 5, "z": 6, "x": 7, "c": 8, "v": 9, "b": 11, "q": 12, "w": 13,
	"e": 14, "r": 15, "y": 16, "t": 17, "1": 18, "2": 19, "3": 20, "4": 21, "6": 22, "5": 23, "9": 25, "7": 26,
	"8": 28, "0": 29, "o": 31, "u": 32, "i": 34, "p": 35, "l": 37, "j": 38, "k": 40, "n": 45, "m": 46,
}

// editActions are the Edit menu's items by their key, which Command
// presses.
var editActions = map[string]string{"a": "selectAll:", "c": "copy:", "v": "paste:", "x": "cut:", "z": "undo:"}

// keyChars are what the special keys type, as AppKit reports them.
var keyChars = map[string]string{
	"enter": "\r", "return": "\r", "tab": "\t", "space": " ", "backspace": "\x7f", "escape": "\x1b",
	"delete": "", "left": "", "right": "", "down": "", "up": "",
	"home": "", "end": "", "pageup": "", "pagedown": "",
}

// key posts a key down and up with chars, ignoring is what it types
// without modifiers.
func (t *testSurface) key(code uint16, flags uint, chars, ignoring string) {
	for _, kind := range []uint{evKeyDown, evKeyUp} {
		onMainSync(func() {
			// keyCode and isARepeat follow the eight register arguments, so
			// they go on the stack packed as Apple's arm64 ABI packs them,
			// which purego, giving each a slot, would not: one word carries
			// both.
			stacked := uint64(code) << 16
			ev := objc.ID(objc.GetClass("NSEvent")).Send(
				sel("keyEventWithType:location:modifierFlags:timestamp:windowNumber:context:characters:charactersIgnoringModifiers:isARepeat:keyCode:"),
				kind, apple.CGPoint{}, flags, uptime(), objc.Send[int](t.s.window, sel("windowNumber")), objc.ID(0),
				apple.NSString(chars), apple.NSString(ignoring), stacked)
			nsApp().Send(sel("postEvent:atStart:"), ev, false)
		})
	}
	time.Sleep(10 * time.Millisecond)
}

func (t *testSurface) Type(text string) error {
	for _, r := range text {
		name := strings.ToLower(string(r))
		code, flags := keyCodes[name], uint(0)
		if r == '\n' {
			code, name = 36, "\r"
		}
		if r >= 'A' && r <= 'Z' {
			flags = flagShift
		}
		t.key(code, flags, string(r), name)
	}
	return nil
}

// Compose marks text in the first responder as an input method does,
// through NSTextInputClient: the field's editor shows it marked and
// reports it, as it does a keyboard's composition.
func (t *testSurface) Compose(text string) error {
	var err error
	onMainSync(func() {
		r := t.s.window.Send(sel("firstResponder"))
		if r == 0 || !objc.Send[bool](r, sel("respondsToSelector:"), sel("setMarkedText:selectedRange:replacementRange:")) {
			err = errors.New("no text input client has the keyboard")
			return
		}
		n := uint(len(utf16.Encode([]rune(text))))
		r.Send(sel("setMarkedText:selectedRange:replacementRange:"), apple.NSString(text), apple.NSRange{Location: n}, apple.NSRange{Location: nsNotFound})
		if text == "" {
			r.Send(sel("unmarkText"))
		}
	})
	time.Sleep(10 * time.Millisecond)
	return err
}

// Commit inserts text over what is marked, which ends the composition, as
// an input method commits.
func (t *testSurface) Commit(text string) error {
	var err error
	onMainSync(func() {
		r := t.s.window.Send(sel("firstResponder"))
		if r == 0 || !objc.Send[bool](r, sel("respondsToSelector:"), sel("insertText:replacementRange:")) {
			err = errors.New("no text input client has the keyboard")
			return
		}
		r.Send(sel("insertText:replacementRange:"), apple.NSString(text), apple.NSRange{Location: nsNotFound})
	})
	time.Sleep(10 * time.Millisecond)
	return err
}

// nsNotFound is NSNotFound, no range to replace: the marked text's.
const nsNotFound = uint(1<<63 - 1)

func (t *testSurface) Key(key string) error {
	parts := strings.Split(strings.ToLower(key), "+")
	name := parts[len(parts)-1]
	var flags uint
	for _, m := range parts[:len(parts)-1] {
		switch m {
		case "mod", "cmd", "meta", "command":
			flags |= flagCommand
		case "ctrl", "control":
			flags |= flagControl
		case "alt", "option":
			flags |= flagOption
		case "shift":
			flags |= flagShift
		default:
			return fmt.Errorf("unknown modifier %q in %q", m, key)
		}
	}
	code, ok := keyCodes[name]
	if !ok {
		return fmt.Errorf("unknown key %q", key)
	}
	if action := editActions[name]; flags == flagCommand && action != "" {
		// The menu's key equivalents need the app active, which a test
		// leaves to the app the user works in: the field does the edit the
		// menu item would ask of it.
		onMainSync(func() {
			if r := t.s.window.Send(sel("firstResponder")); r != 0 {
				r.Send(sel("tryToPerform:with:"), sel(action), objc.ID(0))
			}
		})
		return nil
	}
	chars := keyChars[name]
	if chars == "" {
		chars = name
	}
	t.key(code, flags, chars, chars)
	return nil
}

// Back is Escape, twice in a field, whose first only leaves it.
func (t *testSurface) Back() error {
	var editing bool
	onMainSync(func() {
		r := t.s.window.Send(sel("firstResponder"))
		editing = r != 0 && objc.Send[bool](r, sel("isKindOfClass:"), objc.ID(objc.GetClass("NSText")))
	})
	if editing {
		t.Key("escape")
	}
	return t.Key("escape")
}

func (t *testSurface) Answer(id uint32, index int) error {
	var err error
	onMainSync(func() {
		shown, ok := t.s.alerts[id]
		if !ok {
			err = fmt.Errorf("no alert %d shows", id)
			return
		}
		window := shown.alert.Send(sel("window"))
		if index < 0 {
			t.s.window.Send(sel("endSheet:returnCode:"), window, 0)
			return
		}
		for i, action := range shown.order {
			if action == index {
				buttons := shown.alert.Send(sel("buttons"))
				buttons.Send(sel("objectAtIndex:"), uint(i)).Send(sel("performClick:"), objc.ID(0))
				return
			}
		}
		err = fmt.Errorf("the alert has no button %d", index)
	})
	return err
}

func (t *testSurface) Screenshot() ([]byte, error) {
	var png []byte
	onMainSync(func() {
		v := t.s.native
		bounds := objc.Send[apple.CGRect](v, selBounds)
		rep := v.Send(sel("bitmapImageRepForCachingDisplayInRect:"), bounds)
		if rep == 0 {
			return
		}
		v.Send(sel("cacheDisplayInRect:toBitmapImageRep:"), bounds, rep)
		const pngType = 4
		data := rep.Send(sel("representationUsingType:properties:"), uint(pngType),
			objc.ID(objc.GetClass("NSDictionary")).Send(sel("dictionary")))
		if data == 0 {
			return
		}
		png = apple.Bytes(data)
	})
	if png == nil {
		return nil, errors.New("the window drew nothing")
	}
	return png, nil
}

func (t *testSurface) Sync() { t.s.engine.Sync() }

func (t *testSurface) Animating() bool {
	var busy bool
	onMainSync(func() {
		// Batches waiting for a frame have not shown yet, nor scrolls
		// their offsets.
		busy = t.s.animator.Ending() || len(t.s.motions) > 0 || len(t.s.ghosts) > 0 || t.s.held() || t.s.scrollsMoving()
	})
	return busy
}

func (t *testSurface) Resize(size driver.Size) error {
	onMainSync(func() {
		t.s.window.Send(sel("setContentSize:"), apple.CGSize{Width: float64(size.Width), Height: float64(size.Height)})
	})
	return nil
}

func (t *testSurface) SetEnvironment(env driver.Environment) error {
	t.test.pin.Set(&env)
	t.s.engine.Repin()
	return nil
}

// menuItem returns the item tagged id in menu or its submenus, and the
// menu it is in.
func menuItem(menu objc.ID, id uint32) (item, in objc.ID) {
	if menu == 0 {
		return 0, 0
	}
	n := objc.Send[int](menu, sel("numberOfItems"))
	for i := range n {
		it := menu.Send(sel("itemAtIndex:"), i)
		if objc.Send[int](it, sel("tag")) == int(id) && it.Send(sel("action")) != 0 {
			return it, menu
		}
		if sub := it.Send(sel("submenu")); sub != 0 {
			if found, at := menuItem(sub, id); found != 0 {
				return found, at
			}
		}
	}
	return 0, 0
}

// ChooseMenu implements testhost.Surface: the main menu's item sends its
// action, as a click on it does.
func (t *testSurface) ChooseMenu(id uint32) error {
	var err error
	onMainSync(func() {
		main := objc.ID(objc.GetClass("NSApplication")).Send(sel("sharedApplication")).Send(sel("mainMenu"))
		item, menu := menuItem(main, id)
		if item == 0 {
			err = fmt.Errorf("the main menu has no item %d", id)
			return
		}
		menu.Send(sel("performActionForItemAtIndex:"), objc.Send[int](menu, sel("indexOfItem:"), item))
	})
	return err
}

// ChoosePopup implements testhost.Surface: the tracking context menu's
// item sends its action and the menu closes, as a click on it does.
func (t *testSurface) ChoosePopup(id uint32) error {
	var err error
	deadline := time.Now().Add(3 * time.Second)
	for {
		shown := false
		onMainSync(func() {
			s := t.s
			if shown = s.popup != 0 && s.tracking; !shown {
				return
			}
			if id != 0 {
				if item, menu := menuItem(s.popup, id); item != 0 {
					menu.Send(sel("performActionForItemAtIndex:"), objc.Send[int](menu, sel("indexOfItem:"), item))
				} else {
					err = fmt.Errorf("the context menu has no item %d", id)
				}
			}
			s.closePopup()
		})
		if shown || time.Now().After(deadline) {
			if !shown {
				return fmt.Errorf("no context menu shows")
			}
			return err
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// Several implements testhost.Host: a desktop shows several windows.
func (host) Several() bool { return true }

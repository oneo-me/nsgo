//go:build ios && nsgotest

package ios

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
	"github.com/oneo-me/nsgo/internal/testhost"
)

// The test host: nsgo test -target ios_simulator runs each test in the
// app, launched in a simulator. Touches are IOHID digitizer events handed
// to UIKit (-[UIApplication _enqueueHIDEvent:]), so UIKit makes the
// UITouches and runs hit testing, gesture recognizers and scroll views as
// for a finger; typing goes through the software keyboard's input
// manager, as UI test tools type. IOKit's event functions and
// BackBoardServices' digitizer info are private, looked up at run time,
// and only in test builds.

func init() { testhost.Register(host{}) }

type host struct{}

func (host) Main(run func() int) {
	sceneReady = make(chan struct{})
	runner = newSurface(nil, Options{ID: "dev.nsgo.test"})
	go func() { os.Exit(run()) }()
	runUIKit(Options{})
}

func (host) Open(root func(cx *core.Scope) core.Node, o testhost.Options) (testhost.Surface, error) {
	select {
	case <-sceneReady:
	case <-time.After(30 * time.Second):
		return nil, errors.New("the scene did not connect")
	}
	test := &testOpen{options: o}
	onMainSync(func() {
		runner.app = root
		runner.startEngine(test)
	})
	if err := loadHID(); err != nil {
		return nil, err
	}
	return &testSurface{s: runner, test: test}, nil
}

func onMainSync(fn func()) {
	done := make(chan struct{})
	apple.OnMain(func() {
		defer close(done)
		fn()
	})
	<-done
}

type testSurface struct {
	s    *surface
	test *testOpen
}

// IOKit's digitizer and keyboard events and BackBoardServices' digitizer
// info, from IOHIDEvent.h and BKSHIDEvent.h.
var (
	hidDigitizer func(alloc uintptr, time uint64, transducer, index, identity, mask, buttons uint32,
		x, y, z, tip, barrel float64, inRange, touching uint8, options uint32) uintptr
	hidFinger func(alloc uintptr, time uint64, index, identity, mask uint32,
		x, y, z, tip, twist float64, inRange, touching uint8, options uint32) uintptr
	hidKeyboard      func(alloc uintptr, time uint64, page, usage uint32, down uint8, options uint32) uintptr
	hidAppend        func(parent, child uintptr, options uint32)
	hidSetInteger    func(event uintptr, field uint32, value int64)
	hidSetFloat      func(event uintptr, field uint32, value float64)
	hidSetSender     func(event uintptr, sender uint64)
	bksDigitizerInfo func(event uintptr, context uint32, gesturePossible, gestureChange uint8, display uintptr, initial float64, force float32)
	machAbsoluteTime func() uint64
)

func loadHID() error {
	if hidDigitizer != nil {
		return nil
	}
	open := func(path string) (uintptr, error) {
		return purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	}
	iokit, err := open("/System/Library/Frameworks/IOKit.framework/IOKit")
	if err != nil {
		return err
	}
	bbs, err := open("/System/Library/PrivateFrameworks/BackBoardServices.framework/BackBoardServices")
	if err != nil {
		return err
	}
	system, err := open("/usr/lib/libSystem.B.dylib")
	if err != nil {
		return err
	}
	purego.RegisterLibFunc(&hidDigitizer, iokit, "IOHIDEventCreateDigitizerEvent")
	purego.RegisterLibFunc(&hidFinger, iokit, "IOHIDEventCreateDigitizerFingerEvent")
	purego.RegisterLibFunc(&hidKeyboard, iokit, "IOHIDEventCreateKeyboardEvent")
	purego.RegisterLibFunc(&hidAppend, iokit, "IOHIDEventAppendEvent")
	purego.RegisterLibFunc(&hidSetInteger, iokit, "IOHIDEventSetIntegerValue")
	purego.RegisterLibFunc(&hidSetFloat, iokit, "IOHIDEventSetFloatValue")
	purego.RegisterLibFunc(&hidSetSender, iokit, "IOHIDEventSetSenderID")
	purego.RegisterLibFunc(&bksDigitizerInfo, bbs, "BKSHIDEventSetDigitizerInfo")
	purego.RegisterLibFunc(&machAbsoluteTime, system, "mach_absolute_time")
	return nil
}

const (
	transducerHand    = 3
	maskRange         = 1 << 0
	maskTouch         = 1 << 1
	maskPosition      = 1 << 2
	fieldIntegrated   = 11<<16 | 25 // kIOHIDEventFieldDigitizerIsDisplayIntegrated
	fieldMajorRadius  = 11<<16 | 20
	fieldMinorRadius  = 11<<16 | 21
	testSender        = 0x4e53_474f // "NSGO"
	keyboardUsagePage = 7
)

// enqueue hands event to UIKit as the window's, then releases it.
func enqueue(event uintptr) {
	context := objc.Send[uint32](window, sel("_contextId"))
	hidSetSender(event, testSender)
	bksDigitizerInfo(event, context, 0, 0, 0, 0, 0)
	objc.ID(objc.GetClass("UIApplication")).Send(sel("sharedApplication")).Send(sel("_enqueueHIDEvent:"), event)
	apple.Release(event)
}

type phase uint8

const (
	down phase = iota
	move
	up
)

// touch is one finger at p, the window's points, in ph.
func touch(p driver.Point, ph phase) {
	onMainSync(func() {
		touching := uint8(0)
		if ph != up {
			touching = 1
		}
		mask := uint32(maskRange | maskTouch)
		if ph == move {
			mask = maskPosition
		}
		now := machAbsoluteTime()
		hand := hidDigitizer(0, now, transducerHand, 0, 0, mask, 0, 0, 0, 0, 0, 0, touching, touching, 0)
		if hand == 0 {
			return
		}
		hidSetInteger(hand, fieldIntegrated, 1)
		finger := hidFinger(0, now, 1, 2, mask, float64(p.X), float64(p.Y), 0, float64(touching), 0, touching, touching, 0)
		if finger != 0 {
			hidSetFloat(finger, fieldMajorRadius, 5)
			hidSetFloat(finger, fieldMinorRadius, 5)
			hidSetInteger(finger, fieldIntegrated, 1)
			hidAppend(hand, finger, 0)
			apple.Release(finger)
		}
		enqueue(hand)
	})
}

// focusAt gives the text field at p the focus: made touches leave the
// first responder where it was, as UI test tools know.
func focusAt(p driver.Point) {
	onMainSync(func() {
		hit := window.Send(sel("hitTest:withEvent:"), apple.CGPoint{X: float64(p.X), Y: float64(p.Y)}, objc.ID(0))
		field, text := objc.ID(objc.GetClass("UITextField")), objc.ID(objc.GetClass("UITextView"))
		for v := hit; v != 0; v = v.Send(sel("superview")) {
			if objc.Send[bool](v, sel("isKindOfClass:"), field) || objc.Send[bool](v, sel("isKindOfClass:"), text) {
				if !objc.Send[bool](v, sel("isFirstResponder")) && objc.Send[bool](v, sel("canBecomeFirstResponder")) {
					v.Send(sel("becomeFirstResponder"))
				}
				return
			}
		}
	})
}

func (t *testSurface) Press(at driver.Point, hold time.Duration) error {
	touch(at, down)
	time.Sleep(max(hold+50*time.Millisecond, 60*time.Millisecond))
	touch(at, up)
	focusAt(at)
	return nil
}

func (t *testSurface) RightClick(driver.Point) error {
	return errors.New("a phone has no secondary click: use LongPress")
}

// fingerAlong moves a finger along path, 10 points a frame, then rests
// still reporting where it is before it lifts, so it ends without a
// fling.
func fingerAlong(path []driver.Point) {
	touch(path[0], down)
	time.Sleep(100 * time.Millisecond)
	at := path[0]
	for _, to := range path[1:] {
		n := int(math.Max(math.Abs(float64(to.X-at.X)), math.Abs(float64(to.Y-at.Y)))/10) + 1
		for i := 1; i <= n; i++ {
			f := driver.Points(i) / driver.Points(n)
			touch(driver.Point{X: at.X + (to.X-at.X)*f, Y: at.Y + (to.Y-at.Y)*f}, move)
			time.Sleep(16 * time.Millisecond)
		}
		at = to
	}
	for range 6 {
		touch(at, move)
		time.Sleep(16 * time.Millisecond)
	}
	touch(at, up)
}

func (t *testSurface) Drag(path []driver.Point) error {
	if len(path) == 0 {
		return errors.New("an empty drag")
	}
	fingerAlong(path)
	return nil
}

// Scroll drags the content the other way from delta: a finger moving up
// shows what is below.
func (t *testSurface) Scroll(at driver.Point, delta driver.Point) error {
	fingerAlong([]driver.Point{at, {X: at.X - delta.X, Y: at.Y - delta.Y}})
	return nil
}

func (t *testSurface) Hover(driver.Point) error { return nil }

// keyboardImpl is the software keyboard's input manager (private), which
// types into the first responder as its keys do, asking the field's
// delegate first.
func keyboardImpl() objc.ID {
	class := objc.GetClass("UIKeyboardImpl")
	if class == 0 {
		return 0
	}
	return objc.ID(class).Send(sel("activeInstance"))
}

func (t *testSurface) Type(text string) error {
	for _, r := range text {
		onMainSync(func() {
			if kb := keyboardImpl(); kb != 0 {
				kb.Send(sel("addInputString:"), apple.NSString(string(r)))
				return
			}
			if f := window.Send(sel("firstResponder")); f != 0 {
				f.Send(sel("insertText:"), apple.NSString(string(r)))
			}
		})
		time.Sleep(30 * time.Millisecond)
	}
	return nil
}

// Compose marks text in the first responder as an input method does,
// through UITextInput.
func (t *testSurface) Compose(text string) error {
	var err error
	onMainSync(func() {
		f := window.Send(sel("firstResponder"))
		if f == 0 || !objc.Send[bool](f, sel("respondsToSelector:"), sel("setMarkedText:selectedRange:")) {
			err = errors.New("no text input has the keyboard")
			return
		}
		n := uint(len(utf16.Encode([]rune(text))))
		f.Send(sel("setMarkedText:selectedRange:"), apple.NSString(text), apple.NSRange{Location: n})
		if text == "" {
			f.Send(sel("unmarkText"))
		}
	})
	time.Sleep(30 * time.Millisecond)
	return err
}

// Commit marks text and unmarks it, which keeps it, as an input method
// commits.
func (t *testSurface) Commit(text string) error {
	var err error
	onMainSync(func() {
		f := window.Send(sel("firstResponder"))
		if f == 0 || !objc.Send[bool](f, sel("respondsToSelector:"), sel("unmarkText")) {
			err = errors.New("no text input has the keyboard")
			return
		}
		n := uint(len(utf16.Encode([]rune(text))))
		f.Send(sel("setMarkedText:selectedRange:"), apple.NSString(text), apple.NSRange{Location: n})
		f.Send(sel("unmarkText"))
	})
	time.Sleep(30 * time.Millisecond)
	return err
}

// usages are the USB HID keyboard usages of the keys tests name.
var usages = map[string]uint32{
	"enter": 0x28, "return": 0x28, "escape": 0x29, "backspace": 0x2a, "tab": 0x2b, "space": 0x2c,
	"right": 0x4f, "left": 0x50, "down": 0x51, "up": 0x52, "delete": 0x4c,
	"shift": 0xe1, "ctrl": 0xe0, "control": 0xe0, "alt": 0xe2, "option": 0xe2, "mod": 0xe3, "cmd": 0xe3, "command": 0xe3, "meta": 0xe3,
}

func keyUsage(name string) (uint32, bool) {
	if u, ok := usages[name]; ok {
		return u, true
	}
	if len(name) == 1 {
		switch c := name[0]; {
		case c >= 'a' && c <= 'z':
			return 0x04 + uint32(c-'a'), true
		case c >= '1' && c <= '9':
			return 0x1e + uint32(c-'1'), true
		case c == '0':
			return 0x27, true
		}
	}
	return 0, false
}

func hidKey(usage uint32, isDown bool) {
	onMainSync(func() {
		d := uint8(0)
		if isDown {
			d = 1
		}
		if ev := hidKeyboard(0, machAbsoluteTime(), keyboardUsagePage, usage, d, 0); ev != 0 {
			enqueue(ev)
		}
	})
	time.Sleep(10 * time.Millisecond)
}

// Key presses a hardware keyboard's keys, as UIKit takes a real one's:
// the modifiers first. The edit keys a field takes from the software
// keyboard go through its input manager.
func (t *testSurface) Key(key string) error {
	name := strings.ToLower(key)
	switch name {
	case "mod+a":
		onMainSync(func() {
			if f := window.Send(sel("firstResponder")); f != 0 {
				f.Send(sel("selectAll:"), objc.ID(0))
			}
		})
		return nil
	case "backspace":
		var done bool
		onMainSync(func() {
			if kb := keyboardImpl(); kb != 0 && window.Send(sel("firstResponder")) != 0 {
				kb.Send(sel("deleteFromInput"))
				done = true
			}
		})
		if done {
			return nil
		}
	case "enter", "return":
		var done bool
		onMainSync(func() {
			if kb := keyboardImpl(); kb != 0 && window.Send(sel("firstResponder")) != 0 {
				kb.Send(sel("addInputString:"), apple.NSString("\n"))
				done = true
			}
		})
		if done {
			return nil
		}
	}
	parts := strings.Split(name, "+")
	var codes []uint32
	for _, p := range parts {
		u, ok := keyUsage(p)
		if !ok {
			return fmt.Errorf("unknown key %q in %q", p, key)
		}
		codes = append(codes, u)
	}
	for _, u := range codes {
		hidKey(u, true)
	}
	for i := len(codes) - 1; i >= 0; i-- {
		hidKey(codes[i], false)
	}
	return nil
}

// Back is the escape the screen reader's gesture makes: the core goes
// back from the topmost overlay, modal or stack.
func (t *testSurface) Back() error {
	t.s.engine.Dispatch(driver.Event{Kind: driver.EventBack})
	return nil
}

// Answer taps the alert's button with the action's title.
func (t *testSurface) Answer(id uint32, index int) error {
	var label string
	var err error
	var at driver.Point
	onMainSync(func() {
		shown, ok := t.s.alerts[id]
		if !ok {
			err = fmt.Errorf("no alert %d shows", id)
			return
		}
		switch {
		case index >= 0 && index < len(shown.actions):
			label = shown.actions[index].Label
		case len(shown.actions) == 0:
			label = "OK"
		default:
			for _, a := range shown.actions {
				if a.Role == driver.AlertCancel {
					label = a.Label
				}
			}
			if label == "" {
				err = errors.New("the alert cannot be dismissed")
				return
			}
		}
		v := findLabel(shown.ctrl.Send(sel("view")), label)
		if v == 0 {
			err = fmt.Errorf("the alert shows no %q", label)
			return
		}
		b := objc.Send[apple.CGRect](v, sel("bounds"))
		p := objc.Send[apple.CGPoint](v, sel("convertPoint:toView:"), apple.CGPoint{X: b.Size.Width / 2, Y: b.Size.Height / 2}, objc.ID(0))
		at = driver.Point{X: driver.Points(p.X), Y: driver.Points(p.Y)}
	})
	if err != nil {
		return err
	}
	return t.Press(at, 0)
}

// findLabel finds the label reading text in v.
func findLabel(v objc.ID, text string) objc.ID {
	if objc.Send[bool](v, sel("isKindOfClass:"), objc.ID(objc.GetClass("UILabel"))) &&
		apple.GoString(v.Send(sel("text"))) == text {
		return v
	}
	subs := v.Send(sel("subviews"))
	for i := range objc.Send[uint](subs, sel("count")) {
		if f := findLabel(subs.Send(sel("objectAtIndex:"), i), text); f != 0 {
			return f
		}
	}
	return 0
}

func (t *testSurface) Screenshot() ([]byte, error) {
	var png []byte
	onMainSync(func() {
		bounds := objc.Send[apple.CGRect](window, selBounds)
		renderer := objc.ID(objc.GetClass("UIGraphicsImageRenderer")).Send(selAlloc).Send(sel("initWithBounds:"), bounds)
		draw := objc.NewBlock(func(_ objc.Block, _ objc.ID) {
			objc.Send[bool](window, sel("drawViewHierarchyInRect:afterScreenUpdates:"), bounds, true)
		})
		data := renderer.Send(sel("PNGDataWithActions:"), draw)
		draw.Release()
		if data != 0 {
			png = apple.Bytes(data)
		}
		renderer.Send(selRelease)
	})
	if png == nil {
		return nil, errors.New("the window drew nothing")
	}
	return png, nil
}

// Animating counts finite Core Animation animations (a screen pushed, a
// sheet presented, an alert fading in) and scroll views gliding.
func (t *testSurface) Sync() { t.s.engine.Sync() }

func (t *testSurface) Animating() bool {
	var busy bool
	onMainSync(func() {
		busy = t.s.animator.Ending() || layersAnimating(window.Send(sel("layer"))) || gliding(window)
	})
	return busy
}

func layersAnimating(layer objc.ID) bool {
	if keys := layer.Send(sel("animationKeys")); keys != 0 {
		for i := range objc.Send[uint](keys, sel("count")) {
			a := layer.Send(sel("animationForKey:"), keys.Send(sel("objectAtIndex:"), i))
			if a == 0 {
				continue
			}
			// A caret's blink repeats "forever" as a huge count.
			repeat, duration := objc.Send[float32](a, sel("repeatCount")), objc.Send[float64](a, sel("duration"))
			if repeat < 1e6 && duration < 1e6 && !math.IsInf(duration, 0) {
				return true
			}
		}
	}
	subs := layer.Send(sel("sublayers"))
	if subs == 0 {
		return false
	}
	for i := range objc.Send[uint](subs, sel("count")) {
		if layersAnimating(subs.Send(sel("objectAtIndex:"), i)) {
			return true
		}
	}
	return false
}

func gliding(v objc.ID) bool {
	if objc.Send[bool](v, sel("isKindOfClass:"), objc.ID(objc.GetClass("UIScrollView"))) && objc.Send[bool](v, sel("isDecelerating")) {
		return true
	}
	subs := v.Send(sel("subviews"))
	for i := range objc.Send[uint](subs, sel("count")) {
		if gliding(subs.Send(sel("objectAtIndex:"), i)) {
			return true
		}
	}
	return false
}

// Resize does nothing: a phone's screen is the size it is.
func (t *testSurface) Resize(driver.Size) error { return nil }

func (t *testSurface) SetEnvironment(env driver.Environment) error {
	t.test.pin.Set(&env)
	t.s.engine.Repin()
	return nil
}

// ChooseMenu implements testhost.Surface: no native menu bar shows here.
func (t *testSurface) ChooseMenu(uint32) error { return testhost.ErrNoMenu }

// ChoosePopup implements testhost.Surface: the core shows no native
// context menus here.
func (t *testSurface) ChoosePopup(uint32) error { return testhost.ErrNoMenu }

// Several implements testhost.Host: it shows one app at a time.
func (host) Several() bool { return false }

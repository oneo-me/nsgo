//go:build linux && !android && nsgotest

package linux

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/testhost"
)

// The test host: nsgo test -target linux runs each test in a GTK window.
// GTK 4 takes no made-up events, so input goes in at the backend's own
// entry points, where GTK's controllers hand it over, and text through
// the focused field's own signals.

func init() { testhost.Register(&host{opens: make(chan openRequest)}) }

type openRequest struct {
	app   func(cx *core.Scope) core.Node
	opts  Options
	reply chan openReply
}

type openReply struct {
	w   *window
	err error
}

type host struct{ opens chan openRequest }

// Main waits on this thread for the test's app, then runs GTK there.
func (h *host) Main(run func() int) {
	go func() { os.Exit(run()) }()
	req := <-h.opens
	w, err := start(req.app, req.opts)
	req.reply <- openReply{w, err}
	if err != nil {
		select {}
	}
	// A test's later apps open their windows on the first one's loop.
	go func() {
		for req := range h.opens {
			w.post(func() {
				o, err := start(req.app, req.opts)
				if err == nil {
					gtkWindowPresent(o.win)
				}
				req.reply <- openReply{o, err}
			})
		}
	}()
	w.run()
	select {}
}

func (h *host) Open(app func(cx *core.Scope) core.Node, o testhost.Options) (testhost.Surface, error) {
	test := &testOpen{options: o}
	reply := make(chan openReply, 1)
	h.opens <- openRequest{app: app, opts: Options{Title: "NSGo test", Width: o.Size.Width, Height: o.Size.Height, test: test}, reply: reply}
	r := <-reply
	if r.err != nil {
		return nil, r.err
	}
	return &testSurface{w: r.w, test: test}, nil
}

type testSurface struct {
	w    *window
	test *testOpen
	// field is the field a press last focused: GTK reports the focus
	// only while the window is active, which a test cannot ask.
	field *view
}

// focused is the field with the keyboard.
func (t *testSurface) focused() *view {
	if t.w.focus != nil {
		return t.w.focus
	}
	if t.field != nil && !t.field.deleted {
		return t.field
	}
	return nil
}

// onLoop runs fn on the GTK thread and waits for it.
func (t *testSurface) onLoop(fn func()) {
	done := make(chan struct{})
	t.w.post(func() {
		defer close(done)
		fn()
	})
	select {
	case <-done:
	case <-time.After(10 * time.Second):
	}
}

// focusAt gives the field at at the keyboard, as a click in it does.
func (t *testSurface) focusAt(at driver.Point) {
	root := t.w.view(t.w.root)
	if root == nil {
		return
	}
	v := hit(root, at, func(v *view) bool { return v.kind == driver.KindInput })
	if v != nil && t.w.focus != v {
		t.w.focusEntry(v)
	}
	t.field = v
}

func (t *testSurface) Press(at driver.Point, hold time.Duration) error {
	t.onLoop(func() {
		t.focusAt(at)
		t.w.pointerDown(at, 1, 0, 0, 1)
	})
	if hold > 0 {
		time.Sleep(hold + 50*time.Millisecond)
	}
	t.onLoop(func() { t.w.pointerUp(at) })
	return nil
}

func (t *testSurface) RightClick(at driver.Point) error {
	t.onLoop(func() {
		t.w.pointerDown(at, 3, 0, 0, 1)
		t.w.pointerUp(at)
	})
	return nil
}

func (t *testSurface) Drag(path []driver.Point) error {
	if len(path) == 0 {
		return errors.New("an empty drag")
	}
	t.onLoop(func() { t.w.pointerDown(path[0], 1, 0, 0, 1) })
	time.Sleep(100 * time.Millisecond)
	at := path[0]
	for _, to := range path[1:] {
		dx, dy := to.X-at.X, to.Y-at.Y
		n := int(max(absPoints(dx), absPoints(dy))/10) + 1
		for i := 1; i <= n; i++ {
			f := driver.Points(i) / driver.Points(n)
			p := driver.Point{X: at.X + dx*f, Y: at.Y + dy*f}
			t.onLoop(func() { t.w.movePointer(p, gdkButton1) })
			time.Sleep(16 * time.Millisecond)
		}
		at = to
	}
	t.onLoop(func() { t.w.pointerUp(at) })
	return nil
}

func absPoints(v driver.Points) driver.Points {
	if v < 0 {
		return -v
	}
	return v
}

// Scroll turns the wheel 20 points at a time, 16 ms apart.
func (t *testSurface) Scroll(at driver.Point, delta driver.Point) error {
	n := int(max(absPoints(delta.X), absPoints(delta.Y))/20) + 1
	for range n {
		t.onLoop(func() { t.w.wheel(at, float64(delta.X)/float64(n), float64(delta.Y)/float64(n)) })
		time.Sleep(16 * time.Millisecond)
	}
	return nil
}

func (t *testSurface) Hover(at driver.Point) error {
	t.onLoop(func() {
		if at.X < 0 || at.Y < 0 {
			leaveCallback(0, t.w.handle)
			return
		}
		motionCallback(0, float64(at.X), float64(at.Y), t.w.handle)
	})
	return nil
}

var (
	textInsert  func(text uintptr, s *byte, length int32, position *int32)
	textGetPosn func(text uintptr) int32
	textSetPosn func(text uintptr, position int32)
)

func loadEditing() {
	if textInsert == nil {
		purego.RegisterLibFunc(&textInsert, gtkLib, "gtk_editable_insert_text")
		purego.RegisterLibFunc(&textGetPosn, gtkLib, "gtk_editable_get_position")
		purego.RegisterLibFunc(&textSetPosn, gtkLib, "gtk_editable_set_position")
	}
}

// Type types into the focused field as its keys would: at the caret,
// through GTK's own editing, so its signals report each change.
func (t *testSurface) Type(text string) error {
	loadEditing()
	for _, r := range text {
		if r == '\n' {
			if err := t.Key("enter"); err != nil {
				return err
			}
			continue
		}
		var err error
		t.onLoop(func() {
			v := t.focused()
			switch {
			case v == nil:
				err = errors.New("no field has the keyboard")
			case v.textView != 0:
				gtkTextBufferInsertAtCursor(v.buffer, cstr(string(r)), -1)
			default:
				at := textGetPosn(v.entry)
				s := string(r)
				textInsert(v.entry, cstr(s), int32(len(s)), &at)
				textSetPosn(v.entry, at)
			}
		})
		if err != nil {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

// imWidget is the focused field's widget its input method reports to:
// an entry's text, or a text view.
func (t *testSurface) imWidget() (*view, uintptr, error) {
	v := t.focused()
	switch {
	case v == nil:
		return nil, 0, errors.New("no field has the keyboard")
	case v.textView != 0:
		return v, v.textView, nil
	}
	return v, gtkEditableGetDelegate(v.entry), nil
}

// Compose has the field's input method context preedit text, as GTK's
// editors report an input method's: by their preedit-changed signal.
func (t *testSurface) Compose(text string) error {
	var err error
	t.onLoop(func() {
		var w uintptr
		if _, w, err = t.imWidget(); err == nil {
			gSignalEmitByName(w, cstr("preedit-changed"), cstr(text))
		}
	})
	time.Sleep(10 * time.Millisecond)
	return err
}

// Commit inserts text at the caret as the context's commit does, then
// ends the preedit.
func (t *testSurface) Commit(text string) error {
	loadEditing()
	var err error
	t.onLoop(func() {
		var v *view
		var w uintptr
		if v, w, err = t.imWidget(); err != nil {
			return
		}
		if v.textView != 0 {
			gtkTextBufferInsertAtCursor(v.buffer, cstr(text), -1)
		} else {
			at := textGetPosn(v.entry)
			textInsert(v.entry, cstr(text), int32(len(text)), &at)
			textSetPosn(v.entry, at)
		}
		gSignalEmitByName(w, cstr("preedit-changed"), cstr(""))
	})
	time.Sleep(10 * time.Millisecond)
	return err
}

// keyvals are GDK's key values of the keys tests name.
var keyvals = map[string]uint32{
	"enter": 0xff0d, "return": 0xff0d, "escape": 0xff1b, "backspace": 0xff08, "tab": 0xff09, "space": 0x20,
	"left": 0xff51, "up": 0xff52, "right": 0xff53, "down": 0xff54, "home": 0xff50, "end": 0xff57,
	"pageup": 0xff55, "pagedown": 0xff56, "delete": 0xffff,
}

// Key presses a key with modifiers: the focused field edits as its key
// bindings would, and other keys reach the window's key controller.
func (t *testSurface) Key(key string) error {
	parts := strings.Split(strings.ToLower(key), "+")
	name := parts[len(parts)-1]
	var state uint32
	for _, m := range parts[:len(parts)-1] {
		switch m {
		case "mod", "ctrl", "control":
			state |= 1 << 2
		case "shift":
			state |= 1 << 0
		case "alt", "option":
			state |= 1 << 3
		case "meta", "super", "cmd":
			state |= 1 << 26
		default:
			return fmt.Errorf("unknown modifier %q in %q", m, key)
		}
	}
	keyval, ok := keyvals[name]
	if !ok && len(name) == 1 {
		keyval, ok = uint32(name[0]), true
	}
	if !ok {
		return fmt.Errorf("unknown key %q", key)
	}
	var handled bool
	t.onLoop(func() {
		v := t.focused()
		if v == nil || v.entry == 0 && v.textView == 0 {
			return
		}
		edit := v.entry
		if v.textView != 0 {
			edit = v.textView
		} else if d := gtkEditableGetDelegate(v.entry); d != 0 {
			edit = d
		}
		switch {
		case state == 1<<2 && name == "a":
			if v.textView != 0 {
				gSignalEmitByName(edit, cstr("select-all"), int32(1))
			} else {
				gtkEditableSelectRegion(v.entry, 0, -1)
			}
		case state == 0 && name == "backspace":
			gSignalEmitByName(edit, cstr("backspace"))
		case state == 0 && (name == "enter" || name == "return") && v.textView == 0:
			gSignalEmitByName(v.entry, cstr("activate"))
		default:
			return
		}
		handled = true
	})
	if handled {
		return nil
	}
	t.onLoop(func() {
		keyPressedCallback(0, keyval, 0, state, t.w.handle)
		keyReleasedCallback(0, keyval, 0, state, t.w.handle)
	})
	return nil
}

// Back is Escape, twice in a field, whose first only leaves it.
func (t *testSurface) Back() error {
	var editing bool
	t.onLoop(func() { editing = t.focused() != nil })
	if editing {
		t.onLoop(func() { t.w.focusEntry(nil); t.field = nil })
	}
	return t.Key("escape")
}

var (
	gtkWindowGetToplevels func() uintptr
	gtkButtonGetLabel     func(b uintptr) uintptr
	gtkButtonType         func() uintptr
	gTypeCheckInstanceIs  func(instance, gtype uintptr) int32
)

func loadDialogs() {
	if gtkWindowGetToplevels != nil {
		return
	}
	purego.RegisterLibFunc(&gtkWindowGetToplevels, gtkLib, "gtk_window_get_toplevels")
	purego.RegisterLibFunc(&gtkButtonGetLabel, gtkLib, "gtk_button_get_label")
	purego.RegisterLibFunc(&gtkButtonType, gtkLib, "gtk_button_get_type")
	purego.RegisterLibFunc(&gTypeCheckInstanceIs, gobjectLib, "g_type_check_instance_is_a")
}

// findButton finds the button labelled label under widget.
func findButton(widget uintptr, label string) uintptr {
	if gTypeCheckInstanceIs(widget, gtkButtonType()) != 0 && goString(gtkButtonGetLabel(widget)) == label {
		return widget
	}
	for c := gtkWidgetGetFirstChild(widget); c != 0; c = gtkWidgetGetNextSibling(c) {
		if b := findButton(c, label); b != 0 {
			return b
		}
	}
	return 0
}

// Answer clicks the button of the alert dialog GTK shows, which it
// presents a moment after the app asks.
func (t *testSurface) Answer(id uint32, index int) error {
	loadDialogs()
	deadline := time.Now().Add(3 * time.Second)
	for {
		done, err := t.answer(id, index)
		if done || err != nil || time.Now().After(deadline) {
			if !done && err == nil {
				err = fmt.Errorf("the alert %d shows no button for %d", id, index)
			}
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (t *testSurface) answer(id uint32, index int) (done bool, err error) {
	t.onLoop(func() {
		if !t.w.alerting || t.w.alertingID != id {
			err = fmt.Errorf("no alert %d shows", id)
			return
		}
		a := t.w.alertShown
		label := ""
		if index >= 0 && index < len(a.Actions) {
			label = a.Actions[index].Label
		} else {
			for _, act := range a.Actions {
				if act.Role == driver.AlertCancel {
					label = act.Label
				}
			}
		}
		tops := gtkWindowGetToplevels()
		for i := range gListModelGetNItems(tops) {
			win := gListModelGetItem(tops, i)
			if win == t.w.win {
				gObjectUnref(win)
				continue
			}
			if label == "" {
				gtkWindowClose(win)
				gObjectUnref(win)
				done = true
				return
			}
			b := findButton(win, label)
			gObjectUnref(win)
			if b != 0 {
				gSignalEmitByName(b, cstr("clicked"))
				done = true
				return
			}
		}
	})
	return done, err
}

// Screenshot renders the surface, hosted views included, offscreen.
func (t *testSurface) Screenshot() ([]byte, error) {
	var img *image.RGBA
	t.onLoop(func() {
		w := t.w
		width, height := int(gtkWidgetGetWidth(w.surface)), int(gtkWidgetGetHeight(w.surface))
		if width <= 0 || height <= 0 {
			return
		}
		surface := cairoImageSurfaceCreate(0, int32(width), int32(height))
		cr := cairoCreate(surface)
		w.paint(cr)
		cairoSurfaceFlush(surface)
		stride := int(cairoImageSurfaceGetStride(surface))
		data := unsafe.Slice((*byte)(cPointer(cairoImageSurfaceGetData(surface))), stride*height)
		img = image.NewRGBA(image.Rect(0, 0, width, height))
		for y := range height {
			for x := range width {
				// Cairo's ARGB32 is premultiplied BGRA in memory, as
				// image.RGBA is premultiplied RGBA.
				s, d := data[y*stride+4*x:], img.Pix[y*img.Stride+4*x:]
				d[0], d[1], d[2], d[3] = s[2], s[1], s[0], s[3]
			}
		}
		cairoDestroy(cr)
		cairoSurfaceDestroy(surface)
	})
	if img == nil {
		return nil, errors.New("the window has no size")
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Animating counts transitions, layout animations and animated values
// that end; endless images do not count.
func (t *testSurface) Sync() { t.w.engine.Sync() }

func (t *testSurface) Animating() bool {
	var busy bool
	t.onLoop(func() {
		busy = t.w.animator.Ending() || len(t.w.layouts) > 0
		for _, v := range t.w.views {
			busy = busy || v != nil && v.transition != nil
		}
	})
	return busy
}

func (t *testSurface) Resize(size driver.Size) error {
	t.onLoop(func() { gtkWindowSetDefaultSize(t.w.win, int32(size.Width), int32(size.Height)) })
	return nil
}

func (t *testSurface) SetEnvironment(env driver.Environment) error {
	t.test.pin.Set(&env)
	t.w.engine.Repin()
	return nil
}

// ChooseMenu implements testhost.Surface: the menu bar's item's action
// is activated, as GTK's menu bar does.
func (t *testSurface) ChooseMenu(id uint32) error {
	var err error
	t.onLoop(func() {
		if t.w.menuBar == 0 {
			// The window shows none: its title bar is the app's.
			err = testhost.ErrNoMenu
			return
		}
		for action, r := range t.w.actionRefs {
			if r.id == id {
				gActionActivate(action, 0)
				return
			}
		}
		err = fmt.Errorf("the menu bar has no item %d", id)
	})
	return err
}

// ChoosePopup implements testhost.Surface: the context menu's item's
// action is activated and the popover pops down, as a click on the item
// does.
func (t *testSurface) ChoosePopup(id uint32) error {
	var err error
	t.onLoop(func() {
		w := t.w
		if w.popover == 0 {
			err = fmt.Errorf("no context menu shows")
			return
		}
		if id != 0 {
			for action, r := range w.popupRefs {
				if r.id == id {
					gActionActivate(action, 0)
				}
			}
		}
		gtkPopoverPopdown(w.popover)
	})
	return err
}

// Several implements testhost.Host: a desktop shows several windows.
func (*host) Several() bool { return true }

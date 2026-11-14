//go:build js && wasm && nsgotest

package harmonyos

import (
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"sync"
	"syscall/js"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/engine"
	"github.com/nsgo-dev/nsgo/internal/testhost"
)

// The test host: a test's app runs in the page, with the ArkTS render layer
// drawing what it mounts, and the host opens it, takes the root view's
// picture, and sends the test's input into the engine. A press goes to the
// view the engine's frames put under the point, as ArkUI's hit test names
// the view a page's press went to, and a field takes the focus the page
// reports.
//
// Screenshots come back through the bridge: the page snapshots the root
// FrameNode, packs a PNG and calls nsgoSnapshotReply.

func init() { testhost.Register(host{}) }

type host struct{}

func (host) Main(run func() int) {
	go func() { os.Exit(run()) }()
	select {}
}

func (host) Several() bool { return false }

func (host) Open(app func(cx *core.Scope) core.Node, o testhost.Options) (testhost.Surface, error) {
	if js.Global().Get("nsgoHarmony").Type() != js.TypeObject {
		return nil, errors.New("the page has no nsgoHarmony binding: run the tests with nsgo test -target harmonyos")
	}
	installSnapshots()
	s := newSurface(Options{})
	env := testhost.Pin(s.env, o.Environment)
	if o.Size.Width > 0 && o.Size.Height > 0 {
		s.size0 = o.Size
	}
	s.engine = engine.Start(s, app, engine.Options{
		Size: s.size0, Environment: env, Clock: o.Clock, Storage: o.Storage,
		Observe: o.Observe, Event: o.Event,
	})
	s.install()
	s.bridge.Call("ready")
	return &testSurface{s: s, epoch: time.Now()}, nil
}

// testSurface is the app's surface for a test: points are the surface's,
// as a finger's are.
type testSurface struct {
	s     *surface
	epoch time.Time
}

// touch reports one finger at at.
func (t *testSurface) touch(phase driver.GesturePhase, at driver.Point) {
	t.s.dispatch(driver.Event{Kind: driver.EventTouch, Phase: phase, Time: time.Since(t.epoch),
		Touches: []driver.Touch{{ID: 0, Position: at, PagePosition: at, Changed: true}}})
}

func (t *testSurface) Press(at driver.Point, hold time.Duration) error {
	// A press goes where the page's own does: to the view under the point,
	// which a page learns from ArkUI and a test from the frames.
	if id := t.s.m.pressTarget(at); id != 0 {
		t.s.m.press(id, driver.EventPress, at)
		// A field takes the keyboard focus as it does when a finger taps
		// it, which the page reports from ArkUI's own focus.
		if v := t.s.m.views[id]; v != nil && (v.kind == driver.KindInput || v.kind == driver.KindWeb) {
			t.s.m.focused = id
			t.s.dispatch(driver.Event{Kind: driver.EventFocus, ID: id})
		}
		return nil
	}
	t.touch(driver.GestureBegan, at)
	// A tap keeps the finger down for a moment, as one does.
	if hold <= 0 {
		hold = 30 * time.Millisecond
	}
	time.Sleep(hold)
	t.touch(driver.GestureEnded, at)
	return nil
}

func (t *testSurface) RightClick(driver.Point) error { return testhost.ErrNoMenu }

func (t *testSurface) Drag(path []driver.Point) error {
	if len(path) == 0 {
		return errors.New("a drag needs a path")
	}
	t.touch(driver.GestureBegan, path[0])
	for _, p := range path[1:] {
		time.Sleep(10 * time.Millisecond)
		t.touch(driver.GestureChanged, p)
	}
	time.Sleep(20 * time.Millisecond)
	t.touch(driver.GestureEnded, path[len(path)-1])
	return nil
}

// Scroll drags the content by delta and rests before lifting, as a finger
// does when it scrolls and lets go.
func (t *testSurface) Scroll(at driver.Point, delta driver.Point) error {
	// ArkUI scrolls its own view, so a test drives the page's Scroller
	// rather than dragging: the page then reports the offset to the app.
	if id := t.s.m.scrollTarget(at); id != 0 {
		t.s.bridge.Call("scrollBy", int(id), float64(delta.X), float64(delta.Y))
		return nil
	}
	from := at
	to := driver.Point{X: at.X - delta.X, Y: at.Y - delta.Y}
	steps := 6
	t.touch(driver.GestureBegan, from)
	for i := 1; i <= steps; i++ {
		f := float32(i) / float32(steps)
		p := driver.Point{X: from.X + (to.X-from.X)*driver.Points(f), Y: from.Y + (to.Y-from.Y)*driver.Points(f)}
		time.Sleep(10 * time.Millisecond)
		t.touch(driver.GestureChanged, p)
	}
	time.Sleep(60 * time.Millisecond)
	t.touch(driver.GestureEnded, to)
	return nil
}

func (t *testSurface) Hover(driver.Point) error { return nil }

// Type appends text to the focused field, as typing does.
func (t *testSurface) Type(text string) error {
	id := t.s.m.focused
	v := t.s.m.views[id]
	if v == nil || v.kind != driver.KindInput {
		return errors.New("no text field has the keyboard focus")
	}
	t.s.m.edited(id, v.value+text)
	return nil
}

func (t *testSurface) Compose(string) error { return testhost.ErrNoInputMethod }
func (t *testSurface) Commit(string) error  { return testhost.ErrNoInputMethod }

// Key presses a key the keymap names, as a keyboard does.
func (t *testSurface) Key(key string) error {
	mods := 0
	for {
		switch {
		case strings.HasPrefix(key, "mod+"):
			mods |= 1
			key = strings.TrimPrefix(key, "mod+")
		case strings.HasPrefix(key, "ctrl+"):
			mods |= 1
			key = strings.TrimPrefix(key, "ctrl+")
		case strings.HasPrefix(key, "alt+"):
			mods |= 2
			key = strings.TrimPrefix(key, "alt+")
		case strings.HasPrefix(key, "shift+"):
			mods |= 4
			key = strings.TrimPrefix(key, "shift+")
		case strings.HasPrefix(key, "meta+"):
			mods |= 8
			key = strings.TrimPrefix(key, "meta+")
		default:
			code, ok := keyCode[key]
			if !ok {
				code = driver.NodeID(key[0])
			}
			t.s.m.key(code, 0, mods, key)
			t.s.m.key(code, 1, 0, "")
			return nil
		}
	}
}

// keyCode names the Linux input codes a key goes by, as the emulator's
// uinput reports them.
var keyCode = func() map[string]driver.NodeID {
	out := make(map[string]driver.NodeID, len(linuxKeys))
	for code, name := range linuxKeys {
		out[name] = code
	}
	return out
}()

func (t *testSurface) Back() error {
	t.s.dispatch(driver.Event{Kind: driver.EventBack})
	return nil
}

func (t *testSurface) Answer(id uint32, index int) error {
	t.s.dispatch(driver.Event{Kind: driver.EventAlert, Count: id, Index: index})
	return nil
}

func (t *testSurface) ChooseMenu(uint32) error  { return testhost.ErrNoMenu }
func (t *testSurface) ChoosePopup(uint32) error { return testhost.ErrNoMenu }

// screenshot requests answers a test's picture: the page snapshots the
// root view and calls nsgoSnapshotReply with the PNG's base64.
var (
	snapshotMu   sync.Mutex
	snapshotNext int
	snapshotWait = map[int]chan []byte{}
)

func installSnapshots() {
	js.Global().Set("nsgoSnapshotReply", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) < 2 {
			return nil
		}
		id := args[0].Int()
		snapshotMu.Lock()
		ch := snapshotWait[id]
		delete(snapshotWait, id)
		snapshotMu.Unlock()
		if ch == nil {
			return nil
		}
		data, err := base64.StdEncoding.DecodeString(args[1].String())
		if err != nil {
			data = nil
		}
		ch <- data
		return nil
	}))
}

func (t *testSurface) Screenshot() ([]byte, error) {
	snapshotMu.Lock()
	snapshotNext++
	id := snapshotNext
	ch := make(chan []byte, 1)
	snapshotWait[id] = ch
	snapshotMu.Unlock()
	t.s.bridge.Call("snapshot", id)
	select {
	case data := <-ch:
		if len(data) == 0 {
			return nil, errors.New("the page took no picture")
		}
		return data, nil
	case <-time.After(5 * time.Second):
		snapshotMu.Lock()
		delete(snapshotWait, id)
		snapshotMu.Unlock()
		return nil, errors.New("the page did not answer the snapshot")
	}
}

func (t *testSurface) Animating() bool { return t.s.anim.Running() }

func (t *testSurface) Resize(size driver.Size) error {
	t.s.size0 = size
	t.s.engine.Resize(size)
	return nil
}

func (t *testSurface) SetEnvironment(env driver.Environment) error {
	t.s.env = env
	t.s.engine.SetEnvironment(env)
	return nil
}

func (t *testSurface) Sync() { t.s.engine.Sync() }

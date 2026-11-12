//go:build ios && nsgotest

package watchos

import (
	"errors"
	"fmt"
	"math"
	"os"
	"time"
	"unicode/utf8"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
	"github.com/nsgo-dev/nsgo/internal/testhost"
)

// The test host: nsgo test -target watchos_simulator runs each test in
// the app, launched in a watch simulator. watchOS gives an app no way to
// post touches, so the host's input enters where the host's touches do,
// at the bridge (hostTouch): Go's hit testing, presses and pans run as
// for a finger, but SwiftUI's gestures do not, so a scroll moves the
// scroll view as the app's own ScrollTo does. Typing edits the field a
// press focused, as the system's text input would.

func init() { testhost.Register(host{}) }

type host struct{}

func (host) Main(run func() int) {
	hostReady = make(chan struct{})
	ready := hostReady
	s = newSurface(nil, Options{ID: "dev.nsgo.test"})
	go func() {
		<-ready
		os.Exit(run())
	}()
	runHost()
}

func (host) Open(root func(cx *core.Scope) core.Node, o testhost.Options) (testhost.Surface, error) {
	test := &testOpen{options: o}
	onMainSync(func() {
		s.app = root
		s.started = true
		s.start(test)
	})
	return &testSurface{test: test}, nil
}

// Several implements testhost.Host: a watch shows one app.
func (host) Several() bool { return false }

func onMainSync(fn func()) {
	done := make(chan struct{})
	apple.OnMain(func() {
		defer close(done)
		fn()
	})
	<-done
}

type testSurface struct{ test *testOpen }

// touchAt sends a touch's phase at at, as the host does, on the main
// thread.
func touchAt(phase uint8, at driver.Point) {
	t := time.Since(s.epoch).Seconds()
	onMainSync(func() { hostTouch(phase, float64(at.X), float64(at.Y), t) })
}

func (t *testSurface) Press(at driver.Point, hold time.Duration) error {
	touchAt(touchBegan, at)
	time.Sleep(max(hold+50*time.Millisecond, 60*time.Millisecond))
	touchAt(touchEnded, at)
	onMainSync(func() {
		root := s.view(s.root)
		if root == nil {
			return
		}
		// The host's TextField takes the tap that focuses a field.
		if v := hit(root, at, func(v *view) bool { return v.field != nil }); v != nil {
			s.focusField(v)
		} else if s.focused != nil {
			s.blurField(s.focused)
		}
		s.flush()
	})
	return nil
}

func (t *testSurface) RightClick(driver.Point) error {
	return errors.New("a watch has no secondary click: use LongPress")
}

// Drag moves a touch along path, 10 points a frame, then rests still
// before it lifts, so it ends without a fling.
func (t *testSurface) Drag(path []driver.Point) error {
	if len(path) == 0 {
		return errors.New("an empty drag")
	}
	touchAt(touchBegan, path[0])
	time.Sleep(100 * time.Millisecond)
	at := path[0]
	for _, to := range path[1:] {
		n := int(math.Max(math.Abs(float64(to.X-at.X)), math.Abs(float64(to.Y-at.Y)))/10) + 1
		for i := 1; i <= n; i++ {
			f := driver.Points(i) / driver.Points(n)
			touchAt(touchMoved, driver.Point{X: at.X + (to.X-at.X)*f, Y: at.Y + (to.Y-at.Y)*f})
			time.Sleep(16 * time.Millisecond)
		}
		at = to
	}
	for range 6 {
		touchAt(touchMoved, at)
		time.Sleep(16 * time.Millisecond)
	}
	touchAt(touchEnded, at)
	return nil
}

// Scroll moves the innermost scroll view under at by delta, which then
// reports its offset as when the user scrolls it.
func (t *testSurface) Scroll(at driver.Point, delta driver.Point) error {
	var err error
	onMainSync(func() {
		root := s.view(s.root)
		if root == nil {
			err = errors.New("nothing shows")
			return
		}
		v := hit(root, at, func(v *view) bool { return v.kind == driver.KindScroll })
		if v == nil {
			err = fmt.Errorf("no scroll view at %v", at)
			return
		}
		to := clampOffset(v, driver.Point{X: v.offset.X + delta.X, Y: v.offset.Y + delta.Y})
		s.w.op(opScrollTo, v.id)
		s.w.f64(float64(to.X), float64(to.Y))
		s.w.flag(false)
		s.flush()
	})
	time.Sleep(100 * time.Millisecond)
	return err
}

func (t *testSurface) Hover(driver.Point) error { return nil }

// Type enters text into the focused field, a character at a time.
func (t *testSurface) Type(text string) error {
	for _, r := range text {
		var err error
		onMainSync(func() {
			v := s.focused
			if v == nil {
				err = errors.New("no field has the focus")
				return
			}
			s.enter(v, v.value+string(r))
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *testSurface) Compose(string) error { return testhost.ErrNoInputMethod }
func (t *testSurface) Commit(string) error  { return testhost.ErrNoInputMethod }

// Key presses Enter, which submits the focused field, or Backspace,
// which deletes its last character; a watch has no other keys.
func (t *testSurface) Key(key string) error {
	var err error
	onMainSync(func() {
		v := s.focused
		if v == nil {
			err = errors.New("no field has the focus")
			return
		}
		switch key {
		case "Enter":
			v.report(driver.Event{Kind: driver.EventSubmitEditing, Text: v.value})
		case "Backspace":
			if _, n := utf8.DecodeLastRuneInString(v.value); n > 0 {
				s.enter(v, v.value[:len(v.value)-n])
			}
		default:
			err = fmt.Errorf("a watch has no %s key", key)
		}
		s.flush()
	})
	return err
}

func (t *testSurface) Back() error {
	s.engine.Dispatch(driver.Event{Kind: driver.EventBack})
	return nil
}

// Answer answers the alert as its button would, and takes it away.
func (t *testSurface) Answer(id uint32, index int) error {
	var err error
	onMainSync(func() {
		actions, ok := s.alerts[id]
		if !ok {
			err = fmt.Errorf("no alert %d shows", id)
			return
		}
		if index >= len(actions) {
			err = fmt.Errorf("the alert has no button %d", index)
			return
		}
		s.w.op(opDismiss, driver.NodeID(id))
		hostAlert(id, int32(index))
		s.flush()
	})
	return err
}

func (t *testSurface) ChooseMenu(uint32) error  { return testhost.ErrNoMenu }
func (t *testSurface) ChoosePopup(uint32) error { return testhost.ErrNoMenu }

// Screenshot has the host render the surface, as a PNG.
func (t *testSurface) Screenshot() ([]byte, error) {
	var png []byte
	onMainSync(func() {
		snapshot = nil
		purego.SyscallN(snapshotFn)
		png, snapshot = snapshot, nil
	})
	if len(png) == 0 {
		return nil, errors.New("the host drew nothing")
	}
	return png, nil
}

func (t *testSurface) Animating() bool {
	var busy bool
	onMainSync(func() {
		busy = s.animator.Ending() || len(s.motions) > 0
		for _, v := range s.views {
			if v != nil && (v.phase == phaseGliding || v.phase == phaseAnimating) {
				busy = true
			}
		}
	})
	return busy
}

// Resize does nothing: a watch's screen is the size it is.
func (t *testSurface) Resize(driver.Size) error { return nil }

func (t *testSurface) SetEnvironment(env driver.Environment) error {
	t.test.pin.Set(&env)
	s.engine.Repin()
	return nil
}

func (t *testSurface) Sync() { s.engine.Sync() }

package nsgotest

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/testhost"
)

// mainRan is set once Main hands the main thread to the platform, and
// opened once a test opened its app there: a process shows one.
var mainRan, opened bool

// Main runs the package's tests, from TestMain:
//
//	func TestMain(m *testing.M) { nsgotest.Main(m) }
//
// Headless it is m.Run. Under `nsgo test -target` it gives the main
// thread to the platform, as macOS, iOS and Windows need their windows'
// loop there, and runs the tests beside it.
func Main(m *testing.M) {
	if h := testhost.Current(); onTarget() && h != nil {
		mainRan = true
		h.Main(m.Run)
		return
	}
	os.Exit(m.Run())
}

// onTarget reports whether `nsgo test -target` runs the tests.
func onTarget() bool { return os.Getenv("NSGO_TEST_TARGET") != "" }

// Target is the platform `nsgo test -target` runs the tests on, such as
// "macos" or "ios_simulator", or "" headless.
func Target() string { return os.Getenv("NSGO_TEST_TARGET") }

// target drives an app shown on a platform: the model follows the
// batches the app's engine sends and the events the platform reports,
// and steps act with the platform's input where the model says a view
// is.
type target struct {
	a       *App
	surface testhost.Surface

	mu      sync.Mutex
	batches []*driver.MountBatch
	events  []driver.Event
}

// quietFor is how long nothing must change for a step to be over, and
// settleWait how long a step waits for that.
const (
	quietFor   = 100 * time.Millisecond
	settleWait = 5 * time.Second
)

func (a *App) startTarget(app func(cx *core.Scope) core.Node) {
	a.t.Helper()
	h := testhost.Current()
	switch {
	case h == nil:
		a.t.Fatal("nsgotest: no test host is linked: test the app's main package, which imports github.com/oneo-me/nsgo, with nsgo test -target")
	case !mainRan:
		a.t.Fatal("nsgotest: a platform needs the main thread: add func TestMain(m *testing.M) { nsgotest.Main(m) }")
	case opened && !h.Several():
		a.t.Skip("nsgotest: the test opens a second app, which only a desktop shows beside the first")
	}
	opened = true
	if a.store == nil {
		a.store = core.NewStore(nil)
	}
	t := &target{a: a}
	a.target = t
	// The test's own graph shares the app's store, as another window
	// would, for Storage.
	a.root = core.NewRoot(core.RootOptions{
		Post: func(fn func()) {
			a.mu.Lock()
			a.posts = append(a.posts, fn)
			a.mu.Unlock()
		},
		Clock: a.Clock, Storage: a.store,
	})
	a.t.Cleanup(a.root.Dispose)
	a.anim = driver.NewAnimator(func(driver.Event) {})
	s, err := h.Open(app, testhost.Options{
		Size: a.size, Environment: a.env, Storage: a.store, Host: a.host, Clock: a.Clock,
		FPS: os.Getenv("NSGO_TEST_FPS") != "",
		Observe: func(b *driver.MountBatch) {
			t.mu.Lock()
			t.batches = append(t.batches, b)
			t.mu.Unlock()
		},
		Event: func(ev driver.Event) {
			switch ev.Kind {
			case driver.EventScroll, driver.EventFocus, driver.EventBlur, driver.EventChangeText:
				t.mu.Lock()
				t.events = append(t.events, ev)
				t.mu.Unlock()
			}
		},
	})
	if err != nil {
		a.t.Fatalf("nsgotest: the platform cannot open the app: %v", err)
	}
	t.surface = s
	// The first batch can take longer than a step's quiet time to come,
	// as on Windows, where the window opens first.
	for deadline := time.Now().Add(settleWait); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		t.mu.Lock()
		came := len(t.batches) > 0
		t.mu.Unlock()
		if came {
			break
		}
	}
	t.settle()
}

// drain applies what came since the last call, reporting whether
// anything did.
func (t *target) drain() bool {
	t.mu.Lock()
	batches, events := t.batches, t.events
	t.batches, t.events = nil, nil
	t.mu.Unlock()
	for _, b := range batches {
		t.a.traceBatch(b, 0)
		t.a.apply(b)
	}
	for _, ev := range events {
		v := t.a.views[ev.ID]
		if v == nil {
			continue
		}
		switch ev.Kind {
		case driver.EventScroll:
			v.Offset, v.Applied = ev.Point, ev.Count
			if ev.Value != 0 {
				v.ZoomScale = ev.Value
			}
		case driver.EventFocus, driver.EventBlur:
			v.Focused = ev.Kind == driver.EventFocus
		case driver.EventChangeText:
			v.Text = ev.Text
		}
	}
	return len(batches)+len(events) > 0
}

// settle waits until no batch came and the platform animated nothing for
// quietFor, settleWait at most, applying what came.
func (t *target) settle() {
	deadline := time.Now().Add(settleWait)
	quiet := time.Now()
	for {
		t.a.mu.Lock()
		posts := t.a.posts
		t.a.posts = nil
		t.a.mu.Unlock()
		for _, fn := range posts {
			fn()
		}
		if t.drain() || t.surface.Animating() {
			quiet = time.Now()
		}
		if time.Since(quiet) >= quietFor || time.Now().After(deadline) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (t *target) check(err error) {
	t.a.t.Helper()
	if err != nil {
		t.a.t.Fatalf("nsgotest: %s: %v", t.a.stepName, err)
	}
}

// reveal scrolls v into view, as a user does before acting on it: each
// scroll view around it that hides its middle moves it to the middle, a
// gesture at a time. It returns where v's middle is then.
func (t *target) reveal(v *View) driver.Point {
	t.a.t.Helper()
	for range 30 {
		_, at := v.center()
		var hider *View
		var box driver.Bounds
		for p := v.Parent; p != nil; p = p.Parent {
			if p.Kind == driver.KindScroll && !p.Scroll.Behavior.NoScroll {
				b := p.surfaceFrame()
				if at.X < b.Origin.X || at.Y < b.Origin.Y || at.X >= b.Origin.X+b.Size.Width || at.Y >= b.Origin.Y+b.Size.Height {
					hider, box = p, b
					break
				}
			}
		}
		if hider == nil {
			return at
		}
		mid := driver.Point{X: box.Origin.X + box.Size.Width/2, Y: box.Origin.Y + box.Size.Height/2}
		d := driver.Point{X: at.X - mid.X, Y: at.Y - mid.Y}
		limit := func(v, size driver.Points) driver.Points { return min(max(v, -size*0.8), size*0.8) }
		if hider.Scroll.Horizontal {
			d = driver.Point{X: limit(d.X, box.Size.Width)}
		} else {
			d = driver.Point{Y: limit(d.Y, box.Size.Height)}
		}
		t.check(t.surface.Scroll(mid, d))
		t.settle()
	}
	_, at := v.center()
	return at
}

func (t *target) storage() core.Storage { return t.a.root.Storage() }

// menu takes a menu step with the platform's own menus: a platform that
// shows none skips the test.
func (t *target) menu(step func(testhost.Surface) error) {
	t.a.t.Helper()
	if err := step(t.surface); errors.Is(err, testhost.ErrNoMenu) {
		t.a.t.Skipf("nsgotest: %s: %v", t.a.stepName, err)
	} else {
		t.check(err)
	}
	t.settle()
}

// advance moves the test's clock, which the app's timers run on, by d:
// a frame at a time, settling between, when run, else at once. The
// platform's own animations run in real time.
func (t *target) advance(d time.Duration, run bool) {
	if !run {
		t.a.Clock.Jump(d)
		t.settle()
		return
	}
	for d > 0 {
		step := min(d, driver.FrameInterval)
		t.a.Clock.Advance(step)
		d -= step
		// What the step's timers posted runs before the next step, so
		// the timers it sets (an interval's next run) fall due in time.
		t.surface.Sync()
		t.drain()
	}
	t.settle()
}

func (t *target) setEnvironment(env driver.Environment) {
	t.check(t.surface.SetEnvironment(env))
	t.settle()
}

func (t *target) resize(size driver.Size) {
	t.check(t.surface.Resize(size))
	t.settle()
}

func (t *target) answer(alert driver.AlertConfig, index int) {
	t.check(t.surface.Answer(alert.ID, index))
	t.settle()
}

func (t *target) press(at driver.Point, hold time.Duration) { t.check(t.surface.Press(at, hold)) }

// pressWhile holds a press for hold, running during once the finger is
// down.
func (t *target) pressWhile(at driver.Point, hold time.Duration, during func()) {
	done := make(chan error, 1)
	go func() { done <- t.surface.Press(at, hold) }()
	time.Sleep(hold / 2)
	during()
	t.check(<-done)
}

func (t *target) rightClick(at driver.Point) { t.check(t.surface.RightClick(at)) }
func (t *target) hover(at driver.Point)      { t.check(t.surface.Hover(at)) }
func (t *target) back()                      { t.check(t.surface.Back()) }

func (t *target) typeText(v *View, text string) { t.check(t.surface.Type(text)) }
func (t *target) compose(text string)           { t.ime(t.surface.Compose(text)) }
func (t *target) commit(text string)            { t.ime(t.surface.Commit(text)) }

// ime checks a composition step: a machine without an input method skips
// the test.
func (t *target) ime(err error) {
	t.a.t.Helper()
	if errors.Is(err, testhost.ErrNoInputMethod) {
		t.a.t.Skipf("nsgotest: %s: %v", t.a.stepName, err)
	}
	t.check(err)
}

// clear selects the field's text and deletes it, as a keyboard does.
func (t *target) clear(v *View) {
	t.check(t.surface.Key("mod+a"))
	t.check(t.surface.Key("Backspace"))
}

func (t *target) keys(v *View, key string) { t.check(t.surface.Key(key)) }

func (t *target) drag(start driver.Point, path []driver.Point) {
	t.check(t.surface.Drag(append([]driver.Point{start}, path...)))
}

// scroll drags v's content by d along its axis, starting from its middle.
func (t *target) scroll(v *View, d driver.Points) {
	_, at := v.center()
	delta := driver.Point{Y: d}
	if v.Scroll.Horizontal {
		delta = driver.Point{X: d}
	}
	t.check(t.surface.Scroll(at, delta))
}

// Screenshot captures the app as the platform shows it, as name.png in
// the directory `nsgo test -target` saves screenshots to. Headless runs
// have no pixels: it logs and does nothing.
func (a *App) Screenshot(name string) {
	a.t.Helper()
	if a.target == nil {
		a.t.Logf("nsgotest: no screenshot %q headless; nsgo test -target takes it", name)
		return
	}
	a.target.settle()
	png, err := a.target.surface.Screenshot()
	if err != nil {
		a.t.Fatalf("nsgotest: screenshot %q: %v", name, err)
	}
	// The tool saves it: the app may run on a phone, or in a browser. A
	// device's log keeps lines of a few kilobytes, so it goes in chunks.
	data := base64.StdEncoding.EncodeToString(png)
	const chunk = 3000
	n := (len(data) + chunk - 1) / chunk
	for i := range n {
		fmt.Printf("nsgo-screenshot %s %d/%d %s\n", name, i+1, n, data[i*chunk:min((i+1)*chunk, len(data))])
	}
}

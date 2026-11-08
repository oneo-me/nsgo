// Package testhost is where a backend built for `nsgo test -target` (the
// nsgotest build tag) offers its test host, and where nsgotest finds it:
// the platform's loop on the main thread, an app shown on a real surface,
// and native input at the surface's points.
package testhost

import (
	"errors"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
)

// Host is a backend's test host.
type Host interface {
	// Main runs the platform's loop on the calling goroutine, the main
	// thread, and run on another; it ends the process with run's code.
	Main(run func() int)
	// Open shows app on a new surface as the backend's Run does, with
	// o's size and what it observes.
	Open(app func(cx *core.Scope) core.Node, o Options) (Surface, error)
	// Several reports whether a process opens several surfaces: a
	// desktop's windows; a phone and a page show one.
	Several() bool
}

// Options are what a test opens an app with.
type Options struct {
	Size driver.Size
	// Environment pins what the test sets over what the platform reports
	// (Pin's fields), from the first frame.
	Environment *driver.Environment
	// Storage seeds the app's storage; in memory when nil.
	Storage *core.Store
	// Observe sees every batch the engine sends, on the engine's
	// goroutine, before the platform applies it; Event every event the
	// platform reports to the engine, on the goroutine it reports from.
	// Neither may block.
	Observe func(*driver.MountBatch)
	Event   func(driver.Event)
	// Host is the channel of the test's embedding host, nil without one.
	Host *core.Host
	// Clock is the test's, which the app's timers run on in place of the
	// system's: they fire as the test moves it.
	Clock core.Clock
	// FPS measures the app with the backend's performance monitor and
	// logs its readings once a second (nsgo test -fps), where the backend
	// has one: macOS, iOS, Windows and Linux.
	FPS bool
}

// Surface is an app shown on a platform. Its methods act as the user
// does, with the platform's own input, at points of the surface (its top
// left is 0, 0), and return once the input is posted.
type Surface interface {
	// Press presses at at for hold (a tap when 0), as a finger or the
	// mouse's primary button does.
	Press(at driver.Point, hold time.Duration) error
	// RightClick clicks the secondary button at at.
	RightClick(at driver.Point) error
	// Drag puts a finger or the primary button down at path[0], rests,
	// moves through the rest of path and lifts.
	Drag(path []driver.Point) error
	// Scroll scrolls the content under at by delta, as a wheel, a
	// trackpad or a drag that rests before it lifts does.
	Scroll(at driver.Point, delta driver.Point) error
	// Hover moves the mouse to at; a point outside the surface leaves it.
	Hover(at driver.Point) error
	// Type types text into the focused field, a key at a time.
	Type(text string) error
	// Compose has an input method compose text in the focused field
	// through the platform's text input client, as one marks what it
	// composes: "" ends the composition with nothing. Commit has it
	// commit text there, ending the composition. Both answer
	// ErrNoInputMethod where none can compose.
	Compose(text string) error
	Commit(text string) error
	// Key presses a key with modifiers, as the keymap names them:
	// "Enter", "Backspace", "Escape", "mod+a".
	Key(key string) error
	// Back goes back as the platform's back button, gesture or Escape.
	Back() error
	// Answer presses the button at index of the alert id shows, or
	// dismisses it with -1.
	Answer(id uint32, index int) error
	// ChooseMenu chooses the application menu's item id, as its menu bar
	// does; ErrNoMenu where the platform shows none.
	ChooseMenu(id uint32) error
	// ChoosePopup chooses item id of the context menu showing, or closes
	// it for 0; ErrNoMenu where the platform shows none.
	ChoosePopup(id uint32) error
	// Screenshot returns the surface as a PNG.
	Screenshot() ([]byte, error)
	// Animating reports whether the platform animates anything that ends:
	// a transition, a scroll's glide, a modal coming in.
	Animating() bool
	// Resize resizes the surface; SetEnvironment changes what it reports,
	// as a system setting does.
	Resize(size driver.Size) error
	SetEnvironment(env driver.Environment) error
	// Sync returns once the app's engine ran what was posted to it
	// before: what the timers the test's clock fired run.
	Sync()
}

var host Host

// ErrNoMenu is a platform's answer to a menu step where it shows no
// native menus: a phone's, or the web's.
var ErrNoMenu = errors.New("the platform shows no native menus")

// ErrNoInputMethod is a composition step's answer where the machine has no
// input method to compose with, as Windows without an IME.
var ErrNoInputMethod = errors.New("no input method composes on this machine")

// Register makes h the test host; a backend's init calls it.
func Register(h Host) { host = h }

// Current returns the test host, or nil when the backend linked has none.
func Current() Host { return host }

// Pin applies what a test set in pin over env, as the platform reports
// it: the appearance, reduced motion, and the locale and text size when
// set. The rest, such as the OS, the idiom and the safe area, stays the
// platform's.
func Pin(env driver.Environment, pin *driver.Environment) driver.Environment {
	if pin == nil {
		return env
	}
	env.Dark, env.ReduceMotion = pin.Dark, pin.ReduceMotion
	if pin.Locale != "" {
		env.Locale = pin.Locale
	}
	if pin.FontScale != 0 {
		env.FontScale = pin.FontScale
	}
	return env
}

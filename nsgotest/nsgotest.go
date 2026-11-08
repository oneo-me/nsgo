// Package nsgotest runs NSGo apps in Go tests, as net/http/httptest runs
// handlers: headless by default, with approximate text, a manual clock
// and every batch applied to a model of the native views that tests
// query; `nsgo test -target` runs the same tests on a platform, with its
// native input.
//
//	func TestAdd(t *testing.T) {
//		app := nsgotest.Start(t, Root)
//		app.Type("id:draft", "Milk")
//		app.Press("role:button Add")
//		app.Find("text:Milk")
//	}
//
// Selectors name the views a user sees, in Playwright's manner:
//
//	id:<TestID>              the view with that test ID
//	label:<Label>            the view with that accessibility label
//	text:<text>              a text, or a field holding it (its placeholder while empty), containing text
//	role:<role>              a view with that role ("button", "checkbox", "textbox", ...)
//	role:<role> <name>       one whose label is name, or whose texts contain it
//	view:<node>              the view of that node ID, as traces print it
//	anything else            a test ID, else a label, else a whole text
//
// label:, text: and a role's name take /regexp/flags, with JavaScript's
// i, m and s flags. " >> " chains selectors, each part looking inside the
// views the last matched, and " >> nth=<i>" keeps one match. Matches come
// in the order a user finds them: the topmost modal's first, then the rest
// in tree order.
//
// Steps act as a user does and send what a platform's input sends: Press
// is a finger down and up, Type a key at a time, Scroll a drag 10 points
// a frame. A step whose selector matches no view, or more than one it
// would act on, or a disabled one, fails the test. Each step then waits,
// moving the clock a frame at a time for at most 2 seconds, until no
// animation that ends runs.
package nsgotest

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
)

// Tests check that each commit sends all that changed.
func init() { driver.CheckCommits = true }

// Option configures Start.
type Option func(*App)

// Size sets the surface size; 390 by 844 points by default.
func Size(width, height driver.Points) Option {
	return func(a *App) { a.size = driver.Size{Width: width, Height: height} }
}

// Storage starts the app with entries in its storage, as if kept from an
// earlier run; it starts empty otherwise.
func Storage(entries map[string]string) Option {
	return func(a *App) { a.store = core.NewStore(seed(entries)) }
}

// Environment starts the app in env from its first frame, as a platform
// reports it before the app mounts: screens have the system header where
// env has HeaderHeight.
func Environment(env driver.Environment) Option {
	return func(a *App) { a.env = &env }
}

// Host runs the app with h as its channel to an embedding host, so a test
// sends it events and answers its calls as the host would.
func Host(h *core.Host) Option {
	return func(a *App) { a.host = h }
}

// TextEngine measures and lays out the app's text with e instead of
// core.ApproxTextEngine, as a test that checks frames another headless
// runner computed needs its measurements.
func TextEngine(e driver.TextEngine) Option {
	return func(a *App) { a.textEngine = e }
}

// Offscreen has selectors match every view mounted and not hidden, also
// one outside the window or clipped away, as NSH's headless runs act.
func Offscreen() Option {
	return func(a *App) { a.offscreen = true }
}

// PendingAlerts leaves the alerts the app shows unanswered unless the
// test answers them, as NSH's headless runs present none.
func PendingAlerts() Option {
	return func(a *App) { a.pendingAlerts = true }
}

// NoCaretMoves keeps typing from reporting where the caret moves, as
// NSH's headless model types: a test reports caret moves itself.
func NoCaretMoves() Option {
	return func(a *App) { a.noCaretMoves = true }
}

// ManualFrames keeps steps from taking display frames: the animations a
// step starts move only as Advance moves the clock, so a test can look at
// them on the way.
func ManualFrames() Option {
	return func(a *App) { a.manualFrames = true }
}

// Errors hands what the app reports failing to report instead of failing
// the test, as a host's overlay shows it while the app runs on. It
// applies headless only.
func Errors(report func(error)) Option {
	return func(a *App) { a.report = report }
}

// seed is a storage backend that loads entries and keeps nothing.
type seed map[string]string

func (s seed) Load() (map[string]string, error) { return s, nil }
func (seed) Write(driver.StorageChange)         {}

// App is an app running in a test. Its methods run on the test's
// goroutine; the app's goroutines reach it through Scope.Post, which App
// runs when it settles.
type App struct {
	t    testing.TB
	size driver.Size
	// report takes what the app reports failing (Errors).
	report func(error)
	// Clipboard is the clipboard's text, ClipboardImage its image as the
	// app last set it or reads it, and ClipboardFiles the file: URLs it
	// holds; Announced what screen readers were asked to say.
	Clipboard      string
	ClipboardImage []byte
	ClipboardFiles []string
	Announced      []string
	// InputSource is the keyboard input source the app selected, as a
	// test sees it; inputSourceWas the one that selection replaced.
	InputSource    string
	inputSourceWas string
	// OpenedURLs are the URLs the app opened, oldest first. OpenURL
	// answers whether each opens; every one does when it is nil.
	OpenedURLs []string
	OpenURL    func(url string) bool
	// Picks are the photo picks the app asked for, oldest first.
	// PickImages answers each with the files the user picks; with none
	// the user cancels.
	Picks      []driver.PickRequest
	PickImages func(r driver.PickRequest) []driver.PickedFile
	// PickScreenColor answers Scope.PickScreenColor with the color the user
	// samples; without it, or with false, the user cancels.
	PickScreenColor func() (driver.Color, bool)
	// FilePicks are the file picks the app asked for, oldest first.
	// PickFiles answers each with the files the user picks; with none
	// the user cancels.
	FilePicks []driver.FilesRequest
	PickFiles func(r driver.FilesRequest) []driver.File
	// DirectoryPicks are the folder picks the app asked for, oldest
	// first. PickDirectories answers each with the folders the user
	// picks; with none the user cancels.
	DirectoryPicks  []driver.DirectoriesRequest
	PickDirectories func(r driver.DirectoriesRequest) []driver.Directory
	// Saves are the saves the app asked for, oldest first. SaveFile
	// answers each with the path the user saves to, where the file is
	// written; with none, or "", the user cancels.
	Saves    []driver.SaveRequest
	SaveFile func(r driver.SaveRequest) string
	// OpenedFiles are the files the app opened in other apps, oldest
	// first. OpenFile answers whether an app opens each; every one does
	// when it is nil.
	OpenedFiles []driver.SharedFile
	OpenFile    func(f driver.SharedFile) bool
	// Shares are the shares the app asked for, oldest first; each sheet
	// shows.
	Shares []driver.ShareRequest
	// Installs are the packages the app handed Android's installer,
	// oldest first; each installer starts. InstallAllowed is what
	// CanInstallPackages answers, and InstallSettings counts the times the
	// app opened the setting, which opens.
	Installs        []driver.SharedFile
	InstallAllowed  bool
	InstallSettings int
	// OnAlert answers each alert the app shows with the index of the
	// button the user presses, or -1 to dismiss it, once the step that
	// showed it ends. Without it Alerts lists them and Answer answers
	// them; one still unanswered when the next step starts gets its
	// default answer: its cancel button, else dismissed when cancelable,
	// else the step fails.
	OnAlert func(driver.AlertConfig) int
	// Appearance is the appearance the app last forced; Quits how often
	// it asked to quit; BackUnhandled how many backs nothing took, which
	// leave an Android app.
	Appearance    driver.Appearance
	Quits         int
	BackUnhandled int
	// PopupMenu is the context menu the app shows with PopupMenu, at
	// PopupAt; nil once it closed.
	PopupMenu []driver.MenuEntry
	PopupAt   driver.Point
	popupID   uint32
	// AccessibilityFocus is the view the app last moved the screen
	// reader's focus to.
	AccessibilityFocus *View
	// Keymap is the keyboard shortcuts the app binds; Title its title.
	Keymap []string
	Title  string
	// Language is the language the platform labels its menus in, and
	// LanguageForced whether the app forces it (core.Scope.SetLocale),
	// which Apple platforms keep for the system's words in later launches.
	Language       string
	LanguageForced bool
	// Menu is the application menu the app set; Window the window
	// commands it sent, oldest first.
	Menu   []driver.MenuEntry
	Window []driver.WindowCommand
	// ImageSizes are the pixel sizes of the images the model's platform
	// loads, by URI, which answer Scope.ImageSize; others fail to load.
	ImageSizes map[string]driver.Size
	tree       *core.Tree
	root       *core.Scope
	store      *core.Store
	Clock      *core.ManualClock

	// StatusBar is the status bar the app last set.
	StatusBar driver.StatusBarConfig
	// LayoutAnimation is the layout animation the last batch to carry
	// one asked the platform for.
	LayoutAnimation *driver.LayoutAnimation

	mu    sync.Mutex
	posts []func()
	// frameWanted is set while the graph waits for a display frame.
	frameWanted bool

	views  map[driver.NodeID]*View
	anim   *driver.Animator
	start  time.Time
	alerts []driver.AlertConfig
	rootID driver.NodeID
	edits  map[driver.NodeID]uint32
	env    *driver.Environment
	// blocked is set while the touch responder keeps native drags from
	// starting (ItemBlockNative).
	blocked bool
	// hovered are the views the mouse is over, innermost first.
	hovered []*View
	// mouse is where it is in the surface.
	mouse driver.Point
	// stepName names the step running, for traces; trace receives them.
	stepName string
	trace    io.Writer
	// target drives the app on a platform under `nsgo test -target`.
	target *target
	// manualFrames keeps steps from waiting for animations.
	manualFrames bool
	// host is the embedding host's channel, nil without one.
	host *core.Host
	// bound are the views' gesture and scroll bindings.
	bound map[driver.NodeID]*Bindings
	// offscreen has selectors match views outside the window too.
	offscreen bool
	// pendingAlerts leaves alerts unanswered between steps.
	pendingAlerts bool
	// noCaretMoves keeps typing from reporting caret moves.
	noCaretMoves bool
	// textEngine measures text; core.ApproxTextEngine when nil.
	textEngine driver.TextEngine
	// lastPress and lastPressAt are the view pressed last and when;
	// clicks counts the presses of it in a row.
	lastPress   driver.NodeID
	lastPressAt time.Time
	clicks      uint32
}

// Start runs app and settles it; the test's cleanup disposes it. Under
// `nsgo test -target` it opens the app on that platform instead.
func Start(t testing.TB, app func(cx *core.Scope) core.Node, opts ...Option) *App {
	t.Helper()
	a := &App{
		t:     t,
		size:  driver.Size{Width: 390, Height: 844},
		Clock: core.NewManualClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
		views: map[driver.NodeID]*View{},
		edits: map[driver.NodeID]uint32{},
	}
	for _, o := range opts {
		o(a)
	}
	if os.Getenv("NSGO_TEST_TRACE") != "" {
		a.trace = os.Stdout
	}
	a.stepName = "start"
	if onTarget() && a.manualFrames {
		t.Skip("nsgotest: ManualFrames steps the clock, which runs headless only")
	}
	if onTarget() {
		a.startTarget(app)
		return a
	}
	a.root = core.NewRoot(core.RootOptions{
		Post: func(fn func()) {
			a.mu.Lock()
			a.posts = append(a.posts, fn)
			a.mu.Unlock()
		},
		Clock: a.Clock,
		Report: func(err error) {
			if a.report != nil {
				a.report(err)
				return
			}
			t.Errorf("nsgotest: the app failed: %v", err)
		},
		// Frames run as the clock advances, as a platform's display does.
		RequestFrame: func() {
			a.mu.Lock()
			a.frameWanted = true
			a.mu.Unlock()
		},
		Storage: a.store,
		Host:    a.host,
	})
	t.Cleanup(a.root.Dispose)
	a.start = a.Clock.Now()
	a.anim = driver.NewAnimator(func(ev driver.Event) {
		a.mu.Lock()
		a.posts = append(a.posts, func() { a.tree.Dispatch(ev) })
		a.mu.Unlock()
	})
	var engine driver.TextEngine = core.ApproxTextEngine{}
	if a.textEngine != nil {
		engine = a.textEngine
	}
	a.tree = core.NewTree(a.root, engine)
	a.tree.Resize(a.size)
	if a.env != nil {
		a.tree.SetEnvironment(*a.env)
	}
	a.tree.Mount(app)
	a.done()
	return a
}

// Storage returns the app's storage, to read what it kept or change it as
// another of the app's windows would.
func (a *App) Storage() core.Storage {
	if a.target != nil {
		return a.target.storage()
	}
	return a.root.Storage()
}

// Settle runs what the app's goroutines posted and applies the commits
// that follow, until nothing is left.
func (a *App) Settle() {
	if a.target != nil {
		a.target.settle()
		return
	}
	for {
		a.mu.Lock()
		posts := a.posts
		a.posts = nil
		a.mu.Unlock()
		for _, fn := range posts {
			fn()
		}
		began := time.Now()
		b := a.tree.Commit()
		if b != nil {
			a.traceBatch(b, time.Since(began))
			a.apply(b)
		}
		a.animate()
		a.mu.Lock()
		more := len(a.posts) > 0
		a.mu.Unlock()
		if b == nil && len(posts) == 0 && !more {
			if runtime.GOOS != "js" || !a.yieldToPage() {
				return
			}
		}
	}
}

// yieldToPage lets the page's engine run what it queued, as the reactions
// of the promises a TSX app settled, which run only once Go gives the page
// its turn, and reports whether that posted anything.
func (a *App) yieldToPage() bool {
	time.Sleep(time.Millisecond)
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.posts) > 0
}

// Post runs fn on the test's goroutine as the next step or wait settles:
// what another goroutine hands the test, as a stubbed request does.
func (a *App) Post(fn func()) {
	a.mu.Lock()
	a.posts = append(a.posts, fn)
	a.mu.Unlock()
}

// settleLimit is how long of the clock's time a step waits for the app
// to be quiet.
const settleLimit = 2 * time.Second

// step starts the step name: the alerts the last one left unanswered get
// their default answer.
func (a *App) step(format string, args ...any) {
	a.t.Helper()
	a.stepName = fmt.Sprintf(format, args...)
	for len(a.alerts) > 0 && !a.pendingAlerts {
		index, err := defaultAnswer(a.alerts[0])
		if err != nil {
			a.t.Fatalf("nsgotest: %v", err)
		}
		a.Answer(index)
	}
}

// done ends a step: it settles, then takes display frames while an
// animation that ends runs and lists bind the rows they pace, for at most
// settleLimit of the clock's time, and has OnAlert answer the alerts
// shown. Frames the app asks for itself
// do not hold a step, since its own loops may never end: Advance runs
// them.
func (a *App) done() {
	a.t.Helper()
	if a.target != nil {
		a.target.settle()
		a.answerAlerts()
		return
	}
	a.Settle()
	limit := a.Clock.Now().Add(settleLimit)
	for !a.manualFrames && (a.anim.Ending() || a.tree.Pacing()) && a.Clock.Now().Before(limit) {
		a.Clock.Advance(driver.FrameInterval)
		a.runFrame()
		a.Settle()
	}
	a.answerAlerts()
}

func (a *App) answerAlerts() {
	a.t.Helper()
	for a.OnAlert != nil && len(a.alerts) > 0 {
		a.Answer(a.OnAlert(a.alerts[0]))
	}
}

// defaultAnswer is how a user backing out answers alert: its cancel
// button, else dismissing it when it can be.
func defaultAnswer(alert driver.AlertConfig) (int, error) {
	for i, b := range alert.Actions {
		if b.Role == driver.AlertCancel {
			return i, nil
		}
	}
	if alert.Cancelable {
		return -1, nil
	}
	name := alert.Title
	if name == "" {
		name = alert.Message
	}
	return 0, fmt.Errorf("the alert %q needs an answer: answer it with Answer or OnAlert", name)
}

func (a *App) wantsFrame() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.frameWanted
}

// Advance moves the clock by d, runs the timers and display frames that
// come due and settles. A frame runs every FrameInterval while the app
// waits for one, as a display's frames do.
func (a *App) Advance(d time.Duration) {
	a.t.Helper()
	if a.target != nil {
		a.target.advance(d, true)
		return
	}
	a.stepName = fmt.Sprintf("advance(%v)", d)
	a.elapse(d)
	a.Settle()
}

// elapse moves the clock by d a frame at a time, as Advance does.
func (a *App) elapse(d time.Duration) {
	for d > 0 {
		step := min(d, driver.FrameInterval)
		a.Clock.Advance(step)
		d -= step
		a.runFrame()
		a.Settle()
	}
}

// FastForward jumps the clock ahead by d, as a laptop closed for a while
// and opened: each timer that comes due on the way runs once, at the
// end, and so does one display frame.
func (a *App) FastForward(d time.Duration) {
	a.t.Helper()
	if a.target != nil {
		a.target.advance(d, false)
		return
	}
	a.stepName = fmt.Sprintf("fastForward(%v)", d)
	a.Clock.Jump(d)
	a.runFrame()
	a.Settle()
}

// runFrame runs the display frame the app waits for, if it waits for one.
func (a *App) runFrame() {
	a.mu.Lock()
	wanted := a.frameWanted
	a.frameWanted = false
	a.mu.Unlock()
	if wanted {
		a.root.RunFrame()
	}
}

// WaitFor settles until selector matches a view, failing the test after
// timeout of real time. Use it for work on other goroutines.
func (a *App) WaitFor(selector string, timeout time.Duration) *View {
	a.t.Helper()
	a.WaitUntil(timeout, func() bool { return len(a.Query(selector)) == 1 })
	return a.Find(selector)
}

// WaitForGone settles until no view matches selector, failing the test
// after timeout of real time.
func (a *App) WaitForGone(selector string, timeout time.Duration) {
	a.t.Helper()
	a.WaitUntil(timeout, func() bool { return len(a.Query(selector)) == 0 })
}

// WaitUntil settles until cond holds, checking it after each change,
// failing the test after timeout of real time.
func (a *App) WaitUntil(timeout time.Duration, cond func() bool) {
	a.t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		a.Settle()
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			a.t.Fatalf("nsgotest: the condition did not hold after %v\n%s", timeout, a.Dump())
		}
		time.Sleep(time.Millisecond)
	}
}

// SetEnvironment reports env as a platform would and settles.
func (a *App) SetEnvironment(env driver.Environment) {
	a.t.Helper()
	if a.target != nil {
		a.target.setEnvironment(env)
		return
	}
	a.tree.SetEnvironment(env)
	a.Settle()
}

// Resize sets the surface size and settles.
func (a *App) Resize(width, height driver.Points) {
	a.t.Helper()
	a.size = driver.Size{Width: width, Height: height}
	if a.target != nil {
		a.target.resize(a.size)
		return
	}
	a.tree.Resize(a.size)
	a.Settle()
}

// FindMounted returns the one view selector matches among every view
// mounted, shown or not: hidden, empty, scrolled away or clipped, as a
// test reading what the platform received of a view finds it.
func (a *App) FindMounted(selector string) *View {
	a.t.Helper()
	vs := a.QueryMounted(selector)
	switch len(vs) {
	case 0:
		a.t.Fatalf("nsgotest: no view matches %q\n%s", selector, a.Dump())
	case 1:
		return vs[0]
	default:
		a.t.Fatalf("nsgotest: %d views match %q\n%s", len(vs), selector, a.Dump())
	}
	return nil
}

// QueryMounted returns the views selector matches among every view
// mounted, shown or not, in tree order.
func (a *App) QueryMounted(selector string) []*View {
	a.t.Helper()
	sel, err := parseSelector(selector)
	if err != nil {
		a.t.Fatalf("nsgotest: %v", err)
	}
	return a.resolveIn(sel, a.mounted())
}

// Find returns the one visible view selector matches.
func (a *App) Find(selector string) *View {
	a.t.Helper()
	vs := a.Query(selector)
	switch len(vs) {
	case 0:
		a.t.Fatalf("nsgotest: no view matches %q\n%s", selector, a.Dump())
	case 1:
		return vs[0]
	default:
		a.t.Fatalf("nsgotest: %d views match %q\n%s", len(vs), selector, a.Dump())
	}
	return nil
}

// Exists reports whether a visible view matches selector.
func (a *App) Exists(selector string) bool {
	a.t.Helper()
	return len(a.Query(selector)) > 0
}

// Enabled reports whether the one view selector matches takes input:
// neither it nor a view around it is disabled, and an input is editable.
func (a *App) Enabled(selector string) bool {
	a.t.Helper()
	return !a.Find(selector).disabled(nil)
}

// Query returns the visible views selector matches, the topmost modal's
// first, then the rest in tree order.
func (a *App) Query(selector string) []*View {
	a.t.Helper()
	sel, err := parseSelector(selector)
	if err != nil {
		a.t.Fatalf("nsgotest: %v", err)
	}
	return a.resolve(sel)
}

// Texts returns the texts shown, in the order Query finds them.
func (a *App) Texts() []string {
	var out []string
	for _, v := range a.byLayer() {
		if v.Kind == driver.KindText {
			out = append(out, v.Text)
		}
	}
	return out
}

// SpanStyle returns the resolved style of the characters selector names
// in a text: "text:<chars>" (or a chain ending in it) finds the text
// that shows them, and the style is that of their first character, as a
// span sets it over the text's.
func (a *App) SpanStyle(selector string) driver.TextStyle {
	a.t.Helper()
	return a.Span(selector).Style
}

// Span returns the span of the characters selector names in a text, as
// SpanStyle finds them: its style and what it draws behind them; the
// text's own style where no span sets one.
func (a *App) Span(selector string) driver.TextSpan {
	a.t.Helper()
	sel, err := parseSelector(selector)
	if err != nil {
		a.t.Fatalf("nsgotest: %v", err)
	}
	last := sel.parts[len(sel.parts)-1]
	v := a.Find(selector)
	if v.Kind != driver.KindText || last.kind != "text" {
		a.t.Fatalf("nsgotest: %q names no characters of a text; end it in text:", selector)
	}
	at := -1
	if last.text.re != nil {
		if loc := last.text.re.FindStringIndex(v.Text); loc != nil {
			at = loc[0]
		}
	} else {
		at = strings.Index(v.Text, last.text.want)
	}
	if at < 0 || at >= len(v.Text) {
		a.t.Fatalf("nsgotest: %q names no character of %q", selector, v.Text)
	}
	for _, sp := range v.Spans {
		if at >= sp.Start && at < sp.End {
			return sp
		}
	}
	return driver.TextSpan{Start: 0, End: len(v.Text), Style: v.TextStyle}
}

// Alerts returns the alerts shown and not answered yet, oldest first.
func (a *App) Alerts() []driver.AlertConfig { return a.alerts }

// Answer chooses the button at index (-1: dismiss) of the oldest alert
// shown.
func (a *App) Answer(index int) {
	a.t.Helper()
	if len(a.alerts) == 0 {
		a.t.Fatal("nsgotest: no alert to answer")
	}
	alert := a.alerts[0]
	a.alerts = a.alerts[1:]
	if a.target != nil {
		a.target.answer(alert, index)
		return
	}
	a.dispatch(driver.Event{Kind: driver.EventAlert, Count: alert.ID, Index: index})
}

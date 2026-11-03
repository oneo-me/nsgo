package tsx_test

import (
	"fmt"
	"io/fs"
	"log"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/nsgotest"
	"github.com/oneo-me/nsgo/tsx/compiler"
)

// The compat cases of NSH (nsh/tests/compat/cases, docs/dev/compatibility.md
// "Scripts"): an app per domain, a module per case, and script.txt, the
// steps that check them. compatRun runs a domain's script on NSGo, a
// subtest per case: the steps it knows, and the others counted as not
// run, which a case of compatUnrun lists.

// compatFailing are the cases a step of fails on NSGo, by domain and
// case: what is left to port (docs/roadmap.md, "The compat cases"). The
// modules domain's app and the macOS mod-standard-* cases import the
// standard modules NSGo leaves out; the others fail on an event, a
// layout or a module NSGo does not match yet. A case that passes is
// taken off.
var compatFailing = map[string][]string{
	"android": {"base-dialog-android"},
	"base":    {"base-alert-dialog", "base-button", "base-dialog", "base-dialog-backdrop", "base-nav-page", "base-nav-stack", "base-safe-area-keyboard", "base-text-view", "base-popups", "base-switch", "base-checkbox", "base-radio", "base-tabs", "base-plot", "base-token", "base-resizable", "base-dock", "base-toast", "base-editor", "base-diff"},
	"browser": {"base-popups-desktop"},
	"core":    {"core-animated", "core-canvas", "core-elements", "core-resource", "core-primitives", "core-motion", "core-overlay"},
	"events":  {"events-context-menu", "events-gestures", "events-input", "events-media", "events-pointer", "events-scroll", "events-touch", "events-composition"},
	"layout":  {"core-theme", "layout-box", "layout-direction", "layout-flex", "layout-grid", "layout-tailwind"},
	"macos":   {"desktop-props", "mod-standard-system", "mod-standard-files", "base-dock"},
	"modules": {"mod-standard-access", "mod-accessibility", "mod-android", "mod-clipboard", "mod-picker", "mod-desktop", "mod-device", "mod-host-channel", "mod-ios", "mod-keymap", "mod-mobile", "mod-storage", "mod-host"},
	"paint":   {"paint-misc", "paint-text", "state-styles", "paint-inline-align"},
	"props":   {"props-list-events", "props-list-layout", "props-refresh-indicator", "props-text-lines", "props-view-styling", "props-media-image-variants", "props-media-image-inline", "props-list-start"},
	"web":     {"web-blob", "web-encoding", "web-events", "web-fetch", "web-streams", "web-timers", "web-url"},
}

// compatUnrun are the kinds of steps not run yet: what checks NSH's text
// forms of styles, props and events, and what stands in for touches and
// states.
var compatUnrun = []string{"style", "nostyle", "prop", "noprop", "event", "touch", "state", "statusbar", "keyboard",
	"origin"}

// TestNSHCompat runs NSH's compat cases headless.
func TestNSHCompat(t *testing.T) {
	root := filepath.Join("..", "nsh", "tests", "compat", "cases")
	domains, err := os.ReadDir(root)
	if err != nil {
		t.Skip("nsh/ is not checked out")
	}
	logs := &compatLog{}
	log.SetOutput(logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	unrun := map[string]int{}
	for _, d := range domains {
		if !d.IsDir() {
			continue
		}
		t.Run(d.Name(), func(t *testing.T) {
			compatRun(t, filepath.Join(root, d.Name()), compatFailing[d.Name()], logs, unrun)
		})
	}
	t.Logf("steps not run yet: %v", unrun)
}

// compatLog keeps what the apps log, which log steps read.
type compatLog struct {
	mu    sync.Mutex
	lines []string
}

func (l *compatLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, line := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		l.lines = append(l.lines, line)
	}
	return len(p), nil
}

func (l *compatLog) has(text string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.ContainsFunc(l.lines, func(s string) bool { return strings.Contains(s, text) })
}

// compatCase is a case of a script: its name and steps.
type compatCase struct {
	name  string
	steps []string
}

// compatScript reads a script: the steps before the first case's tap,
// which set up the app, and its cases.
func compatScript(t *testing.T, path string) (setup []string, cases []compatCase) {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "covers ") {
			continue
		}
		if name, ok := strings.CutPrefix(line, "tap id:case-"); ok {
			cases = append(cases, compatCase{name: name})
		}
		if len(cases) == 0 {
			setup = append(setup, line)
		} else {
			cases[len(cases)-1].steps = append(cases[len(cases)-1].steps, line)
		}
	}
	return setup, cases
}

// compatTB fails a step by panicking with a compatFailure, for the case
// to report and the next case to go on.
type compatTB struct{ testing.TB }

type compatFailure struct{ msg string }

func (compatTB) Fatalf(format string, args ...any) {
	panic(compatFailure{fmt.Sprintf(format, args...)})
}
func (compatTB) Fatal(args ...any) { panic(compatFailure{fmt.Sprint(args...)}) }
func (compatTB) FailNow()          { panic(compatFailure{"failed"}) }
func (compatTB) Errorf(format string, args ...any) {
	panic(compatFailure{fmt.Sprintf(format, args...)})
}
func (compatTB) Error(args ...any) { panic(compatFailure{fmt.Sprint(args...)}) }

func compatRun(t *testing.T, dir string, failing []string, logs *compatLog, unrun map[string]int) {
	fsys := nshApp(t, dir)
	setup, cases := compatScript(t, filepath.Join(dir, "script.txt"))
	env := core.Environment{Locale: "en-US", FontScale: 1, PixelRatio: 1, OS: core.OSIOS, Idiom: core.IdiomPhone}
	for _, step := range setup {
		f := strings.Fields(step)
		switch f[0] {
		case "platform":
			env.OS = core.OS(f[1])
			if env.OS != core.OSIOS && env.OS != core.OSAndroid || slices.Contains(f, "desktop") {
				env.Idiom = core.IdiomDesktop
			}
		case "fontscale":
			scale, _ := strconv.ParseFloat(f[1], 32)
			env.FontScale = float32(scale)
		}
	}
	// Each case runs on an app of its own, as a script runs its cases
	// one after the other on one: a case that breaks its app leaves the
	// others theirs.
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			known := slices.Contains(failing, c.name)
			step, err := compatCaseRun(t, fsys, env, c, logs, unrun)
			switch {
			case err != nil && !known:
				t.Errorf("%s: %v", step, err)
			case err == nil && known:
				t.Errorf("it passes: take it off compatFailing")
			case err != nil:
				t.Skipf("left to port: %s: %v", step, firstLine(err.Error()))
			}
		})
	}
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

// compatCaseRun opens the domain's app and runs case c's steps, its
// tap first, returning the step that failed and why.
func compatCaseRun(t *testing.T, fsys fs.FS, env core.Environment, c compatCase, logs *compatLog, unrun map[string]int) (step string, err error) {
	defer func() {
		if v := recover(); v != nil {
			f, ok := v.(compatFailure)
			if !ok {
				panic(v)
			}
			err = fmt.Errorf("%s", f.msg)
		}
	}()
	step = "open"
	app := nsgotest.Start(compatTB{t}, compiler.App(fsys, "App.tsx"), nsgotest.Environment(env))
	// A dialog is answered as a user backing out does: its cancel button,
	// else dismissed.
	app.OnAlert = func(a core.AlertConfig) int {
		for i, b := range a.Actions {
			if b.Role == core.AlertCancel {
				return i
			}
		}
		return -1
	}
	for _, step = range c.steps {
		f := strings.Fields(step)
		if slices.Contains(compatUnrun, f[0]) {
			unrun[f[0]]++
			continue
		}
		if err := compatStep(app, step, logs); err != nil {
			return step, err
		}
	}
	return "", nil
}

// compatSelector is a script's selector as nsgotest reads it, and how
// many views out (parent:) and before (below:) the view is from what it
// names.
func compatSelector(s string) (sel string, up, below int) {
	for {
		switch {
		case strings.HasPrefix(s, "parent:"):
			s, up = s[len("parent:"):], up+1
			continue
		case strings.HasPrefix(s, "below:"):
			s, below = s[len("below:"):], below+1
			continue
		case strings.HasPrefix(s, "scroll:"):
			s = s[len("scroll:"):]
			continue
		}
		break
	}
	switch {
	case strings.HasPrefix(s, "text:") || strings.HasPrefix(s, "span:"):
		text := s[len("text:"):]
		if !strings.HasPrefix(text, `"`) {
			text = `"` + text + `"`
		}
		return "text:" + text, up, below
	case strings.HasPrefix(s, "label:"):
		return "label:" + strings.Trim(s[len("label:"):], `"`), up, below
	}
	return s, up, below
}

// compatView is the view selector names in app.
func compatView(app *nsgotest.App, selector string) *nsgotest.View {
	sel, up, below := compatSelector(selector)
	v := app.Find(sel)
	for range up {
		if v.Parent == nil {
			panic(compatFailure{selector + " has no view around it"})
		}
		v = v.Parent
	}
	for range below {
		p := v.Parent
		i := slices.Index(p.Children, v)
		if p == nil || i < 1 {
			panic(compatFailure{selector + " has no view below it"})
		}
		v = p.Children[i-1]
	}
	return v
}

// compatWait settles until cond holds, moving the app's time a frame at a
// time for at most 3 seconds of it, as NSH's compat runner waits.
func compatWait(app *nsgotest.App, cond func() bool) bool {
	for range 180 {
		app.Settle()
		if cond() {
			return true
		}
		app.Advance(16667 * time.Microsecond)
		time.Sleep(100 * time.Microsecond)
	}
	return cond()
}

// compatFields splits a step at its spaces but those in quotes.
func compatFields(step string) []string {
	var out []string
	var b strings.Builder
	quoted := false
	for _, r := range step {
		switch {
		case r == '"':
			quoted = !quoted
			b.WriteRune(r)
		case r == ' ' && !quoted:
			if b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}

// compatText is a step's text argument: what follows its first n fields.
func compatText(step string, n int) string {
	rest := step
	for range n {
		_, rest, _ = strings.Cut(strings.TrimSpace(rest), " ")
	}
	return strings.TrimSpace(rest)
}

func compatStep(app *nsgotest.App, step string, logs *compatLog) error {
	f := compatFields(step)
	// The selector of a step whose last n fields are its arguments.
	selector := func(n int) string { return strings.Join(f[1:len(f)-n], " ") }
	switch f[0] {
	case "expect":
		text := compatText(step, 1)
		if !compatWait(app, func() bool { return app.Exists(`text:"` + text + `"`) }) {
			return fmt.Errorf("no text %q on screen: %q", text, app.Texts())
		}
	case "absent":
		text := compatText(step, 1)
		if !compatWait(app, func() bool { return !app.Exists(`text:"` + text + `"`) }) {
			return fmt.Errorf("%q still shows", text)
		}
	case "tap":
		target := compatText(step, 1)
		sel, up, below := compatSelector(target)
		if up > 0 || below > 0 {
			v := compatView(app, target)
			app.Press(fmt.Sprintf("view:%d", v.ID))
		} else {
			app.Press(sel)
		}
	case "wait":
		ms, _ := strconv.Atoi(f[1])
		app.Advance(time.Duration(ms) * time.Millisecond)
	case "viewport":
		w, _ := strconv.ParseFloat(f[1], 32)
		h, _ := strconv.ParseFloat(f[2], 32)
		app.Resize(core.Points(w), core.Points(h))
	case "frame":
		v := compatView(app, selector(4))
		var want [4]float64
		for i := range want {
			want[i], _ = strconv.ParseFloat(f[len(f)-4+i], 64)
		}
		b := v.Frame
		got := [4]float64{float64(b.Origin.X), float64(b.Origin.Y), float64(b.Size.Width), float64(b.Size.Height)}
		for i := range want {
			if math.Abs(got[i]-want[i]) > 0.5 {
				return fmt.Errorf("the frame is %v", got)
			}
		}
	case "scroll":
		x, _ := strconv.ParseFloat(f[len(f)-2], 32)
		y, _ := strconv.ParseFloat(f[len(f)-1], 32)
		app.ScrollTo(fmt.Sprintf("view:%d", compatView(app, selector(2)).ID), core.Point{X: core.Points(x), Y: core.Points(y)})
	case "type":
		text := strings.TrimSpace(step[strings.Index(step, f[1])+len(f[1]):])
		app.Type(fmt.Sprintf("view:%d", compatView(app, f[1]).ID), text)
	case "key":
		app.KeyPress(fmt.Sprintf("view:%d", compatView(app, f[1]).ID), f[2])
	case "listens":
		v := compatView(app, selector(1))
		kind, ok := compatEvents[f[len(f)-1]]
		if !ok {
			return fmt.Errorf("no event %s known to the runner", f[2])
		}
		if !v.Events.Has(kind) {
			return fmt.Errorf("the platform hears no %s", f[2])
		}
	case "log":
		text := compatText(step, 1)
		if !compatWait(app, func() bool { return logs.has(text) }) {
			return fmt.Errorf("no log line %q", text)
		}
	case "platform", "fontscale":
		// Read before the app starts.
	case "pixelratio":
		return fmt.Errorf("the runner cannot move the app to another screen yet")
	default:
		return fmt.Errorf("unknown step")
	}
	return nil
}

// compatEvents are the event kinds of NSH's handler names.
var compatEvents = map[string]driver.EventKind{
	"onPress": driver.EventPress, "onLongPress": driver.EventLongPress, "onPressDown": driver.EventPressDown,
	"onPressUp": driver.EventPressUp, "onHoverIn": driver.EventHoverIn, "onHoverOut": driver.EventHoverOut,
	"onHoverMove": driver.EventHoverMove, "onScroll": driver.EventScroll,
	"onContextMenu": driver.EventContextMenu, "onFocus": driver.EventFocus, "onBlur": driver.EventBlur,
	"onMouseMove": driver.EventMouseMove,
}

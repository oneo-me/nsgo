package tsx_test

import (
	"fmt"
	"io/fs"
	"log"
	"math"
	"os"
	"path"
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
// subtest per case, each step read as nsh-compat's apps.rs and values.rs
// read it (nshcompat_values_test.go, nshcompat_events_test.go).

// compatFailing are the cases a step of fails on NSGo, by domain and
// case: those of the apps that import the standard modules NSGo leaves
// out (docs/roadmap.md, "The compat cases"), the modules domain's and
// the macOS app, which imports console. A case that passes is taken off;
// a case a script opens twice is named the second time as its subtest
// is (events-pointer#01).
var compatFailing = map[string][]string{
	"macos":   {"desktop-props", "mod-standard-system", "mod-standard-files", "base-dock"},
	"modules": {"mod-standard-access", "mod-accessibility", "mod-android", "mod-clipboard", "mod-picker", "mod-desktop", "mod-device", "mod-host-channel", "mod-ios", "mod-keymap", "mod-mobile", "mod-storage", "mod-host"},
}

// compatDepartures are the steps of a case, by domain/case, that check
// what NSGo does otherwise on purpose, with the step that checks the
// same part on NSGo, or none where no selector reaches it: NSH puts what
// a render prop or a node prop renders in views of its own (resizable,
// plot), stacks its drawer's layers deeper (dock), centers a plot's hover
// dots by transform where NSGo's center in an anchor of no size, and
// gives a resize handle a band 9 points across where NSGo's is 3
// (docs/architecture/components.md).
var compatDepartures = map[string]map[string]string{
	// NSH's Android clipboard carries text; NSGo's holds images.
	"android/mod-clipboard-android": {
		"expect Write: rejected ERR_FEATURE_UNAVAILABLE_ON_PLATFORM": "expect Write: ok undefined",
		"expect Read: rejected ERR_FEATURE_UNAVAILABLE_ON_PLATFORM":  "expect Read: ok [object Object]",
		"expect Has: ok false": "expect Has: ok true",
	},
	"base/base-number-input": {
		"frame parent:parent:text:Less 0 0 30 30":                      "frame parent:text:Less 0 0 30 30",
		"style parent:parent:text:Less backgroundColor #ff0000ff":      "style parent:text:Less backgroundColor #ff0000ff",
		"frame parent:parent:text:More 130 0 30 30":                    "frame parent:text:More 130 0 30 30",
		"style parent:parent:text:More backgroundColor #00ff00ff":      "style parent:text:More backgroundColor #00ff00ff",
		"frame id:weight-suffix 0 0 30 10":                             "frame id:weight-suffix 270 0 30 10",
		"frame parent:parent:text:Up 0 0 30 15":                        "frame parent:text:Up 0 0 30 15",
		"frame parent:parent:text:Down 0 15 30 15":                     "frame parent:text:Down 0 15 30 15",
		"style parent:parent:parent:text:Up backgroundColor #0000ffff": "style parent:parent:text:Up backgroundColor #0000ffff",
		`frame parent:parent:text:"Locked less" 0 0 20 30`:             `frame parent:text:"Locked less" 0 0 20 30`,
		`frame parent:parent:text:"Locked more" 120 0 20 30`:           `frame parent:text:"Locked more" 120 0 20 30`,
	},
	// A multiline Input's bar needs its text's scroll offset, which no
	// backend reports yet (docs/roadmap.md, "Bars in multiline inputs").
	"base/base-scrollbar": {
		"frame label:Notes 290 0 10 100": "",
	},
	"base/base-resizable": {
		"frame label:Resize 146 0 9 100": "frame label:Resize 149 0 3 100",
		"frame label:Resize 176 0 9 100": "frame label:Resize 179 0 3 100",
		`style parent:parent:parent:text:"Grip 0 horizontal idle" backgroundColor #ff0000ff`: `style parent:text:"Grip 0 horizontal idle" backgroundColor #ff0000ff`,
		`style parent:parent:parent:text:"Grip 0 horizontal idle" opacity 0.5`:               `style parent:text:"Grip 0 horizontal idle" opacity 0.5`,
	},
	"base/base-plot": {
		"frame below:parent:parent:parent:id:tip 0 0 10 10":                 "",
		"frame below:below:parent:parent:parent:id:tip 0 0 12 12":           "",
		"frame below:below:below:parent:parent:parent:id:tip 0 0 8 8":       "",
		"frame below:below:below:below:parent:parent:parent:id:tip 0 0 4 6": "",
		"frame parent:parent:id:tip 0 6 40 16":                              "frame parent:id:tip 0 6 40 16",
		"frame parent:parent:id:tip 6 -22 40 16":                            "frame parent:id:tip 6 -22 40 16",
		"frame parent:parent:id:tip 6 6 40 16":                              "frame parent:id:tip 6 6 40 16",
	},
	"base/base-dock": {
		"style parent:parent:parent:parent:parent:parent:parent:parent:id:panel-files backgroundColor #ffffffff":       "style parent:parent:parent:parent:parent:id:panel-files backgroundColor #ffffffff",
		"style below:parent:parent:parent:parent:parent:parent:parent:parent:id:panel-files backgroundColor #00000066": "style below:parent:parent:parent:parent:parent:id:panel-files backgroundColor #00000066",
		"tap below:parent:parent:parent:parent:parent:parent:parent:parent:id:panel-files":                             "tap below:parent:parent:parent:parent:parent:id:panel-files",
	},
}

// TestNSHCompat runs NSH's compat cases headless.
func TestNSHCompat(t *testing.T) {
	root := nshDir(t, "tests", "compat", "cases")
	domains, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	logs := &compatLog{}
	log.SetOutput(logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	for _, d := range domains {
		if !d.IsDir() {
			continue
		}
		t.Run(d.Name(), func(t *testing.T) {
			compatRun(t, filepath.Join(root, d.Name()), compatFailing[d.Name()], logs)
		})
	}
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

func compatRun(t *testing.T, dir string, failing []string, logs *compatLog) {
	fsys := nshApp(t, dir)
	compatFS = fsys
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
			// A case a script opens twice is named the second time as
			// its subtest is: events-pointer#01.
			known := slices.Contains(failing, path.Base(t.Name()))
			step, err := compatCaseRun(t, fsys, env, filepath.Base(dir), c, logs)
			switch {
			case err != nil && !known:
				t.Errorf("%s: %v", step, err)
			case err == nil && known:
				t.Errorf("it passes: take it off compatFailing")
			case err != nil:
				msg := firstLine(err.Error())
				if os.Getenv("NSGO_COMPAT_ALL") != "" {
					msg = err.Error()
				}
				t.Skipf("left to port: %s: %v", step, msg)
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
func compatCaseRun(t *testing.T, fsys fs.FS, env core.Environment, domain string, c compatCase, logs *compatLog) (step string, err error) {
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
	compatEnv = env
	app := nsgotest.Start(compatTB{t}, compiler.App(fsys, "App.tsx"), nsgotest.Environment(env),
		nsgotest.TextEngine(nshTextEngine{}), nsgotest.NoCaretMoves(), nsgotest.ManualFrames(),
		nsgotest.PendingAlerts(), nsgotest.Offscreen())
	// NSGO_COMPAT_ALL=1 goes on past the expect steps that fail, to list
	// them all.
	var missed []string
	defer func() {
		if len(missed) > 0 && err == nil {
			step, err = "expect", fmt.Errorf("missed:\n%s\nlog:\n%s", strings.Join(missed, "\n"), strings.Join(logs.lines, "\n"))
		}
	}()
	for _, step = range c.steps {
		if alt, ok := compatDepartures[domain+"/"+c.name][step]; ok {
			if alt == "" {
				continue
			}
			step = alt
		}
		if err := compatStep(app, step, logs); err != nil {
			if os.Getenv("NSGO_COMPAT_ALL") != "" && strings.HasPrefix(step, "expect ") {
				missed = append(missed, step)
				continue
			}
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
	// What steps read of a view the platform has whether it shows or not.
	// Of views that share a selector, the first the platform holds.
	var v *nsgotest.View
	if shown := app.Query(sel); len(shown) == 1 {
		v = shown[0]
	} else if all := app.QueryMounted(sel); len(all) > 0 {
		v = all[0]
	} else {
		v = app.FindMounted(sel)
	}
	if strings.HasPrefix(selector, "scroll:") {
		for v != nil && v.Kind != driver.KindScroll {
			v = v.Parent
		}
		if v == nil {
			panic(compatFailure{selector + " is in no scroll view"})
		}
	}
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

// compatField is the text field v is or holds first, as an editor's
// label names the field inside it.
func compatField(v *nsgotest.View) *nsgotest.View {
	if v.Kind == driver.KindInput {
		return v
	}
	for _, c := range v.Children {
		if f := compatField(c); f.Kind == driver.KindInput {
			return f
		}
	}
	return v
}

// compatShown reports whether a view selector matches shows: mounted
// and not hidden.
func compatShown(app *nsgotest.App, selector string) bool {
	for _, v := range app.QueryMounted(selector) {
		// NSGo's core hides the screens a stack covers, which NSH leaves
		// to its platforms: headless, they show.
		hidden := false
		for p := v; p != nil; p = p.Parent {
			hidden = hidden || p.Style.Hidden && p.Screen == (driver.ScreenConfig{})
		}
		if !hidden {
			return true
		}
	}
	return false
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

// compatRest is the value of a step whose fields are its kind, a
// selector and a key: what follows the key.
func compatRest(step string, f []string) string {
	at := len(f[0]) + 1 + len(f[1]) + 1 + len(f[2])
	return strings.TrimSpace(step[min(at, len(step)):])
}

// compatText is a step's text argument: what follows its first n fields.
func compatText(step string, n int) string {
	rest := step
	for range n {
		_, rest, _ = strings.Cut(strings.TrimSpace(rest), " ")
	}
	return strings.TrimSpace(rest)
}

// compatEnv is the environment the case's app runs in, which keyboard
// and pixelratio steps change.
var compatEnv core.Environment

// compatStates are the interaction states state steps reported, by view.
var compatStates = map[driver.NodeID]map[string]bool{}

// compatState reports the interaction states of the view selector names
// as a platform does, its press, hover and focus: what was on and is not
// ends. It fails on a state the core did not ask the platform to track.
func compatState(app *nsgotest.App, selector string, states []string) error {
	v := compatView(app, selector)
	want := map[string]bool{}
	for _, s := range states {
		if s != "none" {
			want[s] = true
		}
	}
	had := compatStates[v.ID]
	kinds := map[string][2]driver.EventKind{"pressed": {driver.EventPressDown, driver.EventPressUp},
		"hovered": {driver.EventMouseEnter, driver.EventMouseLeave}, "focused": {driver.EventFocus, driver.EventBlur}}
	for _, s := range []string{"focused", "hovered", "pressed"} {
		k := kinds[s]
		tracked := v.Events.Has(k[0]) || s == "focused" && v.Kind == driver.KindInput
		switch {
		case want[s] && !had[s]:
			if !tracked {
				return fmt.Errorf("the platform does not track %s", s)
			}
			app.Report(nsgotest.Event{Kind: k[0], ID: v.ID})
		case !want[s] && had[s]:
			app.Report(nsgotest.Event{Kind: k[1], ID: v.ID})
		}
	}
	compatStates[v.ID] = want
	return nil
}

// finger is the finger a touch step put down.
var finger *nsgotest.Finger

// shadowColors are the shadow colors a case checked, by view.
var shadowColors = map[driver.NodeID]string{}

func compatStep(app *nsgotest.App, step string, logs *compatLog) error {
	f := compatFields(step)
	// The selector of a step whose last n fields are its arguments.
	selector := func(n int) string { return strings.Join(f[1:len(f)-n], " ") }
	switch f[0] {
	case "expect":
		text := compatText(step, 1)
		// A text counts wherever it is mounted, below the window too.
		if !compatWait(app, func() bool { return compatShown(app, `text:"`+text+`"`) }) {
			return fmt.Errorf("no text %q on screen: %q", text, app.Texts())
		}
	case "absent":
		text := compatText(step, 1)
		if !compatWait(app, func() bool { return !compatShown(app, `text:"`+text+`"`) }) {
			return fmt.Errorf("%q still shows", text)
		}
	case "tap":
		target := compatText(step, 1)
		if chars, ok := strings.CutPrefix(target, "span:"); ok {
			// A press on the link or pressable span where chars start.
			chars = strings.Trim(chars, `"`)
			for _, v := range app.QueryMounted("text:" + chars) {
				if at := strings.Index(v.Text, chars); at >= 0 && v.Events.Has(driver.EventPress) {
					app.Report(nsgotest.Event{Kind: driver.EventPress, ID: v.ID, Index: at, Count: 1})
					return nil
				}
			}
			return fmt.Errorf("no pressable text shows %q", chars)
		}
		if v := compatView(app, target); v.Props.Disabled {
			// A disabled view takes no press: it goes to the pressable
			// view around it.
			for p := v.Parent; p != nil; p = p.Parent {
				if p.Events.Has(driver.EventPress) && !p.Props.Disabled {
					app.Press(fmt.Sprintf("view:%d", p.ID))
					return nil
				}
			}
		}
		// NSH's tap reports the press alone, to the view or the nearest
		// one around it that takes presses.
		v := compatView(app, target)
		disabled := false
		for p := v; p != nil; p = p.Parent {
			if p.Events.Has(driver.EventPress) {
				if p.Props.Disabled {
					// It takes no press: the one around it may.
					disabled = true
					continue
				}
				app.Report(nsgotest.Event{Kind: driver.EventPress, ID: p.ID, Count: 1})
				return nil
			}
		}
		if disabled {
			return nil
		}
		app.Press(fmt.Sprintf("view:%d", v.ID))
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
		// NSGo holds a view inside a text in a view of the span's, which
		// the text places: NSH's frame is the pair's, in the text.
		if w := v.Parent; w != nil && w.Parent != nil && w.Parent.Kind == driver.KindText && len(w.Children) == 1 && w.Props.TestID == "" {
			b.Origin.X += w.Frame.Origin.X
			b.Origin.Y += w.Frame.Origin.Y
		}
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
		app.Type(fmt.Sprintf("view:%d", compatField(compatView(app, f[1])).ID), text)
	case "key":
		app.KeyPress(fmt.Sprintf("view:%d", compatField(compatView(app, f[1])).ID), f[2])
	case "listens":
		v := compatView(app, selector(1))
		if h := f[len(f)-1]; h == "onOpen" || h == "onClose" || h == "onCancel" {
			// A modal hears its platform's reports and the back whoever
			// listens.
			for m := v; m != nil; m = m.Parent {
				if m.Kind == driver.KindModal {
					return nil
				}
			}
		}
		kind, ok := compatEvents[f[len(f)-1]]
		if !ok {
			kind, ok = compatHandlers[f[len(f)-1]]
		}
		if !ok {
			return fmt.Errorf("no event %s known to the runner", f[2])
		}
		// Escape and back reach the core whoever listens.
		if !v.Events.Has(kind) && kind != driver.EventBack {
			return fmt.Errorf("the platform hears no %s", f[2])
		}
	case "style":
		t := compatTargetOf(app, f[1])
		key, want := f[2], compatRest(step, f)
		if key == "shadowColor" {
			shadowColors[t.view.ID] = want
		}
		t.shadowColor = shadowColors[t.view.ID]
		if ok, got := compatStyleIs(t, key, want); !ok {
			if os.Getenv("NSGO_COMPAT_DUMP") != "" {
				return fmt.Errorf("%s is %q: %+v %+v", key, got, t.view.Style, t.view.Props)
			}
			return fmt.Errorf("%s is %q", key, got)
		}
	case "nostyle":
		t := compatTargetOf(app, f[1])
		if got, set := compatStyle(t, f[len(f)-1]); set {
			return fmt.Errorf("%s is set: %q", f[len(f)-1], got)
		}
	case "prop":
		t := compatTargetOf(app, f[1])
		key, want := f[2], compatRest(step, f)
		if ok, got := compatPropIs(t, key, want); !ok {
			if got == compatHidden {
				return fmt.Errorf("NSGo's platforms do not receive %s", key)
			}
			return fmt.Errorf("%s is %q", key, got)
		}
	case "noprop":
		t := compatTargetOf(app, f[1])
		if got, set := compatProp(t, f[2]); set || got == compatHidden {
			if got == compatHidden {
				return fmt.Errorf("NSGo's platforms do not receive %s", f[2])
			}
			return fmt.Errorf("%s is set: %q", f[2], got)
		}
	case "event":
		return compatEvent(app, f[1], f[2], f[3:])
	case "touch":
		at := func(i int) core.Point {
			x, _ := strconv.ParseFloat(f[i], 32)
			y, _ := strconv.ParseFloat(f[i+1], 32)
			return core.Point{X: core.Points(x), Y: core.Points(y)}
		}
		switch f[1] {
		case "start":
			sel, _, _ := compatSelector(f[2])
			if sel != f[2] || strings.Contains(f[2], ":") && !strings.HasPrefix(f[2], "id:") {
				sel = fmt.Sprintf("view:%d", compatView(app, f[2]).ID)
			}
			// A finger lands on the next display frame, 16 ms in NSH's
			// runner, whose touches count time from the app's start.
			app.Advance(16 * time.Millisecond)
			finger = app.Touch(sel, at(3))
			return nil
		}
		if finger == nil {
			return fmt.Errorf("no finger is down")
		}
		switch f[1] {
		case "move":
			finger.Move(at(2))
		case "end":
			if len(f) > 2 {
				finger.LiftAt(at(2))
			} else {
				finger.Lift()
			}
			finger = nil
		case "cancel":
			finger.Cancel()
			finger = nil
		}
	case "log":
		text := compatText(step, 1)
		if !compatWait(app, func() bool { return logs.has(text) }) {
			return fmt.Errorf("no log line %q", text)
		}
	case "platform", "fontscale":
		// Read before the app starts.
	case "pixelratio":
		r, _ := strconv.ParseFloat(f[1], 32)
		compatEnv.PixelRatio = float32(r)
		app.SetEnvironment(compatEnv)
	case "keyboard":
		h, _ := strconv.ParseFloat(f[1], 32)
		ms, _ := strconv.Atoi(f[2])
		compatEnv.KeyboardHeight, compatEnv.KeyboardDuration = core.Points(h), time.Duration(ms)*time.Millisecond
		app.SetEnvironment(compatEnv)
	case "origin":
		v := compatView(app, f[1])
		x, _ := strconv.ParseFloat(f[2], 64)
		y, _ := strconv.ParseFloat(f[3], 64)
		// In the content: scroll offsets do not move it.
		var at core.Point
		for c := v; c != nil; c = c.Parent {
			at.X, at.Y = at.X+c.Frame.Origin.X, at.Y+c.Frame.Origin.Y
		}
		if math.Abs(float64(at.X)-x) > 0.5 || math.Abs(float64(at.Y)-y) > 0.5 {
			return fmt.Errorf("it is at %v", at)
		}
	case "statusbar":
		sb := app.StatusBar
		got := fmt.Sprintf("foreground=%s hidden=%t animated=%t",
			[...]string{"auto", "auto", "dark", "light"}[sb.Style], sb.Hidden, sb.Animated)
		if want := compatText(step, 1); got != want {
			return fmt.Errorf("the status bar is %s", got)
		}
	case "state":
		return compatState(app, f[1], f[2:])
	case "drawing", "nodrawing":
		// What a canvas told the platform to draw, an operation as NSH's
		// runner writes it; "..." ends one the step gives the start of.
		ops := compatDrawing(compatView(app, f[1]))
		want := compatText(step, 2)
		drew := slices.ContainsFunc(ops, func(op string) bool {
			if start, ok := strings.CutSuffix(want, "..."); ok {
				return strings.HasPrefix(op, start)
			}
			return op == want
		})
		if drew != (f[0] == "drawing") {
			return fmt.Errorf("it drew %q", ops)
		}
	default:
		return fmt.Errorf("unknown step")
	}
	return nil
}

// compatEvents are the event kinds of NSH's handler names.
var compatEvents = map[string]driver.EventKind{
	"onPress": driver.EventPress, "onLongPress": driver.EventLongPress, "onPressDown": driver.EventPressDown,
	"onPressUp": driver.EventPressUp, "onMouseEnter": driver.EventMouseEnter, "onMouseLeave": driver.EventMouseLeave,
	"onScroll":      driver.EventScroll,
	"onContextMenu": driver.EventContextMenu, "onFocus": driver.EventFocus, "onBlur": driver.EventBlur,
	"onMouseMove": driver.EventMouseMove, "onPressOutside": driver.EventPressOutside,
}

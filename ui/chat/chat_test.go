package chat

import (
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
	"github.com/oneo-me/nsgo/ui"
)

// TestMain hands the main thread to the platform under nsgo test -target.
func TestMain(m *testing.M) { nsgotest.Main(m) }

var (
	desktopEnv = core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop, Locale: "en-US"}
	phoneEnv   = core.Environment{OS: core.OSIOS, Idiom: core.IdiomPhone, Locale: "en-US"}
)

// each runs fn in the four presentations every component is tested in.
func each(t *testing.T, fn func(t *testing.T, e core.Environment)) {
	darkDesktop, darkPhone := desktopEnv, phoneEnv
	darkDesktop.Dark, darkPhone.Dark = true, true
	for name, e := range map[string]core.Environment{"desktop": desktopEnv, "desktop-dark": darkDesktop, "phone": phoneEnv, "phone-dark": darkPhone} {
		t.Run(name, func(t *testing.T) { fn(t, e) })
	}
}

// start runs content under ui's theme for e, as an app sets it, with opts.
func start(t *testing.T, e core.Environment, content func(cx *core.Scope) core.Node, opts ...nsgotest.Option) *nsgotest.App {
	t.Helper()
	size := nsgotest.Size(1000, 760)
	if e.Idiom != core.IdiomDesktop {
		size = nsgotest.Size(390, 844)
	}
	return nsgotest.Start(t, func(cx *core.Scope) core.Node {
		cx.SetTheme(ui.Theme())
		colors := ui.ColorsOf(cx)
		return core.View{Style: core.Style{FlexGrow: 1, Padding: core.All(16), Gap: 12}, StyleFunc: ui.Filled(colors.Background),
			Children: []core.Node{core.Func(content)}}
	}, append([]nsgotest.Option{size, nsgotest.Environment(e)}, opts...)...)
}

// chatStage is a transcript of one turn and a composer the test drives.
type chatStage struct {
	status *core.Signal[Status]
	final  *core.Signal[bool]
	draft  *core.Signal[string]
	sends  int
	stops  int
}

func startChat(t *testing.T, e core.Environment) (*nsgotest.App, *chatStage) {
	t.Helper()
	s := &chatStage{}
	app := start(t, e, func(cx *core.Scope) core.Node {
		s.status, s.final, s.draft = core.NewSignal(cx, Running), core.NewSignal(cx, false), core.NewSignal(cx, "")
		state := NewConversationState(cx)
		return FloatingConversation{State: state, JumpLabel: "Latest",
			Children: []core.Node{ui.Button{Text: "Complete", OnPress: func() { s.status.Set(Completed) }}, ui.Button{Text: "Finish", OnPress: func() { s.final.Set(true) }}, Turn{Status: s.status.Get, Conversation: state,
				Summary: func() string { return "Worked" }, Working: func() string { return "Working" },
				Prompt: func(*core.Scope) core.Node { return core.Text{Content: "Prompt"} },
				Steps: func() []Step {
					return []Step{{ID: "work", Title: "Ran it", Activities: []Activity{{ID: "read", Action: "Read", Detail: "main.go", Status: Completed,
						Content: func(*core.Scope) core.Node { return core.Text{Content: "Output"} }}}}}
				},
				HasFinal: s.final.Get, Final: func(*core.Scope) core.Node { return core.Text{Content: "Answer"} }}},
			Composer: []core.Node{Composer{Draft: s.draft.Get, OnDraftChange: s.draft.Set, Label: "Message", SendLabel: "Send", StopLabel: "Stop",
				Running: func() bool { return s.status.Get() == Running }, Glass: true, State: state,
				OnSend: func() { s.sends++ }, OnStop: func() { s.stops++ }}}}
	})
	return app, s
}

// TestTurnFolds checks a live group, then the fold a completed turn
// takes with its final response, and that a choice outlives updates.
func TestTurnFolds(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, _ := startChat(t, e)
		app.Find("text:Read")
		app.Find("text:Working")
		app.Press("role:button Complete")
		// A settled group closes, but the turn stays open for its answer.
		if app.Exists("text:Read") {
			t.Fatal("a settled group stayed open")
		}
		app.Find("text:Ran it")
		app.Press("role:button Finish")
		app.Find("text:Answer")
		if app.Exists("text:Ran it") {
			t.Fatal("a completed turn kept its steps")
		}
		app.Press("role:button Worked")
		app.Press("role:button Ran it")
		app.Press("role:button Read main.go")
		app.Find("text:Output")
	})
}

// TestComposer checks Send follows the draft and that a draft takes
// Stop's place while running.
func TestComposer(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, s := startChat(t, e)
		app.Press("role:button Stop")
		if s.stops != 1 {
			t.Fatalf("stops = %d", s.stops)
		}
		app.Type("id:composer", "Hello")
		if app.Exists("role:button Stop") {
			t.Fatal("Stop shows beside a draft")
		}
		app.Press("role:button Send")
		if s.sends != 1 {
			t.Fatalf("sends = %d", s.sends)
		}
	})
}

// TestPickerChooses checks the options a query leaves and choosing one.
func TestPickerChooses(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app := start(t, e, func(cx *core.Scope) core.Node {
			v := core.NewSignal(cx, "a")
			return Picker{Label: v.Get, Selected: v.Get, OnChange: v.Set, SearchLabel: "Search", EmptyText: "None", Searchable: 1, TestID: "picker",
				Options: []Option{{ID: "a", Label: "Alpha"}, {ID: "b", Label: "Beta", Group: "Second"}}}
		})
		app.Press("id:picker")
		app.Press("role:button Beta")
		app.Find("text:b")
		if app.Exists("role:button Alpha") {
			t.Fatal("choosing kept the popover open")
		}
	})
}

// TestEffortStaysOpen checks a press on the bar picks its level and
// keeps the panel.
func TestEffortStaysOpen(t *testing.T) {
	chosen := ""
	app := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		v := core.NewSignal(cx, "off")
		return EffortPicker{Label: "Effort", Selected: v.Get, OnChange: func(id string) { chosen = id; v.Set(id) }, Off: true, TestID: "effort",
			Levels: []Option{{ID: "off", Label: "Off"}, {ID: "low", Label: "Low"}, {ID: "max", Label: "Top"}}}
	})
	app.Press("id:effort")
	app.Press("id:effort-bar")
	app.Advance(time.Second)
	if chosen != "low" || !app.Exists("id:effort-bar") {
		t.Fatalf("chose %q, want the middle level with the panel open", chosen)
	}
}

// TestPickerLeavesNoFill checks the trigger returns to rest once the
// pointer has left while its popover was open.
func TestPickerLeavesNoFill(t *testing.T) {
	app := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		v := core.NewSignal(cx, "a")
		return Picker{Label: v.Get, Selected: v.Get, OnChange: v.Set, TestID: "picker",
			Options: []Option{{ID: "a", Label: "Alpha"}, {ID: "b", Label: "Beta"}}}
	})
	rest := app.Find("id:picker").Style.Background
	app.Hover("id:picker")
	app.Press("id:picker")
	app.HoverOut()
	app.Press("role:button Beta")
	if got := app.Find("id:picker").Style.Background; got != rest {
		t.Fatalf("background after closing = %x, want %x", got, rest)
	}
}

// TestFloatingMargin lets a footer stand in the safe area, and the keyboard
// replace the safe area.
func TestFloatingMargin(t *testing.T) {
	e := phoneEnv
	e.SafeArea = core.Edges{Bottom: 34}
	var withFooter, without core.Points
	start(t, e, func(cx *core.Scope) core.Node {
		withFooter, without = FloatingMargin(cx, FooterHeight), FloatingMargin(cx, 0)
		return core.View{}
	})
	if withFooter != 14 || without != 42 {
		t.Fatalf("margins %v and %v", withFooter, without)
	}
}

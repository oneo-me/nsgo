package ui

import (
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

// chatStage is a transcript of one turn and a composer the test drives.
type chatStage struct {
	status *core.Signal[ChatStatus]
	final  *core.Signal[bool]
	draft  *core.Signal[string]
	sends  int
	stops  int
}

func startChat(t *testing.T, e core.Environment) (*nsgotest.App, *chatStage) {
	t.Helper()
	s := &chatStage{}
	app, _ := start(t, e, func(cx *core.Scope) core.Node {
		s.status, s.final, s.draft = core.NewSignal(cx, ChatRunning), core.NewSignal(cx, false), core.NewSignal(cx, "")
		state := NewChatConversationState(cx)
		return ChatFloatingConversation{State: state, JumpLabel: "Latest",
			Children: []core.Node{Button{Text: "Complete", OnPress: func() { s.status.Set(ChatCompleted) }}, Button{Text: "Finish", OnPress: func() { s.final.Set(true) }}, ChatTurn{Status: s.status.Get, Conversation: state,
				Summary: func() string { return "Worked" }, Working: func() string { return "Working" },
				Prompt: func(*core.Scope) core.Node { return core.Text{Content: "Prompt"} },
				Steps: func() []ChatStep {
					return []ChatStep{{ID: "work", Title: "Ran it", Activities: []ChatActivity{{ID: "read", Action: "Read", Detail: "main.go", Status: ChatCompleted,
						Content: func(*core.Scope) core.Node { return core.Text{Content: "Output"} }}}}}
				},
				HasFinal: s.final.Get, Final: func(*core.Scope) core.Node { return core.Text{Content: "Answer"} }}},
			Composer: []core.Node{ChatComposer{Draft: s.draft.Get, OnDraftChange: s.draft.Set, Label: "Message", SendLabel: "Send", StopLabel: "Stop",
				Running: func() bool { return s.status.Get() == ChatRunning }, Glass: true, State: state,
				OnSend: func() { s.sends++ }, OnStop: func() { s.stops++ }}}}
	})
	return app, s
}

// TestChatTurnFolds checks a live group, then the fold a completed turn
// takes with its final response, and that a choice outlives updates.
func TestChatTurnFolds(t *testing.T) {
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

// TestChatComposer checks Send follows the draft and that a draft takes
// Stop's place while running.
func TestChatComposer(t *testing.T) {
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

// TestChatPickerChooses checks the options a query leaves and choosing one.
func TestChatPickerChooses(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			v := core.NewSignal(cx, "a")
			return ChatPicker{Label: v.Get, Selected: v.Get, OnChange: v.Set, SearchLabel: "Search", EmptyText: "None", Searchable: 1, TestID: "picker",
				Options: []ChatOption{{ID: "a", Label: "Alpha"}, {ID: "b", Label: "Beta", Group: "Second"}}}
		})
		app.Press("id:picker")
		app.Press("role:button Beta")
		app.Find("text:b")
		if app.Exists("role:button Alpha") {
			t.Fatal("choosing kept the popover open")
		}
	})
}

// TestChatEffortStaysOpen checks a press on the bar picks its level and
// keeps the panel.
func TestChatEffortStaysOpen(t *testing.T) {
	chosen := ""
	app, _ := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		v := core.NewSignal(cx, "off")
		return ChatEffortPicker{Label: "Effort", Selected: v.Get, OnChange: func(id string) { chosen = id; v.Set(id) }, Off: true, TestID: "effort",
			Levels: []ChatOption{{ID: "off", Label: "Off"}, {ID: "low", Label: "Low"}, {ID: "max", Label: "Top"}}}
	})
	app.Press("id:effort")
	app.Press("id:effort-bar")
	app.Advance(time.Second)
	if chosen != "low" || !app.Exists("id:effort-bar") {
		t.Fatalf("chose %q, want the middle level with the panel open", chosen)
	}
}

// TestChatPickerLeavesNoFill checks the trigger returns to rest once the
// pointer has left while its popover was open.
func TestChatPickerLeavesNoFill(t *testing.T) {
	app, _ := start(t, desktopEnv, func(cx *core.Scope) core.Node {
		v := core.NewSignal(cx, "a")
		return ChatPicker{Label: v.Get, Selected: v.Get, OnChange: v.Set, TestID: "picker",
			Options: []ChatOption{{ID: "a", Label: "Alpha"}, {ID: "b", Label: "Beta"}}}
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

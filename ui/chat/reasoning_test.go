package chat

import (
	"slices"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

// TestReasoningFoldsWhenItEnds streams reasoning, watches its duration, and
// finds it folded once it ends.
func TestReasoningFoldsWhenItEnds(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var text *core.Signal[string]
		var status *core.Signal[Status]
		var elapsed *core.Signal[time.Duration]
		app := start(t, e, func(cx *core.Scope) core.Node {
			text, status, elapsed = core.NewSignal(cx, ""), core.NewSignal(cx, Running), core.NewSignal(cx, time.Duration(0))
			return Reasoning{Text: text.Get, Status: status.Get, Elapsed: elapsed.Get, TestID: "reason"}
		})
		// Nothing to show until reasoning arrives.
		if app.Exists("id:reason") {
			t.Fatal("the block shows before a word of reasoning")
		}
		text.Set("Reading the tree.")
		app.WaitFor("id:reason", time.Second)
		app.WaitFor("text:Reading the tree.", time.Second)
		// A running block reads its cost again every second.
		elapsed.Set(3 * time.Second)
		app.Advance(1100 * time.Millisecond)
		app.WaitFor("text:/Thinking.+3 s/", 2*time.Second)
		// It folds itself when it ends.
		status.Set(Completed)
		app.WaitForGone("text:Reading the tree.", 2*time.Second)
		app.WaitFor("text:/Thought for 3 s/", 2*time.Second)
	})
}

// TestReasoningStaysOpenForTheReader opens a finished block, streams more
// into it, and finds the text still there.
func TestReasoningStaysOpenForTheReader(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var text *core.Signal[string]
		var status *core.Signal[Status]
		app := start(t, e, func(cx *core.Scope) core.Node {
			text, status = core.NewSignal(cx, "First thought."), core.NewSignal(cx, Completed)
			return Reasoning{Text: text.Get, Status: status.Get, Elapsed: func() time.Duration { return 2 * time.Second }, TestID: "reason"}
		})
		app.WaitFor("text:/Thought for 2 s/", time.Second)
		app.WaitForGone("text:First thought.", time.Second)
		app.Press("id:reason >> role:button")
		app.WaitFor("text:First thought.", time.Second)
		// More reasoning arrives while it is open, and a rebuild keeps it.
		text.Set("First thought. And a second.")
		app.WaitFor("text:/And a second/", time.Second)
		status.Set(Running)
		app.Advance(200 * time.Millisecond)
		status.Set(Completed)
		app.Advance(200 * time.Millisecond)
		app.WaitFor("text:/And a second/", time.Second)
	})
}

// TestReasoningHoldsOpenWhileReading keeps a block open for a reader who is
// still up there, though it ended.
func TestReasoningHoldsOpenWhileReading(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app := start(t, e, func(cx *core.Scope) core.Node {
			held := core.NewSignal(cx, true)
			return Reasoning{Text: func() string { return "Kept for the reader." }, Status: func() Status { return Completed },
				Elapsed: func() time.Duration { return time.Second }, HoldOpen: held.Get, TestID: "reason"}
		})
		app.WaitFor("text:Kept for the reader.", time.Second)
	})
}

// TestReasoningCountsFromTheFirstWord streams reasoning into a block that
// started empty, with a cost the app reads from a clock rather than a
// signal, and finds the duration counting.
func TestReasoningCountsFromTheFirstWord(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var text *core.Signal[string]
		app := start(t, e, func(cx *core.Scope) core.Node {
			text = core.NewSignal(cx, "")
			began := cx.Now()
			return Reasoning{Text: text.Get, Status: func() Status { return Running },
				Elapsed: func() time.Duration { return cx.Now().Sub(began) }, TestID: "reason"}
		})
		text.Set("Streaming.")
		app.WaitFor("id:reason", time.Second)
		app.Advance(3200 * time.Millisecond)
		app.WaitFor("text:/Thinking.+3 s/", 2*time.Second)
	})
}

// TestReasoningWithoutTime says no duration when the app gives none.
func TestReasoningWithoutTime(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var status *core.Signal[Status]
		app := start(t, e, func(cx *core.Scope) core.Node {
			status = core.NewSignal(cx, Running)
			return Reasoning{Text: func() string { return "Reading." }, Status: status.Get, TestID: "reason"}
		})
		app.WaitFor("text:Thinking…", time.Second)
		status.Set(Completed)
		app.WaitFor("text:Thoughts", time.Second)
	})
}

// TestTurnReasoning shows a turn's reasoning after its prompt, where it
// stays when the turn folds its steps.
func TestTurnReasoning(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var status *core.Signal[Status]
		app := start(t, e, func(cx *core.Scope) core.Node {
			status = core.NewSignal(cx, Running)
			return Turn{Status: status.Get, HasFinal: func() bool { return status.Get() == Completed },
				Summary: func() string { return "Worked" }, Working: func() string { return "Working" },
				Prompt: func(*core.Scope) core.Node { return core.Text{Content: "Prompt"} },
				Reasoning: func(*core.Scope) core.Node {
					return Reasoning{Text: func() string { return "Reading." }, Status: status.Get, TestID: "reason"}
				},
				Steps: func() []Step {
					return []Step{{ID: "work", Title: "Ran it", Activities: []Activity{{ID: "read", Action: "Read", Status: Completed}}}}
				},
				Final: func(*core.Scope) core.Node { return core.Text{Content: "Answer"} }, TestID: "turn"}
		})
		turn := app.Find("id:turn")
		index := func(selector string) int {
			v := app.Find(selector)
			for v.Parent != turn {
				v = v.Parent
			}
			return slices.Index(turn.Children, v)
		}
		if prompt, reason := index("text:Prompt"), index("id:reason"); reason != prompt+1 {
			t.Fatalf("the reasoning is the turn's view %d, the prompt %d", reason, prompt)
		}
		status.Set(Completed)
		app.WaitForGone("text:Ran it", time.Second)
		app.Find("role:button Thoughts")
	})
}

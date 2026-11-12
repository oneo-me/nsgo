package chat

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

// TestActionsCopy copies a message and shows the receipt for a moment.
func TestActionsCopy(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app := start(t, e, func(cx *core.Scope) core.Node {
			return Actions{Text: func() string { return "the message" }, TestID: "acts"}
		})
		app.Press("label:Copy")
		app.WaitFor("label:Copied", time.Second)
		// The receipt hides by itself.
		app.Advance(2 * time.Second)
		app.WaitFor("label:Copy", 2*time.Second)
		app.WaitForGone("label:Copied", time.Second)
	})
}

// TestActionsRetryOnlyOnTheLast fails a retry that is not allowed any more,
// and allows it again when the app says so.
func TestActionsRetryOnlyOnTheLast(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var last *core.Signal[bool]
		retries := 0
		app := start(t, e, func(cx *core.Scope) core.Node {
			last = core.NewSignal(cx, true)
			return Actions{Text: func() string { return "the reply" }, OnRetry: func() { retries++ }, CanRetry: last.Get, TestID: "acts"}
		})
		if !app.Enabled("label:Retry") {
			t.Fatal("the last reply cannot be retried")
		}
		last.Set(false)
		app.Advance(100 * time.Millisecond)
		if app.Enabled("label:Retry") {
			t.Fatal("a reply that is not the last can be retried")
		}
		last.Set(true)
		app.Advance(100 * time.Millisecond)
		app.Press("label:Retry")
		if retries != 1 {
			t.Fatalf("retried %d times, want 1", retries)
		}
	})
}

// TestActionsEditOnlyUnsuperseded shows an edit that its rules keep away
// from a message something replaced.
func TestActionsEditOnlyUnsuperseded(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var live *core.Signal[bool]
		app := start(t, e, func(cx *core.Scope) core.Node {
			live = core.NewSignal(cx, false)
			return Actions{OnEdit: func() {}, CanEdit: live.Get, TestID: "acts"}
		})
		// No text to copy, so no copy action.
		if app.Exists("label:Copy") {
			t.Fatal("an action with no text offers copy")
		}
		if app.Enabled("label:Edit") {
			t.Fatal("a superseded message is editable")
		}
		live.Set(true)
		app.Advance(100 * time.Millisecond)
		if !app.Enabled("label:Edit") {
			t.Fatal("a live message is not editable")
		}
	})
}

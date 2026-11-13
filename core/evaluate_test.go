package core_test

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// Evaluate's answers come back by script, through the page's channel,
// never to OnMessage; a page leaving ends those still waiting.
func TestEvaluateAnswersThroughThePagesChannel(t *testing.T) {
	ref := new(core.Ref)
	var messages, got []string
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.WebView{Ref: ref, TestID: "web", Source: core.WebSource{HTML: "<p>hi</p>"},
			OnMessage: func(m string) { messages = append(messages, m) },
			Style:     core.Style{Width: core.Pt(100), Height: core.Pt(100)}}
	})
	answer := func(json string, err error) {
		if err != nil {
			got = append(got, "error: "+err.Error())
			return
		}
		got = append(got, json)
	}
	ref.Evaluate("document.title", answer)
	ref.Evaluate("missing()", answer)
	app.Settle()
	app.AnswerScript("id:web", `"Hello"`, "")
	app.AnswerScript("id:web", "", "missing is not defined")
	app.PostMessage("id:web", "the page's own")
	want := []string{`"Hello"`, "error: missing is not defined"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("answers %q, want %q", got, want)
	}
	if len(messages) != 1 || messages[0] != "the page's own" {
		t.Errorf("OnMessage heard %q", messages)
	}
	got = nil
	ref.Evaluate("1", answer)
	app.Settle()
	app.Navigate("id:web", driver.WebNavigation{URL: "https://example.com/"})
	if len(got) != 1 || got[0][:6] != "error:" {
		t.Errorf("a page leaving answered %q", got)
	}
}

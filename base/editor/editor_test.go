package editor_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/base/editor"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

const sample = `package main

func main() {
	println("hi")
}
`

func TestEditorTypesAndEdits(t *testing.T) {
	var value string
	var requests []editor.CompletionRequest
	ref := &editor.Ref{}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		text := core.NewSignal(cx, sample)
		return editor.Editor{
			Ref: ref, TestID: "editor", Language: "go", Label: "Code",
			Value:      text.Get,
			OnChange:   func(s string) { value = s; text.Set(s) },
			Style:      core.Style{Width: core.Pt(400), Height: core.Pt(300)},
			Styles:     editor.Styles{Caret: core.Style{Background: core.RGB(0x000000)}},
			Completion: true,
			OnCompletionRequest: func(r editor.CompletionRequest) {
				requests = append(requests, r)
				ref.PresentCompletions(r.ID, []editor.CompletionItem{{Label: "println"}, {Label: "print"}, {Label: "panic"}}, false)
			},
		}
	})
	app.Find(`text:println("hi")`)
	app.Find("text:package main")
	ref.SetSelections([]editor.Selection{{Anchor: editor.Position{Line: 3, Column: 1}, Head: editor.Position{Line: 3, Column: 1}}})
	ref.Focus()
	app.Settle()
	app.Type("label:Code", "x")
	if !strings.Contains(value, "\tx"+`println("hi")`) {
		t.Fatalf("typed: %q", value)
	}
	app.Key("end")
	app.Key("enter")
	app.Type("label:Code", "{")
	app.Key("enter")
	if !strings.Contains(value, `println("hi")`+"\n\t{\n\t\t\n\t}") {
		t.Fatalf("braces and indentation: %q", value)
	}
	// The line break, then the braces typed.
	app.Key("ctrl+z")
	app.Key("ctrl+z")
	if strings.Contains(value, "\t{") {
		t.Fatalf("undo left the brace: %q", value)
	}
	// Completion: typing a word asks, the list filters, Enter accepts.
	app.Type("label:Code", "pr")
	if len(requests) == 0 || requests[0].Trigger != editor.CompletionTyping {
		t.Fatalf("requests %+v", requests)
	}
	app.Find("role:menuitem >> nth=0")
	if n := len(app.Query("role:menuitem")); n != 2 {
		t.Errorf("%d items for pr, want println and print", n)
	}
	app.Key("enter")
	if !strings.Contains(value, "println\n") && !strings.Contains(value, "print\n") {
		t.Fatalf("accepted: %q", value)
	}
	// Comments and lines.
	app.Key("ctrl+/")
	if !strings.Contains(value, "// print") {
		t.Fatalf("commented: %q", value)
	}
}

func TestEditorFoldsSearchesAndMarks(t *testing.T) {
	var result editor.SearchResult
	var folds []editor.FoldRange
	ref := &editor.Ref{}
	search := editor.Search{}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		q := core.NewSignal(cx, search)
		cx.AfterFunc(10*time.Millisecond, func() { q.Set(editor.Search{Query: "main"}) })
		return editor.Editor{
			Ref: ref, Language: "go", DefaultValue: sample,
			Search:         q.Get,
			OnSearchChange: func(r editor.SearchResult) { result = r },
			OnFoldChange:   func(f []editor.FoldRange) { folds = f },
			Diagnostics: func() []editor.Diagnostic {
				return []editor.Diagnostic{{Range: editor.Range{Start: editor.Position{Line: 3, Column: 10}, End: editor.Position{Line: 3, Column: 14}},
					Message: "unused"}}
			},
			Styles: editor.Styles{DiagnosticError: core.Style{TextDecoration: core.TextDecorationUnderline}},
			Style:  core.Style{Width: core.Pt(400), Height: core.Pt(300)},
		}
	})
	app.Advance(50 * time.Millisecond)
	if result.Count != 2 {
		t.Errorf("search found %d, want main twice", result.Count)
	}
	ref.Run(editor.FindNext)
	if got := ref.SelectedText(); got != "main" {
		t.Errorf("find next selected %q", got)
	}
	ref.Fold(2)
	app.Settle()
	if len(folds) != 1 || folds[0] != (editor.FoldRange{StartLine: 2, EndLine: 3}) {
		t.Fatalf("folds %v", folds)
	}
	if app.Exists(`text:println("hi")`) {
		t.Error("a folded line still shows")
	}
	ref.Unfold(2)
	app.Settle()
	app.Find(`text:println("hi")`)
	if style := app.SpanStyle(`text:println("hi")`); style.Decoration == 0 {
		t.Logf("diagnostic span style %+v", style)
	}
}

func TestEditorHighlightsOffTheEngine(t *testing.T) {
	var b strings.Builder
	for i := range 200_000 {
		fmt.Fprintf(&b, "func f%d() int { return %d } // c\n", i, i)
	}
	start := time.Now()
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return editor.Editor{Language: "go", DefaultValue: b.String(), Style: core.Style{Width: core.Pt(400), Height: core.Pt(300)}}
	})
	if took := time.Since(start); took > 3*time.Second {
		t.Errorf("200 000 lines took %v to show", took)
	}
	app.Find("text:func f0() int")
	if n := len(app.Query("text:func f")); n > 200 {
		t.Errorf("%d rows made, want the window's", n)
	}
	app.WaitUntil(5*time.Second, func() bool {
		return app.SpanStyle("text:func f1() int >> nth=0").Color != 0
	})
}

// words colors every word that starts with a capital letter as a title.
type words struct{}

func (words) Highlight(lines []string, from, to int) [][]editor.Token {
	out := make([][]editor.Token, to-from)
	for i := range out {
		l := lines[from+i]
		for at := 0; at < len(l); {
			end := strings.IndexByte(l[at:], ' ')
			if end < 0 {
				end = len(l) - at
			}
			if end > 0 && l[at] >= 'A' && l[at] <= 'Z' {
				out[i] = append(out[i], editor.Token{Start: at, End: at + end, Kind: "title"})
			}
			at += end + 1
		}
	}
	return out
}

func TestEditorInlineCompletionAndSelections(t *testing.T) {
	var value string
	var asked []editor.InlineCompletionRequest
	ref := &editor.Ref{}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		text := core.NewSignal(cx, "say(\"hi\")\n")
		return editor.Editor{
			Ref: ref, Language: "go", Label: "Code",
			Value:      text.Get,
			OnChange:   func(s string) { value = s; text.Set(s) },
			Completion: true,
			OnInlineCompletionRequest: func(r editor.InlineCompletionRequest) {
				asked = append(asked, r)
				ref.PresentInlineCompletion(r.ID, "lo()")
			},
			Style: core.Style{Width: core.Pt(400), Height: core.Pt(300)},
		}
	})
	ref.SetSelections([]editor.Selection{{Anchor: editor.Position{Line: 1}, Head: editor.Position{Line: 1}}})
	ref.Focus()
	app.Settle()
	app.Type("label:Code", "hel")
	app.Advance(100 * time.Millisecond)
	if len(asked) != 0 {
		t.Fatal("asked before typing paused")
	}
	app.Advance(400 * time.Millisecond)
	if len(asked) != 1 || asked[0].Offset != 13 || asked[0].Position != (editor.Position{Line: 1, Column: 3}) {
		t.Fatalf("requests %+v", asked)
	}
	app.Find("text:lo()")
	app.Key("tab")
	if !strings.HasSuffix(value, "hello()") || len(app.Query("text:lo()")) != 1 {
		t.Fatalf("tab took %q", value)
	}
	// Expanding and shrinking, then back through the moves.
	ref.SetSelections([]editor.Selection{{Anchor: editor.Position{Line: 0, Column: 5}, Head: editor.Position{Line: 0, Column: 5}}})
	ref.Run(editor.ExpandSelection)
	ref.Run(editor.ExpandSelection)
	if got := ref.SelectedText(); got != `"hi"` {
		t.Fatalf("expanded to %q", got)
	}
	ref.Run(editor.ShrinkSelection)
	if got := ref.SelectedText(); got != "hi" {
		t.Fatalf("shrank to %q", got)
	}
	ref.Run(editor.UndoSelection)
	if got := ref.SelectedText(); got != `"hi"` {
		t.Fatalf("undoSelection went to %q", got)
	}
	ref.Run(editor.RedoSelection)
	if got := ref.SelectedText(); got != "hi" {
		t.Fatalf("redoSelection went to %q", got)
	}
}

func TestEditorGrowsAndDrawsGuides(t *testing.T) {
	ref := &editor.Ref{}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return editor.Editor{
			Ref: ref, TestID: "editor", DefaultValue: "a  b\tc", MinRows: 2, MaxRows: 4,
			ShowWhitespaces: true, Rulers: []int{10},
			Highlighter: func(string) editor.Highlighter { return words{} },
			Styles:      editor.Styles{Ruler: core.Style{Background: core.RGB(0xff0000)}, Whitespace: core.Style{Color: core.RGB(0x888888)}},
			Style:       core.Style{Width: core.Pt(400)},
		}
	})
	if h := app.Find("id:editor").Frame.Size.Height; h != 40 {
		t.Errorf("one line in at least two rows: %v tall", h)
	}
	app.Find("text:··")
	app.Find("text:→")
	ref.SetValue("One\ntwo\nthree\nfour\nfive\nsix")
	app.Settle()
	if h := app.Find("id:editor").Frame.Size.Height; h != 80 {
		t.Errorf("six lines in at most four rows: %v tall", h)
	}
	app.WaitUntil(5*time.Second, func() bool { return app.SpanStyle("text:One").Color != 0 })
}

// TestComposition inserts what an input method commits: the commit's
// change comes while the composition lasts, and its end after it.
func TestComposition(t *testing.T) {
	var value string
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return editor.Editor{Label: "Code", OnChange: func(s string) { value = s }, Style: core.Style{Width: core.Pt(400), Height: core.Pt(300)}}
	}, nsgotest.Size(400, 300))
	app.Type("label:Code", "a")
	app.Compose("label:Code", "ni")
	app.Type("label:Code", "你")
	app.Compose("label:Code", "")
	app.Settle()
	if value != "a你" {
		t.Errorf("value = %q, want a你", value)
	}
}

// TestSoftKeyboard takes a software keyboard's Return and its Backspace in
// an empty sink, which reach no key binding.
func TestSoftKeyboard(t *testing.T) {
	var value string
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return editor.Editor{Label: "Code", OnChange: func(s string) { value = s }, Style: core.Style{Width: core.Pt(400), Height: core.Pt(300)}}
	}, nsgotest.Size(400, 300))
	app.Type("label:Code", "ab")
	app.Submit("label:Code")
	if value != "ab\n" {
		t.Fatalf("value = %q after Return", value)
	}
	app.KeyPress("label:Code", "Backspace")
	if value != "ab" {
		t.Errorf("value = %q after Backspace", value)
	}
}

func TestEditorMarksTheGutterAndOverviewRuler(t *testing.T) {
	var text strings.Builder
	for i := range 100 {
		text.WriteString("line " + strconv.Itoa(i) + "\n")
	}
	marks := []editor.Decoration{}
	var set *core.Signal[[]editor.Decoration]
	bar := core.Style{Background: core.RGB(0x00ff00)}
	lane := core.Style{Background: core.RGB(0x0000ff)}
	line := func(n int) editor.Position { return editor.Position{Line: n} }
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		set = core.NewSignalFunc(cx, marks, func(a, b []editor.Decoration) bool { return false })
		return editor.Editor{DefaultValue: text.String(), LineHeight: 20, Decorations: set.Get, TestID: "ed",
			Styles: editor.Styles{OverviewRuler: core.Style{Width: core.Pt(30)}},
			Style:  core.Style{Width: core.Pt(400), Height: core.Pt(200)}}
	})
	numberX := func() core.Points { return app.Find(`text:"1"`).Parent.Parent.Frame.Size.Width }
	before := numberX()
	set.Set([]editor.Decoration{
		// Lines 1 and 2: the range ends at line 3's start.
		{Range: editor.Range{Start: line(1), End: line(3)}, Gutter: &editor.GutterMark{Style: bar},
			OverviewRuler: &editor.OverviewRulerMark{Lane: editor.LaneRight, Style: lane}},
		{Range: editor.Range{Start: line(5), End: line(5)}, Gutter: &editor.GutterMark{Placement: editor.GutterBefore, Style: bar}},
		{Range: editor.Range{Start: line(50), End: line(50)}, OverviewRuler: &editor.OverviewRulerMark{Lane: editor.LaneFull, Style: lane}},
	})
	app.Settle()
	if after := numberX(); after != before+3 {
		t.Fatalf("the numbers moved from %v to %v, want the strip's 3 points", before, after)
	}
	painted := func(color core.Color) []*nsgotest.View {
		var out []*nsgotest.View
		var walk func(v *nsgotest.View)
		walk = func(v *nsgotest.View) {
			if v.Style.Background == color {
				out = append(out, v)
			}
			for _, c := range v.Children {
				walk(c)
			}
		}
		walk(app.Find("ed"))
		return out
	}
	bars := painted(bar.Background)
	if len(bars) != 3 {
		t.Fatalf("%d gutter marks, want lines 1, 2 and 5's boundary", len(bars))
	}
	for _, v := range bars[:2] {
		if b := v.Frame; b.Size.Width != 3 || b.Size.Height != 20 {
			t.Fatalf("a line's bar is %+v", b)
		}
	}
	if b := bars[2].Frame; b.Size.Width != 3 || b.Size.Height != 2 || b.Origin.Y != -1 {
		t.Fatalf("a boundary's line is %+v", b)
	}
	rulers := painted(lane.Background)
	if len(rulers) != 2 {
		t.Fatalf("%d ruler marks, want 2", len(rulers))
	}
	if b := rulers[0].Frame; b.Size.Width != 10 || b.Origin.X != 20 || b.Size.Height != 6 {
		t.Fatalf("a right lane's mark is %+v of a 30-point ruler", b)
	}
	if b := rulers[1].Frame; b.Size.Width != 30 || b.Origin.Y <= 0 {
		t.Fatalf("a full lane's mark is %+v", b)
	}
}

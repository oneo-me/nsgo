package editor

import (
	"slices"
	"strings"
	"testing"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

const nested = `func a() {
	if x {
		y()
	}
	for {
		z()
	}
}
func b() {
	w()
}
`

func TestRefEditsAsOneStepAndFoldsByNesting(t *testing.T) {
	ref := &Ref{}
	var value string
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return Editor{Ref: ref, DefaultValue: nested, Language: "go", OnChange: func(s string) { value = s },
			Style: core.Style{Width: core.Pt(400), Height: core.Pt(300)}}
	})
	ref.SetSelections([]Selection{{Anchor: Position{2, 2}, Head: Position{2, 3}}})
	ref.Edit([]TextEdit{
		{Range: Range{Position{0, 5}, Position{0, 6}}, Text: "alpha"},
		{Range: Range{Position{0, 5}, Position{0, 7}}, Text: "skipped"},
		{Range: Range{Position{2, 2}, Position{2, 3}}, Text: "yy"},
	})
	app.Settle()
	if !strings.HasPrefix(value, "func alpha() {\n\tif x {\n\t\tyy()") {
		t.Fatalf("edited %q", value)
	}
	if s := ref.Selections()[0]; s.Anchor != (Position{2, 2}) || s.Head != (Position{2, 4}) {
		t.Errorf("the selection is %+v, want over what replaced it", s)
	}
	ref.Run(Undo)
	app.Settle()
	if value != nested {
		t.Errorf("one undo left %q", value)
	}
	ref.FoldLevel(2)
	if got := ref.Folds(); !slices.Equal(got, []FoldRange{{1, 2}, {4, 5}}) {
		t.Errorf("level 2 folds %v", got)
	}
	ref.SetFolds(nil)
	ref.FoldRecursive(0)
	if got := ref.Folds(); len(got) != 3 {
		t.Errorf("recursive folds %v, want the function's and its two inside", got)
	}
	ref.UnfoldRecursive(2)
	if got := ref.Folds(); len(got) != 0 {
		t.Errorf("unfolding around line 2 left %v", got)
	}
	ref.FoldRange(FoldRange{8, 9})
	if got := ref.Folds(); !slices.Equal(got, []FoldRange{{8, 9}}) {
		t.Errorf("a range folded %v", got)
	}
}

func TestRefConvertsAndMeasures(t *testing.T) {
	ref := &Ref{}
	var lines []string
	for i := range 100 {
		lines = append(lines, strings.Repeat("é", i%5)+"line")
	}
	text := strings.Join(lines, "\n")
	var presses []TextPressEvent
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{Padding: core.All(10)}, Children: []core.Node{
			Editor{Ref: ref, DefaultValue: text, OnTextPress: func(e TextPressEvent) { presses = append(presses, e) },
				Style: core.Style{Width: core.Pt(400), Height: core.Pt(200)}}}}
	}, nsgotest.Environment(core.Environment{Idiom: core.IdiomDesktop}))
	e := ref.e
	// Line 2 is "éé" then "line": 6 units, after 4 + 1 and 5 + 1.
	if p := ref.OffsetToPosition(13); p != (Position{2, 2}) {
		t.Errorf("offset 13 is %+v", p)
	}
	if o := ref.PositionToOffset(Position{2, 2}); o != 13 {
		t.Errorf("position 2:2 is offset %d", o)
	}
	if p := ref.OffsetToPosition(1 << 20); p.Line != 99 {
		t.Errorf("past the end is %+v", p)
	}
	boxes := ref.RangeBounds(Range{Position{1, 0}, Position{2, 3}})
	if len(boxes) != 2 || boxes[0].Bounds.Origin.Y != core.Points(e.lh) || boxes[1].Page.Origin.Y != boxes[1].Bounds.Origin.Y+10 {
		t.Fatalf("boxes %+v", boxes)
	}
	if b, ok := ref.CaretBounds(); !ok || b.Bounds.Size.Height != core.Points(e.lh) {
		t.Errorf("caret %+v %v", b, ok)
	}
	ref.SetScrollOffset(core.Point{Y: 400})
	app.Settle()
	if o := ref.ScrollOffset(); o.Y != 400 {
		t.Errorf("scrolled to %v", o)
	}
	if v := ref.VisibleRange(); v.Start != (Position{20, 0}) || v.End.Line != 29 {
		t.Errorf("visible %+v, want lines 20 to 29", v)
	}
	if boxes := ref.RangeBounds(Range{Position{20, 0}, Position{20, 1}}); boxes[0].Bounds.Origin.Y != 0 {
		t.Errorf("a box at the top after scrolling: %+v", boxes[0])
	}
	ref.SetScrollOffset(core.Point{})
	app.Settle()
	e.pressDown(core.Point{X: core.Points(pad + 2.5*e.charW), Y: core.Points(2.5 * e.lh)}, 1, core.Modifiers{})
	e.textPress(core.Point{X: core.Points(pad + 2.5*e.charW), Y: core.Points(2.5 * e.lh)}, core.Point{}, core.Modifiers{}, 1)
	e.textPress(core.Point{X: core.Points(pad + 40*e.charW), Y: core.Points(2.5 * e.lh)}, core.Point{}, core.Modifiers{}, 1)
	if len(presses) != 1 || presses[0].Position != (Position{2, 2}) || presses[0].Offset != 13 || presses[0].Word.End != (Position{2, 6}) {
		t.Errorf("presses %+v, want the third character of line 2 alone", presses)
	}
}

func TestCommandsOfWordsParagraphsAndCase(t *testing.T) {
	ref := &Ref{}
	var value string
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return Editor{Ref: ref, DefaultValue: "one two one\n\nthree one\nfour\n\nfive", OnChange: func(s string) { value = s },
			Style: core.Style{Width: core.Pt(400), Height: core.Pt(300)}}
	})
	at := func(line, col int) { ref.SetSelections([]Selection{{Anchor: Position{line, col}, Head: Position{line, col}}}) }
	at(2, 7)
	ref.Run(SelectWord)
	if got := ref.SelectedText(); got != "one" {
		t.Errorf("select word: %q", got)
	}
	ref.Run(SelectPreviousOccurrence)
	ref.Run(SelectPreviousOccurrence)
	if sels := ref.Selections(); len(sels) != 3 || sels[2].Anchor != (Position{0, 0}) {
		t.Errorf("previous occurrences: %+v", sels)
	}
	at(2, 0)
	ref.Run(MoveParagraphDown)
	if h := ref.Selections()[0].Head; h != (Position{4, 0}) {
		t.Errorf("paragraph down to %+v", h)
	}
	ref.Run(SelectParagraphUp)
	if s := ref.Selections()[0]; s.Anchor != (Position{4, 0}) || s.Head != (Position{1, 0}) {
		t.Errorf("paragraph up selecting %+v", s)
	}
	ref.SetSelections([]Selection{{Anchor: Position{2, 0}, Head: Position{3, 4}}})
	ref.Run(TransformToTitleCase)
	app.Settle()
	if !strings.Contains(value, "Three One\nFour") {
		t.Errorf("title case: %q", value)
	}
	app.Clipboard = "pasted "
	at(0, 0)
	ref.Run(Paste)
	app.Settle()
	if !strings.HasPrefix(value, "pasted one") {
		t.Errorf("paste: %q", value)
	}
}

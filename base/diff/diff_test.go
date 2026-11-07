package diff

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/nsgotest"
)

const oldGo = `package main

import "fmt"

func main() {
	fmt.Println("hello")
	x := 1
	fmt.Println(x)
}
`

const newGo = `package main

import "fmt"

func main() {
	fmt.Println("hello, world")
	y := 2
	fmt.Println(y)
	fmt.Println("done")
}
`

func TestModelPairsAlikeLinesAndMarksWords(t *testing.T) {
	m := newModel(oldGo, newGo)
	if len(m.hunks) != 1 {
		t.Fatalf("%d hunks, want one", len(m.hunks))
	}
	h := m.hunks[0]
	if h.oldEnd-h.oldStart != 3 || h.newEnd-h.newStart != 4 || len(h.pairs) != 3 {
		t.Fatalf("hunk %+v", h)
	}
	old, new := inline(m.old[h.pairs[0][0]], m.new[h.pairs[0][1]], InlineWordAlt)
	if len(old) != 0 || len(new) != 1 || m.new[h.pairs[0][1]][new[0][0]:new[0][1]] != ", world" {
		t.Fatalf("inline %v %v", old, new)
	}
	o, n := m.revert(h)
	if o != oldGo || n != oldGo {
		t.Fatalf("revert gave %q", n)
	}
	o, _ = m.accept(h)
	if o != newGo {
		t.Fatalf("accept gave %q", o)
	}
	// A missing final line break is a change.
	if m := newModel("a\nb\n", "a\nb"); len(m.hunks) != 1 {
		t.Fatalf("final line break: %d hunks", len(m.hunks))
	}
}

const patch = `From 1234 Mon Sep 17 00:00:00 2001
Subject: [PATCH] greet

diff --git a/main.go b/main.go
index 111..222 100644
--- a/main.go
+++ b/main.go
@@ -4,4 +4,4 @@ import "fmt"

 func main() {
-	fmt.Println("hello")
+	fmt.Println("hello, world")
 	x := 1
diff --git a/old.txt b/new.txt
similarity index 90%
rename from old.txt
rename to new.txt
--- a/old.txt
+++ b/new.txt
@@ -1 +1,2 @@
 one
+two
\ No newline at end of file
`

func TestPatchFilesAndTexts(t *testing.T) {
	files, err := parsePatch(patch)
	if err != nil || len(files) != 2 {
		t.Fatalf("files %d, err %v", len(files), err)
	}
	if f := files[1]; f.status != FileRenamed || f.oldPath != "old.txt" || f.path != "new.txt" {
		t.Fatalf("rename: %+v", f)
	}
	old, new, on, nn := files[0].texts()
	if !strings.Contains(old, `"hello")`) || !strings.Contains(new, "hello, world") || on[0] != 4 || nn[0] != 4 {
		t.Fatalf("texts %q %q %v %v", old, new, on, nn)
	}
	_, new, _, _ = files[1].texts()
	if new != "one\ntwo" {
		t.Fatalf("no final line break: %q", new)
	}
}

func TestDiffViewShowsChangesAndSeparators(t *testing.T) {
	var stats Stats
	ref := &Ref{}
	var lines strings.Builder
	for i := range 40 {
		lines.WriteString("line " + string(rune('a'+i%26)) + "\n")
	}
	oldText := lines.String()
	newText := strings.Replace(oldText, "line t\n", "line T\n", 1)
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return DiffView{
			Ref: ref, OldValue: func() string { return oldText }, NewValue: func() string { return newText },
			OnStatsChange: func(s Stats) { stats = s },
			OnChange:      func(Change) {},
			Style:         core.Style{Width: core.Pt(600), Height: core.Pt(400)},
		}
	})
	if stats != (Stats{Added: 1, Deleted: 1, Hunks: 1}) {
		t.Fatalf("stats %+v", stats)
	}
	app.Find("text:⋯ 16 unchanged lines")
	app.Find("text:⋯ 17 unchanged lines")
	app.Find("text:line T")
	app.Press("text:⋯ 16 unchanged lines")
	if app.Exists("text:⋯ 16 unchanged lines") {
		t.Error("a pressed separator still hides its lines")
	}
	if h := ref.Hunks(); len(h) != 1 || h[0].OldStart != 20 {
		t.Errorf("hunks %+v", h)
	}
	ref.RevertHunk(0)
	app.Settle()
	if ref.Stats().Hunks != 0 {
		t.Errorf("reverted: %+v", ref.Stats())
	}
}

func TestDiffViewShowsAPatchByFile(t *testing.T) {
	var toggled []File
	ref := &Ref{}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return DiffView{Ref: ref, Patch: func() string { return patch }, Unified: true,
			OnFileToggle: func(f File) { toggled = append(toggled, f) },
			Style:        core.Style{Width: core.Pt(600), Height: core.Pt(400)}}
	})
	app.Find(`text:import "fmt"`)
	app.Find("text:old.txt → new.txt")
	files := ref.Files()
	if len(files) != 2 || files[0].Added != 1 || files[1].Added != 1 {
		t.Fatalf("files %+v", files)
	}
	app.Press("label:main.go")
	if len(toggled) != 1 || !toggled[0].Collapsed || app.Exists("text:hello, world") {
		t.Fatalf("collapse: %+v", toggled)
	}
	if !slices.ContainsFunc(app.Texts(), func(s string) bool { return s == "two" }) {
		t.Error("the other file's lines are gone")
	}
}

func TestDiffViewSelectsLinesAndAnnotates(t *testing.T) {
	var selections []*LineRange
	ref := &Ref{}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		notes := core.NewSignalFunc(cx, []Annotation(nil), slices.Equal)
		return DiffView{
			Ref: ref, OldValue: func() string { return oldGo }, NewValue: func() string { return newGo },
			Annotations:           notes.Get,
			OnLineSelectionChange: func(r *LineRange) { selections = append(selections, r) },
			RenderAnnotation: func(cx *core.Scope, a Annotation) core.Node {
				return core.Text{Content: "Note on " + a.ID, Style: core.Style{Height: core.Pt(30)}}
			},
			RenderAddAnnotation: func(cx *core.Scope, lines LineRange) core.Node {
				return core.View{Role: core.RoleButton, Label: "Add note", Style: core.Style{Width: core.Pt(12), Height: core.Pt(12)}, OnPress: func(core.PressEvent) {
					notes.Set(append(notes.Get(), Annotation{ID: "n1", Side: lines.Side, Line: lines.End}))
				}}
			},
			Style: core.Style{Width: core.Pt(800), Height: core.Pt(400)},
		}
	})
	// The new side's number 7 is `y := 2`.
	app.Press("text:7 >> nth=1")
	if len(selections) != 1 || *selections[0] != (LineRange{Side: New, Start: 6, End: 6}) {
		t.Fatalf("selections %+v", selections)
	}
	ref.v.selectLine(New, 7, true)
	app.Settle()
	if got := ref.SelectedText(); got != "\ty := 2\n\tfmt.Println(y)" {
		t.Fatalf("selected %q", got)
	}
	top := func() float32 { return ref.v.tops[ref.v.rowOfLine(New, 8)] }
	below := top()
	app.Press("label:Add note")
	app.Find("text:Note on n1")
	if moved := top(); moved != below+30 {
		t.Errorf("the row under the note at %v, was %v", moved, below)
	}
	ref.SetSelectedLines(nil)
	if _, ok := ref.SelectedLines(); ok || len(selections) != 2 {
		t.Errorf("cleared silently: %+v", selections)
	}
}

func TestDiffViewWrapsLongLines(t *testing.T) {
	long := strings.Repeat("word ", 40)
	ref := &Ref{}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return DiffView{Ref: ref, OldValue: func() string { return "a\n" }, NewValue: func() string { return "a\n" + long + "\nb\n" },
			SoftWrap: true, Unified: true, Style: core.Style{Width: core.Pt(400), Height: core.Pt(400)}}
	})
	if n := len(app.Query("text:word word")); n < 3 {
		t.Errorf("the long line in %d rows", n)
	}
	if y := ref.v.tops[ref.v.rowOfLine(New, 2)]; y < 80 {
		t.Errorf("the line after the wrapped one at %v", y)
	}
}

func TestWrapStartsBreakAfterSpaces(t *testing.T) {
	if got := wrapStarts("aaa bbb ccc", 5); !slices.Equal(got, []int{0, 4, 8}) {
		t.Errorf("starts %v", got)
	}
	if got := wrapStarts("abcdefghij", 4); !slices.Equal(got, []int{0, 4, 8}) {
		t.Errorf("hard breaks %v", got)
	}
}

func TestDiffViewDrawsTheAppsSeparatorsAndHeaders(t *testing.T) {
	ref := &Ref{}
	var lines strings.Builder
	for i := range 60 {
		lines.WriteString("line " + strings.Repeat("x", i%7) + string(rune('a'+i%26)) + "\n")
	}
	oldText := lines.String()
	ls := strings.Split(oldText, "\n")
	ls[10], ls[45] = "changed ten", "changed forty-five"
	newText := strings.Join(ls, "\n")
	var seps []Separator
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return DiffView{Ref: ref, TestID: "diff", OldValue: func() string { return oldText }, NewValue: func() string { return newText },
			Unified: true, Style: core.Style{Width: core.Pt(600), Height: core.Pt(1400)},
			RenderSeparator: func(cx *core.Scope, s Separator) core.Node {
				seps = append(seps, s)
				return core.Text{Content: "sep " + strconv.Itoa(s.Gap) + " of " + strconv.Itoa(s.Lines)}
			}}
	}, nsgotest.Size(600, 1400))
	if len(seps) == 0 || seps[0] != (Separator{Gap: 0, Lines: 7, First: true}) {
		t.Fatalf("separators %+v", seps)
	}
	if !app.Exists("text:sep 1 of 28") {
		t.Fatalf("the second separator is not the app's: %q", app.Texts())
	}
	ref.Expand(1, ExpandUp, 5)
	app.Settle()
	if !app.Exists("text:sep 1 of 23") {
		t.Errorf("after five lines up: %q", app.Texts())
	}
	ref.Expand(1, ExpandDown, 0)
	app.Settle()
	if !app.Exists("text:sep 1 of 3") {
		t.Errorf("after twenty lines down: %q", app.Texts())
	}
	ref.Expand(1, ExpandEvery, 0)
	app.Settle()
	if app.Exists("text:sep 1 of 3") {
		t.Error("the gap still shows a separator after expanding all of it")
	}
	if h := hunkOf(ref.CurrentHunk()); h != 0 {
		t.Errorf("current change %d at the top, want 0", h)
	}
	// From the top, the first change below it; then on from the change
	// gone to.
	if h := hunkOf(ref.NextHunk()); h != 0 {
		t.Errorf("next change from the top %d, want 0", h)
	}
	if h := hunkOf(ref.NextHunk()); h != 1 || hunkOf(ref.CurrentHunk()) != 1 {
		t.Errorf("next change %d, current %d, want 1", h, hunkOf(ref.CurrentHunk()))
	}
	if h := hunkOf(ref.NextHunk()); h != 0 {
		t.Errorf("after the last change %d, want the first", h)
	}
	if h := hunkOf(ref.PreviousHunk()); h != 1 {
		t.Errorf("before the first change %d, want the last", h)
	}
}

func TestDiffViewDrawsTheAppsFileHeaders(t *testing.T) {
	ref := &Ref{}
	var toggled []File
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return DiffView{Ref: ref, Patch: func() string { return patch }, Style: core.Style{Width: core.Pt(600), Height: core.Pt(400)},
			OnFileToggle: func(f File) { toggled = append(toggled, f) },
			RenderFileHeader: func(cx *core.Scope, f File) core.Node {
				mark := "open"
				if f.Collapsed {
					mark = "shut"
				}
				return core.Text{Content: strconv.Itoa(f.Index) + " " + f.Path + " " + mark}
			}}
	}, nsgotest.Size(600, 400))
	if !app.Exists("text:0 main.go open") || !app.Exists("text:1 new.txt open") {
		t.Fatalf("headers %q", app.Texts())
	}
	ref.CollapseFile("new.txt")
	app.Settle()
	ref.CollapseFile("new.txt")
	app.Settle()
	if !app.Exists("text:1 new.txt shut") || len(toggled) != 1 || app.Exists("text:two") {
		t.Errorf("collapsing: %q, toggles %d", app.Texts(), len(toggled))
	}
	ref.ExpandFile("new.txt")
	app.Settle()
	if !app.Exists("text:1 new.txt open") || !app.Exists("text:two") || len(toggled) != 2 {
		t.Errorf("expanding: %q, toggles %d", app.Texts(), len(toggled))
	}
}

// hunkOf is a change's index, -1 for none.
func hunkOf(i int, ok bool) int {
	if !ok {
		return -1
	}
	return i
}

// A selectable diff makes its text column a selection area, its numbers
// outside it.
func TestDiffViewSelectable(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return DiffView{OldValue: func() string { return "one\ntwo\n" }, NewValue: func() string { return "one\nthree\n" }, Unified: true, Selectable: true,
			Style: core.Style{Width: core.Pt(400), Height: core.Pt(200)}}
	}, nsgotest.Size(400, 200))
	inArea := func(selector string) bool {
		for v := app.Find(selector); v != nil; v = v.Parent {
			if v.Props.SelectionArea {
				return true
			}
		}
		return false
	}
	if !inArea("text:three") || inArea(`text:"2" >> nth=0`) {
		t.Fatalf("text in an area: %v, number: %v\n%s", inArea("text:three"), inArea(`text:"2" >> nth=0`), app.Dump())
	}
}

// Lines deleted past the new text's end show, as all of a deleted file's do.
func TestDiffViewShowsLinesPastTheNewText(t *testing.T) {
	for _, unified := range []bool{true, false} {
		app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
			return DiffView{OldValue: func() string { return "alpha\nbeta\n" }, NewValue: func() string { return "" }, Unified: unified,
				Style: core.Style{Width: core.Pt(400), Height: core.Pt(200)}}
		}, nsgotest.Size(400, 200))
		app.Find("text:alpha")
		app.Find("text:beta")
	}
}

// What the values read changing leaves the rows alone while the values stay,
// as making them again would end a text selection.
func TestDiffViewKeepsItsRowsWhileItsTextsStay(t *testing.T) {
	var tick, text *core.Signal[string]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		tick, text = core.NewSignal(cx, ""), core.NewSignal(cx, "two\n")
		return DiffView{OldValue: func() string { return "one\n" + tick.Get()[:0] }, NewValue: text.Get, Unified: true,
			Style: core.Style{Width: core.Pt(400), Height: core.Pt(200)}}
	}, nsgotest.Size(400, 200))
	line := app.Find("text:two").ID
	tick.Set("again")
	app.Settle()
	if app.Find("text:two").ID != line {
		t.Fatal("the rows were made again")
	}
	text.Set("three\n")
	app.Settle()
	app.Find("text:three")
}

func TestRegionActsOnTheChangeUnderThePointerOrTheLinesDragged(t *testing.T) {
	ref := &Ref{}
	var got Change
	var regions []Region
	oldText := "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n"
	newText := "a\nb\nc\nD\nE\nf\ng\nh\ni\nj\n"
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return DiffView{Ref: ref, TestID: "diff", Unified: true, OldValue: func() string { return oldText }, NewValue: func() string { return newText },
			OnChange: func(c Change) { got = c },
			RenderRegionActions: func(cx *core.Scope, r Region) core.Node {
				regions = append(regions, r)
				return core.View{TestID: "actions", Role: core.RoleButton, Style: core.Style{Width: core.Pt(40), Height: core.Pt(16)},
					OnPress: func(core.PressEvent) { ref.AcceptRegion(r) }}
			},
			Style: core.Style{Width: core.Pt(600), Height: core.Pt(400)}}
	}, nsgotest.Environment(core.Environment{Idiom: core.IdiomDesktop}), nsgotest.Size(600, 400))
	app.Settle()
	if app.Exists("id:actions") {
		t.Fatal("actions before the pointer is on a change")
	}
	// Unchanged lines next to the change hold it; those further do not.
	app.Hover("text:b")
	app.Settle()
	if !app.Exists("id:actions") {
		t.Fatal(app.Dump())
	}
	r := regions[len(regions)-1]
	if !slices.Equal(r.Old, []int{3, 4}) || !slices.Equal(r.New, []int{3, 4}) || r.Selected {
		t.Fatalf("hovered %+v", r)
	}
	app.Hover("text:a >> nth=0")
	app.Advance(hoverLeave * 2)
	app.Settle()
	if app.Exists("id:actions") {
		t.Fatal("actions two lines past the frame")
	}
	// A selection from the old d to the new D frames those rows whole.
	app.SelectText("id:diff-rows", driver.Bounds{Origin: driver.Point{X: 12, Y: 70}, Size: driver.Size{Width: 8, Height: 40}})
	app.Settle()
	r = regions[len(regions)-1]
	if !slices.Equal(r.Old, []int{3, 4}) || !slices.Equal(r.New, []int{3}) || !r.Selected {
		t.Fatalf("selected %+v", r)
	}
	// A press on the actions clears the selection around them first.
	app.Hover("id:actions")
	app.SelectText("id:diff-rows", driver.Bounds{})
	app.Settle()
	app.Press("id:actions")
	app.Settle()
	if want := "a\nb\nc\nD\nf\ng\nh\ni\nj\n"; got.OldValue != want || got.NewValue != newText {
		t.Fatalf("staged %q %q", got.OldValue, got.NewValue)
	}
}

func TestLinesRevertAndAcceptApart(t *testing.T) {
	m := newModel("a\nb\nc\n", "a\nB\nC\nD\n")
	o, n := m.acceptLines([]int{1}, []int{1})
	if o != "a\nB\nc\n" || n != "a\nB\nC\nD\n" {
		t.Fatalf("accept %q %q", o, n)
	}
	o, n = m.revertLines([]int{1}, []int{2})
	if o != "a\nb\nc\n" || n != "a\nb\nB\nD\n" {
		t.Fatalf("revert %q %q", o, n)
	}
}

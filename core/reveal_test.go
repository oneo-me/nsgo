package core_test

import (
	"fmt"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// Reveal scrolls every scroll view around a view, innermost first, each
// clamped to its content, to where the alignments say.
func TestRevealScrollsNestedScrollViews(t *testing.T) {
	target := new(core.Ref)
	rows := func(n int, mark int) []core.Node {
		var out []core.Node
		for i := range n {
			v := core.View{Style: core.Style{Height: core.Pt(50), Width: core.Pt(50)}}
			if i == mark {
				v.Ref, v.TestID = target, "target"
			}
			out = append(out, v)
		}
		return out
	}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.Scroll{TestID: "outer", Style: core.Style{Height: core.Pt(200)}, Children: []core.Node{
			core.View{Style: core.Style{Height: core.Pt(300)}},
			core.Scroll{TestID: "inner", Horizontal: true, Style: core.Style{Height: core.Pt(50), MinHeight: core.Pt(50), Width: core.Pt(100)},
				Children: []core.Node{core.View{Style: core.Style{Direction: core.Row}, Children: rows(10, 6)}}},
			core.View{Style: core.Style{Height: core.Pt(300)}},
		}}
	}, nsgotest.Size(200, 400))
	target.Reveal(core.RevealOptions{})
	app.Settle()
	// Nearest horizontally: its right edge at the inner view's end; the
	// start vertically: the inner view at the outer's top.
	if x := app.Find("id:inner").Offset.X; x != 250 {
		t.Errorf("inner offset %v, want 250", x)
	}
	if y := app.Find("id:outer").Offset.Y; y != 300 {
		t.Errorf("outer offset %v, want 300", y)
	}
	// Centered, and clamped to the content at the end.
	target.Reveal(core.RevealOptions{Vertical: core.ScrollAlignEnd, Horizontal: core.ScrollAlignCenter})
	app.Settle()
	if x := app.Find("id:inner").Offset.X; x != 275 {
		t.Errorf("centered inner offset %v, want 275", x)
	}
	if y := app.Find("id:outer").Offset.Y; y != 150 {
		t.Errorf("outer offset at its end %v, want 150", y)
	}
	// Nearest leaves a view that shows whole where it is.
	target.Reveal(core.RevealOptions{Vertical: core.ScrollAlignNearest, Horizontal: core.ScrollAlignNearest})
	app.Settle()
	if x, y := app.Find("id:inner").Offset.X, app.Find("id:outer").Offset.Y; x != 275 || y != 150 {
		t.Errorf("nearest moved a shown view to %v, %v", x, y)
	}
}

// The keyboard moving the focus scrolls the stop it reaches into view; a
// press's focus scrolls nothing.
func TestKeyboardFocusScrollsIntoView(t *testing.T) {
	var rows []core.Node
	for i := range 20 {
		rows = append(rows, core.View{TestID: fmt.Sprint("row", i), Focusable: func() bool { return true },
			OnPress: func(core.PressEvent) {}, Style: core.Style{Height: core.Pt(50)}})
	}
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.Scroll{TestID: "scroll", Style: core.Style{Height: core.Pt(200)}, Children: rows}
	}, nsgotest.Size(200, 400), nsgotest.Environment(core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop}))
	for range 5 {
		app.Key("tab")
	}
	if !app.Find("id:row4").Focused {
		t.Fatal("Tab did not reach the fifth row")
	}
	if y := app.Find("id:scroll").Offset.Y; y != 50 {
		t.Errorf("offset %v, want the fifth row at the end (50)", y)
	}
	app.Press("id:row1")
	if y := app.Find("id:scroll").Offset.Y; y != 50 {
		t.Errorf("a press scrolled to %v", y)
	}
}

// A list keeps the rows beside the focused one, so Tab walks every row,
// each scrolled into view as the keyboard reaches it.
func TestTabWalksEveryRowOfAList(t *testing.T) {
	list := new(core.Ref)
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.List{Ref: list, TestID: "list", Style: core.Style{Height: core.Pt(200)}, Count: func() int { return 100 },
			Row: func(cx *core.Scope, row core.ListRow) core.Node {
				return core.View{OnPress: func(core.PressEvent) {}, Style: core.Style{Height: core.Pt(50)},
					Children: []core.Node{core.Text{ContentFunc: func() string { return fmt.Sprint("row ", row.Index()) }}}}
			}}
	}, nsgotest.Size(200, 400), nsgotest.Environment(core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop}))
	for i := range 40 {
		app.Key("tab")
		if !app.Find(fmt.Sprint("text:row ", i)).Parent.Focused {
			t.Fatalf("Tab %d did not reach row %d\n%s", i+1, i, app.Dump())
		}
	}
	if y := app.Find("id:list").Offset.Y; y != 39*50+50-200 {
		t.Errorf("offset %v, want row 39 at the end", y)
	}
	// Scrolled far from the focus, the list keeps its row and the next.
	list.ScrollTo(core.Point{Y: 4000}, false)
	app.Settle()
	app.Key("tab")
	if !app.Find("text:row 40").Parent.Focused {
		t.Fatalf("Tab after a scroll away did not reach row 40\n%s", app.Dump())
	}
}

// A list keeps a row holding an open overlay, such as its menu, however
// far it scrolls away.
func TestListKeepsARowHoldingAnOverlay(t *testing.T) {
	list := new(core.Ref)
	var open *core.Signal[bool]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		open = core.NewSignal(cx, false)
		return core.List{Ref: list, Style: core.Style{Height: core.Pt(200)}, Count: func() int { return 100 },
			Row: func(cx *core.Scope, row core.ListRow) core.Node {
				return core.View{Style: core.Style{Height: core.Pt(50)}, Children: []core.Node{
					core.Text{ContentFunc: func() string { return fmt.Sprint("row ", row.Index()) }},
					core.Show{When: func() bool { return row.Index() == 0 && open.Get() }, Then: func(*core.Scope) core.Node {
						return core.Overlay{TestID: "menu", Children: []core.Node{core.Text{Content: "menu of row 0"}}}
					}},
				}}
			}}
	}, nsgotest.Size(200, 400))
	open.Set(true)
	app.Settle()
	list.ScrollTo(core.Point{Y: 3000}, false)
	app.Settle()
	if !app.Exists("text:menu of row 0") {
		t.Fatal("scrolled away, the list let go of the row holding an open overlay")
	}
	open.Set(false)
	list.ScrollTo(core.Point{Y: 3500}, false)
	app.Settle()
	if app.Exists("text:row 0") {
		t.Error("the list kept row 0 after its overlay closed")
	}
}

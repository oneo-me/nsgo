package core_test

import (
	"fmt"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// chat is an end-aligned list of n rows 80 high, estimated at 50.
func chat(t *testing.T, n int) (*nsgotest.App, *core.Signal[int], *core.Ref) {
	return listOf(t, n, core.ListAlignEnd)
}

func listOf(t *testing.T, n int, align core.ListAlignment) (*nsgotest.App, *core.Signal[int], *core.Ref) {
	var count *core.Signal[int]
	ref := new(core.Ref)
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		count = core.NewSignal(cx, n)
		return core.List{Ref: ref, TestID: "list", Alignment: align, Style: core.Style{Height: core.Pt(400)},
			Count: count.Get, Row: func(cx *core.Scope, row core.ListRow) core.Node {
				return core.View{Style: core.Style{Height: core.Pt(80)},
					Children: []core.Node{core.Text{ContentFunc: func() string { return fmt.Sprint("row ", row.Index()) }}}}
			}}
	}, nsgotest.Size(200, 400))
	return app, count, ref
}

// bottomOf is where row i ends in the list's viewport.
func bottomOf(app *nsgotest.App, i int) float32 {
	list := app.Find("id:list")
	row := app.Find(fmt.Sprint("text:row ", i)).Parent
	y := float32(row.Frame.Origin.Y + row.Frame.Size.Height)
	for p := row.Parent; p != nil && p != list; p = p.Parent {
		y += float32(p.Frame.Origin.Y)
	}
	return y - float32(list.Offset.Y)
}

// An end-aligned list opens at its end with its rows measured, keeps
// there as rows come, stops following while scrolled away, and holds
// short content at its end.
func TestListAlignedToItsEnd(t *testing.T) {
	app, count, ref := chat(t, 3)
	if b := bottomOf(app, 2); b != 400 {
		t.Fatalf("a short list's last row ends at %v, not the viewport's end", b)
	}
	count.Set(30)
	app.Settle()
	if b := bottomOf(app, 29); b != 400 {
		t.Fatalf("the last of 30 rows ends at %v", b)
	}
	count.Set(31)
	app.Settle()
	if b := bottomOf(app, 30); b != 400 {
		t.Fatalf("a row arriving at the end ends at %v", b)
	}
	ref.ScrollTo(core.Point{Y: 200}, false)
	app.Settle()
	count.Set(32)
	app.Settle()
	if y := app.Find("id:list").Offset.Y; y != 200 {
		t.Errorf("scrolled away, the list followed to %v", y)
	}
}

func TestListOpensAtItsEnd(t *testing.T) {
	app, _, _ := chat(t, 100)
	if b := bottomOf(app, 99); b != 400 {
		t.Fatalf("the last of 100 rows ends at %v as the list opens", b)
	}
}

// ListPosition reads the row the viewport starts in and how far into
// it, which ScrollToIndexAt restores in a list made anew.
func TestListPositionRestores(t *testing.T) {
	app, _, ref := chat(t, 100)
	ref.ScrollTo(core.Point{Y: 1234}, false)
	app.Settle()
	p, ok := ref.ListPosition()
	if !ok || p.Offset < 0 || p.Offset >= 80 {
		t.Fatalf("position %+v %v", p, ok)
	}
	top := bottomOf(app, p.Index) - 80
	if top != -float32(p.Offset) {
		t.Fatalf("row %d starts %v into the viewport, not %v", p.Index, top, -p.Offset)
	}
	again, _, ref2 := chat(t, 100)
	ref2.ScrollToIndexAt(p.Index, 0, -p.Offset, false)
	again.Settle()
	if q, _ := ref2.ListPosition(); q != p {
		t.Errorf("restored to %+v, want %+v", q, p)
	}
	if top := bottomOf(again, p.Index) - 80; top != -float32(p.Offset) {
		t.Errorf("restored row %d starts %v into the viewport, not %v", p.Index, top, -p.Offset)
	}
}

// An end-aligned list keeps at its end as rows go, and follows it again
// once scrolled back there.
func TestListAlignedToItsEndShrinksAndFollowsAgain(t *testing.T) {
	app, count, ref := chat(t, 30)
	count.Set(20)
	app.Settle()
	if b := bottomOf(app, 19); b != 400 {
		t.Fatalf("after rows went the last ends at %v", b)
	}
	ref.ScrollTo(core.Point{Y: 100}, false)
	app.Settle()
	ref.ScrollToEnd(false)
	app.Settle()
	count.Set(21)
	app.Settle()
	if b := bottomOf(app, 20); b != 400 {
		t.Errorf("back at the end, a new row ends at %v", b)
	}
}

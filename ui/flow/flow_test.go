package flow

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestCardSlots(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return Card{Title: "Replaced title",
			Header: core.Text{Content: "Status"}, Leading: core.Text{Content: "Icon"}, Body: core.Text{Content: "Custom body"},
			Trailing: core.Text{Content: "Badge"}, Footer: core.Text{Content: "Counts"}}
	}, nsgotest.Size(400, 300))
	if app.Exists("text:Replaced title") {
		t.Fatal("the title shows beside a body slot")
	}
	var boxes []driver.Point
	for _, s := range []string{"Status", "Icon", "Custom body", "Badge", "Counts"} {
		var at driver.Point
		for v := app.Find("text:" + s); v != nil; v = v.Parent {
			at.X, at.Y = at.X+v.Frame.Origin.X, at.Y+v.Frame.Origin.Y
		}
		boxes = append(boxes, at)
	}
	if !(boxes[0].Y < boxes[2].Y && boxes[2].Y < boxes[4].Y) || !(boxes[1].X < boxes[2].X && boxes[2].X < boxes[3].X) {
		t.Fatalf("rows out of place: %v", boxes)
	}
	if w := app.Find("text:Status").Parent.Parent.Frame.Size.Width; w != 240 {
		t.Fatalf("the card is %v wide", w)
	}
}

func TestViewShowsCardsAndEmptyText(t *testing.T) {
	var state State
	var data *core.Signal[Data]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		data = core.NewSignalFunc(cx, Data{Nodes: []Node{
			{ID: "a", Label: "Survey", Content: Card{Title: "Survey", Description: "Collect requirements"}},
			{ID: "b", Label: "Design", Content: Card{Title: "Design", Variant: CardAccent}},
		}, Edges: []Edge{{Source: "a", Target: "b", Label: "informs"}}}, func(a, b Data) bool { return false })
		return View{Data: data.Get, State: &state, EmptyText: "Nothing planned", Style: core.Style{Width: core.Pt(720), Height: core.Pt(480)}}
	}, nsgotest.Size(720, 480), nsgotest.Environment(driver.Environment{ReduceMotion: true, Idiom: driver.IdiomDesktop}))
	app.Settle()
	_, a, _ := state.Position("a")
	_, b, _ := state.Position("b")
	if !state.Ready() || a >= b || !app.Exists("text:Collect requirements") {
		t.Fatalf("diagram not laid out: ready %v, %v above %v\n%s", state.Ready(), a, b, app.Dump())
	}
	app.Post(func() { data.Set(Data{}) })
	app.Settle()
	if !app.Exists("text:Nothing planned") {
		t.Fatalf("empty text is missing\n%s", app.Dump())
	}
}

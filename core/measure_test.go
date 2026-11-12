package core

import (
	"fmt"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// countingEngine counts what its prepared texts measure.
type countingEngine struct{ measures *int }

func (e countingEngine) Prepare(t *driver.AttributedText) driver.PreparedText {
	return countingText{ApproxTextEngine{}.Prepare(t), e.measures}
}

type countingText struct {
	driver.PreparedText
	measures *int
}

func (c countingText) Measure(width Points) Size {
	*c.measures++
	return c.PreparedText.Measure(width)
}

func TestIdenticalTextsMeasureOnce(t *testing.T) {
	measures := 0
	app := func(cx *Scope) Node {
		var rows []Node
		for range 20 {
			rows = append(rows, View{Style: Style{Direction: Row}, Children: []Node{Text{Content: "same words"}}})
		}
		return View{Children: rows}
	}
	h := newHost(t, Size{Width: 300, Height: 900}, nil)
	h.tree = NewTree(h.root, countingEngine{&measures})
	h.tree.Resize(Size{Width: 300, Height: 900})
	h.tree.Mount(app)
	h.commit()
	one := measures
	h2 := newHost(t, Size{Width: 300, Height: 900}, nil)
	measures = 0
	h2.tree = NewTree(h2.root, countingEngine{&measures})
	h2.tree.Resize(Size{Width: 300, Height: 900})
	h2.tree.Mount(func(cx *Scope) Node {
		return View{Children: []Node{View{Style: Style{Direction: Row}, Children: []Node{Text{Content: "same words"}}}}}
	})
	h2.commit()
	if one != measures {
		t.Errorf("20 identical texts measured %d times, one %d", one, measures)
	}
}

func TestAColorChangeLaysNothingOut(t *testing.T) {
	var red *Signal[bool]
	h := newHost(t, Size{Width: 300, Height: 300}, func(cx *Scope) Node {
		red = NewSignal(cx, false)
		return View{Children: []Node{Text{TestID: "text", Content: "a price", StyleFunc: func() Style {
			if red.Get() {
				return Style{Color: RGB(0xff0000)}
			}
			return Style{}
		}}}}
	})
	h.commit()
	red.Set(true)
	h.resolved()
	if h.tree.view.layoutDirty || h.find("text").layoutDirty {
		t.Error("a text that changed color marked layout")
	}
	if got := kinds(h.commit()); got[driver.ItemText] != 1 || len(got) != 1 {
		t.Errorf("items %v, want the text's new lines alone", got)
	}
}

func TestTheMeasureCacheIsBounded(t *testing.T) {
	c := newMeasureCache()
	p := ApproxTextEngine{}.Prepare(&driver.AttributedText{Text: "x", Style: driver.TextStyle{Font: Font{Size: 10}}})
	for i := range measureCacheSize * 3 {
		c.size(c.hash(&driver.AttributedText{Text: fmt.Sprint(i)}), p, 100)
	}
	if n := len(c.now) + len(c.old); n > 2*measureCacheSize {
		t.Errorf("the cache holds %d sizes", n)
	}
}

func TestApproxLayoutsHitTest(t *testing.T) {
	p := ApproxTextEngine{}.Prepare(&driver.AttributedText{Text: "ab😀 cd", Style: driver.TextStyle{Font: Font{Size: 10}}})
	l := p.Layout(Size{Width: 24, Height: 24}).(*ApproxLayout)
	// "ab😀" on the first line, 6 points a character; the emoji is two
	// UTF-16 units.
	if c := l.HitTest(Point{X: 17, Y: 2}); c.Offset != 4 || c.Affinity != Downstream {
		t.Fatalf("hit %+v, want after the emoji", c)
	}
	if r, ok := l.CaretRect(Caret{Offset: 2}); !ok || r.Origin.X != 12 {
		t.Fatalf("caret %v %v, want 12", r, ok)
	}
	if c := l.HitTest(Point{X: 1, Y: 15}); c.Offset != 5 {
		t.Fatalf("second line hit %+v, want cd's start", c)
	}
	if got := l.RangeRects(1, 6, nil); len(got) != 2 || got[0].Origin.X != 6 || got[1].Size.Width != 6 {
		t.Fatalf("range boxes %v", got)
	}
}

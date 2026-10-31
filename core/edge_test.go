package core

import (
	"slices"
	"testing"

	"github.com/oneo-me/nsgo/internal/driver"
)

func scrollConfig(b *driver.MountBatch, id driver.NodeID) *driver.ScrollConfig {
	for _, it := range b.Items {
		if it.Kind == driver.ItemScrollConfig && it.ID == id {
			return it.Scroll
		}
	}
	return nil
}

// A scroll view's edges reach as far as the bars over them, which it
// names, and fade into the background its content scrolls over.
func TestScrollEdgesFollowTheirBars(t *testing.T) {
	scroll := &Ref{}
	var tall, shown *Signal[bool]
	h := newHost(t, Size{Width: 200, Height: 300}, func(cx *Scope) Node {
		tall, shown = NewSignal(cx, false), NewSignal(cx, true)
		pinned := func(top bool) Style {
			s := Style{Position: Absolute, Left: Pt(0), Right: Pt(0), Height: Pt(40)}
			if top {
				s.Top = Pt(0)
			} else {
				s.Bottom = Pt(0)
			}
			return s
		}
		return View{Style: Style{FlexGrow: 1, Background: RGB(0x102030)}, Children: []Node{
			Scroll{Ref: scroll, TestID: "scroll", Style: Style{FlexGrow: 1},
				EdgeEffects: EdgeEffects{Top: EdgeEffectSoft, Bottom: EdgeEffectHard, Left: EdgeEffectHidden},
				Children:    []Node{View{Style: Style{Height: Pt(1000)}}}},
			Show{When: shown.Get, Then: func(*Scope) Node {
				return View{TestID: "top", EdgeBar: EdgeBar{Scroll: scroll, Side: SideTop}, StyleFunc: func() Style {
					s := pinned(true)
					if tall.Get() {
						s.Height = Pt(64)
					}
					return s
				}}
			}},
			View{TestID: "bottom", EdgeBar: EdgeBar{Scroll: scroll, Side: SideBottom}, Style: pinned(false)},
		}}
	})
	b := h.commit()
	id := h.find("scroll").id
	c := scrollConfig(b, id)
	if c == nil {
		t.Fatal("no scroll config")
	}
	top, bottom := c.Edges[driver.SideTop], c.Edges[driver.SideBottom]
	if top.Effect != EdgeEffectSoft || top.Extent != 40 || !slices.Equal(top.Bars, []driver.NodeID{h.find("top").id}) ||
		top.Tint != RGB(0x102030) || top.Line == 0 {
		t.Errorf("top edge = %+v", top)
	}
	if bottom.Effect != EdgeEffectHard || bottom.Extent != 40 || len(bottom.Bars) != 1 {
		t.Errorf("bottom edge = %+v", bottom)
	}
	if left := c.Edges[driver.SideLeft]; left.Effect != EdgeEffectHidden || left.Extent != 0 || left.Drawn() != EdgeEffectHidden {
		t.Errorf("left edge = %+v", left)
	}
	if top.Band() != 40+driver.SoftEdgeTail || bottom.Band() != 40 {
		t.Errorf("bands = %v, %v", top.Band(), bottom.Band())
	}

	tall.Set(true)
	if c := scrollConfig(h.commit(), id); c == nil || c.Edges[driver.SideTop].Extent != 64 {
		t.Fatalf("a taller bar sent %+v", c)
	}
	shown.Set(false)
	if c := scrollConfig(h.commit(), id); c == nil || c.Edges[driver.SideTop].Extent != 0 || c.Edges[driver.SideTop].Bars != nil ||
		c.Edges[driver.SideBottom].Extent != 40 {
		t.Fatalf("a removed bar sent %+v", c)
	}
	if c := scrollConfig(h.commit(), id); c != nil {
		t.Errorf("an unchanged edge was sent again: %+v", c.Edges)
	}
}

// A soft edge's blur levels double toward the side, each fading out
// where the next takes over, and add up to the whole blur.
func TestSoftEdgeLevels(t *testing.T) {
	levels := driver.SoftEdgeLevels()
	if len(levels) < 3 {
		t.Fatalf("levels = %+v", levels)
	}
	var variance float64
	for i, l := range levels {
		variance += float64(l.Blur * l.Blur)
		if d := variance - float64(l.Total*l.Total); d > 1e-3 || d < -1e-3 {
			t.Errorf("level %d: blurs add up to %v, not %v", i, variance, l.Total*l.Total)
		}
		if l.Full >= l.Gone || i > 0 && l.Gone != levels[i-1].Full {
			t.Errorf("level %d spans %v to %v", i, l.Full, l.Gone)
		}
	}
	if last := levels[len(levels)-1]; last.Total != driver.SoftEdgeBlur || last.Full != 0 || levels[0].Gone != 1 {
		t.Errorf("levels = %+v", levels)
	}
}

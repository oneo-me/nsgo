package core_test

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// rowText is the text of row i in these tests.
func rowText(i int) string { return fmt.Sprintf("<%d>", i) }

// rows is a Row func of texts h(i) tall.
func rows(h func(i int) core.Points) func(*core.Scope, core.ListRow) core.Node {
	return func(cx *core.Scope, r core.ListRow) core.Node {
		return core.Text{ContentFunc: func() string { return rowText(r.Index()) },
			StyleFunc: func() core.Style { return core.Style{Height: core.Pt(h(r.Index()))} }}
	}
}

func fixed(h core.Points) func(int) core.Points { return func(int) core.Points { return h } }

// cell returns the cell of row i, and false while it is not mounted.
func cell(a *nsgotest.App, i int) (*nsgotest.View, bool) {
	vs := a.Query("text:" + rowText(i))
	if len(vs) == 0 {
		return nil, false
	}
	return vs[0].Parent, true
}

// onScreen is where row i is in list's viewport, along its axis.
func onScreen(t *testing.T, a *nsgotest.App, list *nsgotest.View, i int) core.Points {
	t.Helper()
	c, ok := cell(a, i)
	if !ok {
		t.Fatalf("row %d is not mounted\n%s", i, a.Dump())
	}
	return c.Frame.Origin.Y - list.Offset.Y + list.StickyShift(c.ID)
}

func TestKeyedListKeepsTheVisibleRowsInPlace(t *testing.T) {
	var ids *core.Signal[[]int]
	a := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		ids = core.NewSignalFunc(cx, makeIDs(100, 0), nil)
		return core.List{TestID: "list", Style: core.Style{Height: core.Pt(400)},
			Count: func() int { return len(ids.Get()) }, Key: func(i int) any { return ids.Get()[i] },
			EstimatedSize: 40, Row: func(cx *core.Scope, r core.ListRow) core.Node {
				return core.Text{ContentFunc: func() string { return rowText(ids.Get()[r.Index()]) }, Style: core.Style{Height: core.Pt(40)}}
			}}
	})
	list := a.Find("id:list")
	a.ScrollTo("id:list", core.Point{Y: 1000})
	before := onScreen(t, a, list, 30)
	// Five rows arrive above the viewport.
	ids.Set(append(makeIDs(5, 1000), ids.Get()...))
	a.Settle()
	if got := onScreen(t, a, list, 30); got != before {
		t.Errorf("row 30 moved from %v to %v on screen after inserting above it", before, got)
	}
	if list.Offset.Y != 1200 {
		t.Errorf("offset %v, want 1200", list.Offset.Y)
	}
	// Removing rows below moves nothing.
	ids.Set(slices.Delete(slices.Clone(ids.Get()), 90, 95))
	a.Settle()
	if got := onScreen(t, a, list, 30); got != before {
		t.Errorf("row 30 moved to %v after removing rows below it", got)
	}
}

func makeIDs(n, from int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = from + i
	}
	return out
}

func TestListColumnsShareRows(t *testing.T) {
	a := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.List{TestID: "list", Style: core.Style{Height: core.Pt(100), Width: core.Pt(320)},
			Count: func() int { return 10 }, NumColumns: 3, ColumnGap: 10, Gap: 5,
			Row: rows(func(i int) core.Points { return core.Points(30 + 10*(i%3)) })}
	})
	c4, _ := cell(a, 4)
	c5, _ := cell(a, 5)
	c6, _ := cell(a, 6)
	// Rows of 50 (the tallest of three) and a gap of 5; columns of 100.
	if c4.Frame.Origin != (core.Point{X: 110, Y: 55}) || c4.Frame.Size.Width != 100 {
		t.Errorf("item 4 at %+v", c4.Frame)
	}
	if c5.Frame.Origin.X != 220 || c6.Frame.Origin != (core.Point{X: 0, Y: 110}) {
		t.Errorf("items 5 and 6 at %+v and %+v", c5.Frame, c6.Frame)
	}
	if list := a.Find("id:list"); list.Content.Height != 3*55+30 {
		t.Errorf("content %v, want three rows of 50 and one of 30", list.Content.Height)
	}
}

func TestInvertedListStartsAtTheEndAndKeepsItsDistanceFromIt(t *testing.T) {
	var n *core.Signal[int]
	a := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		n = core.NewSignal(cx, 100)
		return core.List{TestID: "list", Style: core.Style{Height: core.Pt(400)}, Inverted: true,
			Count: n.Get, EstimatedSize: 50, Row: rows(fixed(50))}
	})
	list := a.Find("id:list")
	if list.Offset.Y != 5000-400 {
		t.Fatalf("starts at %v, want the end", list.Offset.Y)
	}
	if got := onScreen(t, a, list, 0); got != 350 {
		t.Errorf("row 0 at %v on screen, want at the bottom", got)
	}
	// Older rows load: what shows stays.
	n.Set(120)
	a.Settle()
	if got := onScreen(t, a, list, 0); got != 350 {
		t.Errorf("row 0 at %v after loading older rows", got)
	}
	// A short list sits at the bottom.
	b := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.List{TestID: "short", Style: core.Style{Height: core.Pt(400)}, Inverted: true,
			Count: func() int { return 2 }, Row: rows(fixed(50))}
	})
	if got := onScreen(t, b, b.Find("id:short"), 1); got != 300 {
		t.Errorf("row 1 of 2 at %v, want 300", got)
	}
}

func TestInvertedListLoadingAtItsStartKeepsTheVisibleRowsAwayFromIt(t *testing.T) {
	var ids *core.Signal[[]int]
	a := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		ids = core.NewSignalFunc(cx, makeIDs(100, 0), nil)
		return core.List{TestID: "list", Style: core.Style{Height: core.Pt(400)}, Inverted: true,
			Count: func() int { return len(ids.Get()) }, Key: func(i int) any { return ids.Get()[i] },
			EstimatedSize: 50, OnLoadMoreAtStart: func() {},
			Row: func(cx *core.Scope, r core.ListRow) core.Node {
				return core.Text{ContentFunc: func() string { return rowText(ids.Get()[r.Index()]) }, Style: core.Style{Height: core.Pt(50)}}
			}}
	})
	list := a.Find("id:list")
	// At its start, a newer row shows at the bottom.
	ids.Set(append(makeIDs(1, 1000), ids.Get()...))
	a.Settle()
	if got := onScreen(t, a, list, 1000); got != 350 {
		t.Errorf("the newest row at %v on screen, want at the bottom", got)
	}
	// Scrolled away from it, newer rows arrive out of sight.
	a.ScrollTo("id:list", core.Point{Y: list.Offset.Y - 300})
	before := onScreen(t, a, list, 10)
	ids.Set(append(makeIDs(4, 2000), ids.Get()...))
	a.Settle()
	if got := onScreen(t, a, list, 10); got != before {
		t.Errorf("row 10 moved from %v to %v on screen as newer rows arrived", before, got)
	}
}

func TestListFollowsItsEndWithinTheThreshold(t *testing.T) {
	var last *core.Signal[core.Points]
	a := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		last = core.NewSignal[core.Points](cx, 50)
		return core.List{TestID: "list", Style: core.Style{Height: core.Pt(400)},
			MaintainVisibleContentPosition: &core.VisibleContentPosition{AutoscrollToBottomThreshold: 20},
			Count:                          func() int { return 20 }, Row: rows(func(i int) core.Points {
				if i == 19 {
					return last.Get()
				}
				return 50
			})}
	})
	a.Find("id:list").Applied = 0
	ref := a.Find("id:list")
	a.ScrollTo("id:list", core.Point{Y: 600})
	last.Set(90)
	a.Settle()
	if ref.Offset.Y != 640 {
		t.Errorf("at the end, the last row grew and the list is at %v, want 640", ref.Offset.Y)
	}
	a.ScrollTo("id:list", core.Point{Y: 100})
	last.Set(150)
	a.Settle()
	if ref.Offset.Y != 100 {
		t.Errorf("scrolled up, the list moved to %v", ref.Offset.Y)
	}
}

func TestStickyHeadersArePinnedNatively(t *testing.T) {
	a := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.List{TestID: "list", Style: core.Style{Height: core.Pt(400)},
			StickyHeaderIndices: []int{0, 30}, StickyHeaderOffset: 20,
			Count: func() int { return 100 }, Row: rows(fixed(50))}
	})
	list := a.Find("id:list")
	c0, _ := cell(a, 0)
	if i := slices.IndexFunc(list.Sticky, func(c core.StickyCell) bool { return c.Cell == c0.ID }); i < 0 ||
		list.Sticky[i].Start != -20 || list.Sticky[i].End != 1430 {
		t.Fatalf("sticky cells %+v, want row 0 from -20 to 1430", list.Sticky)
	}
	// Scrolled far past it, row 0 stays mounted while pinned.
	a.ScrollTo("id:list", core.Point{Y: 1000})
	if got := onScreen(t, a, list, 0); got != 20 {
		t.Errorf("row 0 at %v on screen, want pinned 20 below the top", got)
	}
	// Row 30 pushes it out and takes its place.
	a.ScrollTo("id:list", core.Point{Y: 1600})
	if got := onScreen(t, a, list, 30); got != 20 {
		t.Errorf("row 30 at %v, want pinned", got)
	}
	// Sticky cells draw above the others.
	c30, _ := cell(a, 30)
	if kids := list.Children; kids[len(kids)-1] != c30 && kids[len(kids)-2] != c30 {
		t.Errorf("row 30's cell is not on top")
	}
}

func TestListSeparatorsGoBetweenRows(t *testing.T) {
	a := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.List{TestID: "list", Style: core.Style{Height: core.Pt(400)},
			Count: func() int { return 3 }, Row: rows(fixed(40)),
			Separator: func(*core.Scope) core.Node {
				return core.View{TestID: "sep", Style: core.Style{Height: core.Pt(1)}}
			}}
	})
	if n := len(a.Query("id:sep")); n != 2 {
		t.Errorf("%d separators, want 2", n)
	}
	if c, _ := cell(a, 2); c.Frame.Origin.Y != 82 {
		t.Errorf("row 2 at %v, want after two rows and separators", c.Frame.Origin.Y)
	}
}

func TestListStartsAtItsInitialIndexAndFollowsScrollTargets(t *testing.T) {
	ref := &core.Ref{}
	a := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.List{TestID: "list", Ref: ref, Style: core.Style{Height: core.Pt(400)},
			Count: func() int { return 1000 }, EstimatedSize: 50, InitialScrollIndex: 40,
			// Rows are 30, not the estimated 50.
			Row: rows(fixed(30))}
	})
	list := a.Find("id:list")
	if got := onScreen(t, a, list, 40); got != 0 {
		t.Errorf("row 40 at %v, want at the top", got)
	}
	ref.ScrollToIndexAt(500, 0.5, 0, false)
	a.Settle()
	if got := onScreen(t, a, list, 500); got != 185 {
		t.Errorf("row 500 at %v, want in the middle (185)", got)
	}
	// Rows never shown keep their estimate, so the end is where those
	// estimates put it.
	ref.ScrollToEnd(false)
	a.Settle()
	if want := list.Content.Height - 400; list.Offset.Y != want {
		t.Errorf("at the end: %v, want %v", list.Offset.Y, want)
	}
	if got := onScreen(t, a, list, 999); got != 370 {
		t.Errorf("row 999 at %v, want at the bottom", got)
	}
}

func TestListReportsViewableItems(t *testing.T) {
	var got []core.ViewableItems
	a := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.List{TestID: "list", Style: core.Style{Height: core.Pt(400)},
			Count: func() int { return 100 }, Row: rows(fixed(100)), Key: func(i int) any { return i * 10 },
			ViewabilityConfig:     core.ViewabilityConfig{ItemVisiblePercentThreshold: 60, MinimumViewTime: 100 * time.Millisecond},
			OnViewableItemsChange: func(v core.ViewableItems) { got = append(got, v) }}
	})
	if len(got) != 0 {
		t.Fatalf("reported before the minimum view time: %+v", got)
	}
	a.Advance(100 * time.Millisecond)
	if len(got) != 1 || len(got[0].Viewable) != 4 || got[0].Viewable[3].Key != 30 {
		t.Fatalf("reports %+v, want rows 0 to 3", got)
	}
	// Scrolled by 50: rows 0 and 4 are half visible, under 60%.
	a.ScrollTo("id:list", core.Point{Y: 50})
	a.Advance(100 * time.Millisecond)
	if len(got) != 2 || got[1].Viewable[0].Index != 1 || len(got[1].Viewable) != 3 ||
		len(got[1].Changed) != 1 || got[1].Changed[0] != (core.ViewToken{Index: 0, Key: 0}) {
		t.Errorf("after scrolling: %+v", got[len(got)-1])
	}
}

func TestListRefreshesAndPassesScrollProps(t *testing.T) {
	var refreshing *core.Signal[bool]
	var drags int
	a := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		refreshing = core.NewSignal(cx, false)
		return core.List{TestID: "list", Style: core.Style{Height: core.Pt(400)},
			Count: func() int { return 100 }, Row: rows(fixed(50)),
			Behavior:  core.ScrollBehavior{SnapToInterval: 50},
			OnRefresh: func() { refreshing.Set(true) }, Refreshing: refreshing.Get,
			OnScrollDragStart: func(core.ScrollEvent) { drags++ }}
	})
	list := a.Find("id:list")
	if list.Scroll.Behavior.SnapToInterval != 50 || !list.Scroll.Refreshable {
		t.Fatalf("scroll config %+v", list.Scroll)
	}
	a.PullToRefresh("id:list")
	if !list.Scroll.Refreshing || drags != 1 {
		t.Errorf("refreshing %v after a pull, %d drags", list.Scroll.Refreshing, drags)
	}
}

func TestListInItsParentsViewport(t *testing.T) {
	ref := &core.Ref{}
	a := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.Scroll{TestID: "page", Style: core.Style{Height: core.Pt(400)}, Children: []core.Node{
			core.View{Style: core.Style{Height: core.Pt(300)}},
			core.List{TestID: "list", Ref: ref, Viewport: core.ListViewportParent, Count: func() int { return 1000 },
				StickyHeaderIndices: []int{0}, Row: rows(fixed(50))},
			core.View{TestID: "after", Style: core.Style{Height: core.Pt(100)}},
		}}
	})
	page, list := a.Find("id:page"), a.Find("id:list")
	if list.Kind != driver.KindView || list.Frame.Size.Height != 50000 {
		t.Fatalf("list is a %v %v long, want a plain view at its full length", list.Kind, list.Frame.Size.Height)
	}
	if page.Content.Height != 300+50000+100 {
		t.Errorf("page content %v", page.Content.Height)
	}
	// Rows 0 to 15 or so: the viewport below the header and its overscan.
	if _, ok := cell(a, 0); !ok {
		t.Fatal("row 0 is not mounted")
	}
	if _, ok := cell(a, 40); ok {
		t.Error("row 40 is mounted at the top of the page")
	}
	a.ScrollTo("id:page", core.Point{Y: 300 + 2000})
	if _, ok := cell(a, 40); !ok {
		t.Error("row 40 is not mounted after scrolling the page to it")
	}
	// The sticky row sticks to the page's viewport, as far as the list's end.
	if len(page.Sticky) != 1 || page.Sticky[0].Start != 300 || page.Sticky[0].End != 300+50000-50 {
		t.Errorf("page sticky cells %+v", page.Sticky)
	}
	// Scroll commands scroll the page.
	ref.ScrollToIndex(100, false)
	a.Settle()
	if page.Offset.Y != 300+5000 {
		t.Errorf("page at %v after ScrollToIndex(100), want 5300", page.Offset.Y)
	}
}

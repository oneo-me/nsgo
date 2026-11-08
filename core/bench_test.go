package core

import (
	"fmt"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
)

// The commit's benchmarks, NSH's bench_view_tick, bench_high_frequency
// and list scrolling, on the approximate text engine:
//
//	go tool nsgo test -bench . ./core -- -benchmem

// benchTree mounts app in a size surface and commits it once, without
// the tests' check of every commit.
func benchTree(b *testing.B, size Size, app func(cx *Scope) Node) *Tree {
	check := driver.CheckCommits
	driver.CheckCommits = false
	b.Cleanup(func() { driver.CheckCommits = check })
	root := NewRoot(RootOptions{Post: func(func()) {}, Clock: NewManualClock(time.Unix(0, 0)), RequestFrame: func() {}})
	tree := NewTree(root, ApproxTextEngine{})
	tree.Resize(size)
	tree.Mount(app)
	tree.Commit()
	return tree
}

// commitEach runs change, then a commit, b.N times, reporting the mount
// items per commit.
func commitEach(b *testing.B, tree *Tree, change func(i int)) {
	b.Helper()
	items := 0
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		change(i)
		if batch := tree.Commit(); batch != nil {
			items += len(batch.Items)
		}
	}
	b.ReportMetric(float64(items)/float64(b.N), "items/op")
}

// BenchmarkViewTick is a view of 300 static rows and a counter text, the
// counter changing on each commit.
func BenchmarkViewTick(b *testing.B) {
	var tick *Signal[int]
	tree := benchTree(b, Size{Width: 400, Height: 800}, func(cx *Scope) Node {
		tick = NewSignal(cx, 0)
		kids := []Node{Text{ContentFunc: func() string { return fmt.Sprintf("tick %d", tick.Get()) }}}
		for i := range 300 {
			kids = append(kids, View{Style: Style{Direction: Row, Padding: All(8), Gap: 8}, Children: []Node{
				View{Style: Style{Width: Pt(20), Height: Pt(20), Background: RGB(0x3366ff)}},
				Text{Content: fmt.Sprintf("row %d", i), Style: Style{FontSize: 14}},
			}})
		}
		return View{Children: kids}
	})
	commitEach(b, tree, func(i int) { tick.Set(i + 1) })
}

// board is NSH's quote board: 80 rows of a mark and five cells, the
// first cells of 40 rows changing together on each tick. Fixed cells
// have a size of their own; widening values change width every tick.
func board(fixed, widening bool) (func(cx *Scope) Node, func(i int)) {
	const rows, ticking = 80, 40
	var ticks []*Signal[int]
	var root *Scope
	cell := func(content Text) Node {
		s := Style{Padding: All(4)}
		if fixed {
			s.Width, s.Height = Pt(56), Pt(24)
		}
		return View{Style: s, Children: []Node{content}}
	}
	app := func(cx *Scope) Node {
		ticks, root = nil, cx
		var kids []Node
		for row := range rows {
			var first Text
			if row < ticking {
				tick := NewSignal(cx, 0)
				ticks = append(ticks, tick)
				first = Text{ContentFunc: func() string {
					t := tick.Get()
					sign := ""
					if widening && t%2 == 1 {
						sign = "+"
					}
					return fmt.Sprintf("%s%d.%02d", sign, 9+t/3, t%100)
				}}
			} else {
				first = Text{Content: fmt.Sprintf("%d.00", row)}
			}
			cells := []Node{View{Style: Style{Width: Pt(12), Height: Pt(12), Background: RGB(0x22aa66)}}, cell(first)}
			for col := 1; col < 5; col++ {
				cells = append(cells, cell(Text{Content: fmt.Sprintf("r%d c%d", row, col)}))
			}
			kids = append(kids, View{Style: Style{Direction: Row, AlignItems: AlignCenter, Padding: All(4), Gap: 8},
				Children: cells})
		}
		return View{Children: kids}
	}
	change := func(int) {
		root.Batch(func() {
			for _, t := range ticks {
				t.Set(t.GetUntracked() + 7)
			}
		})
	}
	return app, change
}

func BenchmarkBoard(b *testing.B) {
	for _, c := range []struct {
		name            string
		fixed, widening bool
	}{{"fixed", true, false}, {"content", false, false}, {"widening", false, true}} {
		b.Run(c.name, func(b *testing.B) {
			app, change := board(c.fixed, c.widening)
			commitEach(b, benchTree(b, Size{Width: 600, Height: 2400}, app), change)
		})
	}
}

// BenchmarkListScroll scrolls a list of 1000 rows of an avatar, two texts
// and a bound color 10 points a commit.
func BenchmarkListScroll(b *testing.B) {
	var shade *Signal[int]
	tree := benchTree(b, Size{Width: 390, Height: 844}, func(cx *Scope) Node {
		shade = NewSignal(cx, 0)
		return List{Count: func() int { return 1000 }, EstimatedSize: 76, Style: Style{FlexGrow: 1},
			Row: func(cx *Scope, r ListRow) Node {
				return View{Style: Style{Direction: Row, Padding: All(12), Gap: 12, Height: Pt(76)}, Children: []Node{
					View{StyleFunc: func() Style {
						return Style{Width: Pt(52), Height: Pt(52), CornerRadius: 26, Background: RGB(uint32(0x204080 + shade.Get() + r.Index()))}
					}},
					View{Style: Style{FlexGrow: 1}, Children: []Node{
						Text{ContentFunc: func() string { return fmt.Sprintf("Row %d", r.Index()) }, Style: Style{FontSize: 16}},
						Text{ContentFunc: func() string { return fmt.Sprintf("Subtitle of row %d", r.Index()) }, Style: Style{FontSize: 13}},
					}},
				}}
			}}
	})
	var list *node
	for _, n := range tree.nodes {
		if n != nil && n.list != nil && n.scroll != nil {
			list = n
		}
	}
	y := Points(0)
	commitEach(b, tree, func(int) {
		y += 10
		if y > 60000 {
			y = 0
		}
		tree.Dispatch(driver.Event{Kind: driver.EventScroll, ID: list.id, Point: Point{Y: y}})
	})
}

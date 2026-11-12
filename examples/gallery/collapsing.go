package main

import (
	"fmt"

	"github.com/nsgo-dev/nsgo/core"

	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/ui"
)

const (
	collapsingOpen = 220 // how tall the header is open
	collapsingBar  = 56  // the bar it collapses to
	// collapsingRange is how far the page scrolls while the header
	// collapses.
	collapsingRange = collapsingOpen - collapsingBar
)

// collapsingRow is a week's heading, or one of its entries.
type collapsingRow struct {
	week          int
	title, detail string
}

// collapsingDemo is a header that collapses to a bar as the page scrolls,
// as Flutter's SliverAppBar does: it is sticky with a negative Top, so it
// scrolls until its last bar's height shows and then stays; its picture
// moves at half speed, its title shrinks into the bar and its color fills
// in, all on the frame clock from the page's offset. Pulled past the top,
// the picture stretches. The title sits in the header's last bar's
// height, the bar it collapses to, so it is centered there at every step.
// The page's list (ListViewportParent) sticks its week headings just
// below the bar; only the rows near the screen exist, and the footer
// after it lies past its full length.
func collapsingDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	var rows []collapsingRow
	var weeks []int
	for week := 1; week <= 20; week++ {
		weeks = append(weeks, len(rows))
		rows = append(rows, collapsingRow{week: week})
		for i := 1; i <= 8; i++ {
			rows = append(rows, collapsingRow{title: tr.CollapsingEntry(fmt.Sprintf("%d.%d", week, i)), detail: tr.CollapsingMinutes((week*7 + i*3) % 60)})
		}
	}
	scroll := core.NewAnimatedValue(cx, 0)
	// 0 open, 1 collapsed.
	collapsed := core.Interpolate(scroll, core.Interpolation{Input: []float64{0, collapsingRange}, Output: []float64{0, 1},
		Extrapolate: core.ExtrapolateClamp})
	lerp := func(in core.AnimatedNode, from, to [2]float64) core.AnimatedNode {
		return core.Interpolate(in, core.Interpolation{Input: from[:], Output: to[:], Extrapolate: core.ExtrapolateClamp})
	}
	fill := func(top, left, size core.Points, right bool, opacity float32) core.Node {
		s := core.Style{Position: core.Absolute, Top: core.Pt(top), Width: core.Pt(size), Height: core.Pt(size), CornerRadius: size / 2,
			Opacity: core.Alpha(opacity)}
		if right {
			s.Right = core.Pt(left)
		} else {
			s.Left = core.Pt(left)
		}
		return core.View{Style: s, StyleFunc: ui.Filled(colors.OnAccent)}
	}
	header := core.View{TestID: "header", Style: core.Style{Position: core.Sticky, Top: core.Pt(-collapsingRange), Height: core.Pt(collapsingOpen),
		Overflow: core.Hidden}, StyleFunc: ui.Filled(colors.AccentFill), Children: []core.Node{
		core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Right: core.Pt(0), Height: core.Pt(collapsingOpen),
			TransformOrigin: core.ObjectPosition{X: core.PositionCenter, Y: core.PositionEnd},
			Animated: &core.AnimatedStyle{
				TranslateY: lerp(scroll, [2]float64{0, collapsingRange}, [2]float64{0, collapsingRange / 2}),
				Scale:      lerp(scroll, [2]float64{-collapsingOpen, 0}, [2]float64{2, 1}),
			}}, Children: []core.Node{
			fill(24, 36, 120, false, 0.14),
			fill(70, 48, 72, true, 0.1),
		}},
		core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Animated: &core.AnimatedStyle{Opacity: collapsed}}, StyleFunc: ui.Filled(colors.AccentFill)},
		// Shown while open, above the bar's strip: it fades as the header
		// collapses.
		core.Text{Content: tr.Collapsing160EntriesByWeek(), Style: core.Style{Position: core.Absolute, Left: core.Pt(16), Bottom: core.Pt(collapsingBar - 4), FontSize: 13, Animated: &core.AnimatedStyle{Opacity: lerp(collapsed, [2]float64{0, 0.5}, [2]float64{0.8, 0})}}, StyleFunc: ui.Colored(colors.OnAccent)},
		// The title, centered in the header's last bar's height: the bar it
		// collapses to. It shrinks to the bar's size around its left edge.
		core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(16), Right: core.Pt(16), Bottom: core.Pt(0), Height: core.Pt(collapsingBar),
			JustifyContent: core.JustifyCenter}, Children: []core.Node{
			core.Text{Content: tr.CollapsingRecent(), Style: core.Style{AlignSelf: core.AlignStart, FontSize: 30, FontWeight: ui.Bold,
				TransformOrigin: core.ObjectPosition{X: core.PositionStart, Y: core.PositionCenter},
				Animated:        &core.AnimatedStyle{Scale: lerp(collapsed, [2]float64{0, 1}, [2]float64{1, 0.62})}}, StyleFunc: ui.Colored(colors.OnAccent)},
		}},
	}}
	list := core.List{Viewport: core.ListViewportParent, Count: func() int { return len(rows) }, EstimatedSize: 52,
		StickyHeaderIndices: weeks, StickyHeaderOffset: collapsingBar,
		Row: func(cx *core.Scope, r core.ListRow) core.Node {
			colors := ui.ColorsOf(cx)
			row := func() collapsingRow { return rows[r.Index()] }
			// A cell shows a heading now and an entry later: Show switches
			// with the row it shows.
			return core.Show{When: func() bool { return row().week > 0 },
				Then: func(*core.Scope) core.Node {
					return core.View{Style: core.Style{Height: core.Pt(34), JustifyContent: core.JustifyCenter, Padding: core.Symmetric(0, 16)}, StyleFunc: ui.Filled(colors.Inset),
						Children: []core.Node{ui.Textf(func() string { return tr.ListsWeek(row().week) }, 13, ui.Bold, colors.Accent)}}
				},
				Else: func(*core.Scope) core.Node {
					return core.View{StyleFunc: func() core.Style {
						return core.Style{Height: core.Pt(52), Direction: core.Row, AlignItems: core.AlignCenter, Padding: core.Symmetric(0, 16),
							Borders: core.Edges{Bottom: 1}, BorderColors: core.EdgeColors{Bottom: colors.Line()}}
					}, Children: []core.Node{
						core.Text{ContentFunc: func() string { return row().title }, Style: core.Style{FlexGrow: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Text)},
						core.Text{ContentFunc: func() string { return row().detail }, Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
					}}
				}}
		}}
	footer := core.View{TestID: "footer", Style: core.Style{AlignItems: core.AlignCenter, Gap: 6, Padding: core.Edges{Top: 32, Right: 16, Bottom: 48, Left: 16}},
		Children: []core.Node{
			ui.Text(tr.CollapsingThatSAll160Entries(), 13, ui.Semibold, colors.Text),
			core.Text{Content: tr.CollapsingAFooterAfterTheList(),
				Style: core.Style{FontSize: 13, TextAlign: core.TextAlignCenter}, StyleFunc: ui.Colored(colors.Muted)},
		}}
	return core.Scroll{TestID: "page", OffsetValues: core.AnimatedXY{Y: scroll}, Style: core.Style{FlexGrow: 1}, StyleFunc: ui.Filled(colors.Background),
		Children: []core.Node{header, list, footer}}
}

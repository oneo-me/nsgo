package main

import (
	"github.com/oneo-me/nsgo/core"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
	"github.com/oneo-me/nsgo/examples/gallery/locales"
)

// layoutDemo is the layout NSGo ports from Taffy: flexbox, wrapping,
// growing and shrinking, positioning, grid, intrinsic sizes, stacking,
// direction and block layout. NSH's gallery has no page of its own for
// it; this one is in its look.
func layoutDemo(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	tints := ui.TintsOf(cx)
	fin := ui.FinOf(cx)
	// paintedBox is a box of style, its colors fill's (raised when nil).
	paintedBox := func(label string, style core.Style, fill func() core.Style) core.Node {
		return core.View{StyleFunc: func() core.Style {
			s := fin.Raised().Over(core.Style{Background: colors.Raised(), BorderWidth: 1, BorderColor: colors.Border(), CornerRadius: 8,
				Padding: core.Symmetric(8, 12), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}).Merge(style)
			// A box of another color drops the raised gradient over it.
			if fill != nil {
				s = s.Merge(fill())
				s.Gradients = nil
			}
			return s
		}, Children: []core.Node{ui.Text(label, 13, ui.Medium, colors.Text)}}
	}
	box := func(label string, style core.Style) core.Node { return paintedBox(label, style, nil) }
	tinted := func() core.Style {
		return core.Style{Background: tints.Accent(), BorderColor: ui.Alpha(colors.Accent(), 0.4)}
	}
	tintedBox := func(label string, style core.Style) core.Node { return paintedBox(label, style, tinted) }
	frame := func(style core.Style, children ...core.Node) core.Node {
		return core.View{StyleFunc: func() core.Style {
			return core.Style{Background: colors.Surface(), BorderWidth: 1, BorderColor: colors.Line(), CornerRadius: 10,
				Padding: core.All(8), Gap: 8}.Merge(style)
		}, Children: children}
	}

	rows := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		var justify []core.Node
		for _, j := range []struct {
			name string
			v    core.Justify
		}{{"JustifyStart", core.JustifyStart}, {"JustifyCenter", core.JustifyCenter}, {"JustifySpaceBetween", core.JustifySpaceBetween}, {"JustifySpaceEvenly", core.JustifySpaceEvenly}} {
			justify = append(justify, ui.API{Text: j.name},
				frame(core.Style{Direction: core.Row, JustifyContent: j.v}, box("A", core.Style{}), box("B", core.Style{}), box("C", core.Style{})))
		}
		justify = append(justify, ui.API{Text: "Column, AlignCenter"},
			frame(core.Style{AlignItems: core.AlignCenter}, box(tr.LayoutOne(), core.Style{}), box(tr.LayoutTwoWider(), core.Style{}), box(tr.LayoutThree(), core.Style{})))
		return ui.Section{Title: tr.LayoutRowAndColumn(), Note: "Direction · JustifyContent", Children: justify}
	}

	wrapCard := func(cx *core.Scope) core.Node {
		var chips []core.Node
		for i, w := range []string{"flex", "wrap", "lays", "children", "onto", "more", "lines", "when", "they", "run", "out", "of", "room"} {
			if i%2 == 1 {
				chips = append(chips, tintedBox(w, core.Style{CornerRadius: 14}))
			} else {
				chips = append(chips, box(w, core.Style{CornerRadius: 14}))
			}
		}
		return ui.Section{Title: "Wrap", Note: "Wrap", Children: []core.Node{frame(core.Style{Direction: core.Row, Wrap: core.Wrap, Gap: 6}, chips...)}}
	}

	grow := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		return ui.Section{Title: tr.LayoutGrowAndShrink(), Note: "FlexGrow · FlexShrink · AutoMargin", Children: []core.Node{
			frame(core.Style{Direction: core.Row}, box("1", core.Style{FlexGrow: 1}), box("2", core.Style{FlexGrow: 2}), box("3", core.Style{FlexGrow: 3})),
			frame(core.Style{Direction: core.Row}, box(tr.LayoutFixed120(), core.Style{Width: core.Pt(120)}), box(tr.Layout900WideShrinksToFit(), core.Style{Width: core.Pt(900), FlexShrink: 1})),
			frame(core.Style{Direction: core.Row}, box(tr.LayoutStart(), core.Style{}), box(tr.LayoutPushedToTheEnd(), core.Style{AutoMargin: core.SideLeft})),
		}}
	}

	absolute := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		return ui.Section{Title: tr.LayoutAbsolutePositioning(), Note: "Position: Absolute", Children: []core.Node{
			frame(core.Style{Height: core.Pt(120)},
				box(tr.LayoutTopLeft(), core.Style{Position: core.Absolute, Top: core.Pt(8), Left: core.Pt(8)}),
				box(tr.LayoutBottomRight(), core.Style{Position: core.Absolute, Bottom: core.Pt(8), Right: core.Pt(8)}),
				tintedBox(tr.LayoutCentered(), core.Style{Position: core.Absolute, Top: core.Pct(50), Left: core.Pct(50),
					Transform: core.Transform{TranslateXPercent: -50, TranslateYPercent: -50}}),
				core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(-6), Right: core.Pt(-6), Width: core.Pt(20), Height: core.Pt(20),
					CornerRadius: 10, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, StyleFunc: ui.Filled(colors.Danger),
					Children: []core.Node{ui.Text("3", 11, ui.Bold, ui.Fixed(core.RGB(0xffffff)))}},
			),
		}}
	}

	grid := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		var cells []core.Node
		for i := 1; i <= 7; i++ {
			cells = append(cells, box(itoa(i), core.Style{Height: core.Pt(44)}))
		}
		return ui.Section{Title: tr.LayoutGrid(), Note: "DisplayGrid", Children: []core.Node{
			ui.API{Text: `GridTemplateColumns: "repeat(3, 1fr)"`},
			core.View{Style: core.Style{Display: core.DisplayGrid, GridTemplateColumns: "repeat(3, 1fr)", GridAutoRows: "44px", Gap: 8}, Children: []core.Node{
				tintedBox(tr.LayoutSpan2(), core.Style{GridColumn: core.GridLines{Start: core.GridSpan(2)}}),
				box("B", core.Style{}), box("C", core.Style{}),
				tintedBox(tr.Layout2Rows(), core.Style{GridRow: core.GridLines{Start: core.GridSpan(2)}}),
				box("E", core.Style{}), box("F", core.Style{}), box("G", core.Style{}), box("H", core.Style{}),
			}},
			ui.API{Text: `"repeat(auto-fill, minmax(80px, 1fr))"`},
			core.View{Style: core.Style{Display: core.DisplayGrid, GridTemplateColumns: "repeat(auto-fill, minmax(80px, 1fr))", Gap: 8}, Children: cells},
		}}
	}

	positions := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		fin := ui.FinOf(cx)
		// Each group holds its rows, so its header stops at the group's
		// end.
		var groups []core.Node
		for _, name := range []string{tr.LayoutFruit(), tr.LayoutVegetables(), tr.LayoutGrains()} {
			g := []core.Node{core.View{StyleFunc: func() core.Style {
				return core.Style{Position: core.Sticky, Top: core.Pt(0), Background: colors.Raised(), Padding: core.Symmetric(6, 10),
					Borders: core.Edges{Bottom: 1}, BorderColors: core.EdgeColors{Bottom: colors.Line()}}
			},
				Children: []core.Node{ui.Text(name, 13, ui.Semibold, colors.Accent)}}}
			for i := 1; i <= 4; i++ {
				g = append(g, core.Text{Content: name + " " + itoa(i), Style: core.Style{Padding: core.Symmetric(7, 10), FontSize: 14}, StyleFunc: ui.Colored(colors.Text)})
			}
			groups = append(groups, core.View{Children: g})
		}
		shown := core.NewSignal(cx, false)
		return ui.Section{Title: tr.LayoutStaticStickyAndFixed(), Note: "Position", Children: []core.Node{
			ui.Note{Text: tr.LayoutStaticIgnoresInsetsAndLets()},
			frame(core.Style{Height: core.Pt(80)},
				core.View{Style: core.Style{Position: core.Static, Top: core.Pt(40), Padding: core.All(4)}, Children: []core.Node{
					box(tr.LayoutAbsoluteInTheFrame(), core.Style{Position: core.Absolute, Bottom: core.Pt(4), Right: core.Pt(4)}),
				}},
			),
			core.Scroll{Style: surfaceBox(cx, 160).Merge(core.Style{Overflow: core.Hidden}), StyleFunc: surfaceInk(cx), Children: groups},
			ui.Button{Text: tr.LayoutShowAFixedNote(), Variant: ui.ButtonSecondary, OnPress: func() { toggled(shown) }},
			core.Show{When: shown.Get, Then: func(*core.Scope) core.Node {
				return core.View{StyleFunc: func() core.Style {
					return core.Style{Position: core.Fixed, Right: core.Pt(24), Bottom: core.Pt(24), Padding: core.Symmetric(10, 14),
						Background: colors.Light(), CornerRadius: ui.RadiusControl, ZIndex: 10, Shadows: fin.Light().Shadows}
				},
					Children: []core.Node{ui.Text(tr.LayoutFixedToTheWindow(), 14, ui.Medium, colors.OnLight)}}
			}},
		}}
	}

	intrinsic := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		return ui.Section{Title: tr.LayoutIntrinsicSizes(), Note: "MinContent · MaxContent · FitContent", Children: []core.Node{
			frame(core.Style{AlignItems: core.AlignStart},
				tintedBox(tr.LayoutMinContentEachWordOn(), core.Style{Width: core.MinContent}),
				tintedBox(tr.LayoutMaxContentOneLine(), core.Style{Width: core.MaxContent}),
				tintedBox(tr.LayoutFitContentOneLineUp(), core.Style{Width: core.FitContent}),
			),
		}}
	}

	stacking := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		return ui.Section{Title: tr.LayoutStacking(), Note: "ZIndex", Children: []core.Node{
			frame(core.Style{Direction: core.Row, Height: core.Pt(90)},
				tintedBox("ZIndex 2", core.Style{Width: core.Pt(100), Height: core.Pt(60), ZIndex: 2}),
				box("0", core.Style{Width: core.Pt(100), Height: core.Pt(60), Margin: core.Edges{Left: -30, Top: 20}}),
				paintedBox("ZIndex 1", core.Style{Width: core.Pt(100), Height: core.Pt(60), Margin: core.Edges{Left: -30}, ZIndex: 1}, ui.Filled(colors.Inset)),
			),
		}}
	}

	direction := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		return ui.Section{Title: tr.LayoutDirection(), Note: "LayoutDirection · WritingDirection", Children: []core.Node{
			frame(core.Style{Direction: core.Row, LayoutDirection: core.DirectionRTL}, box("first", core.Style{}), box("second", core.Style{}), box("third", core.Style{})),
			frame(core.Style{},
				core.Text{Content: "שלום עולם, hello world", Style: core.Style{WritingDirection: core.WritingAuto, FontSize: 15}, StyleFunc: ui.Colored(colors.Text)},
				core.Text{Content: "Hello world, שלום עולם", Style: core.Style{WritingDirection: core.WritingAuto, FontSize: 15}, StyleFunc: ui.Colored(colors.Text)},
			),
		}}
	}

	block := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		return ui.Section{Title: tr.LayoutBlock(), Note: "DisplayBlock", Children: []core.Node{
			frame(core.Style{Display: core.DisplayBlock},
				box(tr.LayoutMarginBottom16(), core.Style{Margin: core.Edges{Bottom: 16}}),
				box(tr.LayoutMarginTop16TheGap(), core.Style{Margin: core.Edges{Top: 16}}),
				box(tr.LayoutWidth60CenteredByAuto(), core.Style{Width: core.Pct(60), AutoMargin: core.SidesHorizontal, Margin: core.Edges{Top: 8}}),
			),
		}}
	}

	return demoPage(cx, pageProps{}, rows, wrapCard, grow, absolute, grid, positions, intrinsic, stacking, direction, block)
}

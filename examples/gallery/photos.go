package main

import (
	"fmt"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"time"

	"github.com/nsgo-dev/nsgo/core"

	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/ui"
)

type photo struct {
	id            int
	title, author string
	tone          Tone
	views         int
}

var (
	photoAuthors = []string{"Ada", "Linus", "Grace", "Ken", "Barbara"}
	photoTones   = []Tone{ToneAccent, ToneInfo, ToneWarn, ToneUp, ToneDanger}
	photoPlaces  = []string{"Harbor", "Ridge", "Old town", "Dunes", "Canal", "Orchard", "Quarry", "Pier", "Meadow"}
)

// photoPage is how many photos a page loads.
const photoPage = 200

func makePhotos(from, n int) []photo {
	out := make([]photo, n)
	for i := range out {
		id := from + i
		out[i] = photo{id, fmt.Sprintf("%s %04d", photoPlaces[id%len(photoPlaces)], id), photoAuthors[id%len(photoAuthors)],
			photoTones[id%len(photoTones)], (id*7919)%9800 + 120}
	}
	return out
}

// compact is 1234 as "1.2k".
func compact(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return itoa(n)
}

// photoRow is a photo in the phone layout's list.
func photoRow(cx *core.Scope, p func() photo) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	return core.View{Role: core.RoleButton, Label: tr.PhotosPhoto(), Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Padding: core.Symmetric(0, 16), Height: core.Pt(76)}, Children: []core.Node{
			core.Image{SourceFunc: func() core.ImageSource { return core.Src(fmt.Sprintf("https://picsum.photos/seed/%d/112/112", p().id)) },
				StyleFunc: func() core.Style {
					return fin.Raised().Over(core.Style{Width: core.Pt(56), Height: core.Pt(56), CornerRadius: ui.RadiusInset, BorderCurve: core.BorderCurveContinuous,
						BorderWidth: 1, BorderColor: colors.Border(), Background: colors.Raised()})
				}},
			core.View{StyleFunc: func() core.Style {
				return core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 4, AlignSelf: core.AlignStretch, JustifyContent: core.JustifyCenter,
					Borders: core.Edges{Bottom: 1}, BorderColors: core.EdgeColors{Bottom: colors.Line()}}
			}, Children: []core.Node{
				core.Text{ContentFunc: func() string { return p().title }, MaxLines: 1, Style: core.Style{FontSize: 13, FontWeight: ui.Semibold}, StyleFunc: ui.Colored(colors.Text)},
				photoByline(cx, p, 13),
			}},
			ui.Textf(func() string { return fmt.Sprintf("#%d", p().id) }, 13, ui.Regular, colors.Faint),
		}},
	}}
}

// photoByline is a photo's tone dot, author and views.
func photoByline(cx *core.Scope, p func() photo, size core.Points) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	return ui.HStack{Gap: 6, Children: []core.Node{
		core.View{StyleFunc: func() core.Style {
			return core.Style{Width: core.Pt(6), Height: core.Pt(6), CornerRadius: 3, Background: p().tone.Color(cx)}
		}},
		core.Text{ContentFunc: func() string { return p().author }, Style: core.Style{FontSize: size}, StyleFunc: ui.Colored(colors.Muted)},
		core.Text{ContentFunc: func() string { return "· " + tr.PhotosViews(compact(p().views)) }, Style: core.Style{FontSize: size}, StyleFunc: ui.Colored(colors.Faint)},
	}}
}

// photoTile is a photo in the desktop layout's grid: the picture over its
// title and author, a fill under the pointer.
func photoTile(cx *core.Scope, p func() photo) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	return core.View{Role: core.RoleButton, Label: tr.PhotosPhoto(), Children: []core.Node{
		core.View{StyleFunc: func() core.Style {
			return core.Style{Padding: core.All(6), Gap: 8, CornerRadius: ui.RadiusCard, BorderCurve: core.BorderCurveContinuous}
		}, Children: []core.Node{
			core.Image{SourceFunc: func() core.ImageSource { return core.Src(fmt.Sprintf("https://picsum.photos/seed/%d/480/360", p().id)) },
				StyleFunc: func() core.Style {
					return fin.Raised().Over(core.Style{Width: core.Pct(100), AspectRatio: 4.0 / 3, CornerRadius: ui.RadiusInset, BorderCurve: core.BorderCurveContinuous,
						BorderWidth: 1, BorderColor: colors.Border(), Background: colors.Raised()})
				}},
			core.View{Style: core.Style{Gap: 3, Padding: core.Edges{Right: 4, Bottom: 4, Left: 4}}, Children: []core.Node{
				core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignBaseline, Gap: 8}, Children: []core.Node{
					core.Text{ContentFunc: func() string { return p().title }, MaxLines: 1, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 13, FontWeight: ui.Semibold}, StyleFunc: ui.Colored(colors.Text)},
					ui.Textf(func() string { return fmt.Sprintf("#%d", p().id) }, 12, ui.Regular, colors.Faint),
				}},
				photoByline(cx, p, 12),
			}},
		}},
	}}
}

const (
	photosOpen = 100 // how tall the header is open
	photosBar  = 44  // the bar it collapses to
	// photosRange is how far the page scrolls while the header collapses.
	photosRange = photosOpen - photosBar
)

// photosDemo is the stream: rows on phones, a grid of as many columns as
// fit (at least 200 pt each) in the desktop layout, a list that recycles
// its cells, pulls to refresh and loads pages as it nears its end. It
// scrolls in its page under a header that collapses to a bar, as the
// Collapsing header demo's does, so the scroll bar runs past it.
func photosDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	wide := shell.desktop()
	photos := core.NewSignalFunc(cx, makePhotos(1, 1000), func(a, b []photo) bool { return false })
	refreshing := core.NewSignal(cx, false)
	refreshes := core.NewSignal(cx, 0)
	// The scroll offset, fed by the platform: the header's collapse and
	// the card's parallax and fade run with no engine work per frame.
	scrollY := core.NewAnimatedValue(cx, 0)
	refresh := func() {
		if refreshing.GetUntracked() {
			return
		}
		refreshing.Set(true)
		cx.AfterFunc(800*time.Millisecond, func() {
			photos.Set(makePhotos(1, 1000))
			refreshes.Update(func(n int) int { return n + 1 })
			refreshing.Set(false)
		})
	}
	refreshKey := BindRefresh(cx, refresh)
	loadMore := func() {
		current := photos.GetUntracked()
		photos.Set(append(current[:len(current):len(current)], makePhotos(len(current)+1, photoPage)...))
	}
	desktopIdiom := cx.Env().Desktop()
	var gutter core.Points = 16
	if wide {
		gutter = 24
	}
	lerp := func(in core.AnimatedNode, from, to [2]float64) core.AnimatedNode {
		return core.Interpolate(in, core.Interpolation{Input: from[:], Output: to[:], Extrapolate: core.ExtrapolateClamp})
	}
	// 0 open, 1 collapsed.
	collapsed := lerp(scrollY, [2]float64{0, photosRange}, [2]float64{0, 1})

	// The header sticks with a negative Top: it scrolls until its last
	// bar's height shows, where the title has shrunk to, and stays.
	header := core.View{TestID: "photos-header", Style: core.Style{Position: core.Sticky, Top: core.Pt(-photosRange), Height: core.Pt(photosOpen),
		ZIndex: 1, Overflow: core.Hidden}, StyleFunc: ui.Filled(colors.Background), Children: []core.Node{
		// Shown while open, above the bar: it fades as the header
		// collapses.
		core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(gutter), Right: core.Pt(gutter), Bottom: core.Pt(photosBar),
			Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Animated: &core.AnimatedStyle{Opacity: lerp(collapsed, [2]float64{0, 0.5}, [2]float64{1, 0})}},
			Children: []core.Node{
				core.Text{Content: tr.PhotosARecycledListOfRemote(), MaxLines: 1, Style: core.Style{FlexShrink: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
				ui.Badge{Text: tr.PhotosRemote(), Variant: ui.BadgeInfo},
			}},
		// The bar: the title, which shrinks into it around its left edge,
		// and the count, which fades in.
		core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(gutter), Right: core.Pt(gutter), Bottom: core.Pt(0), Height: core.Pt(photosBar),
			Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
			core.Text{Content: tr.PhotosPhotoStream(), Role: core.RoleHeader, MaxLines: 1, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 26, FontWeight: ui.Bold,
				TransformOrigin: core.ObjectPosition{X: core.PositionStart, Y: core.PositionCenter},
				Animated:        &core.AnimatedStyle{Scale: lerp(collapsed, [2]float64{0, 1}, [2]float64{1, 0.58})}}, StyleFunc: ui.Colored(colors.Text)},
			core.View{Style: core.Style{Animated: &core.AnimatedStyle{Opacity: lerp(collapsed, [2]float64{0.6, 1}, [2]float64{0, 1})}}, Children: []core.Node{
				ui.Tabular(ui.Textf(func() string { return thousands(len(photos.Get())) + tr.PhotosPhotos() }, 13, ui.Regular, colors.Muted)),
			}},
		}},
		// The bar's hairline, once collapsed.
		core.View{Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), Height: core.Pt(1),
			Animated: &core.AnimatedStyle{Opacity: lerp(collapsed, [2]float64{0.8, 1}, [2]float64{0, 1})}}, StyleFunc: ui.Filled(colors.Line)},
	}}

	hero := func() core.Node {
		hintText := tr.PhotosPullToRefresh(photoPage)
		if desktopIdiom {
			hintText = tr.PhotosLoadsMore(photoPage)
			if refreshKey != "" {
				hintText = refreshKey + tr.PhotosRefreshes() + hintText
			}
		}
		foot := []core.Node{ui.Icon(lucide.ArrowDown, 14, colors.Faint), core.Text{Content: hintText, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Faint)}}
		// The mouse does not pull scroll views down: desktops refresh here.
		if desktopIdiom {
			foot = append(foot, core.Dynamic(func(*core.Scope) core.Node {
				label := tr.PhotosRefresh()
				if refreshing.Get() {
					label = tr.PhotosRefreshing()
				}
				return ui.Button{Text: label, Variant: ui.ButtonSecondary, Disabled: refreshing.Get, OnPress: refresh}
			}))
		}
		return core.View{Style: core.Style{Padding: core.Edges{Top: 8, Right: gutter, Bottom: 16, Left: gutter}, Animated: &core.AnimatedStyle{
			Opacity: lerp(scrollY, [2]float64{photosRange, photosRange + 150}, [2]float64{1, 0}),
			// Moves at half speed once the header has collapsed: the list
			// slides over it.
			TranslateY: core.Interpolate(scrollY, core.Interpolation{Input: []float64{photosRange, photosRange + 300}, Output: []float64{0, 150}, Extrapolate: core.ExtrapolateClamp}),
		}}, Children: []core.Node{
			core.View{StyleFunc: func() core.Style { return ui.CardStyle(cx).Merge(core.Style{Padding: core.All(16), Gap: 12}) }, Children: []core.Node{
				core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12}, Children: []core.Node{
					core.View{StyleFunc: func() core.Style {
						return fin.Raised().Over(core.Style{Width: core.Pt(40), Height: core.Pt(40), CornerRadius: 12, BorderCurve: core.BorderCurveContinuous,
							Background: colors.Raised(), BorderWidth: 1, BorderColor: colors.Border(), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter})
					},
						Children: []core.Node{logo(cx, 24)}},
					core.Text{Spans: []core.Span{
						{ContentFunc: func() string { return thousands(len(photos.Get())) }},
						{Content: tr.PhotosPhotos(), Style: core.Style{FontSize: 13, FontWeight: ui.Medium}, StyleFunc: ui.Colored(colors.Accent)},
					}, Style: core.Style{FontSize: 40, FontWeight: ui.Semibold, LetterSpacing: -1}, StyleFunc: ui.Colored(colors.Text)},
				}},
				core.View{StyleFunc: func() core.Style {
					return core.Style{Direction: core.Row, Borders: core.Edges{Top: 1}, BorderColors: core.EdgeColors{Top: colors.Line()}, Padding: core.Edges{Top: 10}}
				}, Children: []core.Node{
					Stat{Label: tr.PhotosPages(), Value: func() string { return itoa((len(photos.Get()) + photoPage - 1) / photoPage) }, Width: core.Pct(33)},
					Stat{Label: tr.PhotosPerPage(), Value: label(itoa(photoPage)), Width: core.Pct(33)},
					core.Dynamic(func(*core.Scope) core.Node {
						t := ToneNeutral
						if refreshes.Get() > 0 {
							t = ToneAccent
						}
						return Stat{Label: tr.PhotosRefreshes2(), Value: func() string { return itoa(refreshes.Get()) }, Tone: t, Width: core.Pct(33)}
					}),
				}},
			}},
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Padding: core.Edges{Top: 12}}, Children: foot},
		}}
	}
	footer := core.Text{Content: tr.PhotosLoadingMore(photoPage), Style: core.Style{TextAlign: core.TextAlignCenter, FontSize: 13, Padding: core.Edges{Top: 16, Right: 16, Bottom: 40, Left: 16}}, StyleFunc: ui.Colored(colors.Faint)}
	at := func(r core.ListRow) func() photo {
		return func() photo {
			ps := photos.Get()
			if i := r.Index(); i < len(ps) {
				return ps[i]
			}
			return photo{}
		}
	}
	list := core.List{TestID: "photos", Viewport: core.ListViewportParent, Count: func() int { return len(photos.Get()) },
		Key: func(i int) any { return photos.GetUntracked()[i].id }, OnLoadMore: loadMore}
	var body core.Node
	if !wide {
		list.EstimatedSize = 76
		list.Row = func(cx *core.Scope, r core.ListRow) core.Node { return photoRow(cx, at(r)) }
		body = list
	} else {
		columns := core.NewMemo(cx, func() int { return columnsFor(200, 8, shell.paneWidth(), 16) })
		// Built again when the number of columns changes.
		body = core.Dynamic(func(cx *core.Scope) core.Node {
			n := columns.Get()
			return core.Func(func(cx *core.Scope) core.Node {
				l := list
				l.NumColumns = n
				l.EstimatedSize = core.Untracked(cx, func() core.Points { return (shell.paneWidth()-48)/core.Points(n)*0.75 + 60 })
				l.Style.Padding, l.Gap, l.ColumnGap = core.Symmetric(0, gutter), 16, 16
				l.Row = func(cx *core.Scope, r core.ListRow) core.Node { return photoTile(cx, at(r)) }
				return l
			})
		})
	}
	return core.Scroll{TestID: "photos-page", OffsetValues: core.AnimatedXY{Y: scrollY}, Refreshing: refreshing.Get, OnRefresh: refresh,
		StyleFunc: func() core.Style { return fillPage.Merge(core.Style{Background: colors.Background()}) },
		Children:  []core.Node{header, hero(), body, footer}}
}

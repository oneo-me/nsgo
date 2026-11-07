package main

import (
	"fmt"
	"github.com/oneo-me/nsgo/icons/lucide"
	"math"
	"slices"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

var scrollTones = []Tone{ToneAccent, ToneInfo, ToneWarn, ToneUp, ToneDanger}

// backdrop is a dot matrix with a low-alpha area and a dashed reference
// line: a page's backdrop.
func backdrop(cx *core.Scope, t Tone, seed float64) core.Node {
	colors := ui.ColorsOf(cx)
	dots, faint := colors.Border, colors.Faint
	return core.Canvas{Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0)},
		Draw: func(d *core.Drawing, size core.Size) {
			c, tint := t.Color(cx), t.Tint(cx)
			w, h := float32(size.Width), float32(size.Height)
			d.SetFillColor(dots())
			for x := float32(10); x < w; x += 12 {
				for y := float32(10); y < h; y += 12 {
					d.FillRect(x, y, 1.5, 1.5)
				}
			}
			// A wave, filled faintly below.
			type pt struct{ x, y float32 }
			var points []pt
			for i := 0; i <= 24; i++ {
				fi := float64(i)
				y := float64(h) * (0.62 - 0.18*math.Sin(fi*0.5+seed) - 0.06*math.Sin(fi*1.3+seed*2))
				points = append(points, pt{float32(i) / 24 * w, float32(y)})
			}
			d.BeginPath().MoveTo(0, h)
			for _, p := range points {
				d.LineTo(p.x, p.y)
			}
			d.LineTo(w, h).ClosePath().SetFillColor(tint).Fill()
			d.BeginPath()
			for i, p := range points {
				if i == 0 {
					d.MoveTo(p.x, p.y)
				} else {
					d.LineTo(p.x, p.y)
				}
			}
			d.SetStrokeColor(c).SetLineWidth(1.5).Stroke()
			d.SetLineDash(3, 4).SetStrokeColor(faint()).SetLineWidth(1).BeginPath().MoveTo(0, h*0.5).LineTo(w, h*0.5).Stroke()
		}}
}

var pagerPages = []struct {
	title       func(locales.Messages) string
	value, note string
	tone        Tone
}{
	{locales.Messages.ScrollRevenue, "48.2k", "+12 %", ToneAccent},
	{locales.Messages.ScrollSessions, "9,410", "+3 %", ToneInfo},
	{locales.Messages.ScrollLatency, "142", "ms p95", ToneWarn},
	{locales.Messages.ScrollUptime, "99.98", "%", ToneUp},
	{locales.Messages.ScrollErrors, "17", "−40 %", ToneDanger},
}

// surfaceBox is a scroll view's box inside a section, surfaceInk its
// colors.
func surfaceBox(cx *core.Scope, height core.Points) core.Style {
	return core.Style{Height: core.Pt(height), CornerRadius: ui.RadiusControl, BorderWidth: 1}
}

// surfaceInk is a surface box's colors, for its StyleFunc.
func surfaceInk(cx *core.Scope) func() core.Style {
	colors := ui.ColorsOf(cx)
	return func() core.Style {
		return core.Style{Background: colors.Surface(), Shadows: ui.FinOf(cx).Card().Shadows, BorderColor: colors.Line()}
	}
}

// pager is full-width pages, one per fling; the indicator and the
// parallax follow the offset on the platform.
func pager(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	width := core.NewSignal(cx, core.Untracked(cx, cx.Env().Window).Width-72)
	page := core.NewSignal(cx, 0)
	x := core.NewAnimatedValue(cx, 0)
	const step = 16
	settle := func(e core.ScrollEvent) {
		if w := width.GetUntracked(); w > 0 {
			page.Set(int(math.Round(float64(e.Offset.X / w))))
		}
	}
	return core.View{Style: core.Style{Gap: 12}, Children: []core.Node{
		core.Scroll{Horizontal: true, HideIndicator: true, Behavior: core.ScrollBehavior{Paging: true}, OffsetValues: core.AnimatedXY{X: x}, TestID: "pager",
			Style: surfaceBox(cx, 160), StyleFunc: surfaceInk(cx), OnLayout: func(b core.Bounds) { width.Set(b.Size.Width) }, OnMomentumEnd: settle, OnScrollDragEnd: settle,
			Children: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
				w := float64(width.Get())
				var pages []core.Node
				for i, p := range pagerPages {
					fi := float64(i)
					rng := []float64{(fi - 1) * w, fi * w, (fi + 1) * w}
					pages = append(pages, core.View{Style: core.Style{Width: core.Pt(core.Points(w)), Height: core.Pt(160), Overflow: core.Hidden}, Children: []core.Node{
						backdrop(cx, p.tone, fi*1.7),
						// The text lags behind the page and fades at the edges.
						core.View{Style: core.Style{Padding: core.All(16), Gap: 4, Animated: &core.AnimatedStyle{
							Opacity:    core.Interpolate(x, core.Interpolation{Input: rng, Output: []float64{0.1, 1, 0.1}, Extrapolate: core.ExtrapolateClamp}),
							TranslateX: core.Interpolate(x, core.Interpolation{Input: rng, Output: []float64{-w * 0.35, 0, w * 0.35}, Extrapolate: core.ExtrapolateClamp}),
						}}, Children: []core.Node{
							ui.Text(tr.ScrollPageOf(p.title(tr), i+1, len(pagerPages)), 13, ui.Regular, colors.Muted),
							Figure{Value: label(p.value), Note: label(p.note), Tone: p.tone},
						}},
					}})
				}
				return core.View{Style: core.Style{Direction: core.Row}, Children: pages}
			})}},
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, Gap: step - 8}, Children: append(func() []core.Node {
				var dots []core.Node
				for range pagerPages {
					dots = append(dots, core.View{Style: core.Style{Width: core.Pt(8), Height: core.Pt(8), CornerRadius: 4}, StyleFunc: ui.Filled(colors.Raised)})
				}
				return dots
			}(), core.View{StyleFunc: func() core.Style {
				last := float64(len(pagerPages) - 1)
				return core.Style{Position: core.Absolute, Left: core.Pt(0), Width: core.Pt(8), Height: core.Pt(8), CornerRadius: 4, Background: colors.Accent(),
					Animated: &core.AnimatedStyle{TranslateX: core.Interpolate(x, core.Interpolation{Input: []float64{0, max(1, last*float64(width.Get()))},
						Output: []float64{0, last * step}, Extrapolate: core.ExtrapolateClamp})}}
			}})},
			core.Text{ContentFunc: func() string { return tr.ScrollSettledOnPage(page.Get() + 1) },
				Style: core.Style{FlexGrow: 1, TextAlign: core.TextAlignEnd, FontSize: 13}, StyleFunc: ui.Colored(colors.Faint)},
		}},
	}}
}

// snapCards are cards that snap; a fling can pass several unless limited
// to one.
func snapCards(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	const size, gap = 220, 12
	oneStep := core.NewSignal(cx, false)
	align := core.NewSignal(cx, core.SnapCenter)
	return core.View{Style: core.Style{Gap: 12}, Children: []core.Node{
		core.View{Style: core.Style{Margin: core.Edges{Left: -14, Right: -14}}, Children: []core.Node{
			core.List{Count: func() int { return 20 }, Horizontal: true, EstimatedSize: size + gap, HideIndicator: true, Gap: gap,
				BehaviorFunc: func() core.ScrollBehavior {
					return core.ScrollBehavior{SnapToInterval: size + gap, SnapToAlignment: align.Get(), DecelerationRate: core.DecelerationFast, DisableIntervalMomentum: oneStep.Get()}
				},
				Style: core.Style{Height: core.Pt(128), Padding: core.Symmetric(0, 14)},
				Row: func(cx *core.Scope, r core.ListRow) core.Node {
					colors := ui.ColorsOf(cx)
					fin := ui.FinOf(cx)
					t := func() Tone { return scrollTones[r.Index()%len(scrollTones)] }
					return core.View{StyleFunc: func() core.Style {
						return fin.Raised().Over(core.Style{Width: core.Pt(size), Height: core.Pt(128), CornerRadius: 18, BorderCurve: core.BorderCurveContinuous,
							Background: colors.Raised(), BorderWidth: 1, BorderColor: colors.Border(), Padding: core.All(16), JustifyContent: core.JustifySpaceBetween, Overflow: core.Hidden})
					},
						Children: []core.Node{
							core.View{StyleFunc: func() core.Style {
								return core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Bottom: core.Pt(0), Width: core.Pt(3), Background: t().Color(cx)}
							}},
							core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter}, Children: []core.Node{
								core.Text{ContentFunc: func() string { return fmt.Sprintf("at %d pt", r.Index()*(size+gap)) }, Style: core.Style{FlexGrow: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Faint)},
								core.Dynamic(func(*core.Scope) core.Node { return ui.Badge{Text: toneNames[t()], Variant: t().Badge()} }),
							}},
							ui.Textf(func() string { return tr.ListsCard(r.Index() + 1) }, 24, ui.Semibold, colors.Text),
						}}
				}},
		}},
		ui.Tabs[core.SnapAlignment]{Equal: true, Options: []ui.Option[core.SnapAlignment]{{Value: core.SnapStart, Label: "start"}, {Value: core.SnapCenter, Label: "center"}, {Value: core.SnapEnd, Label: "end"}}, Value: align.Get, OnChange: align.Set},
		SwitchRow{Label: tr.ScrollOneCardPerFling(), Code: "DisableIntervalMomentum", Checked: oneStep.Get, OnChange: oneStep.Set},
	}}
}

type logged struct {
	name string
	y    int
	tone Tone
}

// scrollEvents is OnScroll, drag and momentum events, NoScroll, NoBounces
// and ContentInset.
func scrollEvents(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	offset := core.NewSignal(cx, core.ScrollEvent{})
	seen := core.NewSignal(cx, false)
	log := core.NewSignalFunc(cx, []logged(nil), slices.Equal)
	enabled, bounces, insetOn := core.NewSignal(cx, true), core.NewSignal(cx, true), core.NewSignal(cx, false)
	noteEvent := func(name string, t Tone) func(core.ScrollEvent) {
		return func(e core.ScrollEvent) {
			log.Update(func(l []logged) []logged {
				out := append([]logged{{name, int(math.Round(float64(e.Offset.Y))), t}}, l...)
				return out[:min(len(out), 5)]
			})
		}
	}
	progress := func() float64 {
		o := offset.Get()
		if o.ContentSize.Height > o.Size.Height {
			return float64(o.Offset.Y / (o.ContentSize.Height - o.Size.Height))
		}
		return 0
	}
	var rows []core.Node
	for i := 1; i <= 30; i++ {
		rows = append(rows, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, Height: core.Pt(32), Padding: core.Symmetric(0, 12)}, Children: []core.Node{
			core.Text{Content: itoa(i), Style: core.Style{Width: core.Pt(24), FontSize: 12, TextAlign: core.TextAlignEnd}, StyleFunc: ui.Colored(colors.Faint)},
			core.View{Style: core.Style{Height: core.Pt(4), CornerRadius: 2, Width: core.Pct(float32(20 + (i*37)%70))}, StyleFunc: ui.Filled(colors.Raised)},
		}})
	}
	return core.View{Style: core.Style{Gap: 12}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignEnd}, Children: []core.Node{
			core.View{Style: fill, Children: []core.Node{Figure{Value: func() string { return itoa(int(math.Round(float64(offset.Get().Offset.Y)))) }, Unit: "pt", Note: func() string {
				if !seen.Get() {
					return tr.ScrollScrollTheBox()
				}
				return fmt.Sprintf("/ %.0f", offset.Get().ContentSize.Height)
			}, Tone: ToneInfo}}},
			ui.Textf(func() string {
				if !seen.Get() {
					return tr.ScrollViewportPt()
				}
				return tr.ScrollViewport(fmt.Sprintf("%.0f", offset.Get().Size.Height))
			}, 13, ui.Regular, colors.Faint),
		}},
		ui.Meter{Value: progress, Variant: ui.MeterInfo},
		core.Scroll{TestID: "events-scroll", Style: surfaceBox(cx, 180), StyleFunc: surfaceInk(cx),
			BehaviorFunc: func() core.ScrollBehavior {
				b := core.ScrollBehavior{NoScroll: !enabled.Get(), NoBounces: !bounces.Get()}
				if insetOn.Get() {
					b.ContentInset = core.Edges{Top: 40, Bottom: 40}
				}
				return b
			},
			OnScroll:          func(e core.ScrollEvent) { offset.Set(e); seen.Set(true) },
			OnScrollDragStart: noteEvent(tr.ScrollBeginDrag(), ToneAccent),
			OnScrollDragEnd:   noteEvent(tr.ScrollEndDrag(), ToneWarn),
			OnMomentumStart:   noteEvent(tr.ScrollMomentumBegin(), ToneInfo),
			OnMomentumEnd:     noteEvent(tr.ScrollMomentumEnd(), ToneUp),
			Children:          []core.Node{core.View{Style: core.Style{Padding: core.Symmetric(6, 0)}, Children: rows}}},
		core.View{Style: core.Style{Gap: 6, MinHeight: core.Pt(100)}, Children: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
			l := log.Get()
			if len(l) == 0 {
				return ui.Text(tr.ScrollNoDragsYet(), 13, ui.Regular, colors.Faint)
			}
			var lines []core.Node
			for i, e := range l {
				lines = append(lines, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Height: core.Pt(20), Opacity: core.Alpha(1 - float32(i)*0.16)}, Children: []core.Node{
					core.View{Style: core.Style{Width: core.Pt(8), Height: core.Pt(8), CornerRadius: 2.5}, StyleFunc: ui.Filled(e.tone.Shade(cx))},
					core.Text{Content: e.name, Style: core.Style{FlexGrow: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
					ui.Text(fmt.Sprintf("at %d pt", e.y), 13, ui.Regular, colors.Faint),
				}})
			}
			return core.Group(lines)
		})}},
		core.View{Children: []core.Node{
			SwitchRow{Label: tr.ScrollScrollEnabled(), Code: "NoScroll", Checked: enabled.Get, OnChange: enabled.Set},
			SwitchRow{Label: tr.ScrollBounces(), Code: "NoBounces", Checked: bounces.Get, OnChange: bounces.Set},
			SwitchRow{Label: tr.ScrollContentInset(), Note: tr.Scroll40PtAtTheTop(), Checked: insetOn.Get, OnChange: insetOn.Set},
		}},
	}}
}

// zoom is pinch to zoom on the platform; pressing a tile zooms in on it.
func zoom(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	ref := &core.Ref{}
	scale := core.NewSignal(cx, float32(1))
	bounces := core.NewSignal(cx, true)
	var tiles []core.Node
	for i := range 24 {
		t := scrollTones[(i*7)%len(scrollTones)]
		tiles = append(tiles, core.View{Role: core.RoleButton, Label: tr.ScrollTile(i + 1),
			StyleFunc: func() core.Style {
				return core.Style{Width: core.Pt(74), Height: core.Pt(50), CornerRadius: 10, BorderCurve: core.BorderCurveContinuous, Background: t.Tint(cx),
					BorderWidth: 1, BorderColor: colors.Border(), Padding: core.All(6), JustifyContent: core.JustifySpaceBetween}
			},
			OnPress: func(core.PressEvent) {
				// The tile's middle, in the content's unzoomed points.
				at := core.Point{X: core.Points(6 + (i%4)*80 + 37), Y: core.Points(6 + (i/4)*56 + 25)}
				to := float32(1)
				if scale.GetUntracked() < 2.5 {
					to = 3
				}
				ref.ZoomTo(to, &at, true)
			},
			Children: []core.Node{
				core.View{Style: core.Style{Width: core.Pt(12), Height: core.Pt(3), CornerRadius: 1.5}, StyleFunc: ui.Filled(t.Shade(cx))},
				ui.Text(tr.ScrollTile(i+1), 11, ui.Regular, colors.Text),
			}})
	}
	zoomTo := func(s float32) func() { return func() { ref.ZoomTo(s, nil, true) } }
	return core.View{Style: core.Style{Gap: 12}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignEnd}, Children: []core.Node{
			core.View{Style: fill, Children: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
				s := scale.Get()
				noteText, t := "fit", ToneInfo
				switch {
				case s > 1.01:
					noteText, t = tr.ScrollZoomedIn(), ToneAccent
				case s < 0.99:
					noteText = tr.ScrollZoomedOut()
				}
				return Figure{Value: label(fmt.Sprintf("%.2f", s)), Unit: "×", Note: label(noteText), Tone: t}
			})}},
			ui.Text(tr.Scroll05To5(), 13, ui.Regular, colors.Faint),
		}},
		core.Dynamic(func(cx *core.Scope) core.Node {
			// The scroll view takes NoBouncesZoom as it mounts.
			noBounce := !bounces.Get()
			return core.Scroll{Ref: ref, TestID: "zoom", Style: surfaceBox(cx, 240), StyleFunc: surfaceInk(cx), MinZoomScale: 0.5, MaxZoomScale: 5, CenterContent: true, NoBouncesZoom: noBounce,
				OnScroll: func(e core.ScrollEvent) { scale.Set(e.ZoomScale) },
				Children: []core.Node{core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Padding: core.All(6), Gap: 6, Width: core.Pt(332)}, Children: tiles}}}
		}),
		core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
			ui.Cell{Child: ui.Button{Text: "0.5×", Variant: ui.ButtonSecondary, OnPress: zoomTo(0.5)}},
			ui.Cell{Child: ui.Button{Text: "1×", OnPress: zoomTo(1)}},
			ui.Cell{Child: ui.Button{Text: "2×", Variant: ui.ButtonSecondary, OnPress: zoomTo(2)}},
		}},
		SwitchRow{Label: tr.ScrollBouncesPastTheLimits(), Code: "NoBouncesZoom", Checked: bounces.Get, OnChange: bounces.Set},
	}}
}

type post struct {
	id    int
	label string
	fresh bool
	tone  Tone
}

// feed is new items arriving at the top while the rows on screen stay
// where they are.
func feed(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	next, freshCount := 0, 0
	mk := func(label string, fresh bool) post {
		p := post{next, label, fresh, scrollTones[next%len(scrollTones)]}
		next++
		return p
	}
	var initial []post
	for i := range 30 {
		initial = append(initial, mk(tr.ScrollPost(i+1), false))
	}
	posts := core.NewSignalFunc(cx, initial, slices.Equal)
	addOnTop := func() {
		var fresh []post
		for range 3 {
			freshCount++
			fresh = append(fresh, mk(tr.ScrollNew(freshCount), true))
		}
		posts.Update(func(p []post) []post { return append(fresh, p...) })
	}
	return core.View{Style: core.Style{Gap: 12}, Children: []core.Node{
		core.List{TestID: "feed", Count: func() int { return len(posts.Get()) }, Key: func(i int) any { return posts.GetUntracked()[i].id }, EstimatedSize: 48,
			Style: surfaceBox(cx, 240), StyleFunc: surfaceInk(cx),
			Row: func(cx *core.Scope, r core.ListRow) core.Node {
				colors := ui.ColorsOf(cx)
				p := func() post { return posts.Get()[r.Index()] }
				return core.View{StyleFunc: func() core.Style {
					return core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Height: core.Pt(48), Padding: core.Symmetric(0, 14),
						Borders: core.Edges{Bottom: 1}, BorderColors: core.EdgeColors{Bottom: colors.Line()}}
				}, Children: []core.Node{
					core.View{StyleFunc: func() core.Style {
						c := colors.Raised
						if p().fresh {
							c = colors.Accent
						}
						return core.Style{Width: core.Pt(8), Height: core.Pt(8), CornerRadius: 4, Background: c()}
					}},
					core.Text{ContentFunc: func() string { return p().label }, StyleFunc: func() core.Style {
						c := colors.Muted
						if p().fresh {
							c = colors.Text
						}
						return core.Style{FlexGrow: 1, FontSize: 13, Color: c()}
					}},
					core.Dynamic(func(*core.Scope) core.Node {
						if p().fresh {
							return ui.Badge{Text: "new", Variant: ui.BadgeAccent}
						}
						return ui.Text(fmt.Sprintf("#%d", p().id), 13, ui.Regular, colors.Faint)
					}),
				}}
			}},
		Caption{Label: tr.ScrollScrollDownALittleThen(), Note: func() string { return tr.ScrollPosts(len(posts.Get())) }},
		ui.Button{Text: tr.ScrollAdd3PostsOnTop(), Icon: lucide.Plus, OnPress: addOnTop},
	}}
}

// scrollingDemo is core.Scroll and core.List's scrolling: paging,
// snapping, zoom, the scroll events, prepending and taps with the keyboard
// up.
func scrollingDemo(cx *core.Scope) core.Node {
	colors := ui.ColorsOf(cx)
	taps := core.NewSignal(cx, core.KeyboardTapsNever)
	presses := core.NewSignal(cx, 0)
	countColor := func() core.Color {
		if cx.Dark() {
			return ui.Mix(colors.Faint(), colors.Background(), 0.5)
		}
		return colors.Faint()
	}
	return demoPage(cx, pageProps{still: true, bottom: 16,
		behaviorFunc: func() core.ScrollBehavior { return core.ScrollBehavior{KeyboardShouldPersistTaps: taps.Get()} }},
		func(cx *core.Scope) core.Node {
			tr := locales.For(cx)
			return Section{Title: tr.ScrollEdgeEffects(), Note: tr.ScrollScrollUnderTheBars(), Children: []core.Node{API{Text: "EdgeEffects · EdgeBar"}, edgeEffects(cx)}}
		},
		func(cx *core.Scope) core.Node {
			tr := locales.For(cx)
			return Section{Title: tr.ScrollPaging(), Note: tr.Scroll5Pages(), Children: []core.Node{API{Text: "Behavior.Paging · OffsetValues"}, pager(cx)}}
		},
		func(cx *core.Scope) core.Node {
			tr := locales.For(cx)
			return Section{Title: tr.ScrollSnapping(), Note: tr.Scroll20Cards(), Children: []core.Node{API{Text: "SnapToInterval · SnapToAlignment"}, snapCards(cx)}}
		},
		func(cx *core.Scope) core.Node {
			tr := locales.For(cx)
			return Section{Title: tr.ScrollZoom(), Note: Hint(cx, tr.ScrollPinchOrTapATile(), tr.ScrollPinchOnATouchpadOr()), Children: []core.Node{API{Text: "MaxZoomScale · Ref.ZoomTo"}, zoom(cx)}}
		},
		func(cx *core.Scope) core.Node {
			tr := locales.For(cx)
			return Section{Title: tr.ScrollEventsAndBehavior(), Children: []core.Node{API{Text: "OnScroll · OnScrollDragStart · OnMomentumEnd"}, scrollEvents(cx)}}
		},
		func(cx *core.Scope) core.Node {
			tr := locales.For(cx)
			return Section{Title: tr.ScrollPrependingKeepsThePosition(), Children: []core.Node{feed(cx)}}
		},
		func(cx *core.Scope) core.Node {
			tr := locales.For(cx)
			colors := ui.ColorsOf(cx)
			fin := ui.FinOf(cx)
			return Section{Title: tr.ScrollTapsWithTheKeyboardUp(), Children: []core.Node{
				API{Text: "KeyboardShouldPersistTaps"},
				ui.Tabs[core.KeyboardTaps]{Equal: true, Options: []ui.Option[core.KeyboardTaps]{{Value: core.KeyboardTapsNever, Label: "never"}, {Value: core.KeyboardTapsHandled, Label: "handled"},
					{Value: core.KeyboardTapsAlways, Label: "always"}}, Value: taps.Get, OnChange: taps.Set},
				core.Text{ContentFunc: func() string {
					switch taps.Get() {
					case core.KeyboardTapsHandled:
						return tr.ScrollTheButtonGetsTheTap()
					case core.KeyboardTapsAlways:
						return tr.ScrollTapsNeverHideTheKeyboard()
					}
					return tr.ScrollATapOutsideTheInput()
				}, Style: core.Style{FontSize: 13, LineHeight: 18}, StyleFunc: ui.Colored(colors.Muted)},
				base.Input{Placeholder: Hint(cx, tr.ScrollTypeHereThenTapThe(), tr.ScrollTypeHereThenClickThe()),
					StyleFunc: func() core.Style {
						return fin.Field().Over(core.Style{PlaceholderColor: colors.Faint(), SelectionColor: colors.Accent(), Height: core.Pt(44), FontSize: 13, Color: colors.Text(),
							Background: colors.Field(), BorderWidth: 1, BorderColor: colors.FieldBorder(), CornerRadius: ui.RadiusControl, Padding: core.Symmetric(0, 14)})
					}},
				ui.Press{OnPress: func() { presses.Update(func(n int) int { return n + 1 }) }, Label: tr.ScrollCountATap(), Children: []core.Node{
					core.View{StyleFunc: func() core.Style {
						return fin.Light().Over(core.Style{Height: core.Pt(44), CornerRadius: ui.RadiusControl, BorderCurve: core.BorderCurveContinuous, Background: colors.Light(),
							Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 8})
					}, Children: []core.Node{
						ui.Text(Hint(cx, tr.ScrollTapMe(), tr.ScrollClickMe()), 13, ui.Semibold, colors.OnLight),
						ui.Textf(func() string { return itoa(presses.Get()) }, 13, ui.Regular, countColor),
					}},
				}},
				core.View{Style: core.Style{Height: core.Pt(300)}},
			}}
		},
	)
}

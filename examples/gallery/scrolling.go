package main

import (
	"fmt"
	"math"
	"slices"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

var scrollTones = []tone{toneAccent, toneInfo, toneWarn, toneUp, toneDanger}

// switchRow is a switch with its label; code names the option it sets.
func switchRow(cx *core.Scope, label, codeText, noteText string, value func() bool, onChange func(bool)) core.Node {
	colors := colorsOf(cx)
	titles := []core.Node{text(label, 15, regular, colors.text)}
	if codeText != "" {
		titles = append(titles, core.Text{Content: codeText, Style: core.Style{FontSize: 12, FontFamily: mono}, StyleFunc: colored(colors.faint)})
	}
	if noteText != "" {
		titles = append(titles, text(noteText, 13, regular, colors.faint))
	}
	return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, MinHeight: core.Pt(48)}, Children: []core.Node{
		core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: titles},
		toggle(value, onChange, label),
	}}
}

// api is the APIs a section shows, as code.
func api(cx *core.Scope, names string) core.Node {
	colors := colorsOf(cx)
	return core.Text{Content: names, Style: core.Style{FontSize: 12, FontFamily: mono}, StyleFunc: colored(colors.faint)}
}

// backdrop is a dot matrix with a low-alpha area and a dashed reference
// line: a page's backdrop.
func backdrop(cx *core.Scope, t tone, seed float64) core.Node {
	colors := colorsOf(cx)
	dots, faint := colors.border, colors.faint
	return core.Canvas{Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0)},
		Draw: func(d *core.Drawing, size core.Size) {
			c, tint := t.color(cx), t.tint(cx)
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
	title, value, note string
	tone               tone
}{
	{"Revenue", "48.2k", "+12 %", toneAccent},
	{"Sessions", "9,410", "+3 %", toneInfo},
	{"Latency", "142", "ms p95", toneWarn},
	{"Uptime", "99.98", "%", toneUp},
	{"Errors", "17", "−40 %", toneDanger},
}

// surfaceBox is a scroll view's box inside a section, surfaceInk its
// colors.
func surfaceBox(cx *core.Scope, height core.Points) core.Style {
	return core.Style{Height: core.Pt(height), CornerRadius: radiusControl, BorderWidth: 1}
}

// surfaceInk is a surface box's colors, for its StyleFunc.
func surfaceInk(cx *core.Scope) func() core.Style {
	colors := colorsOf(cx)
	return func() core.Style {
		return core.Style{Background: colors.surface(), Shadows: finOf(cx).card().shadows, BorderColor: colors.line()}
	}
}

// pager is full-width pages, one per fling; the indicator and the
// parallax follow the offset on the platform.
func pager(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
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
							text(fmt.Sprintf("%s · %d of %d", p.title, i+1, len(pagerPages)), 13, regular, colors.muted),
							figure(cx, label(p.value), "", label(p.note), p.tone),
						}},
					}})
				}
				return core.View{Style: core.Style{Direction: core.Row}, Children: pages}
			})}},
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter}, Children: []core.Node{
			core.View{Style: core.Style{Direction: core.Row, Gap: step - 8}, Children: append(func() []core.Node {
				var dots []core.Node
				for range pagerPages {
					dots = append(dots, core.View{Style: core.Style{Width: core.Pt(8), Height: core.Pt(8), CornerRadius: 4}, StyleFunc: filled(colors.raised)})
				}
				return dots
			}(), core.View{StyleFunc: func() core.Style {
				last := float64(len(pagerPages) - 1)
				return core.Style{Position: core.Absolute, Left: core.Pt(0), Width: core.Pt(8), Height: core.Pt(8), CornerRadius: 4, Background: colors.accent(),
					Animated: &core.AnimatedStyle{TranslateX: core.Interpolate(x, core.Interpolation{Input: []float64{0, max(1, last*float64(width.Get()))},
						Output: []float64{0, last * step}, Extrapolate: core.ExtrapolateClamp})}}
			}})},
			core.Text{ContentFunc: func() string { return fmt.Sprintf("Settled on page %d", page.Get()+1) },
				Style: core.Style{FlexGrow: 1, TextAlign: core.TextAlignEnd, FontSize: 13}, StyleFunc: colored(colors.faint)},
		}},
	}}
}

// snapCards are cards that snap; a fling can pass several unless limited
// to one.
func snapCards(cx *core.Scope) core.Node {
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
					colors := colorsOf(cx)
					fin := finOf(cx)
					t := func() tone { return scrollTones[r.Index()%len(scrollTones)] }
					return core.View{StyleFunc: func() core.Style {
						return fin.raised().over(core.Style{Width: core.Pt(size), Height: core.Pt(128), CornerRadius: 18, BorderCurve: core.BorderCurveContinuous,
							Background: colors.raised(), BorderWidth: 1, BorderColor: colors.border(), Padding: core.All(16), JustifyContent: core.JustifySpaceBetween, Overflow: core.Hidden})
					},
						Children: []core.Node{
							core.View{StyleFunc: func() core.Style {
								return core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Bottom: core.Pt(0), Width: core.Pt(3), Background: t().color(cx)}
							}},
							core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter}, Children: []core.Node{
								core.Text{ContentFunc: func() string { return fmt.Sprintf("at %d pt", r.Index()*(size+gap)) }, Style: core.Style{FlexGrow: 1, FontSize: 13}, StyleFunc: colored(colors.faint)},
								core.Dynamic(func(*core.Scope) core.Node { return badge(cx, toneNames[t()], t()) }),
							}},
							textf(func() string { return fmt.Sprintf("Card %d", r.Index()+1) }, 24, semibold, colors.text),
						}}
				}},
		}},
		segmented([]option[core.SnapAlignment]{{value: core.SnapStart, label: "start"}, {value: core.SnapCenter, label: "center"}, {value: core.SnapEnd, label: "end"}},
			align.Get, align.Set, true),
		switchRow(cx, "One card per fling", "DisableIntervalMomentum", "", oneStep.Get, oneStep.Set),
	}}
}

type logged struct {
	name string
	y    int
	tone tone
}

// scrollEvents is OnScroll, drag and momentum events, NoScroll, NoBounces
// and ContentInset.
func scrollEvents(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	offset := core.NewSignal(cx, core.ScrollEvent{})
	seen := core.NewSignal(cx, false)
	log := core.NewSignalFunc(cx, []logged(nil), slices.Equal)
	enabled, bounces, insetOn := core.NewSignal(cx, true), core.NewSignal(cx, true), core.NewSignal(cx, false)
	noteEvent := func(name string, t tone) func(core.ScrollEvent) {
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
			core.Text{Content: itoa(i), Style: core.Style{Width: core.Pt(24), FontSize: 12, TextAlign: core.TextAlignEnd}, StyleFunc: colored(colors.faint)},
			core.View{Style: core.Style{Height: core.Pt(4), CornerRadius: 2, Width: core.Pct(float32(20 + (i*37)%70))}, StyleFunc: filled(colors.raised)},
		}})
	}
	return core.View{Style: core.Style{Gap: 12}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignEnd}, Children: []core.Node{
			core.View{Style: fill, Children: []core.Node{figure(cx, func() string { return itoa(int(math.Round(float64(offset.Get().Offset.Y)))) }, "pt", func() string {
				if !seen.Get() {
					return "Scroll the box"
				}
				return fmt.Sprintf("/ %.0f", offset.Get().ContentSize.Height)
			}, toneInfo)}},
			textf(func() string {
				if !seen.Get() {
					return "viewport — pt"
				}
				return fmt.Sprintf("viewport %.0f pt", offset.Get().Size.Height)
			}, 13, regular, colors.faint),
		}},
		meter(cx, progress, toneInfo, -1),
		core.Scroll{TestID: "events-scroll", Style: surfaceBox(cx, 180), StyleFunc: surfaceInk(cx),
			BehaviorFunc: func() core.ScrollBehavior {
				b := core.ScrollBehavior{NoScroll: !enabled.Get(), NoBounces: !bounces.Get()}
				if insetOn.Get() {
					b.ContentInset = core.Edges{Top: 40, Bottom: 40}
				}
				return b
			},
			OnScroll:          func(e core.ScrollEvent) { offset.Set(e); seen.Set(true) },
			OnScrollDragStart: noteEvent("begin drag", toneAccent),
			OnScrollDragEnd:   noteEvent("end drag", toneWarn),
			OnMomentumStart:   noteEvent("momentum begin", toneInfo),
			OnMomentumEnd:     noteEvent("momentum end", toneUp),
			Children:          []core.Node{core.View{Style: core.Style{Padding: core.Symmetric(6, 0)}, Children: rows}}},
		core.View{Style: core.Style{Gap: 6, MinHeight: core.Pt(100)}, Children: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
			l := log.Get()
			if len(l) == 0 {
				return text("No drags yet", 13, regular, colors.faint)
			}
			var lines []core.Node
			for i, e := range l {
				lines = append(lines, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Height: core.Pt(20), Opacity: core.Alpha(1 - float32(i)*0.16)}, Children: []core.Node{
					core.View{Style: core.Style{Width: core.Pt(8), Height: core.Pt(8), CornerRadius: 2.5}, StyleFunc: filled(e.tone.shade(cx))},
					core.Text{Content: e.name, Style: core.Style{FlexGrow: 1, FontSize: 13}, StyleFunc: colored(colors.muted)},
					text(fmt.Sprintf("at %d pt", e.y), 13, regular, colors.faint),
				}})
			}
			return core.Group(lines)
		})}},
		core.View{Children: []core.Node{
			switchRow(cx, "Scroll enabled", "NoScroll", "", enabled.Get, enabled.Set),
			switchRow(cx, "Bounces", "NoBounces", "", bounces.Get, bounces.Set),
			switchRow(cx, "Content inset", "", "40 pt at the top and bottom", insetOn.Get, insetOn.Set),
		}},
	}}
}

// zoom is pinch to zoom on the platform; pressing a tile zooms in on it.
func zoom(cx *core.Scope) core.Node {
	colors := colorsOf(cx)
	ref := &core.Ref{}
	scale := core.NewSignal(cx, float32(1))
	bounces := core.NewSignal(cx, true)
	var tiles []core.Node
	for i := range 24 {
		t := scrollTones[(i*7)%len(scrollTones)]
		tiles = append(tiles, core.View{Role: core.RoleButton, Label: fmt.Sprintf("Tile %d", i+1),
			StyleFunc: func() core.Style {
				return core.Style{Width: core.Pt(74), Height: core.Pt(50), CornerRadius: 10, BorderCurve: core.BorderCurveContinuous, Background: t.tint(cx),
					BorderWidth: 1, BorderColor: colors.border(), Padding: core.All(6), JustifyContent: core.JustifySpaceBetween}
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
				core.View{Style: core.Style{Width: core.Pt(12), Height: core.Pt(3), CornerRadius: 1.5}, StyleFunc: filled(t.shade(cx))},
				text(fmt.Sprintf("Tile %d", i+1), 11, regular, colors.text),
			}})
	}
	zoomTo := func(s float32) func() { return func() { ref.ZoomTo(s, nil, true) } }
	return core.View{Style: core.Style{Gap: 12}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignEnd}, Children: []core.Node{
			core.View{Style: fill, Children: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
				s := scale.Get()
				noteText, t := "fit", toneInfo
				switch {
				case s > 1.01:
					noteText, t = "zoomed in", toneAccent
				case s < 0.99:
					noteText = "zoomed out"
				}
				return figure(cx, label(fmt.Sprintf("%.2f", s)), "×", label(noteText), t)
			})}},
			text("0.5× to 5×", 13, regular, colors.faint),
		}},
		core.Dynamic(func(cx *core.Scope) core.Node {
			// The scroll view takes NoBouncesZoom as it mounts.
			noBounce := !bounces.Get()
			return core.Scroll{Ref: ref, TestID: "zoom", Style: surfaceBox(cx, 240), StyleFunc: surfaceInk(cx), MinZoomScale: 0.5, MaxZoomScale: 5, CenterContent: true, NoBouncesZoom: noBounce,
				OnScroll: func(e core.ScrollEvent) { scale.Set(e.ZoomScale) },
				Children: []core.Node{core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, Padding: core.All(6), Gap: 6, Width: core.Pt(332)}, Children: tiles}}}
		}),
		core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
			cell(button(buttonProps{label: "0.5×", variant: secondary}, zoomTo(0.5))),
			cell(button(buttonProps{label: "1×"}, zoomTo(1))),
			cell(button(buttonProps{label: "2×", variant: secondary}, zoomTo(2))),
		}},
		switchRow(cx, "Bounces past the limits", "NoBouncesZoom", "", bounces.Get, bounces.Set),
	}}
}

type post struct {
	id    int
	label string
	fresh bool
	tone  tone
}

// feed is new items arriving at the top while the rows on screen stay
// where they are.
func feed(cx *core.Scope) core.Node {
	next, freshCount := 0, 0
	mk := func(label string, fresh bool) post {
		p := post{next, label, fresh, scrollTones[next%len(scrollTones)]}
		next++
		return p
	}
	var initial []post
	for i := range 30 {
		initial = append(initial, mk(fmt.Sprintf("Post %d", i+1), false))
	}
	posts := core.NewSignalFunc(cx, initial, slices.Equal)
	addOnTop := func() {
		var fresh []post
		for range 3 {
			freshCount++
			fresh = append(fresh, mk(fmt.Sprintf("New %d", freshCount), true))
		}
		posts.Update(func(p []post) []post { return append(fresh, p...) })
	}
	return core.View{Style: core.Style{Gap: 12}, Children: []core.Node{
		core.List{TestID: "feed", Count: func() int { return len(posts.Get()) }, Key: func(i int) any { return posts.GetUntracked()[i].id }, EstimatedSize: 48,
			Style: surfaceBox(cx, 240), StyleFunc: surfaceInk(cx),
			Row: func(cx *core.Scope, r core.ListRow) core.Node {
				colors := colorsOf(cx)
				p := func() post { return posts.Get()[r.Index()] }
				return core.View{StyleFunc: func() core.Style {
					return core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10, Height: core.Pt(48), Padding: core.Symmetric(0, 14),
						Borders: core.Edges{Bottom: 1}, BorderColors: core.EdgeColors{Bottom: colors.line()}}
				}, Children: []core.Node{
					core.View{StyleFunc: func() core.Style {
						c := colors.raised
						if p().fresh {
							c = colors.accent
						}
						return core.Style{Width: core.Pt(8), Height: core.Pt(8), CornerRadius: 4, Background: c()}
					}},
					core.Text{ContentFunc: func() string { return p().label }, StyleFunc: func() core.Style {
						c := colors.muted
						if p().fresh {
							c = colors.text
						}
						return core.Style{FlexGrow: 1, FontSize: 15, Color: c()}
					}},
					core.Dynamic(func(*core.Scope) core.Node {
						if p().fresh {
							return badge(cx, "new", toneAccent)
						}
						return text(fmt.Sprintf("#%d", p().id), 13, regular, colors.faint)
					}),
				}}
			}},
		caption(cx, "Scroll down a little, then add", func() string { return fmt.Sprintf("%d posts", len(posts.Get())) }),
		button(buttonProps{label: "Add 3 posts on top", icon: iconPlus}, addOnTop),
	}}
}

// scrollingDemo is core.Scroll and core.List's scrolling: paging,
// snapping, zoom, the scroll events, prepending and taps with the keyboard
// up.
func scrollingDemo(cx *core.Scope) core.Node {
	taps := core.NewSignal(cx, core.KeyboardTapsNever)
	presses := core.NewSignal(cx, 0)
	countColor := func() core.Color {
		if cx.Dark() {
			return core.RGB(0x58534c)
		}
		return core.RGB(0xa29e95)
	}
	return demoPage(cx, pageProps{still: true, bottom: 16,
		behaviorFunc: func() core.ScrollBehavior { return core.ScrollBehavior{KeyboardShouldPersistTaps: taps.Get()} }},
		func(cx *core.Scope) core.Node {
			return section(cx, "Edge effects", "Scroll under the bars", api(cx, "EdgeEffects · EdgeBar"), edgeEffects(cx))
		},
		func(cx *core.Scope) core.Node {
			return section(cx, "Paging", "5 pages", api(cx, "Behavior.Paging · OffsetValues"), pager(cx))
		},
		func(cx *core.Scope) core.Node {
			return section(cx, "Snapping", "20 cards", api(cx, "SnapToInterval · SnapToAlignment"), snapCards(cx))
		},
		func(cx *core.Scope) core.Node {
			return section(cx, "Zoom", hint(cx, "Pinch or tap a tile", "Pinch on a touchpad or click a tile"), api(cx, "MaxZoomScale · Ref.ZoomTo"), zoom(cx))
		},
		func(cx *core.Scope) core.Node {
			return section(cx, "Events and behavior", "", api(cx, "OnScroll · OnScrollDragStart · OnMomentumEnd"), scrollEvents(cx))
		},
		func(cx *core.Scope) core.Node { return section(cx, "Prepending keeps the position", "", feed(cx)) },
		func(cx *core.Scope) core.Node {
			colors := colorsOf(cx)
			fin := finOf(cx)
			return section(cx, "Taps with the keyboard up", "",
				api(cx, "KeyboardShouldPersistTaps"),
				segmented([]option[core.KeyboardTaps]{{value: core.KeyboardTapsNever, label: "never"}, {value: core.KeyboardTapsHandled, label: "handled"},
					{value: core.KeyboardTapsAlways, label: "always"}}, taps.Get, taps.Set, true),
				core.Text{ContentFunc: func() string {
					switch taps.Get() {
					case core.KeyboardTapsHandled:
						return "The button gets the tap and the keyboard stays; a tap on empty space hides it."
					case core.KeyboardTapsAlways:
						return "Taps never hide the keyboard."
					}
					return "A tap outside the input only hides the keyboard; the button does not get it."
				}, Style: core.Style{FontSize: 13, LineHeight: 18}, StyleFunc: colored(colors.muted)},
				base.Input{Placeholder: hint(cx, "Type here, then tap the button", "Type here, then click the button"),
					StyleFunc: func() core.Style {
						return fin.field().over(core.Style{PlaceholderColor: colors.faint(), SelectionColor: colors.accent(), Height: core.Pt(44), FontSize: 15, Color: colors.text(),
							Background: colors.field(), BorderWidth: 1, BorderColor: colors.fieldBorder(), CornerRadius: radiusControl, Padding: core.Symmetric(0, 14)})
					}},
				press(pressProps{onPress: func() { presses.Update(func(n int) int { return n + 1 }) }, label: "Count a tap"},
					core.View{StyleFunc: func() core.Style {
						return fin.light().over(core.Style{Height: core.Pt(44), CornerRadius: radiusControl, BorderCurve: core.BorderCurveContinuous, Background: colors.light(),
							Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 8})
					}, Children: []core.Node{
						text(hint(cx, "Tap me", "Click me"), 15, semibold, colors.onLight),
						textf(func() string { return itoa(presses.Get()) }, 15, regular, countColor),
					}}),
				core.View{Style: core.Style{Height: core.Pt(300)}},
			)
		},
	)
}

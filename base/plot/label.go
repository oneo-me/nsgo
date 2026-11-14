package plot

import "github.com/nsgo-dev/nsgo/core"

// TextAlign is where a PlotText sits along the x axis from its point.
type TextAlign uint8

const (
	// TextLeft starts the text at the point.
	TextLeft TextAlign = iota
	// TextCenter centers it on the point.
	TextCenter
	// TextRight ends it at the point.
	TextRight
)

// PlotText is a text a Label draws, as gpui-base's Text: what it reads, the
// point its box's top edge is at, as values of the plot's scales, and how
// it sits along x from that point.
type PlotText struct {
	Text string
	// X and Y are values of the plot's scales: a row's index on a point or
	// band scale, a value on a linear one.
	X, Y  float64
	Align TextAlign
	// Class and Style are over what it inherits from the Label: its font
	// and color, and its distance from the point.
	Class string
	Style core.Style
}

// Label is texts at points of a Plot, as gpui-base's PlotLabel: a text over
// a peak, a tag at a line's last point, a caption at a grid line's end. The
// layout places the texts, so they follow the plot's size; they glide with
// the scales' domains, fade in with the plot's appear, and take no pointer.
// Their look is the app's: Class and Style give them the text styles they
// inherit. Screen readers hear the plot, not its labels.
type Label struct {
	Texts func() []PlotText
	// Scale is the y scale its texts' Y are values of.
	Scale     ValueScale
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
}

// Render implements core.Node.
func (l Label) Render(cx *core.Scope) core.Node {
	layer := core.Style{Position: core.Absolute, Left: pts(0), Top: pts(0), Right: pts(0), Bottom: pts(0)}.Merge(l.Style)
	view := core.View{Class: l.Class, Style: layer, StyleFunc: l.StyleFunc, PointerEvents: core.PointerEventsNone}
	plot := plotOf(cx, "Label")
	if plot == nil || l.Texts == nil {
		return view
	}
	// The texts' points, which Motion.Update moves.
	moves := newMoves(cx, plot, false, func() (valuePose, bool) {
		all := l.Texts()
		p := valuePose{rows: make([][]float64, len(all))}
		for i, t := range all {
			p.rows[i] = []float64{t.X, t.Y}
		}
		return p, true
	})
	at := func(p valuePose, i int) (float32, float32, bool) {
		if i >= len(p.rows) {
			return 0, 0, false
		}
		x, okX := plot.xMap.Get().tick(p.rows[i][0])
		y, okY := plot.yMapOf(l.Scale).tick(p.rows[i][1])
		return x, y, okX && okY
	}
	// Where text i's point is on the plot, and how it sits there.
	place := func(i int) (float32, float32, TextAlign, bool) {
		all := l.Texts()
		x, y, ok := at(moves.now().to, i)
		if i >= len(all) || !ok {
			return 0, 0, 0, false
		}
		return x, y, all[i].Align, true
	}
	indices := func() []int {
		out := make([]int, len(l.Texts()))
		for i := range out {
			out[i] = i
		}
		return out
	}
	yTween := plot.yTweenOf(l.Scale)
	view.Children = []core.Node{core.Show{When: plot.sized, Then: func(*core.Scope) core.Node {
		return core.Each(indices, func(i int) int { return i }, func(cx *core.Scope, item func() int, _ func() int) core.Node {
			i := item()
			// While a domain glides, the text moves from where the layout
			// puts it to where the domains shown put it, as the axes' ticks.
			x, y := core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0)
			// And with Motion.Update from where its point was, by what is
			// left of the move.
			dx, dy := core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0)
			cx.Effect(func() {
				moving := plot.xTween.moving.Get() || yTween.moving.Get()
				at0, bt0, _, ok := place(i)
				if !ok {
					return
				}
				if moving {
					x.Set(float64(at0))
					y.Set(float64(bt0))
				}
				ddx, ddy := 0.0, 0.0
				if m := moves.now(); m.moving {
					if j := m.from.match(m.to)(i); j >= 0 {
						if wx, wy, ok := at(m.from, j); ok {
							ddx, ddy = float64(wx-at0), float64(wy-bt0)
						}
					}
				}
				dx.Set(ddx)
				dy.Set(ddy)
			})
			left := moves.left()
			text := func() PlotText {
				if all := l.Texts(); i < len(all) {
					return all[i]
				}
				return PlotText{}
			}
			// A point with no size whose text does not shrink: the text keeps
			// its own size and overflows the point on the side its alignment
			// gives it.
			return core.View{
				Style: core.Style{Position: core.Absolute, Width: pts(0), Height: pts(0), Direction: core.Row, AlignItems: core.AlignStart,
					Animated: &core.AnimatedStyle{TranslateX: core.Add(plot.xTween.moved(x), plot.xTween.stretched(core.Mul(dx, left))),
						TranslateY: core.Add(yTween.moved(y), yTween.stretched(core.Mul(dy, left))), Opacity: plot.appear.progress()}},
				StyleFunc: func() core.Style {
					at, bt, align, ok := place(i)
					if !ok {
						return core.Style{Display: core.DisplayNone}
					}
					s := core.Style{Left: pts(at), Top: pts(bt), JustifyContent: core.JustifyStart}
					switch align {
					case TextCenter:
						s.JustifyContent = core.JustifyCenter
					case TextRight:
						s.JustifyContent = core.JustifyEnd
					}
					return s
				},
				PointerEvents: core.PointerEventsNone,
				Children: []core.Node{core.Text{ContentFunc: func() string { return text().Text },
					ClassFunc: func() string { return text().Class }, StyleFunc: func() core.Style { return text().Style }}},
			}
		})
	}}}
	return view
}

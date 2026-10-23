//go:build linux && !android

package linux

import (
	"time"

	"github.com/oneo-me/nsgo/core"
)

const (
	frameMillis    = 16
	scrollDuration = 300 * time.Millisecond

	allProperties = core.TransitionOpacity | core.TransitionBackground | core.TransitionTransform |
		core.TransitionBorder | core.TransitionShadow
)

// transition is a view's style animating from one ViewStyle to the next.
type transition struct {
	from, to core.ViewStyle
	start    time.Time // after the delay
	duration time.Duration
	curve    core.Curve
	props    core.TransitionProperty
	shown    core.ViewStyle
}

// scrolling is a scroll view's offset animating to a point.
type scrolling struct {
	from, to core.Point
	start    time.Time
}

// styleOf returns the style v shows now: mid transition, or its own,
// with the values the animator binds over it.
func (w *window) styleOf(v *view) *core.ViewStyle {
	s := &v.style
	if v.transition != nil {
		s = &v.transition.shown
	}
	if v.animated != nil {
		out := v.animated.Apply(*s)
		return &out
	}
	return s
}

// setStyle applies v's new style, animating to it when it has a
// transition.
func (w *window) setStyle(v *view, s *core.ViewStyle) {
	from := *w.styleOf(v)
	styled := v.styled
	v.style, v.styled, v.transition = *s, true, nil
	t := s.Transition
	if !styled || t.Duration <= 0 || w.env.ReduceMotion {
		return
	}
	props := t.Properties
	if props == 0 {
		props = allProperties
	}
	first, last := mix(from, *s, 0, props), mix(from, *s, 1, props)
	if first == last {
		return
	}
	v.transition = &transition{from: from, to: *s, start: time.Now().Add(t.Delay), duration: t.Duration,
		curve: t.Easing, props: props, shown: first}
	w.animate(v)
}

// scrollAnimated scrolls v to offset over a moment, or at once when the
// user turned animation effects off.
func (w *window) scrollAnimated(v *view, offset core.Point) {
	if w.env.ReduceMotion {
		w.scrollTo(v, offset)
		return
	}
	v.scrolling = &scrolling{from: v.offset, to: offset, start: time.Now()}
	w.animate(v)
}

// Animate implements engine.Platform: the frame clock runs while the app
// wants frames, and only stops once nothing is animating either. Called
// from the engine goroutine, so it hops to the main thread.
func (w *window) Animate(on bool) {
	w.post(func() {
		if on {
			w.startClock()
			return
		}
		if w.animator.Running() || len(w.animating) > 0 {
			return
		}
		w.stopClock()
	})
}

// stopClock removes the frame timer; tickCallback would otherwise keep
// repeating until the next tick noticed.
func (w *window) stopClock() {
	if w.ticking && w.tickID != 0 {
		gSourceRemove(w.tickID)
	}
	w.ticking = false
	w.tickID = 0
}

// animate ticks v's animations each frame until they end.
func (w *window) animate(v *view) {
	for _, a := range w.animating {
		if a == v {
			return
		}
	}
	w.animating = append(w.animating, v)
	w.startClock()
}

// startClock ticks every frame until nothing animates: no transition or
// scroll, no animator run, no layout animation and no animated image.
func (w *window) startClock() {
	if !w.ticking && !w.closing {
		w.tickID = gTimeoutAdd(frameMillis, cbTick, w.handle)
		w.ticking = true
	}
}

// animatorReport hands what the animator reports to the engine.
func (w *window) animatorReport(ev core.Event) { w.engine.Dispatch(ev) }

// animations applies a batch's changes to the animation graph.
func (w *window) animations(ops []core.AnimationOp) {
	w.animator.Apply(ops)
	if w.animator.Running() {
		w.startClock()
	}
}

// showAnimated draws the views whose bound values changed with them.
func (w *window) showAnimated() {
	w.animator.Changed(func(id core.NodeID, vals core.AnimatedValues) {
		v := w.view(id)
		if v == nil {
			return
		}
		if vals.Set == 0 {
			v.animated = nil
		} else {
			v.animated = &vals
		}
		w.moved(v)
	})
}

// monotonic is the animator's clock: time since the window opened.
func (w *window) monotonic(now time.Time) time.Duration { return now.Sub(w.epoch) }

// tickCallback moves every animation to now and repaints.
func tickCallback(data uintptr) int32 {
	w := windowOf(data)
	if w == nil || w.closing {
		return 0
	}
	w.tick()
	// The timer repeats while something animates.
	return boolInt(w.ticking)
}

// tick moves every animation to now and repaints.
func (w *window) tick() {
	if w.engine != nil && w.engine.WantsFrames() {
		w.engine.Frame()
	}
	now := time.Now()
	live := w.animating[:0]
	for _, v := range w.animating {
		if v.deleted {
			continue
		}
		if t := v.transition; t != nil {
			if now.Before(t.start) {
				live = append(live, v)
				continue
			}
			elapsed := now.Sub(t.start)
			w.moved(v)
			if elapsed >= t.duration {
				v.transition = nil
			} else {
				p := t.curve.At(float64(elapsed) / float64(t.duration))
				t.shown = mix(t.from, t.to, p, t.props)
				live = append(live, v)
			}
		}
		if s := v.scrolling; s != nil {
			elapsed := now.Sub(s.start)
			if elapsed >= scrollDuration {
				v.scrolling = nil
				w.scrollTo(v, s.to)
			} else {
				p := float64(elapsed) / float64(scrollDuration)
				at := core.Point{
					X: core.Points(float64(s.from.X) + float64(s.to.X-s.from.X)*p),
					Y: core.Points(float64(s.from.Y) + float64(s.to.Y-s.from.Y)*p),
				}
				w.scrollTo(v, at)
				live = append(live, v)
			}
		}
	}
	w.animating = live
	running := false
	if w.animator.Running() {
		w.animator.Step(w.monotonic(now))
		running = w.animator.Running()
	}
	w.showAnimated()
	if w.stepLayout(now) {
		running = true
	}
	if w.stepImages(now) {
		running = true
	}
	if len(live) == 0 && !running && (w.engine == nil || !w.engine.WantsFrames()) {
		// tickCallback's return removes the timer.
		w.ticking = false
		w.tickID = 0
	}
}

// mix returns a style between from and to at t, moving only props.
func mix(from, to core.ViewStyle, t float64, props core.TransitionProperty) core.ViewStyle {
	out := to
	if props&core.TransitionOpacity != 0 {
		out.Opacity = mixFloat(from.Opacity, to.Opacity, t)
	}
	if props&core.TransitionBackground != 0 {
		out.Background = mixColor(from.Background, to.Background, t)
	}
	if props&core.TransitionTransform != 0 {
		out.Transform = mixTransform(from.Transform, to.Transform, t)
	}
	if props&core.TransitionBorder != 0 {
		out.BorderWidth = mixPoints(from.BorderWidth, to.BorderWidth, t)
		out.BorderColor = mixColor(from.BorderColor, to.BorderColor, t)
		out.Borders = mixEdges(from.Borders, to.Borders, t)
		out.BorderColors = mixEdgeColors(from.BorderColors, to.BorderColors, t)
		out.CornerRadius = mixPoints(from.CornerRadius, to.CornerRadius, t)
	}
	if props&core.TransitionShadow != 0 {
		out.ShadowRadius = mixPoints(from.ShadowRadius, to.ShadowRadius, t)
		out.ShadowColor = mixColor(from.ShadowColor, to.ShadowColor, t)
		out.ShadowOffset = core.Point{
			X: mixPoints(from.ShadowOffset.X, to.ShadowOffset.X, t),
			Y: mixPoints(from.ShadowOffset.Y, to.ShadowOffset.Y, t),
		}
	}
	return out
}

func mixPoints(a, b core.Points, t float64) core.Points { return a + (b-a)*core.Points(t) }

func mixFloat(a, b float32, t float64) float32 { return a + (b-a)*float32(t) }

// mixColor blends two colors in straight alpha, taking the visible
// color of a transparent start rather than fading through black.
func mixColor(a, b core.Color, t float64) core.Color {
	if a == b {
		return a
	}
	ar, ag, ab, aa := a.Components()
	br, bg, bb, ba := b.Components()
	alpha := aa + (ba-aa)*t
	var r, g, bl float64
	if alpha > 0 {
		// Weight each color by its alpha, so a transparent color does
		// not darken the result.
		r = (ar*aa + (br*ba-ar*aa)*t) / alpha
		g = (ag*aa + (bg*ba-ag*aa)*t) / alpha
		bl = (ab*aa + (bb*ba-ab*aa)*t) / alpha
	}
	return core.RGBA(uint32(r*255+0.5)<<16|uint32(g*255+0.5)<<8|uint32(bl*255+0.5), float32(alpha))
}

func mixTransform(a, b core.Transform, t float64) core.Transform {
	return core.Transform{
		TranslateX: mixPoints(a.TranslateX, b.TranslateX, t),
		TranslateY: mixPoints(a.TranslateY, b.TranslateY, t),
		Scale:      mixFloat(a.Scale, b.Scale, t),
		ScaleX:     mixFloat(a.ScaleX, b.ScaleX, t),
		ScaleY:     mixFloat(a.ScaleY, b.ScaleY, t),
		Rotate:     mixFloat(a.Rotate, b.Rotate, t),
	}
}

func mixEdges(a, b core.Edges, t float64) core.Edges {
	return core.Edges{
		Top: mixPoints(a.Top, b.Top, t), Right: mixPoints(a.Right, b.Right, t),
		Bottom: mixPoints(a.Bottom, b.Bottom, t), Left: mixPoints(a.Left, b.Left, t),
	}
}

func mixEdgeColors(a, b core.EdgeColors, t float64) core.EdgeColors {
	return core.EdgeColors{
		Top: mixColor(a.Top, b.Top, t), Right: mixColor(a.Right, b.Right, t),
		Bottom: mixColor(a.Bottom, b.Bottom, t), Left: mixColor(a.Left, b.Left, t),
	}
}

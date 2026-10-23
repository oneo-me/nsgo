//go:build darwin && !ios

package macos

import (
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/platform/internal/apple"
)

// The surface's frame clock steps the native animation graph and the
// layout animations once a frame while either runs.

var selTick = sel("nsgoTick:")

// motion is a view's part of a layout animation: its frame moving, or
// the view appearing.
type motion struct {
	from, to core.Bounds
	frame    *core.LayoutTransition // moves from to to
	appear   *core.LayoutTransition // fades or grows in
	start    time.Duration
}

// ghost is a view a layout animation removes, left in place while it
// fades or shrinks away, then released.
type ghost struct {
	native  objc.ID
	opacity float32
	tr      *core.LayoutTransition
	start   time.Duration
}

// clock returns the frame clock's time.
func (s *surface) clock() time.Duration { return time.Since(s.epoch) }

// startTicking runs the frame clock until nothing animates: AppKit's
// display link where there is one (macOS 14), else a timer at the usual
// 60 Hz, both in the run loop's common modes so they keep running while a
// menu tracks or a window resizes.
func (s *surface) startTicking() {
	if s.ticking {
		return
	}
	s.ticking = true
	if s.link != 0 {
		s.link.Send(sel("setPaused:"), false)
		return
	}
	loop := objc.ID(objc.GetClass("NSRunLoop")).Send(sel("mainRunLoop"))
	if objc.Send[bool](s.native, sel("respondsToSelector:"), sel("displayLinkWithTarget:selector:")) {
		s.link = s.native.Send(sel("displayLinkWithTarget:selector:"), s.native, selTick).Send(sel("retain"))
		if s.link == 0 {
			// No window yet: a link needs the view to be in one. Leave
			// ticking off so the next mount asks again.
			s.ticking = false
			return
		}
		s.link.Send(sel("addToRunLoop:forMode:"), loop, apple.CommonModes())
		return
	}
	s.timer = objc.ID(objc.GetClass("NSTimer")).Send(sel("timerWithTimeInterval:target:selector:userInfo:repeats:"),
		1.0/60, s.native, selTick, objc.ID(0), true).Send(sel("retain"))
	loop.Send(sel("addTimer:forMode:"), s.timer, apple.CommonModes())
}

func (s *surface) stopTicking() {
	if !s.ticking {
		return
	}
	s.ticking = false
	if s.link != 0 {
		s.link.Send(sel("setPaused:"), true)
		return
	}
	s.timer.Send(sel("invalidate"))
	s.timer.Send(selRelease)
	s.timer = 0
}

func surfaceTick(self objc.ID, _ objc.SEL, _ objc.ID) {
	if v := views[self]; v != nil {
		v.surface.tick()
	}
}

// tick steps one frame: the engine's frame callbacks first, then the
// native animations the same clock drives.
func (s *surface) tick() {
	if s.engine != nil && s.engine.WantsFrames() {
		s.engine.Frame()
	}
	now := s.clock()
	if s.animator.Running() {
		s.animator.Step(now)
	}
	apple.WithoutAnimation(func() {
		s.stepMotions(now)
		s.applyAnimated()
	})
	s.startImages()
	if !s.animator.Running() && len(s.motions) == 0 && len(s.ghosts) == 0 &&
		(s.engine == nil || !s.engine.WantsFrames()) {
		s.stopTicking()
	}
}

// applyAnimated draws the animated values that changed.
func (s *surface) applyAnimated() {
	s.animator.Changed(func(id core.NodeID, vals core.AnimatedValues) {
		if int(id) >= len(s.views) || s.views[id] == nil {
			return
		}
		v := s.views[id]
		v.animated = vals
		v.restyle()
	})
}

// restyle draws v's style with its animated values and what a layout
// animation does to it.
func (v *view) restyle() {
	vs := v.animated.Apply(v.style)
	vs.Transition = core.Transition{}
	if m := v.surface.motions[v]; m != nil && m.appear != nil {
		p, _ := m.appear.At(v.surface.clock() - m.start)
		if m.appear.Property == core.LayoutScale {
			sx, sy := vs.Transform.Scales()
			vs.Transform.ScaleX, vs.Transform.ScaleY = sx*float32(p), sy*float32(p)
		} else {
			vs.Opacity *= float32(p)
		}
	}
	layer := v.native.Send(selLayer)
	apple.StyleLayer(layer, &vs, true)
	v.transform = vs.Transform
}

// animatesStyle reports whether v's style is drawn by restyle rather than
// as the batch sends it.
func (v *view) animatesStyle() bool {
	if v.animated.Set != 0 {
		return true
	}
	m := v.surface.motions[v]
	return m != nil && m.appear != nil
}

// layoutBatch is what a batch with a layout animation tracks.
type layoutBatch struct {
	anim    *core.LayoutAnimation
	created map[core.NodeID]bool
	deleted map[objc.ID]bool // natives of views the batch deleted
}

// setFrame moves v, animating from where it shows now when the batch
// animates layout.
func (s *surface) setFrame(v *view, f core.Bounds, lb *layoutBatch) {
	m := s.motions[v]
	target := v.frame
	if m != nil && m.frame != nil {
		target = m.to
	}
	if f == target && v.framed {
		return
	}
	if lb != nil && lb.anim.Update != nil && !lb.created[v.id] && v.framed {
		from := v.frame
		if m == nil {
			m = &motion{}
			s.motions[v] = m
		}
		m.from, m.to, m.frame, m.start = from, f, lb.anim.Update, s.clock()
		s.startTicking()
		return
	}
	if m != nil {
		m.frame = nil
	}
	v.place(f)
}

// place sets v's frame and what follows it.
func (v *view) place(f core.Bounds) {
	v.frame, v.framed = f, true
	v.native.Send(selSetFrame, apple.Rect(float64(f.Origin.X), float64(f.Origin.Y),
		float64(f.Size.Width), float64(f.Size.Height)))
	if v.transform != (core.Transform{}) {
		apple.SetTransform(v.native.Send(selLayer), v.transform)
	}
	apple.LayoutBorders(v.native.Send(selLayer))
	if v.drawing != nil {
		v.native.Send(selSetNeedsDisplay, true)
	}
	if v.image != nil {
		v.image.Layout(apple.CGSize{Width: float64(f.Size.Width), Height: float64(f.Size.Height)})
		v.surface.images = append(v.surface.images, v)
	}
}

// appear starts v appearing, as the batch that created it animates.
func (s *surface) appear(v *view, lb *layoutBatch, parent core.NodeID) {
	if lb == nil || lb.anim.Create == nil || !lb.created[v.id] || lb.created[parent] {
		return
	}
	m := s.motions[v]
	if m == nil {
		m = &motion{}
		s.motions[v] = m
	}
	m.appear, m.start = lb.anim.Create, s.clock()
	s.startTicking()
}

// vanish keeps a deleted view's native in place, fading or shrinking it
// away, unless an ancestor the batch deleted already does. It reports
// whether it kept the view.
func (s *surface) vanish(v *view, lb *layoutBatch) bool {
	if lb == nil || lb.anim.Delete == nil || !v.framed {
		return false
	}
	up := v.native.Send(selSuperview)
	if up == 0 {
		return false
	}
	for n := up; n != 0; n = n.Send(selSuperview) {
		if lb.deleted[n] {
			return false
		}
	}
	lb.deleted[v.native] = true
	v.native.Send(sel("retain"))
	s.ghosts = append(s.ghosts, &ghost{native: v.native, opacity: objc.Send[float32](v.native.Send(selLayer), sel("opacity")),
		tr: lb.anim.Delete, start: s.clock()})
	s.startTicking()
	return true
}

// inGhost reports whether native is inside a view going away, which
// keeps its subviews until it goes.
func (s *surface) inGhost(native objc.ID) bool {
	if len(s.ghosts) == 0 {
		return false
	}
	for n := native.Send(selSuperview); n != 0; n = n.Send(selSuperview) {
		for _, g := range s.ghosts {
			if g.native == n {
				return true
			}
		}
	}
	return false
}

// stepMotions moves the layout animations to now.
func (s *surface) stepMotions(now time.Duration) {
	for v, m := range s.motions {
		if s.views[v.id] != v {
			delete(s.motions, v)
			continue
		}
		done := true
		if m.frame != nil {
			p, end := m.frame.At(now - m.start)
			v.place(core.LerpBounds(m.from, m.to, p))
			if end {
				v.place(m.to)
				m.frame = nil
			} else {
				done = false
			}
		}
		if m.appear != nil {
			if _, end := m.appear.At(now - m.start); end {
				m.appear = nil
			} else {
				done = false
			}
			v.restyle()
		}
		if done {
			delete(s.motions, v)
		}
	}
	live := s.ghosts[:0]
	for _, g := range s.ghosts {
		p, end := g.tr.At(now - g.start)
		layer := g.native.Send(selLayer)
		if g.tr.Property == core.LayoutScale {
			apple.SetTransform(layer, core.Transform{Scale: float32(max(1-p, 0.0001))})
		} else {
			layer.Send(sel("setOpacity:"), g.opacity*float32(1-p))
		}
		if end {
			apple.ForgetTransform(layer)
			g.native.Send(selRemoveFromSuperview)
			g.native.Send(selRelease)
			continue
		}
		live = append(live, g)
	}
	clear(s.ghosts[len(live):])
	s.ghosts = live
}

// panned moves the values bound to v's drags, before the pan is
// reported, so they follow in the same frame.
func (s *surface) panned(v *view, translation core.Point) {
	if s.animator.Pan(v.id, translation) {
		apple.WithoutAnimation(s.applyAnimated)
	}
}

//go:build darwin && !ios

package macos

import (
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// The surface's frame clock steps the native animation graph, the layout
// animations and the scroll views once a frame while one runs.

var (
	selTick      = sel("nsgoTick:")
	selWatch     = sel("nsgoWatch:")
	selStallTick = sel("nsgoStallTick:")
)

const (
	// frameBatches is how long a frame applies batches for while something
	// animates, at least one whole batch, as NSH's FRAME_BATCHES: a
	// scroll's frames are not delayed by batches landing between them,
	// and a burst of batches spreads over frames.
	frameBatches = 4 * time.Millisecond
	// stallAfter is how long the display link may go without a frame
	// while frames are wanted before a 60 Hz timer ticks in its place: the
	// link stops while the window is occluded or minimized or the screen
	// is locked, and animations must still end.
	stallAfter = 100 * time.Millisecond
)

// motion is a view's part of a layout animation: its frame moving, or
// the view appearing.
type motion struct {
	from, to driver.Bounds
	frame    *driver.LayoutTransition // moves from to to
	appear   *driver.LayoutTransition // fades or grows in
	start    time.Duration
}

// ghost is a view a layout animation removes, left in place while it
// fades or shrinks away, then released.
type ghost struct {
	native  objc.ID
	opacity float32
	tr      *driver.LayoutTransition
	start   time.Duration
}

// clock returns the frame clock's time.
func (s *surface) clock() time.Duration { return time.Since(s.epoch) }

// frameRateRange is what the display link asks for: the pace, or the
// screen's highest rate, at least 80 there, so a ProMotion display does not
// settle at 60 Hz.
type frameRateRange struct {
	Minimum, Maximum, Preferred float32
}

func (s *surface) frameRateRange() frameRateRange {
	if s.pace > 0 {
		return frameRateRange{float32(s.pace), float32(s.pace), float32(s.pace)}
	}
	max := float32(60)
	screen := s.window.Send(sel("screen"))
	if screen == 0 {
		screen = objc.ID(objc.GetClass("NSScreen")).Send(sel("mainScreen"))
	}
	if screen != 0 {
		if n := objc.Send[int](screen, sel("maximumFramesPerSecond")); n >= 1 {
			max = float32(n)
		}
	}
	return frameRateRange{Minimum: min(max, 80), Maximum: max, Preferred: max}
}

// startTicking runs the frame clock until nothing animates: AppKit's
// display link where there is one (macOS 14), else a timer at the usual
// 60 Hz, both in the run loop's common modes so they keep running while a
// menu tracks or a window resizes.
func (s *surface) startTicking() {
	if s.ticking {
		s.setPace()
		return
	}
	s.ticking = true
	s.pace = s.wantedPace()
	if s.link != 0 {
		// The window may have moved to another screen meanwhile.
		s.link.Send(sel("setPreferredFrameRateRange:"), s.frameRateRange())
		s.link.Send(sel("setPaused:"), false)
		s.watch()
		return
	}
	if objc.Send[bool](s.native, sel("respondsToSelector:"), sel("displayLinkWithTarget:selector:")) {
		// A link needs the view in a window: without one, the next mount
		// asks again.
		link := s.native.Send(sel("displayLinkWithTarget:selector:"), s.native, selTick)
		if link == 0 {
			s.ticking = false
			return
		}
		s.link = link.Send(sel("retain"))
		s.link.Send(sel("setPreferredFrameRateRange:"), s.frameRateRange())
		s.link.Send(sel("addToRunLoop:forMode:"), mainLoop(), apple.CommonModes())
		s.watch()
		return
	}
	s.timer = repeating(1.0/60, s.native, selTick)
}

// wantedPace is the rate the frame clock need run at: the animations' own
// when they are all it steps, else the screen's highest.
func (s *surface) wantedPace() float64 {
	if len(s.motions) > 0 || len(s.ghosts) > 0 || len(s.live) > 0 || s.frameWanted || s.selecting != nil || s.perf != nil {
		return 0
	}
	return s.animator.FrameRate()
}

// setPace moves the running link to the rate wanted now. The timers keep
// theirs: the animator holds a slowed run to its rate on any clock.
func (s *surface) setPace() {
	pace := s.wantedPace()
	if pace == s.pace {
		return
	}
	s.pace = pace
	if s.link != 0 {
		s.link.Send(sel("setPreferredFrameRateRange:"), s.frameRateRange())
	}
}

// repeating returns a retained timer that sends action to target every
// interval seconds in the main run loop's common modes.
func repeating(interval float64, target objc.ID, action objc.SEL) objc.ID {
	t := objc.ID(objc.GetClass("NSTimer")).Send(sel("timerWithTimeInterval:target:selector:userInfo:repeats:"),
		interval, target, action, objc.ID(0), true).Send(sel("retain"))
	mainLoop().Send(sel("addTimer:forMode:"), t, apple.CommonModes())
	return t
}

func mainLoop() objc.ID { return objc.ID(objc.GetClass("NSRunLoop")).Send(sel("mainRunLoop")) }

func invalidate(t *objc.ID) {
	if *t != 0 {
		t.Send(sel("invalidate"))
		t.Send(selRelease)
		*t = 0
	}
}

// watch starts the timer that checks, while the link runs, that it ticks.
func (s *surface) watch() {
	s.ticked = time.Now()
	if s.watcher == 0 {
		s.watcher = repeating(stallAfter.Seconds(), s.native, selWatch)
	}
}

func (s *surface) stopTicking() {
	if !s.ticking {
		return
	}
	s.ticking = false
	invalidate(&s.watcher)
	invalidate(&s.stall)
	if s.link != 0 {
		s.link.Send(sel("setPaused:"), true)
		return
	}
	invalidate(&s.timer)
}

func surfaceTick(self objc.ID, _ objc.SEL, _ objc.ID) {
	if v := views[self]; v != nil {
		s := v.surface
		// The link is back.
		invalidate(&s.stall)
		var period time.Duration
		if s.perf != nil {
			period = s.framePeriod()
		}
		s.tick(period)
	}
}

// surfaceWatch ticks with a timer once the display link stalled.
func surfaceWatch(self objc.ID, _ objc.SEL, _ objc.ID) {
	if v := views[self]; v != nil {
		if s := v.surface; s.ticking && s.stall == 0 && time.Since(s.ticked) >= stallAfter {
			s.stall = repeating(1.0/60, s.native, selStallTick)
		}
	}
}

func surfaceStallTick(self objc.ID, _ objc.SEL, _ objc.ID) {
	if v := views[self]; v != nil {
		v.surface.tick(time.Second / 60)
	}
}

// framePeriod is the display link's period, or 60 Hz where there is none.
func (s *surface) framePeriod() time.Duration {
	if s.link != 0 {
		stamp := objc.Send[float64](s.link, selTimestamp)
		target := objc.Send[float64](s.link, selTargetTimestamp)
		if target > stamp {
			return time.Duration((target - stamp) * float64(time.Second))
		}
	}
	return time.Second / 60
}

// tick steps one frame: the native animations first, then the frame the
// app waits for.
func (s *surface) tick(period time.Duration) {
	last := s.ticked
	s.ticked = time.Now()
	dirty := s.animator.Running() || len(s.motions) > 0 || len(s.ghosts) > 0 || len(s.images) > 0 || len(s.live) > 0
	var start time.Time
	if s.perf != nil {
		start = time.Now()
	}
	// The batches that waited for this frame, before it steps.
	if s.held() {
		s.applyBatches(frameBatches, false)
		dirty = true
	}
	if len(s.live) > 0 {
		s.tickScrolls(mediaTime())
	}
	selecting := s.autoscrollSelection(min(max(s.ticked.Sub(last), time.Second/240), time.Second/30))
	now := s.clock()
	if s.animator.Running() {
		s.animator.Step(now)
	}
	apple.WithoutAnimation(func() {
		s.stepMotions(now)
		s.applyAnimated()
	})
	if dirty || selecting {
		s.staleEffects()
	}
	s.drawShaders()
	s.startImages()
	if len(s.motions) > 0 {
		s.revisit()
	}
	if s.frameWanted {
		s.frameWanted = false
		if s.engine != nil {
			s.engine.Frame()
		}
	}
	if s.perf != nil {
		s.perf.RecordWork(time.Since(start), dirty)
		s.perf.Vsync(time.Now(), period)
	}
	if !s.animating() && !s.frameWanted && !s.held() && !selecting && s.perf == nil {
		s.stopTicking()
		return
	}
	s.setPace()
}

// animating reports whether something moves on the frame clock: the
// animation graph, a layout animation, or a scroll view moving or fading
// its indicator. Batches wait for the start of a frame meanwhile.
func (s *surface) animating() bool {
	return s.animator.Running() || len(s.motions) > 0 || len(s.ghosts) > 0 || len(s.live) > 0
}

// held reports whether batches wait for a frame.
func (s *surface) held() bool { return len(s.waiting) > 0 || s.batches.Len() > 0 }

// applyAnimated draws the animated values that changed.
func (s *surface) applyAnimated() {
	s.animator.Changed(func(id driver.NodeID, vals driver.AnimatedValues) {
		if int(id) >= len(s.views) || s.views[id] == nil {
			return
		}
		v := s.views[id]
		recolor := (v.animated.Has(driver.PropTextColor) || vals.Has(driver.PropTextColor)) &&
			(v.animated.TextColor != vals.TextColor || v.animated.Has(driver.PropTextColor) != vals.Has(driver.PropTextColor))
		v.animated = vals
		v.restyle()
		if recolor {
			v.redraw()
		}
	})
	s.animator.ChangedCanvases(func(id driver.NodeID) {
		if int(id) < len(s.views) && s.views[id] != nil {
			s.views[id].redrawAnimated()
		}
	})
	s.animator.ChangedNatives(func(id driver.NodeID, b driver.NativeBinding, num float64, col driver.Color) {
		if int(id) < len(s.views) && s.views[id] != nil {
			driver.SetAnimatedProp(s.views[id].component, b, num, col)
		}
	})
	s.repinBound()
}

// restyle draws v's style with its animated values and what a layout
// animation does to it.
func (v *view) restyle() {
	vs := v.animated.Apply(v.style)
	vs.Transition = driver.Transition{}
	if m := v.surface.motions[v]; m != nil && m.appear != nil {
		p, _ := m.appear.At(v.surface.clock() - m.start)
		if m.appear.Property == driver.LayoutScale {
			sx, sy := vs.Transform.Scales()
			vs.Transform.ScaleX, vs.Transform.ScaleY = sx*float32(p), sy*float32(p)
		} else {
			vs.Opacity *= float32(p)
		}
	}
	v.styleLayer(&vs)
}

// styleLayer draws vs with v's layer, only what changed since it was
// last drawn when that is all an animation moves.
func (v *view) styleLayer(vs *driver.ViewStyle) {
	// A view created by an earlier batch shows the default style.
	var shown *driver.ViewStyle
	if v.styled || v.born != v.surface.batch {
		shown = &v.drawn
	}
	apple.RestyleLayer(v.native.Send(selLayer), shown, vs, v.native)
	v.drawn, v.styled = *vs, true
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
	anim    *driver.LayoutAnimation
	created map[driver.NodeID]bool
	deleted map[objc.ID]bool // natives of views the batch deleted
}

// setFrame moves v to the item's frame, animating from where it shows now when
// the batch animates layout or the item carries a transition.
func (s *surface) setFrame(v *view, it *driver.MountItem, lb *layoutBatch) {
	f := it.Frame
	m := s.motions[v]
	target := v.frame
	if m != nil && m.frame != nil {
		target = m.to
	}
	if f == target && v.framed {
		return
	}
	tr := it.Transition
	if lb != nil && lb.anim.Update != nil && !lb.created[v.id] {
		tr = lb.anim.Update
	}
	if tr != nil && v.framed {
		from := v.frame
		if m == nil {
			m = &motion{}
			s.motions[v] = m
		}
		m.from, m.to, m.frame, m.start = from, f, tr, s.clock()
		s.startTicking()
		return
	}
	if m != nil {
		m.frame = nil
	}
	v.place(f)
}

// place sets v's frame and what follows it.
func (v *view) place(f driver.Bounds) {
	resized := f.Size != v.frame.Size
	v.frame, v.framed = f, true
	f.Origin.X += v.shift.X
	f.Origin.Y += v.shift.Y
	v.native.Send(selSetFrame, apple.Rect(float64(f.Origin.X), float64(f.Origin.Y),
		float64(f.Size.Width), float64(f.Size.Height)))
	if v.transform != (driver.Transform{}) {
		apple.SetTransform(v.native.Send(selLayer), v.transform)
	}
	apple.LayoutDecor(v.native.Send(selLayer))
	if v.backdrop != 0 && !v.glass {
		v.fitBackdrop(&v.style)
	}
	if resized {
		v.sizeContent()
		if v.sc != nil {
			v.sizeScroll()
		}
	}
	if v.image != nil {
		v.image.Layout(apple.CGSize{Width: float64(f.Size.Width), Height: float64(f.Size.Height)})
		v.surface.images = append(v.surface.images, v)
	}
}

// appear starts v appearing, as the batch that created it animates.
func (s *surface) appear(v *view, lb *layoutBatch, parent driver.NodeID) {
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
			v.place(driver.LerpBounds(m.from, m.to, p))
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
		if g.tr.Property == driver.LayoutScale {
			apple.SetTransform(layer, driver.Transform{Scale: float32(max(1-p, 0.0001))})
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

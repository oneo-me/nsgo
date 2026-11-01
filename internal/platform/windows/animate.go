//go:build windows

package windows

import (
	"slices"
	"sync/atomic"
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
)

var (
	setTimer  = user32.NewProc("SetTimer")
	killTimer = user32.NewProc("KillTimer")
	dwmFlush  = dwmapi.NewProc("DwmFlush")
)

const (
	wmTimer = 0x0113
	// wmTick is a frame of the frame clock.
	wmTick = 0x8000 + 7
	// frameInterval is a frame's length where the compositor gives none.
	frameInterval  = 16 * time.Millisecond
	scrollDuration = 300 * time.Millisecond

	allProperties = driver.TransitionOpacity | driver.TransitionBackground | driver.TransitionTransform |
		driver.TransitionBorder | driver.TransitionShadow
)

// transition is a view's style animating from one ViewStyle to the next.
type transition struct {
	from, to driver.ViewStyle
	start    time.Time // after the delay
	duration time.Duration
	curve    driver.Curve
	props    driver.TransitionProperty
	shown    driver.ViewStyle // at the last frame
}

// scrolling is a scroll view's offset animating to a point.
type scrolling struct {
	from, to driver.Point
	start    time.Time
	momentum bool // a snap, which reports EventMomentumEnd
}

// moving is a view's frame animating to the one it was given, as a
// LayoutAnimation's Update moves it.
type moving struct {
	from  driver.Bounds
	start time.Time // when the batch came
	tr    driver.LayoutTransition
	shown driver.Bounds // at the last frame
}

// fading is a view appearing or going, as a LayoutAnimation's Create and
// Delete show it: shown multiplies its opacity, or scales it about its
// center, by the transition's property.
type fading struct {
	start time.Time
	tr    driver.LayoutTransition
	out   bool
	from  float64 // what showed when it started going
	shown float64 // at the last frame: 0 none of it, 1 all
}

// leaving is a child of a view that a LayoutAnimation's Delete keeps
// drawn while it goes: at is where it was among the children.
type leaving struct {
	v  *view
	at int
}

// layoutBatch is what a batch with a LayoutAnimation animates.
type layoutBatch struct {
	anim     *driver.LayoutAnimation
	now      time.Time
	created  map[*view]bool
	inserted []*view // the views created and inserted
}

// layoutBatch returns what b animates, or nil when it animates nothing:
// it has no LayoutAnimation, or the user turned animation effects off.
func (w *window) layoutBatch(b *driver.MountBatch) *layoutBatch {
	if b.LayoutAnimation == nil || w.env.ReduceMotion {
		return nil
	}
	return &layoutBatch{anim: b.LayoutAnimation, now: time.Now(), created: map[*view]bool{}}
}

// frameOf returns the frame v is drawn in now: mid animation, or its own.
func frameOf(v *view) driver.Bounds {
	if v.moving != nil {
		return v.moving.shown
	}
	return v.frame
}

// setFrame gives v its new frame, which a batch's Update animates to
// from the one v shows when v existed before the batch.
func (w *window) setFrame(v *view, f driver.Bounds, lb *layoutBatch) {
	old := frameOf(v)
	v.laid = f
	f.Origin.X += v.shift.X
	f.Origin.Y += v.shift.Y
	v.frame = f
	if lb == nil || lb.anim.Update == nil || lb.created[v] || old == (driver.Bounds{}) || old == f {
		v.moving = nil
		return
	}
	v.moving = &moving{from: old, start: lb.now, tr: *lb.anim.Update, shown: old}
	w.animate(v)
}

// appear fades or scales in the views a batch created and inserted, those
// whose parent was there before it, as its Create says.
func (w *window) appear(lb *layoutBatch) {
	if lb == nil || lb.anim.Create == nil {
		return
	}
	for _, v := range lb.inserted {
		if !v.deleted && v.parent != nil && !lb.created[v.parent] {
			v.fading = &fading{start: lb.now, tr: *lb.anim.Create}
			w.animate(v)
		}
	}
}

// leave keeps v, which a batch deletes, drawn as a batch's Delete fades or
// shrinks it out, and reports whether it does: v is the top of the
// subtree going, or in one. Its ID is free at once, for the batch to use
// again.
func (w *window) leave(v *view, lb *layoutBatch) bool {
	p := v.parent
	if p != nil && p.retired {
		// Goes with the subtree it is in.
		w.retire(v)
		return true
	}
	if lb == nil || lb.anim.Delete == nil || p == nil || p.deleted || !w.shown(v) || v.frame.Size == (driver.Size{}) {
		return false
	}
	from := 1.0
	if fd := v.fading; fd != nil {
		from = fd.shown
	}
	at := slices.Index(p.children, v)
	v.detach()
	p.leaving = append(p.leaving, leaving{v: v, at: at})
	slices.SortStableFunc(p.leaving, func(a, b leaving) int { return a.at - b.at })
	v.leftFrom = p
	w.retire(v)
	v.fading = &fading{start: lb.now, tr: *lb.anim.Delete, out: true, from: from, shown: from}
	w.animate(v)
	return true
}

// finishLeaving destroys v, done going, and its subtree.
func (w *window) finishLeaving(v *view) {
	w.redrawView(v)
	w.destroy(v)
}

// baseStyle returns v's style before animated values: mid transition, or
// its own.
func (w *window) baseStyle(v *view) *driver.ViewStyle {
	if v.transition != nil {
		return &v.transition.shown
	}
	return &v.style
}

// styleOf returns the style v shows now: mid transition, or its own,
// with the animated values bound to it over it.
func (w *window) styleOf(v *view) *driver.ViewStyle {
	s := w.baseStyle(v)
	if v.animated.Set == 0 {
		return s
	}
	v.drawn = v.animated.Apply(*s)
	return &v.drawn
}

// applyAnimations applies a batch's changes to the animation graph.
func (w *window) applyAnimations(ops []driver.AnimationOp) {
	w.animator.Apply(ops)
	if w.animator.Running() {
		w.startClock()
	}
}

// animatedChanged gives the views whose bound values changed their new
// ones, to draw in the next frame.
func (w *window) animatedChanged() {
	w.animator.Changed(func(id driver.NodeID, vals driver.AnimatedValues) {
		if v := w.view(id); v != nil {
			w.redrawView(v)
			v.animated = vals
		}
	})
	w.changedCanvases()
	w.changedNatives()
	w.changedShaders()
	w.repinBound()
}

// gestured moves the values bound to a pan, pinch or rotation before
// it is reported, for the next frame to draw them there, and reports it.
func (w *window) gestured(ev driver.Event) {
	if w.animator.Gesture(&ev) {
		w.animatedChanged()
	}
	w.engine.Dispatch(ev)
}

// setStyle applies v's new style, animating to it when it has a
// transition. Windows turns its own animations off when the user turns
// off animation effects, so these snap then.
func (w *window) setStyle(v *view, s *driver.ViewStyle) {
	from := *w.baseStyle(v)
	styled := v.styled
	if s.Animation != v.style.Animation || !styled {
		v.keyframeStart = time.Now()
		if s.Animation.Keyframes != 0 {
			w.animate(v)
		}
	}
	v.style, v.styled, v.transition = *s, true, nil
	t := s.Transition
	if !styled || t.Duration <= 0 || w.env.ReduceMotion {
		return
	}
	props := t.Properties
	if props == 0 {
		props = allProperties
	}
	first, last := driver.MixStyles(from, *s, 0, props), driver.MixStyles(from, *s, 1, props)
	if first == last {
		return
	}
	v.transition = &transition{from: from, to: *s, start: time.Now().Add(t.Delay), duration: t.Duration,
		curve: t.Easing, props: props, shown: first}
	w.animate(v)
}

// scrollAnimated scrolls v to offset over a moment, or at once when the
// user turned animation effects off.
func (w *window) scrollAnimated(v *view, offset driver.Point) {
	if w.env.ReduceMotion {
		w.scrollTo(v, offset)
		return
	}
	v.scrolling = &scrolling{from: v.offset, to: offset, start: time.Now()}
	w.animate(v)
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

// onFrame calls fn once a frame, with the frame's time, until it reports
// false: videos copy their frames from it, animated images change theirs.
func (w *window) onFrame(fn func(now time.Time) bool) {
	w.frames = append(w.frames, fn)
	w.startClock()
}

// frameClock follows the compositor, as NSH's ticker does: a goroutine
// waits for each composition (DwmFlush) and posts wmTick, one at a time,
// while the window animates, and parks while it does not. A timer's
// ticks fall between the compositor's frames, which the system's timer
// granularity (15.6 ms by default) makes uneven too.
type frameClock struct {
	hwnd uintptr
	wake chan struct{}
	done chan struct{}
	// running is set while ticks are wanted; posted while one waits for
	// the window.
	running, posted atomic.Bool
}

func newFrameClock(hwnd uintptr) *frameClock {
	c := &frameClock{hwnd: hwnd, wake: make(chan struct{}, 1), done: make(chan struct{})}
	go c.run()
	return c
}

// minTick is the least time between ticks, should DwmFlush return at
// once (nothing composing yet): faster than any display's frames.
const minTick = 3 * time.Millisecond

func (c *frameClock) run() {
	var last time.Time
	for {
		if !c.running.Load() {
			select {
			case <-c.wake:
				continue
			case <-c.done:
				return
			}
		}
		// It fails where nothing composes (a session without a desktop):
		// a frame's length then.
		if hr, _, _ := dwmFlush.Call(); int32(hr) < 0 {
			time.Sleep(frameInterval)
		} else if since := time.Since(last); since < minTick {
			time.Sleep(minTick - since)
		}
		last = time.Now()
		select {
		case <-c.done:
			return
		default:
		}
		if c.running.Load() && c.posted.CompareAndSwap(false, true) {
			postMessageW.Call(c.hwnd, wmTick, 0, 0)
		}
	}
}

func (c *frameClock) start() {
	c.running.Store(true)
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

func (c *frameClock) stop() { c.running.Store(false) }

// close ends the goroutine.
func (c *frameClock) close() { close(c.done) }

// startClock starts the frame clock, which tick stops when nothing
// animates: no transition, scroll or layout animation, no animated value
// running and no frame client.
func (w *window) startClock() {
	if w.ticking {
		return
	}
	if w.clock == nil {
		w.clock = newFrameClock(w.hwnd)
	}
	w.clock.start()
	w.ticking = true
}

// ticked runs the frame the clock posted.
func (w *window) ticked() {
	if w.clock != nil {
		w.clock.posted.Store(false)
	}
	w.vsync()
	if w.ticking {
		done := w.measuring(true)
		w.tick()
		done()
	}
}

// tick moves every animation to now, runs the frame clients and repaints.
func (w *window) tick() {
	now := time.Now()
	frames := w.frames
	w.frames = nil
	for _, fn := range frames {
		if fn(now) {
			w.frames = append(w.frames, fn)
		}
	}
	live := w.animating[:0]
	for _, v := range w.animating {
		// A view a layout animation keeps going is retired, not gone.
		if v.deleted && !v.retired {
			continue
		}
		w.redrawView(v)
		if mv := v.moving; mv != nil {
			p, done := mv.tr.At(now.Sub(mv.start))
			mv.shown = driver.LerpBounds(mv.from, v.frame, p)
			if done {
				v.moving = nil
			}
		}
		if fd := v.fading; fd != nil {
			p, done := fd.tr.At(now.Sub(fd.start))
			if fd.out {
				fd.shown = fd.from * (1 - p)
			} else {
				fd.shown = p
			}
			if done {
				v.fading = nil
				if fd.out {
					w.finishLeaving(v)
					continue
				}
			}
		}
		if t := v.transition; t != nil {
			x := 0.0
			if now.After(t.start) {
				x = min(float64(now.Sub(t.start))/float64(t.duration), 1)
			}
			t.shown = driver.MixStyles(t.from, t.to, t.curve.At(x), t.props)
			if x >= 1 {
				v.transition = nil
			}
		}
		if sc := v.scrolling; sc != nil && !v.deleted {
			x := min(float64(now.Sub(sc.start))/float64(scrollDuration), 1)
			e := driver.Points(driver.EaseInOut.At(x))
			at := driver.Point{X: sc.from.X + (sc.to.X-sc.from.X)*e, Y: sc.from.Y + (sc.to.Y-sc.from.Y)*e}
			w.scrollTo(v, at)
			if x >= 1 {
				v.scrolling = nil
				if sc.momentum {
					w.scrollEvent(v, driver.EventMomentumEnd)
				}
				w.restScroll(v)
			}
		}
		_, keyframed := v.keyframe()
		if keyframed {
			_, keyframed = v.style.Animation.At(now.Sub(v.keyframeStart))
		}
		if v.transition != nil || v.scrolling != nil || v.moving != nil || v.fading != nil || keyframed {
			live = append(live, v)
		}
	}
	clear(w.animating[len(live):])
	w.animating = live
	// Animated values, on the same clock: monotonic time since the window
	// opened.
	if w.animator.Running() {
		w.animator.Step(now.Sub(w.epoch))
	}
	w.animatedChanged()
	if w.frameWanted {
		w.frameWanted = false
		if w.engine != nil {
			w.engine.Frame()
		}
	}
	// A monitor closes a frame at every composition.
	if len(live) == 0 && len(w.frames) == 0 && !w.animator.Running() && !w.frameWanted && w.hud == nil {
		w.clock.stop()
		w.ticking = false
	}
}

// keyframe returns the step of v's keyframe animation now, and whether
// one runs.
func (v *view) keyframe() (driver.Keyframe, bool) {
	a := v.style.Animation
	if a.Keyframes == 0 {
		return driver.Keyframe{Opacity: 1, Scale: 1}, false
	}
	k, _ := a.At(time.Since(v.keyframeStart))
	return k, true
}

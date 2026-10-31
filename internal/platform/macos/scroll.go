//go:build darwin && !ios

package macos

import (
	"math"
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
	"github.com/oneo-me/nsgo/internal/platform/scrollbar"
)

// Scroll views keep offsets of their own, as NSH's macOS backend does. A
// scroll view is a plain view holding its children in a container that
// fills it. Its offset, UIKit's contentOffset, lives in scrollState and
// shows as the container layer's sublayerTransform while it moves, so a
// scroll frame is one layer write. The container's bounds origin takes
// it, as NSClipView scrolls, once the view and the frame clock rest
// (commitDue): setBoundsOrigin: walks the whole subtree, which drops
// frames of a long list. AppKit's geometry (hit tests, conversions,
// tracking areas) follows the bounds, so what asks it first commits
// (commitPending): a press, a mouse move, a scroll gesture's start, a
// selection.
//
// A scroll view moves along its main axis only. Trackpads follow the
// system: a gesture rubber-bands past the edges unless NoBounces and
// springs back when the fingers lift, and the momentum after it is the
// system's, shown in step with the display (scrollbar.Resampler);
// snapping views snap from the release instead. Mouse wheels move 10
// points a line, stop at the edges, and snapping views settle once the
// notches stop. A scroll stays on the view it began in: a gesture and its
// momentum until they end, wheel notches while they come within 200 ms.
// Mouse drags never scroll; the mouse drags the indicator as a scroll
// bar. Pulling a refreshable view past its top refreshes it, and a
// spinner, or the app's own indicator, keeps room above the content while
// it refreshes. A view that zooms moves along both axes instead
// (zoom.go). DecelerationRate shapes only snaps; the keyboard props do
// nothing on the Mac, as in NSH.

const (
	// wheelLine is how far a mouse wheel's line scrolls, and wheelLatch
	// how long its notches stay on the view they began on.
	wheelLine  = 10
	wheelLatch = 200 * time.Millisecond
	// momentumWait is how soon after a gesture the system's momentum
	// begins, if it does.
	momentumWait = 100 * time.Millisecond
	// commitDelay is how long after the last move the bounds commit, and
	// restWait the longest it waits for the frame clock to rest once the
	// view does: past the indicator's fade, so a scroll alone commits when
	// nothing draws, while an endless animation holds it back briefly.
	commitDelay = 140 * time.Millisecond
	restWait    = time.Second
	// A press within thumbGrab of the edge and thumbSlop of the
	// indicator's ends grabs it, as it is thinner than a pointer is
	// precise. Its thickness, inset and margins are scrollbar's.
	thumbGrab driver.Points = 12
	thumbSlop driver.Points = 4
)

// phaseMayBegin is the NSEvent phase of fingers resting on the trackpad
// (the others are gesture.go's).
const phaseMayBegin = 32

// phaseKind is what moves a scroll view.
type phaseKind uint8

const (
	atRest phaseKind = iota
	// onThumb: the mouse drags the indicator, from offset from.
	onThumb
	// wheeling: wheel or trackpad input, the last at last and total
	// points so far; gesture for a trackpad's, whose deltas put the
	// offset at raw before the rubber band.
	wheeling
	// awaitingMomentum: a trackpad gesture ended at last; the system's
	// momentum may follow.
	awaitingMomentum
	// inMomentum: the system's momentum scrolls.
	inMomentum
	// easing: an eased move from from to to, from start for duration;
	// momentum for a snap after a release, settles to let the values that
	// snap settle as it arrives. A zooming view eases its zoom too.
	easing
	// pinching: a trackpad pinch zooms the view.
	pinching
)

type scrollPhase struct {
	kind                       phaseKind
	from, to, raw, total       driver.Points
	last, start, duration      time.Duration
	gesture, momentum, settles bool
	// A zooming view's: rawAt and rawZoom are where a gesture's deltas
	// and a pinch's put the offset and the zoom before the rubber band,
	// and zoom is an easing (zoom.go).
	rawAt   driver.Point
	rawZoom float64
	zoom    zoomEase
}

// scrollState is a scroll view's offset and what moves it.
type scrollState struct {
	config  driver.ScrollConfig
	content driver.Size
	// at is the offset, which shows as the container layer's translation
	// shift until the container's bounds origin takes it: committed.
	at, committed, shift driver.Point
	// zoom is the content's scale, which the container's bounds take at
	// once, as zoomed.
	zoom, zoomed float64
	phase        scrollPhase
	// resampled holds where trackpad events put the offset until frames
	// show it.
	resampled scrollbar.Resampler
	// ticking says the view runs on the frame clock; report that the
	// offset moved since it was last reported.
	ticking, report bool
	// The indicator: when it last showed, its opacity and whether it
	// shows for a dark appearance.
	shown time.Duration
	alpha float32
	dark  bool
	// refreshing shows the spinner, turned spin radians.
	refreshing bool
	spin       float64
	// overlay holds the indicator's layer, thumb, and the spinner's with
	// its spokes above the container; drawn is what the indicator last
	// showed.
	overlay, thumb, spinner objc.ID
	spokes                  [refreshSpokes]objc.ID
	drawn                   indicatorLook
	// edges are the effects at its sides.
	edges [4]edgeView
	// The bounds commit is due at commitAt; armed while a timer waits
	// for it, which saw the view rest from restedAt.
	commitAt, restedAt time.Duration
	armed              bool
	// applied counts the core's ItemScrollTo and ItemScrollBy items,
	// reported with each offset; sticky are its sticky cells.
	applied uint32
	sticky  []driver.StickyCell
}

// indicatorLook is how the indicator shows.
type indicatorLook struct {
	frame        driver.Bounds
	alpha        float32
	dark, hidden bool
}

// mediaTime is the clock scroll views move on: Core Animation's, which
// NSEvent timestamps count on too.
func mediaTime() time.Duration { return apple.MediaTime() }

func eventTime(event objc.ID) time.Duration {
	return time.Duration(objc.Send[float64](event, selTimestamp) * float64(time.Second))
}

// newScroll makes scroll view v's container, where its children go. The
// container clips them, as NSClipView does: the view's own layer clips as
// its style says.
func (v *view) newScroll() {
	v.sc = &scrollState{zoom: 1, zoomed: 1}
	v.hover = trackingArea(v.native, true)
	v.content = objc.ID(classContainer).Send(selNew)
	v.content.Send(selSetWantsLayer, true)
	v.content.Send(selLayer).Send(sel("setMasksToBounds:"), true)
	v.native.Send(selAddSubview, v.content)
	v.content.Send(selRelease)
}

// main returns p along the view's axis.
func (v *view) main(p driver.Point) driver.Points {
	if v.sc.config.Horizontal {
		return p.X
	}
	return p.Y
}

// setMain sets p along the view's axis.
func (v *view) setMain(p *driver.Point, to driver.Points) {
	if v.sc.config.Horizontal {
		p.X = to
	} else {
		p.Y = to
	}
}

// pos is the offset along the view's axis.
func (v *view) pos() driver.Points { return v.main(v.sc.at) }

// viewport is the view's length along its axis.
func (v *view) viewport() driver.Points {
	return v.main(driver.Point{X: v.frame.Size.Width, Y: v.frame.Size.Height})
}

// lead and trail are the content insets before and after the content
// along the axis.
func (v *view) lead() driver.Points {
	in := v.sc.config.Behavior.ContentInset
	return v.main(driver.Point{X: in.Left, Y: in.Top})
}

func (v *view) trail() driver.Points {
	in := v.sc.config.Behavior.ContentInset
	return v.main(driver.Point{X: in.Right, Y: in.Bottom})
}

// restLimits are the offsets the view rests within along its axis.
func (v *view) restLimits() (lo, hi driver.Points) {
	return v.axisLimits(v.sc.config.Horizontal, v.sc.zoom)
}

// axisLimits are the offsets the view rests within along an axis at zoom
// z: from the start of the content inset to the end of the content and
// its trailing inset. A zooming view with CenterContent centers content
// smaller than the viewport.
func (v *view) axisLimits(horizontal bool, z float64) (lo, hi driver.Points) {
	c := &v.sc.config
	in := c.Behavior.ContentInset
	lead, trail, length, viewport := in.Top, in.Bottom, v.sc.content.Height, v.frame.Size.Height
	if horizontal {
		lead, trail, length, viewport = in.Left, in.Right, v.sc.content.Width, v.frame.Size.Width
	}
	length *= driver.Points(z)
	lo = -lead
	if c.CenterContent && c.Zooms() && lead+length+trail < viewport {
		lo -= (viewport - lead - length - trail) / 2
		return lo, lo
	}
	return lo, max(length+trail-viewport, lo)
}

// limits are restLimits with the spinner's room above the content while
// it refreshes.
func (v *view) limits() (lo, hi driver.Points) {
	lo, hi = v.restLimits()
	if v.sc.refreshing && !v.sc.config.Horizontal {
		lo -= v.sc.config.Behavior.RefreshInset()
	}
	return lo, hi
}

// reported is the offset events and the engine see: within the content
// and its insets, as NSH's.
func (v *view) reported() driver.Point {
	p := v.sc.at
	if v.zooms() {
		return v.clampAt(p, v.sc.zoom)
	}
	lo, hi := v.restLimits()
	v.setMain(&p, min(max(v.pos(), lo), hi))
	return p
}

// indicates reports whether the view shows its indicator, which the
// mouse drags as a scroll bar. Unlike NSH's, a horizontal view hides it
// too when the app says so, as on NSGo's other desktops.
func (v *view) indicates() bool { return !v.sc.config.HideIndicator }

// moveTo moves the offset along the axis to pos, unclamped since gestures
// rubber-band, with what follows it: the translation, the sticky cells,
// the indicator, the values bound to the offset and what comes into view.
// The frame clock reports it.
func (v *view) moveTo(pos driver.Points) {
	at := v.sc.at
	v.setMain(&at, pos)
	v.moveZoom(at, v.sc.zoom)
}

// moveZoom is moveTo of an offset along both axes at zoom z.
func (v *view) moveZoom(at driver.Point, z float64) {
	s := v.sc
	if s.at == at && s.zoom == z {
		return
	}
	pos := v.main(at)
	s.at, s.zoom = at, z
	if v.indicates() {
		s.shown, s.alpha, s.dark = mediaTime(), 1, v.surface.env.Dark
	}
	s.report = true
	apple.WithoutAnimation(func() {
		v.applyOffset()
		v.pinSticky()
		v.updateOverlay()
	})
	sf := v.surface
	scrolled := sf.animator.Scroll(v.id, v.reported())
	if !s.config.Horizontal && sf.animator.Pull(v.id, max(-v.lead()-pos, 0)) {
		scrolled = true
	}
	// While a batch applies, the animated values are drawn after it.
	if scrolled && !sf.applying {
		apple.WithoutAnimation(sf.applyAnimated)
	}
	sf.revisit()
	if apple.Msg(v.native, selWindow) != 0 {
		v.tickOn()
	} else {
		v.reportScroll() // no frames come out of a window
	}
}

// applyOffset shows the offset as the container layer's translation from
// its bounds: the pixels move without AppKit's geometry changing, which
// commit then catches up with. A new zoom commits at once.
func (v *view) applyOffset() {
	s := v.sc
	if s.zoom != s.zoomed {
		v.commit()
		return
	}
	// The translation is in the content's points, which the bounds scale.
	z := driver.Points(s.zoomed)
	shift := driver.Point{X: (s.committed.X - s.at.X) / z, Y: (s.committed.Y - s.at.Y) / z}
	if shift != s.shift {
		s.shift = shift
		apple.SetSublayerTranslation(v.content.Send(selLayer), float64(shift.X), float64(shift.Y))
	}
	if shift != (driver.Point{}) {
		v.armCommit()
	}
}

// armCommit commits the offset to the container's bounds commitDelay
// after it last moved.
func (v *view) armCommit() {
	s := v.sc
	s.commitAt = mediaTime() + commitDelay
	if s.armed {
		return
	}
	s.armed = true
	if v.surface.armed == nil {
		v.surface.armed = map[*view]struct{}{}
	}
	v.surface.armed[v] = struct{}{}
	v.later(commitDelay, v.commitWhenDue)
}

// later runs fn on the main thread after d, if v is still mounted.
func (v *view) later(d time.Duration, fn func()) {
	time.AfterFunc(d, func() {
		apple.OnMain(func() {
			if s := v.surface; !s.closed && int(v.id) < len(s.views) && s.views[v.id] == v {
				fn()
			}
		})
	})
}

func (v *view) commitWhenDue() {
	s := v.sc
	if !s.armed {
		return
	}
	if v.surface.applying {
		v.later(20*time.Millisecond, v.commitWhenDue)
		return
	}
	if !commitDue(mediaTime(), s.commitAt, s.phase.kind == atRest, v.surface.ticking, &s.restedAt) {
		v.later(32*time.Millisecond, v.commitWhenDue)
		return
	}
	v.commit()
}

// commitDue reports whether an armed bounds commit lands at t: the offset
// has not moved since commitAt and the view rests (idle), then once the
// frame clock rests too (busy false) or restWait after the view came to
// rest, which restedAt keeps (0 while it has not). Input and glides go on
// as long as they last, moving the translation only.
func commitDue(t, commitAt time.Duration, idle, busy bool, restedAt *time.Duration) bool {
	if commitAt-t > time.Millisecond || !idle {
		*restedAt = 0
		return false
	}
	if *restedAt == 0 {
		*restedAt = t
	}
	return !busy || t-*restedAt >= restWait
}

// commit moves the offset the translation shows into the container's
// bounds: one walk of the subtree tells AppKit where views are.
func (v *view) commit() {
	s := v.sc
	s.armed, s.restedAt = false, 0
	delete(v.surface.armed, v)
	apple.WithoutAnimation(func() {
		if s.committed != s.at || s.zoomed != s.zoom {
			s.committed, s.zoomed = s.at, s.zoom
			v.content.Send(sel("setBounds:"), v.contentBounds())
		}
		if s.shift != (driver.Point{}) {
			s.shift = driver.Point{}
			apple.SetSublayerTranslation(v.content.Send(selLayer), 0, 0)
		}
	})
}

// contentBounds are the container's bounds for the committed offset and
// zoom: AppKit scales what bounds smaller than its frame hold.
func (v *view) contentBounds() apple.CGRect {
	s := v.sc
	z := s.zoomed
	return apple.Rect(float64(s.committed.X)/z, float64(s.committed.Y)/z,
		float64(v.frame.Size.Width)/z, float64(v.frame.Size.Height)/z)
}

// commitPending commits the offsets still shown as translations, before
// something asks AppKit where views are.
func (s *surface) commitPending() {
	for v := range s.armed {
		v.commit()
	}
}

// sizeScroll keeps the container and the overlay over the view as its
// size changes.
func (v *view) sizeScroll() {
	s := v.sc
	w, h := float64(v.frame.Size.Width), float64(v.frame.Size.Height)
	apple.WithoutAnimation(func() {
		v.content.Send(selSetFrame, apple.Rect(0, 0, w, h))
		if b := v.contentBounds(); objc.Send[apple.CGRect](v.content, selBounds) != b {
			v.content.Send(sel("setBounds:"), b)
		}
		// AppKit lays the layer out again.
		apple.SetSublayerTranslation(v.content.Send(selLayer), float64(s.shift.X), float64(s.shift.Y))
		if s.overlay != 0 {
			s.overlay.Send(selSetFrame, apple.Rect(0, 0, w, h))
		}
	})
	v.updateOverlay()
	v.placeEdges()
	if v.zooms() {
		v.clampZoom()
	} else if _, hi := v.limits(); v.pos() > hi+0.5 {
		v.tickOn() // the frame pulls it back
	}
}

// tickOn runs the view on the frame clock until it has nothing to do.
func (v *view) tickOn() {
	if v.sc.ticking {
		return
	}
	v.sc.ticking = true
	v.surface.moving(v, true)
}

// moving records whether scroll view v runs on the frame clock, which
// holds the surface's batches for the start of its frames.
func (s *surface) moving(v *view, on bool) {
	switch {
	case on && !s.live[v]:
		if s.live == nil {
			s.live = map[*view]bool{}
		}
		s.live[v] = true
		s.startTicking()
	case !on && s.live[v]:
		delete(s.live, v)
		// What waited for the scroll's frames goes now.
		s.applyQueued()
	}
}

// tickScrolls steps the scroll views on the frame clock at t.
func (s *surface) tickScrolls(t time.Duration) {
	var done []*view
	for v := range s.live {
		if !v.tickScroll(t) {
			done = append(done, v)
		}
	}
	for _, v := range done {
		v.sc.ticking = false
		s.moving(v, false)
	}
}

// scrollsMoving reports whether a scroll view moves on its own or under
// the user's input, an indicator fading aside.
func (s *surface) scrollsMoving() bool {
	for v := range s.live {
		if k := v.sc.phase.kind; v.sc.resampled.Active() || k != atRest && k != onThumb {
			return true
		}
	}
	return false
}

// tickScroll steps one frame of the view at t, and reports whether
// frames are still needed.
func (v *view) tickScroll(t time.Duration) bool {
	s := v.sc
	// Events still come in these phases: hold at the newest until the next
	// one rather than end.
	p := s.phase
	input := p.kind == wheeling && p.gesture || p.kind == awaitingMomentum || p.kind == inMomentum
	if s.resampled.Active() {
		// A little more than an interval back: the event after it has come
		// by then, even a little late.
		pos, past := s.resampled.Sample(t - s.resampled.Interval - 2*time.Millisecond)
		if past && !input {
			s.resampled.Reset()
		}
		v.moveTo(pos)
	}
	_, hi := v.limits()
	switch p = s.phase; {
	case p.kind == easing:
		k := float64(t-p.start) / float64(p.duration)
		if v.zooms() {
			v.easeZoom(&p.zoom, scrollbar.EaseOut(min(k, 1)))
		} else {
			v.moveTo(p.from + (p.to-p.from)*driver.Points(scrollbar.EaseOut(k)))
		}
		if k >= 1 {
			s.phase = scrollPhase{}
			if p.momentum {
				v.emit(driver.EventMomentumEnd)
			}
			if p.settles {
				v.scrollEnded()
			}
		}
	case p.kind == wheeling && !p.gesture && t-p.last > scrollbar.WheelIdle:
		// A mouse wheel came to rest: a snapping view moves on to the next
		// snap point the wheel's way.
		s.phase = scrollPhase{}
		if s.config.Behavior.Snaps() {
			var velocity float32
			switch {
			case p.total >= 1:
				velocity = scrollbar.WheelSnap
			case p.total <= -1:
				velocity = -scrollbar.WheelSnap
			}
			v.release(0, velocity)
		} else {
			v.scrollEnded()
		}
	case p.kind == awaitingMomentum && t-p.last > momentumWait:
		s.phase = scrollPhase{}
		v.scrollEnded()
	case p.kind == atRest && v.zooms():
		v.clampZoom()
	case p.kind == atRest && v.pos() > hi+0.5:
		v.moveTo(hi) // the content shrank
	}
	v.reportScroll()
	k := s.phase.kind
	busy := s.resampled.Active() ||
		!(k == atRest || k == onThumb || k == inMomentum || k == wheeling && s.phase.gesture)
	overlays := false
	if k == onThumb {
		s.shown = t // a held thumb stays until let go
	} else if s.alpha > 0 {
		if a := scrollbar.IndicatorAlpha(t - s.shown); a != s.alpha {
			s.alpha, overlays = a, true
		}
		busy = busy || s.alpha > 0
	}
	if !s.config.Horizontal && s.refreshing && v.pos() < -v.lead() && s.config.Behavior.RefreshIndicator == 0 {
		s.spin = math.Mod(t.Seconds()*2*math.Pi, 2*math.Pi)
		overlays, busy = true, true
	}
	if overlays {
		v.updateOverlay()
	}
	return busy
}

// reportScroll reports the offset if it moved since it was last reported.
func (v *view) reportScroll() {
	s := v.sc
	if !s.report {
		return
	}
	s.report = false
	if v.events.Has(driver.EventScroll) {
		v.surface.engine.Dispatch(driver.Event{Kind: driver.EventScroll, ID: v.id, Point: v.reported(), Value: v.zoomValue(), Count: s.applied})
	}
}

// emit reports kind on the scroll view at its offset.
func (v *view) emit(kind driver.EventKind) {
	v.report(driver.Event{Kind: kind, Point: v.reported(), Value: v.zoomValue(), Count: v.sc.applied})
}

// scrollEnded follows the view coming to rest: the values that snap as a
// scroll ends settle.
func (v *view) scrollEnded() {
	s := v.surface
	s.animator.Settle(v.id)
	if s.animator.Running() {
		s.startTicking()
	}
}

// interrupt ends the momentum or snap a new input stops.
func (v *view) interrupt() {
	v.showGoal()
	p := v.sc.phase
	v.sc.phase = scrollPhase{}
	if p.kind == inMomentum || p.kind == easing && p.momentum {
		v.emit(driver.EventMomentumEnd)
	}
}

// showGoal shows where trackpad events put the view at once, as something
// else moves it from there.
func (v *view) showGoal() {
	if goal, ok := v.sc.resampled.Goal(); ok {
		v.sc.resampled.Reset()
		v.moveTo(goal)
	}
}

// goal is where trackpad events put the view, ahead of what shows.
func (v *view) goal() driver.Points {
	if goal, ok := v.sc.resampled.Goal(); ok {
		return goal
	}
	return v.pos()
}

// feed records that a trackpad event at at put the offset at pos, shown
// from the next frame on.
func (v *view) feed(pos driver.Points, at time.Duration) {
	if apple.Msg(v.native, selWindow) == 0 {
		v.moveTo(pos)
		return
	}
	v.sc.resampled.Push(at, pos, v.pos())
	v.tickOn()
}

// animateTo eases to to, clamped to the limits, in d; momentum reports
// the move as momentum, settles lets snapping values settle at its end.
func (v *view) animateTo(to driver.Points, d time.Duration, momentum, settles bool) {
	v.showGoal()
	lo, hi := v.limits()
	to = min(max(to, lo), hi)
	from := v.pos()
	if math.Abs(float64(to-from)) < 0.5 || d <= 0 {
		v.sc.phase = scrollPhase{}
		v.moveTo(to)
		if momentum {
			v.emit(driver.EventMomentumEnd)
		}
		if settles {
			v.scrollEnded()
		}
		return
	}
	v.sc.phase = scrollPhase{kind: easing, from: from, to: to, start: mediaTime(), duration: d, momentum: momentum, settles: settles}
	v.tickOn()
}

// release follows the user letting go at velocity (points per
// millisecond along the axis): the view springs back, snaps or rests. A
// snap goes snapVelocity's way when it is not 0 (a mouse wheel that came
// to rest).
func (v *view) release(velocity, snapVelocity float32) {
	v.showGoal()
	s := v.sc
	b := &s.config.Behavior
	pos := v.pos()
	lo, hi := v.limits()
	if snapVelocity == 0 {
		snapVelocity = velocity
	}
	length := v.main(driver.Point{X: s.content.Width, Y: s.content.Height})
	target, snaps := b.SnapTarget(pos, driver.Projected(pos, velocity, b.Deceleration()), snapVelocity,
		v.viewport(), length+v.trail())
	switch {
	case pos < lo-0.5 || pos > hi+0.5:
		v.animateTo(pos, scrollbar.Animation, false, true)
	case snaps:
		d := scrollbar.SnapDuration(target-pos, velocity)
		if d > 0 {
			v.emit(driver.EventMomentumStart)
		}
		v.animateTo(target, d, d > 0, true)
	default:
		s.phase = scrollPhase{}
		v.scrollEnded()
	}
}

// configureScroll applies a scroll view's configuration.
func (v *view) configureScroll(c *driver.ScrollConfig) {
	s := v.sc
	old := s.config.Behavior.ContentInset
	indicator := s.config.Behavior.RefreshIndicator
	zoomed, scale := s.config.Zooms(), s.config.ZoomScale
	s.config = *c
	if !v.indicates() {
		s.alpha = 0
	}
	if c.Behavior.NoScroll && (s.phase.kind == onThumb || s.phase.kind == wheeling) {
		s.phase = scrollPhase{}
	}
	in := c.Behavior.ContentInset
	if in != old {
		// A view resting at its start stays there.
		s.at = scrollbar.KeepStart(old, in, s.at)
	}
	switch {
	case c.Zooms() && !zoomed:
		s.zoom = float64(c.ClampZoom(c.ZoomScale))
	case c.Zooms() && s.phase.kind != pinching:
		s.zoom = v.limitZoom(s.zoom, false)
	case c.Horizontal:
		// It shows the cross axis's inset.
		s.zoom, s.at.Y = 1, -in.Top
	default:
		s.zoom, s.at.X = 1, -in.Left
	}
	if c.Zooms() && s.phase.kind == atRest {
		s.at = v.clampAt(s.at, s.zoom)
	}
	s.report = s.report || in != old || c.Zooms() != zoomed
	apple.WithoutAnimation(v.applyOffset)
	v.setRefreshing(c.Refreshing && !c.Horizontal)
	// A refresh in progress keeps the room of the indicator now showing.
	if c.Behavior.RefreshIndicator != indicator && s.refreshing && !c.Horizontal && !(s.phase.kind == wheeling && s.phase.gesture) &&
		v.pos() < -v.lead() {
		v.animateTo(-v.lead()-c.Behavior.RefreshInset(), scrollbar.Animation, false, false)
	}
	v.updateOverlay()
	v.placeEdges()
	if s.phase.kind == atRest && !s.ticking {
		v.commit() // nothing moves: AppKit may as well know
	}
	v.reportScroll()
	if c.Zooms() && zoomed && c.ZoomScale != scale {
		v.zoomTo(&driver.ZoomRequest{Scale: c.ZoomScale}, true)
	}
}

// setContent follows the content's new size: content that shrank below
// the offset pulls it back on the next frame, once the rest of the batch
// (the ItemScrollBy keeping the visible content in place) is applied.
func (v *view) setContent(size driver.Size) {
	v.sc.content = size
	if v.zooms() {
		v.clampZoom()
	} else if _, hi := v.limits(); v.pos() > hi+0.5 {
		v.tickOn()
	}
	v.updateOverlay()
}

// setRefreshing shows the spinner above the content, scrolling to it from
// the top, or hides it.
func (v *view) setRefreshing(on bool) {
	s := v.sc
	if s.refreshing == on {
		return
	}
	s.refreshing = on
	v.updateOverlay()
	if s.phase.kind == wheeling && s.phase.gesture {
		return // the fingers hold it
	}
	switch start := -v.lead(); {
	case on && v.pos() <= start+0.5:
		v.animateTo(start-s.config.Behavior.RefreshInset(), scrollbar.Animation, false, false)
	case !on && v.pos() < start:
		v.animateTo(start, scrollbar.Animation, false, false)
	}
}

// pulledToRefresh starts a refresh when a release leaves the view pulled
// far enough past its top, as UIRefreshControl does: the spinner shows
// right away, and the next config says whether the app keeps it.
func (v *view) pulledToRefresh() {
	s := v.sc
	if c := &s.config; !c.Refreshable || c.Horizontal || s.refreshing || v.pos() > -v.lead()-c.Behavior.RefreshThreshold() {
		return
	}
	s.refreshing = true
	v.report(driver.Event{Kind: driver.EventRefresh, Point: v.reported(), Count: s.applied})
	v.updateOverlay()
}

// scrollTo follows ItemScrollTo: the offset goes to p along the axis,
// clamped to what the view shows.
func (v *view) scrollTo(p driver.Point, animated bool) {
	s := v.sc
	lo, hi := v.limits()
	to := min(max(v.main(p), lo), hi)
	s.applied++
	s.report = true
	v.interrupt()
	if v.zooms() {
		v.panTo(v.clampAt(p, s.zoom), animated)
	} else if animated {
		v.animateTo(to, scrollbar.Animation, false, true)
	} else {
		v.moveTo(to)
	}
	v.reportScroll()
}

// scrollBy follows ItemScrollBy: the core keeps the visible content in
// place as rows above it change size. It is unclamped, as the core knows
// its content, and carries a gesture or animation in flight along.
func (v *view) scrollBy(d driver.Point) {
	s := v.sc
	s.applied++
	s.report = true
	// The cross axis would only push the content out of view.
	by := v.main(d)
	switch p := &s.phase; p.kind {
	case onThumb:
		p.from += by
	case wheeling:
		p.raw += by
	case easing:
		p.from += by
		p.to += by
	}
	s.resampled.Shift(by)
	if v.zooms() {
		s.phase.rawAt.X, s.phase.rawAt.Y = s.phase.rawAt.X+d.X, s.phase.rawAt.Y+d.Y
		v.moveZoom(driver.Point{X: s.at.X + d.X, Y: s.at.Y + d.Y}, s.zoom)
	} else if by != 0 {
		v.moveTo(v.pos() + by)
	}
	v.reportScroll()
}

// reveal scrolls the scroll views around v, animated, so that it shows, as
// phones show the field that gets the keyboard focus.
func (v *view) reveal() {
	s := v.surface
	// The rectangles are AppKit's, which the bounds place.
	s.commitPending()
	// Content the views inside moved, which rectangles measured now do not
	// show yet.
	var moved driver.Point
	for n := v.up; n != nil; n = n.up {
		if !s.alive(n) || apple.Msg(n.native, selWindow) == 0 {
			continue
		}
		r := objc.Send[apple.CGRect](v.native, sel("convertRect:toView:"), objc.Send[apple.CGRect](v.native, selBounds), n.native)
		start := driver.Points(r.Origin.Y) - moved.Y
		length := driver.Points(r.Size.Height)
		if n.sc.config.Horizontal {
			start, length = driver.Points(r.Origin.X)-moved.X, driver.Points(r.Size.Width)
		}
		// A running scroll counts from where it goes.
		base := n.pos()
		if n.sc.phase.kind == easing {
			base = n.sc.phase.to
		}
		start -= base - n.pos()
		lo, hi := n.limits()
		to := min(max(base+scrollbar.RevealDelta(start, start+length, n.viewport()), lo), hi)
		if abs(to-base) < 0.5 {
			continue
		}
		if n.sc.config.Horizontal {
			moved.X += to - base
		} else {
			moved.Y += to - base
		}
		n.interrupt()
		n.animateTo(to, scrollbar.Animation, false, true)
	}
}

// canScroll reports whether the user can move the view by wheel deltas
// (dx, dy), positive as the offset grows: it scrolls and is not at that
// edge yet.
func (v *view) canScroll(dx, dy driver.Points) bool {
	if v.sc.config.Behavior.NoScroll {
		return false
	}
	if v.zooms() {
		return v.zoomCanScroll(dx, dy)
	}
	lo, hi := v.limits()
	if hi <= lo {
		return false
	}
	d, pos := v.wheelDelta(dx, dy), v.pos()
	return d > 0 && pos < hi-0.5 || d < 0 && pos > lo+0.5
}

// canBounce reports whether a trackpad gesture by (dx, dy) may stretch
// the view past its edge: it scrolls, bounces and lies along the gesture.
func (v *view) canBounce(dx, dy driver.Points) bool {
	b := &v.sc.config.Behavior
	return !b.NoScroll && !b.NoBounces && v.along(abs(dx) > abs(dy))
}

// along reports whether the view scrolls along the horizontal axis or
// the vertical one: a zooming view moves along both.
func (v *view) along(horizontal bool) bool {
	return v.sc.config.Horizontal == horizontal || v.zooms()
}

// wheelDelta is the share of wheel deltas (dx, dy) along the axis.
// Vertical wheels scroll horizontal views too.
func (v *view) wheelDelta(dx, dy driver.Points) driver.Points {
	if v.sc.config.Horizontal && dx != 0 {
		return dx
	}
	return dy
}

// pinSticky moves the scroll view's sticky cells for its offset, as
// NSH's StickyCells: each by clamp(offset - start, 0, end - start) from
// its frame.
func (v *view) pinSticky() {
	if len(v.sc.sticky) == 0 {
		return
	}
	at := v.pos()
	for _, c := range v.sc.sticky {
		if int(c.Cell) < len(v.surface.views) {
			if cv := v.surface.views[c.Cell]; cv != nil {
				cv.stick(v.surface.animator.StickyShift(c, at), v.sc.config.Horizontal)
			}
		}
	}
}

// setSticky replaces the scroll view's sticky cells; cells no longer
// sticky go back to their frames.
func (v *view) setSticky(cells []driver.StickyCell) {
	for _, c := range v.sc.sticky {
		if int(c.Cell) < len(v.surface.views) {
			if cv := v.surface.views[c.Cell]; cv != nil {
				cv.stick(0, false)
			}
		}
	}
	v.sc.sticky = cells
	bound := false
	for _, c := range cells {
		bound = bound || c.Node != 0
	}
	if bound {
		if v.surface.stickyBound == nil {
			v.surface.stickyBound = map[*view]bool{}
		}
		v.surface.stickyBound[v] = true
	} else {
		delete(v.surface.stickyBound, v)
	}
	v.pinSticky()
}

// repinBound pins again the sticky cells whose insets follow animated
// values, which may have moved.
func (s *surface) repinBound() {
	for v := range s.stickyBound {
		if v.sc != nil && s.views[v.id] == v {
			v.pinSticky()
		} else {
			delete(s.stickyBound, v)
		}
	}
}

// stick moves a sticky cell d along its list from its frame.
func (v *view) stick(d driver.Points, horizontal bool) {
	shift := driver.Point{Y: d}
	if horizontal {
		shift = driver.Point{X: d}
	}
	if shift != v.shift {
		v.shift = shift
		if v.framed {
			v.place(v.frame)
		}
	}
}

// The trackpad and the mouse wheel.

// wheelBegin starts a trackpad gesture on the view; fingers resting on
// the trackpad begin one too, which stops what moves.
func (v *view) wheelBegin() {
	if v.sc.config.Behavior.NoScroll {
		return
	}
	v.interrupt()
	v.sc.phase = scrollPhase{kind: wheeling, last: mediaTime(), gesture: true, raw: v.pos(), rawAt: v.sc.at}
	v.emit(driver.EventScrollDragStart)
}

// wheelBy scrolls by wheel deltas (dx, dy) of an event at at. Outside a
// gesture it is a mouse wheel's, which settles by itself once it stops.
func (v *view) wheelBy(dx, dy driver.Points, at time.Duration) {
	s := v.sc
	if s.config.Behavior.NoScroll {
		return
	}
	if s.phase.kind != wheeling {
		v.interrupt()
		s.phase = scrollPhase{kind: wheeling, raw: v.pos(), rawAt: s.at}
	}
	p := &s.phase
	if v.zooms() {
		v.zoomWheelBy(dx, dy)
		return
	}
	lo, hi := v.limits()
	d, pos := v.wheelDelta(dx, dy), v.pos()
	var to driver.Points
	if p.gesture {
		// Trackpads stretch past the edges, as macOS's do.
		p.raw += d
		to = scrollbar.Dragged(p.raw, lo, hi, v.viewport(), !s.config.Behavior.NoBounces)
	} else {
		// Past an edge (the spinner's room), wheels only move back.
		to = min(max(pos+d, min(lo, pos)), max(hi, pos))
		p.raw = to
	}
	p.total += d
	p.last = mediaTime()
	if p.gesture {
		v.feed(to, at)
	} else {
		v.moveTo(to)
	}
	v.tickOn()
}

// wheelEnd ends a trackpad gesture let go at (vx, vy) points per
// millisecond: the view springs back, snaps, or waits for the system's
// momentum.
func (v *view) wheelEnd(vx, vy float32) {
	s := v.sc
	p := s.phase
	if p.kind != wheeling {
		return
	}
	velocity := float32(v.wheelDelta(driver.Points(vx), driver.Points(vy)))
	s.phase = scrollPhase{}
	if p.gesture {
		v.emit(driver.EventScrollDragEnd)
		v.pulledToRefresh()
	}
	if v.zooms() {
		v.zoomWheelEnd(p.gesture)
		return
	}
	// After the refresh: refreshing lowers the top limit.
	pos := v.goal()
	lo, hi := v.limits()
	outside := pos < lo-0.5 || pos > hi+0.5
	if p.gesture && !outside && !s.config.Behavior.Snaps() {
		s.phase = scrollPhase{kind: awaitingMomentum, last: mediaTime()}
		v.tickOn()
		return
	}
	var snap float32
	if velocity == 0 && p.total >= 1 {
		snap = scrollbar.WheelSnap
	} else if velocity == 0 && p.total <= -1 {
		snap = -scrollbar.WheelSnap
	}
	v.release(velocity, snap)
}

// momentumBegin starts the system's momentum after a gesture, and reports
// whether the view takes it: not when it springs back or snaps instead.
func (v *view) momentumBegin() bool {
	if v.sc.phase.kind != awaitingMomentum {
		return false
	}
	v.sc.phase = scrollPhase{kind: inMomentum}
	v.emit(driver.EventMomentumStart)
	return true
}

// momentumBy moves the view by the system's momentum, which stops at the
// edges.
func (v *view) momentumBy(dx, dy driver.Points, at time.Duration) {
	if v.sc.phase.kind != inMomentum {
		return
	}
	if v.zooms() {
		v.moveZoom(v.zoomWheelTo(dx, dy), v.sc.zoom)
		return
	}
	lo, hi := v.limits()
	pos := v.goal()
	v.feed(min(max(pos+v.wheelDelta(dx, dy), min(lo, pos)), max(hi, pos)), at)
}

// momentumEnd ends the system's momentum.
func (v *view) momentumEnd() {
	if v.sc.phase.kind != inMomentum {
		return
	}
	v.sc.phase = scrollPhase{}
	v.emit(driver.EventMomentumEnd)
	v.scrollEnded()
}

// catch stops a snap after a release where it is, as a finger catches a
// glide on iOS, and reports whether it caught one.
func (v *view) catch() bool {
	if p := v.sc.phase; p.kind != easing || !p.momentum {
		return false
	}
	v.interrupt()
	v.scrollEnded()
	return true
}

// The indicator as a scroll bar.

// thumbRect returns the indicator's frame in the view and how far it
// travels along its track, or false when the content fits.
func (v *view) thumbRect() (driver.Bounds, driver.Points, bool) {
	return v.bar().Indicator()
}

// bar returns the view's scroll bar, as the shared geometry lays it out.
func (v *view) bar() scrollbar.Bar {
	s := v.sc
	z := driver.Points(s.zoom)
	return scrollbar.Bar{Horizontal: s.config.Horizontal, Frame: v.frame.Size,
		Content: driver.Size{Width: s.content.Width * z, Height: s.content.Height * z}, Offset: s.at, Inset: s.config.Behavior.ContentInset}
}

// grabsThumb reports whether a mouse press at p, in the view, grabs the
// indicator, faded out or not: on the desktop it is a scroll bar.
func (v *view) grabsThumb(p driver.Point) bool {
	if !v.indicates() || v.sc.config.Behavior.NoScroll {
		return false
	}
	r, _, ok := v.thumbRect()
	if !ok {
		return false
	}
	along, across, start, length, edge := p.Y, p.X, r.Origin.Y, r.Size.Height, v.frame.Size.Width
	if v.sc.config.Horizontal {
		along, across, start, length, edge = p.X, p.Y, r.Origin.X, r.Size.Width, v.frame.Size.Height
	}
	return across >= edge-thumbGrab && across <= edge && along >= start-thumbSlop && along <= start+length+thumbSlop
}

// thumbBegin starts the mouse dragging the indicator.
func (v *view) thumbBegin() {
	v.interrupt()
	s := v.sc
	s.phase = scrollPhase{kind: onThumb, from: v.pos()}
	s.shown, s.alpha, s.dark = mediaTime(), 1, v.surface.env.Dark
	v.updateOverlay()
	v.tickOn()
	v.emit(driver.EventScrollDragStart)
}

// thumbMove follows the indicator dragged by moved since it was grabbed:
// the content moves the other way, so the thumb follows the pointer.
func (v *view) thumbMove(moved driver.Point) {
	s := v.sc
	if s.phase.kind != onThumb {
		return
	}
	_, travel, ok := v.thumbRect()
	if !ok || travel <= 0 {
		return
	}
	start, end := v.restLimits()
	lo, hi := v.limits()
	v.moveTo(min(max(s.phase.from+v.main(moved)*(end-start)/travel, lo), hi))
}

// thumbEnd lets the indicator go: it fades from now, and a snapping view
// settles on a snap point.
func (v *view) thumbEnd() {
	s := v.sc
	if s.phase.kind != onThumb {
		return
	}
	s.phase = scrollPhase{}
	s.shown = mediaTime()
	v.tickOn()
	v.emit(driver.EventScrollDragEnd)
	v.release(0, 0)
}

// whenScrolling caches whether the user shows scroll bars only while
// scrolling, read again at most once a second.
var whenScrolling struct {
	on   bool
	read time.Time
}

// hovered shows the indicator while the pointer is over the view, unless
// the user shows it only while scrolling.
func (v *view) hovered() {
	if time.Since(whenScrolling.read) > time.Second {
		defaults := objc.ID(objc.GetClass("NSUserDefaults")).Send(sel("standardUserDefaults"))
		whenScrolling.on = apple.GoString(defaults.Send(sel("stringForKey:"), apple.NSString("AppleShowScrollBars"))) == "WhenScrolling"
		whenScrolling.read = time.Now()
	}
	s := v.sc
	if whenScrolling.on || !v.indicates() || s.config.Behavior.NoScroll {
		return
	}
	if _, _, ok := v.thumbRect(); !ok {
		return
	}
	s.shown = mediaTime()
	if s.alpha == 1 {
		return
	}
	s.alpha, s.dark = 1, v.surface.env.Dark
	v.updateOverlay()
	v.tickOn()
}

// The overlay: the indicator and the refresh spinner.

// refreshSpokes is how many spokes the refresh spinner has.
const refreshSpokes = 8

// grays are the CGColors the overlay draws with, made once.
var grays = map[driver.Color]uintptr{}

// gray returns the CGColor of level (0 black, 1 white) at alpha.
func gray(level, alpha float64) uintptr {
	l, a := uint32(level*255+0.5), uint32(alpha*255+0.5)
	c := driver.Color(l<<24 | l<<16 | l<<8 | a)
	if ref, ok := grays[c]; ok {
		return ref
	}
	ref := apple.CGColor(c)
	grays[c] = ref
	return ref
}

// overlayLayer adds a layer to the view's overlay, a view above the
// container that lets the mouse through, made on first use.
func (v *view) overlayLayer() objc.ID {
	s := v.sc
	if s.overlay == 0 {
		s.overlay = objc.ID(classOverlay).Send(selAlloc).Send(sel("initWithFrame:"),
			apple.Rect(0, 0, float64(v.frame.Size.Width), float64(v.frame.Size.Height)))
		s.overlay.Send(selSetWantsLayer, true)
		v.native.Send(selAddSubview, s.overlay)
		s.overlay.Send(selRelease)
	}
	layer := objc.ID(objc.GetClass("CALayer")).Send(selNew)
	layer.Send(sel("setAnchorPoint:"), apple.CGPoint{})
	s.overlay.Send(selLayer).Send(sel("addSublayer:"), layer)
	layer.Send(selRelease)
	return layer
}

// updateOverlay shows the indicator and the refresh spinner at the
// offset.
func (v *view) updateOverlay() {
	s := v.sc
	look := indicatorLook{hidden: true}
	if s.alpha > 0 && v.indicates() {
		if frame, _, ok := v.thumbRect(); ok {
			look = indicatorLook{frame: frame, alpha: s.alpha, dark: s.dark}
		}
	}
	apple.WithoutAnimation(func() {
		if look != s.drawn && (s.thumb != 0 || !look.hidden) {
			// Each frame of a scroll sends only what changed.
			old := s.drawn
			if s.thumb == 0 {
				s.thumb = v.overlayLayer()
				s.thumb.Send(sel("setCornerRadius:"), float64(scrollbar.Thickness)/2)
				old = indicatorLook{alpha: -1, dark: !look.dark, hidden: true}
			}
			f := look.frame
			if !look.hidden && f.Size != old.frame.Size {
				s.thumb.Send(sel("setBounds:"), apple.Rect(0, 0, float64(f.Size.Width), float64(f.Size.Height)))
			}
			if !look.hidden && f.Origin != old.frame.Origin {
				s.thumb.Send(sel("setPosition:"), apple.CGPoint{X: float64(f.Origin.X), Y: float64(f.Origin.Y)})
			}
			if !look.hidden && look.dark != old.dark {
				color := gray(0, 0.45)
				if look.dark {
					color = gray(1, 0.55)
				}
				s.thumb.Send(sel("setBackgroundColor:"), color)
			}
			if !look.hidden && look.alpha != old.alpha {
				s.thumb.Send(sel("setOpacity:"), look.alpha)
			}
			if look.hidden != old.hidden {
				s.thumb.Send(selSetHidden, look.hidden)
			}
			if look.hidden {
				look.frame, look.alpha, look.dark = old.frame, old.alpha, old.dark
			}
			s.drawn = look
		}
		v.updateSpinner()
	})
}

// updateSpinner shows the refresh spinner as an activity indicator's
// eight spokes: while pulling, as many as the pull is far, and turning in
// whole spokes while it refreshes. The app's own indicator replaces it.
func (v *view) updateSpinner() {
	s := v.sc
	pos, start := v.pos(), -v.lead()
	b := &s.config.Behavior
	if s.config.Horizontal || pos >= start || !s.refreshing && !s.config.Refreshable || b.RefreshIndicator > 0 {
		if s.spinner != 0 {
			s.spinner.Send(selSetHidden, true)
		}
		return
	}
	if s.spinner == 0 {
		s.spinner = v.overlayLayer()
		for i := range refreshSpokes {
			spoke := objc.ID(objc.GetClass("CALayer")).Send(selNew)
			spoke.Send(sel("setBounds:"), apple.Rect(0, 0, 2.5, 5.5))
			spoke.Send(sel("setCornerRadius:"), 1.25)
			// Each 7.25 points from the center, pointing outwards.
			angle := float64(i) * 2 * math.Pi / refreshSpokes
			spoke.Send(sel("setPosition:"), apple.CGPoint{X: math.Sin(angle) * 7.25, Y: -math.Cos(angle) * 7.25})
			apple.RotateLayer(spoke, angle)
			s.spinner.Send(sel("addSublayer:"), spoke)
			spoke.Send(selRelease)
			s.spokes[i] = spoke
		}
	}
	progress := 1.0
	if !s.refreshing {
		progress = min(float64(start-pos)/float64(b.RefreshThreshold()), 1)
	}
	s.spinner.Send(selSetHidden, false)
	s.spinner.Send(sel("setPosition:"), apple.CGPoint{X: float64(v.frame.Size.Width) / 2, Y: float64(-pos - b.RefreshInset()/2)})
	shown := min(int(math.Ceil(progress*refreshSpokes)), refreshSpokes)
	step := 2 * math.Pi / refreshSpokes
	lead := int(s.spin/step) % refreshSpokes
	level := 0.25
	if s.dark {
		level = 1
	}
	for i, spoke := range s.spokes {
		fade := 0.8
		if progress >= 1 {
			fade = 1 - float64((lead+refreshSpokes-i)%refreshSpokes)/refreshSpokes*0.85
		}
		spoke.Send(sel("setBackgroundColor:"), gray(level, fade))
		spoke.Send(selSetHidden, i >= shown)
	}
}

func abs(v driver.Points) driver.Points {
	if v < 0 {
		return -v
	}
	return v
}

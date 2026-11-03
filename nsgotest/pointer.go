package nsgotest

import (
	"math"
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
)

const (
	// dragStep is how far a finger or a scroll moves in one display frame.
	dragStep = 10
	// landing is how long a finger rests where it lands before it moves:
	// the views under it hear it land first.
	landing = 100 * time.Millisecond
	// pressSlop is how far a finger may move and still press.
	pressSlop = 8
)

// dragMode is what a drag turned into.
type dragMode uint8

const (
	dragIdle   dragMode = iota // nothing started yet
	dragFree                   // nothing native will: the responder took it, or nothing could
	dragPan                    // a view's pan
	dragScroll                 // a scroll view
)

// finger is the finger of a Drag.
type finger struct {
	start, at, last driver.Point
	elapsed         time.Duration
	chain           []*View // the view it landed on and those around it
	reporting       bool    // the views listening to touches hear it
	press           *View   // the pressable view under it
	pressed         bool    // press heard PressDown
	cancelled       bool    // it moved away from press
	mode            dragMode
	view            *View        // the pan's or scroll view's
	from            driver.Point // the scroll view's offset when it took the drag
	grabbed         driver.Point // where the finger was then
	moved           bool
}

// Drag puts a finger on the middle of what selector names, rests it there
// for 100 ms, moves it through each point of path in turn (offsets from
// where it landed) 10 points a display frame, and lifts it, as the
// platforms decide what takes it: the views listening to touches hear
// them, past the slop a pan or the scroll view the drag runs along takes
// it unless the touch responder holds it, and the press under the finger
// is cancelled once it moves away. The clock moves with the frames.
func (a *App) Drag(selector string, path ...driver.Point) {
	a.t.Helper()
	a.step("drag(%s, %v)", selector, path)
	v := a.pick(selector, "view", func(m *View) (*View, bool) { return m, true })
	_, start := v.center()
	if a.target != nil {
		start = a.target.reveal(v)
	}
	points := make([]driver.Point, len(path))
	for i, d := range path {
		points[i] = driver.Point{X: start.X + d.X, Y: start.Y + d.Y}
	}
	if a.target != nil {
		a.target.drag(start, points)
		a.done()
		return
	}
	f := &finger{start: start, at: start, last: start, reporting: a.listensToTouches()}
	for p := v; p != nil; p = p.Parent {
		f.chain = append(f.chain, p)
	}
	// A finger going down ends what a responder held before.
	a.blocked = false
	inScroll := false
	for _, p := range f.chain {
		if f.press == nil && (pressable(p) || p.Events.Has(driver.EventLongPress)) && !v.disabled(p) {
			f.press = p
		}
		inScroll = inScroll || p != v && p.Kind == driver.KindScroll && !p.Scroll.Behavior.NoScroll
	}
	a.touchAt(driver.GestureBegan, start, 0)
	// Inside a scroll view a touch presses only once it rests.
	if !inScroll {
		a.pressIn(f)
	}
	for f.elapsed+driver.FrameInterval <= landing {
		a.frame(f)
	}
	a.pressIn(f)
	prev := start
	for _, to := range points {
		n := int(math.Ceil(math.Max(math.Abs(float64(to.X-prev.X)), math.Abs(float64(to.Y-prev.Y))) / dragStep))
		for i := 1; i <= max(n, 1); i++ {
			a.frame(f)
			t := driver.Points(i) / driver.Points(max(n, 1))
			a.fingerMoved(f, driver.Point{X: prev.X + (to.X-prev.X)*t, Y: prev.Y + (to.Y-prev.Y)*t})
		}
		prev = to
	}
	a.fingerLifted(f)
	a.done()
}

// frame takes one display frame of a gesture.
func (a *App) frame(f *finger) {
	a.elapse(driver.FrameInterval)
	f.elapsed += driver.FrameInterval
}

func (a *App) pressIn(f *finger) {
	if f.press != nil && !f.pressed && !f.cancelled {
		f.pressed = true
		ev := a.pressEvent(f.press, f.start)
		ev.Kind = driver.EventPressDown
		a.send(f.press, ev)
	}
}

func (a *App) fingerMoved(f *finger, at driver.Point) {
	f.last, f.at = f.at, at
	first := !f.moved
	f.moved = true
	t := driver.Point{X: at.X - f.start.X, Y: at.Y - f.start.Y}
	if f.reporting {
		a.touchAt(driver.GestureChanged, at, f.elapsed)
	}
	distance := driver.Points(math.Hypot(float64(t.X), float64(t.Y)))
	if f.pressed && !f.cancelled && distance > pressSlop {
		f.cancelled = true
		ev := a.pressEvent(f.press, f.start)
		ev.Kind = driver.EventPressUp
		a.send(f.press, ev)
	}
	// The touch responder hears the first move first: it may take the
	// drag before anything native starts.
	if f.mode == dragIdle && !(f.reporting && first) {
		a.startDrag(f, t)
	}
	switch f.mode {
	case dragPan:
		a.pan(f, driver.GestureChanged, t)
	case dragScroll:
		a.dragScroll(f)
	}
}

// startDrag decides what takes a drag translated by t, as platforms do
// once it passes the slop: the innermost pan it activates, else the
// innermost scroll view it runs along that can move that way.
func (a *App) startDrag(f *finger, t driver.Point) {
	if a.blocked {
		f.mode = dragFree
		return
	}
	vertical := math.Abs(float64(t.Y)) >= math.Abs(float64(t.X))
	for _, v := range f.chain {
		if v.Events.Has(driver.EventPan) {
			switch v.Props.PanActivation.Decide(t) {
			case 1:
				f.mode, f.view = dragPan, v
				a.pan(f, driver.GestureBegan, t)
				return
			case 0:
				// An undecided pan holds the drag back.
				return
			}
		}
		if v.Kind == driver.KindScroll && !v.Scroll.Behavior.NoScroll && v.Scroll.Horizontal != vertical &&
			driver.Points(math.Hypot(float64(t.X), float64(t.Y))) >= driver.DefaultPanSlop && a.canScroll(v, t) {
			if f.reporting {
				// The scroll view takes the touches over.
				a.touchAt(driver.GestureCancelled, f.at, f.elapsed)
				f.reporting = false
			}
			f.mode, f.view, f.from, f.grabbed = dragScroll, v, v.Offset, f.at
			a.scrollEvent(v, driver.EventScrollDragStart)
			return
		}
	}
	if driver.Points(math.Hypot(float64(t.X), float64(t.Y))) >= driver.DefaultPanSlop {
		f.mode = dragFree
	}
}

// canScroll reports whether scroll view v moves for a finger translated
// by t: its content goes that way, or a refreshable view is pulled down
// at its top.
func (a *App) canScroll(v *View, t driver.Point) bool {
	at, end, d := v.Offset.Y, v.scrollEnd(), -t.Y
	if v.Scroll.Horizontal {
		at, d = v.Offset.X, -t.X
	}
	if d < 0 {
		return at > 0 || !v.Scroll.Horizontal && v.Scroll.Refreshable && !v.Scroll.Refreshing
	}
	return at < end
}

// scrollEnd is the largest offset of scroll view v along its axis.
func (v *View) scrollEnd() driver.Points {
	in := v.Scroll.Behavior.ContentInset
	if v.Scroll.Horizontal {
		return max(v.Content.Width-v.Frame.Size.Width, 0) + in.Right
	}
	return max(v.Content.Height-v.Frame.Size.Height, 0) + in.Bottom
}

func (a *App) pan(f *finger, phase driver.GesturePhase, t driver.Point) {
	ev := driver.Event{Kind: driver.EventPan, ID: f.view.ID, Phase: phase, Point: t, Velocity: f.velocity()}
	// A platform moves bound values before it reports the move.
	a.anim.Gesture(&ev)
	a.dispatch(ev)
}

// velocity is the finger's, in points per second, over its last frame.
func (f *finger) velocity() driver.Point {
	s := driver.Points(driver.FrameInterval.Seconds())
	return driver.Point{X: (f.at.X - f.last.X) / s, Y: (f.at.Y - f.last.Y) / s}
}

// dragScroll moves the scroll view with the finger, within its content,
// or past its top when it refreshes.
func (a *App) dragScroll(f *finger) {
	v := f.view
	off := v.Offset
	if v.Scroll.Horizontal {
		off.X = min(max(f.from.X-(f.at.X-f.grabbed.X), -v.Scroll.Behavior.ContentInset.Left), v.scrollEnd())
	} else {
		lo := -v.Scroll.Behavior.ContentInset.Top
		if v.Scroll.Refreshable {
			lo = -math.MaxFloat32
		}
		off.Y = min(max(f.from.Y-(f.at.Y-f.grabbed.Y), lo), v.scrollEnd())
	}
	a.scrollView(v, off)
}

// scrollView moves v to off as a user's scroll and reports it.
func (a *App) scrollView(v *View, off driver.Point) {
	v.Offset = off
	a.anim.Scroll(v.ID, off)
	a.pullValue(v)
	a.dispatch(driver.Event{Kind: driver.EventScroll, ID: v.ID, Point: off, Value: v.ZoomScale, Count: v.Applied})
}

// pulled is how far vertical scroll view v is pulled past its top.
func (v *View) pulled() driver.Points {
	if v.Scroll.Horizontal {
		return 0
	}
	return max(-v.Offset.Y-v.Scroll.Behavior.ContentInset.Top, 0)
}

// pullValue sets the value bound to v's pull, as a platform does: how far
// it is pulled, and at least the room it keeps while refreshing.
func (a *App) pullValue(v *View) {
	if v.Scroll.Horizontal || !a.anim.Pulls(v.ID) {
		return
	}
	pull := v.pulled()
	if v.Scroll.Refreshing {
		pull = max(pull, v.Scroll.Behavior.RefreshInset())
	}
	a.anim.Pull(v.ID, pull)
}

func (a *App) fingerLifted(f *finger) {
	if f.reporting {
		a.touchAt(driver.GestureEnded, f.at, f.elapsed)
	}
	switch f.mode {
	case dragPan:
		a.pan(f, driver.GestureEnded, driver.Point{X: f.at.X - f.start.X, Y: f.at.Y - f.start.Y})
	case dragScroll:
		v := f.view
		a.scrollEvent(v, driver.EventScrollDragEnd)
		if !v.Scroll.Horizontal && v.Offset.Y < -v.Scroll.Behavior.ContentInset.Top {
			if v.pulled() >= v.Scroll.Behavior.RefreshThreshold() && v.Scroll.Refreshable && !v.Scroll.Refreshing {
				a.dispatch(driver.Event{Kind: driver.EventRefresh, ID: v.ID, Point: v.Offset, Count: v.Applied})
			}
			// It springs back to its top.
			a.scrollEvent(v, driver.EventMomentumStart)
			a.scrollView(v, v.clamp(v.Offset))
			a.scrollEvent(v, driver.EventMomentumEnd)
		}
		a.settle(v)
	}
	if f.pressed && !f.cancelled {
		// A finger that lifts where it landed taps, and taps count up as
		// presses do.
		a.countClick(f.press)
		ev := a.pressEvent(f.press, f.start)
		ev.Kind, ev.Count = driver.EventPressUp, a.clicks
		a.send(f.press, ev)
		ev.Kind = driver.EventPress
		a.send(f.press, ev)
	}
}

// Hover moves the mouse over the middle of what selector names, as a
// desktop's pointer does: the views from it out that hover matters to
// (hover styles or hover events) replace those hovered before, which
// hear HoverOut, the new ones HoverIn, and each hovered view HoverMove.
func (a *App) Hover(selector string) {
	a.t.Helper()
	a.step("hover(%s)", selector)
	a.hoverAt(selector, nil)
}

// HoverAt moves the mouse over what selector names as Hover does, to at in
// its coordinates: a chart's row under the pointer.
func (a *App) HoverAt(selector string, at driver.Point) {
	a.t.Helper()
	a.step("hoverAt(%s, %v, %v)", selector, at.X, at.Y)
	a.hoverAt(selector, &at)
}

func (a *App) hoverAt(selector string, local *driver.Point) {
	a.t.Helper()
	v := a.pick(selector, "view", func(m *View) (*View, bool) { return m, true })
	mid, at := v.center()
	shift := driver.Point{}
	if local != nil {
		shift = driver.Point{X: local.X - mid.X, Y: local.Y - mid.Y}
	}
	at = driver.Point{X: at.X + shift.X, Y: at.Y + shift.Y}
	if a.target != nil {
		c := a.target.reveal(v)
		a.target.hover(driver.Point{X: c.X + shift.X, Y: c.Y + shift.Y})
		a.done()
		return
	}
	var next []*View
	for p := v; p != nil; p = p.Parent {
		if p.Events.Has(driver.EventHoverIn) || p.Events.Has(driver.EventHoverOut) || p.Events.Has(driver.EventHoverMove) {
			next = append(next, p)
		}
	}
	a.setHovered(next, at)
	for _, h := range next {
		ev := a.pressEvent(h, at)
		ev.Kind, ev.Count = driver.EventHoverMove, 0
		a.send(h, ev)
	}
	a.done()
}

// HoverOut moves the mouse out of the surface: every view hovered hears
// HoverOut.
func (a *App) HoverOut() {
	a.t.Helper()
	a.step("hoverOut")
	if a.target != nil {
		a.target.hover(driver.Point{X: -1, Y: -1})
		a.done()
		return
	}
	a.setHovered(nil, driver.Point{})
	a.done()
}

// setHovered makes next, innermost first, the views hovered: those that
// leave hear HoverOut, innermost first, and those that come HoverIn,
// outermost first.
func (a *App) setHovered(next []*View, at driver.Point) {
	for _, h := range a.hovered {
		if !containsView(next, h) && a.views[h.ID] == h {
			h.Hovered = false
			a.send(h, driver.Event{Kind: driver.EventHoverOut})
		}
	}
	for i := len(next) - 1; i >= 0; i-- {
		if h := next[i]; !containsView(a.hovered, h) {
			h.Hovered = true
			a.send(h, driver.Event{Kind: driver.EventHoverIn})
		}
	}
	a.hovered = next
}

// Scroll drags the scroll view selector names (the match, the nearest
// scroll view around it, else the first in it) by distance points along
// its axis, 10 points a display frame within its content, as a finger
// does: ScrollDragStart, the offsets as platforms report them, then
// ScrollDragEnd. It fails when the view cannot move that way at all or
// the user cannot scroll it.
func (a *App) Scroll(selector string, distance driver.Points) {
	a.t.Helper()
	a.step("scroll(%s, %v)", selector, distance)
	isScroll := func(v *View) bool { return v.Kind == driver.KindScroll }
	v := a.pick(selector, "scroll view", func(m *View) (*View, bool) {
		t := m.upTo(isScroll)
		if t == nil {
			t = m.downTo(isScroll)
		}
		return t, t != nil && !t.Scroll.Behavior.NoScroll
	})
	at := func() driver.Points {
		if v.Scroll.Horizontal {
			return v.Offset.X
		}
		return v.Offset.Y
	}
	start := at()
	to := min(max(start+distance, 0), v.scrollEnd())
	if math.Abs(float64(to-start)) < 0.5 {
		a.t.Fatalf("nsgotest: %q cannot scroll by %v: it is at %v of %v", selector, distance, start, v.scrollEnd())
	}
	if a.target != nil {
		a.target.scroll(v, to-start)
		a.done()
		return
	}
	a.scrollEvent(v, driver.EventScrollDragStart)
	f := &finger{}
	// The app may shift the offset meanwhile (ScrollBy): each frame moves
	// on from where the view is.
	for frames := int(math.Abs(float64(to-start))/dragStep)*2 + 10; frames > 0; frames-- {
		now := at()
		if math.Abs(float64(to-now)) < 0.5 {
			break
		}
		next := min(now+dragStep, to)
		if to < now {
			next = max(now-dragStep, to)
		}
		off := v.Offset
		if v.Scroll.Horizontal {
			off.X = next
		} else {
			off.Y = next
		}
		a.scrollView(v, off)
		a.frame(f)
	}
	a.scrollEvent(v, driver.EventScrollDragEnd)
	a.settle(v)
	a.done()
}

// ScrollTo reports that the scroll view selector matches is at offset, as
// a platform reports where it is; it moves no finger.
func (a *App) ScrollTo(selector string, offset driver.Point) {
	a.t.Helper()
	v := a.Find(selector)
	a.scrollView(v, offset)
}

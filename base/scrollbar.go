package base

import (
	"time"

	"github.com/oneo-me/nsgo/core"
)

// ScrollbarMode is when a Scrollbar shows. A drag of its thumb keeps it
// shown in every mode, and it never shows while the content fits.
type ScrollbarMode uint8

const (
	// ScrollbarScrolling shows it while the target scrolls, until Idle
	// after it stops; the pointer on the bar while it shows keeps it.
	ScrollbarScrolling ScrollbarMode = iota
	// ScrollbarHover also shows it while the pointer is over it.
	ScrollbarHover
	// ScrollbarAlways shows it always.
	ScrollbarAlways
)

// ScrollbarEntrance is how a Scrollbar comes in.
type ScrollbarEntrance uint8

const (
	// ScrollbarFade fades it in without moving.
	ScrollbarFade ScrollbarEntrance = iota
	// ScrollbarSlideAndFade slides it in from its edge while it fades.
	ScrollbarSlideAndFade
)

// ScrollbarMotion is how a Scrollbar shows and hides, natively.
type ScrollbarMotion struct {
	// Idle is how long it stays after the last scroll, drag or hover, in
	// milliseconds, 2000 when 0.
	Idle float64
	// Enter and Exit are how long it takes to appear and to go, in
	// milliseconds; 0 snaps.
	Enter, Exit float64
	Entrance    ScrollbarEntrance
	// ThumbHoverEntrance is how it comes in when the pointer on its hidden
	// thumb shows it.
	ThumbHoverEntrance ScrollbarEntrance
}

// defaultThumbLength is the shortest a thumb gets by default.
const defaultThumbLength = 48

// Scrollbar is a bar of the app's look for a scroll view or list it names,
// placed by the app like any other view. It owns the thumb's length and
// place, which follow the target's content, viewport and offset, dragging
// the thumb or pressing the track, which scrolls the target, and when it
// shows; it has no look of its own. Hidden, it takes no presses. Turn the
// target's own indicator off with HideIndicator.
type Scrollbar struct {
	// Scroll is the view it follows and moves, read in every commit.
	Scroll func() *core.Ref
	// Horizontal says the target scrolls sideways; read as it is made.
	Horizontal bool
	Mode       func() ScrollbarMode
	Motion     func() ScrollbarMotion
	// MinThumbLength is the shortest the thumb gets, 48 when 0.
	MinThumbLength func() core.Points
	Class          string
	Style          core.Style
	StyleFunc      func() core.Style
	ThumbClass     string
	ThumbStyle     core.Style
	// ThumbStyleFunc is the thumb's style following what it reads.
	ThumbStyleFunc func() core.Style
	Label          string
	TestID         string
}

// scrollbarCtl is what the bar's parts and listeners share.
type scrollbarCtl struct {
	s          Scrollbar
	cx         *core.Scope
	metrics    *core.Signal[core.ScrollEvent]
	measured   *core.Signal[bool]
	rail       *core.Signal[float32]
	railOrigin core.Point
	active     *core.Signal[bool]
	overTrack  *core.Signal[bool]
	overThumb  *core.Signal[bool]
	dragging   *core.Signal[bool]
	from       float32
	deadline   time.Time
	opacity    *core.AnimatedValue
	position   *core.AnimatedValue
	fade       scrollbarFade
}

// scrollbarFade is the bar's fade toward shown (1) or hidden (0), sampled
// to start a reversed fade where the native one is.
type scrollbarFade struct {
	opacity, position, target float32
	entrance                  ScrollbarEntrance
	start                     time.Time
	duration                  float32
}

func (f scrollbarFade) sample(now time.Time) (float32, float32) {
	t := float32(1)
	if f.duration > 0 {
		t = min(max(float32(now.Sub(f.start).Seconds()*1000)/f.duration, 0), 1)
	}
	in := t * t * t
	fade, slide := in, in
	if f.target > f.opacity || f.target > f.position {
		u := 1 - t
		fade, slide = t, 1-u*u*u
	}
	return f.opacity + (f.target-f.opacity)*fade, f.position + (f.target-f.position)*slide
}

func (c *scrollbarCtl) main(p core.Point) float32 {
	if c.s.Horizontal {
		return float32(p.X)
	}
	return float32(p.Y)
}

func (c *scrollbarCtl) size(s core.Size) float32 {
	if c.s.Horizontal {
		return float32(s.Width)
	}
	return float32(s.Height)
}

func (c *scrollbarCtl) along() (view, content, offset float32, ok bool) {
	if !c.measured.Get() {
		return 0, 0, 0, false
	}
	m := c.metrics.Get()
	return c.size(m.Size), c.size(m.ContentSize), c.main(m.Offset), true
}

// scrollRange is how far the target scrolls.
func (c *scrollbarCtl) scrollRange() float32 {
	view, content, _, ok := c.along()
	if !ok {
		return 0
	}
	return max(content-view, 0)
}

// thumb is the thumb's length.
func (c *scrollbarCtl) thumb() float32 {
	rail := c.rail.Get()
	view, content, _, ok := c.along()
	if !ok {
		return rail
	}
	shortest := float32(defaultThumbLength)
	if c.s.MinThumbLength != nil {
		if m := c.s.MinThumbLength(); m > 0 {
			shortest = float32(m)
		}
	}
	shortest = min(shortest, rail)
	return min(max(rail*view/max(content, 1), shortest), rail)
}

// position is where the thumb starts along the rail: the offset, held to
// the content, so overscrolls leave it at an end.
func (c *scrollbarCtl) thumbAt() float32 {
	r := c.scrollRange()
	_, _, offset, ok := c.along()
	if !ok || r <= 0 {
		return 0
	}
	return max(c.rail.Get()-c.thumb(), 0) * min(max(offset/r, 0), 1)
}

func (c *scrollbarCtl) offsetAt(position float32) float32 {
	return core.Untracked(c.cx, func() float32 {
		travel := max(c.rail.Get()-c.thumb(), 0)
		if travel <= 0 {
			return 0
		}
		return min(max(position/travel, 0), 1) * c.scrollRange()
	})
}

// scrollTo scrolls the target along the axis. The thumb moves in the same
// batch as the command, not when the platform reports the scroll.
func (c *scrollbarCtl) scrollTo(offset float32) {
	r := core.Untracked(c.cx, c.s.Scroll)
	if r == nil || !c.measured.GetUntracked() {
		return
	}
	m := c.metrics.GetUntracked()
	if c.s.Horizontal {
		m.Offset.X = core.Points(offset)
	} else {
		m.Offset.Y = core.Points(offset)
	}
	r.ScrollTo(m.Offset, false)
	c.metrics.Set(m)
}

func (c *scrollbarCtl) motion() ScrollbarMotion {
	var m ScrollbarMotion
	if c.s.Motion != nil {
		m = core.Untracked(c.cx, c.s.Motion)
	}
	if m.Idle == 0 {
		m.Idle = 2000
	}
	return m
}

func (c *scrollbarCtl) mode() ScrollbarMode {
	if c.s.Mode != nil {
		return c.s.Mode()
	}
	return ScrollbarScrolling
}

// touch keeps the bar shown: it hides Idle after the last. One timer runs
// at a time and waits again while activity moved the deadline on.
func (c *scrollbarCtl) touch() {
	if !c.active.GetUntracked() {
		c.active.Set(true)
	}
	delay := time.Duration(max(c.motion().Idle, 0) * float64(time.Millisecond))
	waiting := !c.deadline.IsZero()
	c.deadline = c.cx.Now().Add(delay)
	if !waiting {
		c.hideAfter(delay)
	}
}

func (c *scrollbarCtl) hideAfter(delay time.Duration) {
	c.cx.AfterFunc(delay, func() {
		if c.deadline.IsZero() {
			return
		}
		if left := c.deadline.Sub(c.cx.Now()); left > 0 {
			c.hideAfter(left)
			return
		}
		c.deadline = time.Time{}
		c.active.Set(false)
	})
}

func (c *scrollbarCtl) shown() bool {
	if c.scrollRange() <= 1 {
		return false
	}
	return c.mode() == ScrollbarAlways || c.dragging.Get() || c.overTrack.Get() || c.active.Get()
}

// fadeTo fades the bar toward visible, from where its fade is.
func (c *scrollbarCtl) fadeTo(visible, first bool) {
	target := float32(0)
	if visible {
		target = 1
	}
	motion, mode := c.motion(), core.Untracked(c.cx, c.mode)
	entrance := motion.Entrance
	if mode == ScrollbarHover && c.overThumb.GetUntracked() {
		entrance = motion.ThumbHoverEntrance
	}
	full := float32(motion.Exit)
	if visible {
		full = float32(motion.Enter)
	}
	if first || mode == ScrollbarAlways || core.Untracked(c.cx, c.cx.Env().ReduceMotion) {
		full = 0
	}
	now := c.cx.Now()
	if full <= 0 {
		c.opacity.Set(float64(target))
		c.position.Set(float64(target))
		c.fade = scrollbarFade{opacity: target, position: target, target: target, start: now}
		return
	}
	if c.fade.target == target && c.fade.entrance == entrance {
		return
	}
	opacity, position := c.fade.sample(now)
	from := position
	if visible && entrance == ScrollbarFade {
		from = 1
	}
	if from != position {
		c.position.Set(float64(from))
	}
	abs := func(v float32) float32 { return max(v, -v) }
	duration := full * max(abs(target-opacity), abs(target-from))
	in := core.BezierEasing(core.Curve{X1: 0.32, Y1: 0, X2: 0.67, Y2: 0})
	out := core.BezierEasing(core.Curve{X1: 0.33, Y1: 1, X2: 0.68, Y2: 1})
	fade, slide := in, in
	if visible {
		fade, slide = core.Easing{}, out
	}
	d := time.Duration(duration * float32(time.Millisecond))
	c.cx.StartAnimation(core.Timing(c.opacity, float64(target), core.TimingConfig{Duration: d, Easing: fade}), nil)
	c.cx.StartAnimation(core.Timing(c.position, float64(target), core.TimingConfig{Duration: d, Easing: slide}), nil)
	c.fade = scrollbarFade{opacity: opacity, position: from, target: target, entrance: entrance, start: now, duration: duration}
}

func (c *scrollbarCtl) hoverTrack(over bool) {
	c.overTrack.Set(over)
	c.touch()
}

// pressTrack puts the thumb's middle at the press, and a drag goes on from
// there.
func (c *scrollbarCtl) pressTrack(e core.ResponderEvent) {
	along := c.main(e.Position) - c.main(c.railOrigin)
	thumb := core.Untracked(c.cx, c.thumb)
	offset := c.offsetAt(along - thumb/2)
	c.from = offset
	c.dragging.Set(true)
	c.scrollTo(offset)
}

func (c *scrollbarCtl) pressThumb(core.ResponderEvent) {
	var offset, r float32
	c.cx.Untrack(func() {
		_, _, offset, _ = c.along()
		r = c.scrollRange()
	})
	c.from = min(max(offset, 0), r)
	c.dragging.Set(true)
}

func (c *scrollbarCtl) dragBy(e core.ResponderEvent) {
	if !c.dragging.GetUntracked() {
		return
	}
	var travel, r float32
	c.cx.Untrack(func() {
		travel, r = max(c.rail.Get()-c.thumb(), 0), c.scrollRange()
	})
	if travel <= 0 {
		return
	}
	c.scrollTo(min(max(c.from+c.main(e.Gesture.Delta)*r/travel, 0), r))
}

func (c *scrollbarCtl) release(core.ResponderEvent) {
	if c.dragging.GetUntracked() {
		c.dragging.Set(false)
		c.touch()
	}
}

func (c *scrollbarCtl) responder(grant func(core.ResponderEvent)) *core.Responder {
	return &core.Responder{Keep: true, Claims: []core.TouchClaim{{}}, OnGrant: grant, OnMove: c.dragBy, OnRelease: c.release, OnTerminate: c.release}
}

// Render implements core.Node.
func (s Scrollbar) Render(cx *core.Scope) core.Node {
	c := &scrollbarCtl{s: s, cx: cx, metrics: core.NewSignal(cx, core.ScrollEvent{}), measured: core.NewSignal(cx, false),
		rail: core.NewSignal[float32](cx, 0), active: core.NewSignal(cx, false), overTrack: core.NewSignal(cx, false),
		overThumb: core.NewSignal(cx, false), dragging: core.NewSignal(cx, false),
		opacity: core.NewAnimatedValue(cx, 0), position: core.NewAnimatedValue(cx, 0), fade: scrollbarFade{start: cx.Now()}}
	target := func() *core.Ref {
		if s.Scroll != nil {
			return s.Scroll()
		}
		return nil
	}
	cx.WatchScroll(target, func(e core.ScrollEvent) {
		first := !c.measured.GetUntracked()
		before := c.metrics.GetUntracked()
		// While a drag scrolls the target the thumb stays where the drag
		// put it: reports of the commands before trail it.
		if !first && c.dragging.GetUntracked() {
			if s.Horizontal {
				e.Offset.X = before.Offset.X
			} else {
				e.Offset.Y = before.Offset.Y
			}
		}
		cx.Batch(func() {
			c.metrics.Set(e)
			c.measured.Set(true)
		})
		if !first && before.Offset != e.Offset {
			c.touch()
		}
	})
	shown := core.NewMemo(cx, c.shown)
	first := true
	cx.Effect(func() {
		visible := shown.Get()
		cx.Untrack(func() {
			c.fadeTo(visible, first)
			first = false
		})
	})
	env := cx.Env()
	hoverReveals := func() bool { return c.mode() == ScrollbarHover && env.Desktop() }
	takes := func() bool { return shown.Get() || hoverReveals() }
	inner := func() core.PointerEvents {
		if takes() {
			return core.PointerEventsBoxNone
		}
		return core.PointerEventsNone
	}
	horizontal := s.Horizontal
	thickness := core.NewAnimatedValue(cx, 0)
	thumb := core.View{NoFocus: true, Class: s.ThumbClass, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pt(0)}.Merge(s.ThumbStyle), StyleFunc: s.ThumbStyleFunc,
		OnMouseEnter: func(core.MouseEvent) { c.overThumb.Set(true) },
		OnMouseLeave: func(core.MouseEvent) { c.overThumb.Set(false) },
		Responder:    c.responder(c.pressThumb)}
	// The thumb's place and length, on a view of the bar's own, which the
	// app's transitions on the thumb never reach.
	holder := core.View{Style: core.Style{FlexShrink: 0},
		StyleFunc: func() core.Style {
			out := core.Style{PointerEvents: inner()}
			if horizontal {
				out.Direction, out.Width = core.Row, core.Pt(core.Points(c.thumb()))
				out.Transform.TranslateX = core.Points(c.thumbAt())
			} else {
				out.Height = core.Pt(core.Points(c.thumb()))
				out.Transform.TranslateY = core.Points(c.thumbAt())
			}
			return out
		},
		Children: []core.Node{thumb}}
	// Off its edge by the track's thickness until the slide brings it in.
	slide := core.Mul(core.Sub(core.Constant(1), c.position), thickness)
	railAnimated := &core.AnimatedStyle{TranslateX: slide}
	if horizontal {
		railAnimated = &core.AnimatedStyle{TranslateY: slide}
	}
	rail := core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pt(0), Animated: railAnimated},
		StyleFunc: func() core.Style {
			out := core.Style{PointerEvents: inner()}
			if horizontal {
				out.Direction = core.Row
			}
			return out
		},
		OnLayout: func(b core.Bounds) {
			c.railOrigin = b.Origin
			c.rail.Set(c.size(b.Size))
		},
		Children: []core.Node{holder}}
	base := core.Style{Group: "scrollbar", Overflow: core.Hidden, Animated: &core.AnimatedStyle{Opacity: c.opacity}}
	if horizontal {
		base.Direction = core.Row
	}
	return core.View{Class: s.Class, Style: base.Merge(s.Style), Label: s.Label, TestID: s.TestID, NoFocus: true,
		StyleFunc: func() core.Style {
			out := core.Style{PointerEvents: core.PointerEventsNone}
			if takes() {
				out.PointerEvents = core.PointerEventsAuto
			}
			if s.StyleFunc != nil {
				out = out.Merge(s.StyleFunc())
			}
			return out
		},
		OnLayout: func(b core.Bounds) {
			if horizontal {
				thickness.Set(float64(b.Size.Height))
			} else {
				thickness.Set(float64(b.Size.Width))
			}
		},
		OnMouseEnter: func(core.MouseEvent) { c.hoverTrack(true) },
		OnMouseLeave: func(core.MouseEvent) { c.hoverTrack(false) },
		Responder:    c.responder(c.pressTrack),
		Children:     []core.Node{rail}}
}

// ScrollbarLook is the look of the built-in bars of one axis, placed in the
// view's viewport (absolute inset-y-0 right-0 w-4 p-1).
type ScrollbarLook struct {
	Class          string
	Style          core.Style
	ThumbClass     string
	ThumbStyle     core.Style
	ThumbStyleFunc func() core.Style
	Label          string
}

// ScrollbarTheme draws built-in bars in place of the platform's indicator,
// for every view with SetScrollbarTheme, or for one as its Scroll.Bar. An
// axis with no look keeps the platform's.
type ScrollbarTheme struct {
	Mode           ScrollbarMode
	Motion         ScrollbarMotion
	MinThumbLength core.Points
	Vertical       *ScrollbarLook
	Horizontal     *ScrollbarLook
}

// Scrollbar implements core.ScrollbarMaker.
func (t *ScrollbarTheme) Scrollbar(_ *core.Scope, target *core.Ref, horizontal bool) core.Node {
	look := t.Vertical
	if horizontal {
		look = t.Horizontal
	}
	if look == nil {
		return nil
	}
	return Scrollbar{Scroll: func() *core.Ref { return target }, Horizontal: horizontal,
		Mode: func() ScrollbarMode { return t.Mode }, Motion: func() ScrollbarMotion { return t.Motion },
		MinThumbLength: func() core.Points { return t.MinThumbLength },
		Class:          look.Class, Style: look.Style, ThumbClass: look.ThumbClass, ThumbStyle: look.ThumbStyle, ThumbStyleFunc: look.ThumbStyleFunc, Label: look.Label}
}

// SetScrollbarTheme draws the bar of every scroll view and list that shows
// its indicator with a Scrollbar of theme; nil gives them the platform's
// back. A view with a Bar of its own keeps it.
func SetScrollbarTheme(cx *core.Scope, theme *ScrollbarTheme) {
	if theme == nil {
		cx.SetScrollbars(nil)
		return
	}
	cx.SetScrollbars(theme)
}

package base

import (
	"math"
	"reflect"
	"slices"
	"sort"
	"time"

	"github.com/oneo-me/nsgo/core"
)

const (
	// carouselEpsilon is how far apart two offsets may be and still be one
	// snap point.
	carouselEpsilon = 0.5
	// carouselRecenter is how long after an animated scroll across the
	// seam the offset moves back into the middle period, when the platform
	// reported no rest before: longer than the platforms' animated
	// scrolls.
	carouselRecenter = 600 * time.Millisecond
	// A slide's translation reads these while the loop is not seamless: an
	// offset and a period so large the nearest period is its own.
	carouselStillShift  = 1.0e6
	carouselStillPeriod = 1.0e7
	// carouselSuperseded is how many replaced scrolls are remembered: a
	// rest reported for one is on its way for a frame or two only.
	carouselSuperseded = 3
)

// carouselIndicator is the pagination indicator's travel, as Tabs'.
var carouselIndicator = core.LayoutTransition{Curve: core.LayoutSpring, Damping: 0.85, Duration: 362 * time.Millisecond}

// carouselSlide is a slide of the content, as it registered.
type carouselSlide struct {
	ref *core.Ref
	// start and length are where it is laid out along the axis, once
	// placed.
	start, length float64
	placed        bool
	// shift is what its translation adds to the scroll offset while the
	// carousel loops.
	shift *core.AnimatedValue
	sent  float64
}

// carouselTrack is where the slides rest, from their layout.
type carouselTrack struct {
	// snaps are each slide's snap offset, in order along the axis; empty
	// until every slide is laid out.
	snaps []float64
	// period is a seamless loop's: from the first slide's start to the
	// last's end, and the gap after it; runway that less the gap. 0 while
	// it does not loop seamlessly.
	period, runway float64
	// origin is the first slide's rest offset in the middle period: past
	// the runway and whatever gap the content leaves after it, which is
	// none when the slides space themselves with margins.
	origin, viewport float64
}

func carouselTracksEqual(a, b carouselTrack) bool {
	return a.period == b.period && a.runway == b.runway && a.origin == b.origin && a.viewport == b.viewport && slices.Equal(a.snaps, b.snaps)
}

// ready says the slides are where their rests are: while the loop is
// seamless, once its runway is in the layout before the first slide.
func (t carouselTrack) ready() bool { return t.period == 0 || t.origin+carouselEpsilon >= t.runway }

// rest is a slide's rest offset in the middle period.
func (t carouselTrack) rest(i int) (float64, bool) {
	if i < 0 || i >= len(t.snaps) {
		return 0, false
	}
	return t.snaps[i] + t.origin, true
}

// carouselFirstAt is the first of the slides whose snap point is slide i's.
func carouselFirstAt(snaps []float64, i int) int {
	for j, s := range snaps {
		if math.Abs(s-snaps[i]) <= carouselEpsilon {
			return j
		}
	}
	return i
}

// nearest is the snap point nearest offset in any period: its slide, where
// it is, and its period around the middle one.
func (t carouselTrack) nearest(offset float64) (index int, at float64, cycle int, ok bool) {
	cycles := []int{0}
	if t.period > 0 {
		cycles = []int{-1, 0, 1}
	}
	best := math.Inf(1)
	for _, k := range cycles {
		for i, s := range t.snaps {
			p := s + t.origin + t.period*float64(k)
			if d := math.Abs(p - offset); d < best {
				best, index, at, cycle, ok = d, i, p, k, true
			}
		}
	}
	if ok {
		index = carouselFirstAt(t.snaps, index)
	}
	return index, at, cycle, ok
}

// carouselCtl is what the parts of a Carousel share.
type carouselCtl struct {
	cx      *core.Scope
	axis    Axis
	looping bool
	index   func() int
	onEvent Carousel
	// slides are in registration order; sortedSlides gives the tree's.
	slides  []*carouselSlide
	changed *core.Signal[int]
	count   *core.Memo[int]
	track   *core.Memo[carouselTrack]
	// max and viewport are what the layout reported, kept until the track
	// takes them whole, as a layout reports each slide's frame apart.
	max, viewport float64
	known         bool
	// runwayAt is where each runway is laid out along the axis.
	runwayAt [2]float64
	settled  *core.Signal[int]
	waiting  bool
	offset   *core.AnimatedValue
	period   *core.AnimatedValue
	own      *core.Signal[int]
	// pending is where a swipe rested until the app answered, -1 for none.
	pending        *core.Signal[int]
	answeredInDrag bool
	shown          *core.Memo[int]
	root, scroller *core.Ref
	at             float64
	inFlight       *float64
	superseded     []float64
	placed         int
	placedPeriod   float64
	wrap           int
	generation     int
	dragging       *core.Signal[bool]
	focused        *core.Signal[int]
}

// isZeroStyle reports whether s sets nothing.
func isZeroStyle(s core.Style) bool { return reflect.ValueOf(s).IsZero() }

func (c *carouselCtl) makeTrack() carouselTrack {
	c.settled.Get()
	var frames [][2]float64
	for _, s := range c.slides {
		if s.placed {
			frames = append(frames, [2]float64{s.start, s.length})
		}
	}
	if len(frames) == 0 || len(frames) != len(c.slides) {
		return carouselTrack{}
	}
	sort.Slice(frames, func(i, j int) bool { return frames[i][0] < frames[j][0] })
	first := frames[0][0]
	last := frames[len(frames)-1]
	gap := 0.0
	if len(frames) > 1 {
		gap = math.Max(frames[1][0]-first-frames[0][1], 0)
	}
	period := last[0] + last[1] - first + gap
	widest := 0.0
	for _, f := range frames {
		widest = math.Max(widest, f[1])
	}
	// A seamless loop shows each slide once: the slides span the viewport
	// and one more slide at least.
	seamless := c.looping && len(frames) > 1 && c.known && period+carouselEpsilon >= c.viewport+widest
	t := carouselTrack{viewport: c.viewport}
	for _, f := range frames {
		s := math.Max(f[0]-first, 0)
		if !seamless && c.known {
			s = math.Min(s, c.max)
		}
		t.snaps = append(t.snaps, s)
	}
	if seamless {
		t.period, t.runway = period, period-gap
		// Past the content's padding, which the nearer runway starts at.
		t.origin = first - min(c.runwayAt[0], c.runwayAt[1])
	}
	return t
}

// settleLater makes the track from what the layout reported, once, after
// the commit's reports are all in.
func (c *carouselCtl) settleLater() {
	if c.waiting {
		return
	}
	c.waiting = true
	c.cx.Post(func() {
		c.waiting = false
		c.settled.Update(func(n int) int { return n + 1 })
	})
}

// followTrack keeps the slides' translations on the track.
func (c *carouselCtl) followTrack() {
	c.cx.Effect(func() {
		t := c.track.Get()
		c.cx.Untrack(func() {
			if t.period > 0 {
				c.period.Set(t.period)
			} else {
				c.period.Set(carouselStillPeriod)
			}
			for _, s := range c.slides {
				if s.shift == nil || !s.placed {
					continue
				}
				to := float64(carouselStillShift)
				if t.period > 0 {
					to = t.viewport/2 - (s.start + s.length/2) + t.period/2
				}
				if s.sent != to {
					s.sent = to
					s.shift.Set(to)
				}
			}
		})
	})
}

// register adds the slide whose view is ref until cx ends, returning its
// translation while the carousel loops.
func (c *carouselCtl) register(cx *core.Scope, ref *core.Ref) (*carouselSlide, core.AnimatedNode) {
	s := &carouselSlide{ref: ref, sent: carouselStillShift}
	var translation core.AnimatedNode
	if c.looping {
		s.shift = core.NewAnimatedValue(cx, carouselStillShift)
		// y - (y mod period): the whole periods between the slide and the
		// viewport's center.
		y := core.Add(c.offset, s.shift)
		translation = core.Sub(y, core.Mod(y, c.period))
	}
	c.slides = append(c.slides, s)
	cx.Untrack(func() { c.changed.Update(func(n int) int { return n + 1 }) })
	cx.Cleanup(func() {
		c.slides = slices.DeleteFunc(c.slides, func(o *carouselSlide) bool { return o == s })
		cx.Untrack(func() { c.changed.Update(func(n int) int { return n + 1 }) })
		c.settleLater()
	})
	return s, translation
}

// place hears slide s laid out at frame.
func (c *carouselCtl) place(s *carouselSlide, frame core.Bounds) {
	start, length := c.along(frame)
	if s.placed && s.start == start && s.length == length {
		return
	}
	s.placed, s.start, s.length = true, start, length
	c.settleLater()
}

// placeRunway hears runway i laid out at frame.
func (c *carouselCtl) placeRunway(i int, frame core.Bounds) {
	if start, _ := c.along(frame); c.runwayAt[i] != start {
		c.runwayAt[i] = start
		c.settleLater()
	}
}

// along is where frame starts on the axis and how long it is.
func (c *carouselCtl) along(frame core.Bounds) (start, length float64) {
	if c.axis == Vertical {
		return float64(frame.Origin.Y), float64(frame.Size.Height)
	}
	return float64(frame.Origin.X), float64(frame.Size.Width)
}

func (c *carouselCtl) measured(e core.ScrollEvent) {
	content, viewport := float64(e.ContentSize.Width), float64(e.Size.Width)
	if c.axis == Vertical {
		content, viewport = float64(e.ContentSize.Height), float64(e.Size.Height)
	}
	m := math.Max(content-viewport, 0)
	if c.known && c.max == m && c.viewport == viewport {
		return
	}
	c.known, c.max, c.viewport = true, m, viewport
	c.settleLater()
}

// step is the slide from moves to, with a snap point of its own, around
// the ends while looping, -1 at an end.
func (c *carouselCtl) step(from int, forward bool) int {
	count := c.count.Get()
	if count == 0 {
		return -1
	}
	snaps := c.track.Get().snaps
	if len(snaps) != count {
		wrap := c.looping && count > 1
		switch {
		case forward && from+1 < count:
			return from + 1
		case forward && wrap:
			return 0
		case !forward && from > 0:
			return from - 1
		case !forward && wrap:
			return count - 1
		}
		return -1
	}
	at := snaps[min(from, count-1)]
	wrap := c.looping && slices.ContainsFunc(snaps, func(s float64) bool { return math.Abs(s-snaps[0]) > carouselEpsilon })
	if forward {
		if i := slices.IndexFunc(snaps, func(s float64) bool { return s > at+carouselEpsilon }); i >= 0 {
			return i
		}
		if wrap {
			return 0
		}
		return -1
	}
	for i := count - 1; i >= 0; i-- {
		if snaps[i] < at-carouselEpsilon {
			return carouselFirstAt(snaps, i)
		}
	}
	if wrap {
		return carouselFirstAt(snaps, count-1)
	}
	return -1
}

func (c *carouselCtl) last() int {
	count := c.count.GetUntracked()
	if count == 0 {
		return -1
	}
	if snaps := c.track.GetUntracked().snaps; len(snaps) == count {
		return carouselFirstAt(snaps, count-1)
	}
	return count - 1
}

func (c *carouselCtl) report(index int) {
	if c.onEvent.OnChange != nil {
		c.onEvent.OnChange(index)
	}
}

// goTo asks for a slide from a control, a key or a pagination item.
func (c *carouselCtl) goTo(index int) {
	if index < 0 || index == c.shown.GetUntracked() {
		return
	}
	if c.index == nil {
		c.own.Set(index)
	}
	c.report(index)
}

func (c *carouselCtl) goStep(forward bool) {
	from := c.shown.GetUntracked()
	to := core.Untracked(c.cx, func() int { return c.step(from, forward) })
	if to < 0 {
		return
	}
	c.wrap = 0
	if forward && to <= from {
		c.wrap = 1
	} else if !forward && to >= from {
		c.wrap = -1
	}
	c.goTo(to)
}

func (c *carouselCtl) dragStarted() {
	c.inFlight, c.superseded = nil, nil
	if !c.dragging.GetUntracked() {
		c.dragging.Set(true)
		if c.onEvent.OnDragStart != nil {
			c.onEvent.OnDragStart()
		}
	}
}

// supersededRest says a rest at offset is that of a scroll sent and then
// replaced: no slide of the user's. Any other rest ends the wait.
func (c *carouselCtl) supersededRest(offset float64) bool {
	near := func(at float64) bool { return math.Abs(at-offset) <= carouselEpsilon }
	arrived := c.inFlight != nil && near(*c.inFlight)
	if !arrived && slices.ContainsFunc(c.superseded, near) {
		return true
	}
	c.inFlight, c.superseded = nil, nil
	return false
}

// rested hears the scroll come to rest at offset: back in the middle
// period, on a slide, which shows, and the app hears of it. An app's Index
// shows again once it has answered, an app's OnChange running right here.
func (c *carouselCtl) rested(offset float64) {
	if c.supersededRest(offset) {
		return
	}
	c.at = offset
	if c.dragging.GetUntracked() {
		c.dragging.Set(false)
		if c.onEvent.OnDragEnd != nil {
			c.onEvent.OnDragEnd()
		}
	}
	t := c.track.GetUntracked()
	index, target, cycle, ok := t.nearest(offset)
	if !ok {
		return
	}
	if math.Abs(target-offset) > carouselEpsilon {
		// A rest between snap points lands on the slide.
		c.scrollTo(target, !core.Untracked(c.cx, c.cx.Env().ReduceMotion))
		c.recenterLater()
	} else if cycle != 0 {
		c.recenter()
	}
	c.placed = index
	if index == c.shown.GetUntracked() {
		return
	}
	if c.index == nil {
		c.own.Set(index)
		c.report(index)
		return
	}
	c.pending.Set(index)
	c.report(index)
	c.pending.Set(-1)
}

func (c *carouselCtl) recenter() {
	t := c.track.GetUntracked()
	if t.period == 0 {
		return
	}
	if _, _, cycle, ok := t.nearest(c.at); ok && cycle != 0 {
		c.scrollTo(c.at-t.period*float64(cycle), false)
	}
}

func (c *carouselCtl) recenterLater() {
	generation := c.generation
	c.cx.AfterFunc(carouselRecenter, func() {
		if c.generation == generation && !c.dragging.GetUntracked() {
			c.recenter()
		}
	})
}

func (c *carouselCtl) scrollTo(offset float64, animated bool) {
	if math.Abs(c.at-offset) <= carouselEpsilon {
		return
	}
	c.at = offset
	c.generation++
	replaced := c.inFlight
	c.inFlight = nil
	if animated {
		c.inFlight = &offset
	}
	if replaced != nil && math.Abs(*replaced-offset) > carouselEpsilon {
		c.superseded = append(c.superseded, *replaced)
		if len(c.superseded) > carouselSuperseded {
			c.superseded = c.superseded[1:]
		}
	}
	to := core.Point{X: core.Points(offset)}
	if c.axis == Vertical {
		to = core.Point{Y: core.Points(offset)}
	}
	c.scroller.ScrollTo(to, animated)
}

// scrollToSlide scrolls to a slide from the one placed before, across the
// seam to the next or the previous period as it goes around the ends.
func (c *carouselCtl) scrollToSlide(t carouselTrack, from int, sameLoop bool, index int) {
	rest, ok := t.rest(index)
	if !ok {
		return
	}
	moved := from >= 0 && from != index
	animated := moved && !core.Untracked(c.cx, c.cx.Env().ReduceMotion)
	wrap := c.wrap
	c.wrap = 0
	if t.period == 0 {
		c.scrollTo(rest, animated)
		return
	}
	if !animated {
		d := math.Mod(math.Mod(c.at-rest, t.period)+t.period, t.period)
		there := d <= carouselEpsilon || t.period-d <= carouselEpsilon
		if !(sameLoop && !moved && from >= 0 && there) {
			c.scrollTo(rest, false)
		}
		return
	}
	if from < 0 {
		from = index
	}
	count := len(t.snaps)
	last := carouselFirstAt(t.snaps, count-1)
	turn := 0
	if from == last && index == 0 && (count > 2 || wrap > 0) {
		turn = 1
	} else if from == 0 && index == last && (count > 2 || wrap < 0) {
		turn = -1
	}
	_, _, cycle, _ := t.nearest(c.at)
	if cycle+turn > 1 || cycle+turn < -1 {
		c.recenter()
		cycle = 0
	}
	c.scrollTo(rest+t.period*float64(cycle+turn), true)
	if cycle+turn != 0 {
		c.recenterLater()
	}
}

// keepFocus gives the focus to the carousel when a part turns disabled
// while it has it: the platform would move it out of the carousel.
func (c *carouselCtl) keepFocus(cx *core.Scope, disabled func() bool) *core.Signal[bool] {
	focused := core.NewSignal(cx, false)
	cx.Effect(func() {
		if disabled() && focused.GetUntracked() {
			c.root.Focus()
		}
	})
	return focused
}

func (c *carouselCtl) focusIn()  { c.focused.Update(func(n int) int { return n + 1 }) }
func (c *carouselCtl) focusOut() { c.focused.Update(func(n int) int { return max(n-1, 0) }) }

var carouselKey = core.NewKey[*carouselCtl]("base.carousel", nil)

// Carousel is slides shown one place at a time, moved by swiping, the
// wheel or the touchpad, the previous and next controls, the pagination
// and the keyboard. The content is a native scroll view that snaps to the
// slides' starts, so a swipe follows the finger on the platform's main
// thread and rests one slide on; the carousel holds which slide shows and
// has no look. A region for assistive technology, its slides groups, and
// a Tab stop: while it, a control or a pagination item has the focus the
// arrows along Axis, swapped right to left, and Home and End move.
type Carousel struct {
	// Index is the slide shown; the app owns it, a change being reported
	// with OnChange. Without it the carousel keeps its own, from
	// DefaultIndex. A swipe's slide shows until the app has answered.
	Index        func() int
	DefaultIndex int
	// Axis is the way the slides follow each other. Read as it is made.
	Axis Axis
	// Looping goes around the ends, seamlessly while the slides span the
	// viewport and one slide more. Read as it is made.
	Looping  bool
	OnChange func(index int)
	// OnDragStart and OnDragEnd follow the user dragging the content.
	OnDragStart, OnDragEnd func()
	// OnFocus and OnBlur follow the focus coming into the carousel, a
	// control or a slide's content, and leaving it.
	OnFocus, OnBlur func()
	Class           string
	Style           core.Style
	StyleFunc       func() core.Style
	Children        []core.Node
	Label           string
	TestID          string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (a Carousel) Render(cx *core.Scope) core.Node {
	c := &carouselCtl{cx: cx, axis: a.Axis, looping: a.Looping, index: a.Index, onEvent: a, root: new(core.Ref), scroller: new(core.Ref), placed: -1}
	c.changed = core.NewSignal(cx, 0)
	c.settled = core.NewSignal(cx, 0)
	c.own = core.NewSignal(cx, a.DefaultIndex)
	c.pending = core.NewSignal(cx, -1)
	c.dragging = core.NewSignal(cx, false)
	c.focused = core.NewSignal(cx, 0)
	c.track = core.NewMemoFunc(cx, c.makeTrack, carouselTracksEqual)
	c.count = core.NewMemo(cx, func() int { c.changed.Get(); return len(c.slides) })
	if a.Looping {
		c.offset = core.NewAnimatedValue(cx, 0)
		c.period = core.NewAnimatedValue(cx, carouselStillPeriod)
		c.followTrack()
	}
	c.shown = core.NewMemo(cx, func() int {
		wanted := c.pending.Get()
		if wanted < 0 {
			if a.Index != nil {
				wanted = a.Index()
			} else {
				wanted = c.own.Get()
			}
		}
		return min(wanted, max(c.count.Get()-1, 0))
	})
	carouselKey.Provide(cx, c)
	keys := []controlKey{{"left", true, func() { c.goStep(false) }}, {"right", true, func() { c.goStep(true) }}}
	if a.Axis == Vertical {
		keys = []controlKey{{"up", true, func() { c.goStep(false) }}, {"down", true, func() { c.goStep(true) }}}
	}
	keys = append(keys, controlKey{"home", false, func() {
		if c.count.GetUntracked() > 0 {
			c.goTo(0)
		}
	}}, controlKey{"end", false, func() { c.goTo(c.last()) }})
	bound := bindWhile(func() bool { return c.focused.Get() > 0 }, core.KeyNotEditing, true, keys...)
	return core.View{Ref: c.root, Class: a.Class, Style: a.Style, StyleFunc: a.StyleFunc, Role: core.RoleRegion, Label: a.Label, TestID: a.TestID, OnLayout: a.OnLayout, Tooltip: a.Tooltip,
		Focusable: func() bool { return true }, OnFocus: c.focusIn, OnBlur: c.focusOut, OnFocusIn: a.OnFocus, OnFocusOut: a.OnBlur,
		Children: append(slices.Clone(a.Children), bound)}
}

// CarouselContent is the viewport the slides scroll in: a native scroll
// view along the carousel's axis with no scroll bar. Its class and style
// size it and lay the slides out as a track: a gap between them, padding
// that shows a peek of the slides beside the one in view.
type CarouselContent struct {
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
	TestID    string
	// OnLayout is its scroll view's.
	OnLayout func(core.Bounds)
	// OffsetValues follow its offset on the platform's main thread, for
	// slides that scale as they near the middle. A looping carousel's
	// offset runs over its periods.
	OffsetValues core.AnimatedXY
}

// Render implements core.Node.
func (a CarouselContent) Render(cx *core.Scope) core.Node {
	c := carouselKey.Use(cx)
	scroll := core.Scroll{Class: a.Class, Style: a.Style, StyleFunc: a.StyleFunc, TestID: a.TestID, OnLayout: a.OnLayout, OffsetValues: a.OffsetValues,
		HideIndicator: true, Horizontal: c == nil || c.axis == Horizontal}
	if c != nil {
		scroll.Style = core.Style{Direction: scrollDirection(c.axis)}.Merge(a.Style)
	}
	scroll.Children = a.Children
	if c == nil {
		return scroll
	}
	cx.WatchScroll(func() *core.Ref { return c.scroller }, c.measured)
	// Scrolls to the slide shown as it changes, animated, and as the
	// slides move, at once, so a resize keeps it in place.
	cx.Effect(func() {
		index := c.shown.Get()
		t := c.track.Get()
		if _, ok := t.rest(index); !ok || !t.ready() {
			return
		}
		cx.Untrack(func() {
			from := c.placed
			c.placed = index
			same := c.placedPeriod == t.period
			c.placedPeriod = t.period
			c.scrollToSlide(t, from, same, index)
		})
	})
	scroll.Ref = c.scroller
	if c.looping {
		// The slides' translations read the app's value when it has one.
		values := &scroll.OffsetValues.X
		if c.axis == Vertical {
			values = &scroll.OffsetValues.Y
		}
		if *values != nil {
			c.offset = *values
		} else {
			*values = c.offset
		}
		scroll.Children = append(append([]core.Node{c.runway(0)}, a.Children...), c.runway(1))
	}
	scroll.BehaviorFunc = func() core.ScrollBehavior {
		t := c.track.Get()
		cycles := 1
		if t.period > 0 {
			cycles = 3
		}
		var offsets []core.Points
		for k := range cycles {
			for _, s := range t.snaps {
				at := s + t.origin + t.period*float64(k-1)
				if len(offsets) == 0 || math.Abs(float64(offsets[len(offsets)-1])-at) > carouselEpsilon {
					offsets = append(offsets, core.Points(at))
				}
			}
		}
		return core.ScrollBehavior{SnapToOffsets: offsets, DisableIntervalMomentum: true}
	}
	axisOf := func(e core.ScrollEvent) float64 {
		if c.axis == Vertical {
			return float64(e.Offset.Y)
		}
		return float64(e.Offset.X)
	}
	scroll.OnScrollDragStart = func(core.ScrollEvent) { c.dragStarted() }
	scroll.OnScrollEnd = func(e core.ScrollEvent) { c.rested(axisOf(e)) }
	return scroll
}

// scrollDirection lays a scroll view's content along a.
func scrollDirection(a Axis) core.FlexDirection {
	if a == Horizontal {
		return core.Row
	}
	return core.Column
}

// runway is runway i of the content, before or after the slides: as long
// as a period less the gap while the loop is seamless, else out of the
// layout.
func (c *carouselCtl) runway(i int) core.Node {
	return core.View{PointerEvents: core.PointerEventsNone, AccessibilityElementsHidden: func() bool { return true },
		OnLayout: func(b core.Bounds) { c.placeRunway(i, b) },
		StyleFunc: func() core.Style {
			s := core.Style{FlexGrow: 0, FlexShrink: 0}
			r := c.track.Get().runway
			switch {
			case r <= 0:
				s.Display = core.DisplayNone
			case c.axis == Vertical:
				s.Height = core.Pt(core.Points(r))
			default:
				s.Width = core.Pt(core.Points(r))
			}
			return s
		}}
}

// CarouselItem is one slide, a child of the CarouselContent: a group for
// assistive technology. Its class and style size it along the axis; where
// it is laid out is a snap point. In a carousel that loops it is drawn in
// the period nearest the viewport's center, a translation of its own that
// the app's transform must not set.
type CarouselItem struct {
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
	Label     string
	TestID    string
	Tooltip   core.Node
}

// Render implements core.Node.
func (a CarouselItem) Render(cx *core.Scope) core.Node {
	v := core.View{Ref: new(core.Ref), Class: a.Class, Style: a.Style, StyleFunc: a.StyleFunc, Role: core.RoleGroup, Label: a.Label, TestID: a.TestID, Tooltip: a.Tooltip, Children: a.Children}
	c := carouselKey.Use(cx)
	if c == nil {
		return v
	}
	slide, translation := c.register(cx, v.Ref)
	if translation != nil {
		// Over what the app animates, as a style's Animated replaces another.
		translated := func(s *core.AnimatedStyle) *core.AnimatedStyle {
			var out core.AnimatedStyle
			if s != nil {
				out = *s
			}
			if c.axis == Vertical {
				out.TranslateY = translation
			} else {
				out.TranslateX = translation
			}
			return &out
		}
		v.Style.Animated = translated(v.Style.Animated)
		if a.StyleFunc != nil {
			v.StyleFunc = func() core.Style {
				s := a.StyleFunc()
				s.Animated = translated(s.Animated)
				return s
			}
		}
	}
	v.OnLayout = func(b core.Bounds) { c.place(slide, b) }
	return v
}

// CarouselPrevious is a button that moves to the previous slide, in the
// Carousel, disabled on the first unless it loops.
type CarouselPrevious struct {
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
	Label     string
	TestID    string
	OnLayout  func(core.Bounds)
	Tooltip   core.Node
}

// Render implements core.Node.
func (a CarouselPrevious) Render(cx *core.Scope) core.Node { return a.control(cx, false) }

// CarouselNext is a button that moves to the next slide, in the Carousel,
// disabled on the last unless it loops.
type CarouselNext CarouselPrevious

// Render implements core.Node.
func (a CarouselNext) Render(cx *core.Scope) core.Node { return CarouselPrevious(a).control(cx, true) }

func (a CarouselPrevious) control(cx *core.Scope, forward bool) core.Node {
	c := carouselKey.Use(cx)
	none := core.NewMemo(cx, func() bool { return c == nil || c.step(c.shown.Get(), forward) < 0 })
	v := core.View{Class: a.Class, Style: a.Style, StyleFunc: a.StyleFunc, Role: core.RoleButton, Label: a.Label, TestID: a.TestID, OnLayout: a.OnLayout, Tooltip: a.Tooltip,
		Disabled: none.Get, Children: a.Children}
	if c != nil {
		v = c.part(cx, v, none.Get, func() { c.goStep(forward) })
	}
	return v
}

// part has v take presses and report its focus to the carousel.
func (c *carouselCtl) part(cx *core.Scope, v core.View, disabled func() bool, press func()) core.View {
	focused := c.keepFocus(cx, disabled)
	v.OnPress = func(core.PressEvent) { press() }
	v.OnFocus = func() {
		focused.Set(true)
		c.focusIn()
	}
	v.OnBlur = func() {
		focused.Set(false)
		c.focusOut()
	}
	return v
}

// carouselPagination is what a pagination item finds of its pagination.
type carouselPagination struct {
	items   []*carouselDot
	changed *core.Signal[int]
}

type carouselDot struct {
	index int
	ref   *core.Ref
}

var carouselPaginationKey = core.NewKey[*carouselPagination]("base.carouselPagination", nil)

// CarouselPagination is the pagination of a Carousel: a group of
// CarouselPaginationItems, one per slide, and an optional indicator that
// follows the item of the slide shown natively, as Tabs' does.
type CarouselPagination struct {
	IndicatorClass string
	IndicatorStyle core.Style
	Class          string
	Style          core.Style
	StyleFunc      func() core.Style
	Children       []core.Node
	Label          string
	TestID         string
	OnLayout       func(core.Bounds)
	Tooltip        core.Node
}

// Render implements core.Node.
func (a CarouselPagination) Render(cx *core.Scope) core.Node {
	p := &carouselPagination{changed: core.NewSignal(cx, 0)}
	carouselPaginationKey.Provide(cx, p)
	c := carouselKey.Use(cx)
	children := a.Children
	if a.IndicatorClass != "" || !isZeroStyle(a.IndicatorStyle) {
		current := func() *core.Ref {
			p.changed.Get()
			if c == nil {
				return nil
			}
			shown := c.shown.Get()
			for _, d := range p.items {
				if d.index == shown {
					return d.ref
				}
			}
			return nil
		}
		transition := carouselIndicator
		indicator := core.View{Class: a.IndicatorClass, Style: core.Style{Position: core.Absolute}.Merge(a.IndicatorStyle), PointerEvents: core.PointerEventsNone,
			AccessibilityElementsHidden: func() bool { return true }, Follow: current, FollowTransition: &transition,
			StyleFunc: func() core.Style {
				if current() == nil {
					return core.Style{Display: core.DisplayNone}
				}
				return core.Style{}
			}}
		children = append([]core.Node{indicator}, children...)
	}
	return core.View{Class: a.Class, Style: a.Style, StyleFunc: a.StyleFunc, Role: core.RoleGroup, Label: a.Label, TestID: a.TestID, OnLayout: a.OnLayout, Tooltip: a.Tooltip,
		Children: children}
}

// CarouselPaginationItem is a button that moves to its slide, selected
// while that slide shows.
type CarouselPaginationItem struct {
	Index int
	// HitSlop is how far outside it presses count.
	HitSlop   core.Edges
	Class     string
	Style     core.Style
	StyleFunc func() core.Style
	Children  []core.Node
	Label     string
	TestID    string
	OnLayout  func(core.Bounds)
	Tooltip   core.Node
}

// Render implements core.Node.
func (a CarouselPaginationItem) Render(cx *core.Scope) core.Node {
	c := carouselKey.Use(cx)
	ref := new(core.Ref)
	if p := carouselPaginationKey.Use(cx); p != nil {
		d := &carouselDot{a.Index, ref}
		p.items = append(p.items, d)
		cx.Untrack(func() { p.changed.Update(func(n int) int { return n + 1 }) })
		cx.Cleanup(func() {
			p.items = slices.DeleteFunc(p.items, func(o *carouselDot) bool { return o == d })
			cx.Untrack(func() { p.changed.Update(func(n int) int { return n + 1 }) })
		})
	}
	selected := core.NewMemo(cx, func() bool { return c != nil && c.shown.Get() == a.Index })
	disabled := core.NewMemo(cx, func() bool { return c == nil || a.Index >= c.count.Get() })
	v := core.View{Ref: ref, Class: a.Class, Style: a.Style, StyleFunc: a.StyleFunc, Role: core.RoleButton, Label: a.Label, TestID: a.TestID, OnLayout: a.OnLayout, Tooltip: a.Tooltip,
		Disabled: disabled.Get, Selected: selected.Get, HitSlop: a.HitSlop, Children: a.Children}
	if c != nil {
		v = c.part(cx, v, disabled.Get, func() { c.goTo(a.Index) })
	}
	return v
}

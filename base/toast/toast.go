package toast

import (
	"math"
	"slices"
	"time"

	"github.com/oneo-me/nsgo/base/internal/parts"
	"github.com/oneo-me/nsgo/core"
)

const (
	// toastSwipeSlop is how far a pointer moves toward the swipe before
	// the toast follows it, so its buttons' presses stay presses.
	toastSwipeSlop = 10
	// toastFlick is how fast a release toward the swipe dismisses it short
	// of the threshold, in points per second.
	toastFlick = 500
	// toastSettle is how long a toast takes to follow a release.
	toastSettle = 200 * time.Millisecond
)

// SwipeDirection is the direction a toast is swiped away in.
type SwipeDirection uint8

const (
	SwipeRight SwipeDirection = iota
	SwipeLeft
	SwipeUp
	SwipeDown
	// SwipeOff keeps the toast from being swiped.
	SwipeOff
)

// along returns how far d goes in the direction.
func (s SwipeDirection) along(d core.Point) core.Points {
	switch s {
	case SwipeRight:
		return d.X
	case SwipeLeft:
		return -d.X
	case SwipeDown:
		return d.Y
	case SwipeUp:
		return -d.Y
	}
	return 0
}

// to returns the translation distance in the direction.
func (s SwipeDirection) to(distance core.Points) (x, y float64) {
	d := float64(distance)
	switch s {
	case SwipeRight:
		return d, 0
	case SwipeLeft:
		return -d, 0
	case SwipeDown:
		return 0, d
	case SwipeUp:
		return 0, -d
	}
	return 0, 0
}

func (s SwipeDirection) horizontal() bool { return s == SwipeRight || s == SwipeLeft }

// Toast is an unstyled toast's root, as gpui-base's: the alert role
// around the app's content. Its look follows Status, a Manager's,
// through StartingStyle in its first frame while starting (so a
// Transition in its style animates its entry from it), EndingStyle while
// ending, and SwipingStyle while a pointer swipes it; they merge over
// Style and StyleFunc, and their classes are important, over both. A finger or the mouse dragged toward
// SwipeDirection moves it once it moved 10 points, so its buttons stay
// buttons; let go past SwipeThreshold (50 when 0) or flicked, it moves on
// out and asks to be dismissed with OnDismissRequest, and short of it
// goes back. While ending it takes no presses. The screen reader reads
// Label as it shows. A Stack lays toasts out.
//
// The swipe moves a view around the root, since a backend draws a view
// with animated values without its Transition.
type Toast struct {
	// Status is its phase, Starting when nil.
	Status         func() Status
	SwipeDirection SwipeDirection
	SwipeThreshold core.Points
	StartingClass  string
	StartingStyle  core.Style
	EndingClass    string
	EndingStyle    core.Style
	SwipingClass   string
	SwipingStyle   core.Style
	Label          string
	// OnDismissRequest: the user swiped it away; dismiss it. It stays
	// where the swipe took it.
	OnDismissRequest func()
	Class            string
	Style            core.Style
	StyleFunc        func() core.Style
	Children         []core.Node
	TestID           string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (t Toast) Render(cx *core.Scope) core.Node {
	status := func() Status {
		if t.Status == nil {
			return Starting
		}
		return t.Status()
	}
	if t.Label != "" {
		cx.Announce(t.Label)
	}
	threshold := t.SwipeThreshold
	if threshold == 0 {
		threshold = 50
	}
	dir := t.SwipeDirection
	if report := leavingKey.Use(cx); report != nil {
		cx.Effect(func() { report(status() == Ending) })
	}

	// Past its first frame: the next display frame, or a frame's time
	// where none comes, whichever is first.
	entered := core.NewSignal(cx, cx.Env().ReduceMotion())
	if !entered.GetUntracked() {
		enter := func() { entered.Set(true) }
		cx.RequestAnimationFrame(func(time.Time) { enter() })
		cx.AfterFunc(parts.FirstFrame, enter)
	}

	x, y := core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0)
	swiping := core.NewSignal(cx, false)
	var size core.Size
	var down core.Point
	moved := func(e core.ResponderEvent) core.Point {
		return core.Point{X: e.PagePosition.X - down.X, Y: e.PagePosition.Y - down.Y}
	}
	settle := func(tx, ty float64) {
		c := core.TimingConfig{Duration: toastSettle, Easing: core.BezierEasing(core.EaseOut)}
		cx.StartAnimation(core.Parallel(core.Timing(x, tx, c), core.Timing(y, ty, c)), nil)
	}
	release := func(e core.ResponderEvent) {
		swiping.Set(false)
		distance := dir.along(moved(e))
		if distance >= threshold || dir.along(e.Gesture.Velocity) >= toastFlick {
			far := size.Height
			if dir.horizontal() {
				far = size.Width
			}
			settle(dir.to(max(far, distance) + 16))
			if t.OnDismissRequest != nil {
				t.OnDismissRequest()
			}
			return
		}
		settle(0, 0)
	}

	startingClass, endingClass, swipingClass := parts.Important(t.StartingClass), parts.Important(t.EndingClass), parts.Important(t.SwipingClass)
	root := core.View{Role: core.RoleAlert, Label: t.Label, TestID: t.TestID, Tooltip: t.Tooltip, Class: t.Class,
		// The state's classes win over Class and Style, as its styles do
		// and as CSS's state selectors do by their specificity.
		ClassFunc: func() string {
			switch st := status(); {
			case st == Ending:
				return endingClass
			case swiping.Get():
				return swipingClass
			case st == Starting && !entered.Get():
				return startingClass
			}
			return ""
		},
		Style: t.Style,
		StyleFunc: func() core.Style {
			var s core.Style
			if t.StyleFunc != nil {
				s = t.StyleFunc()
			}
			switch st := status(); {
			case st == Ending:
				s = s.Merge(t.EndingStyle)
			case swiping.Get():
				s = s.Merge(t.SwipingStyle)
			case st == Starting && !entered.Get():
				s = s.Merge(t.StartingStyle)
			}
			return s
		},
		OnLayout: func(b core.Bounds) {
			size = b.Size
			if t.OnLayout != nil {
				t.OnLayout(b)
			}
		},
		OnTouch: func(e core.TouchEvent) {
			if e.Phase == core.TouchStart && len(e.Touches) > 0 {
				down = e.Touches[0].PagePosition
			}
		},
		Responder: &core.Responder{
			Claims: []core.TouchClaim{{OnMove: true, Ask: func(e core.ResponderEvent) bool {
				if dir == SwipeOff || core.Untracked(cx, status) == Ending {
					return false
				}
				d := e.Gesture.Delta
				main, cross := d.Y, d.X
				if dir.horizontal() {
					main, cross = d.X, d.Y
				}
				return dir.along(d) >= toastSwipeSlop && abs(main) > abs(cross)
			}}},
			Keep:    true,
			OnGrant: func(core.ResponderEvent) { swiping.Set(true) },
			OnMove: func(e core.ResponderEvent) {
				// Toward the direction only.
				tx, ty := dir.to(max(dir.along(moved(e)), 0))
				x.Set(tx)
				y.Set(ty)
			},
			OnRelease:   release,
			OnTerminate: func(core.ResponderEvent) { swiping.Set(false); settle(0, 0) },
		},
		// What is going takes no more presses.
		Children: append(slices.Clip(t.Children), core.Show{When: func() bool { return status() == Ending }, Then: func(*core.Scope) core.Node {
			return core.View{Style: parts.Fill, OnPress: func(core.PressEvent) {}, NoFocus: true}
		}}),
	}
	return core.View{Style: core.Style{Animated: &core.AnimatedStyle{TranslateX: x, TranslateY: y}}, Children: []core.Node{root}}
}

func abs(p core.Points) core.Points { return max(p, -p) }

// leavingKey is how a Toast tells the Stack around it that it is ending.
var leavingKey = core.NewKey[func(bool)]("toast.leaving", nil)

// Anchor is the edge of the viewport a Stack lays out from: the
// newest toast is nearest it.
type Anchor uint8

const (
	AnchorTopRight Anchor = iota
	AnchorTopLeft
	AnchorTopCenter
	AnchorBottomLeft
	AnchorBottomCenter
	AnchorBottomRight
	AnchorLeftCenter
	AnchorRightCenter
)

// bottom reports whether the stack grows upward.
func (a Anchor) bottom() bool {
	return a == AnchorBottomLeft || a == AnchorBottomCenter || a == AnchorBottomRight
}

// center reports whether the stack is centered on a side.
func (a Anchor) center() bool { return a == AnchorLeftCenter || a == AnchorRightCenter }

// Stack lays toasts out, as gpui-base's: Toasts are its items, oldest
// first, each rendered by Toast and matched by Key as core.Each matches
// rows. From Placement's edge, collapsed toasts overlap CollapsedPeek
// apart, each further one narrower by CollapsedScaleStep, and only
// CollapsedVisible show; expanded, while the pointer or a finger is on
// it, they are a list ExpandedGap apart, so the app pauses its manager
// (OnExpandedChange). Reverse turns the stack around: the newest toast is
// the farthest from the edge and the oldest the nearest, as a list
// growing away from it. Arrivals, departures and expansion move them with
// critically damped springs of the motion's Duration; a Toast that is
// ending stays where it shows and takes no room, so the others close up
// as it leaves. While it holds toasts it hears Escape and back (OnCancel:
// dismiss the newest), unless NoCloseOnEscape. It has no look: its style
// gives its width, and the app places it, in an Overlay at that edge
// inside the safe area, centered at a center anchor.
type Stack[T any, K comparable] struct {
	Toasts           func() []T
	Key              func(T) K
	Toast            func(cx *core.Scope, item func() T) core.Node
	Motion           Motion
	Placement        Anchor
	Reverse          bool
	NoCloseOnEscape  bool
	OnExpandedChange func(expanded bool)
	OnCancel         func()
	Class            string
	Style            core.Style
	StyleFunc        func() core.Style
	TestID           string
}

// toastHeights is the stack's height collapsed and expanded, for items of
// heights, oldest first.
func toastHeights(heights []core.Points, gap, peek core.Points) (collapsed, expanded core.Points) {
	n := len(heights)
	for i, h := range heights {
		expanded += h
		collapsed = max(collapsed, h+peek*core.Points(n-1-i))
	}
	if n > 0 {
		expanded += gap * core.Points(n-1)
		collapsed = max(collapsed, heights[n-1]+peek*core.Points(n-1))
	}
	return collapsed, expanded
}

// toastDistance is how far item i of heights is from the anchored edge,
// collapsed and expanded.
func toastDistance(heights []core.Points, i int, gap, peek core.Points) (collapsed, expanded core.Points) {
	rank := core.Points(max(len(heights)-1-i, 0))
	var newer core.Points
	for _, h := range heights[min(i+1, len(heights)):] {
		newer += h
	}
	return peek * rank, newer + gap*rank
}

// toastSpring is a critically damped spring whose response is d: no
// overshoot, as a height or an opacity past its target reads as a glitch.
func toastSpring(d time.Duration) core.SpringConfig {
	response := max(d.Seconds(), 0.01)
	k := math.Pow(2*math.Pi/response, 2)
	return core.SpringConfig{Stiffness: k, Damping: 2 * math.Sqrt(k), Mass: 1}
}

// Render implements core.Node.
func (s Stack[T, K]) Render(cx *core.Scope) core.Node {
	m := s.Motion.resolved()
	hovered, touched := core.NewSignal(cx, false), core.NewSignal(cx, false)
	expanded := core.NewMemo(cx, func() bool { return hovered.Get() || touched.Get() })
	was := false
	cx.Effect(func() {
		if now := expanded.Get(); now != was {
			was = now
			if s.OnExpandedChange != nil {
				cx.Untrack(func() { s.OnExpandedChange(now) })
			}
		}
	})
	measured := core.NewSignalFunc(cx, map[K]core.Points{}, func(a, b map[K]core.Points) bool { return false })
	leaving := core.NewSignalFunc(cx, map[K]bool{}, func(a, b map[K]bool) bool { return false })
	// heights are the measured heights of the items taking room, oldest
	// first, and index is where key is among them; one not measured yet
	// counts as nothing.
	heights := func() []core.Points {
		hs, gone := measured.Get(), leaving.Get()
		var out []core.Points
		for _, it := range s.Toasts() {
			if k := s.Key(it); !gone[k] {
				out = append(out, hs[k])
			}
		}
		return out
	}
	index := func(key K) int {
		gone := leaving.Get()
		i := 0
		for _, it := range s.Toasts() {
			switch k := s.Key(it); {
			case k == key:
				return i
			case !gone[k]:
				i++
			}
		}
		return -1
	}
	bottom := s.Placement.bottom()

	item := func(cx *core.Scope, it func() T, _ func() int) core.Node {
		key := s.Key(core.Untracked(cx, it))
		leavingKey.Provide(cx, func(on bool) {
			if leaving.GetUntracked()[key] != on {
				leaving.Update(func(gone map[K]bool) map[K]bool { gone[key] = on; return gone })
			}
		})
		y, sx, op := core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 1), core.NewAnimatedValue(cx, 1)
		// offset is its distance from the line that stays put as the stack's
		// height changes, in the layout: the anchored edge, or the stack's
		// middle at a center anchor. Hit tests go by frames, not transforms,
		// so each toast takes its presses where it shows. y plays its way
		// there, less laid, where the layout put it.
		offset := core.NewSignal(cx, core.Points(0))
		laid := core.NewAnimatedValue(cx, 0)
		placed := false
		cx.Effect(func() {
			if leaving.Get()[key] {
				return
			}
			open := expanded.Get()
			hs := heights()
			i := index(key)
			if i < 0 || i >= len(hs) {
				return
			}
			collapsed, full := toastDistance(hs, i, m.ExpandedGap, m.CollapsedPeek)
			height, fullHeight := toastHeights(hs, m.ExpandedGap, m.CollapsedPeek)
			rank := max(len(hs)-1-i, 0)
			d := collapsed
			if open {
				d, height = full, fullHeight
			}
			if s.Reverse {
				d = height - d - hs[i]
			}
			if s.Placement.center() {
				d -= height / 2
			}
			scale := 1.0
			if !open {
				scale = 1 - float64(m.CollapsedScaleStep)*float64(min(rank, m.CollapsedVisible-1))
			}
			opacity := 0.0
			if open || rank < m.CollapsedVisible {
				opacity = 1
			}
			ty := float64(d)
			if bottom {
				ty = -ty
			}
			moved := placed && !core.Untracked(cx, cx.Env().ReduceMotion)
			placed = hs[i] > 0
			if !moved {
				y.Set(ty)
				sx.Set(scale)
				op.Set(opacity)
			}
			offset.Set(d)
			laid.Set(-ty)
			if moved {
				c := toastSpring(m.Duration)
				cx.StartAnimation(core.Parallel(core.Spring(y, ty, c), core.Spring(sx, scale, c), core.Spring(op, opacity, c)), nil)
			}
		})
		cx.Cleanup(func() {
			measured.Update(func(hs map[K]core.Points) map[K]core.Points { delete(hs, key); return hs })
			leaving.Update(func(gone map[K]bool) map[K]bool { delete(gone, key); return gone })
		})
		place := core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0),
			Animated: &core.AnimatedStyle{TranslateY: core.Add(y, laid), ScaleX: sx, Opacity: op}}
		return core.View{Style: place, PointerEvents: core.PointerEventsBoxNone,
			StyleFunc: func() core.Style {
				switch {
				case s.Placement.center():
					return core.Style{Top: core.Pct(50), Margin: core.Edges{Top: offset.Get()}}
				case bottom:
					return core.Style{Bottom: core.Pt(offset.Get())}
				}
				return core.Style{Top: core.Pt(offset.Get())}
			},
			OnLayout: func(b core.Bounds) {
				if h := b.Size.Height; measured.GetUntracked()[key] != h {
					measured.Update(func(hs map[K]core.Points) map[K]core.Points { hs[key] = h; return hs })
				}
			},
			Children: []core.Node{s.Toast(cx, it)}}
	}

	return core.View{TestID: s.TestID, Class: s.Class, Style: s.Style,
		StyleFunc: func() core.Style {
			var st core.Style
			if s.StyleFunc != nil {
				st = s.StyleFunc()
			}
			collapsed, full := toastHeights(heights(), m.ExpandedGap, m.CollapsedPeek)
			h := collapsed
			if expanded.Get() {
				h = full
			}
			return st.Merge(core.Style{Position: core.Relative, Height: core.Pt(h)})
		},
		OnMouseEnter: func(core.MouseEvent) { hovered.Set(true) },
		OnMouseLeave: func(core.MouseEvent) { hovered.Set(false) },
		// Observed, never claimed: the toasts' presses and swipes go on.
		OnTouch: func(e core.TouchEvent) {
			switch e.Phase {
			case core.TouchStart:
				touched.Set(true)
			case core.TouchEnd, core.TouchCancel:
				touched.Set(false)
			}
		},
		Children: []core.Node{
			core.Each(s.Toasts, s.Key, item),
			// Escape and back, only while it holds toasts: an overlay of no
			// size, so nothing else loses them to an empty stack.
			core.Show{When: func() bool { return !s.NoCloseOnEscape && len(s.Toasts()) > 0 }, Then: func(*core.Scope) core.Node {
				return core.Overlay{Style: core.Style{Width: core.Pt(0), Height: core.Pt(0)}, PointerEvents: core.PointerEventsNone,
					OnCancel: func() {
						if s.OnCancel != nil {
							s.OnCancel()
						}
					}}
			}},
		},
	}
}

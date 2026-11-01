package base

import (
	"math"
	"strings"
	"time"

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
	// toastFirstFrame is how long a toast waits for its first display
	// frame before it counts as past it without one.
	toastFirstFrame = 50 * time.Millisecond
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

// important returns class with every utility important (opacity-0 is
// opacity-0!), as NSH's toast root does.
func important(class string) string {
	tokens := strings.Fields(class)
	for i, t := range tokens {
		if u := t[strings.LastIndexByte(t, ':')+1:]; !strings.HasSuffix(t, "!") && !strings.HasPrefix(u, "!") {
			tokens[i] = t + "!"
		}
	}
	return strings.Join(tokens, " ")
}

// Toast is an unstyled toast's root, as gpui-base's: the alert role
// around the app's content. Its look follows Status, a ToastManager's,
// through StartingStyle in its first frame while starting (so a
// Transition in its style animates its entry from it), EndingStyle while
// ending, and SwipingStyle while a pointer swipes it; they merge over
// Style and StyleFunc, and their classes are important, over both. A finger or the mouse dragged toward
// SwipeDirection moves it once it moved 10 points, so its buttons stay
// buttons; let go past SwipeThreshold (50 when 0) or flicked, it moves on
// out and asks to be dismissed with OnDismissRequest, and short of it
// goes back. The screen reader reads Label as it shows. A ToastStack lays
// toasts out.
type Toast struct {
	// Status is its phase, ToastStarting when nil.
	Status         func() ToastStatus
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
	status := func() ToastStatus {
		if t.Status == nil {
			return ToastStarting
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

	// Past its first frame: the next display frame, or a frame's time
	// where none comes, whichever is first.
	entered := core.NewSignal(cx, cx.Env().ReduceMotion())
	if !entered.GetUntracked() {
		enter := func() { entered.Set(true) }
		cx.RequestAnimationFrame(func(time.Time) { enter() })
		cx.AfterFunc(toastFirstFrame, enter)
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

	startingClass, endingClass, swipingClass := important(t.StartingClass), important(t.EndingClass), important(t.SwipingClass)
	return core.View{Role: core.RoleAlert, Label: t.Label, TestID: t.TestID, Tooltip: t.Tooltip, Class: t.Class,
		// The state's classes win over Class and Style, as its styles do
		// and as CSS's state selectors do by their specificity.
		ClassFunc: func() string {
			switch st := status(); {
			case st == ToastEnding:
				return endingClass
			case swiping.Get():
				return swipingClass
			case st == ToastStarting && !entered.Get():
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
			case st == ToastEnding:
				s = s.Merge(t.EndingStyle)
			case swiping.Get():
				s = s.Merge(t.SwipingStyle)
			case st == ToastStarting && !entered.Get():
				s = s.Merge(t.StartingStyle)
			}
			return s.Merge(core.Style{Animated: &core.AnimatedStyle{TranslateX: x, TranslateY: y}})
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
				if dir == SwipeOff || core.Untracked(cx, status) == ToastEnding {
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
		Children: t.Children,
	}
}

func abs(p core.Points) core.Points { return max(p, -p) }

// Anchor is the edge of the viewport a ToastStack lays out from: the
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

// ToastStack lays toasts out, as gpui-base's: Toasts are its items, oldest
// first, each rendered by Toast and matched by Key as core.Each matches
// rows. From Placement's edge, collapsed toasts overlap CollapsedPeek
// apart, each further one narrower by CollapsedScaleStep, and only
// CollapsedVisible show; expanded, while the pointer or a finger is on
// it, they are a list ExpandedGap apart, so the app pauses its manager
// (OnExpandedChange). Arrivals, departures and expansion move them with
// critically damped springs of the motion's Duration. While it holds
// toasts it hears Escape and back (OnCancel: dismiss the newest), unless
// NoCloseOnEscape. It has no look: its style gives its width, and the app
// places it, in an Overlay at that edge inside the safe area.
type ToastStack[T any, K comparable] struct {
	Toasts           func() []T
	Key              func(T) K
	Toast            func(cx *core.Scope, item func() T) core.Node
	Motion           ToastMotion
	Placement        Anchor
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
func (s ToastStack[T, K]) Render(cx *core.Scope) core.Node {
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
	// heights are the items' measured heights, oldest first; one not
	// measured yet counts as nothing.
	heights := func() []core.Points {
		items, hs := s.Toasts(), measured.Get()
		out := make([]core.Points, len(items))
		for i, it := range items {
			out[i] = hs[s.Key(it)]
		}
		return out
	}
	bottom := s.Placement.bottom()

	item := func(cx *core.Scope, it func() T, index func() int) core.Node {
		key := s.Key(core.Untracked(cx, it))
		y, sx, op := core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 1), core.NewAnimatedValue(cx, 1)
		first := true
		cx.Effect(func() {
			open := expanded.Get()
			hs := heights()
			i := index()
			collapsed, full := toastDistance(hs, i, m.ExpandedGap, m.CollapsedPeek)
			rank := max(len(hs)-1-i, 0)
			d := collapsed
			if open {
				d = full
			}
			ty := float64(d)
			if bottom {
				ty = -ty
			}
			scale := 1.0
			if !open {
				scale = 1 - float64(m.CollapsedScaleStep)*float64(min(rank, m.CollapsedVisible-1))
			}
			opacity := 0.0
			if open || rank < m.CollapsedVisible {
				opacity = 1
			}
			if first || core.Untracked(cx, cx.Env().ReduceMotion) {
				first = false
				y.Set(ty)
				sx.Set(scale)
				op.Set(opacity)
				return
			}
			c := toastSpring(m.Duration)
			cx.StartAnimation(core.Parallel(core.Spring(y, ty, c), core.Spring(sx, scale, c), core.Spring(op, opacity, c)), nil)
		})
		cx.Cleanup(func() {
			measured.Update(func(hs map[K]core.Points) map[K]core.Points { delete(hs, key); return hs })
		})
		place := core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), Top: core.Pt(0),
			Animated: &core.AnimatedStyle{TranslateY: y, ScaleX: sx, Opacity: op}}
		if bottom {
			place.Top, place.Bottom = core.Length{}, core.Pt(0)
		}
		return core.View{Style: place, PointerEvents: core.PointerEventsBoxNone,
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
		OnHoverIn:  func() { hovered.Set(true) },
		OnHoverOut: func() { hovered.Set(false) },
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

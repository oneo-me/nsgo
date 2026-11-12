package main

import (
	"fmt"
	"math"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

const (
	cardWidth  = 300
	cardHeight = 440
	cardRadius = 6
	// radius is how far behind a card the ring's axis stands: far enough
	// for six cards to stand side by side around it.
	radius = 330
	// step is the angle between neighboring cards, in degrees.
	step = 360.0 / 6
	// dragRate is how many degrees the ring turns per point dragged.
	dragRate = 0.3
	// flingTime is how far ahead a released drag's speed carries the
	// ring before it picks the card to rest on, in seconds.
	flingTime = 0.2
	// perspective is how far the viewer stands from the stage, and
	// stageLift how far the stage sits above the window's middle; the
	// dial's shader draws the floor from both.
	perspective = 1500
	stageLift   = 16
	// introTurn is where the ring starts at launch, sweeping in to the
	// first card.
	introTurn = 100.0
)

var (
	// ringSpring settles the ring on a card: just under critical damping,
	// so it lands with a trace of give and no bounce.
	ringSpring = core.SpringConfig{Stiffness: 150, Damping: 23}
	// introSpring is the launch's slower sweep.
	introSpring = core.SpringConfig{Stiffness: 38, Damping: 12.5}
)

// ring is the ring's angle and the card in front. The angle is turn, where
// the ring rests or springs, and drag, which a drag on the space moves on
// the main thread; the core knows only where turn is headed.
type ring struct {
	cx    *core.Scope
	turn  *core.AnimatedValue
	drag  *core.AnimatedValue
	angle core.AnimatedNode
	// phase is the front card's place on the ring, in 0 to 360: what the
	// odometer, the index and the dial follow.
	phase core.AnimatedNode
	// intro fades the ring in at launch.
	intro *core.AnimatedValue
	front *core.Signal[int]
	// target is where turn rests, and base where it was as a drag began.
	target, base float64
	width        core.Points
	reduce       bool
}

func newRing(cx *core.Scope) *ring {
	reduce := cx.Env().ReduceMotion()
	start, shown := introTurn, 0.0
	if reduce {
		start, shown = 0, 1
	}
	r := &ring{cx: cx, turn: core.NewAnimatedValue(cx, start), drag: core.NewAnimatedValue(cx, 0),
		intro: core.NewAnimatedValue(cx, shown), front: core.NewSignal(cx, 0), reduce: reduce}
	r.angle = core.Add(r.turn, core.Mul(r.drag, core.Constant(dragRate)))
	r.phase = core.Mod(core.Mul(r.angle, core.Constant(-1)), core.Constant(360))
	if !reduce {
		cx.StartAnimation(core.Parallel(
			core.Timing(r.intro, 1, core.TimingConfig{Duration: 1100 * time.Millisecond, Easing: outQuint, Delay: 100 * time.Millisecond}),
			core.Sequence(core.Delay(100*time.Millisecond), core.Spring(r.turn, 0, introSpring)),
		), nil)
	}
	return r
}

// facing is the card in front of the ring at angle a.
func facing(a float64) int {
	n := len(instruments)
	return ((int(math.Round(-a/step)) % n) + n) % n
}

// apart is how many cards lie between card i and the front one, either
// way round.
func (r *ring) apart(i int) int {
	n := len(instruments)
	d := ((i-r.front.Get())%n + n) % n
	return min(d, n-d)
}

// settle springs the ring to angle a from where it is, with a speed in
// degrees per second.
func (r *ring) settle(a, velocity float64) {
	r.target = a
	r.front.Set(facing(a))
	if r.reduce {
		r.turn.Set(a)
		return
	}
	c := ringSpring
	c.Velocity = velocity
	r.cx.StartAnimation(core.Spring(r.turn, a, c), nil)
}

// grab holds the ring where it is as a gesture takes it. It may be caught
// mid spring: the gesture goes on from there once the platform says where
// that is.
func (r *ring) grab() {
	r.base = r.target
	r.turn.Stop()
	r.turn.Read(r.cx, func(a float64) { r.base = a })
}

// fling lets the ring go at angle a with a speed in degrees per second:
// the speed carries it on for flingTime, and it settles on the card
// nearest there.
func (r *ring) fling(a, speed float64) {
	r.settle(math.Round((a+speed*flingTime)/step)*step, speed)
}

func (r *ring) next() { r.settle(r.target-step, 0) }
func (r *ring) prev() { r.settle(r.target+step, 0) }

// turnTo brings card i to the front the shorter way round.
func (r *ring) turnTo(i int) {
	d := math.Mod(-float64(i)*step-r.target, 360)
	if d > 180 {
		d -= 360
	} else if d < -180 {
		d += 360
	}
	r.settle(r.target+d, 0)
}

// space is the layer behind the ring that drags turn it, and a press
// beside the front card turns that side's card to the front.
func (r *ring) space(style core.Style) core.Node {
	return core.View{TestID: "space", NoFocus: true, Style: style,
		PanValues:     core.AnimatedXY{X: r.drag, Y: core.NewAnimatedValue(r.cx, 0)},
		PanActivation: core.PanActivation{ActiveOffsetX: 4},
		OnLayout:      func(b core.Bounds) { r.width = b.Size.Width },
		OnPress: func(e core.PressEvent) {
			switch mid := r.width / 2; {
			case e.Position.X < mid-cardWidth/2:
				r.prev()
			case e.Position.X > mid+cardWidth/2:
				r.next()
			}
		},
		OnPan: func(e core.PanEvent) {
			switch e.Phase {
			case core.GestureBegan:
				r.grab()
			case core.GestureChanged:
				r.front.Set(facing(r.base + float64(e.Translation.X)*dragRate))
			case core.GestureEnded, core.GestureCancelled:
				at := r.base + float64(e.Values.X)*dragRate
				// The drag folds into turn, in the same batch.
				r.turn.Set(at)
				r.drag.Set(0)
				r.fling(at, float64(e.Velocity.X)*dragRate)
			}
		},
	}
}

// slot stands card i on the ring, built with its angle from the front:
// turned around the ring's axis, receding into the paper as it turns away
// and hidden from behind, while the front card lifts toward the viewer on
// a deeper shadow. Only the front card's controls take the pointer, its
// frame alone being where it shows: the others' frames lie under it, and
// they keep the pointer from their controls.
func (r *ring) slot(i int, build func(rel core.AnimatedNode) core.Node) core.Node {
	// The card's angle from the front, in -180 to 180.
	rel := core.Sub(core.Mod(core.Add(r.angle, core.Constant(float64(i)*step+180)), core.Constant(360)), core.Constant(180))
	by := func(in, out []float64) core.AnimatedNode {
		return core.Interpolate(rel, core.Interpolation{Input: in, Output: out})
	}
	fade := by([]float64{-150, -100, -60, 0, 60, 100, 150}, []float64{0, 0.3, 0.72, 1, 0.72, 0.3, 0})
	near := []float64{-60, 0, 60}
	animated := &core.AnimatedStyle{
		RotateY:       rel,
		Opacity:       core.Mul(fade, r.intro),
		TranslateY:    by(near, []float64{0, -10, 0}),
		ShadowRadius:  by(near, []float64{8, 34, 8}),
		ShadowOpacity: by(near, []float64{0.05, 0.2, 0.05}),
	}
	child := build(rel)
	return core.View{TestID: fmt.Sprintf("card-%d", i), StyleFunc: func() core.Style {
		apart := r.apart(i)
		s := core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Width: core.Pt(cardWidth), Height: core.Pt(cardHeight),
			CornerRadius: cardRadius, ShadowColor: ink, ShadowOffset: core.Point{Y: 22},
			TransformOriginZ: -radius, BackfaceHidden: true, ZIndex: len(instruments) - apart, Animated: animated}
		if apart != 0 {
			s.PointerEvents = core.PointerEventsBoxOnly
		}
		return s
	}, Children: []core.Node{child}}
}

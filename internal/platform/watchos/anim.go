//go:build ios

package watchos

import (
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// What moves on its own runs here on the host's frames, as the Linux
// backend runs it on GTK's: style transitions, keyframes, and a commit's
// layout animation moving frames and bringing views in. The host only
// shows each frame's look.

// allProperties are what a transition with no properties named moves.
const allProperties = driver.TransitionOpacity | driver.TransitionBackground | driver.TransitionTransform |
	driver.TransitionBorder | driver.TransitionShadow

// transition is a view's style moving from one ViewStyle to the next.
type transition struct {
	from, to driver.ViewStyle
	start    time.Time // after the delay
	duration time.Duration
	curve    driver.Curve
	props    driver.TransitionProperty
	shown    driver.ViewStyle
}

// motion is a view's part of a layout animation: its frame moving, and
// it coming in.
type motion struct {
	move      *driver.LayoutTransition
	moveStart time.Time
	from      driver.Bounds
	shown     driver.Bounds

	fade      *driver.LayoutTransition
	fadeStart time.Time
	part      float64 // how much of the view shows, 0 to 1
}

// layoutBatch is what applying a commit with a LayoutAnimation keeps.
type layoutBatch struct {
	anim    *driver.LayoutAnimation
	created map[driver.NodeID]bool
}

// keyframeStart is when each view's keyframe animation began.
var keyframeStarts = map[*view]time.Time{}

// setStyle applies v's new style, moving to it when it has a transition.
func (s *surface) setStyle(v *view, st *driver.ViewStyle) {
	from := v.shownStyle()
	if v.animated != nil {
		from = v.style
		if v.trans != nil {
			from = v.trans.shown
		}
	}
	styled := v.styled || v.born != s.batch
	if st.Animation != v.style.Animation || !styled {
		if st.Animation.Keyframes != 0 {
			keyframeStarts[v] = time.Now()
			s.motions[v] = true
		} else {
			delete(keyframeStarts, v)
		}
	}
	v.style, v.styled, v.trans = *st, true, nil
	s.touched(v)
	t := st.Transition
	if !styled || t.Duration <= 0 || s.env.ReduceMotion {
		return
	}
	props := t.Properties
	if props == 0 {
		props = allProperties
	}
	first, last := driver.MixStyles(from, *st, 0, props), driver.MixStyles(from, *st, 1, props)
	if first == last {
		return
	}
	v.trans = &transition{from: from, to: *st, start: time.Now().Add(t.Delay), duration: t.Duration,
		curve: t.Easing, props: props, shown: first}
	s.motions[v] = true
}

// setFrame applies a frame to v, moving to it with the batch's layout
// animation when v was there before.
func (s *surface) setFrame(v *view, frame driver.Bounds, lb *layoutBatch) {
	old := v.shownFrame()
	v.frame = frame
	s.touched(v)
	if v.kind == driver.KindInput {
		s.relayoutField(v)
	}
	if v.image != nil {
		v.image.Layout(apple.CGSize{Width: float64(frame.Size.Width), Height: float64(frame.Size.Height)})
	}
	if lb == nil || lb.anim.Update == nil || lb.created[v.id] || v.kind == driver.KindModal || old == frame {
		if m := v.moving; m != nil {
			m.move = nil
			s.dropMotion(v)
		}
		return
	}
	m := s.motionOf(v)
	m.move, m.moveStart, m.from, m.shown = lb.anim.Update, time.Now(), old, old
}

// appear brings v in with the batch's Create when the batch made it and
// put it into a view that was there before.
func (s *surface) appear(v *view, lb *layoutBatch) {
	if lb.anim.Create == nil || v.parent == nil || lb.created[v.parent.id] || v.kind == driver.KindModal {
		return
	}
	m := s.motionOf(v)
	m.fade, m.fadeStart, m.part = lb.anim.Create, time.Now(), 0
	s.touched(v)
}

func (s *surface) motionOf(v *view) *motion {
	if v.moving == nil {
		v.moving = &motion{part: 1, shown: v.frame}
	}
	s.motions[v] = true
	s.tick(true)
	return v.moving
}

func (s *surface) dropMotion(v *view) {
	if m := v.moving; m != nil && m.move == nil && m.fade == nil {
		v.moving = nil
	}
}

// stepMotions moves what animates on its own to now.
func (s *surface) stepMotions() {
	now := time.Now()
	for v := range s.motions {
		if v.deleted {
			delete(s.motions, v)
			continue
		}
		keep := false
		if t := v.trans; t != nil {
			if now.Before(t.start) {
				keep = true
			} else if e := now.Sub(t.start); e >= t.duration {
				v.trans = nil
			} else {
				t.shown = driver.MixStyles(t.from, t.to, t.curve.At(float64(e)/float64(t.duration)), t.props)
				keep = true
			}
			s.touched(v)
		}
		if start, ok := keyframeStarts[v]; ok {
			if _, running := v.style.Animation.At(now.Sub(start)); running {
				keep = true
			}
			s.touched(v)
		}
		if m := v.moving; m != nil {
			if m.move != nil {
				p, done := m.move.At(now.Sub(m.moveStart))
				if done {
					m.move = nil
				} else {
					m.shown = driver.LerpBounds(m.from, v.frame, p)
				}
			}
			if m.fade != nil {
				p, done := m.fade.At(now.Sub(m.fadeStart))
				m.part = p
				if done {
					m.fade = nil
				}
			}
			s.dropMotion(v)
			keep = keep || v.moving != nil
			s.touched(v)
		}
		if !keep {
			delete(s.motions, v)
		}
	}
}

// keyframe is v's keyframe now, and whether it has keyframes.
func (v *view) keyframe() (driver.Keyframe, bool) {
	start, ok := keyframeStarts[v]
	if !ok || v.style.Animation.Keyframes == 0 {
		return driver.Keyframe{Opacity: 1, Scale: 1}, false
	}
	k, _ := v.style.Animation.At(time.Since(start))
	return k, true
}

// layoutPart is what a view coming in multiplies its opacity by, and
// scales it by.
func (v *view) layoutPart() (opacity, scale float64) {
	m := v.moving
	if m == nil || m.fade == nil {
		return 1, 1
	}
	if m.fade.Property == driver.LayoutScale {
		return 1, max(m.part, 1e-3)
	}
	return min(max(m.part, 0), 1), 1
}

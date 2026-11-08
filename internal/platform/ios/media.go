//go:build ios

package ios

import (
	"log"
	"math"
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// The surface's frame clock, a CADisplayLink, steps the native animation
// graph and the layout animations once a display frame while either
// runs.

var selTick = sel("nsgoTick:")

// motion is a view's part of a layout animation: its frame moving, or
// the view appearing.
type motion struct {
	from, to driver.Bounds
	frame    *driver.LayoutTransition
	appear   *driver.LayoutTransition
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

func (s *surface) clock() time.Duration { return time.Since(s.epoch) }

// layoutStart is when a layout change on tr starts: now, or for the
// keyboard's curve when the keyboard started to move, however late the
// batch arrived.
func (s *surface) layoutStart(tr *driver.LayoutTransition) time.Duration {
	now := s.clock()
	if tr.Curve == driver.LayoutKeyboard && s.keyboardStart > 0 && now-s.keyboardStart < tr.End() {
		return s.keyboardStart
	}
	return now
}

func (s *surface) startTicking() {
	if s.ticking {
		s.setPace()
		return
	}
	s.ticking = true
	s.pace = s.wantedPace()
	if s.link == 0 {
		// A link needs the view in a window: without one, the next mount
		// asks again.
		link := objc.ID(objc.GetClass("CADisplayLink")).Send(sel("displayLinkWithTarget:selector:"), s.native, selTick)
		if link == 0 {
			s.ticking = false
			return
		}
		s.link = link.Send(sel("retain"))
		s.link.Send(sel("setPreferredFramesPerSecond:"), s.framesPerSecond())
		s.link.Send(sel("addToRunLoop:forMode:"), objc.ID(objc.GetClass("NSRunLoop")).Send(sel("mainRunLoop")),
			apple.CommonModes())
		return
	}
	s.link.Send(sel("setPreferredFramesPerSecond:"), s.framesPerSecond())
	s.link.Send(sel("setPaused:"), false)
}

// framesPerSecond is what the link asks for: the pace, or the screen's
// highest rate, as a link left at its default runs at 60 Hz on a
// ProMotion iPhone.
func (s *surface) framesPerSecond() int {
	if s.pace > 0 {
		return max(int(math.Ceil(s.pace)), 1)
	}
	screen := objc.ID(objc.GetClass("UIScreen")).Send(sel("mainScreen"))
	return max(objc.Send[int](screen, sel("maximumFramesPerSecond")), 0)
}

// wantedPace is the rate the frame clock need run at, as macOS's: the
// animations' own when they are all it steps, else the screen's highest.
// Batches apply as they land, so none waits for a slowed frame.
func (s *surface) wantedPace() float64 {
	if len(s.motions) > 0 || len(s.ghosts) > 0 || s.frameWanted || s.selecting != nil || s.perf != nil {
		return 0
	}
	return s.animator.FrameRate()
}

// setPace moves the running link to the rate wanted now.
func (s *surface) setPace() {
	if pace := s.wantedPace(); pace != s.pace {
		s.pace = pace
		if s.link != 0 {
			s.link.Send(sel("setPreferredFramesPerSecond:"), s.framesPerSecond())
		}
	}
}

func (s *surface) stopTicking() {
	if s.ticking {
		s.ticking = false
		s.link.Send(sel("setPaused:"), true)
	}
}

func surfaceTick(self objc.ID, _ objc.SEL, _ objc.ID) {
	if v := views[self]; v != nil {
		v.surface.tick()
	}
}

func (s *surface) tick() {
	if s.perf != nil {
		s.perf.Vsync(time.Now(), time.Duration(objc.Send[float64](s.link, sel("duration"))*float64(time.Second)))
		defer s.measuring(s.animator.Running() || len(s.motions) > 0 || len(s.ghosts) > 0 || s.frameWanted)()
	}
	now := s.clock()
	if s.animator.Running() {
		s.animator.Step(now)
	}
	apple.WithoutAnimation(func() {
		s.stepMotions(now)
		s.applyAnimated()
	})
	if s.animator.Running() || len(s.motions) > 0 {
		s.staleEffects()
	}
	s.drawShaders()
	s.startImages()
	if s.frameWanted {
		s.frameWanted = false
		if s.engine != nil {
			s.engine.Frame()
		}
	}
	// A monitor closes a frame at every refresh.
	if !s.animator.Running() && len(s.motions) == 0 && len(s.ghosts) == 0 && !s.frameWanted && s.perf == nil {
		s.stopTicking()
		return
	}
	s.setPace()
}

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
	v.followScale(vs)
	// A view created by an earlier batch shows the default style.
	var shown *driver.ViewStyle
	if v.styled || v.born != v.surface.batch {
		shown = &v.drawn
	}
	apple.RestyleLayer(v.native.Send(selLayer), shown, vs, 0)
	v.drawn, v.styled = *vs, true
}

func (v *view) animatesStyle() bool {
	if v.animated.Set != 0 {
		return true
	}
	m := v.surface.motions[v]
	return m != nil && m.appear != nil
}

type layoutBatch struct {
	anim    *driver.LayoutAnimation
	created map[driver.NodeID]bool
	deleted map[objc.ID]bool
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
		if m == nil {
			m = &motion{}
			s.motions[v] = m
		}
		m.from, m.to, m.frame, m.start = v.frame, f, tr, s.layoutStart(tr)
		s.startTicking()
		return
	}
	if m != nil {
		m.frame = nil
	}
	v.place(f)
}

// place sets v's frame and what follows it. Bounds and center, unlike
// the frame, hold with a transform; the bounds' origin is a scroll view's
// offset.
func (v *view) place(f driver.Bounds) {
	resized := f.Size != v.frame.Size
	v.frame, v.framed = f, true
	b := objc.Send[apple.CGRect](v.native, selBounds)
	b.Size = apple.CGSize{Width: float64(f.Size.Width), Height: float64(f.Size.Height)}
	v.native.Send(sel("setBounds:"), b)
	v.native.Send(sel("setCenter:"), apple.CGPoint{X: float64(f.Origin.X + v.shift.X + f.Size.Width/2),
		Y: float64(f.Origin.Y + v.shift.Y + f.Size.Height/2)})
	if v.styled && v.drawn.Transform != (driver.Transform{}) {
		// Its origin is a point of the old size.
		apple.SetTransform(v.native.Send(selLayer), v.drawn.Transform)
	}
	apple.LayoutDecor(v.native.Send(selLayer))
	if v.backdrop != 0 {
		apple.FitBackdrop(v.backdrop, &v.style, float64(f.Size.Width), float64(f.Size.Height))
	}
	if v.componentView != 0 {
		v.componentView.Send(selSetFrame, apple.Rect(0, 0, float64(f.Size.Width), float64(f.Size.Height)))
	}
	v.sizeContent()
	if resized && v.sc != nil && v.hasEdges() {
		// Each step of a layout animation, as the keyboard moves a composer.
		v.placeEdges()
	}
	if v.image != nil {
		v.image.Layout(apple.CGSize{Width: float64(f.Size.Width), Height: float64(f.Size.Height)})
		v.surface.images = append(v.surface.images, v)
	}
	if v.video != nil {
		v.video.Layout()
	}
	if v.shader != nil {
		v.sizeShader()
	}
	if field := v.surface.accessories[v.id]; field != nil && field.input != nil {
		// A taller bar shows only once the keyboard asks for it again.
		field.input.field.Send(sel("reloadInputViews"))
	}
}

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
	v.native.Send(sel("setUserInteractionEnabled:"), false)
	s.ghosts = append(s.ghosts, &ghost{native: v.native, opacity: objc.Send[float32](v.native.Send(selLayer), sel("opacity")),
		tr: lb.anim.Delete, start: s.clock()})
	s.startTicking()
	return true
}

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

// configureNative makes a native view's component on its first item and
// passes it the props that changed.
func (s *surface) configureNative(v *view, c *driver.NativeConfig) {
	if c.Bind {
		s.animator.BindNative(v.id, c.Bindings)
	}
	if v.component == nil && !v.componentTried {
		v.componentTried = true
		factory := s.components[c.Component]
		if factory == nil {
			factory = s.hostFactory(c.Component)
		}
		if factory == nil && c.Component == driver.TabBarComponent {
			factory = newTabBar
		}
		if factory == nil {
			if !s.unknown[c.Component] {
				s.unknown[c.Component] = true
				log.Printf("nsgo: no native component %q is registered", c.Component)
			}
			return
		}
		e := s.engine
		id := v.id
		v.component = factory(driver.NewNativeContext(
			func(name string, data any) {
				e.Dispatch(driver.Event{Kind: driver.EventNative, ID: id, Text: name, Data: data})
			},
			func(size driver.Size) { e.Dispatch(driver.Event{Kind: driver.EventNativeSize, ID: id, Size: size}) },
		))
		if v.component == nil {
			return
		}
		var native objc.ID
		switch h := v.component.View().(type) {
		case objc.ID:
			native = h
		case uintptr:
			native = objc.ID(h)
		}
		if native != 0 {
			native.Send(selSetFrame, objc.Send[apple.CGRect](v.native, selBounds))
			v.native.Send(selAddSubview, native)
			v.componentView = native
			s.directTree(native)
		}
	}
	if v.component == nil {
		return
	}
	for name, value := range c.Props {
		v.component.SetProp(name, value)
	}
}

// setAccessory makes the InputAccessoryView id the bar above input v's
// keyboard, or removes the bar for 0.
func (s *surface) setAccessory(v *view, id driver.NodeID) {
	if v.accessory == id {
		return
	}
	if old := v.accessory; old != 0 && s.accessories[old] == v {
		delete(s.accessories, old)
	}
	v.accessory = id
	bar := objc.ID(0)
	if id != 0 && int(id) < len(s.views) && s.views[id] != nil {
		bar = s.views[id].native
		const flexibleWidth = 2
		bar.Send(sel("setAutoresizingMask:"), uint(flexibleWidth))
		s.accessories[id] = v
	}
	v.input.setBar(bar)
}

//go:build darwin && !ios

package macos

import (
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// Effects, transitions and navigation shaders (driver.Shader.Role) draw
// over their host's children with Metal, from pictures of them: the
// children go into a container view, which an empty mask hides while the
// shader draws in their place (AppKit hit-tests views, so they still take
// the pointer), and the shader's layer lies over it at the top of the
// host's layer. A view's own effect hosts on it; a screen shader's on the
// root view. Pictures are taken with renderInContext: after a batch that
// says they changed (ItemShaderInput) or a frame in which anything moved,
// at most once a frame, and only those the program reads; a running
// transition's live side again each frame. Without Metal nothing is
// masked: the children show as they are.

// effect is a view's effect, transition or navigation shader.
type effect struct {
	shader *driver.Shader
	layer  *apple.ShaderLayer
	// pics are content, backdrop, from and to; before is the picture an
	// ItemShaderTransition took, which the Shader that follows places.
	pics      [4]apple.Picture
	before    apple.Picture
	hasBefore bool
	stale     bool
	// host is the view drawn over, once attached; masked whether its
	// container hides its children.
	host   *view
	masked bool
	frame  driver.Bounds
}

// zEffect puts an effect's layer over the host's children.
const zEffect = 1e6

var emptyMask objc.ID

// setEffect gives v its effect, transition or navigation shader, or takes
// it away for nil.
func (s *surface) setEffect(v *view, sh *driver.Shader) {
	fx := v.fx
	if sh == nil || sh.Program == nil {
		if fx != nil {
			s.endEffect(v)
		}
		return
	}
	if fx == nil {
		fx = &effect{}
		v.fx = fx
	}
	if fx.layer == nil {
		fx.layer = apple.NewShaderLayer(float64(s.env.PixelRatio))
		if fx.layer == nil {
			// No Metal: the children show as they are.
			return
		}
		fx.layer.Layer.Send(selActions, contentActions)
		fx.layer.Layer.Send(selSetZPosition, float64(zEffect))
		fx.layer.Ready = func() { s.shadeLater(v) }
	}
	was := fx.shader
	fx.shader = sh
	fx.layer.Set(sh)
	s.animator.BindShader(v.id, sh.Nodes)
	if sh.Role.Transitions() {
		switch {
		case sh.Running && fx.hasBefore:
			side := 2
			if sh.Backwards {
				side = 3
			}
			fx.pics[side], fx.before = fx.before, fx.pics[side]
			fx.hasBefore = false
		case !sh.Running && was != nil && was.Running:
			fx.pics[2].Release()
			fx.pics[3].Release()
		}
	}
	fx.stale = true
	s.effects[v] = true
	s.shadeLater(v)
}

// startTransition pictures what v's transition starts from, before the
// batch that starts it is applied.
func (s *surface) startTransition(v *view) {
	if v.fx == nil {
		v.fx = &effect{}
	}
	fx := v.fx
	host := s.effectHost(v)
	if host == nil {
		return
	}
	if fx.shader != nil && fx.shader.Running && fx.host != nil {
		// One that runs ends here: its picture is the one it drew last.
		s.drawEffect(v)
	}
	fx.hasBefore = s.picture(fx, host, &fx.before, 0)
}

// effectHost is the view v's shader draws over: v, or the root view for
// a screen shader.
func (s *surface) effectHost(v *view) *view {
	if v.fx != nil && v.fx.shader != nil && v.fx.shader.Role.Covers() {
		return s.views[s.root]
	}
	return v
}

// container puts host's children into a view of their own, which an
// effect masks, once.
func (s *surface) container(host *view) objc.ID {
	if host.content != host.native {
		return host.content
	}
	c := objc.ID(classView).Send(selNew)
	c.Send(selSetWantsLayer, true)
	c.Send(selSetFrame, objc.Send[apple.CGRect](host.native, selBounds))
	const widthSizable, heightSizable = 2, 16
	c.Send(sel("setAutoresizingMask:"), uint(widthSizable|heightSizable))
	subs := host.native.Send(sel("subviews"))
	n := objc.Send[uint](subs, sel("count"))
	var children []objc.ID
	for i := range n {
		sub := subs.Send(sel("objectAtIndex:"), i)
		if w := views[sub]; w != nil && w.up == host {
			children = append(children, sub)
		}
	}
	for _, sub := range children {
		sub.Send(sel("retain"))
		sub.Send(selRemoveFromSuperview)
		c.Send(selAddSubview, sub)
		sub.Send(selRelease)
	}
	host.native.Send(selAddSubview, c)
	host.content = c
	views[c] = host
	return c
}

// attachEffect puts fx's layer over its host, a container under it, as the
// host changes: a screen shader's root comes with the first batch.
func (s *surface) attachEffect(v *view) bool {
	fx := v.fx
	host := s.effectHost(v)
	if host == nil || fx.layer == nil {
		return false
	}
	if fx.host != host {
		s.unmask(fx)
		fx.layer.Layer.Send(sel("removeFromSuperlayer"))
		s.container(host)
		host.native.Send(selLayer).Send(sel("addSublayer:"), fx.layer.Layer)
		fx.host = host
	}
	return true
}

// shows reports whether fx draws over its host's children now: an effect
// always, a transition while it runs.
func (fx *effect) shows() bool {
	return fx.shader != nil && (!fx.shader.Role.Transitions() || fx.shader.Running)
}

func (s *surface) mask(fx *effect, on bool) {
	if fx.masked == on || fx.host == nil {
		return
	}
	fx.masked = on
	layer := fx.host.content.Send(selLayer)
	apple.WithoutAnimation(func() {
		if on {
			if emptyMask == 0 {
				emptyMask = objc.ID(apple.Class("CALayer")).Send(selNew)
			}
			layer.Send(sel("setMask:"), emptyMask)
		} else {
			layer.Send(sel("setMask:"), objc.ID(0))
		}
		fx.layer.Layer.Send(selSetHidden, !on)
	})
}

func (s *surface) unmask(fx *effect) { s.mask(fx, false) }

// picture takes pic of what host shows for fx, the box and the overflow
// on each side; the backdrop (kind 1) is the root drawn without host.
func (s *surface) picture(fx *effect, host *view, pic *apple.Picture, kind int) bool {
	ov := 0.0
	if fx.shader != nil {
		ov = fx.shader.Overflow
	}
	b := objc.Send[apple.CGRect](host.native, selBounds)
	w, h := b.Size.Width+2*ov, b.Size.Height+2*ov
	scale := float64(s.env.PixelRatio)
	if kind == 1 {
		root := s.views[s.root]
		if root == nil {
			return false
		}
		at := objc.Send[apple.CGPoint](host.native, sel("convertPoint:toView:"), apple.CGPoint{X: -ov, Y: -ov}, root.native)
		return pic.Take(root.native.Send(selLayer), at.X, at.Y, w, h, scale, host.native.Send(selLayer))
	}
	layer := s.container(host).Send(selLayer)
	return pic.Take(layer, -ov, -ov, w, h, scale, 0)
}

// drawEffects pictures what changed of the effects that show and draws
// them.
func (s *surface) drawEffects() {
	for v := range s.effects {
		if s.views[v.id] != v || v.fx == nil {
			delete(s.effects, v)
			continue
		}
		s.drawEffect(v)
	}
}

func (s *surface) drawEffect(v *view) {
	fx := v.fx
	if fx.layer == nil || !s.attachEffect(v) {
		return
	}
	shows := fx.shows() && fx.layer.Drawable()
	s.mask(fx, shows)
	if !shows {
		return
	}
	host := fx.host
	ov := fx.shader.Overflow
	b := objc.Send[apple.CGRect](host.native, selBounds)
	frame := driver.Bounds{Origin: driver.Point{X: driver.Points(-ov), Y: driver.Points(-ov)},
		Size: driver.Size{Width: driver.Points(b.Size.Width + 2*ov), Height: driver.Points(b.Size.Height + 2*ov)}}
	if frame != fx.frame {
		fx.frame = frame
		apple.WithoutAnimation(func() {
			fx.layer.Layer.Send(selSetFrame, apple.Rect(-ov, -ov, b.Size.Width+2*ov, b.Size.Height+2*ov))
		})
		fx.layer.Resize(b.Size.Width+2*ov, b.Size.Height+2*ov, float64(s.env.PixelRatio))
		fx.stale = true
	}
	sh := fx.shader
	reads := sh.Pictures
	if sh.Role.Transitions() {
		// The live side is pictured each frame while it runs.
		live := 3
		if sh.Backwards {
			live = 2
		}
		if reads&(1<<live) != 0 {
			s.picture(fx, host, &fx.pics[live], 0)
		}
	} else if fx.stale {
		if reads&driver.PictureContent != 0 {
			s.picture(fx, host, &fx.pics[0], 0)
		}
		if reads&driver.PictureBackdrop != 0 {
			s.picture(fx, host, &fx.pics[1], 1)
		}
	}
	fx.stale = false
	fx.layer.SetPictures([4]*apple.Picture{&fx.pics[0], &fx.pics[1], &fx.pics[2], &fx.pics[3]})
	fx.layer.Draw(func(lanes []float64) { s.animator.ShaderLanes(v.id, lanes) })
}

// staleEffects marks every effect for its pictures again, after a frame
// in which views moved.
func (s *surface) staleEffects() {
	for v := range s.effects {
		if v.fx != nil {
			v.fx.stale = true
		}
	}
}

// navigating reports whether a navigation shader runs over host, which
// the change it draws then shows at once under it.
func (s *surface) navigating(host *view) bool {
	return host != nil && host.fx != nil && host.fx.shader != nil && host.fx.shader.Role == driver.ShaderNav &&
		host.fx.shader.Running && host.fx.layer != nil
}

// endEffect takes v's shader away: its children show as they are.
func (s *surface) endEffect(v *view) {
	fx := v.fx
	if fx == nil {
		return
	}
	v.fx = nil
	delete(s.effects, v)
	s.unmask(fx)
	if fx.layer != nil {
		fx.layer.Layer.Send(sel("removeFromSuperlayer"))
		fx.layer.Release()
	}
	for i := range fx.pics {
		fx.pics[i].Release()
	}
	fx.before.Release()
}

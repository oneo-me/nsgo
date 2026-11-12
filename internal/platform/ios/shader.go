//go:build ios

package ios

import (
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// A shader view draws with Metal into a layer of its own below its
// subviews' layers: when its uniforms, its size or the animated nodes its
// lanes follow change. The frame clock runs while those nodes animate.

// newShader gives a shader view its Metal layer, or none where the
// device has no Metal.
func (v *view) newShader() {
	sl := apple.NewShaderLayer(float64(v.surface.env.PixelRatio))
	if sl == nil {
		return
	}
	sl.Layer.Send(sel("setZPosition:"), apple.Z(contentZ))
	sl.Ready = func() { v.surface.shadeLater(v) }
	v.native.Send(selLayer).Send(sel("addSublayer:"), sl.Layer)
	v.shader = sl
}

// setShader gives v what it draws and binds its animated lanes.
func (s *surface) setShader(v *view, sh *driver.Shader) {
	if v.shader == nil {
		return
	}
	v.shader.Set(sh)
	s.animator.BindShader(v.id, sh.Nodes)
	s.shadeLater(v)
}

// sizeShader keeps v's Metal layer over its bounds, drawing it again.
func (v *view) sizeShader() {
	w, h := float64(v.frame.Size.Width), float64(v.frame.Size.Height)
	apple.WithoutAnimation(func() {
		v.shader.Layer.Send(selSetFrame, apple.Rect(0, 0, w, h))
	})
	v.shader.Resize(w, h, float64(v.surface.env.PixelRatio))
	v.surface.shadeLater(v)
}

// shadeLater draws v once the batch or the frame is applied, or soon on
// the main thread when neither applies, as after a program compiled.
func (s *surface) shadeLater(v *view) {
	if v.shader == nil && v.fx == nil {
		return
	}
	for _, w := range s.shading {
		if w == v {
			return
		}
	}
	s.shading = append(s.shading, v)
	if len(s.shading) == 1 {
		apple.OnMain(s.drawShaders)
	}
}

// drawShaders draws the shader views whose animated lanes changed, and
// those waiting to draw.
func (s *surface) drawShaders() {
	s.animator.ChangedShaders(func(id driver.NodeID) {
		if int(id) < len(s.views) && s.views[id] != nil {
			s.shadeLater(s.views[id])
		}
	})
	for _, v := range s.shading {
		if s.views[v.id] == v && v.shader != nil {
			v.shader.Draw(func(lanes []float64) { s.animator.ShaderLanes(v.id, lanes) })
		}
	}
	clear(s.shading)
	s.shading = s.shading[:0]
	s.drawEffects()
}

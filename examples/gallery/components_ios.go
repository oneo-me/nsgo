//go:build ios

package main

import (
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/core"
)

func init() {
	components[sliderComponent] = func(cx *core.NativeContext) core.NativeComponent {
		return newAppleSlider(cx, "UISlider", func(v objc.ID) {
			const valueChanged = 1 << 12
			v.Send(sel("addTarget:action:forControlEvents:"), sliderTarget, sel("changed:"), uint(valueChanged))
		})
	}
}

func (s *appleSlider) value() float64     { return float64(objc.Send[float32](s.view, sel("value"))) }
func (s *appleSlider) setValue(v float64) { s.view.Send(sel("setValue:"), float32(v)) }

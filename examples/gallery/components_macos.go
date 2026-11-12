//go:build darwin && !ios

package main

import (
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/core"
)

func init() {
	components[sliderComponent] = func(cx *core.NativeContext) core.NativeComponent {
		return newAppleSlider(cx, "NSSlider", func(v objc.ID) {
			v.Send(sel("setMinValue:"), 0.0)
			v.Send(sel("setMaxValue:"), 1.0)
			v.Send(sel("setContinuous:"), true)
			v.Send(sel("setTarget:"), sliderTarget)
			v.Send(sel("setAction:"), sel("changed:"))
		})
	}
}

func (s *appleSlider) value() float64     { return objc.Send[float64](s.view, sel("doubleValue")) }
func (s *appleSlider) setValue(v float64) { s.view.Send(sel("setDoubleValue:"), v) }

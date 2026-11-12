//go:build darwin

package main

import (
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/core"
)

// appleSlider is AppKit's NSSlider or UIKit's UISlider, as a native
// component: it reports its value as "change" events.
type appleSlider struct {
	view objc.ID
	cx   *core.NativeContext
}

var (
	sliders      = map[objc.ID]*appleSlider{}
	sliderTarget objc.ID
	sel          = objc.RegisterName
)

// newAppleSlider makes a slider of class with the value's selectors of
// its toolkit.
func newAppleSlider(cx *core.NativeContext, class string, setup func(objc.ID)) *appleSlider {
	if sliderTarget == 0 {
		c, err := objc.RegisterClass("GallerySliderTarget", objc.GetClass("NSObject"), []*objc.Protocol{}, []objc.FieldDef{},
			[]objc.MethodDef{{Cmd: sel("changed:"), Fn: func(_ objc.ID, _ objc.SEL, sender objc.ID) {
				if s := sliders[sender]; s != nil {
					s.cx.Emit("change", s.value())
				}
			}}})
		if err != nil {
			panic(err)
		}
		sliderTarget = objc.ID(c).Send(sel("new"))
	}
	s := &appleSlider{view: objc.ID(objc.GetClass(class)).Send(sel("new")), cx: cx}
	setup(s.view)
	sliders[s.view] = s
	cx.SetSize(core.Size{Width: 220, Height: 28})
	return s
}

func (s *appleSlider) View() any { return s.view }

func (s *appleSlider) SetProp(name string, value any) {
	if v, ok := value.(float64); ok && name == "value" {
		s.setValue(v)
	}
}

func (s *appleSlider) Command(name string, args any) {
	if name == "reset" {
		s.setValue(0)
		s.cx.Emit("change", 0.0)
	}
}

func (s *appleSlider) Close() {
	delete(sliders, s.view)
	s.view.Send(sel("release"))
}

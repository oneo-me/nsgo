//go:build js && wasm

package main

import (
	"syscall/js"

	"github.com/nsgo-dev/nsgo/core"
)

// rangeSlider is an <input type=range>, as a native component.
type rangeSlider struct {
	el     js.Value
	cx     *core.NativeContext
	listen js.Func
}

func init() {
	components[sliderComponent] = func(cx *core.NativeContext) core.NativeComponent {
		s := &rangeSlider{el: js.Global().Get("document").Call("createElement", "input"), cx: cx}
		s.el.Set("type", "range")
		s.el.Set("min", 0)
		s.el.Set("max", 1)
		s.el.Set("step", 0.01)
		s.listen = js.FuncOf(func(js.Value, []js.Value) any {
			cx.Emit("change", s.el.Get("valueAsNumber").Float())
			return nil
		})
		s.el.Call("addEventListener", "input", s.listen)
		cx.SetSize(core.Size{Width: 220, Height: 28})
		return s
	}
}

func (s *rangeSlider) View() any { return s.el }

func (s *rangeSlider) SetProp(name string, value any) {
	if v, ok := value.(float64); ok && name == "value" {
		s.el.Set("value", v)
	}
}

func (s *rangeSlider) Command(name string, args any) {
	if name == "reset" {
		s.el.Set("value", 0)
		s.cx.Emit("change", 0.0)
	}
}

func (s *rangeSlider) Close() {
	s.el.Call("removeEventListener", "input", s.listen)
	s.listen.Release()
}

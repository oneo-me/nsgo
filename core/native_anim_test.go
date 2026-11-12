package core_test

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestNativePropsTakeAnimatedValues(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		progress := core.NewAnimatedValue(cx, 0)
		tint := core.Interpolate(progress, core.Interpolation{Input: []float64{0, 1}, Colors: []core.Color{core.RGB(0x000000), core.RGB(0xffffff)}})
		bound := core.NewSignal(cx, true)
		return core.View{Children: []core.Node{
			core.View{TestID: "go", Style: core.Style{Height: core.Pt(20)}, OnPress: func(core.PressEvent) {
				cx.StartAnimation(core.Timing(progress, 1, core.TimingConfig{Duration: 100 * time.Millisecond}), nil)
			}},
			core.View{TestID: "unbind", Style: core.Style{Height: core.Pt(20)}, OnPress: func(core.PressEvent) { bound.Set(false) }},
			core.Native{Component: "gauge", TestID: "gauge", Style: core.Style{Height: core.Pt(20)},
				PropsFunc: func() map[string]any {
					if bound.Get() {
						return map[string]any{"progress": progress, "tint": tint, "label": "Load"}
					}
					return map[string]any{"progress": 0.25, "label": "Load"}
				}},
		}}
	})
	props := func() map[string]any { return app.Find("id:gauge").NativeProps }
	if p := props(); p["progress"] != 0.0 || p["tint"] != core.RGB(0x000000) || p["label"] != "Load" {
		t.Fatalf("props = %v, want the bound values at their start", p)
	}
	app.Press("id:go")
	app.Advance(200 * time.Millisecond)
	if p := props(); p["progress"] != 1.0 || p["tint"] != core.RGB(0xffffff) {
		t.Fatalf("props = %v, want the values the animation reached", p)
	}
	app.Press("id:unbind")
	app.Advance(50 * time.Millisecond)
	if p := props(); p["progress"] != 0.25 || p["tint"] != nil {
		t.Fatalf("props = %v, want the plain value and the bound color gone", p)
	}
}

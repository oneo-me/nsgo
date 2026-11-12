package core_test

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestEnvInsets(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.View{TestID: "safe", Style: core.Style{Padding: core.All(10),
			PaddingEnv: core.EnvEdges{Top: core.EnvSafeArea, Bottom: core.EnvSafeArea}},
			Children: []core.Node{
				core.View{TestID: "avoid", Style: core.Style{PaddingEnv: core.EnvEdges{Bottom: core.EnvKeyboard}},
					Children: []core.Node{core.Text{Content: "x", Style: core.Style{FontSize: 17}}}},
			}}
	})
	app.SetEnvironment(core.Environment{SafeArea: core.Edges{Top: 44, Bottom: 34, Left: 5}, KeyboardHeight: 300})
	safe, avoid := app.Find("id:safe"), app.Find("id:avoid")
	if got := avoid.Frame.Origin; got != (core.Point{X: 10, Y: 54}) {
		t.Errorf("content origin = %v, want the inset added to the top padding only", got)
	}
	// Text 17 points tall is 20.4, rounded up to 21.
	if got, want := safe.Frame.Size.Height, core.Points(10+44+300+21+10+34); got != want {
		t.Errorf("height = %v, want %v", got, want)
	}
	app.LayoutAnimation = nil
	app.SetEnvironment(core.Environment{SafeArea: core.Edges{Top: 44, Bottom: 34, Left: 5}, KeyboardHeight: 0,
		KeyboardDuration: 280 * time.Millisecond})
	if a := app.LayoutAnimation; a == nil || a.Update == nil || a.Update.Curve != core.LayoutKeyboard ||
		a.Update.Duration != 280*time.Millisecond {
		t.Fatalf("the keyboard going animates %+v, want its frames on the keyboard's curve and time", a)
	}
	if got, want := app.Find("id:safe").Frame.Size.Height, core.Points(10+44+21+10+34); got != want {
		t.Errorf("height without the keyboard = %v, want %v", got, want)
	}
}

func TestEnvInsetsFollowTheirEdge(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		rtl, keyboard := core.NewSignal(cx, false), core.NewSignal(cx, true)
		return core.View{Style: core.Style{FlexGrow: 1}, StyleFunc: func() core.Style {
			if rtl.Get() {
				return core.Style{LayoutDirection: core.DirectionRTL}
			}
			return core.Style{LayoutDirection: core.DirectionLTR}
		}, Children: []core.Node{
			core.View{TestID: "toggle", Style: core.Style{Height: core.Pt(10)},
				OnPress: func(core.PressEvent) { rtl.Set(true); keyboard.Set(false) }},
			core.View{TestID: "row", Style: core.Style{Height: core.Pt(10), MarginEnv: core.EnvEdges{Left: core.EnvSafeArea}}},
			core.View{TestID: "bar", StyleFunc: func() core.Style {
				s := core.Style{Height: core.Pt(10), Padding: core.Edges{Bottom: -20}}
				if keyboard.Get() {
					s.PaddingEnv = core.EnvEdges{Bottom: core.EnvKeyboard}
				}
				return s
			}},
		}}
	})
	app.SetEnvironment(core.Environment{SafeArea: core.Edges{Left: 30, Right: 7}, KeyboardHeight: 100})
	if got := app.Find("id:row").Frame.Origin.X; got != 30 {
		t.Errorf("left to right, row x = %v, want the left inset", got)
	}
	if got := app.Find("id:bar").Frame.Size.Height; got != 80 {
		t.Errorf("bar height = %v, want the keyboard less 20", got)
	}
	app.Press("id:toggle")
	row := app.Find("id:row").Frame
	if got := app.Find("id:bar").Frame.Size.Width - row.Origin.X - row.Size.Width; got != 7 {
		t.Errorf("right to left, the row's right margin = %v, want the right inset", got)
	}
	if got := app.Find("id:bar").Frame.Size.Height; got != 10 {
		t.Errorf("bar height without the term = %v, want its own", got)
	}
}

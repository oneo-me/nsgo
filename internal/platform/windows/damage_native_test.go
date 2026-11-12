//go:build windows

package windows

import (
	"fmt"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// TestPartialFramesDrawWhatAWholeOneDoes changes a tree a step at a time
// (a box moving and changing color, a text, a view hidden, a canvas drawn
// again) and checks after each that the partial frames left the window
// as a whole frame draws it.
func TestPartialFramesDrawWhatAWholeOneDoes(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-native-" + t.Name()
	steps := make(chan func(int), 1)
	app := func(cx *core.Scope) core.Node {
		step, dot := core.NewSignal(cx, 0), core.NewSignal(cx, 0)
		steps <- func(n int) {
			if n < 0 {
				cx.Post(func() { dot.Set(dot.Get() + 1) })
			} else {
				cx.Post(func() { step.Set(n) })
			}
		}
		colors := []driver.Color{driver.RGB(0xcc2222), driver.RGB(0x22aa22), driver.RGB(0x2222cc), driver.RGB(0x888800)}
		return core.View{Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(240), Background: driver.RGB(0xffffff)},
			Children: []core.Node{
				// A dot whose changes take the frame after a whole one,
				// which draws everything.
				core.View{StyleFunc: func() core.Style {
					return core.Style{Position: core.Absolute, Left: driver.Pt(300), Top: driver.Pt(4), Width: driver.Pt(6),
						Height: driver.Pt(6), Background: colors[dot.Get()%len(colors)]}
				}},
				core.View{StyleFunc: func() core.Style {
					n := step.Get()
					return core.Style{Position: core.Absolute, Left: driver.Pt(driver.Points(20 + 50*n)), Top: driver.Pt(30),
						Width: driver.Pt(40), Height: driver.Pt(40), Background: colors[n%len(colors)], CornerRadius: 8,
						ShadowColor: driver.RGBA(0, 0.4), ShadowRadius: 6, ShadowOffset: driver.Point{X: 2, Y: 3}}
				}},
				// A layer, as a partial frame's areas are, so its text antialiases
				// the same there as in a whole frame.
				core.Text{ContentFunc: func() string { return fmt.Sprintf("step %d", step.Get()) },
					Style: core.Style{Position: core.Absolute, Left: driver.Pt(20), Top: driver.Pt(90), Opacity: core.Alpha(0.999)}},
				core.View{StyleFunc: func() core.Style {
					s := core.Style{Position: core.Absolute, Left: driver.Pt(200), Top: driver.Pt(90), Width: driver.Pt(60),
						Height: driver.Pt(30), Background: driver.RGB(0x222222)}
					if step.Get() == 2 {
						s.Visibility = core.VisibilityHidden
					}
					return s
				}},
				core.Canvas{Style: core.Style{Position: core.Absolute, Left: driver.Pt(20), Top: driver.Pt(140),
					Width: driver.Pt(280), Height: driver.Pt(80)}, Draw: func(d *core.Drawing, size core.Size) {
					n := step.Get()
					d.SetFillColor(driver.RGB(0xeeeeff)).FillRect(0, 0, float32(size.Width), float32(size.Height))
					d.SetStrokeColor(driver.RGB(0x2266cc)).SetLineWidth(2).BeginPath().MoveTo(0, 40)
					for i := 0; i <= 28; i++ {
						d.LineTo(float32(i*10), 40+float32((i*7+n*13)%30)-15)
					}
					d.Stroke()
				}},
			}}
	}
	hwnd := startApp(t, title, app, 320, 240)
	var set func(int)
	select {
	case set = <-steps:
	case <-time.After(5 * time.Second):
		t.Fatal("the app did not start")
	}
	settle := func() []byte {
		time.Sleep(400 * time.Millisecond)
		pix, _, _, ok := clientPixels(hwnd)
		if !ok {
			t.Skip("cannot read the window's pixels")
		}
		return pix
	}
	settle()
	for n := 1; n <= 4; n++ {
		set(-1)
		settle()
		set(n)
		partial := settle()
		onMain(t, hwnd, func(w *window) {
			if w.damage.lastWhole || len(w.damage.last) == 0 {
				t.Errorf("step %d: the last frame was not partial", n)
			}
		})
		onMain(t, hwnd, func(w *window) { w.redrawAll() })
		whole := settle()
		differ := 0
		for i := range partial {
			if d := int(partial[i]) - int(whole[i]); d > 2 || d < -2 {
				differ++
			}
		}
		if differ > 0 {
			t.Errorf("step %d: %d bytes of the partial frames differ from a whole frame", n, differ)
		}
	}
}

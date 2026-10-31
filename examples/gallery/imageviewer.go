package main

import (
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/base/textview"
	"github.com/oneo-me/nsgo/core"
)

// The viewer's deepest zoom and where a double tap zooms to, how close
// in time two taps are a double tap, and how far a drag down at 1×
// closes it.
const (
	viewerMaxZoom    = 4
	viewerTapZoom    = 2.5
	viewerDoubleTap  = 300 * time.Millisecond
	viewerSwipeClose = 120
)

// imageViewer is NSH's full-screen viewer for an image a TextView
// reports (OnImagePress): the image fitted over a dark scrim, fading and
// growing in. A pinch (two fingers, a touchpad) zooms up to 4×, a double
// tap toggles 2.5× at the point, a drag pans the zoomed image and at 1× a
// drag down closes it. A press on the scrim, the close button, Escape and
// back close it (onClose); the alt text is the caption. The gestures move
// animated values on the platform's main thread, and only their ends
// settle the result into bounds.
func imageViewer(cx *core.Scope, image func() textview.Image, onClose func()) core.Node {
	// The image stays while the dialog fades out.
	shown := core.NewSignal(cx, textview.Image{})
	natural := core.NewSignal(cx, core.Size{})
	viewport := core.NewSignal(cx, core.Size{})
	scale, x, y := core.NewAnimatedValue(cx, 1), core.NewAnimatedValue(cx, 0), core.NewAnimatedValue(cx, 0)
	enter := core.NewAnimatedValue(cx, 0)
	// at is where the values settle, the gestures' starting points.
	type place struct{ scale, x, y float64 }
	at := place{scale: 1}
	var lastTap time.Time
	var tap core.Point
	src := ""
	cx.Effect(func() {
		im := image()
		if im.Src == "" {
			return
		}
		cx.Untrack(func() {
			if im.Src != src {
				natural.Set(core.Size{})
			}
			src = im.Src
			shown.Set(im)
			at = place{scale: 1}
			scale.Set(1)
			x.Set(0)
			y.Set(0)
			enter.Set(0)
			spring(cx, enter, 1, settleSpring)
		})
	})
	// box is the image's box at 1×: its aspect fitted into the viewport.
	box := func() core.Size {
		v, n := viewport.Get(), natural.Get()
		if n.Width == 0 || n.Height == 0 || v.Width == 0 {
			return v
		}
		k := min(v.Width/n.Width, v.Height/n.Height)
		return core.Size{Width: n.Width * k, Height: n.Height * k}
	}
	clamp := func(v, bound float64) float64 { return max(-bound, min(bound, v)) }
	// settle springs to next, within the zoom's limits and the image's
	// edges.
	settle := func(next place) {
		s := max(1, min(viewerMaxZoom, next.scale))
		b, v := core.Untracked(cx, box), viewport.GetUntracked()
		at = place{scale: s,
			x: clamp(next.x, max(0, (float64(b.Width)*s-float64(v.Width))/2)),
			y: clamp(next.y, max(0, (float64(b.Height)*s-float64(v.Height))/2))}
		spring(cx, scale, at.scale, settleSpring)
		spring(cx, x, at.x, settleSpring)
		spring(cx, y, at.y, settleSpring)
	}
	// pressImage is a tap on the image: the second of two zooms in at the
	// point, or out.
	pressImage := func(e core.PressEvent) {
		now := cx.Now()
		if now.Sub(lastTap) > viewerDoubleTap {
			lastTap, tap = now, e.Position
			return
		}
		lastTap = time.Time{}
		if at.scale > 1.01 {
			settle(place{scale: 1})
			return
		}
		// The tapped point, from the box's middle unzoomed, stays under the
		// finger.
		b := core.Untracked(cx, box)
		dx, dy := float64(tap.X-b.Width/2), float64(tap.Y-b.Height/2)
		settle(place{scale: viewerTapZoom, x: at.x + (at.scale-viewerTapZoom)*dx, y: at.y + (at.scale-viewerTapZoom)*dy})
	}
	grow := core.Interpolate(enter, core.Interpolation{Input: []float64{0, 1}, Output: []float64{0.9, 1}})
	env := cx.Env()
	return base.Dialog{Open: func() bool { return image().Src != "" }, Animation: core.ModalFade, Label: "Image",
		StatusBar: core.StatusBarSetting{Foreground: core.StatusBarLight},
		OnOpenChange: func(open bool, _ base.Reason) {
			if !open {
				onClose()
			}
		},
		PopupStyle: core.Style{FlexGrow: 1, AlignSelf: core.AlignStretch},
		Children: []core.Node{core.View{TestID: "image-viewer", NoFocus: true,
			OnPress:    func(core.PressEvent) { onClose() },
			OnLayout:   func(b core.Bounds) { viewport.Set(b.Size) },
			PinchValue: scale, PanValues: core.AnimatedXY{X: x, Y: y},
			OnPinch: func(e core.GestureEvent) {
				if e.Phase == core.GestureEnded || e.Phase == core.GestureCancelled {
					scale.Read(cx, func(s float64) { settle(place{scale: s, x: at.x, y: at.y}) })
				}
			},
			OnPan: func(e core.PanEvent) {
				if e.Phase != core.GestureEnded && e.Phase != core.GestureCancelled {
					return
				}
				if dy := float64(e.Values.Y) - at.y; at.scale <= 1.01 && (dy > viewerSwipeClose || e.Velocity.Y > 1200) {
					onClose()
					return
				}
				settle(place{scale: at.scale, x: float64(e.Values.X), y: float64(e.Values.Y)})
			},
			Style: core.Style{FlexGrow: 1, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Background: black(0.92)},
			Children: []core.Node{
				core.View{Role: core.RoleImage, OnPress: pressImage, NoFocus: true,
					LabelFunc: func() string {
						if alt := shown.Get().Alt; alt != "" {
							return alt
						}
						return "Image"
					},
					StyleFunc: func() core.Style {
						b := box()
						return core.Style{Width: core.Pt(b.Width), Height: core.Pt(b.Height),
							Animated: &core.AnimatedStyle{Opacity: enter, TranslateX: x, TranslateY: y, Scale: core.Mul(scale, grow)}}
					},
					Children: []core.Node{core.Image{SourceFunc: func() core.ImageSource { return core.Src(shown.Get().Src) }, ResizeMode: core.Contain,
						OnLoad: natural.Set, Style: core.Style{Width: core.Pct(100), Height: core.Pct(100)}}}},
				core.Show{When: func() bool { return shown.Get().Alt != "" }, Then: func(*core.Scope) core.Node {
					return core.View{PointerEvents: core.PointerEventsNone, StyleFunc: func() core.Style {
						return core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0),
							Padding: core.Edges{Left: 24, Right: 24, Top: 32, Bottom: env.SafeArea().Bottom + 20}}
					}, Children: []core.Node{core.Text{ContentFunc: func() string { return shown.Get().Alt },
						Style: core.Style{FontSize: 14, LineHeight: 20, TextAlign: core.TextAlignCenter, Color: white(0.85)}}}}
				}},
				core.View{StyleFunc: func() core.Style {
					a := env.SafeArea()
					return core.Style{Position: core.Absolute, Top: core.Pt(a.Top + 12), Right: core.Pt(a.Right + 12)}
				}, Children: []core.Node{press(pressProps{onPress: onClose, label: "Close", scale: 0.9, hitSlop: 8},
					core.View{Style: core.Style{Width: core.Pt(36), Height: core.Pt(36), CornerRadius: 18, Background: white(0.16),
						AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
						Children: []core.Node{viewerBar(45), viewerBar(-45)}})}},
			}}},
	}
}

// viewerBar is a stroke of the close button's cross, turned degrees.
func viewerBar(degrees float32) core.Node {
	return core.View{Style: core.Style{Position: core.Absolute, Width: core.Pt(16), Height: core.Pt(2), CornerRadius: 1,
		Background: core.RGB(0xffffff), Transform: core.Transform{Rotate: degrees}}}
}

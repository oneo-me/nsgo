//go:build linux && !android

package linux

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// TestShadersDrawWithGL opens a window with a shader view whose program
// mixes a uniform color, and reads its pixels.
func TestShadersDrawWithGL(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	prog := core.MustCompileShader(`package main

var Tint vec4

func Fragment(dst vec4, pos vec2) vec4 {
	if pos.x < rectSize().x/2 {
		return Tint
	}
	return vec4(0, 0, 1, 1)
}
`)
	title := "nsgo-native-" + t.Name()
	app := func(cx *core.Scope) core.Node {
		cx.SetTitle(title)
		return core.View{Style: core.Style{Width: driver.Pt(200), Height: driver.Pt(100), Background: driver.RGB(0xffffff)},
			Children: []core.Node{core.Shader{Program: prog, Uniforms: core.Uniforms{"Tint": driver.RGB(0x00ff00)},
				Style: core.Style{Width: driver.Pt(200), Height: driver.Pt(100)}}}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 200, Height: 100})
	waitForTitle(t, w, title)
	waitPixels(t, w, "the shader's green left half and blue right half", func(p pixels) bool {
		left, right := p.at(40, 50), p.at(160, 50)
		return left[1] > 0xf0 && left[2] < 0x10 && right[0] > 0xf0 && right[1] < 0x10
	})
}

// TestShadersReadImages draws an image stretched over a shader view:
// red on its left half, blue on its right.
func TestShadersReadImages(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	src := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for x := range 4 {
		for y := range 2 {
			c := color.RGBA{255, 0, 0, 255}
			if x >= 2 {
				c = color.RGBA{0, 0, 255, 255}
			}
			src.SetRGBA(x, y, c)
		}
	}
	var encoded bytes.Buffer
	png.Encode(&encoded, src)
	prog := core.MustCompileShader(`package main

func Fragment(dst vec4, pos vec2) vec4 {
	return image0At(pos * image0Size() / rectSize())
}
`)
	title := "nsgo-native-" + t.Name()
	app := func(cx *core.Scope) core.Node {
		cx.SetTitle(title)
		return core.View{Style: core.Style{Width: driver.Pt(200), Height: driver.Pt(100), Background: driver.RGB(0xffffff)},
			Children: []core.Node{core.Shader{Program: prog, Images: []core.ImageSource{{Data: encoded.Bytes()}},
				Style: core.Style{Width: driver.Pt(200), Height: driver.Pt(100)}}}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 200, Height: 100})
	waitForTitle(t, w, title)
	waitPixels(t, w, "the image's red left half and blue right half", func(p pixels) bool {
		left, right := p.at(40, 50), p.at(160, 50)
		return left[2] > 0xf0 && left[0] < 0x10 && right[0] > 0xf0 && right[2] < 0x10
	})
}

---
title: Canvas and shaders
description: Draw 2D graphics on the engine, and fill a view with a GPU fragment shader.
group: Building UI
order: 20
---

## A canvas

`core.Canvas` records 2D drawing on the engine and has the platform play it, as a web page's `<canvas>` does. `Draw` runs again when a signal it read or the canvas's size changes, and the platform draws the recording on the main thread.

```go
core.Canvas{
	Style: core.Style{Height: core.Pt(220)},
	Draw: func(d *core.Drawing, size core.Size) {
		d.SetFillColor(core.RGB(0xf1f5f9)).FillRect(0, 0, float32(size.Width), float32(size.Height))
		d.SetStrokeColor(core.RGB(0x007aff)).SetLineWidth(2).SetLineCap(core.CapRound)
		d.BeginPath().MoveTo(24, 160).LineTo(90, 80).LineTo(160, 120).LineTo(230, 40).Stroke()
	},
}
```

The recording follows the web's `CanvasRenderingContext2D`: `Save` and `Restore`, transforms, paths of lines, curves, rectangles and arcs, fills and strokes with colors and gradients, clipping, dashes, shadows, text and images. `core.ParsePath` makes a `core.Path` from SVG path data, and `FillFromView` and `StrokeFromView` take the canvas view's inherited `Fill` and `Stroke` styles, so a chart follows the app's theme.

```go
d.SetFillGradient(core.CanvasLinear(0, 0, float32(size.Width), 0,
	core.GradientStop{Offset: 0, Color: core.RGB(0x4da3ff)},
	core.GradientStop{Offset: 1, Color: core.RGB(0x007aff)},
)).FillRect(0, 0, float32(size.Width), 40)
```

## Text, images and parts

`MeasureText`, `TextMetrics` and `SetTextAlign` lay text out on the platform's text engine, so labels use the same fonts as the rest of the app. `DrawImage`, `DrawImageIn` and `DrawImageFrom` draw pictures, and `OnPress` can test the last drawing with `IsPointInPath` and `IsPointInStroke`.

`Drawing.Node` names a part of a drawing, such as a chart's bar or a map's region, with bounds, a role, a label and a selected state. Assistive technology reads those parts as the canvas's children, and tests find them by label, text or test ID; a press on one is a press at its center.

```go
d.Node(core.CanvasNode{
	Bounds: core.Bounds{Origin: core.Point{X: x, Y: y}, Size: core.Size{Width: w, Height: h}},
	Role:   core.RoleButton,
	Label:  "Open the report",
	TestID: "report",
})
```

## Animated arguments

`Drawing.Bind` binds an argument of the last operation to an animated node, and the platform redraws only that part on its frame clock, without a commit. `core.ArgX`, `ArgY`, `ArgR`, `ArgA` and `ArgColor` name the arguments; macOS, iOS and Linux redraw only the changed area, while Windows records it again and Android and the web redraw the whole canvas.

```go
needle := core.NewAnimatedValue(cx, 0)
d.Save().Translate(cx, cy).Rotate(0).Bind(core.ArgA, needle).
	BeginPath().MoveTo(-14, -3).LineTo(80, 0).LineTo(-14, 3).ClosePath().Fill().Restore()
```

## Shaders

`core.Shader` fills its box with a fragment shader the GPU runs for every pixel. `core.CompileShader` takes a program written in Go's syntax with a `Fragment` function, and `core.CompileWGSL` takes WGSL. `Uniforms` sets the program's exported variables, and `Images` gives it up to four textures.

```go
var program = core.MustCompileShader(`
package main

var Tint vec4

func Fragment(dst vec4, pos vec2) vec4 {
	v := 0.5 + 0.5*sin(pos.x/40)
	return Tint * vec4(v, v, v, 1)
}
`)

core.Shader{
	Program:  program,
	Uniforms: core.Uniforms{"Tint": core.RGB(0x007aff)},
	Style:    core.Style{Height: core.Pt(220), CornerRadius: 16},
}
```

A Go shader receives the pixel's center in `dst` and the same point in points in `pos`, and has GLSL's built-in functions; `rectSize`, `pixelScale`, `image0At` to `image3At` and their sizes are NSGo's additions. A WGSL program writes `fn shade(uv: vec2f) -> vec4f` and reads `nsh.time` and `nsh.size`; `Paused` stops its clock. A platform without a GPU path for shaders draws nothing there.

`core.ShaderEffect` draws a view, a page or a dialog through a shader, and `core.TransitionShader` uses one for a page's transition. `core.Canvas` and `core.Shader` both take `Label` and their children draw over them.

## Next

- [Images and icons](images.md): the pictures a canvas draws.
- [Animation](animation.md): values a canvas binds.
- [Accessibility](accessibility.md): the parts a canvas exposes.

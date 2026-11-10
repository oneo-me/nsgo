---
title: 画布与着色器
description: 在引擎上绘制 2D 图形，并用 GPU 片元着色器填满一个视图。
group: 核心概念
order: 59
---

## 画布

`core.Canvas` 在引擎上记录 2D 绘图，再由平台播放，像网页的 `<canvas>`。`Draw` 会在它读取的 signal 或画布尺寸变化时再次运行，平台在主线程上绘制记录。

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

记录接口遵循网页的 `CanvasRenderingContext2D`：`Save` 和 `Restore`、变换、直线与曲线路径、矩形和圆弧、颜色和渐变填充与描边、裁剪、虚线、阴影、文本和图片。`core.ParsePath` 从 SVG 路径数据生成 `core.Path`，`FillFromView` 和 `StrokeFromView` 使用画布视图继承的 `Fill` 和 `Stroke` 样式，因此图表会跟随应用主题。

```go
d.SetFillGradient(core.CanvasLinear(0, 0, float32(size.Width), 0,
	core.GradientStop{Offset: 0, Color: core.RGB(0x4da3ff)},
	core.GradientStop{Offset: 1, Color: core.RGB(0x007aff)},
)).FillRect(0, 0, float32(size.Width), 40)
```

## 文本、图片与部件

`MeasureText`、`TextMetrics` 和 `SetTextAlign` 用平台的文本引擎排版，因此标签与应用其余部分使用相同的字体。`DrawImage`、`DrawImageIn` 和 `DrawImageFrom` 绘制图片，`OnPress` 可以用 `IsPointInPath` 和 `IsPointInStroke` 测试最后一次绘制。

`Drawing.Node` 给绘图的一部分命名，例如图表的柱或地图的区域，带边界、角色、标签和选中状态。辅助技术把这些部分当作画布的子元素，测试可以按标签、文本或 test ID 找到它们；按下其中一个就是按下它的中心。

```go
d.Node(core.CanvasNode{
	Bounds: core.Bounds{Origin: core.Point{X: x, Y: y}, Size: core.Size{Width: w, Height: h}},
	Role:   core.RoleButton,
	Label:  "打开报告",
	TestID: "report",
})
```

## 动画参数

`Drawing.Bind` 把最后一次操作的某个参数绑定到动画节点，平台在帧时钟上只重绘那部分，不需要提交。`core.ArgX`、`ArgY`、`ArgR`、`ArgA` 和 `ArgColor` 指定参数；macOS、iOS 和 Linux 只重绘变化的区域，Windows 重新记录，Android 和 Web 重绘整个画布。

```go
needle := core.NewAnimatedValue(cx, 0)
d.Save().Translate(cx, cy).Rotate(0).Bind(core.ArgA, needle).
	BeginPath().MoveTo(-14, -3).LineTo(80, 0).LineTo(-14, 3).ClosePath().Fill().Restore()
```

## 着色器

`core.Shader` 用 GPU 为每个像素运行的片元着色器填满盒子。`core.CompileShader` 接受用 Go 语法写的程序，其中有一个 `Fragment` 函数；`core.CompileWGSL` 接受 WGSL。`Uniforms` 设置程序导出的变量，`Images` 给它最多四张纹理。

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

Go 着色器在 `dst` 中收到像素中心，在 `pos` 中收到同一个点的点坐标，并可以使用 GLSL 的内建函数；`rectSize`、`pixelScale`、`image0At` 到 `image3At` 及其尺寸是 NSGo 的扩展。WGSL 程序写 `fn shade(uv: vec2f) -> vec4f`，读取 `nsh.time` 和 `nsh.size`；`Paused` 停止它的时钟。没有 GPU 着色器路径的平台在那里不绘制内容。

`core.ShaderEffect` 用一个着色器绘制视图、页面或对话框，`core.TransitionShader` 用它做页面转场。`core.Canvas` 和 `core.Shader` 都接受 `Label`，子元素会绘制在它们之上。

## 下一步

- [图片与图标](images.md)：画布可以绘制的图片。
- [动画](animation.md)：画布绑定的值。
- [无障碍](accessibility.md)：画布暴露的部件。

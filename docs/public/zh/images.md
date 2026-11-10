---
title: 图片与图标
description: 显示来自 URL、应用资源或字节的图片，并绘制 Lucide 图标和 SVG。
group: 核心概念
order: 54
---

## 一张图片

`core.Image` 显示位图或 SVG：一个 URL、一个文件、应用资源中的 `asset:` 路径，或应用持有的字节。平台在主线程之外加载、解码和缓存，主线程只绘制结果。

```go
core.Image{
	Source: core.Src("https://example.com/cover.jpg"),
	ResizeMode: core.Cover,
	Style: core.Style{Width: core.Pt(160), Height: core.Pt(100), CornerRadius: 12},
}
```

`core.Cover` 填满盒子并裁掉溢出部分；`core.Contain` 完整放下图片，`core.Center` 保持自然尺寸，`core.ScaleDown` 在图片大于盒子时缩小。`BlurRadius` 按点模糊，`TintColor` 重新给不透明像素上色，`ObjectPosition` 决定图片在盒子中的位置。

`Source` 声明布局使用的尺寸：知道一边可以推出另一边，两边都不知道时图片在样式给出尺寸前是 0×0。`core.ImageSource{Data: data, Size: core.Size{Width: 24, Height: 24}}` 显示用 `go:embed` 嵌入的字节，`Size` 也给 SVG 一个固有尺寸。

## 资源与像素密度

`resources/` 下的文件随应用一起发布。`core.Src("asset:images/cover.png")` 加载其中一个，`Scales: core.ScalesOf(1, 2, 3)` 按屏幕像素比选择 `cover.png`、`cover@2x.png` 或 `cover@3x.png`。

```go
core.Image{Source: core.ImageSource{
	URI:    "asset:images/cover.png",
	Scales: core.ScalesOf(1, 2, 3),
}}
```

`Placeholder` 在图片加载时显示另一个来源，`FadeDuration` 让加载好的图片在其上淡入；0 由平台决定，纳秒则立即显示。`OnLoadStart`、`OnProgress`、`OnLoad`、`OnError` 和 `OnLoadEnd` 跟随加载过程。`cx.ImageSize` 不显示图片就能读取它的像素尺寸，`cx.PrefetchImage` 把它下载到缓存。原生后端会把远程图片保存在磁盘缓存中。

## 动图与矢量图

GIF 和 WebP 动画在视图中播放，每一帧按自己的延迟显示，并按文件中的次数循环。SVG 文档作为图片加载，在任何尺寸下都保持清晰；单色 SVG 可以用 `TintColor` 着色。

## 图标

`core.Icon` 用继承的文本颜色绘制 SVG，默认和字号一样高，除非样式给出尺寸。它的盒子保持 SVG 的宽高比，因此方形盒子不会拉伸图标。

```go
core.Icon{SVG: lucide.Search, Style: core.Style{FontSize: 20}, Label: "搜索"}
```

`icons/lucide` 为每个 Lucide 图标提供一个访问函数，名称采用 PascalCase（`lucide.ArrowUpRight`）。除非 `Label` 命名它，否则图标对辅助技术只是装饰。`ui.Icon` 使用 `ui` 的默认值，`core.SVG` 和 `core.SVGView` 绘制应用自己构建的路径。

## 带样式的图片

`ui.Image` 是放在圆角容器中的图片，加载时显示占位内容，失败时显示一个淡淡的图片图标。它保持来源的宽高比，没有宽高比时为 3:2，除非样式同时指定了宽高。

```go
ui.Image{
	Source:      core.Src("asset:images/cover.png"),
	Label:       "封面",
	ResizeMode:  core.Cover,
	AspectRatio: 16.0 / 9.0,
	Placeholder: ui.Skeleton{Style: core.Style{Height: core.Pt(180)}},
}
```

## 下一步

- [文本](text.md)：把图片作为 span 放进一行文字。
- [样式与主题](styles.md)：圆角、阴影和着色。
- [列表与滚动](lists.md)：会回收的图片行。

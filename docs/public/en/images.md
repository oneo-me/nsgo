---
title: Images and icons
description: Show a picture from a URL, the app's resources or bytes, and draw Lucide icons and SVGs.
group: Building UI
order: 14
---

## A picture

`core.Image` shows a bitmap or an SVG: a URL, a file, an `asset:` path in the app's resources, or bytes the app holds. The platform loads, decodes and caches it off the main thread, and the main thread draws the result.

```go
core.Image{
	Source: core.Src("https://example.com/cover.jpg"),
	ResizeMode: core.Cover,
	Style: core.Style{Width: core.Pt(160), Height: core.Pt(100), CornerRadius: 12},
}
```

`core.Cover` fills the box and crops what overflows; `core.Contain` fits the whole picture, `core.Center` keeps its natural size, and `core.ScaleDown` keeps it unless it is larger than the box. `BlurRadius` blurs the picture in points, `TintColor` recolors its opaque pixels, and `ObjectPosition` places it in the box.

`Source` declares the size layout uses: one dimension derives the other, and an image without one is 0 by 0 until its style sizes it. `core.ImageSource{Data: data, Size: core.Size{Width: 24, Height: 24}}` shows bytes embedded with `go:embed`, and `Size` gives an SVG its intrinsic size.

## Resources and scales

Files under `resources/` ship with the app. `core.Src("asset:images/cover.png")` loads one, and `Scales: core.ScalesOf(1, 2, 3)` picks `cover.png`, `cover@2x.png` or `cover@3x.png` for the screen's pixel ratio.

```go
core.Image{Source: core.ImageSource{
	URI:    "asset:images/cover.png",
	Scales: core.ScalesOf(1, 2, 3),
}}
```

`Placeholder` shows a source while the picture loads, and `FadeDuration` fades the loaded picture in over it; 0 lets the platform choose, and a nanosecond shows it at once. `OnLoadStart`, `OnProgress`, `OnLoad`, `OnError` and `OnLoadEnd` follow the load. `cx.ImageSize` reads a source's size in pixels without showing it, and `cx.PrefetchImage` downloads one into the cache. Native backends keep remote images in a disk cache of their own.

## Animated and vector images

GIF and WebP animations play in the view, each frame for its own delay, and loop as the file says. SVG documents load as images and stay sharp at any size; a one-color SVG can be tinted with `TintColor`.

## Icons

`core.Icon` draws an SVG in the text color it inherits, as high as the font size unless its style sizes it. Its box keeps the SVG's aspect ratio, so a square box does not stretch it.

```go
core.Icon{SVG: lucide.Search, Style: core.Style{FontSize: 20}, Label: "Search"}
```

`icons/lucide` has an accessor per Lucide icon, named in PascalCase (`lucide.ArrowUpRight`). An icon is decoration to assistive technology unless `Label` names it. `ui.Icon` is the same with `ui`'s defaults, and `core.SVG` and `core.SVGView` draw paths the app builds itself.

## The styled image

`ui.Image` is a picture in a rounded well, with a placeholder while it loads and a faint picture icon when it fails. It keeps the source's aspect ratio, or 3:2 without one, unless its style sizes both sides.

```go
ui.Image{
	Source:      core.Src("asset:images/cover.png"),
	Label:       "Cover",
	ResizeMode:  core.Cover,
	AspectRatio: 16.0 / 9.0,
	Placeholder: ui.Skeleton{Style: core.Style{Height: core.Pt(180)}},
}
```

## Next

- [Text](text.md): images inside a line, as spans.
- [Styles and themes](styles.md): corners, shadows and tints.
- [Lists and scrolling](lists.md): rows of pictures that recycle.

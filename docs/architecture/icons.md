# Icons

`core.Icon` draws an SVG as an icon in the text color it inherits, and `icons/lucide` is the bundled Lucide pack. Read this before changing SVG parsing, the icon element or the generated pack.

An icon is a vector element, not an image: `core.SVG` turns a document into canvas operations, and every backend already plays those, so icons need no rasterizer, no per-platform SVG engine and no pixel-density variants. SVG files loaded as an `Image` take the image path instead, as in NSH: `internal/platform/imagedecode` plays `core.SVG`'s operations into pixels ([images](images.md)).

## SVG documents

`ParseSVG` reads static drawings, icons, logos and illustrations:

| Read | Not read |
| --- | --- |
| `path`, `rect`, `circle`, `ellipse`, `line`, `polyline`, `polygon` inside `svg`, `g`, `a`, `switch` and `use` (of shapes, groups and symbols) | Text, images, patterns, masks, filters, markers, animations, nested viewports' `viewBox`es |
| `fill`, `stroke`, `stroke-width`, `stroke-linecap`, `stroke-linejoin`, `stroke-dasharray`, `stroke-dashoffset`, `fill-rule`, `clip-rule`, `fill-opacity`, `stroke-opacity`, `color`, `visibility`, inherited through groups; `opacity`, `display="none"`, `transform` | `stroke-miterlimit`, `paint-order`, blend modes |
| `none`, `currentColor`, hex, `rgb()`, `rgba()`, `hsl()`, `hsla()`, CSS names, `transparent`; `url(#id)` with a fallback | `var()`, `currentColor` in a gradient stop (black) |
| Linear and radial gradients with `href` inheritance, both units, `gradientTransform` and spread methods; clip paths in user space units | Clip paths in `objectBoundingBox` units; a focal point outside its circle moves onto it, as in SVG 1.1 |
| Presentation attributes, the `style` attribute, and style sheets' type, class and id selectors | Other selectors, at-rules |

Parsing reads the document into a tree first, so references may point forward, then records `DrawOp`s in the `viewBox` with the color zero standing for `currentColor`. `SVG.Draw` substitutes the icon's color, scales the document uniformly to fit its box, centers it and leaves it as one immutable `Drawing` the platform replays. A non-square box keeps the aspect ratio.

- Transforms are baked into the coordinates: rects, rounded rects and circles stay what they are under translations and scales that keep them aligned, and become curves otherwise; stroke widths scale by the square root of the transform's determinant.
- Opacities multiply into each paint's alpha, so overlapping shapes in a translucent group show through each other. `currentColor` keeps the color zero and fades with `DrawAlpha` in a saved state.
- A gradient paint records `DrawFillGradient` or `DrawStrokeGradient` with a `core.Gradient` already in the shape's coordinates, after its mean color as a plain fill or stroke color. Players that do not paint gradients use that color; Android's runtime paints them, as the image rasterizer does.
- A clip path records `DrawClip` in a saved state, its shapes merged into one path; several shapes clip by the nonzero rule.
- Path data supports every SVG 1.1 command; elliptical arcs become cubic curves of at most a quarter turn each.
- A rounded `rect` takes elliptical corner radii; only equal radii become one `DrawRoundRect`.
- Unknown elements are skipped, an unread paint keeps the inherited one, and a reference to no gradient paints its fallback, or nothing.
- Nesting deeper than 256 elements fails, and so does a document whose `use` elements draw over a million elements or that records over two million operations; a `use` never draws an element it is inside of.

## Icon element

`core.Icon` mounts as a `KindCanvas` view, so no backend knows about icons. It takes the text color of its ancestors and, without a style size, is as high as the inherited font size, as wide as the SVG's aspect ratio makes it. `Measure` sizes it like an image, and `SVG.Draw` fits it into the result.

- Each commit's style resolution writes the inherited color into the icon's signal; a changed color redraws without a relayout.
- `SVGFunc` re-reads a document from a signal, as `Image.SourceFunc` does, so a list's cell follows its row without being rebuilt. A document whose intrinsic size changes marks the icon for layout.
- `SVGView` is the same element sized by the document's `width` and `height`, else its `viewBox`, for an SVG drawn as authored, as NSH's `svg()`.
- `Label` gives the icon the image role and a name for assistive technology; without it the icon is a decoration that technology skips.

## Lucide pack

`icons/lucide` has one accessor per icon of a pinned Lucide release. `upstream.json` records the repository, the full commit and a digest of the license and icons. Each accessor parses its document on first use and returns the same `*core.SVG` afterwards; a zero-value cache keeps unused icons out of package initializers, so a binary carries only the icons it uses.

There is no runtime registry. The exhaustive catalog lives in `icons_gen_test.go`, so the package carries only what an app imports.

Regenerate from the repository root with a clean Lucide checkout at the commit:

```sh
git -C /path/to/lucide checkout <40-character-commit>
go run ./tools/lucidegen -upstream /path/to/lucide -commit <40-character-commit>
```

- The generator requires a clean checkout at exactly that commit, parses every icon with `core.ParseSVG` and rejects any icon that is not 24 by 24.
- It writes `icons/lucide/icons_gen.go`, `icons_gen_test.go`, `LICENSE` and `upstream.json`.
- It reports accessors added and removed. Review removals and renames before accepting an update, since they change the public API.
- `-check` verifies the files without writing them.
- Names are upstream kebab-case in PascalCase; upstream aliases and lab icons are excluded.

The pack keeps Lucide's ISC license and Feather's MIT notice in `icons/lucide/LICENSE`; `NOTICE` records the attribution. Never edit the generated SVG strings: fidelity to upstream is the point.

## Verification

`go test ./internal/driver -run SVG` covers the parser, the path grammar and fitting; `go test ./internal/platform/imagedecode` checks rasterized SVG pixels, decoded frames and the bounds. `go test ./icons/lucide` parses every icon, checks its size and that accessors share their document.

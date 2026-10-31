# Images

`Image` shows a picture the platform loads: a URL, a file, an `asset:` path in the app's resources, or bytes the app holds. Bitmaps, animated GIF and WebP files and SVG documents all load as images, as in NSH's `docs/dev/images.md`. An `asset:` source lists the `@Nx` variants it ships in `Scales`, and the core sends the one for the pixel ratio, as NSH's does; without `Scales` the plain file loads. Read this before changing how a backend loads, decodes or plays images.

## Fit, position, blur and fade

- `ResizeMode.Fit` is NSH's `ResizeMode::fit`: it sizes the natural size by the mode (`Center` keeps it, as CSS's `none`; `ScaleDown` shrinks a larger image, as CSS's `scale-down`) and places it at the style's `ObjectPosition`.
- Every backend draws at that rectangle, clipped by the view; Android's Java runtime repeats it in `ImageNode.fit`.
- The natural size is the size the source declares, an SVG's intrinsic size, or the pixels over a name's `@2x` density. Android takes its drawable's intrinsic size and ignores a declared size.
- The position is the image's, not the view's: it travels in `ImageConfig.Position`, and a change sends `ItemImage` again, never `ItemViewStyle`. Android receives each axis as a fraction of the free space plus points, which any position resolves to.
- Apple frames a picture layer at the fitted rectangle inside the clipping layer, the tint and its mask inside it; the web sets the background's or mask's size and position in pixels.
- `BlurRadius` is in points: Core Image on Apple, CSS's `blur()` on the web, `RenderEffect` on Android 12 and newer, before it a stack blur of the image shrunk to its drawn size, a Direct2D Gaussian blur on Windows and a GSK blur on Linux.
- Windows and Linux draw a blurred image enlarged under a clip to its own rectangle, so its edges stay opaque.
- A source that loads fades in over its placeholder for `FadeDuration`: 0 by default, as NSH's iOS, except Android's 300 ms. Windows and Linux fade on their frame clocks, even when the bytes came from the cache.
- Videos fit with `Fit` on Windows and Android; Apple's `AVPlayerLayer` and GTK's `GtkPicture` have no natural-size fit, so `Center` there shrinks as `ScaleDown` does, and Android's video shows both as `Contain`.

## Size and prefetch

`Scope.ImageSize` and `Scope.PrefetchImage` are NSH's `Image.getSize` and `Image.prefetch`: `ItemImageInfo` carries the source, its `asset:` variant picked as for views, and the platform answers with `EventImageInfo`. The backends that load in Go call `imageload.Info`, so the bytes stay in `imageload`'s 32 MiB cache for the views that show them next, and read the size from the header (`imagedecode.Size`; ImageIO on Apple). Android answers in its Go process. The web loads an `<img>` outside the page and keeps the bytes in the browser's cache. The size is the one `OnLoad` reports.

## Disk cache

Every native backend loads `http(s)` images through `imageload`, which keeps their bytes in a disk cache under the app's cache directory (`imageload.UseDiskCache`, 256 MiB), as NSH's `DiskCache`. The web leaves caching to the browser.

- The policy is RFC 9111's for a private cache, simplified as NSH's: `200` and `203` are kept unless `no-store`, `Vary: *` or over an eighth of the cache.
- A copy is fresh for `max-age`, else `Expires` less `Date`, else a tenth of the time since `Last-Modified`, at most 7 days, less its age; `no-cache` or no freshness makes it stale at once.
- A stale copy is revalidated with its ETag and `Last-Modified`: `304` refreshes its headers, `4xx` drops it, and a failed request or a `5xx` serves it unless `must-revalidate` or `no-cache`.
- Entries are keyed by URL. Each is one file named by a digest of it, with a CRC, written under a temporary name and renamed, so every process sees a whole entry or none; one that does not read back whole is deleted.
- The least recently used go first: a hit refreshes a file's time at most every 10 minutes, and past the limit a scan deletes the oldest down to three quarters, and temporary files over an hour old.

## Decoding

- Formats are sniffed from the bytes, never from names or types: `imagedecode.Sniff` finds an SVG after a byte order mark, an XML declaration, comments and a doctype, or inside gzip.
- At most as many images as processors decode at once, each in a slot of `imagedecode.Wait`, as NSH's decode on a dispatch queue the system sizes; Android's Java decodes bitmaps one at a time. A load cancelled while it waits never decodes, so a fast scroll down a grid of pictures decodes about the visible ones and stays near its resting footprint.
- `OnLoad` reports the source's size in pixels, whatever size a view decodes it at, and an SVG's intrinsic size in whole points.
- An animated image plays on its own in each view, with each frame's delay (10 ms or less shows for 100 ms, as in browsers) and its loop count. A tinted one shows its first frame, as NSH's does, and so does a blurred one on Apple platforms.
- `internal/platform/imagedecode` decodes in pure Go for the backends without a decoder of their own: PNG, JPEG, every frame of GIF and WebP animations (at most 600 frames and 48 MiB of them), and SVG through its own rasterizer within NSH's bounds ([icons](icons.md#svg-documents)).

| Target | Bitmaps and animations | SVG |
| --- | --- | --- |
| Apple | ImageIO, at the view's pixel size; GIF, WebP and APNG frames play as a discrete keyframe animation of the layer's contents | `imagedecode`, its shorter side as long as the view's longer side in pixels |
| Web | `<img>` | `<img>` from an `image/svg+xml` blob |
| Windows, Linux | `imagedecode`; frames on the backend's frame clock | `imagedecode`, 4 pixels per point, at most 2048 a side |
| Android | `ImageDecoder`, with `AnimatedImageDrawable` for animations | Rasterized by `imagedecode` in the Go process at the view's size when it loads, and sent as a bitmap |

## SVG drawing

`imagedecode` draws an SVG at the size NSH's `svg` module would: covering a view's pixel size, at its intrinsic size times a scale, or oversampled 4 times to at most 2048 pixels a side, within 8192 a side and 16 Mi pixels.

Its scanline rasterizer computes exact coverage across each row with 16 samples down it, flattens curves within 0.05 pixels, strokes with caps, joins and a miter limit of 4 as a union of convex pieces, and paints gradients through a 1024-color table. A document is at most 16 MiB, an `.svgz` once inflated, and drawing one is bounded by edge work, so a small file cannot ask for minutes.

## Verification

`go test ./internal/platform/imageload` checks the disk cache's policy against a test server. `go test ./internal/platform/imagedecode` checks decoded frames, rasterized pixels and the bounds; the Windows and Linux native tests check animated GIFs and SVG pixels in a window. The Windows and Linux native tests also check a fit at a position, `Center`'s natural size, blur, fade and `ImageSize` in a window. The gallery's Images page shows every resize mode, blur, placeholders and fades, `ImageSize`, `PrefetchImage` and progress, an animated GIF and WebP, and SVG, plain and tinted.

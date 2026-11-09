# WeChat mini programs

The `miniprogram` target runs a Go app in a WeChat mini program, as NSH's community platform `nsh-platform-miniprogram` runs NSH's Rust apps. Read this before changing `internal/platform/miniprogram`, its runtime in `cmd/nsgo/miniprogram` or how the tool builds it.

## Layers

A mini program has two layers: the logic layer runs JavaScript without a DOM, and the render layer, a web view, draws the page's WXML from the data the logic layer sets with `setData`. NSGo runs entirely in the logic layer:

```text
logic layer
  app.wasm.br           The engine and the app, Go for js/wasm, tagged nsgominiprogram
                        and nsgo_noshader, loaded with WXWebAssembly
  nsgo/wasm_exec.js     Go's own, as a function of the globals it reaches
  nsgo/runtime.js       The bridge (nsgoMP), the offscreen canvas text is measured on,
                        the device, and the events of the page
  nsgo/tree.js          The views as plain objects; each turn's batches become the
                        fewest setData paths
render layer
  pages/index           The root view, then the modals, and a page container that
                        takes the back gesture while back goes through the app
  components/nsgo-node  A view at its frame, with its style, runs, picture, image or
                        field, and its children: itself again
```

- `mount.go` turns each batch into operations (its comment lists them) and `touch.go` the render layer's touches into presses, pans, pinches and `EventTouch`. Both are plain Go over a `host`, tested on any machine; `miniprogram.go` and `services.go` are the bridge.
- Frames are placed in the parent's padding box, as CSS places absolute children, so a child's frame is sent less its parent's border.
- Text is prepared, measured and laid out in Go (`text.go`): widths come from the logic layer's canvas, cached by font and text; lines break greedily after spaces and around CJK characters, spans keep their styles on one baseline, and attachments take their placeholders' places. The render layer draws each run with `white-space: pre` where it was placed, so both layers name their fonts rather than `sans-serif`, which they resolve differently.
- Canvases are played into one offscreen canvas with `canvas2d`, the web's player, and shown as pictures, which move and scroll as views do.
- Animated values run on the engine's `Animator`, frames timed by the runtime, and go to the render layer as an inline style over the view's. Keyframes and modal presentations run in the render layer through the component's `animate`, with no work per frame.
- Styles are the web's CSS, written by `webstyle`.

## Input

Taps go to the innermost view that takes them (`catchtap`), presses going down and up to the innermost that follows them (`mut-bind`), and touches to every view that follows them: the runtime numbers each touch event, so the views it bubbles through add nothing, and a finger belongs to the view it went down on. A pan that fails on vertical drags leaves them to the scroll views around it; others capture the drags they start (`catchtouchmove`), as a mini program cannot decide on a drag as it moves.

Text fields are WeChat's `input` and `textarea` in their content box, which leave the page in place (`adjust-position`) and report the keyboard's height to the environment instead. Scroll views are `scroll-view`s in enhanced mode, reporting drags, their offsets and an end 150 ms after the last.

## Services

Alerts are WeChat's modal or action sheet, the clipboard holds text, storage is the mini program's under the app's prefix, photos come from `chooseMedia` and files from a chat (`chooseMessageFile`). A link goes on the clipboard, as a mini program opens no browser. Requests WeChat has no API for are answered as failed.

## The package

`WXWebAssembly` loads only files of the code package, `.wasm.br` among them, and WeChat holds each package to 2 MB, so the module goes in a subpackage of its own, which the page loads before it, and its size decides which apps fit:

- `nsgo_noshader` leaves the shader compilers out of `core` (`shader_nocompile.go`), as the render layer draws no shader: programs compile to none and take any uniform.
- The module is built with `GOEXPERIMENT=nojsonv2`, encoding/json's smaller first implementation, and again without inlining when it is still too large.
- TSX does not run: a mini program evaluates no code it did not ship, which the page's engine needs, and goja does not fit. The tool refuses apps that link `tsx`.

`examples/embedded` compresses to 1.9 MB. NSH's gallery in Go does not fit.

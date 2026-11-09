# Shaders

`core.Shader` fills its box with a fragment shader the GPU runs for each pixel; `ShaderEffect`, `ShaderTransition`, `ScreenEffect`, `ScreenTransition` and a page's or a dialog's `TransitionShader` draw views through one, from pictures the platform takes of them. Go apps write shaders in Ebitengine's Kage or in WGSL, TSX apps in WGSL, as NSH's. What animates runs on the platform's frame clock. Read this before changing `core/shader.go`, `core/shadereffect.go`, `internal/shader`, `internal/shaderir` or a backend's shader view or effects.

## Languages

Both front ends check a program once and hand the platforms `shaderir`, so a backend links only the writer for its GPU (MSL, GLSL ES 3.00, HLSL), and an app that compiles no shader links no compiler.

- **Kage** (`core.CompileShader`, `internal/shader`): Go's syntax with GLSL's types, a fork of Ebitengine's compiler whose files keep its notices. The entry point is `Fragment(dst vec4, pos vec2) vec4`: `dst` in the view's pixels from its top left corner, `pos` in points, the result premultiplied. `image0At` to `image3At` read `Shader.Images` by pixel.
- **WGSL** (`core.CompileWGSL`, `internal/shader/wgsl`): NSH's contract, `fn shade(uv: vec2f) -> vec4f` returning straight sRGB, with `nsh.size`, `time`, `scale`, `progress` and `overflow`, `struct Uniforms` read as `uniforms`, and `sample_content`, `sample_backdrop`, `sample_from` and `sample_to`. Its limits are NSH's: 64 KiB of source, no entry points or resources of its own, uniforms of `f32`, `i32`, `u32`, `vec2f` to `vec4f` and `array<vec4f, N>` in 4096 bytes. Errors carry the line and column of the app's source; a source compiles once for the process (64 kept).

The WGSL front end is NSGo's own, not naga: a lexer, a parser and a checker that writes `shaderir` as it checks, with WGSL's abstract numbers and constant folding.

- `vec<bool>` is a float vector of 0 and 1 in the IR, which has no boolean vectors; component-wise comparisons, `select`, `any` and `all` work on it.
- A `for` loop over a counter the body does not assign, to a bound by a constant step, is the IR's loop; `while`, `loop` with `continuing` and `break if`, and other `for` loops are a loop only `break` ends, with a one-pass loop around a body whose `continue` must still run `continuing`. A `switch` is an `if` chain, in a one-pass loop when a case breaks.
- Dynamic indices are clamped into range, as WGSL's runtimes read them. Float `%` truncates, as WGSL's does.
- The built-ins (`wgsl.go`) are WGSL themselves, compiled after the app's source with two intrinsics: a picture's texel and its size. A picture is read between its pixels from four texel reads, so no backend needs a sampler.
- Not supported yet: `var<private>`, structs other than `Uniforms`, non-square matrices, `f16`, pointers and the bit-counting, packing and `modf`/`frexp` built-ins.

The IR gained what WGSL needs and Kage does not name: `trunc`, `roundEven`, `sinh`, `cosh`, `tanh` and the float bitcasts, which the writers map to their languages (MSL's `as_type` through overloads in its prelude).

## Uniforms

The core flattens `Uniforms` and `UniformsFunc` into lanes in declaration order (a vector's components, a matrix's columns, an array's elements), as `driver.Shader.Lanes`; each backend maps lane `i` to its buffer layout with `UniformLanes` and writes it with `shaderir.PutLanes`. Before them come the bridge's lanes, which platforms set: the surface's size in points, its scale and each texture's size. A WGSL program's first own lanes are its built-ins (`driver.ShaderBuiltinLanes`): the time, the progress and the overflow.

- A lane holding an `AnimatedNode` goes in `driver.Shader.Nodes`; a color node fills a color's lanes, one channel each (`driver.Channel`). The backend binds them with `Animator.BindShader`, reads them with `ShaderLanes` and draws again only when `ChangedShaders` reports a change. Android's Java animator binds numbers only.
- `nsh.time` is a node the core runs on the platform's frame clock from when the shader appears, an hour a run, held while `Paused`; only a program that reads it gets one, so only those draw every frame.
- `UniformsFunc` changes cost a commit and an `ItemShader`, so a value that changes every frame belongs in a node.

## Effects and transitions

`driver.Shader.Role` says what a shader draws: a shader view's fill under its children, an effect or a transition over a view's children and in their place, the same over the whole surface (screen roles, on a view of no size), or a change of pages or a dialog (`ShaderNav`). `Pictures` says which pictures the program reads, which only those the platform takes.

- **Effects** picture the children again after a batch that changed them (`ItemShaderInput`, which the core sends for an effect with an item of the batch in its subtree, and for screen shaders and backdrop readers on any change) and after a frame in which anything animated, at most once a frame.
- **Transitions** run when `Value` changes after its first: the core starts `nsh.progress`'s run and puts `ItemShaderTransition` first in the batch, so the platform pictures what shows before applying the rest; the shader draws while `Running`, the live side pictured each frame, and its end shows the children as they are. Without a `Program` a transition cross-fades (`wgsl.Crossfade`).
- **Pages and dialogs**: as a stack's top screen changes, the core runs the pushed or popped screen's `TransitionShader` on the stack's node; a `Modal` with one runs it on its host as it is mounted and as it goes. `Backwards` makes the first picture the upper side, so one program draws both ways. Under less motion nothing runs and the page's own `Animation` does; platforms that run the shader skip their own motion for the change.
- The children keep taking input under an effect: platforms hide them visually and hit-test them as before.

## Platforms

Each backend draws a shader view into a surface of its own below the view's children when the batch or the frame that changed it is applied, or an image it reads arrives, never on a timer of its own. The main thread only writes the uniforms and never waits for the GPU.

| Target | Fills with | Pictures for effects | Over the children |
| --- | --- | --- | --- |
| macOS, iOS | A `CAMetalLayer` drawn by a render goroutine of its own (`apple/shader.go`); MSL compiled once per program on a goroutine | `renderInContext:` of a container holding the children, mask lifted, into a kept bitmap and its Metal texture (`apple/picture.go`); the backdrop: the root with the view's layer hidden | A Metal layer at the top of the host; the container masked with an empty layer |
| Web | A `<canvas>` with its own WebGL 2 context, once an animation frame at most | `paintTree` of the DOM into a 2D canvas: backgrounds, rounded corners, uniform borders, images, canvases, text runs line by line, clipping and opacity; no transforms, gradients or shadows, nor images from other origins without CORS | A `<canvas>` over the children; the host's `nsgo-fx` class puts them at opacity 0 |
| Windows | A texture on Direct2D's Direct3D 11 device, drawn as a bitmap; HLSL with `D3DCompile` | The painter draws the children into a Direct2D bitmap over a texture the program reads with no copy; frames are whole while an effect shows | The program's texture drawn in the children's place |
| Linux | A `GtkGLArea`, OpenGL 3.3 or ES 3.0 as GTK's context has | The children's render nodes rendered by the window's GSK renderer into a texture and downloaded | A `GtkGLArea` of the effect's own over them |
| Android | A `TextureView` drawn on one shared GL thread | — | — |

Android's runtime draws no effects yet: its backend leaves their items out, so children show as they are and pages and dialogs come with their own motion. The backdrop is pictured on macOS, iOS and the web only, and reads as transparent elsewhere. `nextDrawable` blocks until the display frees a drawable, so Apple's frames are encoded off the main thread. A machine without Metal, WebGL 2, Direct3D 11 or OpenGL 3.3/ES 3.0 leaves a shader view empty and shows an effect's children as they are. Mini programs draw no shader: their builds take the `nsgo_noshader` tag, which leaves the compilers out of `core` for the module's size, so every program compiles to an empty one that takes any uniform ([mini programs](miniprogram.md#the-package)).

## Design choices

| Choice | Rather than | Why |
| --- | --- | --- |
| WGSL checked into `shaderir` by a front end of NSGo's own | naga, or WGSL translated to Kage's source | No C or Rust; one IR and three writers serve both languages, and Kage's loops cannot say WGSL's |
| Pictures sampled between pixels from texel reads in the built-ins | A sampler bound on each backend | The writers' texture reads stay the one they have |
| Transitions run by a core node and started first in their batch | The platform's own clock for each change | The engine knows what changes; one protocol serves elements, pages and dialogs |
| Animated nodes in lanes, stepped by the platform's `Animator` | The app setting the time and the pointer each frame | A shader animates with no engine work per frame |
| A GPU surface per shader view; `nsgotest` models roles, lanes and starts | One renderer for the whole window | Platforms draw native views; the core's tests check lanes, not pixels |

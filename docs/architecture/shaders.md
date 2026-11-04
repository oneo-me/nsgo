# Shaders

`core.Shader` fills its box with a fragment shader the GPU runs for each pixel. NSH has no shaders; NSGo forks Ebitengine's Kage compiler, and keeps its rule that what animates runs on the platform's frame clock. Read this before changing `core/shader.go`, `internal/shader`, `internal/shaderir` or a backend's shader view.

## Language

Shaders are written in Ebitengine's Kage, Go's syntax with GLSL's types and built-ins, so an app keeps one source for every GPU API. `core.CompileShader` parses and checks it once and reports errors with the source's lines; the checked program travels to the platform as `shaderir`, so a backend links only the writer for its GPU, and an app that compiles no shader links no compiler.

- `internal/shader` is the front end and `internal/shaderir` the representation, with a backend each for MSL (`msl`), GLSL ES 3.00 (`glsl`) and HLSL (`hlsl`). Their files keep Ebitengine's notices and say what the fork changed.
- The bridge (`internal/shader/bridge.go`) appends what platforms set before the app's uniforms: the view's size in points (`rectSize`), its pixels a point (`pixelScale`) and, for a program that reads images, their sizes in pixels. The entry point is `Fragment(dst vec4, pos vec2) vec4`: `dst` in the view's pixels from its top left corner, `pos` the same point in points, and the result premultiplied.
- `image0At` to `image3At` read `Shader.Images` (or `ImagesFunc`) by pixel from the image's top left corner, premultiplied and transparent outside it, and `image0Size` to `image3Size` give their sizes. The compiler adds them up to the highest image a program calls, so its bridge lanes number `shader.BridgeLanes`: 3, plus 2 an image.
- Shaders fill only their view: no effects over other views and no backdrop.

## Uniforms

The core flattens `Shader.Uniforms` and `UniformsFunc` into lanes in declaration order (a vector's components, a matrix's columns, an array's elements), as `driver.Shader.Lanes`; each backend maps lane `i` to its buffer layout with `UniformLanes` and writes it with `shaderir.PutLanes`. An unknown name or a value with more lanes than its uniform panics into the nearest `Catch`.

- A lane holding an `AnimatedNode` goes in `driver.Shader.Nodes`. The backend binds them with `Animator.BindShader`, reads them with `ShaderLanes` and draws a view again only when `ChangedShaders` reports that a lane it reads changed; Android's Java animator binds them as it binds a canvas's arguments.
- A clock is a `Loop` of a `Timing`; quantizing it with `Sub(t, Mod(t, step))` redraws a slow drift less often than the display, as the gallery's signal field does.
- `UniformsFunc` changes cost a commit and an `ItemShader`, so a value that changes every frame belongs in a node.
- Input sets values at once: the gallery `Set`s the pointer with each move, and the backend draws the batch as it arrives. A spring started again on every move would not follow, since each run starts at the next frame.

## Platforms

Each backend draws a shader view into a surface of its own below the view's children when the batch or the frame that changed it is applied, or an image it reads arrives, never on a timer of its own. The main thread only writes the uniforms and never waits for the GPU. Images load as an image view's do, into textures sampled at their nearest pixel; until one arrives it reads as transparent.

| Target | Draws with | Compiles |
| --- | --- | --- |
| macOS, iOS | A `CAMetalLayer` in sRGB, a triangle covering it (`internal/platform/apple/shader.go`), drawn by a render goroutine of the view's own with the latest uniforms the main thread handed over | MSL, once per program for the process, on a goroutine; the view draws when it is ready |
| Web | A `<canvas>` with its own WebGL 2 context and a std140 uniform block, drawn in the next animation frame, once a frame at most | GLSL ES 3.00, once per view; the context is lost with the view, as browsers keep few |
| Windows | A texture of the view's own on Direct2D's Direct3D 11 device, which the painter draws as a bitmap | HLSL with `d3dcompiler_47.dll`'s `D3DCompile`, once per program |
| Linux | A `GtkGLArea` hosted like an input, with OpenGL 3.3 or ES 3.0 as GTK's context has, through the libepoxy GTK loads | GLSL, once per area, when it first draws |
| Android | A `TextureView` (`ShaderNode.java`) drawn with OpenGL ES 3.0 on one GL thread whose shared EGL context serves every shader view | GLSL ES 3.00 translated by Go, sent with the first item that draws a program and compiled once |

`nextDrawable` blocks until the display frees a drawable, a whole refresh once frames come faster than it shows them. Called on the main thread it took 15 ms of every 16.7 ms frame while the pointer moved, and dropped 38% of frames; on the render goroutine the main thread spends 0.35 ms a frame and drops none.

macOS skips views out of sight and draws them as they show, as it does content layers. A machine without Metal, WebGL 2, Direct3D 11, OpenGL 3.3 or OpenGL ES 3.0 leaves the view empty.

## Design choices

| Choice | Rather than | Why |
| --- | --- | --- |
| Animated nodes in lanes, stepped by the platform's `Animator` | The app setting the time and the pointer each frame | A shader animates with no engine work per frame |
| A GPU surface per shader view, empty without a GPU; `nsgotest` models it as a view of kind shader | One renderer for the whole window, with a CPU interpreter as fallback | Platforms draw native views; the core's tests check lanes, not pixels |
| Fills of the view, reading up to four images | Backdrops, layer effects, filled text and paths | What the gallery needs first; the bridge grows with them |

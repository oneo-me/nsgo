# Animation

Style transitions, animated values and layout animations run on the platform's main thread, stepped by its own frame clock, so a running animation costs the engine nothing per frame. They follow NSH's `docs/dev/animation.md`. Read this before changing `core/animated.go`, `internal/driver/animated.go`, `internal/driver/animator.go` or a backend's frame clock.

Layout runs in the engine, so the frame clock moves only what a platform draws without it: a transition of drawn properties and an animated value never change a view's frame. A view's size and position animate as a [layout animation](#layout-animation), of a whole commit or of one view's own changes.

`View.OnTransitionEnd` and `Overlay.OnTransitionEnd` run once a change to the view's style or classes has had its new `Transition`'s delay and duration after the commit that sends it, at once without a duration or under reduced motion; a later change starts the wait over. The core times it instead of hearing it from the platform, so it runs alike everywhere, whether or not anything moved.

## Frames

`Scope.RequestAnimationFrame(fn)` calls fn once, on the graph's goroutine, when the next display frame begins, with the frame's time from the graph clock, as timers do. stop cancels the wait, and a callback whose owner is disposed does not run; registering again from fn is the loop that runs every frame, as `requestAnimationFrame`'s does. `Scope.Animate` steps this way, so an app draws at the display's rate without a timer of its own.

The core holds the callbacks waiting for the next frame and tells the host when the first one appears (`RootOptions.RequestFrame`), once: the host calls `Scope.RunFrame` when that frame begins, and asks again when a callback waits for the next. The engine forwards the request to its platform (`engine.Platform.RequestFrame`) and runs the frame when the platform reports it (`Engine.Frame`), before committing what the callbacks changed.

Each target's frame source is its frame clock (below): macOS and iOS share their surface's display link with the native animations, the web uses `requestAnimationFrame`, Windows and Linux their frame timer, and Android the runtime's `Choreographer`, which reports each frame over the pipe while the engine waits for one.

Headless tests run frames as the manual clock advances: `nsgotest.App.Advance` moves the clock a frame at a time and runs a waiting frame at each, as NSH's clock with frames does.

## Animated values

The app builds a graph once: `AnimatedValue`s, nodes derived with `Interpolate` (numbers or colors, with React Native's extrapolation and easings), `Add`, `Sub`, `Mul`, `Div`, `Mod`, `DiffClamp` and `SnappingDiffClamp`, and animations that move values. `Style.Animated` binds a view's opacity, translation, scales, rotations about each axis, background, border color, shadow color, shadow radius, shadow opacity and text color to nodes, over its style, and a sticky list header's inset (`StickyInset`). A bound text color tints the whole drawn text, as NSH's does.

- `Timing`, `Spring` and `Decay` move one value; `Sequence`, `Parallel`, `Stagger`, `Delay` and `Loop` compose them. `Scope.StartAnimation` runs one, calls back once with whether it finished, and returns an `AnimationRun` whose `Stop` ends it where it is and `Reset` puts its values back where they were as it started. Each step of a composite starts at the exact end of the one before.
- Drivers are closed-form functions of the time since they started, so dropped frames change nothing: React Native's damped spring, and NSH's decay, which stops at or bounces off its bounds at the exact time it reaches them. `SampleEasing`, `SampleSpring` and `SampleDecay` return their curves from the same code, to draw them.
- A value's `Set`, `Stop`, or another animation of it ends its run unfinished; so does the owner going. `Read` asks the platform for a value.
- `AtFrameRate` caps how often a run moves its values, for what loops as long as work lasts: the animator holds it to the rate on every target but Android, whose Java animator ignores it, and macOS slows its display link to the fastest rate the runs ask for while nothing else needs the clock (a layout animation, a moving scroll view, a waiting frame, a selection's drag or the performance monitor), and applies batches as they land meanwhile rather than holding them for frames that far apart. iOS's link keeps the screen's rate.
- `ValueKeyframes` plays a value through stops as Web Animations does: offsets spread evenly when missing, an easing per segment, a delay, iterations and directions. It becomes a composite of timings, so the platforms learn nothing new; `StepsEasing` adds CSS's `steps()`.
- A `Motion` moves a view's opacity, translation, scale and rotation through keyframes together, from a `MotionSpec` or one of NSH's `MotionPreset`s (entrances, exits, attention seekers), landing at once under reduced motion.
- `SnappingDiffClamp` is a `DiffClamp` that, when the scroll view its input follows comes to rest, eases in 200 ms to whichever end is nearer, so a collapsing header is never left half hidden.
- `View.PanValues` has drags move values by their translation, `PinchValue` scales a value by a pinch and `RotationValue` turns one by a rotation, each from where the value was as the gesture began, so a gesture grabs a value an animation is moving (`Animator.Gesture`). A pan's start and end report the values in `PanEvent.Values`, to snap from. `Scroll.OffsetValues` follows a scroll view's offset. Each moves in the frame the pointer or the content does.
- Nodes also drive a `Shader`'s uniform lanes and a `Canvas`'s drawing arguments, which the backend draws again when one changes ([shaders](shaders.md), [canvas](canvas.md)).
- Values are released with the owner that made them; derived nodes with the owner running when they were made.
- `ProjectDecay` returns where a decay brings a value to rest, to choose the point a released drag snaps to, and `RubberBandClamp` resists a value past its bounds as a scroll view's edge does. Both are plain functions for handlers, not nodes.

The graph travels as `ItemAnimation`, after the batch's views. Each backend keeps a `driver.Animator`, applies the ops, steps it once a frame while a run is live and draws what `Changed` reports with `AnimatedValues.Apply` over the view's style. Finished runs and reads come back as `EventAnimation`.

| Target | Frame clock | Draws bound values with |
| --- | --- | --- |
| macOS | AppKit's `CADisplayLink` (macOS 14), else a 60 Hz timer | The layer's properties, without implicit animation |
| iOS | `CADisplayLink` at the screen's `maximumFramesPerSecond` | The layer's properties |
| Web | `requestAnimationFrame` | Inline styles |
| Windows, Linux | The backend's 16 ms frame timer | The next paint |
| Android | `Choreographer` | A Java port of the animator on the UI thread |

A display link runs only in the run loop mode object it is given: Foundation's `NSRunLoopCommonModes` (`apple.CommonModes`), not an equal string. A ProMotion iPhone grants links more than 60 Hz only to apps whose `Info.plist` sets `CADisableMinimumFrameDurationOnPhone`, which `nsgo` writes; iPads ignore it.

## Layout animation

`Scope.AnimateLayout` animates the next batch that changes anything, as React Native's `LayoutAnimation.configureNext`: the batch carries the `LayoutAnimation`, with React Native's presets as `LayoutEaseInEaseOutPreset`, `LayoutLinearPreset` and `LayoutSpringPreset`.

- Update: a view that existed before the batch moves and resizes from the frame it shows, mid-animation included, to its new one. Springs fit into the duration as UIKit's do.
- Create: the top view of each subtree the batch inserts fades in, or grows from its center.
- Delete: the top view of each subtree the batch deletes keeps drawing where it was while it fades or shrinks away, its ID already free for reuse. Views it holds go with it.
- A view whose style's `Transition` lists `TransitionLayout` (`transition-all`; TSX's `transitionProperty` `all`, `width`, `height`, `inset` and their kin) moves and resizes the same way, on its transition's timing and easing, when a change to its own style gives it a new frame. Its `ItemFrame` carries the `LayoutTransition`; a batch's `Update` wins. Its parent's resize alone moves it at once, as CSS transitions a percentage width only when the width changes. What it holds and its siblings take their new frames at once, so it suits a view nothing follows, such as a progress bar's indicator; a section that opens and pushes what is below it wants `AnimateLayout`.
- Reduced motion turns layout animations off. Web Animations play them on the web, springs sampled into CSS `linear()`.
- `LayoutKeyboard` is the software keyboard's curve, which a style's `EnvKeyboard` terms animate on: UIKit's as a Bézier, timed from the keyboard notification on iOS however late the batch arrives, and on Android the IME's `WindowInsetsAnimation`: its interpolator and duration, from where its motion already is.

## Where Go departs from NSH

| NSH | NSGo | Why |
| --- | --- | --- |
| `createAnimatedValue`, nodes as `Copy` handles | `*AnimatedValue` and `AnimatedNode` values, released with their owner | Go's garbage collector and owners |
| Optional velocities | `Velocity` 0 keeps the value's current velocity | Zero values are defaults |
| `Easing` as data decoded in Rust | `Easing` values with a `Kind` and `Mode`; zero is linear | Zero values are defaults |
| A transition's properties are drawn ones; `width` and the like are unknown | `TransitionLayout` moves the frame a view's own style change sets | A component animates its own size without animating the rest of the commit |
| No transition end | `OnTransitionEnd`, timed by the core | A popup leaves the tree once its exit transition is over |

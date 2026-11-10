---
title: Animation
description: Transitions, animated values, layout animation and motion presets that run on the platform's frame clock.
group: Building UI
order: 16
---

## Animated values

An animation runs on the platform's main thread, stepped by its own frame clock, so it costs the engine nothing per frame. An `AnimatedValue` is a number the platform moves; `Style.Animated` binds a view's opacity, translation, scale, rotation, background, border color, shadow and text color to it.

```go
fade := core.NewAnimatedValue(cx, 0)
cx.StartAnimation(core.Timing(fade, 1, core.TimingConfig{
	Duration: 300 * time.Millisecond,
	Easing:   core.EaseOut,
}), nil)

core.View{Style: core.Style{Animated: &core.AnimatedStyle{Opacity: fade}}}
```

`Timing`, `Spring` and `Decay` move one value, and `Sequence`, `Parallel`, `Stagger`, `Delay` and `Loop` compose them. `cx.StartAnimation` returns an `AnimationRun` whose `Stop` ends a run where it is and `Reset` puts its values back. A run starts from where its value is, and a new animation of a value ends the old one.

```go
scale := core.NewAnimatedValue(cx, 1)
ui.Spring(cx, scale, 1.1, core.SpringConfig{Stiffness: 220, Damping: 14})
```

Derived nodes follow a value: `core.Interpolate` maps numbers or colors with its own easing and extrapolation, and `core.Add`, `Sub`, `Mul`, `Div`, `Mod`, `DiffClamp` and `SnappingDiffClamp` compute other values from it. `ui.Lerp` is a short interpolation.

```go
angle := core.Interpolate(scroll, core.Interpolation{
	Input:  []float64{0, 240},
	Output: []float64{0, 90},
	Easing: core.EaseOut,
})
```

`Scope.RequestAnimationFrame` runs a function once at the next display frame, and `Scope.Animate` steps a duration on that clock. Both run on the graph's goroutine, so they can set signals.

## Transitions

A `Transition` animates a view's style or classes when they change, on the platform, with no work for the engine per frame. `Properties` says what moves; 0 means every property but the frame.

```go
core.View{Style: core.Style{
	Background:  core.RGB(0xffffff),
	CornerRadius: 12,
	Transition:  core.Transition{Duration: 200 * time.Millisecond, Properties: core.TransitionBackground | core.TransitionTransform},
	Transform:   core.Transform{Scale: 1},
	Hover:       &core.Style{Transform: core.Transform{Scale: 1.02}, Background: core.RGB(0xf5f5f5)},
}}
```

`TransitionLayout` moves and resizes a view from where it shows when its own style gives it a new frame. A layout animation animates a whole commit instead, as React Native's `LayoutAnimation` does: `cx.AnimateLayout(core.LayoutEaseInEaseOutPreset)` animates the next batch's moves, insertions and removals, and the presets cover easing, linear and spring.

```go
cx.AnimateLayout(core.LayoutEaseInEaseOutPreset)
open.Set(!open.Get())
```

`View.OnTransitionEnd` runs once a change has had its transition, whether or not anything moved. Reduced motion turns transitions and layout animations off, so respect the platform's setting by keeping the end state reachable.

## Motion

`core.Motion` moves one view's opacity, translation, scale and rotation through keyframes together, from a `MotionSpec` of the app's or a ready-made `MotionPreset`: entrances, exits, attention seekers and loops. Binding its style sets the view at rest; `Play` runs a spec, and `Exit` lets a view leave with its motion inside a `core.Presence`.

```go
enter := core.SlideInUp.Spec()
motion := core.NewMotion(cx, &enter)
motion.Exit(core.FadeOut.Spec())

core.View{Style: core.Style{Animated: motion.Style()}, Children: content}
```

`core.Presence{When, Then, Else}` keeps a branch until every exit it registered has finished, so a sheet or a toast leaves with its motion whatever removes it. `MotionPresets()` lists every preset and `Kind()` says whether it enters, exits, draws attention or loops.

## Keyframes

`KeyframeAnimation` repeats keyframes on a platform: opacity, scale, rotation and vertical translation at each offset, with a duration, delay, easing and iterations. `KeyframesSpin`, `KeyframesPing`, `KeyframesPulse` and `KeyframesBounce` are Tailwind's.

```go
core.View{Style: core.Style{Animation: core.KeyframeAnimation{
	Keyframes: core.KeyframesPulse,
	Duration:  2 * time.Second,
	Iterations: 0, // forever
}}}
```

`ValueKeyframes` moves an animated value through stops as Web Animations does, with offsets, a per-segment easing, a delay, iterations and a direction. Gestures can drive values too: `View.PanValues`, `PinchValue` and `RotationValue` move them on the main thread from where they were as the gesture began.

## Next

- [Styles and themes](styles.md): the properties a transition moves.
- [Lists and scrolling](lists.md): values that follow a scroll view.
- [Layout](layout.md): what animated frames are.

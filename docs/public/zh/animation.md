---
title: 动画
description: 过渡、动画值、布局动画和动效预设，全部运行在平台的帧时钟上。
group: 构建界面
order: 16
---

## 动画值

动画运行在平台主线程上，由平台自己的帧时钟推进，因此每帧都不消耗引擎。`AnimatedValue` 是平台移动的一个数字；`Style.Animated` 把视图的透明度、位移、缩放、旋转、背景、边框颜色、阴影和文本颜色绑定到它。

```go
fade := core.NewAnimatedValue(cx, 0)
cx.StartAnimation(core.Timing(fade, 1, core.TimingConfig{
	Duration: 300 * time.Millisecond,
	Easing:   core.EaseOut,
}), nil)

core.View{Style: core.Style{Animated: &core.AnimatedStyle{Opacity: fade}}}
```

`Timing`、`Spring` 和 `Decay` 移动一个值，`Sequence`、`Parallel`、`Stagger`、`Delay` 和 `Loop` 组合它们。`cx.StartAnimation` 返回 `AnimationRun`，`Stop` 让动画停在当前位置，`Reset` 把值放回开始处。动画从值的当前位置开始，同一个值的新动画会结束旧的。

```go
scale := core.NewAnimatedValue(cx, 1)
ui.Spring(cx, scale, 1.1, core.SpringConfig{Stiffness: 220, Damping: 14})
```

派生节点跟随一个值：`core.Interpolate` 按自己的缓动和外推映射数字或颜色，`core.Add`、`Sub`、`Mul`、`Div`、`Mod`、`DiffClamp` 和 `SnappingDiffClamp` 由它计算其他值。`ui.Lerp` 是简短的插值。

```go
angle := core.Interpolate(scroll, core.Interpolation{
	Input:  []float64{0, 240},
	Output: []float64{0, 90},
	Easing: core.EaseOut,
})
```

`Scope.RequestAnimationFrame` 在下一个显示帧运行一次函数，`Scope.Animate` 在该时钟上推进一段时间。两者都运行在响应式图的 goroutine 上，因此可以设置 signal。

## 过渡

`Transition` 在视图的样式或类名变化时，由平台播放过渡，每帧不需要引擎参与。`Properties` 指定哪些属性移动；为 0 时表示除尺寸位置外的所有属性。

```go
core.View{Style: core.Style{
	Background:  core.RGB(0xffffff),
	CornerRadius: 12,
	Transition:  core.Transition{Duration: 200 * time.Millisecond, Properties: core.TransitionBackground | core.TransitionTransform},
	Transform:   core.Transform{Scale: 1},
	Hover:       &core.Style{Transform: core.Transform{Scale: 1.02}, Background: core.RGB(0xf5f5f5)},
}}
```

`TransitionLayout` 在视图自己的样式给出新框架时，从它当前显示的位置移动和改变大小。布局动画则动画整个提交，像 React Native 的 `LayoutAnimation`：`cx.AnimateLayout(core.LayoutEaseInEaseOutPreset)` 动画下一批的移动、插入和删除，预设覆盖缓动、线性和弹簧。

```go
cx.AnimateLayout(core.LayoutEaseInEaseOutPreset)
open.Set(!open.Get())
```

`View.OnTransitionEnd` 在一次变化完成它的过渡后运行，无论是否有东西移动。减少动态效果会关闭过渡和布局动画，因此要让最终状态仍然可达。

## 动效

`core.Motion` 用关键帧同时移动一个视图的透明度、位移、缩放和旋转，关键帧来自应用自己的 `MotionSpec` 或现成的 `MotionPreset`：进入、退出、吸引注意和循环。绑定它的样式会把视图放在静止状态；`Play` 播放一个 spec，`Exit` 让视图在 `core.Presence` 中带着动效离开。

```go
enter := core.SlideInUp.Spec()
motion := core.NewMotion(cx, &enter)
motion.Exit(core.FadeOut.Spec())

core.View{Style: core.Style{Animated: motion.Style()}, Children: content}
```

`core.Presence{When, Then, Else}` 会保留一个分支，直到它注册的所有退出动画结束，因此 sheet 或 toast 无论被什么移除，都会带着动效离开。`MotionPresets()` 列出所有预设，`Kind()` 说明它是进入、退出、吸引注意还是循环。

## 关键帧

`KeyframeAnimation` 在平台上重复关键帧：每个偏移处的透明度、缩放、旋转和纵向位移，带时长、延迟、缓动和次数。`KeyframesSpin`、`KeyframesPing`、`KeyframesPulse` 和 `KeyframesBounce` 是 Tailwind 的。

```go
core.View{Style: core.Style{Animation: core.KeyframeAnimation{
	Keyframes:  core.KeyframesPulse,
	Duration:   2 * time.Second,
	Iterations: 0, // 永远
}}}
```

`ValueKeyframes` 像 Web Animations 一样让动画值经过多个停靠点，支持偏移、分段缓动、延迟、次数和方向。手势也可以驱动值：`View.PanValues`、`PinchValue` 和 `RotationValue` 在手势开始时从原值出发，在主线程上移动它们。

## 下一步

- [样式与主题](styles.md)：过渡移动的属性。
- [列表与滚动](lists.md)：跟随滚动视图的值。
- [布局](layout.md)：动画的框架是什么。

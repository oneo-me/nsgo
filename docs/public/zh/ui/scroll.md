---
title: 滚动
description: 平台的滚动视图，支持分页、吸附、缩放和下拉刷新。
group: 组件
order: 39
---

`core.Scroll` 是平台自己的滚动视图。它的子元素按纵向排开，设置 `Horizontal` 后按横向排开；它填满容器，内容在其中滚动。

```go preview=scroll/basic height=240
core.Scroll{Style: core.Style{FlexGrow: 1}, Children: []core.Node{
	core.View{Style: core.Style{Padding: core.All(16), Gap: 12}, Children: rows},
}}
```

内边距放在内容上，而不是滚动视图上，因此滚动条贴着边缘。除非样式另有设置，滚动视图会收缩以适应父容器。

## 行为

`Behavior` 决定滚动方式：`Paging`、`SnapToInterval` 或 `SnapToOffsets` 配合 `SnapToAlignment`、`DecelerationRate`、`NoBounces`、`ContentInset`，以及键盘规则。

```go preview=scroll/paging height=240
core.Scroll{
	Behavior: core.ScrollBehavior{
		Paging:                    true,
		KeyboardDismissMode:       core.KeyboardDismissOnDrag,
		KeyboardShouldPersistTaps: core.KeyboardTapsHandled,
	},
	Children: pages,
}
```

`HideIndicator` 隐藏平台滚动条，`Bar` 绘制应用自己的滚动条。`MinZoomScale` 和 `MaxZoomScale` 允许双指缩放；`NoPinchZoom` 忽略缩放。`CenterContent` 把小于视口的内容居中。

## 刷新与事件

`OnRefresh` 让视图可以下拉刷新；工作运行时设置 `Refreshing`。`PullValue` 跟随内容被拉动的距离，用于应用自己的指示器，`Behavior.RefreshIndicator` 设置它的高度。

```go
refreshing := core.NewSignal(cx, false)
core.Scroll{
	Refreshing: refreshing.Get,
	OnRefresh:  func() { refreshing.Set(true); reload(cx) },
	Children:   content,
}
```

`OnScroll` 每轮最多报告一次偏移，或按 `ScrollEventThrottle` 的间隔报告。`OnScrollDragStart`、`OnScrollDragEnd`、`OnMomentumStart`、`OnMomentumEnd` 和 `OnScrollEnd` 标出用户手势的起止。

`OffsetValues` 在主线程跟随偏移，用于没有每帧提交也能折叠的头部。`Ref` 用 `ScrollTo`、`ScrollToEnd` 和 `ZoomTo` 从代码滚动。

## 布局

`Horizontal` 横向滚动。`Behavior.ContentInset` 在内容周围留出随内容滚动的空间。要让页面避开安全区域，在内容上使用 `PaddingEnv`，不要放在滚动视图上。

## 无障碍

`Label` 命名滚动区域，`Role` 说明它是什么，例如 listbox。平台滚动手势、键盘滚动和屏幕阅读器滚动都会保留；`Busy` 报告仍在加载的视图。

## 平台

每个后端都使用平台的滚动视图，因此惯性、回弹、滚动条和触摸滚动都是原生的。边缘效果不同：macOS 和 iOS 回弹，Android 拉伸，Windows 和 Linux 停止，Web 跟随浏览器。`EdgeEffects` 和 `EdgeBar` 在应用需要时把栏放在内容上方。

## 相关

- [列表与滚动](../lists.md)：滚动和列表的完整指南。
- [列表](list.md)：会回收的行。
- [布局](../layout.md)：内边距和安全区域。
- gallery 的 Scroll 和 Lists 页面。

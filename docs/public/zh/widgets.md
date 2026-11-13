---
title: 小组件
description: 在 iPhone 和 iPad 的主屏幕与锁屏、Mac 的桌面上显示应用的视图。
group: 平台功能
order: 47
---

## 一种小组件

无论应用是否在运行，小组件都展示它的摘要：iPhone 和 iPad 的主屏幕与锁屏，以及 Mac 的桌面和通知中心。一种小组件是系统小组件库中的一项，有它支持的尺寸：

```yaml
widgets:
  - kind: counter
    name: Counter                 # 显示在系统的小组件库中
    description: Shows the count.
    families: [small, medium, circular]  # 另有 large、extra_large、rectangular 和 inline
# apple: { app_group: group.dev.example.hello }  # iOS 的 App Group，默认为 group.<id>
```

小组件由小组件扩展绘制，`build` 和 `dev` 用 Xcode 的 Swift 编译器把它编进应用（[构建与发布](building.md#小组件与实时活动)）。扩展不运行应用的任何代码，因此小组件显示应用最后写入的内容，直到它再次写入。

## 时间线

`widget.Update` 写入一种小组件从现在起显示的内容，由若干条目组成时间线：系统显示日期已过的最后一条，之后的更新在各自的日期显示，不需要应用运行。`Date` 为零表示现在。没有条目时，小组件显示这种小组件的名称和描述。应用在前台时可以任意多次更新小组件，在后台时每天有几十次。

```go
err := widget.Update("counter", widget.Entry{
	View: widget.Column{Style: widget.Style{Gap: 4}, Children: []widget.View{
		widget.Text{Content: "Count", Style: widget.Style{TextStyle: widget.Caption, Color: widget.Secondary}},
		widget.Text{Content: strconv.Itoa(n), Style: widget.Style{FontSize: 44, FontWeight: core.FontWeightBold}},
	}},
})
```

`Entry.Families` 为某个尺寸显示它自己的视图，代替 `Entry.View`；`Entry.Background` 填充小组件在系统留出的边距之外的背景；`Entry.URL` 在点击小组件时用应用打开。没有编入小组件时 `widget.Update` 返回 `widget.ErrNoWidgets`，在 macOS 和 iOS 之外返回 `widget.ErrUnsupported`，`widget.Available` 可以在写入前判断属于哪种情况。它可以在任意 goroutine 上调用，在 `nsgo.Run` 之前也可以。

## 视图

小组件的视图不是 core 的元素：它们按 SwiftUI 的方式布局，视图默认和内容一样大，除非 `Style` 指定尺寸或让它撑满；容器默认把内容居中，除非 `AlignItems` 和 `Justify` 另有指定。`Text`、`Symbol` 和 `Image`（来自字节，或来自 `Update` 读取的文件）显示内容，`Column`、`Row`、`Stack`、`Spacer` 和 `Divider` 排列内容，`Rectangle`、`Progress` 和 `Gauge` 负责绘制，`Date` 用系统的格式显示时间，其中 `Relative`、`Offset` 和 `Timer` 会自己持续计时，不需要为每一秒写入条目。

```go
widget.Entry{
	View: widget.Column{Style: widget.Style{Gap: 4}, Children: []widget.View{
		widget.Text{Content: "Next hour in", Style: widget.Style{TextStyle: widget.Caption, Color: widget.Secondary}},
		widget.Date{Time: hour, Format: widget.Timer, Style: widget.Style{TextStyle: widget.Title2, FontVariant: core.FontTabularNums}},
		widget.Progress{Value: 0.4, Style: widget.Style{Color: widget.Accent}},
	}},
}
```

`Style` 提供间距、内边距、尺寸或撑满、对齐、带圆角的背景、系统的文字样式或字号、字重、字形和变体、行数限制，以及用应用打开的 `Link` 和把视图列入系统着色的 `Accented`。`widget.Primary`、`Secondary` 和 `Accent` 跟随外观，`RGB` 和 `RGBA` 可以用 `Dark` 指定深色模式下的另一种颜色。

## 实时活动

同一个扩展还负责绘制应用的实时活动，显示在锁屏和灵动岛里（[实时活动](live-activities.md)）。

## 下一步

- [实时活动](live-activities.md)：应用在事情发生过程中跟随的内容。
- [通知](notifications.md)：与小组件并用的通知和图标角标。
- [构建与发布](building.md)：扩展共享的 App Group。

---
title: 实时活动
description: 在锁屏和灵动岛里、以及作为 Android 的 Live Update，跟随正在发生的事。
group: 平台功能
order: 48
---

## 开启

实时活动跟随正在发生的事，比如外卖或计时器：在 iOS 16.1 起的 iPhone 上显示在锁屏和灵动岛里，在 Android 16 上作为 Live Update 显示在通知栏顶部和状态栏的胶囊里，更早的 Android 则显示为常驻通知。在 `nsgo.yaml` 中开启：

```yaml
live_activities:
  enabled: true
  # frequent_updates: true   # iOS：在用户允许时，可超出系统的更新额度
```

## 开始、更新与结束

`widget.StartActivity` 开始一个实时活动，它的类型说明跟随什么，写法与小组件的类型相同，并显示一份 `widget.ActivityContent`。`Activity.Update` 替换它显示的内容，`Activity.Alert` 在替换的同时提醒用户，`Activity.End` 结束它，不为 nil 时显示最终内容，并按 `Dismissal` 离开锁屏。`widget.Activities` 列出应用正在运行的实时活动，包括应用之前几次运行留下的；`widget.ActivitiesEnabled` 说明用户和平台是否允许。

```go
a, err := widget.StartActivity("delivery", widget.ActivityContent{
	Title: "Your order", Body: "On its way", Symbol: "box.truck.fill",
	Timer: arrival, ShowProgress: true, Progress: 0.4,
	Actions: []widget.ActivityAction{{ID: "pause", Title: "Pause", Symbol: "pause.fill"}},
})
if err != nil {
	return
}
a.Update(widget.ActivityContent{Title: "Your order", Body: "Arrived", Status: "Now"})
a.End(nil, widget.DismissAt(time.Now().Add(2*time.Hour)))
```

没有编入实时活动时 `StartActivity` 返回 `widget.ErrNoActivities`，在 iOS 16.1 和 Android 之外返回 `widget.ErrActivitiesUnsupported`，用户关闭实时活动时返回 `widget.ErrActivitiesDisabled`；活动结束后，`Update` 和 `End` 返回 `widget.ErrActivityEnded`。ActivityKit 和 APNs 都只接受 4 KB 以内的内容。

## 显示的内容

两个平台都会显示标题和正文、状态或由系统计时的计时、进度条、最多三个按钮和强调色。iOS 用小组件扩展绘制它们，并可以用 `widget` 的视图替换锁屏和灵动岛各个部分的内容：展开时的 `LockScreen`、`Leading`、`Trailing`、`Center` 和 `Bottom`，紧凑时的 `CompactLeading` 和 `CompactTrailing`，以及多个活动共用灵动岛时的 `Minimal`。Android 只显示这些字段，配以应用的图标，因为 Live Update 不接受应用的视图。

```go
widget.ActivityContent{
	Title: "Order 412", Body: "On its way", Status: "8 min", Symbol: "box.truck.fill",
	Tint: widget.RGB(0x30d158), ShowProgress: true, Progress: 0.4,
	Trailing: widget.Text{Content: "8 min", Style: widget.Style{FontSize: 34, FontWeight: core.FontWeightBold}},
}
```

`URL` 在点击实时活动时用应用打开，`Stale` 是 iOS 把内容显示为过时的时间，`Background` 填充锁屏上的实时活动。

## 按下按钮

`widget.OnAction` 运行按钮或 `widget.Button` 被按下时的处理函数，参数中带有实时活动和按钮的 ID。要在 `main` 中、`nsgo.Run` 之前设置它：在 iOS 上，系统会为一次按下在后台启动应用，不带任何视图，并在处理函数返回前保持这个实时活动，所以处理函数要在其中更新它；在 Android 上，按下会打开应用。处理函数运行在系统的 goroutine 上，而不是应用视图的 goroutine，需要视图时通过 `core.Scope` 的 `Post` 访问。

```go
func main() {
	widget.OnAction(func(p widget.ActivityPress) {
		if p.Action == "pause" {
			p.Activity.Update(paused)
		}
	})
	nsgo.Run(Root)
}
```

在 iOS 17 及以后，实时活动自己的视图也可以做到：`widget.Button{Action: "pause"}` 会运行同一个处理函数而不打开应用；在按下只会打开应用的地方，它只显示自己的子视图。

## 推送更新

应用在后台时只有仍在运行才能更新实时活动，因此要跟随别处发生的事，就用 `widget.ActivityOptions{Push: true}` 开始活动，把 `widget.OnPushToken` 收到的 token 发给服务器，由服务器推送 APNs 的 `liveactivity` 更新，其 `content-state` 为 `widget.ActivityState` 对该内容的返回值：推送中不能包含图片。从 iOS 17.2 起，应用还会得到用推送开始活动的 token，属性类型为 `widget.ActivityAttributesType`，属性中包含类型和 id。推送需要应用的描述文件包含推送通知权限。

```go
state, err := widget.ActivityState(content)  // 推送的 content-state
```

## 限制

系统会在八小时后结束实时活动。`widget.DismissDefault` 让结束的活动在 iOS 的锁屏上最多再留四小时，在 Android 上立即移除；`widget.DismissNow` 立即移除；`widget.DismissAt` 在四小时内的某个时间移除。Android 会自行申请 `POST_NOTIFICATIONS` 和 `POST_PROMOTED_NOTIFICATIONS` 权限，请先用 `cx.RequestNotificationPermission` 向用户请求（[通知](notifications.md)）。gallery 的 Widgets 演示和 `examples/widgets` 都运行了一个实时活动。

## 下一步

- [小组件](widgets.md)：由同一个扩展绘制的主屏幕和锁屏时间线。
- [通知](notifications.md)：Android 会先请求的权限。
- [构建与发布](building.md)：扩展和它需要的权限。

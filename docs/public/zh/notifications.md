---
title: 通知
description: 请求显示通知的权限，显示和关闭通知，在应用图标上显示数字并接收点按。
group: 平台功能
order: 45
---

## 权限

`cx.RequestNotificationPermission` 在用户做需要通知的操作时请求显示通知的权限，第一次会显示系统弹窗，并以 `core.NotificationPermission` 回答：`NotificationsGranted`、`NotificationsDenied`，或在平台或构建版本不显示通知时为 `NotificationsUnavailable`。iOS 和 Android 只会询问一次，浏览器也会拒绝用户没有触发的弹窗，所以要在用户做需要通知的操作时再请求。`cx.NotificationPermission` 不打扰用户地读取结果。

```go
cx.RequestNotificationPermission(func(p core.NotificationPermission) {
	if p == core.NotificationsGranted {
		cx.ShowNotification(core.Notification{ID: "chat:42", Title: "Ada", Body: "一起吃午饭？"}, nil)
	}
})
```

## 显示通知

`cx.ShowNotification` 显示一条通知，应用在前台时也显示，并替换同一 `ID` 的通知；`Silent` 让通知不带系统的声音和振动。它通过 `done` 回答系统是否接受了这条通知，没有权限时为 false；对于没有 ID 或标题的通知，它返回 `core.ErrNotification`，且不调用 `done`。`cx.CloseNotification` 移除 ID 指定的通知。

```go
err := cx.ShowNotification(core.Notification{ID: "sync", Title: "已同步", Body: "12 条笔记"}, func(shown bool) {})
cx.CloseNotification("sync")
```

## 点按

`cx.OnNotificationPress` 在设置它的组件存活期间，以用户点按的通知的 `ID` 调用函数，并把应用带到前台。启动应用的那一次点按会交给第一个注册的处理函数。把点按要打开的内容记在 `ID` 里（如 `"chat:42"`），因为点按也可能重新启动应用。在 Windows 上，只有由安装程序放进开始菜单的应用才会被点按重新启动；以其他方式运行的应用只在运行期间收到点按。

```go
cx.OnNotificationPress(func(id string) { openChat(strings.TrimPrefix(id, "chat:")) })
```

从 Android 13 起请求权限的 Android 应用要在 `nsgo.yaml` 的 `android.permissions` 中列出 `android.permission.POST_NOTIFICATIONS`。

## 图标角标

`cx.SetBadge` 在应用图标上显示数字：macOS 是程序坞图标，iOS 是主屏幕图标（需要通知权限），Windows 是任务栏按钮，兼容 Unity 的 Linux 启动器，以及已安装的 Web 应用。0 或更小的数字会清除它。没有 Linux 角标接口的启动器和 Android 没有可以设置的数字；Android 启动器会标记有通知显示的应用。

```go
cx.SetBadge(len(unread.Get()))
```

## 下一步

- [触感反馈](haptics.md)：设备在用户手指下播放的反馈。
- [平台服务](services.md)：警告框、剪贴板、文件、存储和网络。
- [实时活动](live-activities.md)：应用跟随的事，显示在锁屏和灵动岛里。
- [构建与发布](building.md)：应用包需要的权限。

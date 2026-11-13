---
title: 触感反馈
description: 值变化、元素碰撞或任务结束时，在用户手指下播放反馈。
group: 平台功能
order: 46
---

## 播放

`mobile.PlayHaptic` 在 iOS 和 Android 上播放用户手指下的触感反馈，其他平台不做任何事，因此要配合用户看得见的反馈。`HapticSelection` 表示值发生变化，如选择器、分段控件或滑块的步进；`HapticLight` 到 `HapticRigid` 表示元素按其重量和弹性碰撞；`HapticSuccess`、`HapticWarning` 和 `HapticError` 表示任务以相应结果结束。

```go
mobile.PlayHaptic(cx, mobile.HapticSelection)
```

## 何时播放

要在手势发生时播放，而不是结果返回时：值经过时给一次选择，视图落位时给一次碰撞，任务结束时给一次结果。只为用户造成的变化播放，应用自己发生的事不播放。

## 下一步

- [通知](notifications.md)：传达同样消息的通知和图标角标。
- [平台服务](services.md)：系统提供的其余能力。
- [无障碍](accessibility.md)：感受不到反馈的用户听到的播报。

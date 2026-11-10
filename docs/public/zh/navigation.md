---
title: 导航
description: 原生导航栈、系统标题栏、模态、菜单和标签栏。
group: 构建界面
order: 17
---

## 导航栈

`base.Nav` 保存一种路由类型的栈：`Push` 添加页面，`Pop` 移除顶部页面，`PopToRoot` 回到第一页，`Replace` 替换顶部页面。它的 `Stack` 把每个路由渲染成一页，顶部页面显示时，下面的页面保持挂载和状态。

```go
nav := base.NewNav(cx, "inbox")

func Inbox(cx *core.Scope) core.Node {
	return nav.Stack(core.Style{FlexGrow: 1}, func(cx *core.Scope, route string, depth func() int) base.NavPage {
		if route == "inbox" {
			return base.NavPage{Title: "收件箱", Children: []core.Node{
				ui.Button{Text: "打开一条消息", OnPress: func() { nav.Push("message") }},
			}}
		}
		return base.NavPage{Title: "消息", Children: []core.Node{
			ui.Button{Text: "返回", Variant: ui.ButtonSecondary, OnPress: func() { nav.Pop(1) }},
		}}
	})
}
```

在 iOS 和 Android 上，平台根据页面的 `Title` 和选项绘制系统标题栏，带返回按钮和转场；在 macOS、Windows、Linux 和 Web 上，页面设置 `Header` 时自己绘制。`HeaderLargeTitle` 请求 iOS 的大标题，`HeaderTransparent` 让内容在标题栏下方，`HeaderHidden` 隐藏标题栏，`HeaderLeft`、`HeaderTitle` 和 `HeaderRight` 放置应用自己的视图。`HeaderStyle` 和 `HeaderClass` 给标题栏上色：`Background`、标题的 `Color` 和按钮的 `TintColor`。

`NoPopOnBack` 在按下返回时保留页面，`OnBackPress` 听到返回，`NoPopOnSwipe` 关闭 iOS 的边缘滑动返回。`cx.OnBackPress(func() bool)` 在栈之前给应用一个处理器，用于未保存的更改。`OnDismiss` 报告用户已经弹出页面，栈会把页面移除。

## 模态

`base.Dialog` 在变暗的应用上显示一个表面，在触摸屏上从底部出现；`base.Sheet` 在某一侧显示面板。两者都接收 `Open`、`OnOpenChange`、内容和动效，除非另有设置，Escape 和点击背景都会关闭。

```go
base.Dialog{
	Open:         dialog.Get,
	OnOpenChange: func(open bool, _ base.Reason) { dialog.Set(open) },
	Title:        "重命名",
	Description:  "新名称会显示在所有地方。",
	Children:     []core.Node{ui.Input{Input: base.Input{Value: name.Get, OnChange: name.Set}}},
	Footer: []core.Node{
		ui.Button{Text: "取消", Variant: ui.ButtonSecondary, OnPress: func() { dialog.Set(false) }},
		ui.Button{Text: "保存", OnPress: save},
	},
}
```

`base.ShowAlertDialog` 显示系统的警告框或操作表，按钮按角色区分（`core.AlertCancel`、`core.AlertDestructive`）。`base.AlertDialog`、`base.Popover` 和 `base/toast` 覆盖其余系统表面。`ui.Dialog`、`ui.Sheet` 和 `ui.AlertDialog` 是同一批组件的带样式形式。

## 菜单

`base.Menu` 从触发器打开 `base.MenuItem`：桌面端在触发器旁，触摸屏上作为 sheet。条目通过 `OnSelect` 报告选择，除非 `NoCloseOnSelect` 否则关闭菜单，并可用 `Checked` 变成可勾选条目。`base.ContextMenu` 用右键或长按做同样的事，`base.Popover`、`base.HoverCard` 和 `base.Tooltip` 显示其他锚定内容。

```go
base.Menu{Trigger: ui.Button{Text: "操作", Variant: ui.ButtonSecondary}, Children: []core.Node{
	base.MenuItem{Label: "重命名", OnSelect: rename},
	base.MenuItem{Label: "复制", OnSelect: duplicate},
	base.MenuItem{Label: "删除", OnSelect: delete},
}}
```

## 标签页

`ui.Tabs[T]` 是分段控件或标签列表，指示器会滑到选中的选项。`ui.Sidebar` 为桌面侧边栏保存分组和行，`ui.TabShell` 在宽窗口把同一组标签放进侧边栏，在窄窗口放进底部栏。

`mobile.TabBar` 是 iOS 和 Android 的系统标签栏，带 Liquid Glass 或 Material 外观；它接收 `mobile.TabBarItem{Title, Icon}` 组成的 `Items`、选中索引和 `OnChange`。其他平台上它不占空间，因此应用在那里显示 `ui.Tabs`。`mobile.IsAvailable(cx)` 说明标签栏是否显示。

```go
mobile.TabBar{
	Items: []mobile.TabBarItem{{Title: "首页", Icon: "house"}, {Title: "搜索", Icon: "magnifyingglass"}},
	Selected: func() int { return tab.Get() },
	OnChange: func(i int) { tab.Set(i) },
}
```

## 下一步

- [表单](forms.md)：栈中的页面放什么。
- [动画](animation.md)：页面和模态如何移动。
- [布局](layout.md)：标题栏周围的安全区域。

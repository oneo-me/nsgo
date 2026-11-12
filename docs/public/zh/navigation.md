---
title: 导航
description: 原生导航栈、系统标题栏、模态、菜单和标签栏。
group: 核心概念
order: 56
---

## 导航栈

`base.NewNav` 保存一种路由类型的栈：`Push` 添加页面，`Pop` 移除顶部页面，`PopToRoot` 回到第一页，`Replace` 替换顶部页面。它的 `Stack` 把每个路由渲染成一页，顶部页面显示时，下面的页面保持挂载和状态。`ui.NavPage` 给每页加上 `ui` 的背景和一个 `ui.NavHeader`，包含标题、返回按钮和其他按钮。

```go
nav := base.NewNav(cx, "inbox")

return nav.Stack(core.Style{FlexGrow: 1}, func(cx *core.Scope, route string, depth func() int) base.NavPage {
	if route == "inbox" {
		return ui.NavPage(cx, base.NavPage{Title: "收件箱", Children: []core.Node{
			ui.Button{Text: "打开一条消息", OnPress: func() { nav.Push("message") }},
		}}, ui.NavHeader{})
	}
	return ui.NavPage(cx, base.NavPage{Title: "消息"}, ui.NavHeader{
		OnBack: func() { nav.Pop(1) }, BackLabel: "收件箱",
		Trailing: []core.Node{ui.IconButton{Icon: lucide.Archive, Label: "归档", Variant: ui.IconButtonGhost, OnPress: archive}},
	})
})
```

在 iOS 和 Android 上，平台根据页面标题和标题栏的按钮绘制系统标题栏，带返回按钮和转场；在 macOS、Windows、Linux 和 Web 上由 `ui` 绘制。

`NoPopOnBack` 在按下返回时保留页面，`OnBackPress` 听到返回，`NoPopOnSwipe` 关闭 iOS 的边缘滑动返回。`cx.OnBackPress(func() bool)` 在栈之前给应用一个处理器，用于未保存的更改。`OnDismiss` 报告用户已经弹出页面，栈会把页面移除。

## 模态

`ui.Dialog` 在变暗的应用上显示一个表面，在触摸屏上从底部出现；`ui.Sheet` 从底部显示面板。除非另有设置，Escape 和点击背景都会关闭它们。

```go
ui.Dialog{
	Open:         dialog.Get,
	OnOpenChange: func(open bool, _ base.Reason) { dialog.Set(open) },
	Title:        "重命名",
	Description:  "新名称会显示在所有地方。",
	Children:     []core.Node{ui.Input{Value: name.Get, OnChange: name.Set}},
	Footer: []core.Node{
		ui.Button{Text: "取消", Variant: ui.ButtonSecondary, OnPress: func() { dialog.Set(false) }},
		ui.Button{Text: "保存", OnPress: save},
	},
}
```

`ui.AlertDialog` 请用户确认，`cx.ShowAlert` 显示系统自己的警告框或操作表（[对话框](ui/dialog.md)）。

## 菜单

`ui.Menu` 从一个按钮打开 `ui.MenuItem`：在按钮旁，或在 `Presentation` 要求时作为底部 sheet，在任何设备上都一样。`ui.ContextMenu` 用右键或长按做同样的事，`ui.Popover`、`ui.HoverCard` 和 `ui.Tooltip` 显示其他锚定内容。

```go
ui.Menu{Trigger: ui.Button{Text: "操作", Variant: ui.ButtonSecondary}, Children: []core.Node{
	ui.MenuItem{Text: "重命名", OnSelect: rename},
	ui.MenuItem{Text: "复制一份", OnSelect: duplicate},
	ui.MenuItem{Text: "删除", Variant: ui.MenuItemDanger, OnSelect: remove},
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

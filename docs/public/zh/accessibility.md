---
title: 无障碍
description: 面向屏幕阅读器和键盘的标签、角色、值、操作和焦点。
group: 核心概念
order: 61
---

## 屏幕阅读器读到什么

视图的 `Label` 是辅助技术朗读的内容，`AccessibilityDescription` 是在标签之后朗读的补充说明，`Hint` 说明激活它会发生什么，`Role` 说明它是什么：`core.RoleButton`、`RoleLink`、`RoleHeader`、`RoleImage`、`RoleCheckbox`、`RoleSwitch`、`RoleRadio`、`RoleTab`、`RoleTextField`、`RoleMenu`、`RoleMenuItem`、`RoleDialog`、`RoleAlertDialog`、`RoleAlert`、`RoleAdjustable`、`RoleProgressBar`、`RoleTable` 等。对话框打开时会朗读它的角色和标签，再朗读它的说明，`ui.Dialog` 的标题和描述就是这样朗读的。没有标签的文本会朗读它的内容。

```go
core.View{
	Role:  core.RoleButton,
	Label: "加入购物车",
	Hint:  "加入商品并打开购物车",
	OnPress: add,
	Children: []core.Node{core.Icon{SVG: lucide.ShoppingCart}},
}
```

`AccessibilityValue` 给出范围值或文本值：滑块的最小值、最大值和当前值，或“3 件商品”这样的文本。`Accessible` 把视图变成一个元素，因此卡片的名称和价格会一起朗读，而不是两个停靠点；没有标签时它会朗读子元素的文本。`AccessibilityElementsHidden` 隐藏装饰，例如箭头或分隔线，`ImportantForAccessibility` 隐藏整个子树。

```go
core.View{Accessible: true, Label: "苹果，每公斤 12 元", Children: []core.Node{
	core.Text{Content: "苹果"},
	core.Text{Content: "每公斤 12 元"},
}}
```

## 操作

`AccessibilityActions` 列出屏幕阅读器在元素上提供的操作：标准操作（`core.ActionActivate`、`ActionIncrement`、`ActionDecrement`、`ActionEscape`、`ActionMagicTap`）和应用自己的操作，每个都有 `Name` 和 `Label`。`OnAccessibilityAction` 接收名称。

```go
core.View{
	Role:  core.RoleAdjustable,
	Label: "数量",
	AccessibilityValue: func() core.AccessibilityValue {
		return core.AccessibilityValue{Max: 10, Now: float32(n.Get()), Text: fmt.Sprintf("%d 件", n.Get())}
	},
	AccessibilityActions: []core.AccessibilityAction{{Name: core.ActionIncrement}, {Name: core.ActionDecrement}},
	OnAccessibilityAction: func(name string) {
		if name == core.ActionIncrement {
			n.Update(func(v int) int { return v + 1 })
		} else {
			n.Update(func(v int) int { return v - 1 })
		}
	},
	Children: quantityControls,
}
```

`Expanded` 说明展开控件或菜单按钮是否打开，`Busy` 表示内容仍在加载，`Invalid` 表示应用拒绝了字段的内容（用 `aria-invalid:` 设置样式），`Checked`、`Selected` 和 `Pressed` 报告控件状态。控件的部件可以通过 `PartStates` 借用它的状态，因此指示器会跟随复选框，而不会成为单独的停靠点。

## 实时变化

`LiveRegion` 让屏幕阅读器在视图标签和其中文本变化时朗读：`core.LiveRegionPolite` 等用户空闲，`LiveRegionAssertive` 打断当前朗读。`cx.Announce` 朗读一次消息。

```go
core.Text{ContentFunc: func() string { return fmt.Sprintf("购物车里有 %d 件商品", count.Get()) },
	LiveRegion: core.LiveRegionPolite}
```

## 焦点与键盘

在 macOS、Web 和 Apple TV 上，可按压的视图和文本框一起成为 Tab 停靠点；`NoFocus` 让一个视图退出，`Focusable` 让没有按压的视图成为停靠点。`Style.FocusVisible` 只在键盘移动焦点时绘制焦点环（`focus-visible:`），`Style.Focus` 对应任何焦点，`Style.FocusWithin` 对应视图内的焦点，`OnFocus` 和 `OnBlur` 听到焦点变化，`Ref.Focus()` 和 `Ref.SetAccessibilityFocus()` 移动焦点。Return 和空格按下获得焦点的视图，方向键在标签列表、单选组和菜单中移动。键盘把焦点移到视图上时，它的 `Tooltip` 立即显示，直到焦点离开或按下 Escape。弹出层的 `OnOpen` 在内容就绪后运行，在其中调用 `Focus` 就能把焦点移进去，非模态对话框应当这样做；popover 的内容在 Tab 顺序中紧跟触发器，弹出层关闭时焦点回到触发器。

`cx.Env().ScreenReader()` 说明是否有屏幕阅读器运行，`cx.Env().ReduceMotion()` 说明用户是否要求减少动态效果，`cx.Env().FontScale()` 是文本大小，`cx.Env().RTL()` 是布局方向。除非设置 `NoFontScaling`，文本会跟随文本大小；从右到左的布局使用逻辑边时会镜像物理边。

## 下一步

- [样式与主题](styles.md)：焦点和禁用状态。
- [表单](forms.md)：字段的标签和值。
- [画布与着色器](canvas.md)：暴露绘图的部件。

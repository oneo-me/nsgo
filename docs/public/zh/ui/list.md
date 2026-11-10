---
title: 列表
description: 任意数量的行，只构建显示中的部分，支持选择与激活。
group: 组件
order: 38
---

`ui.List` 在原生滚动视图上显示行，只构建屏幕附近的行，并以 Onevra 外观提供选择、激活和键盘操作。

```go
ui.List{
	Count:         func() int { return len(items) },
	Key:           func(i int) any { return items[i].ID },
	EstimatedSize: 44,
	Row: func(cx *core.Scope, row core.ListRow) core.Node {
		item := items[row.Index()]
		return ui.Text(item.Title, 13, ui.Regular, colors.Text)
	},
}
```

## 行与身份

`Count` 说明有多少行，`Row` 构建一行。`Key` 给每一行一个身份：保留下来的行在上下增删时保留自己的单元格和状态。行会新增、删除或重排时使用 ID 而不是下标；下标是一个跟随单元格显示其他行的 signal。

```go
ui.List{Count: count, Key: func(i int) any { return items[i].ID }, Row: func(cx *core.Scope, row core.ListRow) core.Node {
	return ui.Text(items[row.Index()].Title, 13, ui.Regular, colors.Text)
}}
```

`EstimatedSize` 是行的预期高度，为 0 时是 50 点。行高度可以不同；列表测量构建出来的行，并估算其余行。

## 布局

`Header`、`Footer` 和 `Empty` 放置列表首尾和没有行时的内容。`Gap` 分隔行，`Columns` 让行并排；列表的内边距是内容的内边距，因此滚动条留在边缘。给列表一个高度，或让它填满父容器。

## 选择与激活

`ui.List` 接受 `Selection`，用 `ui.NewSelection(cx, multiple)` 创建。Key、标签、激活和键盘都跟随它：上、下、Home 和 End 移动光标，Shift 扩展，Mod+A 全选，空格在多选时切换当前行，Enter 或双击激活一行。

```go
selected := ui.NewSelection(cx, true)
ui.List{
	Count:      count,
	Key:        func(i int) any { return items[i].ID },
	Selection:  selected,
	OnActivate: func(i int) { open(items[i]) },
	RowLabel:   func(i int) string { return items[i].Title },
	Row:        func(cx *core.Scope, row core.ListRow) core.Node { return rowContent(cx, items[row.Index()]) },
}
```

`TapSelects` 让手机上的点击只选择而不激活。`RowLabel` 是辅助技术朗读的内容，也是 typeahead 匹配的文字。

## 加载与滚动

`OnLoadMore` 在接近末尾时运行，每批行只运行一次。`Ref` 用 `ScrollToIndexAt`、`ScrollToEnd` 和 `ScrollToIndex` 滚动。它下层的普通列表 `core.List` 还有 `OnLoadMoreAtStart`、用于聊天的 `Inverted`、`StickyHeaderIndices`、`MaintainVisibleContentPosition`、`Viewport` 和 `ViewabilityConfig` 这些低层能力（[列表与滚动](../lists.md)）。

## 无障碍

列表是一个列表或 listbox，由 `Label` 命名，行的文字或 `RowLabel` 是行的名称。可选择列表会报告光标所在的行，并在键盘移动时朗读。屏幕阅读器移出可见区域时会要求列表滚到那里，列表会构建到达的行。

## 平台

原生滚动视图和虚拟化在每个目标上相同。行会被回收，因此行的状态必须跟随 `Key`；把构建状态放在应用里而不是行里，是通常的规则。

## 相关

- [列表与滚动](../lists.md)：完整指南，包括固定表头和加载更多。
- [滚动](scroll.md)：普通滚动视图。
- [表格](../lists.md#表格)：大量行和列的表格。
- gallery 的 List、Table 和 DataTable 页面。

---
title: 组件与状态
description: 元素、组件、signal 与样式。
group: 核心概念
order: 50
---

## 元素

屏幕是一棵元素树，元素是 Go 结构体值。`core` 提供基本元素：`View`、`Text`、`Image`、`Scroll`、`Input` 等。`ui` 提供应用使用的控件，例如 `Button`、`Select` 和 `DatePicker`，外观跟随主题。（`base` 提供同一批控件的无外观形式，供[组件作者](authoring/index.md)使用。）

```go
core.View{
	Style: core.Style{Padding: core.All(24), Gap: 12},
	Children: []core.Node{
		core.Text{Content: "Hello"},
	},
}
```

## 组件

组件是一个接收 scope、返回元素的函数。它只运行一次，之后的变化由 signal 驱动。

```go
func Greeting(cx *core.Scope) core.Node {
	return core.Text{Content: "Hello from NSGo"}
}
```

带属性的组件是一个有 `Render` 方法的类型：

```go
type Badge struct{ Text string }

func (b Badge) Render(*core.Scope) core.Node {
	return core.Text{Content: b.Text}
}
```

## Signal

signal 保存一个值。在元素调用的函数（例如 `ContentFunc`）中读取它，就会让屏幕上的这一部分订阅它；值变化时，只有这一部分更新。

```go
count := core.NewSignal(cx, 0)

core.Text{ContentFunc: func() string { return fmt.Sprintf("Pressed %d times", count.Get()) }}

ui.Button{Text: "Press me", OnPress: func() { count.Update(func(n int) int { return n + 1 }) }}
```

整个切片放在一个 signal 里时，读它的每一行都会更新。`core.NewCollection` 按 key 保存条目，每个条目有自己的 signal，所以每一行只跟随自己的条目，改一个条目只会重算一行：

```go
todos := core.NewCollection(cx, func(t Todo) int { return t.ID }, nil)

core.List{Count: todos.Len, Key: func(i int) any { return todos.KeyAt(i) },
	Row: func(cx *core.Scope, row core.ListRow) core.Node {
		return core.Text{ContentFunc: func() string { t, _ := todos.At(row.Index()); return t.Title }}
	}}

todos.Update(id, func(t Todo) Todo { t.Done = true; return t })
```

其他状态也同样建立在 signal 之上：

- `core.NewSelector` 判断某个 key 是否被选中，选中项变化时只重算离开和到达的那两行。
- `core.NewHistory` 保存一个可撤销、重做的值，它的 `CanUndo` 和 `CanRedo` 是 signal，可以直接用于编辑菜单。
- `core.NewPersisted` 把带类型的值以带版本号的 JSON 保存在应用的存储里，并迁移旧版本写下的数据。
- `core.From` 跟随响应式图之外的数据源，例如在其他 goroutine 上推送数据的订阅。

## 样式

`core.Style` 遵循 CSS：flexbox 与 grid、内外边距、圆角、边框、阴影、渐变、变换，以及 `Active` 等状态样式。元素也可以通过 `Class` 使用 Tailwind 类名：

```go
core.Text{Content: "Done", Class: "self-start rounded-full bg-zinc-100 px-3 py-1 text-sm"}
```

主题为 `ui` 的组件提供浅色与深色外观。

## 桌面应用

`nsgo.Run(Root)` 显示一个根视图。桌面应用也可以自己决定显示什么：`nsgo.Run(App)` 运行一次 `func App(app *core.Scope)`，由它通过 `desktop` 打开窗口、面板、托盘图标和全局快捷键，它们共享它的 signal。窗口、面板和托盘图标目前仅支持 macOS；Linux 的全局快捷键使用 X11 或合成器 portal 后端。`examples/tray` 不出现在程序坞中，只驻留在菜单栏。

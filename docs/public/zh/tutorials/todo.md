---
title: 示例：待办应用
description: 用 signal、列表和测试，一步一步构建一个小型待办应用。
group: 入门
order: 11
---

本教程构建一个小型待办应用：添加条目、勾选完成、删除条目。大约十分钟可以完成，开始前先完成[环境准备](../getting-started.md)。

## 创建应用

```sh
mkdir todo && cd todo
go mod init example.com/todo
go get github.com/nsgo-dev/nsgo
go get -tool github.com/nsgo-dev/nsgo/cmd/nsgo
go tool nsgo init
go tool nsgo dev
```

## 构建待办应用

生成的应用是一个计数器。把 `app.go` 的全部内容替换为：

```go program height=480
package main

import (
	"fmt"
	"strings"

	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
)

type todo struct {
	id   int
	text string
	done bool
}

func main() { nsgo.Run(Root) }

func Root(cx *core.Scope) core.Node {
	items := core.NewSignalFunc(cx, []todo{}, nil)
	draft := core.NewSignal(cx, "")
	nextID := core.NewSignal(cx, 1)
	colors := ui.ColorsOf(cx)

	add := func() {
		text := strings.TrimSpace(draft.Get())
		if text == "" {
			return
		}
		id := nextID.Get()
		nextID.Set(id + 1)
		items.Update(func(list []todo) []todo {
			return append(list, todo{id: id, text: text})
		})
		draft.Set("")
	}
	setDone := func(id int, done bool) {
		items.Update(func(list []todo) []todo {
			next := append([]todo(nil), list...)
			for i := range next {
				if next[i].id == id {
					next[i].done = done
				}
			}
			return next
		})
	}
	remove := func(id int) {
		items.Update(func(list []todo) []todo {
			out := list[:0:0]
			for _, it := range list {
				if it.id != id {
					out = append(out, it)
				}
			}
			return out
		})
	}
	left := func() int {
		n := 0
		for _, it := range items.Get() {
			if !it.done {
				n++
			}
		}
		return n
	}

	return core.View{
		Style: core.Style{FlexGrow: 1, FlexShrink: 1, Padding: core.All(24), PaddingEnv: core.EnvAll(core.EnvSafeArea), Gap: 16},
		StyleFunc: func() core.Style {
			return core.Style{Background: colors.Background()}
		},
		Children: []core.Node{
			ui.Text("待办", 28, ui.Bold, colors.Text),
			core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
				ui.Input{Value: draft.Get, OnChange: draft.Set, Placeholder: "要做什么？",
					OnSubmit: func(string) { add() }, TestID: "draft", Style: core.Style{FlexGrow: 1}},
				ui.Button{Text: "添加", Icon: lucide.Plus, OnPress: add, TestID: "add"},
			}},
			ui.Textf(func() string { return fmt.Sprintf("%d 项未完成", left()) }, 13, ui.Regular, colors.Muted),
			core.List{
				Count:         func() int { return len(items.Get()) },
				Key:           func(i int) any { return items.Get()[i].id },
				EstimatedSize: 44,
				Style:         core.Style{FlexGrow: 1},
				Empty:         ui.Text("还没有待办，在上方添加一项。", 13, ui.Regular, colors.Muted),
				Row: func(cx *core.Scope, row core.ListRow) core.Node {
					index := row.Index()
					it := items.Get()[index]
					return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Padding: core.Symmetric(8, 0)},
						Children: []core.Node{
							ui.Checkbox{
								Checked: func() bool {
									for _, x := range items.Get() {
										if x.id == it.id {
											return x.done
										}
									}
									return false
								},
								OnChange: func(done bool) { setDone(it.id, done) },
								Text:     it.text,
								TestID:   fmt.Sprintf("todo-%d", it.id),
								Style:    core.Style{FlexGrow: 1, FlexShrink: 1},
							},
							ui.IconButton{Icon: lucide.Trash, Label: "删除", Variant: ui.IconButtonGhost,
								TestID: fmt.Sprintf("delete-%d", it.id), OnPress: func() { remove(it.id) }},
						}}
				},
			},
		},
	}
}
```

保存后应用会重新启动。输入一项任务，点击**添加**，再勾选它。

## 它是如何工作的

**状态是 signal。** `items`、`draft` 和 `nextID` 都是 signal。signal 保存一个值并跟踪读取它的地方：`items` 变化时，只有读取它的文本和行会更新。`core.NewSignal` 用 `==` 比较值，因此切片要用没有相等函数的 `core.NewSignalFunc`。

**更新会替换切片。** `items.Update` 返回一个新切片，列表的身份和行的 key 决定哪些内容留在屏幕上。`nextID` 给每个待办一个 ID，`core.List` 的 `Key` 使用它；没有稳定的 key，行会把自己的单元格复用给其他条目。

**屏幕是一棵盒子树。** `Root` 返回一个 `core.View`，带内边距、间距和安全区域内边距。`FlexGrow: 1` 和 `FlexShrink: 1` 让它恰好填满窗口，所以长列表会在其中滚动，而不是把它撑出窗口边缘。输入行设置 `Direction: core.Row`，其余部分使用默认的纵向排列。

**输入框是受控的。** `draft.Get` 是输入框的值，`draft.Set` 接收每次变化。按下回车时 `OnSubmit` 添加任务，`add` 清空草稿，因此输入框会自己清空。

**列表只构建显示的部分。** `core.List` 按 `Count` 请求行，只在行接近屏幕时构建它。没有条目时显示 `Empty`，每一行的复选框和删除按钮带着待办的 ID，因此列表变化后按下时仍能找到正确的条目。

**外观来自 `ui`。** `ui.Text`、`ui.Input`、`ui.Button`、`ui.Checkbox` 和 `ui.IconButton` 从主题取色，并自动跟随浅色和深色。`ui.ColorsOf(cx)` 给出应用自己绘制部分所需的颜色，`StyleFunc` 会在主题变化时原地跟随。

## 测试它

生成的测试按的是计数器的按钮，因此把 `app_test.go` 替换为待办应用的测试：

```go
package main

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestAddCompleteAndDelete(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.Find("text:还没有待办，在上方添加一项。")
	app.Type("id:draft", "买牛奶")
	app.Press("id:add")
	app.WaitFor("text:买牛奶", time.Second)
	app.Press("id:todo-1")
	app.WaitFor("text:0 项未完成", time.Second)
	app.Press("id:delete-1")
	app.WaitFor("text:还没有待办，在上方添加一项。", time.Second)
}
```

测试以无窗口方式启动应用，按文本或 test ID 找到元素，并像用户一样操作。运行它：

```sh
go tool nsgo test
```

`TestMain` 在目标上运行测试时把主线程交给平台，`go tool nsgo test -target ios_simulator` 会用模拟器和它的原生输入运行同一个测试。

## 发布它

```sh
go tool nsgo build
```

`build` 为 `nsgo.yaml` 列出的每个目标写出发布包，放到 `dist/<名称>-<版本>-<目标>`：`.app`、`.ipa`、`.apk`、Windows 和 Linux 的目录，或一个网站。所有要交付出去的东西也都会放到 `dist/releases`：磁盘映像、安装程序、Debian 和 Arch 软件包、IPA、APK、网站、小程序，以及从你的 GitHub Releases 安装应用的脚本。[构建与发布](../building.md)介绍安装程序、签名与公证、发布、图标、小组件、轻 App 和 App Store。

## 下一步

- [示例：笔记应用](notes.md)和[示例：GitHub 热门仓库](repos.md)：再一步步做两个应用。
- [组件与状态](../components.md)：更深入地了解元素、组件、signal 和区域。
- [布局](../layout.md)和[样式与主题](../styles.md)：界面的结构和外观。
- [列表与滚动](../lists.md)：长列表、固定表头和加载更多。
- [测试](../testing.md)：找到控件、按下它们并截图。
- [嵌入](../embedding.md)：把应用放进另一个应用。

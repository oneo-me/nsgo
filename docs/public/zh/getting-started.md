---
title: 第一个应用
description: 安装 NSGo，并一步一步构建一个小型待办应用。
group: 开始
order: 2
---

本教程构建一个小型待办应用：添加条目、勾选完成、删除条目。装好环境后，大约十分钟可以完成。

## 环境要求

- [Go](https://go.dev/dl/) 1.26 或更新版本。
- 为 iOS、tvOS 和 watchOS 构建需要装有 Xcode 的 Mac。其他目标只需 macOS 上的 Go；x86_64 Android 模拟器还需要 Android NDK。
- 在 iPhone 上运行需要 Xcode 的 Accounts 中有 Apple ID，iPhone 已信任该电脑，并开启开发者模式。
- 在 Android 上运行需要 Android SDK platform-tools 中的 `adb`。
- 在 Windows 上运行只需要 Go，它也能构建 Windows 和 Web 包。
- 在 Linux 上运行需要 GTK 4.14 或更新版本、Pango 和 Cairo；网络图片需要 libsoup 3，网页视图需要 WebKitGTK 6.0，视频需要 GStreamer 插件。

## 创建应用

创建一个模块，加入 NSGo 和它的工具，然后让工具生成应用：

```sh
mkdir todo && cd todo
go mod init example.com/todo
go get github.com/oneo-me/nsgo
go get -tool github.com/oneo-me/nsgo/cmd/nsgo
go tool nsgo init
```

`go get -tool` 把 `nsgo` 加入模块一次，因此 `go tool nsgo` 无需安装任何东西就能运行。`init` 写出：

| 文件 | 内容 |
| --- | --- |
| `app.go` | 应用：一个 `Root` 组件，以及调用 `nsgo.Run(Root)` 的 `main` |
| `app_test.go` | 一个按下按钮的测试 |
| `nsgo.yaml` | 应用的名称、id、版本和目标 |

`go tool nsgo init -template tsx` 会生成 TSX 应用，`-template mixed` 则生成一个入口用 TSX、内部使用 Go 组件的应用。

## 运行应用

```sh
go tool nsgo dev
```

`dev` 为当前机器构建应用并打开它。每次保存都会重新构建并重启应用，只修改 TSX 时会原地更新正在运行的应用。构建失败会显示在应用上方，并提供复制错误和重新构建的按钮。

用 `-target` 在其他地方运行：

```sh
go tool nsgo dev -target web                # 在浏览器中
go tool nsgo dev -target ios_simulator      # 在 iOS 模拟器中
go tool nsgo dev -target tvos_simulator     # 在 Apple TV 模拟器中
go tool nsgo dev -target watchos_simulator  # 在 Apple Watch 模拟器中
go tool nsgo dev -target android            # 在已连接的设备或模拟器上
```

`-device` 选择 iPhone、Apple TV、Apple Watch、模拟器或 Android 设备，`-port` 设置 Web 服务器端口，`-trace` 打印运行中应用的每次提交及其时间和挂载项，`-fps` 在 macOS、iOS、Windows 和 Linux 上显示性能监视器。`-h` 列出所有参数。

## 构建待办应用

生成的应用是一个计数器。把 `app.go` 的全部内容替换为：

```go
package main

import (
	"fmt"
	"strings"

	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
	"github.com/oneo-me/nsgo/ui"
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
		Style: core.Style{FlexGrow: 1, Padding: core.All(24), PaddingEnv: core.EnvAll(core.EnvSafeArea), Gap: 16},
		StyleFunc: func() core.Style {
			return core.Style{Background: colors.Background()}
		},
		Children: []core.Node{
			ui.Text("待办", 28, ui.Bold, colors.Text),
			core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
				ui.Input{Input: base.Input{Value: draft.Get, OnChange: draft.Set, Placeholder: "要做什么？",
					OnSubmit: func(string) { add() }, TestID: "draft", Style: core.Style{FlexGrow: 1}}},
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

**屏幕是一棵盒子树。** `Root` 返回一个 `core.View`，带 `FlexGrow: 1`、内边距、间距和安全区域内边距。输入行设置 `Direction: core.Row`，其余部分使用默认的纵向排列。

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

	"github.com/oneo-me/nsgo/nsgotest"
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

`build` 为 `nsgo.yaml` 列出的每个目标写出发布包，放到 `dist/<名称>-<版本>-<目标>`：`.app`、`.ipa`、`.apk`、Windows 和 Linux 的目录，或一个网站。[构建与发布](building.md)介绍签名、图标、小组件、轻 App 和 App Store。

## 下一步

- [组件与状态](components.md)：更深入地了解元素、组件、signal 和区域。
- [布局](layout.md)和[样式与主题](styles.md)：界面的结构和外观。
- [列表与滚动](lists.md)：长列表、固定表头和加载更多。
- [测试](testing.md)：找到控件、按下它们并截图。
- [嵌入](embedding.md)：把应用放进另一个应用。

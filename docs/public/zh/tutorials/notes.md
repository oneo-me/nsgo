---
title: 示例：笔记应用
description: 用导航栈、多行编辑器和保存在设备上的笔记，构建一个笔记应用。
group: 入门
order: 12
---

本教程构建一个笔记应用：一个笔记列表、一个编辑笔记的页面，并且应用重新启动后笔记还在。开始前先完成[环境准备](../getting-started.md)。

## 创建应用

```sh
mkdir notes && cd notes
go mod init example.com/notes
go get github.com/oneo-me/nsgo
go get -tool github.com/oneo-me/nsgo/cmd/nsgo
go tool nsgo init
go tool nsgo dev
```

## 编写应用

把 `app.go` 整个替换为：

```go
package main

import (
	"encoding/json"
	"strings"

	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
	"github.com/oneo-me/nsgo/ui"
)

type note struct {
	ID   int    `json:"id"`
	Text string `json:"text"`
}

// title is a note's first line.
func (n note) title() string {
	line, _, _ := strings.Cut(strings.TrimSpace(n.Text), "\n")
	if line == "" {
		return "New note"
	}
	return line
}

func main() { nsgo.Run(Root) }

func Root(cx *core.Scope) core.Node {
	store := cx.Storage()
	var saved []note
	if data, ok := store.GetItem("notes"); ok {
		json.Unmarshal([]byte(data), &saved)
	}
	notes := core.NewSignalFunc(cx, saved, nil)
	save := func(list []note) {
		notes.Set(list)
		data, _ := json.Marshal(list)
		store.SetItem("notes", string(data))
	}

	// A route is the ID of the note it edits, and 0 the list.
	nav := base.NewNav(cx, 0)

	find := func(id int) note {
		for _, n := range notes.Get() {
			if n.ID == id {
				return n
			}
		}
		return note{ID: id}
	}
	add := func() {
		id := 1
		for _, n := range notes.Get() {
			id = max(id, n.ID+1)
		}
		save(append([]note{{ID: id}}, notes.Get()...))
		nav.Push(id)
	}
	edit := func(id int, text string) {
		list := append([]note(nil), notes.Get()...)
		for i := range list {
			if list[i].ID == id {
				list[i].Text = text
			}
		}
		save(list)
	}
	remove := func(id int) {
		var list []note
		for _, n := range notes.Get() {
			if n.ID != id {
				list = append(list, n)
			}
		}
		save(list)
		nav.Pop(1)
	}

	return nav.Stack(core.Style{FlexGrow: 1}, func(cx *core.Scope, id int, _ func() int) base.NavPage {
		if id == 0 {
			return listPage(cx, notes.Get, nav.Push, add)
		}
		return ui.NavPage(cx, base.NavPage{
			TitleFunc: func() string { return find(id).title() },
			Children: []core.Node{core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Padding: core.All(16)}, Children: []core.Node{
				ui.Input{Input: base.Input{
					Multiline:   true,
					Value:       func() string { return find(id).Text },
					OnChange:    func(text string) { edit(id, text) },
					Placeholder: "Write something",
					TestID:      "editor",
					Style:       core.Style{FlexGrow: 1, FlexShrink: 1},
				}},
			}}},
		}, ui.NavHeader{
			OnBack:    func() { nav.Pop(1) },
			BackLabel: "Notes",
			Trailing: []core.Node{ui.IconButton{Icon: lucide.Trash, Label: "Delete", Variant: ui.IconButtonGhost,
				TestID: "delete", OnPress: func() { remove(id) }}},
		})
	})
}

func listPage(cx *core.Scope, notes func() []note, open func(int), add func()) base.NavPage {
	colors := ui.ColorsOf(cx)
	return ui.NavPage(cx, base.NavPage{
		Title: "Notes",
		Children: []core.Node{core.List{
			Count:         func() int { return len(notes()) },
			Key:           func(i int) any { return notes()[i].ID },
			EstimatedSize: 48,
			Style:         core.Style{FlexGrow: 1},
			Empty: core.View{Style: core.Style{Padding: core.All(16)}, Children: []core.Node{
				ui.Text("No notes yet.", 13, ui.Regular, colors.Muted),
			}},
			Row: func(cx *core.Scope, row core.ListRow) core.Node {
				n := notes()[row.Index()]
				return core.View{
					Role:    core.RoleButton,
					OnPress: func(core.PressEvent) { open(n.ID) },
					Style:   core.Style{Padding: core.Symmetric(12, 16), Borders: core.Edges{Bottom: 1}},
					StyleFunc: func() core.Style {
						return core.Style{BorderColor: colors.Border()}
					},
					Children: []core.Node{ui.Text(n.title(), 15, ui.Regular, colors.Text)},
				}
			},
		}},
	}, ui.NavHeader{
		Trailing: []core.Node{ui.IconButton{Icon: lucide.Plus, Label: "New note", Variant: ui.IconButtonGhost,
			TestID: "new", OnPress: add}},
	})
}
```

按 **+** 新建一条笔记并输入内容；第一行会成为它在列表中的标题。退出应用再重新启动，笔记依然在。

## 工作原理

**页面栈。** `base.NewNav(cx, 0)` 维护一个路由栈，这里路由是笔记 ID，`0` 表示列表。`nav.Stack` 为每个路由构建一个页面，并让下面的页面保持挂载，所以打开笔记时列表的滚动位置不会丢失。`Push` 打开一条笔记，`Pop` 返回。

**带标题栏的页面。** `ui.NavPage` 给页面加上 `ui` 的背景和一个 `ui.NavHeader`，包含标题、返回按钮和其他按钮。在 iOS 和 Android 上由系统绘制标题栏，带有系统自己的返回手势和转场；其他平台由 `ui` 绘制。

**保存在设备上。** `cx.Storage()` 是应用的键值存储。`Root` 启动时读取一次笔记，每当笔记变化，`save` 就把它们写成 JSON。存储只保存字符串；应用写入的文件放在 `cx.DataDir()` 中。

**唯一的数据来源。** 每个页面都读同一个 `notes` signal。编辑器的 `Value` 读取笔记，`OnChange` 保存它；标题栏的 `TitleFunc` 和列表的行也跟随这个 signal，所以输入时它们会一起更新。

## 测试

替换 `app_test.go`。`nsgotest.Storage` 以预先写好的存储启动应用，就像它之前运行过一样：

```go
package main

import (
	"strings"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/nsgotest"
)

func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestWriteANote(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.Find("text:No notes yet.")
	app.Press("id:new")
	app.Type("id:editor", "Groceries\nMilk")
	app.WaitFor("role:header Groceries", time.Second)
	if data, _ := app.Storage().GetItem("notes"); !strings.Contains(data, "Milk") {
		t.Fatalf("saved %q", data)
	}
}

func TestSavedNotes(t *testing.T) {
	app := nsgotest.Start(t, Root, nsgotest.Storage(map[string]string{
		"notes": `[{"id":1,"text":"Ideas\nA notes app"}]`,
	}))
	app.Press("text:Ideas")
	app.Press("id:delete")
	app.WaitFor("text:No notes yet.", time.Second)
}
```

```sh
go tool nsgo test
```

## 下一步

- [示例：GitHub 热门仓库](repos.md)：从网络获取数据而不阻塞界面。
- [导航](../navigation.md)：模态框、菜单和标签栏。
- [平台服务](../services.md)：文件、分享、剪贴板等。

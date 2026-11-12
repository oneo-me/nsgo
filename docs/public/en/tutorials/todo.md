---
title: "Example: A todo app"
description: Build a small todo app step by step, with signals, a list and a test.
group: Get started
order: 11
---

This tutorial builds a small todo app: add items, check them off and delete them. It takes about ten minutes and assumes the [setup](../getting-started.md).

## Create the app

```sh
mkdir todo && cd todo
go mod init example.com/todo
go get github.com/nsgo-dev/nsgo
go get -tool github.com/nsgo-dev/nsgo/cmd/nsgo
go tool nsgo init
go tool nsgo dev
```

## Build the todo app

The generated app is a counter. Replace the whole of `app.go` with this app:

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
			ui.Text("Todos", 28, ui.Bold, colors.Text),
			core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
				ui.Input{Value: draft.Get, OnChange: draft.Set, Placeholder: "What needs doing?",
					OnSubmit: func(string) { add() }, TestID: "draft", Style: core.Style{FlexGrow: 1}},
				ui.Button{Text: "Add", Icon: lucide.Plus, OnPress: add, TestID: "add"},
			}},
			ui.Textf(func() string { return fmt.Sprintf("%d left", left()) }, 13, ui.Regular, colors.Muted),
			core.List{
				Count:         func() int { return len(items.Get()) },
				Key:           func(i int) any { return items.Get()[i].id },
				EstimatedSize: 44,
				Style:         core.Style{FlexGrow: 1},
				Empty:         ui.Text("Nothing yet. Add one above.", 13, ui.Regular, colors.Muted),
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
							ui.IconButton{Icon: lucide.Trash, Label: "Delete", Variant: ui.IconButtonGhost,
								TestID: fmt.Sprintf("delete-%d", it.id), OnPress: func() { remove(it.id) }},
						}}
				},
			},
		},
	}
}
```

Save the file and the app restarts. Type a task, press **Add** and check it off.

## How it works

**State is signals.** `items`, `draft` and `nextID` are signals. A signal holds a value and tracks what reads it: when `items` changes, only the text and rows that read it update. `core.NewSignal` compares values with `==`, so a slice uses `core.NewSignalFunc` with no equality function.

**Updates replace the slice.** `items.Update` returns a new slice, so the list's identity and the row keys decide what stays on screen. `nextID` gives each todo an ID, and `core.List`'s `Key` uses it; without a stable key, rows would reuse their cells for other items.

**The screen is a tree of boxes.** `Root` returns a `core.View` with padding, a gap and the safe area's insets. `FlexGrow: 1` and `FlexShrink: 1` make it fill the window and no more, so a long list scrolls inside it instead of stretching it past the window's edge. The input row sets `Direction: core.Row`; everything else is the default column.

**The input is controlled.** `draft.Get` is the field's value and `draft.Set` receives each change. `OnSubmit` adds a task when Return is pressed, and `add` clears the draft, so the field empties itself.

**The list only builds what shows.** `core.List` asks for `Count` rows and builds a row only when it nears the screen. `Empty` shows while there are no items, and each row's checkbox and delete button carry their todo's ID, so a press still finds the right item after the list changes.

**The look is `ui`'s.** `ui.Text`, `ui.Input`, `ui.Button`, `ui.Checkbox` and `ui.IconButton` take their colors from the theme and follow light and dark automatically. `ui.ColorsOf(cx)` gives the colors for the pieces the app draws itself, and a `StyleFunc` follows a change of theme in place.

## Test it

The generated test presses the counter's button, so replace `app_test.go` with a test of the todo app:

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
	app.Find("text:Nothing yet. Add one above.")
	app.Type("id:draft", "Buy milk")
	app.Press("id:add")
	app.WaitFor("text:Buy milk", time.Second)
	app.Press("id:todo-1")
	app.WaitFor("text:0 left", time.Second)
	app.Press("id:delete-1")
	app.WaitFor("text:Nothing yet. Add one above.", time.Second)
}
```

The test starts the app headless, finds elements by their text or test ID, and acts as a user would. Run it with:

```sh
go tool nsgo test
```

`TestMain` gives the platform the main thread when the tests run on a target, and `go tool nsgo test -target ios_simulator` runs the same test in the simulator with its native input.

## Ship it

```sh
go tool nsgo build
```

`build` writes a release package for each target `nsgo.yaml` lists, into `dist/<name>-<version>-<target>`: an `.app`, an `.ipa`, an `.apk`, Windows' and Linux's directories, or a web site. Everything to hand out also lands in `dist/releases`: the disk image, setup program, Debian and Arch packages, IPAs, APKs, the site, mini programs, and scripts that install the app from your GitHub releases. [Building and shipping](../building.md) covers installers, signing and notarization, publishing, icons, widgets, App Clips and the App Store.

## Next

- [Example: A notes app](notes.md) and [Example: Popular GitHub repositories](repos.md): two more apps, step by step.
- [Components and state](../components.md): elements, components, signals and regions in more depth.
- [Layout](../layout.md) and [Styles and themes](../styles.md): the screen's structure and look.
- [Lists and scrolling](../lists.md): long lists, sticky headers and loading more.
- [Testing](../testing.md): find controls, press them and take screenshots.
- [Embedding](../embedding.md): put the app inside another app.

---
title: Your first app
description: Install NSGo and build a small todo app step by step.
group: Start
order: 2
---

This tutorial builds a small todo app: add items, check them off and delete them. After the requirements, it takes about ten minutes.

## Requirements

- [Go](https://go.dev/dl/) 1.26 or later.
- To build for iOS, tvOS and watchOS, a Mac with Xcode. Every other target builds from macOS with Go alone; the x86_64 Android emulator's also needs an Android NDK.
- To run on an iPhone, an Apple ID in Xcode's Accounts and the iPhone trusted, with Developer Mode on.
- To run on Android, `adb` from the Android SDK's platform-tools.
- To run on Windows, Go alone, which also builds the Windows and web packages.
- To run on Linux, GTK 4.14 or newer, Pango and Cairo; libsoup 3 for images from the network, WebKitGTK 6.0 for web views and GStreamer plug-ins for videos.

## Create the app

Make a module, add NSGo and its tool, then let the tool write an app:

```sh
mkdir todo && cd todo
go mod init example.com/todo
go get github.com/oneo-me/nsgo
go get -tool github.com/oneo-me/nsgo/cmd/nsgo
go tool nsgo init
```

`go get -tool` adds `nsgo` to the module once, so `go tool nsgo` runs it without installing anything. `init` writes:

| File | What it is |
| --- | --- |
| `app.go` | The app: a `Root` component and `main`, which calls `nsgo.Run(Root)` |
| `app_test.go` | A test that presses the button |
| `nsgo.yaml` | The app's name, id, version and targets |

`go tool nsgo init -template tsx` writes a TSX app instead, and `-template mixed` a Go app whose TSX entry uses Go components of its own.

## Run it

```sh
go tool nsgo dev
```

`dev` builds the app for this machine and opens it. On each save it rebuilds and restarts the app, and a change of TSX alone updates the running app in place. A failed build shows over the app, with buttons to copy the error and to build again.

Run it somewhere else with `-target`:

```sh
go tool nsgo dev -target web                # in the browser
go tool nsgo dev -target ios_simulator      # in the iOS simulator
go tool nsgo dev -target tvos_simulator     # in the Apple TV simulator
go tool nsgo dev -target watchos_simulator  # in the Apple Watch simulator
go tool nsgo dev -target android            # on a connected device or emulator
```

`-device` picks the iPhone, Apple TV, Apple Watch, simulator or Android device, `-port` the web server's port, `-trace` prints each commit of the running app with its time and mount items, and `-fps` shows the performance monitor on macOS, iOS, Windows and Linux. `-h` lists every flag.

## Build the todo app

The generated app is a counter. Replace the whole of `app.go` with this app:

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
			ui.Text("Todos", 28, ui.Bold, colors.Text),
			core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
				ui.Input{Input: base.Input{Value: draft.Get, OnChange: draft.Set, Placeholder: "What needs doing?",
					OnSubmit: func(string) { add() }, TestID: "draft", Style: core.Style{FlexGrow: 1}}},
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

**The screen is a tree of boxes.** `Root` returns a `core.View` with `FlexGrow: 1`, padding, a gap and the safe area's insets. The input row sets `Direction: core.Row`; everything else is the default column.

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

	"github.com/oneo-me/nsgo/nsgotest"
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

`build` writes a release package for each target `nsgo.yaml` lists, into `dist/<name>-<version>-<target>`: an `.app`, an `.ipa`, an `.apk`, Windows' and Linux's directories, or a web site. Everything to hand out also lands in `dist/releases`: the disk image, setup program, Debian and Arch packages, IPAs, APKs, the site, mini programs, and scripts that install the app from your GitHub releases. [Building and shipping](building.md) covers installers, signing and notarization, publishing, icons, widgets, App Clips and the App Store.

## Next

- [Components and state](components.md): elements, components, signals and regions in more depth.
- [Layout](layout.md) and [Styles and themes](styles.md): the screen's structure and look.
- [Lists and scrolling](lists.md): long lists, sticky headers and loading more.
- [Testing](testing.md): find controls, press them and take screenshots.
- [Embedding](embedding.md): put the app inside another app.

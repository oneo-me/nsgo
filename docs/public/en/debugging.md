---
title: Running and debugging
description: Run an app on any target with hot reload, and see its errors, updates, speed and other languages.
group: Toolchain
order: 20
---

## Run

```sh
go tool nsgo dev
```

`dev` builds the app for this machine and opens it, or for the target `nsgo.yaml`'s `dev.target` names. On each save of a Go file it rebuilds and restarts the app, running `go mod tidy` first when the app imports a package whose module `go.mod` or `go.sum` lacks; a change of TSX alone updates the running app in place, keeping its state. The run ends with the app.

`-target` runs it elsewhere, and `-device` picks the iPhone, Apple TV, Apple Watch, simulator or Android device by name, UDID or serial:

```sh
go tool nsgo dev -target web                # in the browser, at -port (8080 by default)
go tool nsgo dev -target ios_simulator      # in the iOS simulator
go tool nsgo dev -target ios                # on the connected iPhone, signed by Xcode
go tool nsgo dev -target tvos_simulator     # in the Apple TV simulator
go tool nsgo dev -target watchos_simulator  # in the Apple Watch simulator
go tool nsgo dev -target android            # on a connected device or emulator, over adb
```

`-h` lists every flag.

## Errors

A failed build shows over the running app, with buttons to copy the error and to build again; fix the file and the next save replaces it. A component that panics while it renders shows the same way. The app's own output, such as `log.Print`, appears in the terminal under the tool's lines.

Before running, `go tool nsgo check` vets the app and type-checks its TSX; `-target all` vets it for every target, so code behind another platform's build tags is checked from a Mac too.

## DevTools

```sh
go tool nsgo dev -devtools
```

`-devtools` opens DevTools in the browser, served by `nsgo dev` for every target, a phone's app too. It shows the running app as it runs:

- **Elements**: the views in a tree. Each shows the component that built it, a link to its line in your editor, the selector a test finds it by, its classes, its bounds, its margin, border and padding as laid out, and the style it sets. Hovering a row draws its margin, padding and content in the app, as a browser's DevTools do, and **Pick** lets you press a view in the app to find it in the tree.
- **State**: the signals your code made, under the components and computations that own them, with their values, memos' values, and how many computations read each. A value no computation reads changes nothing on screen. Strings, booleans and numbers can be edited, so you can try a state without a code change. Values NSGo made for itself are behind **NSGo's own**.
- **Profiler**: each commit's time, split into your code before it, the commit with its layout, and applying it on the platform's main thread. It also shows the lines whose writes caused the commit, and opens on the computations it ran, the longest first. A second list names the computations that ran most in the last second, by the line that made them. Android's runtime applies batches in its own Java code, so Android commits have no platform time.
- **Recorder**: what you press, type, clear and scroll in the app, written as an `nsgotest` test file to copy into a `_test.go` file. In Elements, **Assert shown** and **Assert text** add a check of the selected view. A text check holds the text in the language the app showed it in, so record with the language your tests run in.
- **Highlight updates** outlines each view a commit changes, as `-highlight` does.

A TSX app's state, computations and writes are named by their TSX lines, through the bundles' source maps, and link to the files in your app.

DevTools read the app through `nsgo dev`'s server, which listens on your network so phones reach it: anyone there can open DevTools, change the app's state while it runs, and add steps to the test the Recorder writes, so read a recorded test before you run it. Web pages of other sites you visit cannot: the server takes DevTools' commands and the app's messages only as JSON, which another site's page cannot send it, and only from its own page.

## Updates

```sh
go tool nsgo dev -highlight
```

`-highlight` outlines for half a second each view a commit changed, so an update that redraws more than it should shows on screen as you use the app. The outlines draw over the app's main layer, under its modals. DevTools turn them on and off while the app runs.

```sh
go tool nsgo dev -trace
```

`-trace` prints each commit of the running app: its number, its time, and the mount items it sent, so you see which views an update created, changed or removed. Above the items it names what the commit followed from: the lines of your code that wrote signals since the last commit, the most writing first, and how many bindings, effects and memos ran.

```text
── commit 12: 40 items, commit 1.65 ms
   after app/market.go:473 ×4, app/market.go:487 ×4, app/market.go:384
   ran 118 bindings, 18 effects, 5 memos
```

A line that writes many times, or many bindings run for one write, is where to batch writes (`cx.Batch`) or narrow what a binding reads. `go tool nsgo test -trace` prints the same under each step of a test.

## Speed

```sh
go tool nsgo dev -fps
go tool nsgo dev -release -fps
```

`-fps` shows the performance monitor over the app on macOS, iOS, Windows and Linux: what its frames and commits cost while it runs. Dev builds carry the dev client; measure with `-release`, which builds as `nsgo build` does.

## Languages and appearance

```sh
go tool nsgo dev -lang zh-Hans
go tool nsgo dev -lang en-XA   # accented and longer, to find untranslated and cut text
go tool nsgo dev -lang ar-XB   # right to left
```

`-lang` starts the app in a locale, a pseudo locale too ([Localization](localization.md)).

## Flags of every build

Every build of an app, packages included, takes NSGo's own flags as it starts, which override `nsgo.yaml` and the app's code and never reach the app's own flags:

| Flag | Effect |
| --- | --- |
| `--nsgo-fps` | Shows the performance monitor on macOS, iOS, Windows and Linux |
| `--nsgo-lang=zh-Hans` | Starts the app in a locale, a pseudo locale too |
| `--nsgo-appearance=dark` | Starts the app `light`, `dark` or following the `system` |

```sh
./Hello --nsgo-fps                                   # Windows and Linux
open Hello.app --args --nsgo-fps                     # macOS
xcrun simctl launch booted dev.example.hello --nsgo-fps
adb shell am start -n dev.example.hello/dev.oneo.nsgo.NsgoActivity --ez nsgo-fps true --es nsgo-lang de
```

On the web they go in the page's query: `?nsgo-fps&nsgo-lang=de`.

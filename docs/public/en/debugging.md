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

## Updates

```sh
go tool nsgo dev -trace
```

`-trace` prints each commit of the running app: its number, its time, and the mount items it sent, so you see which views an update created, changed or removed. A signal change that updates more than you expect shows up here first. `go tool nsgo test -trace` prints the same under each step of a test.

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

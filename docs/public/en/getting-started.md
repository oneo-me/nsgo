---
title: Getting started
description: Install the nsgo tool, create an app and run it with hot reload.
group: Start
order: 2
---

## Requirements

- [Go](https://go.dev/dl/) 1.26 or later.
- To build for iOS, a Mac with Xcode. Every other target builds from macOS with Go alone; the x86_64 Android emulator's also needs an Android NDK.
- To run on an iPhone, an Apple ID in Xcode's Accounts and the iPhone trusted, with Developer Mode on.
- To run on Android, `adb` from the Android SDK's platform-tools.
- To run on Windows, Go alone, which also builds the Windows and web packages.
- To run on Linux, GTK 4.14 or newer, Pango and Cairo; WebKitGTK 6.0 for web views and GStreamer plug-ins for videos.

## Create an app

Make a module, add NSGo and its tool, then let the tool write an app:

```sh
mkdir hello && cd hello
go mod init example.com/hello
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

`dev` builds the app for this machine and opens it. On each save it rebuilds and restarts the app, and a change of TSX alone updates the running app in place, keeping top-level signals, navigation stacks and the state of the components it did not change. A failed build shows over the app; Command or Control+R builds again. `dev.hot_reload: false` in `nsgo.yaml` runs it once instead.

Run it somewhere else with `-target`:

```sh
go tool nsgo dev -target web            # in the browser
go tool nsgo dev -target ios_simulator  # in the iOS simulator
go tool nsgo dev -target android        # on a connected device or emulator
```

`-device` picks the iPhone, simulator or Android device, `-port` the web server's port, `-trace` prints each commit of the running app with its time and mount items, and `-fps` shows the performance monitor on macOS, iOS, Windows and Linux. `-h` lists every flag.

## The project

Beside `nsgo.yaml`, an app may have:

- `resources/`: files the app ships, such as images it loads with `asset:` and fonts it registers with `nsgo.Fonts("fonts/Brand.ttf")`.
- `assets/`: project files that do not ship, such as `AppIcon.icon` and `Icon.png`.
- `locales/`: the app's messages, a YAML file per language ([Localization](localization.md)).

## Next

- [Components and state](components.md): how an app is built.
- [Testing](testing.md): test the app the way a user uses it.
- [Building and shipping](building.md): packages for every platform.

---
title: Setup
description: Install what NSGo needs, create an app and run it on every target.
group: Get started
order: 10
---

## Requirements

- [Go](https://go.dev/dl/) 1.26 or later.
- To build for iOS, tvOS and watchOS, a Mac with Xcode. Every other target builds from macOS with Go alone; the x86_64 Android emulator's also needs an Android NDK.
- To run on an iPhone, an Apple ID in Xcode's Accounts and the iPhone trusted, with Developer Mode on.
- To run on Android, `adb` from the Android SDK's platform-tools.
- To run on Windows, Go alone, which also builds the Windows and web packages.
- To run on Linux, GTK 4.14 or newer, Pango and Cairo; libsoup 3 for images from the network, WebKitGTK 6.0 for web views and GStreamer plug-ins for videos.

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
| `app.go` | The app, a counter: a `Root` component and `main`, which calls `nsgo.Run(Root)` |
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

`go tool nsgo doctor` lists every target with whether this machine builds it and runs it, and what it lacks for those it cannot, such as Xcode, `adb`, a connected device or GTK.

[Running and debugging](debugging.md) covers devices, tracing updates, the performance monitor and other languages.

## Next

- [Example: A todo app](tutorials/todo.md): state, a list and a test, in about ten minutes.
- [Example: A notes app](tutorials/notes.md): a navigation stack and notes kept on the device.
- [Example: Popular GitHub repositories](tutorials/repos.md): loading from the network with errors and a retry.

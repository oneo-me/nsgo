---
title: Compared with other frameworks
description: Where NSGo fits beside NSH, MyGo and other UI frameworks, and what it gives up.
order: 2
---

NSGo's place among UI frameworks comes from three choices: apps are written in Go (or TSX on top of it), each platform draws with its own views, and one codebase covers phones, desktops and the web. Other frameworks make other choices, and each choice costs something.

## At a glance

| Framework | Language | Draws with | Targets |
| --- | --- | --- | --- |
| NSGo | Go, TSX | Each platform's views | iOS, tvOS, watchOS, Android, macOS, Windows, Linux, web |
| [NSH](https://nsh.dev) | Rust, TSX | Each platform's views | iOS, Android, macOS, Windows, Linux, web |
| [MyGo](https://github.com/egoist/mygo) | Go, web pages | Its own GPU renderer, or the system webview | macOS, Windows, Linux, web |
| [Fyne](https://fyne.io), [Gio](https://gioui.org) | Go | Their own renderer | Desktops, phones, web |
| [Wails](https://wails.io) | Go, web pages | The system webview | Desktops |
| [Flutter](https://flutter.dev) | Dart | Its own renderer | Phones, desktops, web |
| [Compose Multiplatform](https://www.jetbrains.com/compose-multiplatform/) | Kotlin | Its own renderer; Android's Compose | Phones, desktops, web |
| [React Native](https://reactnative.dev) | JavaScript, TSX | Each platform's views | iOS, Android; others through community platforms |
| [.NET MAUI](https://dotnet.microsoft.com/apps/maui) | C# | Each platform's controls | iOS, Android, macOS, Windows |
| [Avalonia](https://avaloniaui.net) | C# | Its own renderer | Desktops, phones, web |
| [Qt](https://www.qt.io) | C++, QML | Its own renderer | Desktops, phones, embedded devices |
| [Electron](https://www.electronjs.org), [Tauri](https://tauri.app) | Web pages; Rust for Tauri | A bundled Chromium; the system webview for Tauri | Desktops; phones for Tauri |

## Strengths

- **Native views.** Text, input methods, selection, scrolling, accessibility and menus are the platform's own, so an app behaves as users expect there, where a framework that draws itself rebuilds each of them.
- **Fine-grained updates.** Components run once and signals update only what reads them, with no virtual DOM or per-frame view function to diff; animation runs on the platform's frame clock.
- **Go only.** The engine, layout and backends are Go, with no C, no cgo and no JavaScript engine for Go apps. Every target builds from a Mac with Go and Xcode: no NDK, Android SDK, JDK, GTK or Windows toolchain.
- **Every platform.** Phones, TVs, desktops and the web share one engine, one API and one test model, and apps can embed in existing iOS, Android, Go desktop and web apps.
- **TSX when wanted.** Screens can be written in TSX with Tailwind classes, on the same engine as Go components.

## Weaknesses

- **Early.** No release and no stable API yet; the [roadmap](https://github.com/oneo-me/nsgo/blob/main/docs/roadmap.md) lists what is missing, such as more than one window on Windows and Linux, and a heap limit for TSX delivered at run time.
- **A small ecosystem.** Few third-party components, examples or answers online, next to Flutter, React Native or Qt.
- **Platforms differ.** Native views mean small differences between platforms, so an app needs checking on each. A framework that draws itself looks the same everywhere.
- **Its own bridges.** On Android the Go program runs beside the Java runtime and talks to it over pipes, and on Linux users need GTK 4.14 or newer.
- **TSX is interpreted.** TSX runs in goja, a JavaScript interpreter written in Go, slower than the engines React Native or Electron use.
- **No automatic updates.** `nsgo build` makes installers, notarizes macOS apps and writes scripts that install from GitHub releases, but installed apps do not update themselves.

## NSH

NSGo ports NSH's design: the architecture, the backends, the API's names and `base`'s unstyled components are NSH's ([status](index.md#status)).

- **Choose NSGo** to write apps in Go rather than Rust or TSX, to build every target from a Mac with Go and Xcode, or for what NSGo adds: GPU shaders written in Go, vector icons, a terminal, apps opening their own windows, panels and tray icons, and styled components in `ui`.
- **Choose NSH** for a more mature project, TSX run by QuickJS with bytecode and heap limits, the GPUI backend, or 32-bit Android.

## MyGo

Both are UI written in Go without cgo, but MyGo draws its native UI itself on the GPU and rebuilds it from a view function every frame, where NSGo hands native views what changed.

- **Choose MyGo** for desktop apps that look the same on every desktop, web-page windows with typed calls into Go, and its desktop APIs and shipping: tray icons, deep links, file associations, installers, notarization and signed delta updates.
- **Choose NSGo** to reach phones too, to use each platform's own text, input and accessibility, or for state held in signals rather than redrawn from the app's data.

## Other Go toolkits

- **Fyne and Gio** draw everything themselves, as MyGo does, and both reach phones and the web. Fyne gives a full widget set with its own look; Gio is immediate mode, low level and flexible. Both have released versions and users; neither looks or feels native.
- **Wails** puts a web frontend in the system webview with Go behind it: any web library works, but the interface is a web page, desktop only.

## Frameworks in other languages

- **Flutter and Compose Multiplatform** draw every pixel themselves: identical on every platform and mature, with large ecosystems, but with text, scrolling and input rebuilt rather than native, and Dart or Kotlin to learn.
- **React Native** shares NSGo's model of a reactive core driving native views, and has the largest ecosystem of its kind. It centers on iOS and Android, and runs app logic in JavaScript; NSGo runs it in Go and treats desktops and the web as first-class.
- **.NET MAUI** wraps each platform's controls in C#, without Linux or the web. **Avalonia** draws itself, close to WPF, on every desktop, phones and the web.
- **Qt** is mature, fast and reaches embedded devices, but draws its own widgets, asks for C++ or QML, and is licensed under the LGPL or commercially.
- **Electron and Tauri** show web pages: the web's tools and libraries in full, at the cost of a browser's memory, Electron's above all, and an interface that feels like a web page.

## Choosing

Choose NSGo for an app that should feel native on phones, desktops and the web from one Go codebase, when an early project is acceptable. For an app that must ship soon on a stable API, take a mature framework from the table above: Flutter or React Native for phones, MyGo, Wails or Tauri for desktops alone.

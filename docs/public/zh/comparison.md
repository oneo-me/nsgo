---
title: 与其他框架对比
description: NSGo 与 NSH、MyGo 及其他 UI 框架相比处在什么位置，又放弃了什么。
group: 开始
order: 3
---

NSGo 在 UI 框架中的位置来自三个选择：应用用 Go 编写（也可以在其上用 TSX），每个平台用自己的视图绘制，一套代码覆盖手机、桌面和 Web。其他框架做了不同的选择，每种选择都有代价。

## 一览

| 框架 | 语言 | 绘制方式 | 目标平台 |
| --- | --- | --- | --- |
| NSGo | Go、TSX | 各平台自己的视图 | iOS、Android、macOS、Windows、Linux、Web |
| [NSH](https://nsh.dev) | Rust、TSX | 各平台自己的视图 | iOS、Android、macOS、Windows、Linux、Web |
| [MyGo](https://github.com/egoist/mygo) | Go、网页 | 自己的 GPU 渲染器，或系统 WebView | macOS、Windows、Linux、Web |
| [Fyne](https://fyne.io)、[Gio](https://gioui.org) | Go | 自己的渲染器 | 桌面、手机、Web |
| [Wails](https://wails.io) | Go、网页 | 系统 WebView | 桌面 |
| [Flutter](https://flutter.dev) | Dart | 自己的渲染器 | 手机、桌面、Web |
| [Compose Multiplatform](https://www.jetbrains.com/compose-multiplatform/) | Kotlin | 自己的渲染器；Android 上为 Compose | 手机、桌面、Web |
| [React Native](https://reactnative.dev) | JavaScript、TSX | 各平台自己的视图 | iOS、Android；其他平台靠社区实现 |
| [.NET MAUI](https://dotnet.microsoft.com/apps/maui) | C# | 各平台自己的控件 | iOS、Android、macOS、Windows |
| [Avalonia](https://avaloniaui.net) | C# | 自己的渲染器 | 桌面、手机、Web |
| [Qt](https://www.qt.io) | C++、QML | 自己的渲染器 | 桌面、手机、嵌入式设备 |
| [Electron](https://www.electronjs.org)、[Tauri](https://tauri.app) | 网页；Tauri 另用 Rust | 内置的 Chromium；Tauri 用系统 WebView | 桌面；Tauri 也支持手机 |

## 优势

- **原生视图。** 文本、输入法、选择、滚动、无障碍和菜单都是平台自己的，应用在每个平台上的表现符合用户预期；自绘框架则要逐一重新实现它们。
- **细粒度更新。** 组件只运行一次，signal 只更新读取它的地方，没有虚拟 DOM，也没有每帧都要比对的视图函数；动画运行在平台的帧时钟上。
- **只有 Go。** 引擎、布局和后端都是 Go，Go 应用不需要 C、cgo 或 JavaScript 引擎。所有目标都能在装有 Go 和 Xcode 的 Mac 上构建：不需要 NDK、Android SDK、JDK、GTK 或 Windows 工具链。
- **覆盖所有平台。** 手机、桌面和 Web 共用一个引擎、一套 API 和一种测试方式，应用还能嵌入已有的 iOS、Android、Go 桌面和 Web 应用。
- **可选 TSX。** 界面可以用 TSX 和 Tailwind 类名编写，与 Go 组件运行在同一个引擎上。

## 劣势

- **还很早期。** 尚无发布版本和稳定 API；[路线图](https://github.com/oneo-me/nsgo/blob/main/docs/roadmap.md)列出了缺少的部分，例如 Windows 和 Linux 上的多窗口，以及运行时下发的签名 TSX 包。
- **生态很小。** 与 Flutter、React Native 或 Qt 相比，第三方组件、示例和网上的答案都很少。
- **平台之间有差异。** 原生视图意味着各平台之间存在细微差别，应用需要在每个平台上检查。自绘框架在哪里看起来都一样。
- **自己的桥接方式。** 在 Android 上，Go 程序运行在 Java 运行时旁边，通过管道与它通信；Linux 用户需要 GTK 4.14 或更新版本。
- **TSX 是解释执行的。** TSX 运行在 goja 中，这是一个用 Go 写的 JavaScript 解释器，比 React Native 或 Electron 使用的引擎慢。
- **发布能力还很基础。** `nsgo build` 能打包并签名应用，但不生成安装程序，也不提供自动更新。

## NSH

NSGo 移植了 NSH 的设计：架构、后端、API 的命名和 `base` 中无样式的组件都来自 NSH（[现状](index.md#现状)）。

- **选择 NSGo**：想用 Go 而不是 Rust 或 TSX 写应用，想在 Mac 上只用 Go 和 Xcode 构建所有目标，或者需要 NSGo 新增的能力：用 Go 编写的 GPU 着色器、矢量图标、终端、应用自行打开的窗口、面板和托盘图标，以及 `ui` 中带样式的组件。
- **选择 NSH**：想要更成熟的项目，需要由 QuickJS 运行、带字节码和堆限制的 TSX，运行时下发 TSX 的签名包和沙盒，GPUI 后端，或 32 位 Android。

## MyGo

两者都是不用 cgo、用 Go 编写的 UI，但 MyGo 在 GPU 上自己绘制原生 UI，每帧都由视图函数重新构建；NSGo 则把变化的部分交给原生视图。

- **选择 MyGo**：做在每个桌面上外观一致的桌面应用，需要能以类型化方式调用 Go 的网页窗口，或者需要它的桌面 API 与发布能力：托盘图标、深度链接、文件关联、安装程序、公证和签名的增量更新。
- **选择 NSGo**：还要覆盖手机，想用各平台自己的文本、输入和无障碍，或者希望状态存放在 signal 中，而不是每次从应用数据重绘。

## 其他 Go 工具包

- **Fyne 和 Gio** 像 MyGo 一样自己绘制一切，也都支持手机和 Web。Fyne 提供一整套外观统一的控件；Gio 是即时模式，偏底层且灵活。两者都已有正式版本和用户，但看起来和用起来都不是原生的。
- **Wails** 把 Web 前端放进系统 WebView，后面是 Go：任何 Web 库都能用，但界面就是网页，而且只支持桌面。

## 其他语言的框架

- **Flutter 和 Compose Multiplatform** 自己绘制每个像素：各平台完全一致、成熟、生态庞大，但文本、滚动和输入是重新实现的而非原生，还需要学习 Dart 或 Kotlin。
- **React Native** 与 NSGo 模型相同，都是响应式核心驱动原生视图，并且拥有同类中最大的生态。它以 iOS 和 Android 为中心，应用逻辑运行在 JavaScript 中；NSGo 用 Go 运行逻辑，并把桌面和 Web 当作一等平台。
- **.NET MAUI** 用 C# 封装各平台的控件，不支持 Linux 和 Web。**Avalonia** 自己绘制，风格接近 WPF，覆盖所有桌面、手机和 Web。
- **Qt** 成熟、快速，还能用于嵌入式设备，但控件是自己绘制的，需要 C++ 或 QML，授权为 LGPL 或商业许可。
- **Electron 和 Tauri** 显示网页：可以完整使用 Web 的工具和库，代价是浏览器的内存占用（Electron 尤甚），以及网页般的界面手感。

## 如何选择

如果应用要在手机、桌面和 Web 上都有原生手感，并且只想维护一套 Go 代码，同时能接受早期项目，选择 NSGo。如果应用必须尽快基于稳定 API 发布，从上表中选一个成熟框架：手机选 Flutter 或 React Native，只做桌面选 MyGo、Wails 或 Tauri。

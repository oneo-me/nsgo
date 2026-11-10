---
title: 环境准备
description: 安装 NSGo 所需的环境，创建一个应用并在各个目标上运行。
group: 入门
order: 10
---

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
mkdir hello && cd hello
go mod init example.com/hello
go get github.com/oneo-me/nsgo
go get -tool github.com/oneo-me/nsgo/cmd/nsgo
go tool nsgo init
```

`go get -tool` 把 `nsgo` 加入模块一次，因此 `go tool nsgo` 无需安装任何东西就能运行。`init` 写出：

| 文件 | 内容 |
| --- | --- |
| `app.go` | 应用，一个计数器：一个 `Root` 组件，以及调用 `nsgo.Run(Root)` 的 `main` |
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

设备选择、追踪更新、性能监视器和其他语言见[运行与调试](debugging.md)。

## 下一步

- [示例：待办应用](tutorials/todo.md)：状态、列表和测试，大约十分钟。
- [示例：笔记应用](tutorials/notes.md)：导航栈和保存在设备上的笔记。
- [示例：GitHub 热门仓库](tutorials/repos.md)：从网络加载，带错误和重试。

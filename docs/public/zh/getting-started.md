---
title: 快速开始
description: 安装 nsgo 工具，创建应用，并以热重载方式运行。
group: 开始
order: 2
---

## 环境要求

- [Go](https://go.dev/dl/) 1.26 或更新版本。
- 构建 iOS 需要装有 Xcode 的 Mac。其他目标在 macOS 上只需 Go 即可构建；x86_64 Android 模拟器的目标另需 Android NDK。
- 在 Linux 上运行需要 GTK 4.14 或更新版本、Pango 和 Cairo；网页视图需要 WebKitGTK 6.0。

## 创建应用

新建一个模块，加入 NSGo 和它的工具，然后让工具写出一个应用：

```sh
mkdir hello && cd hello
go mod init example.com/hello
go get github.com/oneo-me/nsgo
go get -tool github.com/oneo-me/nsgo/cmd/nsgo
go tool nsgo init
```

`go get -tool` 把 `nsgo` 加入模块一次，之后 `go tool nsgo` 无需安装就能运行。`init` 会写出：

| 文件 | 内容 |
| --- | --- |
| `app.go` | 应用：`Root` 组件和调用 `nsgo.Run(Root)` 的 `main` |
| `app_test.go` | 一个按下按钮的测试 |
| `nsgo.yaml` | 应用的名称、标识、版本和构建目标 |

`go tool nsgo init -template tsx` 改为写出 TSX 应用，`-template mixed` 则写出一个 TSX 入口使用自有 Go 组件的 Go 应用。

## 运行

```sh
go tool nsgo dev
```

`dev` 为本机构建并打开应用。每次保存都会重新构建并重启应用；只改动 TSX 时，会直接更新正在运行的应用。构建失败会显示在应用上方，按 Command 或 Control+R 重新构建。

用 `-target` 在别处运行：

```sh
go tool nsgo dev -target web            # 在浏览器中
go tool nsgo dev -target ios_simulator  # 在 iOS 模拟器中
go tool nsgo dev -target android        # 在连接的设备或模拟器上
```

## 项目结构

在 `nsgo.yaml` 旁边，应用还可以有：

- `resources/`：随应用发布的文件，例如用 `asset:` 加载的图片和字体。
- `assets/`：不随应用发布的项目文件，例如 `AppIcon.icon` 和 `Icon.png`。
- `locales/`：应用的文案，每种语言一个 YAML 文件。

## 下一步

- [组件与状态](components.md)：应用是如何构建的。
- [测试](testing.md)：像用户一样测试应用。
- [构建与发布](building.md)：为每个平台打包。

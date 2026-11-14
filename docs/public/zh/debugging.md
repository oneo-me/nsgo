---
title: 运行与调试
description: 在任意目标上运行应用并热重载，查看它的错误、更新、性能和其他语言下的样子。
group: 工具链
order: 20
---

## 运行

```sh
go tool nsgo dev
```

`dev` 为本机构建应用并打开它，或者为 `nsgo.yaml` 中 `dev.target` 指定的目标构建。每次保存 Go 文件，它都会重新构建并重启应用；应用导入的包所在的 module 不在 `go.mod` 或 `go.sum` 中时，会先执行 `go mod tidy`；只改了 TSX 时，会在运行中的应用里原地更新，并保留状态。应用退出时运行结束。

`-target` 在别处运行，`-device` 按名称、UDID 或序列号选择 iPhone、Apple TV、Apple Watch、模拟器或 Android 设备：

```sh
go tool nsgo dev -target web                # 在浏览器中，端口为 -port（默认 8080）
go tool nsgo dev -target ios_simulator      # 在 iOS 模拟器中
go tool nsgo dev -target ios                # 在连接的 iPhone 上，由 Xcode 签名
go tool nsgo dev -target tvos_simulator     # 在 Apple TV 模拟器中
go tool nsgo dev -target watchos_simulator  # 在 Apple Watch 模拟器中
go tool nsgo dev -target android            # 通过 adb 在连接的设备或模拟器上
```

`-h` 列出全部参数。

## 错误

构建失败时，错误显示在运行中的应用上方，带有复制错误和重新构建的按钮；修好文件后，下一次保存就会替换它。组件渲染时 panic 也以同样方式显示。应用自己的输出（比如 `log.Print`）出现在终端里工具输出的下方。

运行前，`go tool nsgo check` 会检查应用并对它的 TSX 做类型检查；加上 `-target all` 会为每个目标再检查一遍，这样在 Mac 上也能检查到其他平台构建标签下的代码。

## 更新

```sh
go tool nsgo dev -highlight
```

`-highlight` 会把每次提交改动过的视图框出来半秒，使用应用时就能在屏幕上看到哪次更新重绘了超出预期的内容。这些框画在应用的主图层上，位于应用的模态视图之下。

```sh
go tool nsgo dev -trace
```

`-trace` 打印运行中应用的每次提交：序号、耗时，以及它发出的挂载项，这样能看到一次更新创建、修改或移除了哪些视图。挂载项上方会列出这次提交的起因：自上次提交以来写入 signal 的代码行（写得最多的排在前面），以及运行了多少个绑定、effect 和 memo。

```text
── commit 12: 40 items, commit 1.65 ms
   after app/market.go:473 ×4, app/market.go:487 ×4, app/market.go:384
   ran 118 bindings, 18 effects, 5 memos
```

某一行写了很多次，或者一次写入触发了大量绑定重算，就是该合并写入（`cx.Batch`）或缩小绑定读取范围的地方。`go tool nsgo test -trace` 会在测试的每一步下打印同样的内容。

## 性能

```sh
go tool nsgo dev -fps
go tool nsgo dev -release -fps
```

`-fps` 在 macOS、iOS、Windows 和 Linux 上于应用上方显示性能监视器：运行时帧和提交的开销。开发构建带有开发客户端；测量时请加 `-release`，它与 `nsgo build` 的构建方式相同。

## 语言与外观

```sh
go tool nsgo dev -lang zh-Hans
go tool nsgo dev -lang en-XA   # 加重音并拉长，找出未翻译和被截断的文字
go tool nsgo dev -lang ar-XB   # 从右到左
```

`-lang` 以某个语言区域启动应用，也可以是伪语言区域（[本地化](localization.md)）。

## 所有构建都接受的参数

应用的每个构建（包括发布包）在启动时都接受 NSGo 自己的参数。它们优先于 `nsgo.yaml` 和应用代码中的设置，也不会传到应用自己的参数解析中：

| 参数 | 作用 |
| --- | --- |
| `--nsgo-fps` | 在 macOS、iOS、Windows 和 Linux 上显示性能监视器 |
| `--nsgo-lang=zh-Hans` | 以某个语言区域启动应用，也可以是伪语言区域 |
| `--nsgo-appearance=dark` | 以 `light`、`dark` 或跟随系统（`system`）的外观启动应用 |

```sh
./Hello --nsgo-fps                                   # Windows 和 Linux
open Hello.app --args --nsgo-fps                     # macOS
xcrun simctl launch booted dev.example.hello --nsgo-fps
adb shell am start -n dev.example.hello/dev.oneo.nsgo.NsgoActivity --ez nsgo-fps true --es nsgo-lang de
```

在 Web 上，这些参数写在页面的查询字符串中：`?nsgo-fps&nsgo-lang=de`。

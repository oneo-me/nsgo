---
title: 代码结构
description: NSGo 的各部分在哪里，以及解释原因的文档。
group: 参与项目
order: 81
---

NSGo 是一个 Go module。应用从上层导入它的包；各平台的后端位于最底层，接收变化的部分，据此创建、更新和移除原生视图。

## 包

| 包 | 内容 |
| --- | --- |
| `nsgo`（根目录） | `nsgo.Run` 和 `nsgo.NewView`，按构建标签选择后端 |
| `core` | 应用的 API：元素、样式、class、主题、signal 和服务；布局与文本准备 |
| `base` | 无样式组件，每个一个文件，复杂组件各自成包（`base/dock`、`base/editor`） |
| `ui` | 带外观的 `base` 组件，以及 `ui/chat` 等包 |
| `mobile`、`desktop` | 只有手机或只有桌面才有的功能 |
| `tsx` | TSX 应用、它的打包器和编译器，以及它导入的模块 |
| `i18n`、`*/locales` | 消息目录和由它生成的代码 |
| `nsgotest` | 像用户一样操作的测试，无头或在平台上运行 |
| `internal/engine`、`internal/driver`、`internal/layout` | 引擎 goroutine、挂载协议和布局 |
| `internal/platform/<target>` | 一个后端：一个目标的原生视图、文本和输入 |
| `cmd/nsgo` | `nsgo` 工具 |
| `examples/` | Gallery 和其他示例应用 |

包只向下导入；`layering_test.go` 在每个目标上检查这张依赖图。

## 设计文档

[ARCHITECTURE.md](https://github.com/nsgo-dev/nsgo/blob/main/ARCHITECTURE.md) 描绘了各个包、线程，以及从一次提交到屏幕的流程，并列出 `docs/architecture/` 中的专题：每个部分一篇，比如布局、文本、滚动、主题和工具。修改某个部分前先读它的专题；专题记录了它如何工作，以及它在哪些地方不同于 NSGo 所移植的 [Native Shell](https://github.com/longbridge/nsh)。

[AGENTS.md](https://github.com/nsgo-dev/nsgo/blob/main/AGENTS.md) 是每个改动都要遵守的规则，对人和编码 agent 都适用。其中几条影响大多数改动：

- 每个 API 只有一个归属：元素和环境在 `core`，控件在 `base`，控件的外观在 `ui`，某类设备专有的在 `mobile` 和 `desktop`。
- `base` 没有外观；`ui` 组件就是加上外观的 `base` 形式。
- 不按设备隐式改变任何东西：`core` 和 `base` 中没有在桌面上变大、在手机上变小的控件。
- 处处使用原生视图，各平台表现一致：某个平台原生具备的东西，其他平台用同样的方式模仿。
- 不手写 C；除 x86_64 Android 模拟器外，构建所有目标只需要 macOS、Xcode 和 Go。

各平台还缺什么，记录在[路线图](https://github.com/nsgo-dev/nsgo/blob/main/docs/roadmap.md)中。

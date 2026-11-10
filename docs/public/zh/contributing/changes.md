---
title: 提交改动
description: 向 NSGo 提交 pull request 需要什么：每个后端的同样改动、测试、文档和一行更新日志。
group: 参与项目
order: 82
---

## 开始之前

比修复更大的改动，请先开一个 issue 或 discussion，在写代码之前就设计达成一致。设计中尚未定论的问题列在 [ARCHITECTURE.md](https://github.com/oneo-me/nsgo/blob/main/ARCHITECTURE.md#open-questions) 中，先在那里定下答案。

## 每个后端

一个后端里的修复或功能，通常其他后端也需要。在每个需要的目标上做同样的改动，或者把缺少的部分记到[路线图](https://github.com/oneo-me/nsgo/blob/main/docs/roadmap.md)里。组件在每个平台上的外观和行为都应一致。

## 检查

使用 `nsgo` 工具，而不是 `go vet`、`go test` 或 `go build`：

```sh
go tool nsgo check   # 检查所有包
go tool nsgo test    # 运行所有包的测试
```

为改动加上测试：用户看到的内容用 `nsgotest`，其余用包自己的测试。在改动涉及的平台上运行 Gallery（[开发环境](../contributing.md#gallery)）。

## 文档与更新日志

- 更新事实发生变化的文档：NSGo 如何工作写在 `docs/architecture/` 的专题里，应用和组件如何使用它写在 `docs/public/` 的页面里（[文档与网站](docs.md)）。
- `README.md` 和 `README.zh-CN.md` 保持一致；其余内容都用英文，中文页面位于 `docs/public/zh/`。
- 用户能看到的改动，在 `CHANGELOG.md` 的 `Unreleased` 下记一行，并附上引用，例如 `(#12 by @you)`。

---
title: 文档与网站
description: 编写和翻译一页文档，并运行网站。
group: 参与项目
order: 83
---

## 一个页面

这些文档就是 `docs/public/` 中的 Markdown，在 GitHub 上也能直接阅读。`en/` 和 `zh/` 按路径一一对应；缺少中文版的页面会显示英文版并附上提示。每个页面以 front matter 开头：

```yaml
---
title: Testing
description: Test an app the way a user uses it, headless or in a native window.
group: Toolchain
order: 21
---
```

- `group` 是页面在侧栏中的分组，用页面自己的语言书写，`order` 决定它的位置。分组按读者的上手顺序排列：入门教程、工具链、组件、核心概念，然后是编写组件和参与项目。没有分组的页面（比如简介）排在它们前面。
- 布局会显示标题和描述，所以正文从 `##` 开始。

页面之间用相对的 `.md` 链接（`[测试](testing.md#在平台上运行)`），网站会把它们转换成自己的 URL。每个页面只回答一个问题；NSGo 内部如何工作留在 `docs/architecture/` 中。

## 案例

案例页面列出 `docs/public/cases/` 中的应用，每个应用一个 Markdown 文档和一张截图；该目录的 README 说明了写法。

## 网站

网站是 `website/` 中的 Astro 站点，需要 [Bun](https://bun.com) 1.4 或更新版本：

```sh
cd website
bun install
bun run dev    # 访问 http://localhost:4321/nsgo/
bun run build  # 类型检查并构建，与部署时相同
```

`website/README.md` 说明了网站如何构建和发布。

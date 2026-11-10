---
title: 文本
description: 段落、span、链接、字体和文档。
group: 组件
order: 61
---

`core.Text` 是一个由平台文本引擎排版的段落。`ui.Text` 和 `ui.Textf` 是 `ui` 字体下的一行辅助函数；元素仍然是 `core.Text`，因此同样接受 `Style` 和 `Spans`。

```go
ui.Text("设置", 20, ui.Bold, colors.Text)
ui.Textf(func() string { return fmt.Sprintf("%d 个文件", n.Get()) }, 13, ui.Regular, colors.Muted)
```

## 内容

`Content` 是固定内容，`ContentFunc` 跟随 signal。`MaxLines` 限制行数，`EllipsizeMode` 在开头、中间或结尾截断；`Style.WhiteSpaceNoWrap` 让段落保持一行。`TextAlign`、`TextTransform`、`LineHeight` 和 `LetterSpacing` 设置段落样式，`OnTextLayout` 在布局后报告行。

```go
core.Text{ContentFunc: func() string { return fmt.Sprintf("按了 %d 次", count.Get()) },
	MaxLines: 2, EllipsizeMode: core.EllipsizeTail, Style: core.Style{FontSize: 15}}
```

## Span 与链接

`Spans` 把文本拆成若干段，每段有自己的样式。带 `OnPress` 的 span 是链接，它的 `Background` 和 `CornerRadius` 在每一行的字符后面绘制。`SpansFunc` 在读取的值变化时重新给出 span，适合代码高亮或会变化的句子。

```go
core.Text{Style: core.Style{FontSize: 15}, Spans: []core.Span{
	{Content: "开始前请阅读"},
	{Content: "指南", Style: core.Style{TextDecoration: core.TextDecorationUnderline}, OnPress: openGuide},
	{Content: "。"},
}}
```

`Attachment` 把一个视图放进一行，替代 span 的内容：图标、图片或小按钮。它按自身尺寸布局，随文字换行，并接收自己的按压。

## 字体与样式

`FontFamily` 指定一个字体系列或 CSS 列表。`FontSize`、`FontWeight`、`FontStyle`、`FontVariant`、`LineHeight`、`LetterSpacing`、`TextDecoration`，以及 `TextShadowColor` 及其偏移和半径设置文本外观。`core.FontMonospace` 是平台的等宽字体。

```go
core.Text{Content: "fn main() {}", Style: core.Style{FontFamily: core.FontMonospace, FontSize: 13}}
```

用 `nsgo.Fonts` 在第一个视图前注册随包发布的字体，或在运行时用 `cx.RegisterFont` 注册字体面。字体到达前排版的文本会重新排版。

## 文档

`base/textview.TextView` 用相同的原语显示 Markdown 或 HTML：标题、段落、列表、表格、代码、脚注、提示块、图片、公式和链接。`textview.Parse` 读取 Markdown，`textview.ParseHTML` 读取 HTML，`Options.Streaming` 修复仍在到达的文档。

```go
textview.TextView{Markdown: "## 笔记\n\n一份 **原生** 文档。", Styles: ui.TextViewStyles(cx),
	OnLinkPress: func(e textview.LinkPressEvent) { open(e.URL) }}
```

## 选择与无障碍

`SelectableOn` 让用户选择和复制文本。带 `Selectable` 的 `core.View` 让其中的文本按文档顺序一起选择。`RoleHeader` 之类的 `Role` 告诉辅助技术这段文字是什么，`Label` 在原文不适合朗读时替换它。

## 平台

平台自己的文本引擎负责排版，因此换行、字体和输入法跟随系统。CJK 换行、emoji 和从右到左的文字按系统处理。应用可以用 `cx.Env().FontScale()` 读取文本大小；除非设置 `NoFontScaling`，文本会跟随它。

## 相关

- [文本](../text.md)：文本、span、字体和文档的完整指南。
- [图片与图标](../images.md)：行内图片和图标。
- [无障碍](../accessibility.md)：标题、标签和选择。
- gallery 的 Text 和 TextView 页面。

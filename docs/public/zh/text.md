---
title: 文本
description: 用 span、字体、装饰、选择和文档展示文本。
group: 核心概念
order: 53
---

## 一段文本

`core.Text` 是一个段落，由平台自己的文本引擎在引擎侧排版，再由主线程绘制。它的尺寸来自字体和宽度，并在盒子内换行。

```go
core.Text{Content: "Hello from NSGo", Style: core.Style{
	FontSize:   15,
	LineHeight: 22,
	FontWeight: core.FontWeightSemibold,
}}
```

`Content` 是固定内容，`ContentFunc` 跟随 signal，因此会变化的文本在函数里读取它。`MaxLines` 限制显示的行数，`EllipsizeMode` 在开头、中间或结尾截断，`WhiteSpaceNoWrap` 让段落保持一行。`AdjustsFontSizeToFit` 会缩小文本，直到它放进盒子和行数限制。

文本字段像 CSS 一样继承：位于设置了 `FontSize`、`Color` 或 `LineHeight` 的视图中的文本会采用这些值，除非自己设置。`TextAlign`、`TextTransform`、`LetterSpacing`、`TextDecoration`、`TextDecorationStyle`、`TextDecorationColor`，以及 `TextShadowColor`、`TextShadowOffset` 和 `TextShadowRadius` 设置行与字形的样式。

## Span 与链接

`Spans` 把文本拆成若干段，每段有自己的样式。带 `OnPress` 的 span 是链接；按在其他 span 上会触发文本自己的 `OnPress`。

```go
core.Text{Style: core.Style{FontSize: 15}, Spans: []core.Span{
	{Content: "继续即表示你同意"},
	{Content: "服务条款", Style: core.Style{TextDecoration: core.TextDecorationUnderline}, OnPress: openTerms},
	{Content: "和"},
	{Content: "隐私政策", Style: core.Style{TextDecoration: core.TextDecorationUnderline}, OnPress: openPrivacy},
	{Content: "。"},
}}
```

`SpansFunc` 在读取的值变化时重新给出 span，只有不同的部分会重新排版，因此代码编辑器的行会在高亮到达时重新着色。span 的 `Background` 和 `CornerRadius` 会在每一行字符后面画一个盒子。

`Attachment` 把一个视图放进文本，替代 span 的内容。它按自身尺寸布局，随周围的文字换行，并接收自己的按压，因此图标、图片或小按钮可以出现在一行里。

```go
core.Text{Style: core.Style{FontSize: 17, LineHeight: 24}, Spans: []core.Span{
	{Content: "用 "},
	{Attachment: core.Icon{SVG: lucide.Search, Style: core.Style{FontSize: 17}}},
	{Content: " 搜索 NSGo"},
}}
```

## 字体

`FontFamily` 指定一个字体系列或 CSS 列表（`Inter, "Noto Sans SC", sans-serif`），前面的字体缺少的字符由后面的字体补上。`FontWeight`、`FontStyle`、`FontVariant`、`LineHeight`、`LineHeightMultiple`、`LetterSpacing` 和 `LetterSpacingEm` 设置字形和间距，`core.FontMonospace` 是平台等宽字体。

把应用的字体放在 `resources/` 下并在第一个视图前注册，或在运行时注册字体字节：

```go
func main() { nsgo.Run(Root, nsgo.Fonts("fonts/Inter-Regular.ttf", "fonts/Inter-Bold.ttf")) }

cx.RegisterFont(data) // 一个 TrueType 或 OpenType 字体
```

字体到达前排版的文本会重新排版。`cx.Env().FontScale()` 是用户的文本大小，文本默认跟随它，`NoFontScaling` 可以关闭，`MaxFontSizeMultiplier` 限制最大倍数。

## 选择与测量

`SelectableOn` 让用户选择和复制文本，带 `Selectable` 的 `View` 让其中的文本按文档顺序一起选择，像网页一样。`SelectionColor` 设置高亮颜色。`cx.MeasureText` 测量一个字符串在某个字体下的尺寸，适合按文本宽度排列的列；`OnTextLayout` 在每次布局后报告行。

## 文档

`base/textview` 用相同的原语显示 Markdown 或 HTML：标题、段落、列表、表格、代码、脚注、提示块、图片、公式和链接。

```go
textview.TextView{
	Markdown: "## NSGo\n\n用 **Go** 写的原生视图。",
	Styles:   ui.TextViewStyles(cx),
	OnLinkPress: func(e textview.LinkPressEvent) {
		if strings.HasPrefix(e.URL, "#") {
			return
		}
		cx.OpenURL(e.URL, nil)
	},
}
```

`textview.Parse` 读取 Markdown，`textview.ParseHTML` 读取 HTML，得到应用也可以自己构建的块。`Options.Streaming` 修复仍在到达的文档，`StreamingCaret` 在末尾绘制光标，适合逐字生成的回复。

## 下一步

- [图片与图标](images.md)：图片和 SVG，也可以放进文本。
- [样式与主题](styles.md)：可继承的颜色和文本字段。
- [布局](layout.md)：文本如何取得宽度。

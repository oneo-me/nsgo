---
title: 本地化
description: 翻译应用的文案，并跟随应用显示的语言。
group: 指南
order: 45
---

## 文案

`nsgo.yaml` 旁边的 `locales/` 保存应用的文案，每种语言一个 YAML 文件（`en.yaml`、`de.yaml`），采用 ICU MessageFormat：

```yaml
greeting: Hello, {name}!
appName:
  message: Hello
  description: The app's name, for translators.
```

一条文案可以是一个字符串，也可以是带有供译者参考的 `description` 和 `placeholders` 的 `message`。

应用的包会导入 `locales/`。`dev`、`build` 和 `test` 会检查这些文案，并在旁边为每条文案写出一个 Go 函数，例如 `locales.Greeting(cx, "Ada")`，它用应用当前显示的语言说出这条文案，并随语言变化而更新；`go tool nsgo i18n` 只写出这些函数。文案可以格式化数字和日期（`{d, date, medium}`），`appName` 则是应用在各语言安装包中的名称。

## 语言

`cx.SetLocale("de")` 选择语言，布局会按该语言的书写方向排列。

```sh
go tool nsgo dev -lang en-XA
```

会以伪语言运行应用，显示未翻译和被截断的文本。任何构建在启动时都接受这个参数：`--nsgo-lang=en-XA`。

## NSGo 自己的文案

NSGo 的组件以 NSGo 已翻译的语言显示自己的文案。对于其他语言，以包名为键（例如 `github.com/oneo-me/nsgo/base/locales`）即可用该文件的语言提供这个包的文案。

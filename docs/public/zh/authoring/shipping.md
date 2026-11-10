---
title: 文案、测试与发布
description: 翻译组件说的话，像用户一样测试它，并提供给 Go 和 TSX 应用。
group: 编写组件
order: 74
---

## 文案

组件自己要说的话，比如按钮给读屏器的名称，通过所在包的目录文件提供，而不是写成字符串字面量。在组件库中放一个 `locales/` 包，包含源语言的目录文件，以及一个生成代码的文件：

```yaml
# locales/en.yaml
clear:
  message: Clear
  description: What screen readers say of the button that empties a chip input.
```

```go
// Package locales says the chips' words in the language the app shows.
package locales

//go:generate go tool nsgo i18n
```

`go generate ./locales` 为每条消息生成一个函数，组件用自己的 scope 调用：`locales.Clear(cx)` 以应用显示的语言说出它。把生成的 `catalog_gen.go` 一并提交，应用只用 Go 就能构建。为每种已翻译的语言加一个文件（`zh-Hans.yaml`）；其他语言由应用在自己的目录文件中提供，键名是你的包的导入路径，与 [NSGo 自己的文案](../localization.md#nsgo-自己的文案)相同。

## 测试

`nsgotest` 以用户的方式无头运行组件。把它挂在一个小的根组件里，按角色和文字操作它：

```go
func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestToggle(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		on := core.NewSignal(cx, false)
		return chips.Toggle{On: on.Get, OnChange: on.Set, Label: "Bold"}
	})
	app.Press("role:switch Bold")
	if !app.Find("role:switch Bold").Props.Checked {
		t.Fatal("the toggle did not turn on")
	}
}
```

在组件库根目录运行 `go tool nsgo test` 会测试所有包，加上 `-target macos` 或其他目标则在原生窗口中运行同样的测试。在两种外观和伪语言（`-lang en-XA`）下都测一遍，可以发现写死的颜色和文案。其余内容见[测试](../testing.md)。

## 发布

组件库就是一个 Go module：应用 `go get` 它并导入其中的包。同时有无样式和带样式两种形式时，把它们放在不同的包里，这样有自己外观的应用不会导入用不到的样式。

TSX 应用在 Go 组件注册后就能使用它们，注册写在 `init` 函数中：

```go
func init() {
	tsx.Register("example.com/chips", map[string]core.Node{"Toggle": Toggle{}})
}
```

之后 `import { Toggle } from "example.com/chips"` 即可把字段作为 props 使用（[TSX 应用](../tsx.md#在-tsx-中使用-go)）。

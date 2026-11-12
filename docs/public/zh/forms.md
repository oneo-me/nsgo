---
title: 表单
description: 文本框、焦点、软件键盘、开关、选择器和校验。
group: 核心概念
order: 57
---

## 文本框

`ui.Input` 是 `ui` 外观的平台文本框，字段与 `base.Input` 相同。设置 `Value` 后它是受控的：应用保存值并在 `OnChange` 中更新；设置 `DefaultValue` 后它自己保存值。

```go
name, email := core.NewSignal(cx, ""), &core.Ref{}

ui.Input{
	Value:         name.Get,
	OnChange:      name.Set,
	Placeholder:   "你的名字",
	ReturnKeyType: core.ReturnKeyNext,
	OnSubmit:      func(string) { email.Focus() },
}
```

`Masked` 隐藏字符，`Readonly` 和 `Disabled` 阻止编辑，`Multiline` 让文本换行并随之增长，除非样式指定了尺寸。`KeyboardType` 选择软件键盘（`core.KeyboardEmail`、`core.KeyboardNumber`、`core.KeyboardURL` 等），`AutoComplete` 告诉系统字段内容以便自动填充，`MaxLength` 限制用户输入的 UTF-16 单元数。`AutoCapitalize` 和 `NoAutoCorrect` 调整键盘行为。

`OnSelectionChange` 和 `OnCaretMove` 报告选区和光标位置，用于在光标处打开菜单。`OnKeyPress` 听到每个按键，`OnPaste` 听到粘贴内容，`OnCompositionChange` 跟随输入法组合。

## 焦点与键盘

`Ref` 让文本框获得或失去焦点，`cx.DismissKeyboard()` 关闭软件键盘。`OnFocus` 和 `OnBlur` 跟随焦点，`View.OnFocus`/`OnBlur` 对任何可聚焦视图做同样的事。

```go
email := &core.Ref{}
ui.Input{Ref: email, Placeholder: "you@example.com"}
email.Focus()
```

软件键盘会盖住手机底部。把 `core.EnvKeyboard` 用作底部内边距或外边距，内容就会停在键盘上方，并随键盘的移动一起动画：

```go
core.View{Style: core.Style{
	Padding:    core.All(16),
	PaddingEnv: core.EnvEdges{Bottom: core.EnvKeyboard},
	Gap:        12,
}, Children: form}
```

表单外的滚动视图可以设置拖动时收起键盘，以及键盘显示时点击的处理方式：

```go
core.Scroll{Behavior: core.ScrollBehavior{
	KeyboardDismissMode:       core.KeyboardDismissOnDrag,
	KeyboardShouldPersistTaps: core.KeyboardTapsHandled,
}, Children: []core.Node{form}}
```

`cx.Env().KeyboardHeight()` 和 `cx.Env().KeyboardDuration()` 在代码中读取键盘的尺寸和时间，`cx.Env().SafeArea()` 读取屏幕四周的内边距。

## 选择

`ui.Switch` 是立即生效的设置，`ui.Checkbox` 有选中、未选中和不确定三种状态，`ui.Radio` 是 `ui.RadioGroup` 中的一个选项。`ui.Select[T]` 在触发器下方打开选项列表，`Presentation` 要求时作为 sheet 打开；`ui.Tabs[T]` 同时显示少量选项。

```go
notifications := core.NewSignal(cx, true)
ui.Switch{Checked: notifications.Get, OnChange: notifications.Set, Label: "通知"}

size := core.NewSignal(cx, "m")
ui.Select[string]{
	Options:  []ui.Option[string]{{Value: "s", Label: "小"}, {Value: "m", Label: "中"}, {Value: "l", Label: "大"}},
	Value:    size.Get,
	OnChange: size.Set,
	Label:    "尺寸",
}
```

`ui.NumberInput` 带边界地步进和拖动数字，`ui.DatePicker` 和 `ui.TimeField` 编辑日期和时间，`ui.ColorPicker` 编辑颜色，`ui.OtpInput` 输入一次性验证码，`ui.TagInput` 编辑一组标签。

```go
quantity := core.NewSignal(cx, 1.0)
ui.NumberInput{Value: quantity.Get, OnChange: quantity.Set, Min: 1, Max: 99, Step: 1, Label: "数量"}
```

## 校验

规则由应用自己保存：一个 signal 保存错误，`OnBlur` 或 `OnSubmit` 设置它，有错误时 `core.Show` 在输入框下方添加提示。

```go
address := core.NewSignal(cx, "")
err := core.NewSignal(cx, "")
colors := ui.ColorsOf(cx)
core.View{Style: core.Style{Gap: 4}, Children: []core.Node{
	ui.Input{
		Value:    address.Get,
		OnChange: func(v string) { address.Set(v); err.Set("") },
		Label:    "邮箱",
		OnSubmit: func(string) {
			if !strings.Contains(address.Get(), "@") {
				err.Set("请输入邮箱地址。")
			}
		},
	},
	core.Show{
		When: func() bool { return err.Get() != "" },
		Then: func(*core.Scope) core.Node { return ui.Textf(err.Get, 12, ui.Regular, colors.Danger) },
	},
}}
```

`cx.AnimateLayout(core.LayoutEaseInEaseOutPreset)` 让出现的提示有动画，带 `ReturnKeyNext` 的 `OnSubmit` 把键盘移到下一个字段。

## 下一步

- [导航](navigation.md)：表单所在的页面。
- [文本](text.md)：文本框的文本如何设置样式。
- [无障碍](accessibility.md)：屏幕阅读器需要的标签和值。

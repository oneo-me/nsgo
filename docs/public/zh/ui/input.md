---
title: 输入框
description: 平台的文本框，受控或非受控，带焦点和软件键盘。
group: 组件
order: 62
---

`base.Input` 是平台的文本框，除非设置 `Multiline`，否则只有一行。`ui.Input` 是同一个输入框在 Onevra 外观下的形式，`ui.Field` 则把它放进带边框的字段容器。

```go
name := core.NewSignal(cx, "")
ui.Input{Input: base.Input{
	Value:       name.Get,
	OnChange:    name.Set,
	Placeholder: "你的名字",
	TestID:      "name",
}}
```

## 受控与非受控

设置 `Value` 后输入框是受控的：应用保存值，并在 `OnChange` 中更新它。设置 `DefaultValue` 后输入框自己保存值。受控输入框可以通过把值设为 `""` 来清空。

```go
draft := core.NewSignal(cx, "")
base.Input{Value: draft.Get, OnChange: draft.Set, Placeholder: "新任务",
	OnSubmit: func(string) { add(); draft.Set("") }}
```

## 字段

`Multiline` 让文本换行并随之增长，除非样式指定了尺寸。`Masked` 隐藏字符，`Readonly` 和 `Disabled` 阻止编辑，`MaxLength` 限制用户输入的 UTF-16 单元数。`Placeholder` 可以用 `PlaceholderFunc` 跟随变化。

```go
base.Input{Masked: true, Value: password.Get, OnChange: password.Set,
	KeyboardType: core.KeyboardDefault, AutoComplete: core.AutoCompletePassword}
```

`KeyboardType` 选择软件键盘（`KeyboardEmail`、`KeyboardNumber`、`KeyboardURL` 等），`ReturnKeyType` 设置回车键的标签，`SubmitBehavior` 决定回车是提交还是换行。`AutoCapitalize`、`NoAutoCorrect` 和 `AutoComplete` 调整系统的辅助行为。

## 焦点与事件

`Ref` 让输入框获得或失去焦点。`OnFocus` 和 `OnBlur` 跟随焦点，`OnSelectionChange` 和 `OnCaretMove` 跟随选区和光标，`OnKeyPress` 收到每个按键，`OnPaste` 收到粘贴内容，`OnCompositionChange` 跟随输入法组合。

```go
email := &core.Ref{}
base.Input{Ref: email, Placeholder: "you@example.com"}
email.Focus()
```

软件键盘会盖住手机底部。把 `core.EnvKeyboard` 放在内容的底部内边距或外边距上，内容就会停在键盘上方，并随键盘的移动一起动画。`cx.DismissKeyboard()` 关闭键盘，`cx.Env().KeyboardHeight()` 读取高度。

## 校验

规则由应用自己保存。一个 signal 保存错误，`OnBlur` 或 `OnSubmit` 设置它，`StyleFunc` 根据它给输入框上色。错误文本如果是 `core.Text` 并设置 `LiveRegionPolite`，屏幕阅读器会朗读它。

```go
err := core.NewSignal(cx, "")
base.Input{Value: email.Get, OnChange: email.Set,
	OnSubmit: func(string) { if !strings.Contains(email.Get(), "@") { err.Set("请输入邮箱地址。") } },
	StyleFunc: func() core.Style {
		if err.Get() != "" { return core.Style{BorderWidth: 1, BorderColor: core.RGB(0xb4444c)} }
		return core.Style{}
	}}
```

## 无障碍

输入框报告 `RoleTextField` 并朗读 `Label`；`Placeholder` 不是标签，因此没有可见标签的输入框要设置 `Label`。`Readonly` 和 `Disabled` 会被报告，`OnSubmit` 是键盘上的动作。

## 平台

iOS、Android、Windows 和 Linux 使用各自的原生文本框；Web 使用 `<input>` 或 `<textarea>`。软件键盘、自动填充、输入法和文字方向属于平台。焦点和输入在每个目标上都可以用相同的方式测试。

## 相关

- [表单](../forms.md)：输入框、校验、选择和键盘。
- [无障碍](../accessibility.md)：标签和焦点。
- [布局](../layout.md)：让内容停在键盘上方。
- gallery 的 Input 和 Form 页面。

---
title: 平台服务
description: 警告框、剪贴板、文件、照片、分享、存储、语音、网络和应用生命周期。
group: 核心概念
order: 60
---

## 警告框与剪贴板

`cx.ShowAlert` 显示平台的警告框或操作表。`Actions` 按下标返回，角色会让平台用对应方式绘制取消和危险按钮。

```go
cx.ShowAlert(core.AlertConfig{
	Title:   "删除这个文件？",
	Message: "此操作无法撤销。",
	Actions: []core.AlertAction{
		{Label: "取消", Role: core.AlertCancel},
		{Label: "删除", Role: core.AlertDestructive},
	},
}, func(index int) {
	if index == 1 {
		remove()
	}
})
```

`SetClipboard` 写入文本，`SetClipboardImage` 写入图片。`ReadClipboard`、`ReadClipboardImage` 和 `ReadClipboardFiles` 异步回答，`HasClipboardText` 和 `HasClipboardImage` 只读取剪贴板内容，不会触发粘贴提示。

```go
cx.ReadClipboard(func(text string) { pasted.Set(text) })
cx.SetClipboard("Hello")
```

## 打开链接与文件

`cx.OpenURL` 用系统的浏览器或应用打开链接，`core.CheckURL` 说明它会拒绝什么：没有 scheme、超过 16 KiB、包含空格、Web 地址没有 host，以及 `javascript:`、`data:`、`blob:`、`file:`、`about:` 和 `asset:`。

```go
if err := core.CheckURL(url); err == nil {
	cx.OpenURL(url, func(opened bool) {
		if !opened {
			status.Set("没有应用能打开这个链接。")
		}
	})
}
```

`cx.OpenFile` 用系统关联的应用打开应用持有的文件，`cx.Share` 显示系统分享面板，可以分享文件、文本或两者。`core.SharedFile` 是一个路径，以及另一个应用看到的名称和类型。

```go
cx.Share(core.ShareOptions{
	Files: []core.SharedFile{{Path: path, Name: "Notes.txt"}},
	Text:  "来自 NSGo 的笔记",
}, func(shown bool, err error) {})
```

## 照片与文件

`cx.PickImages` 显示系统照片选择器，返回 `core.PickedFile`，其中包含字节、名称、类型和像素尺寸。`cx.TakePhoto` 在平台有相机时打开相机；`cx.Env().CanTakePhoto` 说明是否有。

```go
cx.PickImages(core.PickOptions{Limit: 1, MaxDimension: 2048}, func(files []core.PickedFile) {
	if len(files) == 1 {
		avatar.Set(files[0].Data)
	}
})
```

`cx.PickFiles` 选择任意类型的文件，`cx.PickDirectories` 把文件夹选成 `fs.FS`，`cx.SaveFile` 通过系统保存对话框保存文件或字节。`cx.PickScreenColor` 在平台允许时从屏幕任意位置取色。

## 存储

`cx.Storage()` 是应用的键值存储，保存在设备上，应用的每个响应式图共享它。读取是同步的，在计算中读取会让它订阅变化；超过配额时 `SetItem` 返回 `core.StorageFullError`（默认 `core.StorageQuota` 为 5 MiB）。

```go
store := cx.Storage()
if token, ok := store.GetItem("token"); ok {
	login(token)
}
store.SetItem("token", token)
```

`store.Keys()` 列出键，`RemoveItem` 删除一个，`Clear` 清空存储。`cx.DataDir()` 和 `cx.CacheDir()` 给出应用存放键值存储放不下的文件的目录；`Scope.ProvideStorage` 给子树一个独立分区。

## 网络

Go 自带的 `net/http` 在每个目标上都能发起请求，使用系统的 TLS 和代理设置。signal 属于引擎的 goroutine，因此其他 goroutine 得到的响应要通过 `cx.Post` 交给它。

```go
go func() {
	resp, err := http.Get("https://api.example.com/items")
	if err != nil {
		cx.Post(func() { errText.Set(err.Error()) })
		return
	}
	defer resp.Body.Close()
	var items []Item
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		cx.Post(func() { errText.Set(err.Error()) })
		return
	}
	cx.Post(func() { list.Set(items) })
}()
```

`cx.Post` 在响应式图的 goroutine 上运行函数，因此可以在那里设置 signal。Android 的清单默认申请网络权限，图片下载会缓存在磁盘上。

## 语音与生命周期

`cx.StartSpeech` 在 `cx.Env().CanRecognizeSpeech()` 为真时用系统识别器识别语音，并随着转写结果变化返回文本。

```go
session, err := cx.StartSpeech(core.SpeechOptions{
	Lang:    "zh-CN",
	Interim: true,
	OnResult: func(text string, final bool) {
		transcript.Set(text)
	},
})
if err == nil {
	defer session.Stop()
}
```

`cx.Env().Lifecycle()` 说明应用处于前台、非活动还是后台，`cx.OnExit` 在退出时运行。`cx.SetAppearance` 强制浅色或深色，`cx.SetLocale` 选择语言，`cx.OnKey` 绑定键盘快捷键，`cx.RegisterHotKey` 绑定任何应用在前台时都生效的快捷键，`cx.OnOpenFiles` 听到 macOS 应用打开的文件。

```go
cx.OnKey("mod+k", func() { palette.Set(true) })
cx.SetLocale("de")
```

## 下一步

- [表单](forms.md)：文本框和键盘。
- [嵌入](embedding.md)：宿主应用提供给嵌入应用的服务。
- [构建与发布](building.md)：应用包需要的权限和文件。

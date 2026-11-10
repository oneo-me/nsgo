---
title: 嵌入
description: 把 NSGo 应用放进你自己的 iOS、Android、桌面或网页应用。
group: 指南
order: 55
---

## 构建可嵌入的应用

可嵌入的应用就是普通的应用包，带 `Root` 和 `main`，用 `-embed` 按宿主的平台构建。每个视图运行一次应用，并且是独立的引擎，因此宿主可以显示任意多个视图，每个都有自己的状态。

```sh
go tool nsgo build -embed -target ios examples/embedded
go tool nsgo build -embed -target android_arm64 examples/embedded
go tool nsgo build -target web examples/embedded
```

iOS 构建写出 `<Name>.xcframework` 和 `<Name>Resources`；Android 构建写出 `<Name>.aar`；Web 构建写出包含 `nsgo.json` 和加载器的目录。

## iOS

把框架作为静态库加入应用目标（不要嵌入），把资源目录作为文件夹引用加入，然后 `import <Name>`。在应用启动时、第一个视图之前调用一次 `NSGoStart()`。

```swift
import Embedded
import UIKit

@main
final class AppDelegate: UIResponder, UIApplicationDelegate {
	func application(_ app: UIApplication, didFinishLaunchingWithOptions: [UIApplication.LaunchOptionsKey: Any]?) -> Bool {
		NSGoStart()
		return true
	}
}

let view = NSGoView(frame: .zero)
view.params = ["user": "Ada"]              // cx.HostParams()
view.dataDirectory = documents.path        // 存储所在的位置
view.localeIdentifier = "de-CH"            // 替换系统语言
view.setFontScale(1.3)
view.overrideUserInterfaceStyle = .dark
view.tokens = ["accent": "#cf222e"]        // 覆盖应用主题
view.handle("time") { _, reply in reply(Date().description, nil) }
view.send("message", ["text": "Hello"])    // cx.OnHostEvent("message", …)
view.onOpenURL = { url in router.open(url) }
view.onError = { message in log(message) }
view.register(component: "host.badge") { context in Badge(context) }
```

`NSGoView` 在进入窗口后启动应用，在释放或收到 `stop()` 时停止。启动值要在进入窗口前设置，其余值可以随时改变。处理器、组件和回调在主线程运行，回复可以从任意线程稍后到达。转发状态栏的视图控制器返回视图的 `preferredStatusBarStyle` 和 `prefersStatusBarHidden`。

## Android

把 AAR 加入应用模块，并让原生库被解压，以便运行时找到它们：

```kotlin
implementation(files("libs/Embedded.aar"))

android { packaging { jniLibs { useLegacyPackaging = true } } }
```

在代码或布局中创建 `dev.oneo.nsgo.NsgoView`。每个视图在第一次获得尺寸时启动一个独立进程的应用，`stop()` 结束它。把 Activity 的结果转发给每个视图，并在 Activity 中自行处理配置变化，以免应用重启。

```java
NsgoView app = new NsgoView(context);
app.setParams(Map.of("user", "Ada"));
app.setStorageDirectory(dir);
app.setLocale(Locale.forLanguageTag("de-CH"));
app.setFontScale(1.3f);
app.setAppearance(NsgoView.APPEARANCE_DARK);
app.setTokens(Map.of("accent", "#cf222e"), null);
app.handle("time", (data, reply) -> reply.resolve(now()));
app.handle("tapped", (data, reply) -> status.setText("tapped " + ((Map<?, ?>) data).get("count")));
app.setOpenUrlHandler(url -> router.open(url));
app.setErrorListener(message -> log(message));
app.registerComponent("host.badge", Badge::new);
```

```java
@Override
protected void onActivityResult(int request, int result, Intent data) {
	super.onActivityResult(request, result, data);
	for (NsgoView app : apps) app.activityResult(request, result, data);
}
```

## 桌面

Go 宿主运行平台的事件循环，把每个视图的 `Handle` 放进自己的窗口。`nsgo.NewView` 在 handle 进入窗口后启动应用，因此之前设置的选项会出现在第一帧。

```go
channel := core.NewHost()
channel.SetParams(map[string]any{"user": "Ada"})
channel.Handle("time", func(c *core.HostCall) { c.Resolve(time.Now().Format("15:04:05")) })
channel.HandleOpenURL(func(url string, answer func(bool)) {
	answer(strings.HasPrefix(url, "https://example.com/orders/"))
})

view := nsgo.NewView(app.Root, nsgo.HostChannel(channel), nsgo.DataDir(dataDir))
defer view.Close()

view.SetTokens(core.Tokens{Values: map[string]string{"accent": "#cf222e"}})
view.SetLocale("de-CH")
view.SetFontScale(1.3)
view.SetAppearance(core.AppearanceDark)
handle := view.Handle() // NSView、GtkWidget 或子窗口
```

`view.Host()` 向应用发送事件并接收应用的调用，`nsgo.Component` 为宿主平台注册原生组件。

## Web

Web 构建本身是一个页面，但页面也可以把它挂载到一个元素里，并成为它的宿主。`mount(element, url, options)` 返回一个 player，`nsgo.json` 给出相对于自身的加载器、wasm 和资源。

```html
<div id="app"></div>
<script type="module">
const url = new URL("./Embedded-0.1.0-web/nsgo.json", location.href);
const config = await (await fetch(url)).json();
const { mount } = await import(new URL(config.loader, url));

const player = await mount(document.getElementById("app"), url.href, {
	config,
	params: { user: "Ada" },
	host: {
		call(name) {
			if (name === "time") return new Date().toLocaleTimeString();
			throw new Error(`no host function ${name}`);
		},
		post(name, data) { console.log(name, data); },
		openURL(url) { return url.startsWith("https://example.com/orders/"); },
	},
});
player.hostEvent("message", { text: "来自页面的问候" });
</script>
```

没有 `host` 选项时，加载器导入 `nsgo.yaml` 中 `web.host` 指定的模块，并等待它的 `createHost(player)`。元素或框架中的应用把浏览器历史标题交给页面管理。

```yaml
web:
  host: host.js
```

## 宿主通道

`core.Host` 是每个宿主都实现的同一份约定：启动参数、宿主发送的事件，以及应用调用的宿主函数。值按 JSON 传递，因此 map 变成 `map[string]any`，数字变成 `float64`，同一份代码在 Swift、Java、Go 和 JavaScript 中都能工作。

- `HostParams` 在启动时读取一次启动参数。
- `cx.OnHostEvent` 听到 `Host.Send` 和页面的 `hostEvent`。
- `cx.CallHost` 调用命名处理器并等待结果；`cx.PostHost` 调用但不等待结果。
- `Host.HandleOpenURL` 在平台之前看到 `cx.OpenURL`，它的回答决定平台是否也打开。
- `Host.SetParams`、`Send`、`Handle` 和 `HandleDefault` 可以从任意 goroutine 调用。

iOS 宿主设置 `params`，用 `handle` 安装处理器，用 `send` 发送事件，并用 `onOpenURL` 回答打开链接的请求。Android 宿主用 `setParams`、`handle`、`send` 和 `setOpenUrlHandler` 做同样的事。桌面宿使用 `core.Host` 的方法，Web 由页面的 `host` 对象回答。

## 原生组件

`core.Native` 显示宿主创建的视图：它的 `Props` 到达宿主组件的 `SetProp`，组件发出的事件到达 `OnEvent`，值为动画节点的 prop 会在帧时钟上移动，不消耗引擎。

```go
core.Native{
	Component: "host.badge",
	PropsFunc: func() map[string]any { return map[string]any{"count": count.Get()} },
	OnEvent:   func(name string, data any) { if name == "tap" { count.Set(0) } },
	Style:     core.Style{MinWidth: core.Pt(120), MinHeight: core.Pt(28)},
}
```

在宿主中注册组件：iOS 用 `app.register(component:)` 和 `NSGoComponent`，Android 用 `registerComponent` 和 `NsgoComponent`，桌面用 `nsgo.Component`。Web 没有原生组件，因此未知名称在那里显示空盒子。

## 设置

`tokens` 按角色名覆盖应用主题（`accent`、`inset`、`raised`），可以只覆盖一种外观；`cx.Palette()` 会看到它们。宿主还设置语言、文本大小、外观和数据目录，并听到应用的失败。`nsgo build -embed` 不生成窗口、菜单或托盘图标：这些属于宿主。

`examples/embed-ios`、`examples/embed-android`、`examples/embed-desktop` 和 `examples/embed-web` 是同一个应用配四种宿主。

## 下一步

- [平台服务](services.md)：应用向平台请求什么。
- [构建与发布](building.md)：`-embed` 生成什么包，以及如何签名。
- [第一个应用](getting-started.md)：宿主嵌入的应用。

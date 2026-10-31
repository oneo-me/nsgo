<p align="center">
  <img src="examples/gallery/assets/Icon.png" alt="NSGo" width="128">
</p>

<h1 align="center">NSGo</h1>

<p align="center">
  <strong>简体中文</strong> |
  <a href="README.md">English</a>
</p>

<p align="center">
  <a href="https://github.com/oneo-me"><img alt="GitHub" src="https://img.shields.io/badge/GitHub-oneo--me-181717?logo=github&logoColor=white"></a>
  <a href="https://oneo.me"><img alt="博客" src="https://img.shields.io/badge/%E5%8D%9A%E5%AE%A2-oneo.me-0969da"></a>
  <a href="https://x.com/oneo666"><img alt="X" src="https://img.shields.io/badge/X-%40oneo666-000000?logo=x&logoColor=white"></a>
  <a href="https://github.com/sponsors/oneo-me"><img alt="GitHub 赞助" src="https://img.shields.io/badge/GitHub%20%E8%B5%9E%E5%8A%A9-ea4aaa?logo=githubsponsors&logoColor=white"></a>
</p>

NSGo 是一个以 Go 为后端的跨平台原生 UI 引擎。应用逻辑、响应式图和布局在 Go 中运行，各平台使用自己的原生 UI 绘制。

NSGo 参考 [Native Shell（NSH）](https://nsh.dev) 的设计，NSH 用 Rust 实现了同样的模型。NSGo 面向想要 NSH 的模型、但不想写 Rust 的团队。

<p align="center">
  <img src="examples/gallery/assets/screenshot.jpg" alt="macOS 上 gallery 的 Stocks 示例">
</p>

## 状态

NSGo 处于早期阶段。它已在 macOS、iOS、Windows、Linux、Web 和 Android 上具备 NSH 的响应式模型、布局、文本、图片、画布、着色器、动画、系统服务和无样式基础组件，应用也可嵌入独立的 iOS、Android 和 Go 桌面应用。尚无发布版本或稳定的 API。各平台进度见 [ARCHITECTURE.md](ARCHITECTURE.md#platforms)。

## 目标

- 用 Go 编写应用和组件，使用 signal、memo 和 effect。
- 使用 flexbox 和 grid 布局，并用各平台自己的视图和文本渲染。
- 与 NSH 一样支持 iOS、Android、macOS、Windows、Linux 和 Web。
- NSGo 应用可嵌入现有原生应用，也可打包为独立应用。

## 用法

一个应用是调用 `nsgo.Run` 的 Go main 包。组件是返回元素的函数或类型；signal 变化时，读取它的地方随之更新：

```go
package main

import (
	"fmt"

	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

func Root(cx *core.Scope) core.Node {
	count := core.NewSignal(cx, 0)
	return core.View{Style: core.Style{Padding: core.All(24), Gap: 12}, Children: []core.Node{
		core.Text{ContentFunc: func() string { return fmt.Sprintf("Pressed %d times", count.Get()) }},
		base.Button{
			Label:    "Press me",
			OnPress:  func(core.PressEvent) { count.Update(func(n int) int { return n + 1 }) },
			Style:    core.Style{Padding: core.Symmetric(6, 14), CornerRadius: 8, Background: core.RGB(0xbd481b), Color: core.RGB(0xffffff)},
			Children: []core.Node{core.Text{Content: "Press me"}},
		},
	}}
}

func main() { nsgo.Run(Root) }
```

用 `nsgotest` 无窗口地测试它；`go tool nsgo test -target macos` 会在真实窗口中用原生输入运行同样的测试，并可截图（`app.Screenshot("name")`）：

```go
func TestMain(m *testing.M) { nsgotest.Main(m) } // 使用 -target 时把主线程交给平台

func TestPress(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.Press("text:Press me")
	app.Find("text:Pressed 1 times")
}
```

应用也可以用 TSX 编写，并用 Tailwind CSS 类名设置样式：包内嵌入源码，由 `tsx/compiler` 在应用启动时编译，不需要 Node。`examples/tsx` 就是这样一个应用。

```go
//go:embed *.tsx
var src embed.FS

func main() { nsgo.Run(compiler.App(src, "App.tsx")) }
```

```tsx
import { mount, signal, Text, View } from "nsgo";

function App() {
  const count = signal(0);
  return (
    <View class="gap-3 p-6" onPress={() => count.update((n) => n + 1)}>
      <Text class="text-lg font-bold">Pressed {count()} times</Text>
    </View>
  );
}

mount(App);
```

在包旁边放一个 `nsgo.yaml`，写明应用名称和目标平台：

```yaml
name: Hello
id: dev.oneo.nsgo.hello
version: 0.1.0
build_number: 1         # CFBundleVersion 和 Android 的 versionCode；否则取自 version
# appearance: dark      # 无论系统设置，固定为 light 或 dark
window: { width: 420, height: 560 }
icon:
  apple: assets/AppIcon.icon  # macOS 和 iOS，或 macOS 的 .icns
  png: assets/Icon.png        # 其他所有目标
dev:
  target: native        # native、web、ios、ios_simulator 或 android
  hot_reload: false     # true 时保存后重新编译并重启
  fps: false            # true 时显示性能监视器（macOS、iOS、Windows、Linux）；-fps 可覆盖
  # icon: { png: assets/Icon-dev.png }  # 在 dev 构建中替换 icon 的文件；默认取 assets/*-dev.*
build:
  output: dist          # 构建产物位于 dist/<name>-<version>-<target>
  open: false           # 设为 true 时在构建后打开输出目录
  fps: false            # 同 dev.fps，作用于构建产物
  targets: [macos_arm64, macos_x64, windows_arm64, windows_x64, linux_arm64, linux_x64, ios, ios_simulator, android_arm64, android_x64, web]
# sign:                 # 商店发布；密码来自 NSGO_KEYSTORE_PASSWORD 和 NSGO_KEY_PASSWORD
#   profile: assets/AppStore.mobileprovision  # 为 IPA 签名
#   keystore: assets/release.p12              # 为 APK 签名，配合 key_alias
```

`nsgo.yaml` 旁边的 `resources/` 存放随应用发布的文件，`asset:` 图片从这里加载，`nsgo.Fonts("fonts/Brand.ttf")` 也从这里注册字体；`assets/` 存放不随应用发布的项目文件，例如图标。

在你的模块中添加一次该工具，之后用 `go tool nsgo` 运行，无需安装：

```sh
go get -tool github.com/oneo-me/nsgo/cmd/nsgo
```

`go tool nsgo dev` 运行一次应用，开启 `dev.hot_reload` 后则在每次修改时重新编译并重启。编译失败会显示在正在运行的应用上方；在应用中按 Command 或 Control+R，或在终端输入 `r` 并回车，会重新编译。`-trace` 在每个目标上打印运行中应用的每次提交及其耗时和挂载项。

`go tool nsgo build` 为每个目标输出一个去除符号的发布包：`.app`、`.ipa`、`.apk`，Windows 和 Linux 的目录，或一个网站。`-run` 启动为本机构建的包，或把包安装到 iPhone、模拟器或 Android 设备上运行，`-device` 选择具体设备。

`-embed` 则把应用打包给你自己的 iOS 或 Android 应用：一个 XCFramework（加 `-dynamic` 则由动态框架组成）或 AAR，由其中的 `NSGoView` 或 `NsgoView` 显示。Go 桌面程序用 `nsgo.NewView` 显示应用，网页则用网站中的 `nsgo.js`。宿主由网页提供，或是 `nsgo.yaml` 的 `web.host` 指定的模块。参见 `examples/embed-*`。

`go tool nsgo test` 运行应用的测试，每个测试输出一行；`-run` 筛选测试，`-junit report.xml` 写出 JUnit 报告，`-trace` 打印每一步的挂载项和提交耗时。`-target` 让测试在 `macos`、`windows`、`linux`、`web`（Chrome、Chromium 或 Edge 中）、`ios_simulator` 或 `android` 上运行，每个测试一个进程，截图保存在 `.nsgo/test/screenshots`。`build.targets` 和 `-target` 都接受 `all`，表示仅需 Go 和 Xcode 的所有平台；安装 NDK 后可再加上 `android_x64`。

参数会覆盖它们对应的 `nsgo.yaml` 值，例如 `-target`、`-version` 或 `-output`；dev 和 build 都接受 `-device` 选择 iPhone、模拟器或 Android 设备，build 的 `-team` 为 iPhone 的包签名，dev 的 `-port` 指定其服务器端口。`-h` 会列出全部参数。

在 macOS 上，滚动视图的边缘效果（`Scroll.EdgeEffects`）通过 Core Animation 的私有类 `CABackdropLayer` 和 `CAFilter` 实现模糊，和 AppKit 自己的边缘效果一样，因为没有公开 API 能让模糊程度在视图内变化。Mac App Store 审核可能拒绝使用私有 API 的应用：NSGo 只在滚动视图有边缘栏时调用它们，但每个 macOS 构建都包含它们的名称。

## 开发

要求：装有 Xcode 的 macOS，以及 Go 1.26 或更高版本。构建 arm64 的所有目标（包括 Android 的 APK）都不需要其他工具。

Android 真机和模拟器开发还需要 Android SDK platform-tools 中的 `adb`；iOS 真机开发需要在 Xcode 的 Accounts 中登录 Apple ID，并连接一台已信任、已开启开发者模式的 iPhone。

x86_64 模拟器目标 `android_x64` 还需要 Android NDK：Go 只能通过 cgo 链接 android/amd64。在 Windows 上，只需 Go 1.26 或更高版本即可运行原生应用，并构建 Windows 和 Web 包。在 Linux 上，安装 GTK 4.14 或更高版本、Pango 和 Cairo 后即可用 Go 运行原生应用；网页视图还需要 WebKitGTK 6.0，视频还需要 GStreamer 插件。

在仓库根目录运行 gallery：用 Go 翻译的 NSH gallery，为每个元素、`base` 组件和服务各提供一个演示。宽窗口在侧边栏列出演示，窄窗口则是手机的索引页。

```sh
go tool nsgo dev examples/gallery                   # nsgo.yaml 的 dev 目标，应用退出后随之退出
go tool nsgo dev -target web examples/gallery       # 在浏览器中运行，直到中断
go tool nsgo dev -target ios_simulator examples/gallery
go tool nsgo dev -target ios examples/gallery       # 在已连接的 iPhone 上运行，由 Xcode 签名
go tool nsgo dev -target android examples/gallery   # 在已连接的设备或模拟器上运行，通过 adb
go tool nsgo build examples/gallery                 # 在 examples/gallery/dist 中生成发布包
go tool nsgo build -target all examples/gallery     # 生成所有平台的发布包
go test ./...
bench/run.sh                                       # 测量行情表基准（见 bench/README.md）
```

NSGo 以 NSH 作为参考实现，并只对齐 NSH 的各个发行版。将它克隆到仓库根目录的 `nsh/`，并检出它的最新发行版标签，不要用 `main`；Git 会忽略该目录：

```sh
git clone https://github.com/longbridge/nsh.git nsh
git -C nsh checkout "$(git -C nsh describe --tags --abbrev=0)"
```

修改代码前请先阅读 [ARCHITECTURE.md](ARCHITECTURE.md) 和 [AGENTS.md](AGENTS.md)。

位于 <https://oneo-me.github.io/nsgo/> 的官方网站是一个 React 首页，其中的在线 Gallery 运行 gallery 的 WebAssembly 构建。在 `website/` 中运行 `bun install` 和 `bun run dev`，或用 `bun run build` 构建；[Website 工作流](.github/workflows/website.yml) 将 `website/dist` 发布到 GitHub Pages。网站需要 [Bun](https://bun.com) 1.4 或更新版本。

## 许可证

本项目以 [Apache License 2.0](LICENSE) 授权。NSGo 是 NSH 的移植，[NOTICE](NOTICE) 记录了移植所需的署名。

除非你明确声明，否则你提交给 NSGo 的任何贡献，按 Apache-2.0 许可证的定义，均以 Apache-2.0 授权，不附加任何额外条款或条件。

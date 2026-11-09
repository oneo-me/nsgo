---
title: 构建与发布
description: 为每个平台打包应用、签名，并把它嵌入其他应用。
group: 指南
order: 40
---

## 构建

```sh
go tool nsgo build
```

`build` 为 `nsgo.yaml` 列出的每个目标写出精简的发布包，放在 `dist/<name>-<version>-<target>`：`.app`、`.ipa`、`.apk`、Windows 和 Linux 的目录，或一个网站，网站页面会在 `import()` 执行时才获取它指定的 TSX 模块。`-target` 替换目标列表，`all` 表示所有只需 Go 和 Xcode 的目标；安装 Android NDK 后可再加上 `android_x64`。`-run` 启动为本机构建的包，或把它安装到 `-device` 选中的 iPhone、模拟器或 Android 设备上。

## nsgo.yaml

```yaml
name: Hello
id: dev.example.hello
version: 0.1.0
build_number: 1         # CFBundleVersion 和 Android 的 versionCode；省略时由 version 推出
# appearance: dark      # 无论系统设置如何，都使用 light 或 dark
# theme: theme.json     # TSX 应用的主题：NSGo 的主题或 shadcn/ui 主题
# i18n: { dir: locales, source: en }  # 应用的文案目录及其编写语言
icon:
  apple: assets/AppIcon.icon  # macOS 和 iOS，或仅用于 macOS 的 .icns
  png: assets/Icon.png        # 其他所有目标
dev:
  target: native        # native、web、ios、ios_simulator 或 android
  hot_reload: true      # false 时只构建一次，不监听文件
  fps: false            # true 时显示性能监视器
  # icon: { png: assets/Icon-dev.png }  # 在 dev 构建中替换 icon 的文件；默认取 assets/*-dev.*
build:
  output: dist
  open: false           # true 时构建后打开输出目录
  fps: false            # 同 dev.fps，作用于发布包
  targets: [macos_arm64, macos_x64, windows_arm64, windows_x64, linux_arm64, linux_x64, ios, ios_simulator, android_arm64, web]
# android: { permissions: [REQUEST_INSTALL_PACKAGES] }  # 运行时所需之外的清单权限
```

命令行参数会替换同名的值，例如 `-target`、`-version` 或 `-output`；`-h` 列出全部参数。

## 签名

```yaml
sign:
  identity: "Apple Development: you@example.com (ABCDE12345)"  # 为 macOS 应用签名，省略时为临时签名
  profile: assets/AppStore.mobileprovision  # 为 IPA 签名
  keystore: assets/release.p12              # 与 key_alias 一起为 APK 签名
  # widgets_profile: assets/Widgets.mobileprovision  # 为 IPA 中的小组件扩展签名
```

密钥库的密码来自 `NSGO_KEYSTORE_PASSWORD` 和 `NSGO_KEY_PASSWORD`。`-team` 用于为 iPhone 包签名。

## 小组件

小组件在 iPhone 和 iPad 的主屏幕与锁屏、Mac 的桌面与通知中心上展示应用的摘要。在 `nsgo.yaml` 中列出小组件的种类，再用 `widget` 包为每种提供要显示的内容：

```yaml
widgets:
  - kind: counter
    name: Counter                 # 显示在系统的小组件库中
    description: Shows the count.
    families: [small, medium, circular]  # 另有 large、extra_large、rectangular 和 inline
# apple: { app_group: group.dev.example.hello }  # iOS 的 App Group，默认为 group.<id>
```

```go
err := widget.Update("counter", widget.Entry{
	View: widget.Column{Style: widget.Style{Gap: 4}, Children: []widget.View{
		widget.Text{Content: "Count", Style: widget.Style{TextStyle: widget.Caption, Color: widget.Secondary}},
		widget.Text{Content: strconv.Itoa(n), Style: widget.Style{FontSize: 44, FontWeight: core.FontWeightBold}},
	}},
})
```

`build` 和 `dev` 会用 Xcode 的 Swift 编译器把小组件扩展编进 macOS 和 iOS 包。扩展用 SwiftUI 绘制这些视图，不运行应用的任何代码，因此无论应用是否在运行，小组件都显示应用最后写入的时间线：内容变化时调用 `Update` 即可。`widget.Date` 配合 `Relative` 或 `Timer` 会自己持续计时。`Entry.URL` 和 `Style.Link` 在点击小组件时打开链接。在 iOS 上，应用与小组件共享一个 App Group：真机上需要为 `<id>` 和 `<id>.widgets` 都开启它，并用 `sign.widgets_profile` 为扩展签名。仓库中的 `examples/widgets` 是一个带小组件的计数器，gallery 的 Widgets 演示则覆盖了所有尺寸。

## 嵌入其他应用

`-embed` 把应用打包给你自己的 iOS 或 Android 应用使用：一个 XCFramework（加 `-dynamic` 时由动态框架组成）或 AAR，通过其中的 `NSGoView` 或 `NsgoView` 显示应用。Go 桌面程序用 `nsgo.NewView` 显示应用，网页则用站点的 `nsgo.js`，其主机为页面所在的主机，或 `nsgo.yaml` 中 `web.host` 指定的主机。仓库中的 `examples/embed-*` 分别给出了示例。

## Mac App Store

在 macOS 上，滚动视图的边缘效果（`Scroll.EdgeEffects`）像 AppKit 自己的边缘效果一样，通过 Core Animation 的私有 `CABackdropLayer` 和 `CAFilter` 实现模糊，因为没有公开 API 能让模糊程度在视图上渐变。App Store 审核可能拒绝使用私有 API 的应用：NSGo 只在带边缘栏的滚动视图中调用它们，但每个 macOS 构建都包含它们的名称。

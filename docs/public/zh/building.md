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

`build` 为 `nsgo.yaml` 列出的每个目标写出精简的发布包，放在 `dist/<name>-<version>-<target>`：`.app`、`.ipa`、`.apk`、Windows 和 Linux 的目录，或一个网站。`-target` 替换目标列表，`all` 表示所有只需 Go 和 Xcode 的目标。`-run` 启动为本机构建的包，或把它安装到 `-device` 选中的 iPhone、模拟器或 Android 设备上。

## nsgo.yaml

```yaml
name: Hello
id: dev.example.hello
version: 0.1.0
icon:
  apple: assets/AppIcon.icon  # macOS 和 iOS
  png: assets/Icon.png        # 其他所有目标
build:
  output: dist
  targets: [macos_arm64, windows_x64, linux_x64, ios, android_arm64, web]
```

命令行参数会替换同名的值，例如 `-target`、`-version` 或 `-output`；`-h` 列出全部参数。

## 签名

`nsgo.yaml` 中的 `sign` 指定 macOS 应用的签名身份、iPhone 包的描述文件，以及 APK 的密钥库，密钥库密码来自 `NSGO_KEYSTORE_PASSWORD` 和 `NSGO_KEY_PASSWORD`。`-team` 用于为 iPhone 包签名。

## 嵌入其他应用

`-embed` 把应用打包给你自己的 iOS 或 Android 应用使用：一个 XCFramework 或 AAR，通过其中的 `NSGoView` 或 `NsgoView` 显示应用。Go 桌面程序用 `nsgo.NewView` 显示应用，网页则用站点的 `nsgo.js`。

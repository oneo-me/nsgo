---
title: 构建与发布
description: 为每个平台打包应用、生成安装程序、签名与公证，发布到 GitHub，并把它嵌入其他应用。
group: 工具链
order: 22
---

## 构建

```sh
go tool nsgo build
```

`build` 为 `nsgo.yaml` 列出的每个目标写出精简的发布包，放在 `dist/<name>-<version>-<target>`：`.app`、`.ipa`、`.apk`、Windows 和 Linux 的目录，或一个网站，网站页面会在 `import()` 执行时才获取它指定的 TSX 模块。`-target` 替换目标列表，`all` 表示所有只需 Go 和 Xcode 的目标；安装 Android NDK 后可再加上 `android_x64`；`miniprogram` 构建全部[小程序](#小程序)，也可单独指定 `miniprogram_wechat`、`miniprogram_qq`、`miniprogram_alipay` 或 `miniprogram_douyin`。`-run` 启动为本机构建的包，或把它安装到 `-device` 选中的 iPhone、模拟器或 Android 设备上。

所有要交付出去的东西都在 `dist/releases`，只需看这一个目录：

| `dist/releases` 中 | 内容 |
| --- | --- |
| `*.dmg`、`*-setup.exe`、`*.deb`、`*.pkg.tar.gz`、`*.AppImage` | macOS、Windows 和 Linux 的[安装程序](#安装程序) |
| `*-ios.ipa`、`*-tvos.ipa`、`*.apk` | iOS、tvOS 和 Android 的包，用于商店或设备 |
| `*.pkg`、`*.msix`、`*.aab`、`*.tar.gz` 和 `flatpak/` | `store` 构建的[商店包](#商店包) |
| `install.sh`、`install.ps1` | 从 [GitHub Releases](#发布到-github) 安装应用的脚本 |
| `appcast-<目标>.xml` | [自动更新](#自动更新)的应用读取的更新源 |
| `*-web.zip` | 网站，解压后原样部署即可 |
| `miniprogram/<平台>/` | 各小程序的项目，用对应开发者工具上传 |

每个文件都命名为 `<name>-<version>-<系统>-<架构>`，例如 `hello-1.2.0-windows-x64-setup.exe`，方便用户在 Release 页面找到自己系统的文件；每个文件旁都有 `.sha256`。构建其他版本时会先清空该目录；同一版本的构建（包括在其他机器上的）会往里追加。模拟器和 watchOS 的包留在各自的 `dist/<name>-<version>-<target>` 目录中。

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
  apple: assets/AppIcon.icon  # macOS、iOS 和 watchOS，或仅用于 macOS 的 .icns
  png: assets/Icon.png        # 其他所有目标
  tvos: assets/AppIconTV.brandassets  # Apple TV 的分层图标和顶部横幅，或一张宽图
  watchos: assets/AppIconWatch.icon   # Apple Watch 自己的图标，按圆形绘制，或一张方图
dev:
  target: native        # native、web、ios、ios_simulator、tvos、tvos_simulator、watchos、watchos_simulator、android 或 miniprogram_<平台>
  hot_reload: true      # false 时只构建一次，不监听文件
  # icon: { png: assets/Icon-dev.png }  # 在 dev 构建中替换 icon 的文件；默认取 assets/*-dev.*
build:
  output: dist
  open: false           # true 时构建后打开输出目录
  installers: true      # false 时不生成磁盘映像、Windows 安装程序和 Linux 软件包
  jobs: 0               # 同时构建的目标数；0 为 CPU 数的一半，1 为逐个构建
  targets: [macos_arm64, macos_x64, windows_arm64, windows_x64, linux_arm64, linux_x64, ios, ios_simulator, tvos, tvos_simulator, watchos, watchos_simulator, android_arm64, web]
# android: { permissions: [REQUEST_INSTALL_PACKAGES] }  # 运行时所需之外的清单权限
# linux: { maintainer: "Jane Doe <jane@example.com>" }  # Linux 软件包的维护者，省略时为应用名
# release: { github: me/hello }  # 安装脚本和自动更新从其 Releases 安装的 GitHub 仓库，省略时取 Git 远程 origin 的；tag_prefix 默认为 v
# updates: { key: updates.pub }  # 应用按平台和渠道更新的方式，见“自动更新”
# miniprogram: { appid: { wechat: wx0123456789abcdef, alipay: "2021000000000000" }, custom_navigation: false }  # 小程序的设置
```

命令行参数会替换同名的值，例如 `-target`、`-version` 或 `-output`；`-h` 列出全部参数。

Apple TV 的图标不是方形，而是一张分层卡片：`tvos` 取 Xcode 的 `.brandassets`（其中的 `App Icon` 堆栈和顶部横幅），或一张如 1280×768 的宽图，工具用它的边缘色铺底、让图铺满卡片，并居中生成顶部横幅；两者都没有时，`png` 按卡片高度的四分之三居中。Apple Watch 会把图标裁成圆形，所以 `watchos` 取按 watchOS 圆形绘制的 `.icon` 或一张方图；没有它时依次用 `apple` 和 `png`。两者与 `apple`、`png` 一样默认取 `assets/AppIconTV.*` 和 `assets/AppIconWatch.*`。

每个构建在启动时还接受 NSGo 自己的参数，例如 `--nsgo-fps`（[运行与调试](debugging.md#所有构建都接受的参数)）。

## 安装程序

对桌面目标，`build` 会把供用户安装的文件写到 `dist/releases`，每个文件旁附带 `.sha256` 校验和：

| 目标 | 安装程序 |
| --- | --- |
| macOS | `<name>-<version>-macos-<arch>.dmg`，打开后显示应用、一个箭头和“应用程序”文件夹的链接，提示把应用拖过去，用 `sign.identity` 签名并[公证](#公证) |
| Windows | `<name>-<version>-windows-<arch>-setup.exe`，[NSIS](https://nsis.sourceforge.io) 安装程序，为当前用户安装，再次安装时装回用户之前选的目录，应用运行中会先关闭它，带开始菜单快捷方式、可选的桌面快捷方式，以及只删除自己装过的文件的卸载程序 |
| Linux | 供 Debian 和 Ubuntu 使用的 `<name>-<version>-linux-<arch>.deb`，以及供 Arch Linux 使用的 `<name>-<version>-linux-<arch>.pkg.tar.gz`，把应用装到 `/opt/<name>`，并带命令、桌面入口和图标；还有 `<name>-<version>-linux-<arch>.AppImage`，无需安装即可在任何发行版上运行 |

Windows 安装程序需要 NSIS 的编译器，在 Mac 上用 `brew install makensis` 安装，因此 Mac 可以构建全部安装程序；没有它时，`build` 把脚本 `<name>.nsi` 留在包里，在任何系统上都可原样编译。安装程序提供应用文案中 NSIS 有翻译的语言，包括简体和繁体中文，并按系统语言选择。`-installers=false` 或 `build.installers: false` 只构建包本身。

安装程序可以使用应用自己的图片和图标，未指定时使用应用图标或内置图片：

```yaml
macos:
  dmg_background: assets/DMGBackground.png  # 磁盘映像窗口的背景，窗口按它的点尺寸设定大小
  dmg_icon: assets/DMGIcon.png              # 磁盘映像的图标：.icns 或正方形 PNG
windows:
  installer_icon: assets/Setup.png          # 安装程序的图标：.ico 或正方形 PNG
  uninstaller_icon: assets/Uninstall.png    # 卸载程序的图标
```

背景 PNG 旁放一张两倍尺寸的同名图（例如 `DMGBackground@2x.png`），在 Retina 屏幕上就能保持清晰。应用图标位于距左边四分之一处，“应用程序”位于距右边四分之一处，图标连同下方的名称整体上下居中，内置图片的箭头就画在它们之间；自定义背景时请为它们留出位置。Linux 软件包没有专门的图片，安装的是应用图标。AppImage 需要 FUSE（大多数发行版自带），与软件包一样使用系统的 GTK 4；它的运行时首次构建时下载到用户缓存中。

## 商店包

`-channel store` 或 `updates.channel: store` 为各商店构建桌面和 Android 目标，由商店负责更新，不再生成安装程序和安装脚本：

| 目标 | 商店包 |
| --- | --- |
| macOS | 用于 Mac App Store 的 `<name>-<version>-macos-<arch>.pkg`：应用运行在 App Sandbox 中，用描述文件对应的身份签名，再用团队的安装器身份打包 |
| Windows | 用于 Microsoft Store 的 `<name>-<version>-windows-<arch>.msix`，发布时由商店签名 |
| Linux | 带应用 AppStream 元数据的 `<name>-<version>-linux-<arch>.tar.gz`，以及 `flatpak/<id>.yml`：Flathub 据此构建 Flatpak 的清单，在 GNOME 运行时上从 Release 下载这些归档 |
| Android | 用于 Google Play 的 App Bundle `<name>-<version>-android-<arch>.aab`，用上传密钥 `sign.keystore` 签名 |

```yaml
macos:
  category: public.app-category.productivity  # LSApplicationCategoryType，Mac App Store 要求填写
  # entitlements: { com.apple.security.files.downloads.read-write: true }  # 网络、用户选择的文件和 apple.info 说明的权限之外的授权
windows:
  store:                               # 按 Partner Center 的“产品标识”填写
    identity: 12345Example.Hello
    publisher: CN=00000000-0000-0000-0000-000000000000
    publisher_name: Example
linux:
  summary: Says hello                  # 软件中心显示的一句话简介
  license: MIT                         # SPDX 表达式；默认为 LicenseRef-proprietary
  # flatpak_runtime: "51"              # GNOME 运行时的版本
android:
  target_sdk: 35                       # Google Play 要求的 targetSdkVersion
sign:
  macos_profile: assets/MacAppStore.provisionprofile  # Mac App Store 的描述文件
  # installer: "3rd Party Mac Developer Installer: Example (ABCDE12345)"  # 默认取钥匙串中描述文件所属团队的安装器身份
```

没有 `sign.macos_profile` 时，Mac 包不签名：它能在你的 Mac 上以沙盒方式运行，可用来测试沙盒，但无法上传。沙盒中的应用只能访问自己的容器、网络和用户选择的文件；小组件暂时无法在沙盒中共享时间线，所以 store 构建会拒绝带小组件的应用。用 Transporter 上传 `.pkg`，在 Partner Center 上传 `.msix`，在 Play Console 上传 `.aab`；Release 中有了清单引用的归档后，把 `flatpak/<id>.yml` 提交到 Flathub，也可以用 `flatpak-builder` 在本地构建。Windows 只安装已签名的 MSIX，商店之外安装需要签名，上架时由商店签名。

在 macOS 上，滚动视图的边缘效果（`Scroll.EdgeEffects`）像 AppKit 自己的边缘效果一样，通过 Core Animation 的私有 `CABackdropLayer` 和 `CAFilter` 实现模糊，因为没有公开 API 能让模糊程度在视图上渐变。App Store 审核可能拒绝使用私有 API 的应用：NSGo 只在带边缘栏的滚动视图中调用它们，但每个 macOS 构建都包含它们的名称。

## 发布到 GitHub

构建包含 macOS、Windows 或 Linux 目标时，`dist/releases` 还会包含 `install.sh` 和 `install.ps1`，它们从 `release.github` 指定的 GitHub 仓库的 Releases 安装应用；未指定时取应用 Git 远程 `origin` 所在的仓库，两者都没有时 `build` 不写脚本并给出提示。把其中顶层的文件（不含 `miniprogram/`）上传到标签为 `v<version>` 的 Release：

```sh
gh release create v1.2.0 dist/releases/*.*
```

之后用户用一行命令即可安装或更新应用，构建时会打印出来：

```sh
curl -fsSL https://github.com/me/hello/releases/latest/download/install.sh | sh    # macOS 和 Linux
```

```powershell
irm https://github.com/me/hello/releases/latest/download/install.ps1 | iex         # Windows
```

在 macOS 上，脚本把应用从磁盘映像复制到 `/Applications`，无写权限时复制到 `~/Applications`；在 Linux 上通过 `sudo` 用 `pacman` 安装 Arch 软件包，否则用 `apt-get` 或 `dpkg` 安装 Debian 软件包；在 Windows 上静默运行安装程序，装到应用已安装的目录。安装前都会先核对校验和。`<NAME>_VERSION`（应用名的大写形式，例如 `HELLO_VERSION=1.1.0`）安装其他版本，`<NAME>_DOWNLOAD_URL` 从镜像下载。

## 自动更新

`updates` 让应用按所在平台和用户获取它的渠道保持最新：

```yaml
updates:
  key: updates.pub                    # nsgo keygen updates 生成的公钥
  # channel: store                    # direct、store 或 none；有 key 时默认 direct，没有时为 none
  # feed: https://example.com/hello/  # appcast 和安装程序所在的地址；默认为 GitHub 上最新的 Release
  # automatic: false                  # 只在要求时检查；默认为 true
```

`nsgo keygen updates` 生成 `updates.pub`（提交到仓库）和 `updates.key`（保密，不要放进仓库）；在 CI 中用 `NSGO_UPDATES_KEY`（密钥文件的内容）代替它。每次 direct 构建都用它签名安装程序，并在旁边写出 `appcast-<目标>.xml`：一个 [Sparkle](https://sparkle-project.org) 格式的更新源，应用从 `feed` 下读取它，把 appcast 和安装程序一起上传即可。没有私钥时，`build` 不写 macOS 和 Windows 的 appcast，因为它们只安装签过名的安装程序。

| 目标 | direct 构建的更新方式 |
| --- | --- |
| macOS | `build` 打进应用的 Sparkle 每天检查，提示新版本，下载磁盘映像，校验签名，安装并重启应用 |
| Windows | 放在可执行文件旁的 WinSparkle 同样如此，并静默运行安装程序，由它启动新版本 |
| Linux、Android | 应用读取自己的 appcast，打开该 Release 的页面，用户从那里安装软件包 |
| iOS | 应用查询负责更新它的 App Store，并打开其中的页面 |

### 渠道

渠道决定由谁更新应用。`-channel` 在单次构建中替换 `updates.channel`，所以同一个项目可以为每个渠道各构建一次：

| 渠道 | 用于 | 构建出的包 |
| --- | --- | --- |
| `direct` | GitHub、网站，以及直接安装 Release 文件的 Homebrew cask 和 winget | 按上表自行更新 |
| `store` | App Store、Mac App Store、Microsoft Store、Google Play、Flathub 和其他应用商店 | 即[商店包](#商店包)，不带更新组件；iOS 查询 App Store，不需要 key |
| `none` | Linux 发行版，以及重新打包应用的人 | 不检查更新 |

应用商店会拒绝自行更新的应用，所以上传 `store` 构建，不要上传 direct 构建。direct 应用的 Homebrew cask 应写 `auto_updates true`。凡是系统能说明应用由谁安装的地方，应用也会检查：从 Google Play 或其他商店安装的 Android 应用、Flatpak 或 Snap，以及 TestFlight 构建都不检查，由各自的商店更新。

### 在应用中检查

`updater.Check(cx)` 如同“检查更新…”菜单项，检查并显示结果；`updater.CheckInBackground(cx)` 只在有新版本时提示，`updater.CheckDaily(cx)` 也是如此，但每天至多检查一次。`automatic` 开启时，Linux、Android 和 iOS 上的应用在启动时和用户回到应用时这样检查；检查失败（例如离线）时，一小时后再试。开发构建从不更新。

## 文件类型

`file_types` 让 macOS 应用能从访达、程序坞和“打开方式”打开它的文档：

```yaml
file_types:
  - name: Flow File
    extensions: [flow]
    # id: dev.example.hello.flow   # 默认为 <id>.<第一个扩展名>；用别人的 ID 则导入该类型
    # icon: assets/Flow.icns       # 或正方形 PNG
    # role: viewer                 # 默认为 editor
    # conforms: [public.json]      # 默认为 public.data
```

应用用 `desktop.OnOpenFiles` 接收这些文件，在 Windows 和 Linux 上它也接收命令行中给出的文件。启动应用时带来的文件会等到第一个处理函数注册：

```go
desktop.OnOpenFiles(cx, func(paths []string) { open(paths[0]) })
```

## 小程序

```sh
go tool nsgo build -target miniprogram_wechat -run
go tool nsgo dev -target miniprogram_alipay
```

每个目标写出对应平台开发者工具的项目，也放在 `dist/releases/miniprogram/<平台>`，`-run` 和 `dev` 会用该工具打开；`dev` 在你保存时重新构建，工具随之刷新：

| 目标 | 平台 | 工具 |
| --- | --- | --- |
| `miniprogram_wechat` | 微信 | [微信开发者工具](https://developers.weixin.qq.com/miniprogram/dev/devtools/download.html)，需开启服务端口（设置 → 安全设置） |
| `miniprogram_qq` | QQ | [QQ 小程序开发者工具](https://q.qq.com/wiki/tools/devtool/)，限 macOS |
| `miniprogram_alipay` | 支付宝 | [支付宝小程序开发者工具](https://opendocs.alipay.com/mini/ide/download)，限 macOS |
| `miniprogram_douyin` | 抖音及字节系其他应用 | [抖音开发者工具](https://developer.open-douyin.com/docs/resource/zh-CN/mini-app/develop/developer-instrument/download/developer-instrument-update-and-download)，限 macOS |

`miniprogram.appid` 按平台（`wechat`、`qq`、`alipay`、`douyin`）设置小程序的 AppID；没有时微信开发者工具以测试号预览，其他工具会要求填写。平台的导航栏显示应用的标题；开启 `custom_navigation` 后应用占满页面，安全区让它避开右上角的胶囊按钮。QQ、支付宝和抖音尚未在各自的工具中试过；支付宝暂不支持由应用处理返回、关键帧动画和键盘高度。

小程序只运行 Go 应用，不运行 TSX，且各平台把应用压缩后的模块限制在 2 MB 以内：几个页面的应用放得下，放不下时 `build` 会说明。着色器在小程序中不绘制，网页视图、视频、原生组件和应用自带的字体文件暂不显示。

## 签名

```yaml
sign:
  identity: "Apple Development: you@example.com (ABCDE12345)"  # 为 macOS 应用签名，省略时为临时签名
  profile: assets/AppStore.mobileprovision  # 为 IPA 签名
  keystore: assets/release.p12              # 与 key_alias 一起为 APK 签名
  # widgets_profile: assets/Widgets.mobileprovision  # 为 IPA 中的小组件扩展签名
  # clip_profile: assets/Clip.mobileprovision        # 为 IPA 中的轻 App 签名
```

密钥库的密码来自 `NSGO_KEYSTORE_PASSWORD` 和 `NSGO_KEY_PASSWORD`。`-team` 用于为 iPhone 包签名。

### 公证

经 Apple 公证的应用在 Mac 上下载后打开时不会出现 Gatekeeper 警告。用 Developer ID 签名，并指定 notarytool 存在钥匙串中的凭据：

```sh
xcrun notarytool store-credentials hello --apple-id you@example.com --team-id ABCDE12345
```

```yaml
sign:
  identity: "Developer ID Application: Example Inc (ABCDE12345)"
  notarize: hello   # 上面的钥匙串配置；也可用 -notarize hello
```

之后 `build` 会以强化运行时签名，提交磁盘映像，等待 Apple 的结果（通常几分钟），并把公证票据装订到磁盘映像和应用上；被拒时会打印 Apple 的日志。在 CI 中可用 `NSGO_NOTARY_KEY`、`NSGO_NOTARY_KEY_ID` 和 `NSGO_NOTARY_ISSUER`（App Store Connect API 密钥的 `.p8` 文件、ID 和 issuer）代替钥匙串配置。强化运行时默认禁止使用麦克风和摄像头，`apple.info` 含 `NSMicrophoneUsageDescription` 或 `NSCameraUsageDescription` 时，`build` 会为应用声明相应权限。

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

## 轻 App（App Clip）

轻 App 是 iOS 应用中的一小部分，用户从链接、App Clip 码或 NFC 标签打开它，无需安装应用。`app_clip` 把它编进 iOS 包，它有自己的 main 包，只链接自己显示的内容：

```yaml
app_clip:
  main: ./clip              # 轻 App 的 main 包，默认为应用自己的
  domains: [example.com]    # 哪些域名的链接打开它
  # name: Hello Clip        # 默认为 "<name> Clip"
  # id: dev.example.hello.Clip  # 默认为 <id>.Clip
  # minimum: "17.0"         # 默认 16.0；17.0 在只从链接打开时允许 100 MB
  # notifications: true     # 请求一天内的通知
  # location: true          # 让轻 App 确认打开它的位置
```

轻 App 用 `mobile.OpenedURL` 读取打开它的链接，再从另一个链接打开时它随之变化；`mobile.IsAppClip` 说明代码是否作为轻 App 运行。系统下载的轻 App 最大 15 MB，iOS 17 起只从链接打开时最大 100 MB，超出时 `build` 会拒绝；链接 `ui` 的轻 App 约 11 MB。`dev -target ios_simulator -clip https://example.com/order?table=12` 在模拟器中单独运行轻 App，如同从该链接打开；`-target ios` 则在 iPhone 上运行，并打印该链接的二维码：在“设置 → 开发者 → Local Experiences”中登记这个链接后，用相机扫码即可打开轻 App。Apple 只为 Apple Developer Program 的团队签名轻 App，个人团队不行。上架 App Store 时需要为 `<id>` 开启 App Clips，用 `sign.clip_profile` 为轻 App 签名，并在这些域名上提供 apple-app-site-association 文件。仓库中的 `examples/appclip` 是一个咖啡馆，它的轻 App 在桌边点单。

## 嵌入其他应用

`-embed` 把应用打包给你自己的 iOS 或 Android 应用使用：一个 XCFramework（加 `-dynamic` 时由动态框架组成）或 AAR，通过其中的 `NSGoView` 或 `NsgoView` 显示应用。Go 桌面程序用 `nsgo.NewView` 显示应用，网页则用站点的 `nsgo.js`，其主机为页面所在的主机，或 `nsgo.yaml` 中 `web.host` 指定的主机。仓库中的 `examples/embed-*` 分别给出了示例；宿主侧见[嵌入](embedding.md)。

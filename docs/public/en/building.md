---
title: Building and shipping
description: Package an app for every platform, make installers, sign and notarize it, publish it on GitHub and embed it in other apps.
group: Toolchain
order: 22
---

## Build

```sh
go tool nsgo build
```

`build` writes a stripped release package for each target `nsgo.yaml` lists, into `dist/<name>-<version>-<target>`: an `.app`, an `.ipa`, an `.apk`, Windows' and Linux's directories, or a web site, whose page fetches each TSX module an `import()` names when the import runs. `-target` replaces the list, and `all` means every target that needs only Go and Xcode; add `android_x64` when an Android NDK is installed, and `miniprogram`, or `miniprogram_wechat`, `miniprogram_qq`, `miniprogram_alipay` or `miniprogram_douyin` alone, for [mini programs](#mini-programs). `-run` starts the package built for this machine, or installs it on the iPhone, simulator or Android device `-device` picks.

Everything to hand out lands in `dist/releases`, the only directory to look in:

| In `dist/releases` | What |
| --- | --- |
| `*.dmg`, `*-setup.exe`, `*.deb`, `*.pkg.tar.gz` | [Installers](#installers) for macOS, Windows and Linux |
| `*-ios.ipa`, `*-tvos.ipa`, `*.apk` | The iOS, tvOS and Android packages, for the stores or devices |
| `install.sh`, `install.ps1` | Scripts installing the app from [GitHub releases](#publish-on-github) |
| `web/` | The site, to serve as it is |
| `miniprogram/<platform>/` | Each mini program's project, to upload from its developer tool |

Each file there has its `.sha256` beside it. A build of another version empties the directory first; builds of the same version, on other machines too, add to it. Simulators' and watchOS's packages stay in their `dist/<name>-<version>-<target>` directories.

## nsgo.yaml

```yaml
name: Hello
id: dev.example.hello
version: 0.1.0
build_number: 1         # CFBundleVersion and Android's versionCode; else from version
# appearance: dark      # light or dark whatever the system's
# theme: theme.json     # a TSX app's theme: NSGo's or a shadcn/ui one
# i18n: { dir: locales, source: en }  # the app's catalogs and the language they are written in
icon:
  apple: assets/AppIcon.icon  # macOS, iOS and watchOS, or an .icns for macOS
  png: assets/Icon.png        # every other target
  tvos: assets/AppIconTV.brandassets  # the Apple TV's layered icon and top shelf art, or a wide PNG
  watchos: assets/AppIconWatch.icon   # the Apple Watch's own, drawn in circles, or a square PNG
dev:
  target: native        # native, web, ios, ios_simulator, tvos, tvos_simulator, watchos, watchos_simulator, android or miniprogram_<platform>
  hot_reload: true      # false builds once, watching no files
  # icon: { png: assets/Icon-dev.png }  # replaces icon's files in dev's builds; defaults to assets/*-dev.*
build:
  output: dist
  open: false           # true opens the output directory after building
  installers: true      # false leaves out the disk image, the Windows installer and the Linux packages
  jobs: 0               # how many targets build at once; 0 is half the CPUs, 1 one after another
  targets: [macos_arm64, macos_x64, windows_arm64, windows_x64, linux_arm64, linux_x64, ios, ios_simulator, tvos, tvos_simulator, watchos, watchos_simulator, android_arm64, web]
# android: { permissions: [REQUEST_INSTALL_PACKAGES] }  # manifest permissions beyond the runtime's
# linux: { maintainer: "Jane Doe <jane@example.com>" }  # the Linux packages' maintainer, else the app's name
# release: { github: me/hello }  # the GitHub repository the install scripts install from, else the Git remote origin's; tag_prefix: v by default
# miniprogram: { appid: { wechat: wx0123456789abcdef, alipay: "2021000000000000" }, custom_navigation: false }  # the mini programs'
```

Flags replace the values they name, such as `-target`, `-version` or `-output`; `-h` lists them all.

An Apple TV's icon is a layered card, not a square: `tvos` takes Xcode's `.brandassets`, its `App Icon` stacks and top shelf images, or a wide image such as 1280×768, which the tool draws to fit the card over its edge color and centers on the top shelf images. Without either, the PNG sits at three quarters of the card's height. An Apple Watch masks its icon to a circle, so `watchos` takes an `.icon` drawn for watchOS's circles or a square PNG; without it the app uses `apple`, then `png`. Both fall back to `assets/AppIconTV.*` and `assets/AppIconWatch.*`, as `apple` and `png` do.

Every build also takes NSGo's own flags as it starts, such as `--nsgo-fps` ([Running and debugging](debugging.md#flags-of-every-build)).

## Installers

For the desktop targets, `build` writes what users install the app from into `dist/releases`, each with a `.sha256` checksum:

| Target | Installer |
| --- | --- |
| macOS | `<name>-<version>-<arch>.dmg`, a disk image showing the app, an arrow and a link to Applications to drag it to, signed with `sign.identity` and [notarized](#notarize) |
| Windows | `<name>-<version>-<arch>-setup.exe`, an [NSIS](https://nsis.sourceforge.io) installer that installs for the user, again in the folder the user chose before, closing the app if it runs, with a Start menu shortcut, an optional desktop one and an uninstaller that removes only what it installed |
| Linux | `<name>_<version>_<arch>.deb` for Debian and Ubuntu, and `<name>-<version>-1-<arch>.pkg.tar.gz` for Arch Linux, which install the app in `/opt/<name>` with a command, its desktop entry and icon |

The Windows installer needs NSIS's compiler, on a Mac `brew install makensis`, so a Mac builds every installer; without it `build` leaves the script, `<name>.nsi`, in the package, which compiles anywhere as it is. The installer speaks the languages of the app's catalogs that NSIS translates, Simplified and Traditional Chinese included, picking the system's. `-installers=false` or `build.installers: false` builds the packages alone.

The installers take pictures and icons of the app's own; each is the app's icon or the built-in picture otherwise:

```yaml
macos:
  dmg_background: assets/DMGBackground.png  # the disk image's window, which takes its size in points
  dmg_icon: assets/DMGIcon.png              # the disk image's icon: an .icns or a square PNG
windows:
  installer_icon: assets/Setup.png          # the setup program's icon: an .ico or a square PNG
  uninstaller_icon: assets/Uninstall.png    # the uninstaller's
```

A background PNG with a twin twice its size, such as `DMGBackground@2x.png`, stays sharp on Retina screens. The app's icon sits a quarter in from the left, Applications a quarter in from the right, each with its name centered from top to bottom, where the built-in picture draws its arrow; leave room for them in a background of your own. The Linux packages have no pictures of their own; they install the app's icon.

## Publish on GitHub

When the build has a macOS, Windows or Linux target, `dist/releases` also holds `install.sh` and `install.ps1`, which install the app from the releases of the GitHub repository `release.github` names, else the one the app's Git remote `origin` is on; without either `build` writes none and says so. Upload its files, the top level without `web/` and `miniprogram/`, to the release tagged `v<version>`:

```sh
gh release create v1.2.0 dist/releases/*.*
```

Users then install or update the app with one line, which the build prints:

```sh
curl -fsSL https://github.com/me/hello/releases/latest/download/install.sh | sh    # macOS and Linux
```

```powershell
irm https://github.com/me/hello/releases/latest/download/install.ps1 | iex         # Windows
```

On macOS the script copies the app from the disk image into `/Applications`, or `~/Applications` when it cannot write there; on Linux it installs the Arch package with `pacman`, else the Debian one with `apt-get` or `dpkg`, through `sudo`; on Windows it runs the installer silently, which installs where the app already is. Each checks the file against its checksum first. `<NAME>_VERSION`, the app's name in capitals, such as `HELLO_VERSION=1.1.0`, installs another release, and `<NAME>_DOWNLOAD_URL` downloads from a mirror.

## File types

`file_types` makes a macOS app open its documents from the Finder, the Dock and Open With:

```yaml
file_types:
  - name: Flow File
    extensions: [flow]
    # id: dev.example.hello.flow   # <id>.<first extension> by default; another's ID imports its type
    # icon: assets/Flow.icns       # or a square PNG
    # role: viewer                 # editor by default
    # conforms: [public.json]      # public.data by default
```

The app hears the files with `desktop.OnOpenFiles`, which also takes those named on the command line on Windows and Linux. Files that launched the app wait for the first handler:

```go
desktop.OnOpenFiles(cx, func(paths []string) { open(paths[0]) })
```

## Mini programs

```sh
go tool nsgo build -target miniprogram_wechat -run
go tool nsgo dev -target miniprogram_alipay
```

Each target writes a project for its platform's developer tool, also in `dist/releases/miniprogram/<platform>`, which `-run` and `dev` open; `dev` builds again as you save, and the tool reloads:

| Target | Platform | Tool |
| --- | --- | --- |
| `miniprogram_wechat` | WeChat | [WeChat DevTools](https://developers.weixin.qq.com/miniprogram/dev/devtools/download.html), with its service port on (Settings, Security) |
| `miniprogram_qq` | QQ | [QQ's developer tool](https://q.qq.com/wiki/tools/devtool/), on macOS |
| `miniprogram_alipay` | Alipay | [Alipay's Mini Program Studio](https://opendocs.alipay.com/mini/ide/download), on macOS |
| `miniprogram_douyin` | Douyin and ByteDance's other apps | [Douyin's developer tool](https://developer.open-douyin.com/docs/resource/zh-CN/mini-app/develop/developer-instrument/download/developer-instrument-update-and-download), on macOS |

`miniprogram.appid` gives the mini program's AppID on each platform (`wechat`, `qq`, `alipay`, `douyin`); without one, WeChat DevTools previews it with its tourist AppID and the other tools ask for one. The platform's navigation bar shows the app's title; with `custom_navigation`, the app fills the page and its safe area keeps it clear of the menu button. QQ, Alipay and Douyin have not been tried in their tools yet, and Alipay does not yet take back through the app, keyframes or the keyboard's height.

A mini program runs Go apps alone, not TSX, and the platforms hold the app's compressed module to 2 MB: an app of a few screens fits, and `build` says when one does not. Shaders draw nothing there, and web views, videos, native components and the app's font files are not shown yet.

## Sign

```yaml
sign:
  identity: "Apple Development: you@example.com (ABCDE12345)"  # signs the macOS app, else ad hoc
  profile: assets/AppStore.mobileprovision  # signs the IPA
  keystore: assets/release.p12              # signs APKs, with key_alias
  # widgets_profile: assets/Widgets.mobileprovision  # signs the IPA's widget extension
  # clip_profile: assets/Clip.mobileprovision        # signs the IPA's App Clip
```

The keystore's passwords come from `NSGO_KEYSTORE_PASSWORD` and `NSGO_KEY_PASSWORD`. `-team` signs the iPhone's package.

### Notarize

Apps downloaded on a Mac open without Gatekeeper's warning once Apple has notarized them. Sign with a Developer ID and name the credentials notarytool keeps in your keychain:

```sh
xcrun notarytool store-credentials hello --apple-id you@example.com --team-id ABCDE12345
```

```yaml
sign:
  identity: "Developer ID Application: Example Inc (ABCDE12345)"
  notarize: hello   # the keychain profile above; or -notarize hello
```

`build` then signs with the hardened runtime, submits the disk image, waits for Apple's verdict, a few minutes, and staples the ticket to the image and the app; a rejection prints Apple's log. In CI, `NSGO_NOTARY_KEY`, `NSGO_NOTARY_KEY_ID` and `NSGO_NOTARY_ISSUER`, an App Store Connect API key's `.p8` file, ID and issuer, take the profile's place. The hardened runtime denies the microphone and the camera unless the app claims them, which `build` does when `apple.info` has `NSMicrophoneUsageDescription` or `NSCameraUsageDescription`.

## Widgets

Widgets show a glance of the app on the Home Screen and the Lock Screen of iPhones and iPads, and on the Mac's desktop and in Notification Center. List their kinds in `nsgo.yaml`, and give each what to show with package `widget`:

```yaml
widgets:
  - kind: counter
    name: Counter                 # shown in the system's gallery of widgets
    description: Shows the count.
    families: [small, medium, circular]  # also large, extra_large, rectangular and inline
# apple: { app_group: group.dev.example.hello }  # the iOS app group, group.<id> by default
```

```go
err := widget.Update("counter", widget.Entry{
	View: widget.Column{Style: widget.Style{Gap: 4}, Children: []widget.View{
		widget.Text{Content: "Count", Style: widget.Style{TextStyle: widget.Caption, Color: widget.Secondary}},
		widget.Text{Content: strconv.Itoa(n), Style: widget.Style{FontSize: 44, FontWeight: core.FontWeightBold}},
	}},
})
```

`build` and `dev` compile a widget extension into the macOS and iOS packages with Xcode's Swift compiler. It draws the views with SwiftUI and runs none of the app's code, so a widget shows the timeline the app wrote last, whether the app runs or not: call `Update` whenever what it shows changes. `widget.Date` with `Relative` or `Timer` keeps counting by itself. `Entry.URL` and `Style.Link` open URLs when a widget is tapped. On iOS the app and its widgets share an app group: for devices, enable it for both `<id>` and `<id>.widgets`, and sign the extension with `sign.widgets_profile`. The repository's `examples/widgets` is a counter with widgets, and the gallery's Widgets demo fills every family.

## App Clips

An App Clip is a small part of an iOS app that runs from a link, a code or an NFC tag without installing the app. `app_clip` builds one into the iOS package, from a main package of its own so it links only what it shows:

```yaml
app_clip:
  main: ./clip              # the clip's main package, the app's own by default
  domains: [example.com]    # whose links open it
  # name: Hello Clip        # "<name> Clip" by default
  # id: dev.example.hello.Clip  # <id>.Clip by default
  # minimum: "17.0"         # 16.0 by default; 17.0 allows 100 MB when only links open it
  # notifications: true     # ask for notifications for a day
  # location: true          # let the clip confirm where it was opened
```

The clip reads the link that opened it with `mobile.OpenedURL`, which changes as another link opens it again, and `mobile.IsAppClip` says whether the code runs as one. The system downloads a clip of up to 15 MB, or 100 MB from iOS 17 when only links open it, and `build` refuses a larger one; a clip linking `ui` takes about 11 MB. `dev -target ios_simulator -clip https://example.com/order?table=12` runs the clip alone in the simulator, opened from that link, and `-target ios` on an iPhone, printing the link's QR code: list the link in Settings > Developer > Local Experiences and the camera opens the clip from it. Apple signs App Clips only for teams of the Apple Developer Program, not personal teams. For the App Store, enable App Clips for `<id>`, sign the clip with `sign.clip_profile`, and serve the apple-app-site-association file of the domains. The repository's `examples/appclip` is a café whose clip orders at a table.

## Embed in another app

`-embed` packages the app for an iOS or Android app of your own: an XCFramework, of dynamic frameworks with `-dynamic`, or an AAR whose `NSGoView` or `NsgoView` shows it. Go desktop programs show apps with `nsgo.NewView`, and web pages with the site's `nsgo.js`, whose host is the page's or the one `web.host` in `nsgo.yaml` names. The repository's `examples/embed-*` show each; [Embedding](embedding.md) covers the host side.

## The Mac App Store

On macOS, a scroll view's edge effects (`Scroll.EdgeEffects`) blur through Core Animation's private `CABackdropLayer` and `CAFilter`, as AppKit's own edge effects do, since no public API varies a blur across a view. App Store review may reject apps that use private APIs: NSGo calls them only for scroll views with edge bars, but every macOS build contains their names.

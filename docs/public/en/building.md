---
title: Building and shipping
description: Package an app for every platform, sign it and embed it in other apps.
group: Guides
order: 40
---

## Build

```sh
go tool nsgo build
```

`build` writes a stripped release package for each target `nsgo.yaml` lists, into `dist/<name>-<version>-<target>`: an `.app`, an `.ipa`, an `.apk`, Windows' and Linux's directories, or a web site, whose page fetches each TSX module an `import()` names when the import runs. `-target` replaces the list, and `all` means every target that needs only Go and Xcode; add `android_x64` when an Android NDK is installed. `-run` starts the package built for this machine, or installs it on the iPhone, simulator or Android device `-device` picks.

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
  apple: assets/AppIcon.icon  # macOS and iOS, or an .icns for macOS
  png: assets/Icon.png        # every other target
dev:
  target: native        # native, web, ios, ios_simulator or android
  hot_reload: true      # false builds once, watching no files
  fps: false            # true shows the performance monitor
  # icon: { png: assets/Icon-dev.png }  # replaces icon's files in dev's builds; defaults to assets/*-dev.*
build:
  output: dist
  open: false           # true opens the output directory after building
  fps: false            # as dev.fps, for packages
  targets: [macos_arm64, macos_x64, windows_arm64, windows_x64, linux_arm64, linux_x64, ios, ios_simulator, android_arm64, web]
# android: { permissions: [REQUEST_INSTALL_PACKAGES] }  # manifest permissions beyond the runtime's
```

Flags replace the values they name, such as `-target`, `-version` or `-output`; `-h` lists them all.

## Sign

```yaml
sign:
  identity: "Apple Development: you@example.com (ABCDE12345)"  # signs the macOS app, else ad hoc
  profile: assets/AppStore.mobileprovision  # signs the IPA
  keystore: assets/release.p12              # signs APKs, with key_alias
```

The keystore's passwords come from `NSGO_KEYSTORE_PASSWORD` and `NSGO_KEY_PASSWORD`. `-team` signs the iPhone's package.

## Embed in another app

`-embed` packages the app for an iOS or Android app of your own: an XCFramework, of dynamic frameworks with `-dynamic`, or an AAR whose `NSGoView` or `NsgoView` shows it. Go desktop programs show apps with `nsgo.NewView`, and web pages with the site's `nsgo.js`, whose host is the page's or the one `web.host` in `nsgo.yaml` names. The repository's `examples/embed-*` show each.

## The Mac App Store

On macOS, a scroll view's edge effects (`Scroll.EdgeEffects`) blur through Core Animation's private `CABackdropLayer` and `CAFilter`, as AppKit's own edge effects do, since no public API varies a blur across a view. App Store review may reject apps that use private APIs: NSGo calls them only for scroll views with edge bars, but every macOS build contains their names.

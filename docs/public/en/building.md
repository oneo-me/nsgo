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

`build` writes a stripped release package for each target `nsgo.yaml` lists, into `dist/<name>-<version>-<target>`: an `.app`, an `.ipa`, an `.apk`, Windows' and Linux's directories, or a web site. `-target` replaces the list, and `all` means every target that needs only Go and Xcode. `-run` starts the package built for this machine, or installs it on the iPhone, simulator or Android device `-device` picks.

## nsgo.yaml

```yaml
name: Hello
id: dev.example.hello
version: 0.1.0
icon:
  apple: assets/AppIcon.icon  # macOS and iOS
  png: assets/Icon.png        # every other target
build:
  output: dist
  targets: [macos_arm64, windows_x64, linux_x64, ios, android_arm64, web]
```

Flags replace the values they name, such as `-target`, `-version` or `-output`; `-h` lists them all.

## Sign

`sign` in `nsgo.yaml` names an identity for the macOS app, a provisioning profile for the iPhone's package and a keystore for APKs, whose passwords come from `NSGO_KEYSTORE_PASSWORD` and `NSGO_KEY_PASSWORD`. `-team` signs the iPhone's package.

## Embed in another app

`-embed` packages the app for an iOS or Android app of your own: an XCFramework or an AAR whose `NSGoView` or `NsgoView` shows it. Go desktop programs show apps with `nsgo.NewView`, and web pages with the site's `nsgo.js`.

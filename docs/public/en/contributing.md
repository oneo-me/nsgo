---
title: Development setup
description: Build NSGo from source, run the gallery and test a change.
group: Contributing
order: 80
---

## Requirements

macOS with Xcode and Go 1.26 or later builds every target for arm64, Android's APK included. Running on devices needs what [Setup](getting-started.md#requirements) lists, and the x86_64 Android emulator's target, `android_x64`, an Android NDK, since Go links android/amd64 only with cgo. The Windows installer needs NSIS (`brew install makensis`); without it `build` leaves its script.

```sh
git clone https://github.com/oneo-me/nsgo.git
cd nsgo
```

Read [ARCHITECTURE.md](https://github.com/oneo-me/nsgo/blob/main/ARCHITECTURE.md) and [AGENTS.md](https://github.com/oneo-me/nsgo/blob/main/AGENTS.md) before changing code.

## The gallery

The gallery has a demo of each element, `base` and `ui` component and service. A wide window shows the demos under tabs in a sidebar; a narrow one shows the tabs and the settings in a bottom bar, with demos pushed over them. Run it from the repository's root:

```sh
go tool nsgo dev examples/gallery                   # nsgo.yaml's dev target, exiting with the app
go tool nsgo dev examples/gallery-tsx               # the gallery's components and apps, written in TSX
go tool nsgo dev -target web examples/gallery       # in the browser, serving until interrupted
go tool nsgo dev -target ios_simulator examples/gallery
go tool nsgo dev -target ios examples/gallery       # on the connected iPhone, signed by Xcode
go tool nsgo dev -target android examples/gallery   # on the connected device or emulator, over adb
go tool nsgo build -target all examples/gallery     # every platform's packages in examples/gallery/dist, what to hand out in dist/releases
```

## Check and test

```sh
go tool nsgo check                 # vet every package
go tool nsgo check -target all     # vet them for every target too
go tool nsgo test                  # every package's tests
go tool nsgo test -bench . ./core  # core's benchmarks
(cd benchmark && go run ./runner)  # measure against other frameworks
```

## NSH

NSGo reads [Native Shell](https://github.com/longbridge/nsh) as a design reference; none of its tests need it. To read it, clone it beside the repository and check out its latest release tag, never `main`:

```sh
git clone https://github.com/longbridge/nsh.git ../nsh
git -C ../nsh checkout "$(git -C ../nsh describe --tags --abbrev=0)"
```

## Next

- [The codebase](contributing/codebase.md): where each part of NSGo lives.
- [Making a change](contributing/changes.md): what a pull request needs.

---
title: The codebase
description: Where each part of NSGo lives, and the documents that explain why.
group: Contributing
order: 81
---

NSGo is one Go module. Apps import its packages from the top; each platform's backend sits at the bottom and receives what changed as native views to create, update and remove.

## Packages

| Package | What it holds |
| --- | --- |
| `nsgo` (the root) | `nsgo.Run` and `nsgo.NewView`, picking a backend by build tags |
| `core` | The app's API: elements, styles, classes, themes, signals and services; layout and text preparation |
| `base` | Unstyled components, a file each, and complex ones as packages (`base/dock`, `base/editor`) |
| `ui` | `base`'s components with a look, and packages such as `ui/chat` |
| `mobile`, `desktop` | What only phones or only desktops have |
| `tsx` | TSX apps, their bundler and compiler, and the modules they import |
| `i18n`, `*/locales` | Message catalogs and the code generated from them |
| `nsgotest` | Tests that act as a user, headless or on a platform |
| `internal/engine`, `internal/driver`, `internal/layout` | The engine goroutine, the mount protocol and the layout |
| `internal/platform/<target>` | A backend: native views, text and input for one target |
| `cmd/nsgo` | The `nsgo` tool |
| `examples/` | The gallery and the other example apps |

Packages import only downward; `layering_test.go` checks the graph on every target.

## Design documents

[ARCHITECTURE.md](https://github.com/nsgo-dev/nsgo/blob/main/ARCHITECTURE.md) maps the packages, the threads and the flows from a commit to the screen, and lists the subjects in `docs/architecture/`: one per part, such as layout, text, scrolling, themes or the tool. Read the subject of what you change before changing it; it records how the part works and where it departs from [Native Shell](https://github.com/longbridge/nsh), whose design NSGo ports.

[AGENTS.md](https://github.com/nsgo-dev/nsgo/blob/main/AGENTS.md) holds the rules every change follows, for people and coding agents alike. A few shape most changes:

- Each API has one home: `core` for elements and the environment, `base` for controls, `ui` for their looks, `mobile` and `desktop` for one kind of device.
- `base` has no look; a `ui` component is its `base` form with one.
- Nothing changes implicitly by device: no larger controls on desktops or smaller ones on phones in `core` or `base`.
- Native views everywhere, the same on every platform: what one platform has natively, others imitate.
- No hand-written C, and nothing beyond macOS, Xcode and Go to build every target but the x86_64 Android emulator's.

What is missing, platform by platform, is in the [roadmap](https://github.com/nsgo-dev/nsgo/blob/main/docs/roadmap.md).

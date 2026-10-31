# Changelog

## [Unreleased]

- Add a reactive Go engine with owners, memos, effects and regions that lays out and sends only what each update changed, to native views in one batch, with a performance monitor over the app on macOS, iOS, Windows and Linux.
- Add native backends for macOS, iOS, Windows, Linux, Android and the web, every one built from a Mac with Go and Xcode.
- Add block, flexbox and grid layout with static, sticky and fixed positions, intrinsic sizes, stacking order, right-to-left subtrees and paddings and margins that follow the safe area and the software keyboard.
- Add CSS-like styles: corners, borders, outlines, shadows, gradients, filters, backdrop blur, masks, blend modes, 3D transforms, state styles, the SVG paint canvases draw with, and Tailwind CSS classes with their variants, on elements and every part of `base`'s components.
- Add TSX apps importing `nsgo`, compiled by `tsx/compiler` with no Node and run on goja with NSH's reactive API, elements, Tailwind classes and error boundaries.
- Add transitions, keyframes, animated values, motion presets and layout animations that run on each platform's frame clock.
- Add selectable, hit-testable rich text with the app's fonts, OpenType figures and small capitals, decoration lines of any style, thickness and offset, inputs, GIF, WebP and SVG images, Canvas 2D drawing, GPU shaders written in Go and Lucide icons.
- Add scroll views with overlay scroll bars, touch scrolling, throttled scroll events, pull to refresh and the app's own refresh indicators, virtualized lists, overlays, modals, navigation stacks with system headers and iOS's system bar buttons, iOS's sliding pushes and pops, predictive back and browser history, the system tab bar and leaving the app in `mobile`, web views, videos and native views.
- Add pointer, touch, focus, keyboard and gesture input, file drops, accessibility, storage and environment signals.
- Add clipboard images and files, opening links, the photo picker, forced light or dark appearance and a disk cache for downloaded images.
- Add custom title bars with placed or hidden window controls, window frames, levels and materials, and application and context menus in `desktop`, with scroll bars and a return from `nsgo.Run` on quit.
- Add `base`: unstyled controls, popups, menus that become sheets on phones, dialogs, sheets, swipeable toasts, resizable panels, a dock, charts, tables, a `DataTable` over a million rows, a streaming `TextView`, a code `Editor` with completion and the app's highlighters, and a `DiffView` with review comments.
- Add embedding in other apps with `nsgo.NewView` on desktops, `nsgo build -embed` frameworks for iOS devices, simulators and Catalyst and for Android, and several web apps on one page, with a channel to the host.
- Add `nsgotest` for app tests that act as a user does, headless or on every platform with its native input, menus and screenshots, the test's own clock and clipboard.
- Add the `nsgo` tool, which creates, runs with hot reload, tests, packages and signs apps for stores from `nsgo.yaml`, reporting each step and each package's build time and size.
- Add themes in light and dark, Onevra's by default, that a subtree can replace or force into one appearance, read from YAML files and overridden by an embedding host.
- Add NSH's gallery translated to Go, with a demo of every element, component and service.
- Add a React home page whose live gallery runs the gallery's WebAssembly build, reporting its download, compilation and start progress.

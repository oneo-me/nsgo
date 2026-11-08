# NSGo

NSGo is a cross-platform native UI engine with a Go backend, ported from the design of NSH. Read [ARCHITECTURE.md](ARCHITECTURE.md) before coding, then the relevant subjects.

## Principles

- Treat NSH's checkout beside the repository (`../nsh`, written `nsh/` in the documents) as read-only reference. Never edit, commit or vendor it. Keep it checked out at the latest release tag, never `main`.
- Before designing a subsystem, read how NSH does it in `nsh/docs/dev/` and its crate's module docs.
- Write idiomatic Go. NSH is a reference, not a constraint: keep its behavior and names by default, depart where a design is clearer or easier to maintain, and record why in the owning subject.
- Keep tests NSGo's own: no test reads NSH's checkout, runs NSH's apps or cases, or holds NSGo to what NSH does. TSX above all grows its own way from here (decided by the user, 2026-11-08).
- Assume only macOS, Xcode and Go on a user's machine. Building every target except the x86_64 emulator's from there must need nothing else: no NDK, Android SDK, JDK, GTK or Windows toolchain.
- Never hand-write C. Reach system APIs through purego or `syscall`, and use cgo only where Go itself demands it: linking for iOS and x86_64 Android, and the one function an embedded iOS framework exports, which the tool generates.
- Prebuild what users cannot build with Go and Xcode, such as the Android runtime's `classes.dex` and `classes.jar`, and commit it with the command that rebuilds it (`internal/platform/android/runtime/build.sh`).
- When implementing, fixing or optimizing anything, check whether other targets' backends need the same change, and make it or record the gap in the [roadmap](docs/roadmap.md).
- Keep each package's imports within the [dependency graph](ARCHITECTURE.md#boundaries); `layering_test.go` checks it.
- Give each API one home, as NSH's modules do: `core` for elements and the environment, `base` for controls and navigation, `ui` for their styled forms, `mobile` and `desktop` for what only phones or only desktops have.
- When `core` implements another package's API, its exported part names that package in its comment.
- Write configuration files people edit by hand in YAML, never JSON, including ones ported from NSH's JSON files.
- Mark unimplemented designs as goals or open questions; never document them as existing behavior.

## Components

`base` holds NSGo's built-in unstyled components, with `nsh-base` the reference for the behavior and names of those NSH has. `ui` holds its built-in styled components, with Onevra the reference for their look.

- A `base` component takes the app's styles part by part and has no look; themes, styled controls and variants belong in `ui`.
- A `ui` component with a `base` form is that form with a look: behavior, state and accessibility stay in `base`, and what it needs that `base` lacks goes into `base`, never a second implementation in `ui`. A component with no behavior worth sharing lives whole in `ui`.
- A `ui` component is product-neutral: what only the gallery's demos use stays in `examples/gallery`.
- A `ui` component takes its look, the color it shows included, from `Variant`, a type of its own (`ButtonVariant`) whose zero value is the default, never from boolean fields or a shared color type.
- Most `ui` components have one size. One that needs more takes `Size`, a `ui.Size` of default, small or large whose zero value is the default, never a `Small` or `Compact` boolean; so far `Button`, `Tabs` and `Switch`. What is one shape at any scale takes `Size` in points instead: `Avatar`, `AvatarGroup`, `ProgressRing`, `Spinner` and `IconButton` ([variants and sizes](docs/architecture/themes.md#variants-and-sizes)).
- A component says its own words through its package's catalogs (`locales/`), never string literals, in English and NSGo's other languages ([languages](docs/architecture/languages.md)).
- In `base` and `ui`, keep one component to a file, named after it (`iconbutton.go`), with the types and parts only it uses; what several share gets a file of its own.
- Lists, trees, tables and sidebars have two looks, default and selected: no hover and no press. Only the rows of a popup that drops down, such as a menu or a select, follow the pointer.
- Leave the pointer's look alone, as native apps do: pressables keep the platform's arrow, and `Style.Cursor` is for what the arrow cannot say, such as text fields and resize or scrub drags.
- Keep a scroll bar against its container's edge: the scroll view fills the container and the padding goes on its content, never around the scroll view.
- Everything that scrolls follows touchpads and touchscreens smoothly and glides on with inertia when let go, on every target and the web included: scroll through `Scroll`, `List` or the platform's own scrolling, never by stepping offsets from pointer or wheel events. Past an edge, follow the platform: a rubber band on macOS and iOS, its stretch on Android, a stop for touchpads on Windows and Linux, and whatever the browser does on the web.
- In the gallery, a playground centers its forms, and a form with no width of its own fills a centered column; what lists its results downward, such as a command palette, starts at the top and fills the stage (`Start`). A number's knob is a value left of a 180-point slider.
- In the gallery, a `base` component keeps its simplest form, in the text's color and grays.
- A complex component takes a directory of its own, whose package may hold many files (`base/dock`, `ui/chat`), and drops the component's name from its API (`dock.Area`). Packages under `base` share code through `base/internal`.

## Working with the user

- Never create sub-Agents or Git worktrees; work directly in the current checkout.
- Confirm before choosing an answer to an open question in [ARCHITECTURE.md](ARCHITECTURE.md#open-questions).
- Commit or push only when the user asks.

## Verification

Run `go tool nsgo check` and `go tool nsgo test` from the repository root, never `go vet`, `go test`, `go build` or `go run` directly; when the tool cannot do something an app or this repository needs, extend the tool. The development setup lives in [Contributing](docs/public/en/contributing.md).

Run `bun run build` in `website/` when changing the website; it type-checks the site and rebuilds the gallery it embeds.

Keep workflows except the website deployment on `workflow_dispatch` until the first release; restore their automatic triggers as part of releasing.

## Documentation

- Update a document only when a fact its reader needs changes.
- Avoid secondary labels and descriptions that restate a heading or value; let the layout explain.
- Keep each document within its budget and written for its reader.
- Record completed user-visible outcomes as one-clause `Unreleased` bullets with references such as `(#12 by @alice)`, ordered by type and merging related ones.
- Until the first release, keep `Unreleased` as the first version's notes: a handful of capability bullets, refined in place, never fixes or changes to unreleased work.
- Repair documents you touch instead of only appending to them.
- Keep `README.md` and `README.zh-CN.md` equivalent; everything else, including code comments, is English.
- Keep documentation on the code branch and reconcile it during merges.

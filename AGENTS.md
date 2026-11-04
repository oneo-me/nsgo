# NSGo

NSGo is a cross-platform native UI engine with a Go backend, ported from the design of NSH. Read [ARCHITECTURE.md](ARCHITECTURE.md) before coding, then the relevant subjects.

## Principles

- Treat NSH's checkout beside the repository (`../nsh`, written `nsh/` in the documents) as read-only reference. Never edit, commit or vendor it. NSGo aligns with NSH's releases only, so keep it checked out at the latest release tag, never `main`.
- Before designing a subsystem, read how NSH does it in `nsh/docs/dev/` and its crate's module docs.
- Write idiomatic Go. NSH is a reference, not a constraint: keep its behavior and names by default, depart where a design is clearer or easier to maintain, and record why in the owning subject.
- Assume only macOS, Xcode and Go on a user's machine. Building every target except the x86_64 emulator's from there must need nothing else: no NDK, Android SDK, JDK, GTK or Windows toolchain.
- Never hand-write C. Reach system APIs through purego or `syscall`, and use cgo only where Go itself demands it: linking for iOS and x86_64 Android, and the one function an embedded iOS framework exports, which the tool generates.
- Prebuild what users cannot build with Go and Xcode, such as the Android runtime's `classes.dex` and `classes.jar`, and commit it with the command that rebuilds it (`internal/platform/android/runtime/build.sh`).
- When implementing, fixing or optimizing anything, check whether other targets' backends need the same change, and make it or record the gap in the [roadmap](docs/roadmap.md).
- Keep each package's imports within the [dependency graph](ARCHITECTURE.md#boundaries); `layering_test.go` checks it.
- Give each API one home, as NSH's modules do: `core` for elements and the environment, `base` for controls and navigation, `mobile` and `desktop` for what only phones or only desktops have.
- When `core` implements another package's API, its exported part names that package in its comment.
- Write configuration files people edit by hand in YAML, never JSON, including ones ported from NSH's JSON files.
- Mark unimplemented designs as goals or open questions; never document them as existing behavior.

## Components

`base` holds NSGo's built-in unstyled components; `nsh-base` is the reference for their behavior and names.

- Each component takes the app's styles part by part and has no look. Themes and styled controls and variants belong to apps or a component library, never here.
- A component says its own words through its package's catalogs (`locales/`), never string literals, in English and NSGo's other languages ([languages](docs/architecture/languages.md)).
- `examples/gallery-ui` is the source of Onexa's components, which copy it: keep its components product-neutral, put what only the gallery's demos use in `gallery.go`, which the copy leaves out, and run `go run ./tools/syncgallery` in `../onexa` after changing them.
- Keep a component in the `base` package unless it links dependencies an app without it should not; then give it a package under `base`, as `base/textview` has.

## Working with the user

- Never create sub-Agents or Git worktrees; work directly in the current checkout.
- Confirm before choosing an answer to an open question in [ARCHITECTURE.md](ARCHITECTURE.md#open-questions).
- Commit or push only when the user asks.

## Verification

Run `go vet ./...` and `go test ./...` from the repository root. The development setup lives in [README.md](README.md#development).

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

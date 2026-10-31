# Roadmap

This plan lists what NSGo still lacks compared with NSH's 0.3.0 release (tag `v0.3.0`, `76fa8a7f`), in the order to build it. Each item names its NSH reference and the result that finishes it. What exists today is in [ARCHITECTURE.md](../ARCHITECTURE.md); design questions that block an item are in its [Open questions](../ARCHITECTURE.md#open-questions).

References are files in `nsh/docs/dev/` unless they give a path from `nsh/`, with the bullet or section to read in brackets. To refresh the plan for NSH's next release, check out its tag in `nsh/`, then read `git log v0.3.0..HEAD` there and the documents it changed, and compare the declarations any `nsh` command run on its gallery writes to `nsh/examples/gallery/.nsh/types/` with `go doc -all` of the public packages.

## Embedding and the tool

| Item | NSH reference | Done when |
| --- | --- | --- |
| Web host channel | `web.md` (the CLI's web target), `website/docs/web.md` (the host's functions) | A page answers an app's host calls and posts and sends it events, through the loader `nsgo.json` feeds and a host module `nsgo.yaml` names, which the web package copies |
| Commit traces | `website/docs/dev-server-and-debugging.md` (Logs), `performance.md` | `nsgo dev -trace` prints each commit's time and mount items from the running app on every target |

## TSX and dynamic delivery

These items port NSH's TSX in order ([TSX and dynamic delivery](architecture/tsx.md)); signed packages wait for their [open question](../ARCHITECTURE.md#open-questions).

| Item | NSH reference | Done when |
| --- | --- | --- |
| TSX runtime | `native-pipeline.md` (The renderer, Lists and node methods), `js-modules.md` (`nsh`), `nsh/crates/nsh-js/src/dom.rs` | `tsx` runs every primitive NSH's compiler emits (`Overlay`, `List`, `WebView`, `Video`, `Canvas`, `Dialog`) with all their props and events, node methods (`focus`, `scrollTo`, `measure`), the animation API and `PanResponder`, `platform`-only props and `nsgo/mobile`'s and `nsgo/desktop`'s exports, so NSH's gallery runs with its imports renamed |
| Go components in TSX | `components.md`, `website/docs/rust.md` (Extending a Rust app with TSX) | `nsgo/base` exports `base`'s components with NSH's props, and an app registers its own; `.nsgo/types` declares them and `nsgo check` passes on `examples/tsx` |
| TSX dev loop | `tooling.md` (Dev server) | A save reloads a TSX app on every target without building Go again, and `nsgo build` compiles the app ahead, so packages ship no compiler |
| Host calls and fetch | `js-modules.md` (`host`), `architecture.md` (Networking) | TSX calls the embedding app's functions and `fetch`es, each checked against capabilities |
| Signed packages | `signed-packages.md` | `nsgo keygen`, `certify`, `publish`, `verify` and `revoke` work; a Go, iOS and Android host verifies a package, runs it in `tsx.View`, and refuses tampered, revoked and older builds |
| Sandbox | `sandbox.md` | A module's calls are cut at their budget, hardening is on outside development, and every request a capability names is refused without it |
| Tailwind's remaining parts | `styles.md`, `nsh/crates/nsh-core/src/tailwind/` | Classes take `nsgo.yaml`'s theme extension and NSH's design tokens, and NSH's class tests (`tailwind/tests.rs`) pass |
| Hot updates | `tooling.md` (Hot module replacement) | An edit renders the changed components again and keeps the rest of the app's state |
| TSX tests | `tooling.md` (`nsh test`) | `nsgo test` runs `*.test.tsx` headless and on a platform |

## Finish the existing platforms

These items wait for community contributions, and a change to one backend records the gaps it leaves here.

| Item | NSH reference | Done when |
| --- | --- | --- |
| Real file drags | `file-drop.md` | A real drag from Finder and from a Linux file manager is checked to reach a view, as one from Explorer is |
| Ink past a text's box | `styles.md` (text) | A text's decoration lines and shadow that reach past its box are checked to draw on Android, Windows and Linux, as they do on Apple and the web |

## Not ported

NSGo leaves out these NSH parts:

- WebSockets, `resource` and the standard modules (`fs/promises`, `process`, `net`, ...), since Go apps use Go's libraries with `Scope.Post` and TSX modules reach the host app instead.
- `EngineContext`, since a host gives each engine's app function its values directly.
- The GPUI backend and the LayerFS trial.
- 32-bit Android (`armeabi-v7a`), since Go links android/arm only with cgo and an NDK.

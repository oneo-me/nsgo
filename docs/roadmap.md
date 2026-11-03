# Roadmap

This plan lists what NSGo still lacks of NSH's 0.3.0 release (tag `v0.3.0`, `76fa8a7f`), in the order to build it. Each item names its NSH reference and the result that finishes it. What exists is in [ARCHITECTURE.md](../ARCHITECTURE.md), and what NSGo leaves out on purpose in [NSGo and NSH](architecture/nsh.md#what-nsgo-leaves-out). Design questions that block an item are in [Open questions](../ARCHITECTURE.md#open-questions).

References are files in `nsh/docs/dev/` unless they give a path from `nsh/`, with the section to read in brackets. [Following NSH's releases](architecture/nsh.md#following-nshs-releases) says how to rebuild this plan for NSH's next release.

## NSH's apps on NSGo

NSH's own apps and tests are the measure of alignment: each runs with its imports renamed from `nsh` to `nsgo`, and `go test ./tsx` runs its gallery, the gallery's tests, its examples with theirs, and its compat cases (`TestNSHGallery`, `TestNSHGalleryTests`, `TestNSHExamples`, `TestNSHCompat`). All pass but what needs the standard modules NSGo leaves out and one test that reads QuickJS's wording of a `TypeError`; `.nsgo/types` exports the type names `.nsh/types` does, and NSH's gallery type-checks against it as against NSH's own.

| Item | NSH reference | Done when |
| --- | --- | --- |
| The compat cases | `compatibility.md` (Scripts), `nsh/tests/compat/` | `TestNSHCompat` runs every step of the scripts, NSH's text forms of `style`, `prop`, `event`, `touch` and `state` steps included (about 600 not run on 2026-11-03), and `compatFailing` lists only the cases that need the standard modules: 42 of 122 cases passed the steps run, the first causes below |

The compat cases first fail on these:

- Steps that stand in for a platform's events (`event`, `touch`, `state`) leave the cases that read them unchecked: `events-*`, most of `base-*`.
- Layout: frames differ, or views the steps name do not show, in `layout-box`, `layout-direction`, `layout-flex`, `layout-grid` and `layout-tailwind`, and a text's width in the `base-*` controls differs by a few points.
- Modules: `TextView.parse`, `URL.createObjectURL`, `nativeModule("nsh/base")`, and what the `web-*` cases check of `Blob`, encodings, events, streams, timers and `URL`.

## TSX and dynamic delivery

What TSX still lacks of NSH's ([TSX and dynamic delivery](architecture/tsx.md)). Signed packages and capabilities wait for their [open question](../ARCHITECTURE.md#open-questions).

| Item | NSH reference | Done when |
| --- | --- | --- |
| TSX on every target | `tooling.md` (Dev server, `nsh test`) | `examples/tsx` runs, updates in place on a save and passes its TSX tests under `nsgo dev` and `nsgo test -target` on all six backends, as it does headless |
| Go classes and async functions | `rust-classes.md`, `nsh/examples/rust-exports` | TSX constructs a registered Go type with `new` and uses its methods, getters, setters and statics. Go functions return promises and call back JS functions they were given, and `examples/tsx` uses one of each, declared in `.nsgo/types` |
| Hot updates that keep state | `tooling.md` (Hot module replacement) | An edited module keeps its top-level `signal()` and `createNavStack()` values while their initializers are unchanged, its contexts keep their identity, and an unchanged `mount` is kept, as `nsh/crates/nsh-engine/tests/engine/hot.rs` checks |
| The tool's TSX commands | `tooling.md` (New apps, `nsh test`) | `nsgo init` makes a TSX app or a Go app with a TSX entry; `nsgo test` given an entry takes steps from standard input; `nsgo test -fps` logs the performance monitor's readings |
| Split bundles | `native-pipeline.md` (Split bundles) | `import()` of an app's module loads a chunk on demand on the web, and a native package stays one bundle |
| Signed packages | `signed-packages.md` | `nsgo keygen`, `certify`, `publish`, `verify` and `revoke` work; a Go, iOS and Android host verifies a package, runs it in `tsx.View`, and refuses tampered, revoked and older builds |
| Capabilities and limits | `sandbox.md` | Every request a capability names (`fetch`, remote images, web views, `openUrl`, host calls) is refused without it. Quotas cut a module's nodes, timers, host calls and requests, a host sets a view's limits, and `nsgo dev -limits` runs an app under them |

## Performance against NSH's baselines

NSH records what its hot paths cost (`data-table.md` (Performance), `performance.md`, `web.md`), and NSH's own build on this machine is the measure: each item runs one scenario on both galleries ([performance](architecture/performance.md#keeping-frames-cheap) says how). The DataTable keeps up with NSH's headless, in headless Chrome and on macOS, and its cells move with the content in each frame on every backend, as NSH's do only on the web.

| Item | NSH reference | Done when |
| --- | --- | --- |
| The other platforms | `data-table.md` (Performance) | The DataTable's scroll, still and ticking, costs on Windows, Linux, iOS and Android what NSH's does there, and its cells move with the content in every frame there as on macOS and the web |
| The web's collector | — | Scrolling the ticking DataTable in headless Chrome shows no task over 50 ms, as NSH's does: Go's collector still paused about 100 ms a few times in a 6 s fast scroll on 2026-11-01 |
| New views' default items | `nsh/AGENTS.md` (Performance rules, 4) | A new view gets no item for a default style, empty props or an empty event mask, every backend starting views in that state: they were 2,646 of the 18,252 items opening the DataTable at 1200×800, and 1,427 of the gallery's 4,525 at start |
| Wheel steps on the web | `nsh/crates/nsh-platform-web/web/nsh.js` | A mouse wheel's step scrolls a page as far as on NSH's: over the scaled DataTable NSGo's moved about twice as far on 2026-11-01 |
| Styles as classes on the web | `web.md` (Styles as classes) | Visual styles reach the page as classes, as NSH's do, and the gallery's batches shrink by about as much (a third of the website's demos' first render) |

## Languages

What [languages](architecture/languages.md) still lacks of MyGo's design (`docs/i18n.md` in MyGo), which NSH does not have.

| Item | Reference | Done when |
| --- | --- | --- |
| Dates and times | MyGo's `i18n.FormatDate` | Messages take `{d, date, short}`, `{d, time}` and skeletons, and `DateTime` placeholders, written by the language's CLDR patterns |
| Native controls in the app's direction | MyGo's `MYGO_DIRECTION` | Native controls, system headers and Android's back gesture mirror as the app's language is written (`ItemDirection`), not as the system's: `semanticContentAttribute` on iOS, `setLayoutDirection` on Android, which needs `classes.dex` rebuilt, and their peers on the desktops and the web |
| Pseudo locales | MyGo's `en-XA`, `ar-XB` | `nsgo dev -lang en-XA` shows accented, longer messages and `ar-XB` mirrored ones, so untranslated and cut text shows |
| TSX messages | MyGo's generated `locales.ts` | A TSX app imports its catalogs' messages as typed functions that follow the app's language |
| The app's name per language | MyGo's `mygo.appName` | Packages list the app under its name in each of its languages: `InfoPlist.strings`, Android's `strings.xml`, the web manifest |

## Motion

What components still lack to move as their parts come, go and travel (#19), which NSH lacks as well (`animation.md`, the table beside gpui-base's).

| Item | Reference | Done when |
| --- | --- | --- |
| Exits | NSH's gap for gpui-base's `Presence`; Motion's `AnimatePresence` | A region like `Show` keeps what it held after its condition turns false until an exit the app plays ends, so a sheet's or a toast's parts leave with their motion without the component tracking it |
| Shared elements | SwiftUI's `matchedGeometryEffect`, Motion's `layoutId` | Views a commit removes and inserts under one match ID count as one: the new view moves from the frame the old one showed, with the commit's layout animation |
| Resisting drags | `RubberBandClamp` | A node resists a value past its bounds on the frame clock, so a drag bound with `PanValues` slows at its ends without a commit per move |

## Finish the existing platforms

These items wait for community contributions, and a change to one backend records the gaps it leaves here.

| Item | NSH reference | Done when |
| --- | --- | --- |
| Folder picks on phones and the web | — | `PickDirectories` answers with folders the app reads on iOS (`UIDocumentPickerViewController` for `.folder`, kept in its security scope), Android (`ACTION_OPEN_DOCUMENT_TREE`, which grants a tree URI, not a path) and the web (`showDirectoryPicker`, or `<input webkitdirectory>`), which need a `Directory` that opens what it holds without a path, as `File.Open` does |
| Real file drags | `file-drop.md` | A real drag from Finder and from a Linux file manager is checked to reach a view, as one from Explorer is |
| Long presses on Android spans | `text.md` (nested texts) | A long press on a text's span reaches the span's `OnLongPress` on Android, as it does on the other backends: the runtime's Java reports the character under it, which needs `classes.dex` rebuilt |
| Ink past a text's box | `styles.md` (text) | A text's decoration lines and shadow that reach past its box are checked to draw on Android, Windows and Linux, as they do on Apple and the web |
| Android blurs | `styles.md` (`backdropFilter`) | From Android 12, a backdrop filter blurs what the view's parent drew under it through a `RenderNode` and a `RenderEffect`, and a scroll view's soft and hard edges ([scrolling](architecture/scrolling.md#edge-effects)) blur the content under its bars |
| Progressive edges before iOS 26 | — | Before iOS 26 a soft scroll edge blurs by a radius that grows toward the side through a `CABackdropLayer`'s variable blur, as macOS's does ([scrolling](architecture/scrolling.md#edge-effects)), rather than fading in an even blur |
| Web glass | — (MyGo's `glass.Glass`) | Liquid Glass draws through MyGo's shader on the web where Chrome's HTML-in-Canvas draws a whole app, its `<canvas>` elements included, without hanging ([styles](architecture/styles.md)) |

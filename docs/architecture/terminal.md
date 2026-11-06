# Terminal

`base/terminal` is a terminal for NSGo apps: a `View` that runs the user's shell, or any program, in a pseudo-terminal, with Ghostty's emulator, libghostty-vt, loaded at run time through purego. Ghostty is the reference for its behavior and names; NSH has no terminal. Read this before changing the package.

## Packages

| Package | What |
| --- | --- |
| `base/terminal` | `Terminal` (the emulator, the program and their goroutines), `View`, themes with Ghostty's, keys |
| `base/terminal/locales` | The lines the terminal writes when its program ends, in the language the view's app shows |
| `base/terminal/internal/vt` | The binding of libghostty-vt's C API: structs checked against the layouts the library reports as it loads, so a library of another version fails to load rather than corrupt memory |
| `base/terminal/internal/pty` | Pseudo-terminals in pure Go: `/dev/ptmx` on macOS and Linux, ConPTY on Windows |
| `base/terminal/internal/library` | Finds the library: `$NSGO_GHOSTTY_VT`; the one the nsgo tool put into the app, an iOS app's `GhosttyVT.framework` or a macOS app's `Frameworks`, or one beside the executable; else on desktops the build it embeds for the platform, written once into `<cache>/nsgo/libghostty-vt/<sha256>/` |

It is a package under `base` because it links what an app without it should not, and the one there that is not portable: it reaches libghostty-vt through purego and programs through `syscall` ([package layers](layers.md)). Desktops run local programs and remote sessions; iOS, which forbids child processes, runs remote sessions only (`Options.Conn`), and `New` without one returns an error there. Android and the web build, and `New` returns an error ([roadmap](../roadmap.md#terminal)). It imports `core` and `i18n`. Its only look is the terminal's palette, as `Editor`'s is its highlighting's: `Theme` and `DarkTheme`, Visual Studio Code's colors when unset; the app styles the view around the grid.

## The library

libghostty-vt is built from Ghostty's sources at the commit the binding names, by `go run ./tools/libghostty` on a Mac with Xcode: it clones Ghostty into the user's cache, downloads the Zig its `build.zig.zon` asks for from ziglang.org, checked against the index's SHA-256, and builds every desktop target, iOS devices and the simulator, stripping the Linux builds' debug sections. The builds are committed in `internal/library/lib/`, as users have no Zig ([principles](../../AGENTS.md#principles)), so nothing is downloaded at run time. A desktop's is embedded in the apps that link the terminal, so `go run` and `go test` find it; the nsgo tool also signs it into macOS apps, where hardened apps load only signed libraries, and into iOS apps as a framework, the only way iOS loads native code ([tool](tool.md#packages)). The command also writes `ghostty_themes.go`, the themes Ghostty ships; `-themes` writes them alone.

The library runs on macOS 13 or later, Linux with glibc 2.28 or later (Debian 10, Ubuntu 20.04) and Windows 10 1809 or later, whose ConPTY runs the program.

## Terminal

`Terminal` takes `Command`, `Dir`, `Env`, `Conn`, `Scrollback`, `Cursor` (which `SetCursor` changes in a running terminal) and the `OnTitle`, `OnExit` and `OnBell` callbacks. A reader goroutine writes the program's output into the emulator under the terminal's lock; a queue writes input on a goroutine of its own, so a program that reads nothing never blocks the reader answering its queries. The program starts at the first size a view gives, or at 80×24 after half a second. Its end writes a line of `locales` in the language the view last drew in.

A terminal no view shows, as a server's for a session, sizes with `Resize` and `ResizePixels` and has `Mirror`s (`mirror.go`): `Frame` returns the VT sequences that bring another terminal, one whose `Conn` reads the frames with `ResizeByConn`, to what this one shows, cell for cell, with its cursor, modes and scrollback. `ClearScreen` clears above the prompt, and programs' OSC 52 writes go on through the frames.

`Search` finds text in the screen and the scrollback, a slice of the search each frame, reports through `OnSearch` and marks the matches the viewport shows.

## View

- The screen is a `core.Canvas`. Each drawing updates libghostty-vt's render state and records the rows: background spans, the selection, then text, ASCII in runs of one style and anything else in its own cells, so a fallback font's width moves nothing after it; decorations; the cursor, hollow without the focus. The terminal's reader asks for a frame by bumping a signal through `Scope.Post`, once until the frame comes.
- Cells are the font's advance of `M` wide and its line height tall, in points; the screen is told their size in pixels by the pixel ratio. A program's synchronized update (mode 2026) holds the render state until it ends, or a second.
- A vertical `Scroll` over the canvas is the hit layer, as `DataTable`'s: its content is as tall as the viewport and the scrollback, and its offset moves the terminal's viewport a row at a time; output moves it back with `Ref.ScrollTo`, whose own report it ignores. The wheel, the touchpad and the scroll bar are then the platform's. Its content's `OnPressDown`, `OnMouseMove` and `OnPressUp` drive libghostty-vt's selection gesture.
- An invisible `Input`, the sink, sits at the cursor as `Editor`'s does, so input methods compose there: `core` has no raw key events. Its changes become backspaces and typed text, a character sent as its key so the Kitty keyboard protocol sees it, and its pastes `Terminal.Paste`, bracketed as the program asks. While it has the focus, the view binds with `KeyAlways` the keys that type no text, with every combination of Shift, Control and Alt, Control with each character key and, but on Apple platforms where Option types characters, Alt with them, and encodes them with libghostty-vt's key encoder, which follows the program's modes.
- Links are programs' hyperlinks (OSC 8) and URLs printed as text, found as Ghostty's `link-url` does in the line under the mouse, through the rows it soft-wraps over, each time the mouse changes cell, a modifier key changes or a frame draws, so output moves nothing from under it (`link.go`). A link underlines faintly as the mouse hovers and fully with a hand while Command, or Control off Apple platforms, is held, when a click opens it with `Scope.OpenURL` instead of selecting; a press released off the link opens nothing. What `OpenURL` refuses, such as `file:` URLs, is no link. An app sees what opens through `Host.HandleOpenURL`.
- The font, the themes and the padding are the view's props, so changing them builds the view again while the terminal and its program go on.

## Touch screens

Where `Env.Desktop` is false:

- A finger scrolls the scroll layer, as any scroll view; a tap gives the sink the focus, which shows the software keyboard, and a long press selects the word under the finger, as a double click does. The press's delay is 200 ms: with the 150 ms a scroll view holds its content's touches back, the word is selected before a context menu's hold of 400 ms opens it. Both of the view's scroll views keep the keyboard through taps (`KeyboardTapsAlways`), or a press on the bar would dismiss it and take the bar away before the press ended.
- A software keyboard's Return and Backspace reach no key binding: Return submits the sink, which keeps the focus and sends Enter, and Backspace is reported only in an empty sink, which sends it; elsewhere it shortens the sink's text.
- The sink's text is what was sent plus what is being composed. An input method's commit is reported before its composition ends, whose end sends how the text differs from what was sent, and nothing else goes while it composes: the keys are unbound then, for the input method, and the frame draws the composition underlined at the cursor, as the sink shows nothing.
- `Ref` copies, pastes, selects all and reports a selection as a signal, so the app's `base.ContextMenu` around the view, which opens as a sheet after a long press and at the pointer on desktops, has the menu's items: the view draws no menu of its own.
- A bar of the keys software keyboards lack (`DefaultKeyBar`: Escape, Tab, Control, Alt, the arrows and `|~/-`) shows under the grid while the sink has the focus, in a horizontal `Scroll`, styled by the app's `KeyBar*` and `Key*` parts. Its keys take no focus. Control and Alt latch for the next key, from the bar or typed, which takes them; their caps and labels are the package's catalogs'.
- The grid and the bar keep above the keyboard: the view's frame in the surface, its window's height and `Env.KeyboardHeight` give how far the keyboard covers its bottom, which an inner box leaves free, so the terminal has the rows above it.

The gallery's Terminal demo (`examples/gallery/terminal.go`) sets each option beside the view, under its API's name, and shows what the shell reports. What the terminal does not do yet is in the [roadmap](../roadmap.md#terminal).

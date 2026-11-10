# Benchmarks against other frameworks

The same three apps written for NSGo, [MyGo](https://github.com/egoist/mygo),
[NSH](https://github.com/longbridge/nsh), [GPUI Kit](https://github.com/longbridge/gpui-kit)
and [GPUI Fast](https://github.com/longbridge/gpui-fast), and the runner that
measures them on macOS, Windows and Linux. The website's Benchmarks page shows
`results/`.

| Case | What it does | Measured |
| --- | --- | --- |
| `counter` | A count and two buttons, left idle (360×260) | Startup, memory, binary size |
| `quotes` | 40 quotes, ten changed every display frame (560×860) | CPU, the compositor's CPU, memory, frames |
| `list` | 10,000 rows, scrolled a row further every display frame (480×800) | The same |

Each framework writes them its own way: NSGo's and NSH's rows are signals that
a tick sets, MyGo builds its view every frame, GPUI renders its view again
when the frame asks. The quotes' market is mulberry32 from seed 42 in both
`internal/bench` and `cases/`, whose tests pin the same rows.

| Folder | What |
| --- | --- |
| `nsgo/`, `mygo/` | The Go programs, a package a case, in this module |
| `nsh/` | NSH's, in Rust on the OS's own backend, a binary a case |
| `gpui/` | GPUI Kit's, a binary a case; the feature `fast` builds them on GPUI Fast |
| `cases/`, `internal/bench/` | What the Rust and the Go programs share |
| `runner/` | Builds, runs and measures them |
| `results/` | `darwin.json`, `windows.json` and `linux.json` |

## Running it

Go, Rust and, on Linux, GTK 4 and a desktop session. From this folder, with
nothing else running and the display awake:

```sh
go run ./runner                                   # every framework and case, 3 runs of 10 s
go run ./runner -frameworks nsgo,nsh -cases list  # some of them
go run ./runner -runs 1 -seconds 3 -out /tmp/x.json
```

It builds release binaries (`go build -ldflags "-s -w"`, `cargo build
--release` with symbols stripped, GPUI Fast in `gpui/target/fast`), then runs
each case's programs one after another, as many rounds as `-runs`, and keeps
each value's median:

- A program prints `bench ready` on its first frame, then every second
  `bench frames N`, the frames it updated. Startup is from launching it to
  the first line.
- After 3 s to settle, the runner reads the process's CPU time over
  `-seconds`, and its memory at the end: the physical footprint on macOS, the
  proportional set size on Linux, the working set on Windows.
- It reads the compositor's CPU over the same time, less what it spent at
  rest, measured before each round with none of the programs open:
  WindowServer on macOS, `dwm.exe` on Windows, the Wayland compositor and
  Xwayland on Linux. Native views (NSGo, NSH) leave drawing to it, where GPU
  renderers (MyGo, GPUI) draw in their process, so compare the two added.
  Reading `dwm.exe` takes an elevated runner.

Updates follow the display: frames stay at its refresh rate while an app keeps
up, and a 120 Hz display does twice a 60 Hz one's work. Compare results of one
machine, not across machines.

The compositor's rest follows everything else on screen, so close other
apps, or at least keep them still. On Windows, run it from the desktop's own
session, as an elevated terminal there: a program started from an SSH session
has no desktop to draw on.

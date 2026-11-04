# Benchmarks

`bench/quotes` measures the CPU cost of a moving screen: a 40-row quote
table where every display frame changes 10 rows, four text cells each,
while the other 30 rows' signals stay untouched. Its values, PRNG
(mulberry32, seed 42), column widths and window size match the gogui and
nsh ports of the same case.

As nsh does from `requestAnimationFrame`, it drives the tick from
`Scope.RequestAnimationFrame`. A callback writes signals; the engine turns
the changes into a commit. One wake-up can run several callbacks before
one commit when frames queue, and a commit is not a presented frame.
`nsgo dev` counts presented frames on macOS; this runner does not.

## Running it

```sh
bench/run.sh          # 3 s warmup, then 15 s measured
bench/run.sh 30       # 3 s warmup, then 30 s measured
```

The script builds a release binary, runs it on its own, and prints:

- `CPU`: the process' user and system CPU time, averaged over the measured
  window. 1000 ms/s is one full core. It excludes the GPU and
  WindowServer, so it is the app's CPU, not the machine's total cost.
- `frames`: animation-frame callbacks per second, from a one-second
  counter. A healthy run stays near the display's refresh rate.
- `commits`: engine commits per second, from the title's post-commit
  layout report. This is the engine's frame rate, not the display's: the
  platform may apply or present a commit later, and may coalesce several
  commits into one draw.
- `frames/commit`: how many animation-frame callbacks a commit carried.
  Above 1 means callbacks queued before the engine committed.
- `interval`: time between commit reports; p50, p95, p99 and max.
- `commit`: time from the last callback of a batch, through that commit,
  to the title's layout report; p50, p95, p99 and max.
- `latency`: time from the first callback's frame time to that report;
  p50, p95, p99 and max.
- `CPU per commit`: `CPU` divided by `commits`, which compares runs at
  different cadences without assuming they updated equally often. The line
  also prints the cost of one callback, dividing by `frames/commit`.

`fps=N` is gone. It counted animation-frame callbacks, so it could stay at
the display rate while a slow commit halved the real frame rate. Presented
frames are what `nsgo dev` counts on macOS; `commits` is what this
runner can count on every target.

## Reading the numbers

Run one binary at a time. Two benchmarks competing for the machine squeeze
each other and their numbers converge, which hides both improvements and
regressions. Compare runs on the same machine, back to back and repeated,
rather than absolute numbers from another machine.

`commit` is engine work; `CPU per commit` covers the whole process, so the
gap between them includes drawing, the frame callback, GC and the stats
themselves.

`ps` reports CPU time in centiseconds, so a short run has coarser numbers;
the 15 s default is a better comparison than a one-second run. Do not switch
windows or run other work during the measured window.

The macOS window must be on screen: a hidden or fully occluded window can
draw less, which lowers CPU without making the app cheaper. The window is
1040 points tall, so on a shorter laptop display (982 points at the default
scale on a 14-inch MacBook Pro) macOS constrains it and clips the last rows;
use a taller display or a scaled mode for a like-for-like run.

The callback follows the display, so a 120 Hz display updates twice as
often as a 60 Hz one. Compare `CPU per commit` across rates rather than
`CPU` alone.

## The case

Each row is one signal, so a tick settles four text nodes per touched row.
The title's tick count changes every frame too. The result is the per-commit
cost of preparing and measuring that text, on top of layout and drawing.

The other 30 rows do not write signals, and a commit visits only the
nodes that changed. Each value is as wide as its cell, so one that fits
it measures as before and lays nothing out: the CPU is preparing the
changed texts, their lines and their drawing.

## What it does not measure

- Presented frames (see `nsgo dev` on macOS), compositor or GPU time,
  and WindowServer CPU.
- Input latency, memory or energy.
- A slow main thread that applies commits late while the engine keeps
  committing.

Read `interval`, `commit`, `latency` and `CPU per commit` together, not a
single number. For presented frames on macOS, use the performance monitor.

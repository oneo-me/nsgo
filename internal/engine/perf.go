// Performance monitoring: what a surface's frames and commits cost while it
// runs, for an on-screen monitor (`nsgo dev`) or the app's own tooling.
//
// A PerfMonitor is shared by the two threads of a surface:
//
//   - the engine goroutine records every wake-up that produced a commit
//     (the app's code and the core's work, CommitTiming) and how long it
//     was busy at all;
//   - the main thread records the time it spends on the surface
//     (RecordWork: applying batches, ticking animations, handling input) and
//     closes a frame at every display refresh (Vsync): a refresh in which
//     something changed is a frame the screen showed, and its cost is the
//     main-thread work since the previous refresh. It also reports how long
//     a batch waited between the engine handing it over and the main thread
//     applying it (RecordLatency).
//
// Snapshot turns what was recorded into readings (PerfSnapshot); Readings
// formats them for a monitor panel.
//
// Nothing is measured while no monitor is installed (engine.Options.Perf is
// nil): the engine checks one pointer per wake-up, the platforms one per
// frame.
//
// The readings follow NSH's (`nsh_engine::perf`): the headline rate counts
// frames that reached the screen, MAX is derived from what a frame cost
// rather than counted (so the monitor never drives frames to find it), and
// the first frames, the cold ones, are not counted.
package engine

import (
	"fmt"
	"math"
	"slices"
	"sync"
	"time"
)

// Samples kept for the distributions (frames, commits).
const perfCapacity = 120

// The window over which rates (frames, commits per second) are counted.
const perfRateWindow = time.Second

// Frames and commits not counted after the monitor is installed: the first
// ones after a start are cold (caches, fonts, the first layout) and are not
// what the app costs to run.
const perfWarmup = 4

// CommitTiming is what one engine wake-up that committed cost.
type CommitTiming struct {
	// App is the app's code: event handlers, timers, effects and the other
	// work of the wake-up less Core.
	App time.Duration
	// Core is the core: applying what changed, rendering components,
	// typesetting text, layout and diffing into mount items.
	Core time.Duration
	// Items are the mount items the commit produced.
	Items int
}

// Stat is the mean, median, 95th percentile and maximum of a set of
// durations.
type Stat struct {
	Mean time.Duration
	P50  time.Duration
	P95  time.Duration
	Max  time.Duration
}

// statOf returns the statistics of samples.
func statOf(samples []time.Duration) Stat {
	if len(samples) == 0 {
		return Stat{}
	}
	sorted := append([]time.Duration(nil), samples...)
	slices.Sort(sorted)
	var sum time.Duration
	for _, d := range sorted {
		sum += d
	}
	at := func(q float64) int {
		i := int(math.Ceil(float64(len(sorted))*q)) - 1
		if i < 0 {
			i = 0
		}
		if i >= len(sorted) {
			i = len(sorted) - 1
		}
		return i
	}
	return Stat{
		Mean: sum / time.Duration(len(sorted)),
		P50:  sorted[at(0.5)],
		P95:  sorted[at(0.95)],
		Max:  sorted[len(sorted)-1],
	}
}

// ProcessSample is the process' resource use.
type ProcessSample struct {
	// CPU is CPU time over wall time since the previous sample, in percent
	// of one core (as top and Activity Monitor show it: 150 is one and a
	// half cores busy).
	CPU *float32
	// Memory is the memory the process owns, in bytes: the physical
	// footprint on Apple platforms (Activity Monitor's Memory column), the
	// resident set elsewhere.
	Memory *uint64
	// GPU is GPU time over wall time, in percent, where the platform tells
	// it per process (macOS).
	GPU *float32
}

// PerfSnapshot is what a PerfMonitor read, as of one moment.
type PerfSnapshot struct {
	// FPS is frames shown per second over the last second: display
	// refreshes in which the surface changed. Low while the app is idle,
	// which is not slowness.
	FPS float32
	// MaxFPS is what the main thread could keep up with: one over the mean
	// frame cost, capped at the display's rate when it is known.
	MaxFPS float32
	// RefreshRate is the rate the display link asks for (Hz), once it ran.
	RefreshRate *float32
	// TickInterval is the gaps between refreshes as they were delivered,
	// over the last capacity: a mean above one period means refreshes came
	// late or were skipped (a main thread busy when they were due).
	TickInterval Stat
	// FrameInterval is the gaps between shown frames over the last
	// capacity (long while the app is idle, one period while it animates
	// smoothly).
	FrameInterval Stat
	// Frame is main-thread time per shown frame (applying batches,
	// animations, input), over the last capacity frames.
	Frame Stat
	// Dropped is the share of those frames that took longer than one
	// refresh, in 0..=1.
	Dropped float32
	// Commits is commits per second over the last second.
	Commits float32
	// App is the app's code per commit.
	App Stat
	// Core is the core's work per commit.
	Core Stat
	// Items is the mean mount items per commit.
	Items float32
	// Latency is how long batches waited between the engine handing them
	// over and the main thread applying them.
	Latency Stat
	// EngineBusy is the engine goroutine's busy time over the last second,
	// in percent.
	EngineBusy float32
	// Frames are the main-thread costs of the retained frames, oldest
	// first (for a trace).
	Frames []time.Duration
	// Process is the process' resource use, as last given to SetProcess.
	Process ProcessSample
}

// frame is one shown frame: when it was closed and what it cost.
type frame struct {
	at   time.Time
	cost time.Duration
}

// commit is one commit: when it finished and what it cost.
type commit struct {
	at     time.Time
	timing CommitTiming
}

// busy is one engine wake-up: when it ended and how long it was busy.
type busy struct {
	at time.Time
	d  time.Duration
}

// PerfMonitor collects a surface's frame and commit timings. It is shared by
// its engine goroutine and its main thread, and safe for both.
type PerfMonitor struct {
	mu    sync.Mutex
	inner perfInner
}

type perfInner struct {
	// Main-thread work since the last refresh, and whether it changed
	// anything.
	pending time.Duration
	dirty   bool
	// Shown frames: when, and their main-thread cost.
	frames      []frame
	frameWarmup int
	refresh     float32
	hasRefresh  bool
	// The previous refresh, and the gaps between refreshes as they were
	// delivered (a late or skipped one shows as a long gap).
	lastVsync time.Time
	ticks     []time.Duration
	commits   []commit
	// Commits not counted after the monitor is installed.
	commitWarmup int
	latencies    []time.Duration
	// Engine wake-ups: when they ended, and how long they were busy.
	busy    []busy
	process ProcessSample
}

// NewPerfMonitor returns an empty monitor.
func NewPerfMonitor() *PerfMonitor {
	return &PerfMonitor{inner: perfInner{frameWarmup: perfWarmup, commitWarmup: perfWarmup}}
}

// push appends value to queue, dropping the oldest sample when it is full.
func push[T any](queue []T, value T) []T {
	if len(queue) == perfCapacity {
		copy(queue, queue[1:])
		queue[perfCapacity-1] = value
		return queue
	}
	return append(queue, value)
}

// RecordWork records main-thread work on the surface: cost was spent, and
// dirty reports whether it changed what the screen shows (a batch was
// applied, an animation stepped, a scroll view moved).
func (m *PerfMonitor) RecordWork(cost time.Duration, dirty bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inner.pending += cost
	m.inner.dirty = m.inner.dirty || dirty
}

// Vsync records that the display refreshed at at, interval after the previous
// refresh (the display link's period). It closes the frame the work since the
// previous refresh belongs to.
func (m *PerfMonitor) Vsync(at time.Time, interval time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	inner := &m.inner
	if interval > 0 {
		rate := float32(1 / interval.Seconds())
		// Smoothed: a link's period is exact, a measured one jitters.
		if inner.hasRefresh && abs32(inner.refresh-rate) < inner.refresh*0.1 {
			inner.refresh += (rate - inner.refresh) * 0.1
		} else {
			inner.refresh = rate
		}
		inner.hasRefresh = true
	}
	if !inner.lastVsync.IsZero() {
		gap := at.Sub(inner.lastVsync)
		// A pause (nothing wanted frames, the link stopped) is not a late
		// refresh.
		if gap < 100*time.Millisecond {
			inner.ticks = push(inner.ticks, gap)
		}
	}
	inner.lastVsync = at
	cost := inner.pending
	inner.pending = 0
	dirty := inner.dirty
	inner.dirty = false
	if !dirty {
		return
	}
	if inner.frameWarmup > 0 {
		inner.frameWarmup--
		return
	}
	inner.frames = push(inner.frames, frame{at: at, cost: cost})
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// RecordLatency records that a batch was applied latency after the engine
// handed it over.
func (m *PerfMonitor) RecordLatency(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// The first batches wait for the cold first frames.
	if m.inner.frameWarmup == 0 {
		m.inner.latencies = push(m.inner.latencies, latency)
	}
}

// RecordCommit records an engine wake-up that committed.
func (m *PerfMonitor) RecordCommit(timing CommitTiming) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.inner.commitWarmup > 0 {
		m.inner.commitWarmup--
		return
	}
	m.inner.commits = push(m.inner.commits, commit{at: time.Now(), timing: timing})
}

// RecordBusy records that an engine wake-up was busy for d.
func (m *PerfMonitor) RecordBusy(d time.Duration) {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	inner := &m.inner
	inner.busy = append(inner.busy, busy{at: now, d: d})
	for len(inner.busy) > 0 && now.Sub(inner.busy[0].at) > perfRateWindow {
		inner.busy = inner.busy[1:]
	}
}

// SetProcess stores the process' latest resource use, shown with the next
// snapshots.
func (m *PerfMonitor) SetProcess(sample ProcessSample) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inner.process = sample
}

// Snapshot returns the readings as of now.
func (m *PerfMonitor) Snapshot() PerfSnapshot {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	inner := &m.inner
	recent := func(at time.Time) bool {
		return now.Sub(at) <= perfRateWindow
	}
	// n frames span n - 1 intervals: the rate is measured over the span they
	// cover, which keeps it right before the window fills.
	shown := make([]time.Time, 0, len(inner.frames))
	for _, f := range inner.frames {
		if recent(f.at) {
			shown = append(shown, f.at)
		}
	}
	var fps float32
	if len(shown) >= 2 && shown[len(shown)-1].After(shown[0]) {
		// The span to now when the frames stopped: an app that went idle
		// reads as fewer frames, not as its last burst.
		span := now.Sub(shown[0])
		if last := shown[len(shown)-1].Sub(shown[0]); last > span {
			span = last
		}
		fps = float32(len(shown)-1) / float32(span.Seconds())
	}
	costs := make([]time.Duration, len(inner.frames))
	for i, f := range inner.frames {
		costs[i] = f.cost
	}
	frameStat := statOf(costs)
	budget := time.Second / 60
	if inner.hasRefresh && inner.refresh > 0 {
		budget = time.Duration(float64(time.Second) / float64(inner.refresh))
	}
	var dropped float32
	if len(inner.frames) > 0 {
		over := 0
		for _, f := range inner.frames {
			if f.cost > budget {
				over++
			}
		}
		dropped = float32(over) / float32(len(inner.frames))
	}
	var maxFPS float32
	if frameStat.Mean > 0 {
		maxFPS = float32(1 / frameStat.Mean.Seconds())
		if inner.hasRefresh {
			maxFPS = min(maxFPS, inner.refresh)
		}
	}
	commits := 0
	for _, c := range inner.commits {
		if recent(c.at) {
			commits++
		}
	}
	var items float32
	if len(inner.commits) > 0 {
		total := 0
		for _, c := range inner.commits {
			total += c.timing.Items
		}
		items = float32(total) / float32(len(inner.commits))
	}
	appSamples := make([]time.Duration, len(inner.commits))
	coreSamples := make([]time.Duration, len(inner.commits))
	for i, c := range inner.commits {
		appSamples[i] = c.timing.App
		coreSamples[i] = c.timing.Core
	}
	frameIntervals := make([]time.Duration, 0, len(inner.frames))
	for i := 1; i < len(inner.frames); i++ {
		frameIntervals = append(frameIntervals, inner.frames[i].at.Sub(inner.frames[i-1].at))
	}
	var busyTime time.Duration
	for _, b := range inner.busy {
		if recent(b.at) {
			busyTime += b.d
		}
	}
	var refresh *float32
	if inner.hasRefresh {
		rate := inner.refresh
		refresh = &rate
	}
	frames := make([]time.Duration, len(inner.frames))
	for i, f := range inner.frames {
		frames[i] = f.cost
	}
	process := inner.process
	return PerfSnapshot{
		FPS:           fps,
		MaxFPS:        maxFPS,
		RefreshRate:   refresh,
		TickInterval:  statOf(inner.ticks),
		FrameInterval: statOf(frameIntervals),
		Frame:         frameStat,
		Dropped:       dropped,
		Commits:       float32(commits),
		App:           statOf(appSamples),
		Core:          statOf(coreSamples),
		Items:         items,
		Latency:       statOf(inner.latencies),
		EngineBusy:    float32(busyTime) / float32(perfRateWindow) * 100,
		Frames:        frames,
		Process:       process,
	}
}

// Readings formats a snapshot as the performance monitor's panel shows it:
// milliseconds as mean and 95th percentile, CPU and GPU as percent of one
// core, memory in megabytes.
func (s PerfSnapshot) Readings() string {
	rate := "-"
	if s.RefreshRate != nil {
		rate = fmt.Sprintf("%.0f Hz", *s.RefreshRate)
	}
	percent := func(v *float32) string {
		if v == nil {
			return "-"
		}
		return fmt.Sprintf("%.0f%%", *v)
	}
	memory := "-"
	if s.Process.Memory != nil {
		memory = fmt.Sprintf("%.0f MB", float64(*s.Process.Memory)/1e6)
	}
	return fmt.Sprintf("%4.0f FPS   max %.0f · %s\n"+
		"TICK %s  gap %.1f\n"+
		"MAIN %s  drop %.0f%%\n"+
		"APP  %s  %.0f/s\n"+
		"CORE %s  %.0f items\n"+
		"HOP  %s  eng %.0f%%\n"+
		"CPU %s  GPU %s  MEM %s",
		s.FPS, s.MaxFPS, rate,
		statText(s.TickInterval), milliseconds(s.FrameInterval.P95),
		statText(s.Frame), s.Dropped*100,
		statText(s.App), s.Commits,
		statText(s.Core), s.Items,
		statText(s.Latency), s.EngineBusy,
		percent(s.Process.CPU), percent(s.Process.GPU), memory)
}

func statText(s Stat) string {
	return fmt.Sprintf("%6.2f  p95 %6.2f", milliseconds(s.Mean), milliseconds(s.P95))
}

func milliseconds(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

// String is the snapshot as one line, as nsgo test -fps logs it.
func (s PerfSnapshot) String() string {
	ms := func(d time.Duration) string { return fmt.Sprintf("%.2fms", float64(d.Microseconds())/1000) }
	line := fmt.Sprintf("fps %.0f (max %.0f) frame %s p95 %s dropped %.0f%% commits %.0f/s app %s core %s items %.0f latency %s engine %.0f%%",
		s.FPS, s.MaxFPS, ms(s.Frame.Mean), ms(s.Frame.P95), s.Dropped*100, s.Commits, ms(s.App.Mean), ms(s.Core.Mean),
		s.Items, ms(s.Latency.Mean), s.EngineBusy)
	if c := s.Process.CPU; c != nil {
		line += fmt.Sprintf(" cpu %.0f%%", *c)
	}
	if g := s.Process.GPU; g != nil {
		line += fmt.Sprintf(" gpu %.0f%%", *g)
	}
	if m := s.Process.Memory; m != nil {
		line += fmt.Sprintf(" memory %.1fMB", float64(*m)/(1<<20))
	}
	return line
}

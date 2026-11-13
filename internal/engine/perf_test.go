package engine

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

func ms(n int64) time.Duration { return time.Duration(n) * time.Millisecond }

func TestStatOfSamples(t *testing.T) {
	samples := make([]time.Duration, 0, 20)
	for i := int64(1); i <= 20; i++ {
		samples = append(samples, ms(i))
	}
	stat := statOf(samples)
	if stat.Mean != 10500*time.Microsecond || stat.P50 != ms(10) || stat.P95 != ms(19) || stat.Max != ms(20) {
		t.Fatalf("stat = %+v", stat)
	}
	if statOf(nil) != (Stat{}) {
		t.Fatal("empty samples have statistics")
	}
}

func TestReadingsOfEmptySnapshot(t *testing.T) {
	text := (PerfSnapshot{}).Readings()
	if !strings.HasPrefix(text, "   0 FPS   max 0 · -") {
		t.Errorf("readings = %q", text)
	}
	if !strings.Contains(text, "CPU -  GPU -  MEM -") {
		t.Errorf("readings = %q", text)
	}
}

func TestFramesAreRefreshesThatChangedSomething(t *testing.T) {
	monitor := NewPerfMonitor()
	start := time.Now()
	period := 8333 * time.Microsecond
	for i := 0; i < perfWarmup+10; i++ {
		monitor.RecordWork(ms(1), true)
		// Work that changed nothing is part of the next frame's cost.
		monitor.RecordWork(ms(1), false)
		monitor.Vsync(start.Add(period*time.Duration(i)), period)
		// A refresh with nothing new is not a frame.
		monitor.Vsync(start.Add(period*time.Duration(i)+period/2), period)
	}
	snapshot := monitor.Snapshot()
	if len(snapshot.Frames) != 10 {
		t.Fatalf("frames = %d, want 10", len(snapshot.Frames))
	}
	if snapshot.Frame.Mean != ms(2) {
		t.Errorf("frame mean = %v, want 2ms", snapshot.Frame.Mean)
	}
	if snapshot.RefreshRate == nil || *(snapshot.RefreshRate)-120 > 0.5 || 120-*(snapshot.RefreshRate) > 0.5 {
		t.Errorf("refresh rate = %v, want 120", snapshot.RefreshRate)
	}
	// 2ms frames could run at 500 per second, but not on a 120 Hz display.
	if snapshot.MaxFPS-120 > 0.5 || 120-snapshot.MaxFPS > 0.5 {
		t.Errorf("max fps = %v, want 120", snapshot.MaxFPS)
	}
	if snapshot.Dropped != 0 {
		t.Errorf("dropped = %v, want 0", snapshot.Dropped)
	}
}

func TestSlowFramesAreDropped(t *testing.T) {
	monitor := NewPerfMonitor()
	start := time.Now()
	period := ms(16)
	for i := 0; i < perfWarmup+4; i++ {
		if i%2 == 0 {
			monitor.RecordWork(ms(20), true)
		} else {
			monitor.RecordWork(ms(1), true)
		}
		monitor.Vsync(start.Add(period*time.Duration(i)), period)
	}
	if got := monitor.Snapshot().Dropped; got != 0.5 {
		t.Errorf("dropped = %v, want 0.5", got)
	}
}

func TestCommitsAfterWarmup(t *testing.T) {
	monitor := NewPerfMonitor()
	for i := 0; i < perfWarmup+3; i++ {
		monitor.RecordCommit(CommitTiming{App: ms(1), Core: ms(2), Items: 4})
	}
	snapshot := monitor.Snapshot()
	if snapshot.Commits != 3 {
		t.Errorf("commits = %v, want 3", snapshot.Commits)
	}
	if snapshot.App.Mean != ms(1) || snapshot.Core.P95 != ms(2) || snapshot.Items != 4 {
		t.Errorf("commit readings = %+v", snapshot)
	}
}

func TestPassesAreTheEnginesWakeUps(t *testing.T) {
	monitor := NewPerfMonitor()
	for range 4 {
		monitor.RecordBusy(ms(2))
	}
	snapshot := monitor.Snapshot()
	if snapshot.Passes != 4 {
		t.Errorf("passes = %v, want 4", snapshot.Passes)
	}
}

func TestLatencyIsKeptAfterTheColdFrames(t *testing.T) {
	monitor := NewPerfMonitor()
	monitor.RecordLatency(ms(9))
	if got := monitor.Snapshot().Latency; got != (Stat{}) {
		t.Fatalf("cold latency = %+v, want none", got)
	}
	start := time.Now()
	for i := 0; i < perfWarmup; i++ {
		monitor.RecordWork(ms(1), true)
		monitor.Vsync(start.Add(time.Duration(i)*ms(16)), ms(16))
	}
	monitor.RecordLatency(ms(3))
	if got := monitor.Snapshot().Latency.Mean; got != ms(3) {
		t.Errorf("latency = %v, want 3ms", got)
	}
}

func TestIdleRefreshesClearWork(t *testing.T) {
	monitor := NewPerfMonitor()
	start := time.Now()
	monitor.RecordWork(ms(5), false)
	monitor.Vsync(start, ms(16))
	// The idle work is gone: the frames that follow cost only their own work.
	for i := 0; i < perfWarmup; i++ {
		monitor.RecordWork(ms(1), true)
		monitor.Vsync(start.Add(time.Duration(i+1)*ms(16)), ms(16))
	}
	monitor.RecordWork(ms(2), true)
	monitor.Vsync(start.Add(time.Duration(perfWarmup+1)*ms(16)), ms(16))
	if got := monitor.Snapshot().Frame.Mean; got != ms(2) {
		t.Errorf("frame mean = %v, want 2ms", got)
	}
}

func TestSamplesStayBounded(t *testing.T) {
	monitor := NewPerfMonitor()
	for i := 0; i < perfWarmup+perfCapacity+20; i++ {
		monitor.RecordCommit(CommitTiming{App: ms(1), Core: ms(1), Items: 1})
	}
	monitor.mu.Lock()
	commits := len(monitor.inner.commits)
	monitor.mu.Unlock()
	if commits != perfCapacity {
		t.Errorf("commits kept = %d, want %d", commits, perfCapacity)
	}
}

func TestProcessSampleInSnapshot(t *testing.T) {
	monitor := NewPerfMonitor()
	cpu, gpu := float32(12), float32(3)
	memory := uint64(96_000_000)
	monitor.SetProcess(ProcessSample{CPU: &cpu, GPU: &gpu, Memory: &memory})
	snapshot := monitor.Snapshot()
	if snapshot.Process.CPU == nil || *snapshot.Process.CPU != 12 {
		t.Errorf("cpu = %v", snapshot.Process.CPU)
	}
	if !strings.Contains(snapshot.Readings(), "CPU 12%  GPU 3%  MEM 96 MB") {
		t.Errorf("readings = %q", snapshot.Readings())
	}
}

func TestPerfRecordsCommitItems(t *testing.T) {
	p := &recorder{mounted: make(chan struct{}, 16)}
	monitor := NewPerfMonitor()
	var count *core.Signal[int]
	e := Start(p, func(cx *core.Scope) core.Node {
		count = core.NewSignal(cx, 0)
		return core.View{OnPress: func(core.PressEvent) { count.Update(func(n int) int { return n + 1 }) }, TestID: "button", Children: []core.Node{
			core.Text{ContentFunc: func() string { return strconv.Itoa(count.Get()) }},
		}}
	}, Options{Size: driver.Size{Width: 100, Height: 100}, Perf: monitor})
	defer e.Close()
	<-p.mounted
	var button driver.NodeID
	p.mu.Lock()
	for _, b := range p.batches {
		for _, it := range b.Items {
			if it.Kind == driver.ItemViewProps && it.ViewProps.TestID == "button" {
				button = it.ID
			}
		}
	}
	p.mu.Unlock()
	if button == 0 {
		t.Fatal("button not found")
	}
	for range perfWarmup + 2 {
		e.Dispatch(driver.Event{Kind: driver.EventPress, ID: button})
		select {
		case <-p.mounted:
		case <-time.After(2 * time.Second):
			t.Fatal("no commit")
		}
	}
	if got := monitor.Snapshot().Items; got == 0 {
		t.Fatal("commits have no mount items")
	}
	p.mu.Lock()
	batches := len(p.batches)
	last := p.batches[batches-1]
	p.mu.Unlock()
	if last.Sent.IsZero() {
		t.Fatal("a measured commit was not timestamped")
	}
}

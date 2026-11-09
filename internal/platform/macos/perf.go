//go:build darwin && !ios

package macos

import (
	"sync"
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/engine"
	"github.com/oneo-me/nsgo/internal/platform/apple"
	"github.com/oneo-me/nsgo/internal/platform/perfhud"
)

// The performance monitor's HUD (`nsgo dev`): a panel at the top right
// of the surface that shows what its frames and commits cost (engine/perf.go),
// redrawn twice a second. It follows NSH's macOS HUD.
//
// - FPS: frames shown over the last second (display refreshes in which the
//   surface changed: an idle app reads low, which is not slowness); MAX is
//   what the main thread could keep up with (one over the mean frame cost,
//   capped at the display's rate), then the display link's rate.
// - TICK: the gaps between refreshes as they arrived (mean, p95) and gap,
//   the p95 gap between shown frames.
// - MAIN: main-thread time per shown frame, its 95th percentile, and drop,
//   the share of frames over one refresh.
// - APP / CORE: per commit, the app's code and the core's work; commits per
//   second and mount items per commit.
// - HOP: how long batches waited between the engine goroutine and the main
//   thread; the engine goroutine's busy share.
// - CPU (100 is one core), GPU (this process' share, from the IO registry),
//   MEM (the physical footprint).
// - The trace: the last 120 frames' main-thread cost against the frame
//   budget (the line), green within half of it, amber within it, red over.
//
// The panel lets the mouse through and fades out while the pointer is over
// it, so what it covers stays visible. It measures nothing while hidden: the
// monitor is removed with it.

// The panel's sizes and colors are engine's (perfpanel.go).
const (
	perfInterval    = engine.PerfInterval
	perfWidth       = engine.PerfWidth
	perfMargin      = engine.PerfMargin
	perfPadding     = engine.PerfPadding
	perfGraphHeight = engine.PerfGraphHeight
)

var (
	selFittingSize             = sel("fittingSize")
	selSetBackgroundColor      = sel("setBackgroundColor:")
	selSetCornerRadius         = sel("setCornerRadius:")
	selTargetTimestamp         = sel("targetTimestamp")
	selPreferredMaxLayoutWidth = sel("setPreferredMaxLayoutWidth:")
	selSubviews                = sel("subviews")
	selLastObject              = sel("lastObject")
)

// measuring starts timing main-thread work when a monitor measures, and
// returns the function that records it; dirty means it changes what the
// screen shows. Without a monitor the returned function does nothing.
func (s *surface) measuring(dirty bool) func() {
	if s == nil || s.perf == nil {
		return func() {}
	}
	start := time.Now()
	return func() { s.perf.RecordWork(time.Since(start), dirty) }
}

// hud is one surface's performance panel.
type hud struct {
	surface *surface
	monitor *engine.PerfMonitor
	panel   objc.ID
	label   objc.ID
	trace   objc.ID
	sampler perfhud.Sampler
	gpu     *apple.GPUProbe
	// snapshot is what the panel shows.
	snapshot engine.PerfSnapshot
	done     chan struct{}
	once     sync.Once
}

// The HUDs by their trace views, for the view's drawRect.
var (
	perfTraces                     = map[objc.ID]*hud{}
	perfClassOnce                  sync.Once
	classPerfPanel, classPerfTrace objc.Class
)

// newHud shows a panel over s and starts measuring its surface.
func newHud(s *surface, monitor *engine.PerfMonitor) *hud {
	registerPerfClasses()
	panel := objc.ID(classPerfPanel).Send(selAlloc).Send(sel("initWithFrame:"), apple.Rect(0, 0, 0, 0))
	panel.Send(selSetWantsLayer, true)
	panel.Send(selSetAccessibilityElement, false)
	panel.Send(selSetAccessibilityHidden, true)
	layer := panel.Send(selLayer)
	fill := apple.CGColor(engine.PerfPanelColor)
	layer.Send(selSetBackgroundColor, fill)
	apple.Release(fill)
	layer.Send(selSetCornerRadius, 8.0)
	trackingArea(panel, false).Send(selRelease)

	label := objc.ID(apple.Class("NSTextField")).Send(sel("labelWithString:"), apple.NSString(""))
	font := objc.ID(apple.Class("NSFont")).Send(sel("monospacedSystemFontOfSize:weight:"), 10.5, 0.0)
	label.Send(selSetFont, font)
	label.Send(selSetTextColor, nsColor(engine.PerfTextColor))
	label.Send(selSetAccessibilityElement, false)
	label.Send(selSetAccessibilityHidden, true)
	label.Send(sel("cell")).Send(sel("setWraps:"), true)
	label.Send(selPreferredMaxLayoutWidth, perfWidth-2*perfPadding)

	trace := objc.ID(classPerfTrace).Send(selAlloc).Send(sel("initWithFrame:"), apple.Rect(0, 0, 0, 0))
	trace.Send(selSetAccessibilityElement, false)
	trace.Send(selSetAccessibilityHidden, true)
	panel.Send(selAddSubview, label)
	panel.Send(selAddSubview, trace)
	trace.Send(selRelease)
	s.native.Send(selAddSubview, panel)

	h := &hud{
		surface: s,
		monitor: monitor,
		panel:   panel,
		label:   label,
		trace:   trace,
		gpu:     apple.NewGPUProbe(),
		done:    make(chan struct{}),
	}
	perfTraces[trace] = h
	h.update()
	go h.run()
	return h
}

// run samples the process and redraws the readings while the HUD shows.
func (h *hud) run() {
	ticker := time.NewTicker(perfInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			sample := h.sampler.Sample()
			if h.gpu != nil {
				sample.GPU = h.gpu.Sample()
			}
			h.monitor.SetProcess(sample)
			apple.OnMain(h.update)
		case <-h.done:
			return
		}
	}
}

// update redraws the readings and the trace.
func (h *hud) update() {
	select {
	case <-h.done:
		return
	default:
	}
	h.snapshot = h.monitor.Snapshot()
	h.label.Send(selSetString, apple.NSString(h.snapshot.Readings()))
	h.layout()
	h.trace.Send(selSetNeedsDisplay, true)
}

// layout sizes the panel to its text, at the top right of its superview
// (a flipped surface: y grows downwards).
func (h *hud) layout() {
	parent := h.panel.Send(selSuperview)
	if parent == 0 {
		return
	}
	bounds := objc.Send[apple.CGRect](parent, selBounds)
	fit := objc.Send[apple.CGSize](h.label, selFittingSize)
	height := 3*perfPadding + fit.Height + perfGraphHeight
	h.panel.Send(selSetFrame, apple.Rect(
		bounds.Size.Width-perfWidth-perfMargin,
		perfMargin,
		perfWidth, height))
	h.label.Send(selSetFrame, apple.Rect(perfPadding, 2*perfPadding+perfGraphHeight,
		perfWidth-2*perfPadding, fit.Height))
	h.trace.Send(selSetFrame, apple.Rect(perfPadding, perfPadding,
		perfWidth-2*perfPadding, perfGraphHeight))
}

// raise puts the panel above the views a batch inserted after it.
func (h *hud) raise() {
	parent := h.panel.Send(selSuperview)
	if parent == 0 {
		return
	}
	if parent.Send(selSubviews).Send(selLastObject) == h.panel {
		return
	}
	h.panel.Send(selRemoveFromSuperview)
	parent.Send(selAddSubview, h.panel)
}

// stop removes the panel and stops sampling.
func (h *hud) stop() {
	h.once.Do(func() {
		close(h.done)
		delete(perfTraces, h.trace)
		h.panel.Send(selRemoveFromSuperview)
		h.panel.Send(selRelease)
		h.surface.perf = nil
		h.surface.hud = nil
	})
}

// registerPerfClasses registers the panel and the trace view once, when the
// HUD is first shown.
func registerPerfClasses() {
	perfClassOnce.Do(func() {
		nsView := apple.Class("NSView")
		passThrough := func(objc.ID, objc.SEL, apple.CGPoint) objc.ID { return 0 }
		// Tracking areas ignore hitTest, and a transparent view keeps its
		// own; a hidden one would not report the pointer leaving.
		fade := func(alpha float64) func(objc.ID, objc.SEL, objc.ID) {
			return func(self objc.ID, _ objc.SEL, _ objc.ID) {
				self.Send(sel("animator")).Send(sel("setAlphaValue:"), alpha)
			}
		}
		classPerfPanel = register("NsgoPerfPanel", nsView, nil, []objc.MethodDef{
			{Cmd: sel("hitTest:"), Fn: passThrough},
			{Cmd: sel("mouseEntered:"), Fn: fade(0)},
			{Cmd: sel("mouseExited:"), Fn: fade(1)},
		})
		classPerfTrace = register("NsgoPerfTrace", nsView, nil, []objc.MethodDef{
			{Cmd: sel("hitTest:"), Fn: passThrough},
			{Cmd: sel("drawRect:"), Fn: drawPerfTrace},
		})
	})
}

// drawPerfTrace draws the trace of a HUD's trace view.
func drawPerfTrace(self objc.ID, _ objc.SEL, _ apple.CGRect) {
	h := perfTraces[self]
	if h == nil {
		return
	}
	context := objc.ID(apple.Class("NSGraphicsContext")).Send(selCurrentContext)
	if context == 0 {
		return
	}
	ctx := objc.Send[uintptr](context, selCGContext)
	if ctx == 0 {
		return
	}
	bounds := objc.Send[apple.CGRect](self, selBounds)
	perfhud.DrawTrace(ctx, h.snapshot, bounds.Size.Width, bounds.Size.Height, false)
}

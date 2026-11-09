//go:build ios

package ios

import (
	"sync"
	"time"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/engine"
	"github.com/oneo-me/nsgo/internal/platform/apple"
	"github.com/oneo-me/nsgo/internal/platform/perfhud"
)

// The performance monitor's panel (`nsgo dev` with fps on), as macOS's:
// the readings (engine.PerfSnapshot.Readings) and the trace of the last
// frames' main-thread cost, at the top right of the surface below the
// safe area, redrawn twice a second. Touches pass through it. While it
// measures the display link runs on every refresh, so an idle app still
// reports the display's rate.

// hud is a surface's performance panel.
type hud struct {
	s        *surface
	monitor  *engine.PerfMonitor
	panel    objc.ID
	label    objc.ID
	trace    objc.ID
	sampler  perfhud.Sampler
	snapshot engine.PerfSnapshot
	done     chan struct{}
}

var (
	// perfTraces are the HUDs by their trace views, for drawRect:.
	perfTraces      = map[objc.ID]*hud{}
	perfClassOnce   sync.Once
	classPerfTrace  objc.Class
	uiGraphicsGetCx func() uintptr
)

// measuring starts timing main-thread work when a monitor measures, and
// returns what records it; dirty says the work changes what the screen
// shows.
func (s *surface) measuring(dirty bool) func() {
	if s.perf == nil {
		return func() {}
	}
	start := time.Now()
	return func() { s.perf.RecordWork(time.Since(start), dirty) }
}

func newHud(s *surface, monitor *engine.PerfMonitor) *hud {
	perfClassOnce.Do(func() {
		if uikit, err := purego.Dlopen(uikitPath, purego.RTLD_NOW); err == nil {
			purego.RegisterLibFunc(&uiGraphicsGetCx, uikit, "UIGraphicsGetCurrentContext")
		}
		classPerfTrace = register("NsgoPerfTrace", apple.Class("UIView"), nil, []objc.MethodDef{
			{Cmd: sel("drawRect:"), Fn: drawPerfTrace},
		})
	})
	panel := objc.ID(apple.Class("UIView")).Send(selAlloc).Send(sel("initWithFrame:"), apple.Rect(0, 0, 0, 0))
	panel.Send(sel("setUserInteractionEnabled:"), false)
	panel.Send(sel("setBackgroundColor:"), uiColor(engine.PerfPanelColor))
	panel.Send(selLayer).Send(sel("setCornerRadius:"), engine.PerfRadius)
	panel.Send(sel("setAccessibilityElementsHidden:"), true)
	label := objc.ID(apple.Class("UILabel")).Send(selAlloc).Send(sel("initWithFrame:"), apple.Rect(0, 0, 0, 0))
	label.Send(sel("setNumberOfLines:"), 0)
	label.Send(selSetFont, objc.ID(apple.Class("UIFont")).Send(sel("monospacedSystemFontOfSize:weight:"),
		engine.PerfFontSize, 0.0))
	label.Send(selSetTextColor, uiColor(engine.PerfTextColor))
	trace := objc.ID(classPerfTrace).Send(selAlloc).Send(sel("initWithFrame:"), apple.Rect(0, 0, 0, 0))
	trace.Send(sel("setOpaque:"), false)
	trace.Send(sel("setBackgroundColor:"), objc.ID(apple.Class("UIColor")).Send(sel("clearColor")))
	panel.Send(selAddSubview, label)
	panel.Send(selAddSubview, trace)
	label.Send(selRelease)
	trace.Send(selRelease)
	s.native.Send(selAddSubview, panel)
	panel.Send(selRelease)
	h := &hud{s: s, monitor: monitor, panel: panel, label: label, trace: trace, done: make(chan struct{})}
	perfTraces[trace] = h
	h.update()
	go h.run()
	return h
}

// run samples the process and redraws the readings while the panel shows.
func (h *hud) run() {
	ticker := time.NewTicker(engine.PerfInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			h.monitor.SetProcess(h.sampler.Sample())
			apple.OnMain(h.update)
		case <-h.done:
			return
		}
	}
}

// update shows the latest readings.
func (h *hud) update() {
	h.snapshot = h.monitor.Snapshot()
	h.label.Send(sel("setText:"), apple.NSString(h.snapshot.Readings()))
	h.layout()
	h.trace.Send(sel("setNeedsDisplay"))
}

// layout puts the panel at the top right below the safe area, sized to
// its text, and above the surface's views.
func (h *hud) layout() {
	const w, m, pad, graph = engine.PerfWidth, engine.PerfMargin, engine.PerfPadding, engine.PerfGraphHeight
	bounds := objc.Send[apple.CGRect](h.s.native, selBounds)
	in := objc.Send[edgeInsets](h.s.native, sel("safeAreaInsets"))
	fit := objc.Send[apple.CGSize](h.label, sel("sizeThatFits:"), apple.CGSize{Width: w - 2*pad, Height: 1e4})
	width := min(w, bounds.Size.Width-2*m)
	h.panel.Send(selSetFrame, apple.Rect(bounds.Size.Width-in.Right-width-m, in.Top+m, width, 3*pad+graph+fit.Height))
	h.trace.Send(selSetFrame, apple.Rect(pad, pad, width-2*pad, graph))
	h.label.Send(selSetFrame, apple.Rect(pad, 2*pad+graph, width-2*pad, fit.Height))
	h.s.native.Send(sel("bringSubviewToFront:"), h.panel)
}

// drawPerfTrace draws a panel's trace; UIKit's context is y down.
func drawPerfTrace(self objc.ID, _ objc.SEL, _ apple.CGRect) {
	h := perfTraces[self]
	if h == nil || uiGraphicsGetCx == nil {
		return
	}
	ctx := uiGraphicsGetCx()
	if ctx == 0 {
		return
	}
	b := objc.Send[apple.CGRect](self, selBounds)
	perfhud.DrawTrace(ctx, h.snapshot, b.Size.Width, b.Size.Height, true)
}

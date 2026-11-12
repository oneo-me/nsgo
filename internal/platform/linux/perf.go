//go:build linux && !android

package linux

import (
	"time"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/internal/engine"
	"github.com/nsgo-dev/nsgo/internal/platform/perfhud"
)

// The performance monitor's panel (`nsgo dev` with fps on), as macOS's:
// the readings (engine.PerfSnapshot.Readings) and the trace of the last
// frames' main-thread cost, drawn at the top right of the surface over
// the root view and redrawn twice a second. While it measures, a tick
// callback on the surface's frame clock runs at every refresh and closes
// the frame (Vsync); the main thread's work is the batches applied, the
// animation ticks, and each frame's layout and paint (from the frame
// clock's layout signal to its after-paint one).

var (
	gtkWidgetAddTickCallback    func(widget, callback, data, destroy uintptr) uint32
	gtkWidgetGetFrameClock      func(widget uintptr) uintptr
	gdkFrameClockGetRefreshInfo func(clock uintptr, base int64, interval, presentation *int64)
	gdkFrameClockGetFrameTime   func(clock uintptr) int64
	cbPerfTick, cbPerfLayout    uintptr
	cbPerfPainted               uintptr
	perfBound                   bool
)

func bindPerf() {
	if perfBound {
		return
	}
	perfBound = true
	reg := purego.RegisterLibFunc
	reg(&gtkWidgetAddTickCallback, gtkLib, "gtk_widget_add_tick_callback")
	reg(&gtkWidgetGetFrameClock, gtkLib, "gtk_widget_get_frame_clock")
	reg(&gdkFrameClockGetRefreshInfo, gtkLib, "gdk_frame_clock_get_refresh_info")
	reg(&gdkFrameClockGetFrameTime, gtkLib, "gdk_frame_clock_get_frame_time")
	cbPerfTick = newCallback(perfTickCallback)
	cbPerfLayout = newCallback(perfLayoutCallback)
	cbPerfPainted = newCallback(perfPaintedCallback)
}

// hud is a window's performance panel.
type hud struct {
	monitor  *engine.PerfMonitor
	sampler  perfhud.Sampler
	snapshot engine.PerfSnapshot
	text     string
	// clock is the frame clock connected to; laidOut when its frame's
	// layout began.
	clock   uintptr
	laidOut time.Time
	done    chan struct{}
}

// startPerf installs w's monitor and shows its panel.
func (w *window) startPerf() *engine.PerfMonitor {
	bindPerf()
	h := &hud{monitor: engine.NewPerfMonitor(), done: make(chan struct{})}
	w.hud = h
	gtkWidgetAddTickCallback(w.surface, cbPerfTick, w.handle, 0)
	go func() {
		ticker := time.NewTicker(engine.PerfInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				h.monitor.SetProcess(h.sampler.Sample())
				w.post(w.updatePerf)
			case <-h.done:
				return
			}
		}
	}()
	return h.monitor
}

// stopPerf stops sampling as the window closes.
func (w *window) stopPerf() {
	if w.hud != nil {
		close(w.hud.done)
	}
}

// measuring starts timing main-thread work when a monitor measures, and
// returns what records it; dirty says the work changes what shows.
func (w *window) measuring(dirty bool) func() {
	if w.hud == nil {
		return func() {}
	}
	start := time.Now()
	return func() { w.hud.monitor.RecordWork(time.Since(start), dirty) }
}

// updatePerf shows the latest readings.
func (w *window) updatePerf() {
	h := w.hud
	h.snapshot = h.monitor.Snapshot()
	h.text = h.snapshot.Readings()
	gtkWidgetQueueDraw(w.surface)
}

// perfTickCallback closes a frame at each refresh of the surface's frame
// clock, and connects to the clock's phases once.
func perfTickCallback(widget, clock, data uintptr) int32 {
	w := windowOf(data)
	if w == nil || w.closing || w.hud == nil {
		return 0
	}
	h := w.hud
	if h.clock != clock {
		h.clock = clock
		gSignalConnect(clock, "layout", cbPerfLayout, w.handle)
		gSignalConnect(clock, "after-paint", cbPerfPainted, w.handle)
	}
	var interval, presentation int64
	gdkFrameClockGetRefreshInfo(clock, gdkFrameClockGetFrameTime(clock), &interval, &presentation)
	h.monitor.Vsync(time.Now(), time.Duration(interval)*time.Microsecond)
	return 1
}

func perfLayoutCallback(clock, data uintptr) {
	if w := windowOf(data); w != nil && w.hud != nil {
		w.hud.laidOut = time.Now()
	}
}

func perfPaintedCallback(clock, data uintptr) {
	if w := windowOf(data); w != nil && w.hud != nil && !w.hud.laidOut.IsZero() {
		w.hud.monitor.RecordWork(time.Since(w.hud.laidOut), true)
		w.hud.laidOut = time.Time{}
	}
}

// snapshotPerf draws the panel over the surface's width, in one Cairo
// node, laid out with the text as that node draws it: the system's
// monospaced face may be wider than macOS's.
func (w *window) snapshotPerf(snap uintptr, width float32) {
	h := w.hud
	if h == nil || h.text == "" {
		return
	}
	const pw, m, pad, graph = engine.PerfWidth, engine.PerfMargin, engine.PerfPadding, engine.PerfGraphHeight
	area := grapheneRect{m, m, width - 2*m, 400}
	if area.w <= 0 {
		return
	}
	cr := gtkSnapshotAppendCairo(snap, &area)
	layout := perfLayout(cr, h.text)
	var tw, th int32
	pangoLayoutGetPixelSize(layout, &tw, &th)
	panelW := min(max(pw, float64(tw)+2*pad), float64(area.w))
	x, y := float64(width)-m-panelW, float64(m)
	setColor(cr, engine.PerfPanelColor)
	roundRect(cr, rect{x: x, y: y, w: panelW, h: 3*pad + graph + float64(th), r: engine.PerfRadius})
	cairoFill(cr)
	for _, b := range engine.PerfTrace(h.snapshot, panelW-2*pad, graph) {
		setColor(cr, b.Color)
		cairoRectangle(cr, x+pad+b.X, y+pad+b.Y, b.W, b.H)
		cairoFill(cr)
	}
	cairoMoveTo(cr, x+pad, y+2*pad+graph)
	setColor(cr, engine.PerfTextColor)
	pangoCairoShowLayout(cr, layout)
	gObjectUnref(layout)
	cairoDestroy(cr)
}

// perfLayout lays the readings out for cr in a monospaced face.
func perfLayout(cr uintptr, text string) uintptr {
	layout := pangoCairoCreateLayout(cr)
	font := pangoFontDescriptionFromString(cstr("Monospace"))
	const pangoScale = 1024
	pangoFontDescriptionSetSize(font, int32(engine.PerfFontSize*pangoScale))
	pangoLayoutSetFontDescription(layout, font)
	pangoFontDescriptionFree(font)
	pangoLayoutSetText(layout, cstr(text), -1)
	return layout
}

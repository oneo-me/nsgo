//go:build windows

package windows

import (
	"strings"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/engine"
	"github.com/nsgo-dev/nsgo/internal/platform/perfhud"
)

// The performance monitor's panel (`nsgo dev` with fps on), as macOS's:
// the readings (engine.PerfSnapshot.Readings) and the trace of the last
// frames' main-thread cost, drawn by each frame over the tree at the top
// right of the surface and drawn again twice a second. While it measures
// the frame clock ticks at every composition and closes the frame
// (Vsync) at the rate DWM composes; the main thread's work is the
// batches applied, the ticks and the frames drawn and presented.

var dwmGetCompositionTimingInfo = dwmapi.NewProc("DwmGetCompositionTimingInfo")

// hud is a window's performance panel.
type hud struct {
	monitor  *engine.PerfMonitor
	sampler  perfhud.Sampler
	snapshot engine.PerfSnapshot
	text     []uint16
	lines    int
	format   *comObject
	done     chan struct{}
	// shown is where the last frame drew it, in the window's points.
	shown rectF
}

// startPerf installs w's monitor and shows its panel.
func (w *window) startPerf() *engine.PerfMonitor {
	h := &hud{monitor: engine.NewPerfMonitor(), done: make(chan struct{}),
		format: textFormat(driver.Font{Family: "Consolas", Size: engine.PerfFontSize, Weight: driver.FontWeightNormal})}
	w.hud = h
	w.startClock()
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
	if h := w.hud; h != nil {
		close(h.done)
		h.format.release()
		w.hud = nil
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

// vsync closes the monitor's frame at a composition.
func (w *window) vsync() {
	if w.hud == nil {
		return
	}
	// DWM_TIMING_INFO, packed: cbSize, then rateRefresh's numerator and
	// denominator.
	var info [292]byte
	*(*uint32)(unsafe.Pointer(&info[0])) = uint32(len(info))
	var interval time.Duration
	if hr, _, _ := dwmGetCompositionTimingInfo.Call(0, uintptr(unsafe.Pointer(&info[0]))); int32(hr) >= 0 {
		num, den := *(*uint32)(unsafe.Pointer(&info[4])), *(*uint32)(unsafe.Pointer(&info[8]))
		if num > 0 {
			interval = time.Duration(float64(time.Second) * float64(den) / float64(num))
		}
	}
	w.hud.monitor.Vsync(time.Now(), interval)
}

// updatePerf shows the latest readings: the next frame draws the panel
// where it was and where it goes.
func (w *window) updatePerf() {
	h := w.hud
	if h == nil {
		return
	}
	h.snapshot = h.monitor.Snapshot()
	readings := h.snapshot.Readings()
	h.text = utf16.Encode([]rune(readings))
	h.lines = strings.Count(readings, "\n") + 1
	if !h.shown.empty() {
		w.damage.old = append(w.damage.old, h.shown)
	}
	if area := w.perfArea(); !area.empty() {
		w.damage.old = append(w.damage.old, area)
	}
	w.redrawPending()
}

// perfArea is where the panel goes, in the window's points.
func (w *window) perfArea() rectF {
	h := w.hud
	if h == nil || len(h.text) == 0 {
		return rectF{}
	}
	var client rect
	getClientRect.Call(w.hwnd, uintptr(unsafe.Pointer(&client)))
	width := float32(float64(client.Right) / w.scale)
	const pw, m, pad, graph = engine.PerfWidth, engine.PerfMargin, engine.PerfPadding, engine.PerfGraphHeight
	panelW := min(float32(pw), width-2*m)
	textH := float32(h.lines) * engine.PerfFontSize * 1.3
	return rectF{width - m - panelW, m, width - m, m + 3*pad + graph + textH}
}

// drawPerf draws the panel over what the frame drew.
func (p *painter) drawPerf() {
	h := p.w.hud
	area := p.w.perfArea()
	if h == nil || area.empty() {
		return
	}
	h.shown = area
	const pad, graph = engine.PerfPadding, engine.PerfGraphHeight
	p.setTransform(identity)
	p.fillRounded(area, engine.PerfRadius, engine.PerfPanelColor)
	for _, b := range engine.PerfTrace(h.snapshot, float64(area.right-area.left-2*pad), graph) {
		x, y := area.left+pad+float32(b.X), area.top+pad+float32(b.Y)
		p.fillRect(rectF{x, y, x + float32(b.W), y + float32(b.H)}, b.Color)
	}
	if h.format == nil {
		return
	}
	r := rectF{area.left + pad, area.top + 2*pad + graph, area.right - pad, area.bottom}
	p.dc.call(rtDrawText, uintptr(unsafe.Pointer(&h.text[0])), uintptr(len(h.text)), h.format.ptr(),
		uintptr(unsafe.Pointer(&r)), p.solid(engine.PerfTextColor).ptr(), 0, 0)
}

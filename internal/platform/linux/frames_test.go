//go:build linux && !android

package linux

import (
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// An app that scrolls a list a row further every display frame, as the
// benchmark's does, wakes the engine once a frame: the scroll each batch
// reports rides with the next frame rather than waking it for a pass of
// its own.
func TestAListScrolledEveryFrameWakesTheEngineOnceAFrame(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-native-" + t.Name()
	var frames atomic.Int64
	app := func(cx *core.Scope) core.Node {
		list := &core.Ref{}
		top := 0
		var frame func(time.Time)
		frame = func(time.Time) {
			frames.Add(1)
			top = (top + 1) % 900
			list.ScrollToIndex(top, false)
			cx.RequestAnimationFrame(frame)
		}
		cx.RequestAnimationFrame(frame)
		return core.List{Ref: list, Count: func() int { return 1000 }, EstimatedSize: 20,
			Style: core.Style{Width: driver.Pt(200), Height: driver.Pt(200)},
			Row: func(cx *core.Scope, r core.ListRow) core.Node {
				return core.Text{ContentFunc: func() string { return strconv.Itoa(r.Index()) }, Style: core.Style{Height: driver.Pt(20)}}
			}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 200, Height: 200, Perf: true})
	time.Sleep(500 * time.Millisecond)
	from := frames.Load()
	time.Sleep(time.Second)
	n, passes := frames.Load()-from, w.hud.monitor.Snapshot().Passes
	if n < 20 || passes > float32(n+n/10+2) {
		t.Errorf("%d frames took %.0f engine passes, want about one a frame", n, passes)
	}
}

//go:build linux && !android

package linux

import "github.com/oneo-me/nsgo/internal/driver"

// snapshotEdges draws scroll view v's edge effects over its children.
// GTK draws nothing of what is behind a widget, so the content fades into
// the background it scrolls over instead of blurring: toward a soft
// edge's side, and evenly under a hard edge's bars, with the hairline.
func snapshotEdges(v *view, snap uintptr) {
	for side := range v.scroll.Edges {
		e := &v.scroll.Edges[side]
		band := e.Band()
		if band <= 0 {
			continue
		}
		r := driver.BandRect(driver.Side(side), v.frame.Size, band)
		x, y, w, h := float32(r.Origin.X), float32(r.Origin.Y), float32(r.Size.Width), float32(r.Size.Height)
		box := grapheneRect{x, y, w, h}
		if e.Drawn() == driver.EdgeEffectSoft {
			x0, y0, x1, y1 := driver.EdgeLine(driver.Side(side))
			start, end := graphenePoint{x + x0*w, y + y0*h}, graphenePoint{x + x1*w, y + y1*h}
			stops := []gskColorStop{{0, rgba(e.Tint.Faded(driver.UnblurredSoftTint))}, {1, rgba(e.Tint.Faded(0))}}
			gtkSnapshotAppendLinear(snap, &box, &start, &end, &stops[0], uintptr(len(stops)))
			continue
		}
		fill := rgba(e.Tint.Faded(driver.UnblurredHardTint))
		gtkSnapshotAppendColor(snap, &fill, &box)
		l := driver.HairlineRect(driver.Side(side), r.Size)
		line := grapheneRect{x + float32(l.Origin.X), y + float32(l.Origin.Y), float32(l.Size.Width), float32(l.Size.Height)}
		color := rgba(e.Line)
		gtkSnapshotAppendColor(snap, &color, &line)
	}
}

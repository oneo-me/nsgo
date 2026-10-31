//go:build js && wasm

package web

import (
	"fmt"
	"strings"
	"syscall/js"

	"github.com/oneo-me/nsgo/internal/driver"
)

// A scroll view's edge effect at a side is a band over the content that
// sticks where it is as the browser scrolls (position: sticky, offset
// from the top left, since the content beside it is absolutely placed).
// A soft band stacks backdrop blurs that double toward the side, each
// fading in along the band (driver.SoftEdgeLevels), each blurring what
// those before it show, under the tint fading out; a hard band blurs
// evenly under the tint, with the hairline past it.

// edgeDirections are the CSS gradient directions across a band from each
// side.
var edgeDirections = [4]string{"to bottom", "to left", "to top", "to right"}

// placeEdges shows the scroll view's edge effects at its size.
func (s *surface) placeEdges(v *view) {
	sc := v.sc
	inner := driver.Size{Width: max(v.size.Width-v.border.Left-v.border.Right, 0),
		Height: max(v.size.Height-v.border.Top-v.border.Bottom, 0)}
	for side := range sc.edges {
		e := &sc.config.Edges[side]
		band := e.Band()
		el := &sc.edges[side]
		if band <= 0 || inner.Width <= 0 || inner.Height <= 0 {
			if el.Truthy() {
				el.Call("remove")
				*el = js.Undefined()
			}
			continue
		}
		if !el.Truthy() {
			*el = s.doc.Call("createElement", "div")
			el.Call("setAttribute", "aria-hidden", "true")
			// After the content, which it shows over without a z-index
			// of its own, so views after the scroll view show over it.
			v.el.Call("appendChild", *el)
		}
		r := driver.BandRect(driver.Side(side), inner, band)
		setStyle(*el, map[string]string{"position": "sticky", "left": px(r.Origin.X), "top": px(r.Origin.Y),
			"width": px(r.Size.Width), "height": px(r.Size.Height), "margin-bottom": px(-r.Size.Height),
			"pointer-events": "none"})
		el.Set("innerHTML", edgeHTML(e, driver.Side(side), r.Size))
	}
}

// edgeHTML is the layers of edge e's band at side, of size size.
func edgeHTML(e *driver.ScrollEdge, side driver.Side, size driver.Size) string {
	dir := edgeDirections[side]
	layer := func(style string) string {
		return `<div style="position:absolute;left:0;top:0;width:100%;height:100%;` + style + `"></div>`
	}
	var b strings.Builder
	if e.Drawn() == driver.EdgeEffectSoft {
		for _, l := range driver.SoftEdgeLevels() {
			mask := fmt.Sprintf("linear-gradient(%s, #000 %g%%, transparent %g%%)", dir, l.Full*100, l.Gone*100)
			b.WriteString(layer(fmt.Sprintf("backdrop-filter:blur(%gpx);-webkit-backdrop-filter:blur(%gpx);mask-image:%s;-webkit-mask-image:%[3]s",
				l.Blur, l.Blur, mask)))
		}
		b.WriteString(layer(fmt.Sprintf("background:linear-gradient(%s, %s, %s)", dir,
			cssColor(e.Tint.Faded(driver.SoftEdgeTint)), cssColor(e.Tint.Faded(0)))))
		return b.String()
	}
	b.WriteString(layer(fmt.Sprintf("backdrop-filter:blur(%gpx);-webkit-backdrop-filter:blur(%[1]gpx);background:%s",
		driver.HardEdgeBlur, cssColor(e.Tint.Faded(driver.HardEdgeTint)))))
	line := driver.HairlineRect(side, size)
	b.WriteString(fmt.Sprintf(`<div style="position:absolute;left:%s;top:%s;width:%s;height:%s;background:%s"></div>`,
		px(line.Origin.X), px(line.Origin.Y), px(line.Size.Width), px(line.Size.Height), cssColor(e.Line)))
	return b.String()
}

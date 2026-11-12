// Package lucide provides Lucide icons for NSGo: one accessor per icon,
// each an SVG that core.Icon draws in the text color it inherits.
//
//	core.Icon{SVG: lucide.Search, Label: "Search"}
//
// Icons keep Lucide's original strokes and scale with the icon; variable
// stroke width and absolute stroke widths are not provided.
//
// Each accessor parses its SVG on first use and returns the same shared
// SVG afterwards. There is no runtime registry: only tests reference the
// whole collection. Names follow upstream kebab-case converted to
// PascalCase (arrow-up-right becomes ArrowUpRight); a name starting with a
// digit takes the Icon prefix. Upstream aliases and lab icons are not
// included.
//
// The generated assets are pinned in upstream.json and covered by the
// adjacent Lucide and Feather LICENSE, not by NSGo's Apache-2.0 license.
package lucide

import (
	"sync"

	"github.com/nsgo-dev/nsgo/core"
)

// Zero-value caches avoid package initializers rooting unused SVG resources.
type lazyIcon struct {
	once sync.Once
	svg  *core.SVG
}

func (i *lazyIcon) get(data string) *core.SVG {
	i.once.Do(func() { i.svg = core.MustParseSVG([]byte(data)) })
	return i.svg
}

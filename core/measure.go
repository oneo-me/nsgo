package core

import (
	"hash/maphash"
	"math"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// measureCache is what texts measured, per engine, as NSH's MeasureCache:
// sizes keyed by a hash of what lays a text out (its colors left out) and
// the width it wraps at, so identical texts, such as a list's rows or a
// value that comes back, measure once. It keeps two generations of at
// most measureCacheSize entries: a full one becomes the old one, and an
// old entry used again moves back.
type measureCache struct {
	seed     maphash.Seed
	now, old map[measureKey]Size
}

const measureCacheSize = 1024

// measureKey is a text's layout hash and a width; Auto is a NaN, which no
// map can key on, so it has a flag of its own.
type measureKey struct {
	text  uint64
	width float32
	auto  bool
}

func newMeasureCache() measureCache {
	return measureCache{seed: maphash.MakeSeed(), now: map[measureKey]Size{}, old: map[measureKey]Size{}}
}

// size returns what p, prepared from a text of layout hash text, measures
// wrapped at width.
func (c *measureCache) size(text uint64, p driver.PreparedText, width Points) Size {
	k := measureKey{text: text}
	if width.IsAuto() {
		k.auto = true
	} else {
		k.width = float32(width)
	}
	if s, ok := c.now[k]; ok {
		return s
	}
	s, ok := c.old[k]
	if !ok {
		s = p.Measure(width)
	}
	c.put(k, s)
	return s
}

func (c *measureCache) put(k measureKey, s Size) {
	if len(c.now) >= measureCacheSize {
		c.old, c.now = c.now, c.old
		clear(c.now)
	}
	c.now[k] = s
}

// word returns how wide word is in style, unwrapped, preparing it only
// when the cache has not measured it.
func (c *measureCache) word(engine driver.TextEngine, word string, style driver.TextStyle) Points {
	at := driver.AttributedText{Text: word, Style: style}
	h := c.hash(&at)
	k := measureKey{text: h, auto: true}
	if s, ok := c.now[k]; ok {
		return s.Width
	}
	s, ok := c.old[k]
	if !ok {
		p := engine.Prepare(&at)
		s = p.Measure(Auto)
		p.Release()
	}
	c.put(k, s)
	return s.Width
}

// hash is the layout hash of at: what measuring it depends on, which is
// all of it but its colors and shadow.
func (c *measureCache) hash(at *driver.AttributedText) uint64 {
	var h maphash.Hash
	h.SetSeed(c.seed)
	h.WriteString(at.Text)
	writeStyle(&h, at.Style)
	var b [8]byte
	word := func(v uint64) {
		for i := range b {
			b[i] = byte(v >> (8 * i))
		}
		h.Write(b[:])
	}
	word(uint64(at.MaxLines))
	word(uint64(at.Ellipsize))
	for _, sp := range at.Spans {
		word(uint64(sp.Start))
		word(uint64(sp.End))
		writeStyle(&h, sp.Style)
	}
	for _, a := range at.Attachments {
		word(uint64(a.Offset))
		word(uint64(math.Float32bits(float32(a.Size.Width)))<<32 | uint64(math.Float32bits(float32(a.Size.Height))))
		word(uint64(a.Align))
	}
	return h.Sum64()
}

// writeStyle writes what of s lays text out: all of it but its colors.
func writeStyle(h *maphash.Hash, s driver.TextStyle) {
	s.Color, s.DecorationColor = 0, 0
	maphash.WriteComparable(h, s)
}

package woff

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/andybalholm/brotli"
)

// knownTags are the tags a WOFF2 directory names by index.
var knownTags = [63]string{
	"cmap", "head", "hhea", "hmtx", "maxp", "name", "OS/2", "post", "cvt ", "fpgm", "glyf", "loca", "prep", "CFF ",
	"VORG", "EBDT", "EBLC", "gasp", "hdmx", "kern", "LTSH", "PCLT", "VDMX", "vhea", "vmtx", "BASE", "GDEF", "GPOS",
	"GSUB", "EBSC", "JSTF", "MATH", "CBDT", "CBLC", "COLR", "CPAL", "SVG ", "sbix", "acnt", "avar", "bdat", "bloc",
	"bsln", "cvar", "fdsc", "feat", "fmtx", "fvar", "gvar", "hsty", "just", "lcar", "mort", "morx", "opbd", "prop",
	"trak", "Zapf", "Silf", "Glat", "Gloc", "Feat", "Sill",
}

// reader reads a stream of a WOFF2 file, its first failure kept.
type reader struct {
	data []byte
	err  error
}

func (r *reader) take(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || n > len(r.data) {
		r.err = io.ErrUnexpectedEOF
		return nil
	}
	b := r.data[:n]
	r.data = r.data[n:]
	return b
}

func (r *reader) u8() byte {
	if b := r.take(1); b != nil {
		return b[0]
	}
	return 0
}

func (r *reader) u16() uint16 {
	if b := r.take(2); b != nil {
		return binary.BigEndian.Uint16(b)
	}
	return 0
}

func (r *reader) u32() uint32 {
	if b := r.take(4); b != nil {
		return binary.BigEndian.Uint32(b)
	}
	return 0
}

// base128 is a UIntBase128: seven bits a byte, high bits first, in at
// most five bytes with no leading zeros.
func (r *reader) base128() uint32 {
	var v uint32
	for i := range 5 {
		b := r.u8()
		if r.err != nil {
			return 0
		}
		if i == 0 && b == 0x80 || v&0xFE000000 != 0 {
			r.err = errors.New("a bad UIntBase128")
			return 0
		}
		v = v<<7 | uint32(b&0x7F)
		if b&0x80 == 0 {
			return v
		}
	}
	r.err = errors.New("a UIntBase128 too long")
	return 0
}

// u255 is a 255UInt16.
func (r *reader) u255() int {
	const oneMore, twoMore, word, lowest = 255, 254, 253, 253
	switch b := int(r.u8()); b {
	case word:
		return int(r.u16())
	case oneMore:
		return int(r.u8()) + lowest
	case twoMore:
		return int(r.u8()) + lowest*2
	default:
		return b
	}
}

// entry is a table of a WOFF2 directory.
type entry struct {
	tag         uint32
	transformed bool
	length      int
	stored      int
}

// unpack2 is WOFF2: one Brotli stream of every table, glyf and loca, and
// perhaps hmtx, transformed into a form that compresses better.
func unpack2(data []byte) ([]byte, error) {
	r := &reader{data: data}
	r.take(4)
	flavor := r.u32()
	r.u32() // length
	n := int(r.u16())
	r.u16() // reserved
	sfntSize, compressed := r.u32(), int(r.u32())
	r.take(24) // version, metadata and private data
	if r.err != nil {
		return nil, errors.New("woff2: the header is cut short")
	}
	if n == 0 {
		return nil, errors.New("woff2: no tables")
	}
	if flavor == tagOf("ttcf") {
		return nil, errors.New("woff2: font collections are not supported")
	}
	if sfntSize > maxTables {
		return nil, fmt.Errorf("woff2: the tables declare more than a font may hold (%d bytes)", maxTables)
	}
	entries := make([]entry, n)
	total := 0
	for i := range entries {
		e := &entries[i]
		flags := r.u8()
		if known := flags & 63; known == 63 {
			e.tag = r.u32()
		} else {
			e.tag = tagOf(knownTags[known])
		}
		version := flags >> 6
		e.transformed = version != 0
		if e.tag == tagGlyf || e.tag == tagLoca {
			// Their version 0 is the transform, and 3 none.
			e.transformed = version != 3
		}
		e.length = int(r.base128())
		e.stored = e.length
		if e.transformed {
			e.stored = int(r.base128())
		}
		if r.err != nil {
			return nil, fmt.Errorf("woff2: the table directory: %w", r.err)
		}
		if total += e.stored; total > maxTables || e.length > maxTables {
			return nil, fmt.Errorf("woff2: the tables declare more than a font may hold (%d bytes)", maxTables)
		}
	}
	stream := r.take(compressed)
	if r.err != nil {
		return nil, errors.New("woff2: the compressed data is cut short")
	}
	raw, err := io.ReadAll(io.LimitReader(brotli.NewReader(bytes.NewReader(stream)), int64(total)+1))
	if err != nil {
		return nil, fmt.Errorf("woff2: %w", err)
	}
	if len(raw) != total {
		return nil, fmt.Errorf("woff2: the tables hold %d bytes, not the %d they declare", len(raw), total)
	}
	stored := make(map[uint32][]byte, n)
	at := 0
	for _, e := range entries {
		stored[e.tag] = raw[at : at+e.stored]
		at += e.stored
	}
	tables := make([]table, 0, n)
	var lsb []int16 // the glyphs' left side bearings, of a rebuilt glyf
	for _, e := range entries {
		if e.transformed && e.tag == tagGlyf {
			glyf, loca, xMins, err := rebuildGlyphs(stored[e.tag])
			if err != nil {
				return nil, fmt.Errorf("woff2: glyf: %w", err)
			}
			lsb = xMins
			tables = append(tables, table{tagGlyf, glyf}, table{tagLoca, loca})
		}
	}
	for _, e := range entries {
		switch {
		case e.tag == tagGlyf && e.transformed, e.tag == tagLoca && e.transformed:
		case e.tag == tagHmtx && e.transformed:
			hmtx, err := rebuildMetrics(stored[e.tag], stored[tagHhea], stored[tagMaxp], lsb)
			if err != nil {
				return nil, fmt.Errorf("woff2: hmtx: %w", err)
			}
			tables = append(tables, table{tagHmtx, hmtx})
		case e.transformed:
			return nil, fmt.Errorf("woff2: %q has a transform the format does not define", tagName(e.tag))
		default:
			tables = append(tables, table{e.tag, stored[e.tag]})
		}
	}
	return assemble(flavor, tables)
}

// rebuildMetrics is the transformed hmtx: the advances, and the bearings
// the file kept, the others being the glyphs' xMin.
func rebuildMetrics(data, hhea, maxp []byte, xMins []int16) ([]byte, error) {
	if len(hhea) < 36 || len(maxp) < 6 {
		return nil, errors.New("hhea or maxp is cut short")
	}
	metrics, glyphs := int(binary.BigEndian.Uint16(hhea[34:])), int(binary.BigEndian.Uint16(maxp[4:]))
	if metrics == 0 || metrics > glyphs || len(xMins) != glyphs {
		return nil, errors.New("the metrics do not match the glyphs")
	}
	r := &reader{data: data}
	flags := r.u8()
	if flags == 0 || flags&^3 != 0 {
		return nil, fmt.Errorf("bad flags %#x", flags)
	}
	advances := r.take(metrics * 2)
	var proportional, monospaced []byte
	if flags&1 == 0 {
		proportional = r.take(metrics * 2)
	}
	if flags&2 == 0 {
		monospaced = r.take((glyphs - metrics) * 2)
	}
	if r.err != nil {
		return nil, r.err
	}
	out := make([]byte, 0, metrics*4+(glyphs-metrics)*2)
	for i := range metrics {
		out = append(out, advances[i*2], advances[i*2+1])
		if proportional != nil {
			out = append(out, proportional[i*2], proportional[i*2+1])
		} else {
			out = binary.BigEndian.AppendUint16(out, uint16(xMins[i]))
		}
	}
	for i := range glyphs - metrics {
		if monospaced != nil {
			out = append(out, monospaced[i*2], monospaced[i*2+1])
		} else {
			out = binary.BigEndian.AppendUint16(out, uint16(xMins[metrics+i]))
		}
	}
	return out, nil
}

// A composite glyph's flags.
const (
	argsAreWords     = 1 << 0
	haveScale        = 1 << 3
	moreComponents   = 1 << 5
	haveXYScale      = 1 << 6
	haveTwoByTwo     = 1 << 7
	haveInstructions = 1 << 8
)

// rebuildGlyphs is the transformed glyf: seven streams of the glyphs'
// parts, from which it writes glyf and loca, and each glyph's xMin.
func rebuildGlyphs(data []byte) (glyf, loca []byte, xMins []int16, err error) {
	r := &reader{data: data}
	r.u16() // reserved
	options := r.u16()
	glyphs, format := int(r.u16()), r.u16()
	sizes := make([]int, 7)
	for i := range sizes {
		sizes[i] = int(r.u32())
	}
	streams := make([]reader, 7)
	for i := range streams {
		streams[i].data = r.take(sizes[i])
	}
	if r.err != nil {
		return nil, nil, nil, r.err
	}
	var overlaps []byte
	if options&1 != 0 {
		overlaps = r.take((glyphs + 7) / 8)
		if r.err != nil {
			return nil, nil, nil, r.err
		}
	}
	contours, points, flagBytes, glyphBytes, composites, boxes, instructions :=
		&streams[0], &streams[1], &streams[2], &streams[3], &streams[4], &streams[5], &streams[6]
	explicit := boxes.take((glyphs + 7) / 8)
	if boxes.err != nil {
		return nil, nil, nil, boxes.err
	}
	has := func(bits []byte, i int) bool { return bits[i>>3]&(0x80>>(i&7)) != 0 }

	offsets := make([]int, glyphs+1)
	xMins = make([]int16, glyphs)
	be := binary.BigEndian
	for i := range glyphs {
		offsets[i] = len(glyf)
		n := contours.u16()
		if contours.err != nil {
			return nil, nil, nil, contours.err
		}
		start := len(glyf)
		switch n {
		case 0:
			if has(explicit, i) {
				return nil, nil, nil, fmt.Errorf("glyph %d: an empty glyph with a box", i)
			}
			continue
		case 0xFFFF:
			if !has(explicit, i) {
				return nil, nil, nil, fmt.Errorf("glyph %d: a composite glyph with no box", i)
			}
			glyf = be.AppendUint16(glyf, n)
			glyf = append(glyf, boxes.take(8)...)
			parts := composites.data
			used, instructed := 0, false
			for more := true; more; {
				flags := composites.u16()
				composites.u16() // the glyph
				size := 2
				if flags&argsAreWords != 0 {
					size = 4
				}
				switch {
				case flags&haveScale != 0:
					size += 2
				case flags&haveXYScale != 0:
					size += 4
				case flags&haveTwoByTwo != 0:
					size += 8
				}
				composites.take(size)
				if composites.err != nil {
					return nil, nil, nil, composites.err
				}
				used += 4 + size
				instructed = instructed || flags&haveInstructions != 0
				more = flags&moreComponents != 0
			}
			glyf = append(glyf, parts[:used]...)
			if instructed {
				length := glyphBytes.u255()
				glyf = be.AppendUint16(glyf, uint16(length))
				glyf = append(glyf, instructions.take(length)...)
			}
		default:
			if n > 0x7FFF {
				return nil, nil, nil, fmt.Errorf("glyph %d: %d contours", i, n)
			}
			ends, total := make([]uint16, n), 0
			for c := range ends {
				count := points.u255()
				if points.err != nil {
					return nil, nil, nil, points.err
				}
				total += count
				if total == 0 || total > 0xFFFF+1 {
					return nil, nil, nil, fmt.Errorf("glyph %d: %d points", i, total)
				}
				ends[c] = uint16(total - 1)
			}
			off, xs, ys, xMin, yMin, xMax, yMax, err := readPoints(flagBytes, glyphBytes, total)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("glyph %d: %w", i, err)
			}
			glyf = be.AppendUint16(glyf, n)
			if has(explicit, i) {
				glyf = append(glyf, boxes.take(8)...)
			} else {
				for _, v := range [4]int{xMin, yMin, xMax, yMax} {
					glyf = be.AppendUint16(glyf, uint16(int16(v)))
				}
			}
			for _, e := range ends {
				glyf = be.AppendUint16(glyf, e)
			}
			length := glyphBytes.u255()
			glyf = be.AppendUint16(glyf, uint16(length))
			glyf = append(glyf, instructions.take(length)...)
			glyf = appendPoints(glyf, off, xs, ys, overlaps != nil && has(overlaps, i))
		}
		for _, s := range streams {
			if s.err != nil {
				return nil, nil, nil, fmt.Errorf("glyph %d: %w", i, s.err)
			}
		}
		xMins[i] = int16(be.Uint16(glyf[start+2:]))
		for len(glyf)%4 != 0 {
			glyf = append(glyf, 0)
		}
	}
	offsets[glyphs] = len(glyf)
	if format == 0 {
		if len(glyf) > 0xFFFF*2 {
			return nil, nil, nil, errors.New("the glyphs do not fit a short loca")
		}
		for _, o := range offsets {
			loca = be.AppendUint16(loca, uint16(o/2))
		}
	} else {
		for _, o := range offsets {
			loca = be.AppendUint32(loca, uint32(o))
		}
	}
	return glyf, loca, xMins, nil
}

// readPoints reads n points of a simple glyph: a flag each, whose top bit
// says the point is off the curve and whose rest says how many bytes of
// glyphs hold its deltas and how to read them.
func readPoints(flags, glyphs *reader, n int) (off []bool, xs, ys []int, xMin, yMin, xMax, yMax int, err error) {
	off, xs, ys = make([]bool, n), make([]int, n), make([]int, n)
	signed := func(flag, v int) int {
		if flag&1 != 0 {
			return v
		}
		return -v
	}
	x, y := 0, 0
	for i := range n {
		flag := int(flags.u8())
		off[i] = flag&0x80 != 0
		flag &= 0x7F
		var dx, dy int
		switch {
		case flag < 10:
			dy = signed(flag, (flag&14)<<7+int(glyphs.u8()))
		case flag < 20:
			dx = signed(flag, ((flag-10)&14)<<7+int(glyphs.u8()))
		case flag < 84:
			b0, b1 := flag-20, int(glyphs.u8())
			dx = signed(flag, 1+b0&0x30+b1>>4)
			dy = signed(flag>>1, 1+(b0&0x0C)<<2+b1&0x0F)
		case flag < 120:
			b0, b1, b2 := flag-84, int(glyphs.u8()), int(glyphs.u8())
			dx = signed(flag, 1+(b0/12)<<8+b1)
			dy = signed(flag>>1, 1+((b0%12)>>2)<<8+b2)
		case flag < 124:
			b1, b2, b3 := int(glyphs.u8()), int(glyphs.u8()), int(glyphs.u8())
			dx = signed(flag, b1<<4+b2>>4)
			dy = signed(flag>>1, (b2&0x0F)<<8+b3)
		default:
			b1, b2, b3, b4 := int(glyphs.u8()), int(glyphs.u8()), int(glyphs.u8()), int(glyphs.u8())
			dx = signed(flag, b1<<8+b2)
			dy = signed(flag>>1, b3<<8+b4)
		}
		if flags.err != nil || glyphs.err != nil {
			return nil, nil, nil, 0, 0, 0, 0, errors.New("the points are cut short")
		}
		x, y = x+dx, y+dy
		xs[i], ys[i] = x, y
		if i == 0 {
			xMin, yMin, xMax, yMax = x, y, x, y
		}
		xMin, yMin, xMax, yMax = min(xMin, x), min(yMin, y), max(xMax, x), max(yMax, y)
	}
	return off, xs, ys, xMin, yMin, xMax, yMax, nil
}

// A simple glyph's point flags.
const (
	onCurve    = 1 << 0
	xShort     = 1 << 1
	yShort     = 1 << 2
	repeatFlag = 1 << 3
	xSame      = 1 << 4
	ySame      = 1 << 5
	overlap    = 1 << 6
)

// appendPoints writes a simple glyph's points as glyf holds them, as
// deltas in the fewest bytes with runs of a flag repeated.
func appendPoints(out []byte, off []bool, xs, ys []int, overlapping bool) []byte {
	var flags, xBytes, yBytes []byte
	delta := func(bytes []byte, d int, short, same byte) (byte, []byte) {
		switch {
		case d == 0:
			return same, bytes
		case d > -256 && d < 256:
			if d > 0 {
				return short | same, append(bytes, byte(d))
			}
			return short, append(bytes, byte(-d))
		default:
			return 0, binary.BigEndian.AppendUint16(bytes, uint16(int16(d)))
		}
	}
	x, y := 0, 0
	var last, repeats byte
	for i := range off {
		var flag byte
		if !off[i] {
			flag = onCurve
		}
		if overlapping && i == 0 {
			flag |= overlap
		}
		var f byte
		f, xBytes = delta(xBytes, xs[i]-x, xShort, xSame)
		flag |= f
		f, yBytes = delta(yBytes, ys[i]-y, yShort, ySame)
		flag |= f
		x, y = xs[i], ys[i]
		switch {
		case i == 0 || flag != last || repeats == 255:
			flags = append(flags, flag)
			last, repeats = flag, 0
		case repeats == 0:
			flags[len(flags)-1] |= repeatFlag
			flags = append(flags, 1)
			repeats = 1
		default:
			repeats++
			flags[len(flags)-1] = repeats
		}
	}
	return append(append(append(out, flags...), xBytes...), yBytes...)
}

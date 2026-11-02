package bundler

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/go-sourcemap/sourcemap"
)

// A frag is generated code as chunks, each copied from the module's
// source or written by the compiler, which keep where they come from for
// the source map.
type frag []chunk

type chunk struct {
	text string
	// src is where text comes from in the module's source, -1 for
	// nowhere; copied text maps each of its lines there.
	src    int
	copied bool
}

func (f *frag) gen(s string, at int) { *f = append(*f, chunk{s, at, false}) }
func (f *frag) add(g frag)           { *f = append(*f, g...) }

func (f frag) String() string {
	var b strings.Builder
	for _, c := range f {
		b.WriteString(c.text)
	}
	return b.String()
}

// positions converts offsets of a text into lines and UTF-16 columns,
// as source maps count them.
type positions struct {
	text  string
	lines []int // the offset each line starts at
}

func newPositions(text string) *positions {
	p := &positions{text: text, lines: []int{0}}
	for i := range len(text) {
		if text[i] == '\n' {
			p.lines = append(p.lines, i+1)
		}
	}
	return p
}

// at returns the 0-based line and UTF-16 column of offset off.
func (p *positions) at(off int) (line, col int) {
	lo, hi := 0, len(p.lines)
	for lo+1 < hi {
		mid := (lo + hi) / 2
		if p.lines[mid] <= off {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo, utf16Len(p.text[p.lines[lo]:min(off, len(p.text))])
}

// offset returns the offset of 0-based line and UTF-16 column col.
func (p *positions) offset(line, col int) int {
	if line < 0 || line >= len(p.lines) {
		return len(p.text)
	}
	i := p.lines[line]
	for n := 0; n < col && i < len(p.text) && p.text[i] != '\n'; {
		r, size := utf8.DecodeRuneInString(p.text[i:])
		if r > 0xFFFF {
			n += 2
		} else {
			n++
		}
		i += size
	}
	return i
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r > 0xFFFF {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// mapped writes f as code with a source map of it into the original file
// (path, whose text is original), through prev: the map from the
// module's source to the original.
func (f frag) mapped(src *positions, prev *sourcemap.Consumer, path, original string) (string, string) {
	var code strings.Builder
	var m mappingWriter
	line, col := 0, 0
	mark := func(off int) {
		sl, sc := src.at(off)
		_, _, ol, oc, ok := prev.Source(sl+1, sc)
		if ok {
			m.add(line, col, ol-1, oc)
		}
	}
	for _, c := range f {
		if c.src >= 0 {
			mark(c.src)
		}
		rest := c.text
		for {
			i := strings.IndexByte(rest, '\n')
			if i < 0 {
				col += utf16Len(rest)
				break
			}
			line, col = line+1, 0
			if c.copied {
				off := c.src + len(c.text) - len(rest) + i + 1
				if off < len(src.text) {
					mark(off)
				}
			}
			rest = rest[i+1:]
		}
		code.WriteString(c.text)
	}
	doc, _ := json.Marshal(map[string]any{
		"version": 3, "sources": []string{path}, "sourcesContent": []string{original},
		"names": []string{}, "mappings": m.String(),
	})
	return code.String(), string(doc)
}

// inline appends a source map to code as a comment that esbuild and goja
// read.
func inline(code, sourceMap string) string {
	return code + "\n//# sourceMappingURL=data:application/json;base64," +
		base64.StdEncoding.EncodeToString([]byte(sourceMap)) + "\n"
}

// mappingWriter writes a source map's mappings, one source.
type mappingWriter struct {
	b                          strings.Builder
	line, col, srcLine, srcCol int
	started                    bool
}

func (w *mappingWriter) add(line, col, srcLine, srcCol int) {
	if line < w.line || line == w.line && w.started && col <= w.col {
		return
	}
	for w.line < line {
		w.b.WriteByte(';')
		w.line++
		w.col, w.started = 0, false
	}
	if w.started {
		w.b.WriteByte(',')
	}
	vlq(&w.b, col-w.col)
	vlq(&w.b, 0)
	vlq(&w.b, srcLine-w.srcLine)
	vlq(&w.b, srcCol-w.srcCol)
	w.col, w.srcLine, w.srcCol, w.started = col, srcLine, srcCol, true
}

func (w *mappingWriter) String() string { return w.b.String() }

const base64Digits = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

func vlq(b *strings.Builder, n int) {
	v := n << 1
	if n < 0 {
		v = -n<<1 | 1
	}
	for {
		d := v & 31
		v >>= 5
		if v > 0 {
			d |= 32
		}
		b.WriteByte(base64Digits[d])
		if v == 0 {
			return
		}
	}
}

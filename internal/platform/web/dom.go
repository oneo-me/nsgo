//go:build js && wasm

package web

import (
	"encoding/binary"
	"math"
	"syscall/js"
	"unicode/utf8"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Every call into JavaScript costs Go's WebAssembly more than the work it
// asks for, and every object and string it passes or returns a finalizer.
// NSH sends a commit to its page as one array; dom does that for what a
// scroll or a ticking table sends most, frames and plain texts: Mount
// queues them as numbers and bytes and the page applies a run of them in
// one call, in order, before any other item. Measuring a text's width
// passes its bytes and returns a number. Where the page's policy forbids
// making the helpers (a Content-Security-Policy without 'unsafe-eval'),
// each goes through syscall/js as before.

// domJS makes the page's side: elements by view, the queued operations and
// the canvas widths are measured on.
const domJS = `
const els = [], inners = [];
const decoder = new TextDecoder();
let ctx = null, font = "", uses = 0;
const px = (v) => Math.round(v * 1000) / 1000 + "px";
function frame(style, ops, i) {
  const mask = ops[i];
  if (mask & 1) style.left = px(ops[i + 1]);
  if (mask & 2) style.top = px(ops[i + 2]);
  if (mask & 4) style.width = px(ops[i + 3]);
  if (mask & 8) style.height = px(ops[i + 4]);
}
return {
  register(id, el, inner) { els[id] = el; inners[id] = inner; },
  forget(id) { els[id] = undefined; inners[id] = undefined; },
  apply(buf, n, bytes) {
    const ops = new Float64Array(buf.buffer, 0, n);
    for (let i = 0; i < n; ) {
      const op = ops[i], id = ops[i + 1];
      i += 2;
      switch (op) {
        case 1: frame(els[id].style, ops, i); i += 5; break;
        case 2: frame(inners[id].style, ops, i); i += 5; break;
        case 3: inners[id].textContent = decoder.decode(bytes.subarray(ops[i], ops[i + 1])); i += 2; break;
        case 4: inners[id].style.whiteSpace = ops[i] ? "pre" : "pre-wrap"; i += 1; break;
      }
    }
  },
  width(f, bytes, n) {
    // A fresh context drops the strings the last one shaped (NSH's
    // MEASURE_ROUNDS).
    if (ctx === null || ++uses >= 5000) {
      uses = 0;
      ctx = typeof OffscreenCanvas !== "undefined" ? new OffscreenCanvas(1, 1).getContext("2d") : document.createElement("canvas").getContext("2d");
      if (font) ctx.font = font;
    }
    if (f !== null && f !== font) { font = f; ctx.font = f; }
    return ctx.measureText(decoder.decode(bytes.subarray(0, n))).width;
  },
};`

const (
	opFrame      = 1
	opInnerFrame = 2
	opText       = 3
	opWhiteSpace = 4
)

// dom queues frames and texts for the page to apply in one call.
type dom struct {
	ok      bool
	helpers js.Value
	ops     []byte // float64s, little-endian
	text    []byte
	n       int // float64s queued
	// opsJS and textJS are the page's copies, grown as needed.
	opsJS, textJS js.Value
	// measured and measuredJS hold the text width measures.
	measured   []byte
	measuredJS js.Value
	font       string // the font width last passed
}

func newDOM() dom {
	var d dom
	func() {
		defer func() {
			if recover() != nil {
				d.ok = false
			}
		}()
		d.helpers = js.Global().Get("Function").New(domJS).Invoke()
		d.ok = d.helpers.Truthy()
	}()
	return d
}

func (d *dom) num(v float64) {
	d.ops = binary.LittleEndian.AppendUint64(d.ops, math.Float64bits(v))
	d.n++
}

// register tells the page v's elements.
func (d *dom) register(v *view) {
	if d.ok {
		d.helpers.Call("register", int(v.id), v.el, v.inner)
	}
}

func (d *dom) forget(id driver.NodeID) {
	if d.ok {
		d.helpers.Call("forget", int(id))
	}
}

// frame queues el's (op opFrame) or inner's (opInnerFrame) frame f, the
// edges that differ from was unless all.
func (d *dom) frame(op int, id driver.NodeID, f, was driver.Bounds, all bool) {
	mask := 0
	if all || f.Origin.X != was.Origin.X {
		mask |= 1
	}
	if all || f.Origin.Y != was.Origin.Y {
		mask |= 2
	}
	if all || f.Size.Width != was.Size.Width {
		mask |= 4
	}
	if all || f.Size.Height != was.Size.Height {
		mask |= 8
	}
	if mask == 0 {
		return
	}
	d.num(float64(op))
	d.num(float64(id))
	d.num(float64(mask))
	d.num(float64(f.Origin.X))
	d.num(float64(f.Origin.Y))
	d.num(float64(f.Size.Width))
	d.num(float64(f.Size.Height))
}

// setText queues a plain text's content.
func (d *dom) setText(id driver.NodeID, text string) {
	start := len(d.text)
	d.text = append(d.text, text...)
	d.num(opText)
	d.num(float64(id))
	d.num(float64(start))
	d.num(float64(len(d.text)))
}

func (d *dom) whiteSpace(id driver.NodeID, oneLine bool) {
	d.num(opWhiteSpace)
	d.num(float64(id))
	if oneLine {
		d.num(1)
	} else {
		d.num(0)
	}
}

// flush has the page apply what is queued.
func (d *dom) flush() {
	if d.n == 0 {
		return
	}
	d.opsJS = copyTo(d.opsJS, d.ops)
	d.textJS = copyTo(d.textJS, d.text)
	d.helpers.Call("apply", d.opsJS, d.n, d.textJS)
	d.ops, d.text, d.n = d.ops[:0], d.text[:0], 0
}

// copyTo copies b into the page's array a, a larger one when it is too
// small, and returns it.
func copyTo(a js.Value, b []byte) js.Value {
	if a.IsUndefined() || a.Length() < len(b) {
		a = js.Global().Get("Uint8Array").New(max(1024, len(b)*2))
	}
	js.CopyBytesToJS(a, b)
	return a
}

// width is text's width in font on the page's canvas, false where the
// helpers could not be made.
func (d *dom) width(font, text string) (float64, bool) {
	if !d.ok || !utf8.ValidString(text) {
		return 0, false
	}
	d.measured = append(d.measured[:0], text...)
	d.measuredJS = copyTo(d.measuredJS, d.measured)
	f := js.Null()
	if font != d.font {
		d.font = font
		f = js.ValueOf(font)
	}
	return d.helpers.Call("width", f, d.measuredJS, len(text)).Float(), true
}

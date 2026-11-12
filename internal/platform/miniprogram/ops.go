package miniprogram

import (
	"math"
	"strconv"
	"unicode/utf8"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// ops builds a batch's operations for the runtime (nsgo/tree.js): a JSON
// array of arrays, each led by its code and the view's ID. It is written
// by hand, as a batch is built every commit and reflection would weigh on
// the module's size and time.
type ops struct {
	b    []byte
	open bool // an operation is being written
}

// reset empties o for the next batch.
func (o *ops) reset() { o.b, o.open = o.b[:0], false }

// empty reports whether o holds no operation.
func (o *ops) empty() bool { return len(o.b) == 0 }

// bytes returns the batch as JSON.
func (o *ops) bytes() []byte {
	o.end()
	if len(o.b) == 0 {
		return []byte("[]")
	}
	return append(append([]byte{'['}, o.b[1:]...), ']')
}

// op starts operation code on view id; its arguments follow.
func (o *ops) op(code string, id driver.NodeID) *ops {
	o.end()
	o.b = append(o.b, ",[\""...)
	o.b = append(o.b, code...)
	o.b = append(o.b, "\","...)
	o.b = strconv.AppendUint(o.b, uint64(id), 10)
	o.open = true
	return o
}

func (o *ops) end() {
	if o.open {
		o.b = append(o.b, ']')
		o.open = false
	}
}

// num appends a number, rounded to a hundredth: the render layer's
// pixels need no more.
func (o *ops) num(v float64) *ops {
	o.b = append(o.b, ',')
	o.b = appendNum(o.b, v)
	return o
}

func appendNum(b []byte, v float64) []byte {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return append(b, '0')
	}
	return strconv.AppendFloat(b, math.Round(v*100)/100, 'f', -1, 64)
}

func (o *ops) pt(v driver.Points) *ops { return o.num(float64(v)) }

func (o *ops) str(s string) *ops {
	o.b = append(o.b, ',')
	o.b = appendString(o.b, s)
	return o
}

func (o *ops) boolean(v bool) *ops {
	if v {
		o.b = append(o.b, ",true"...)
	} else {
		o.b = append(o.b, ",false"...)
	}
	return o
}

func (o *ops) null() *ops {
	o.b = append(o.b, ",null"...)
	return o
}

// raw appends JSON written elsewhere.
func (o *ops) raw(json []byte) *ops {
	o.b = append(o.b, ',')
	o.b = append(o.b, json...)
	return o
}

// appendString appends s as a JSON string.
func appendString(b []byte, s string) []byte {
	const hex = "0123456789abcdef"
	b = append(b, '"')
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			switch {
			case c == '"' || c == '\\':
				b = append(b, '\\', c)
			case c == '\n':
				b = append(b, '\\', 'n')
			case c == '\r':
				b = append(b, '\\', 'r')
			case c == '\t':
				b = append(b, '\\', 't')
			case c < 0x20:
				b = append(b, '\\', 'u', '0', '0', hex[c>>4], hex[c&0xf])
			default:
				b = append(b, c)
			}
			i++
			continue
		}
		r, n := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && n == 1:
			b = append(b, `�`...)
		case r == 0x2028 || r == 0x2029:
			// Line separators end JavaScript lines.
			b = append(b, '\\', 'u', '2', '0', '2', hex[r&0xf])
		default:
			b = append(b, s[i:i+n]...)
		}
		i += n
	}
	return append(b, '"')
}

// object builds a JSON object.
type object struct{ b []byte }

func (j *object) key(k string) {
	if len(j.b) == 0 {
		j.b = append(j.b, '{')
	} else {
		j.b = append(j.b, ',')
	}
	j.b = appendString(j.b, k)
	j.b = append(j.b, ':')
}

func (j *object) str(k, v string) *object { j.key(k); j.b = appendString(j.b, v); return j }
func (j *object) num(k string, v float64) *object {
	j.key(k)
	j.b = appendNum(j.b, v)
	return j
}
func (j *object) boolean(k string, v bool) *object {
	j.key(k)
	j.b = strconv.AppendBool(j.b, v)
	return j
}
func (j *object) raw(k string, v []byte) *object { j.key(k); j.b = append(j.b, v...); return j }

func (j *object) bytes() []byte {
	if len(j.b) == 0 {
		return []byte("{}")
	}
	return append(j.b, '}')
}

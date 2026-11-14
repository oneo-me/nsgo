// Package ops writes a commit's operations as JSON for a host that draws
// them: an array of arrays, each led by its code and the view's ID. It is
// written by hand, as a batch is built at every commit and reflection would
// weigh on the host's size and time.
package ops

import (
	"math"
	"strconv"
	"unicode/utf8"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Writer builds one batch's operations.
type Writer struct {
	b    []byte
	open bool // an operation is being written
}

// Reset empties w for the next batch.
func (w *Writer) Reset() { w.b, w.open = w.b[:0], false }

// Empty reports whether w holds no operation.
func (w *Writer) Empty() bool { return len(w.b) == 0 }

// Bytes returns the batch as JSON.
func (w *Writer) Bytes() []byte {
	w.end()
	if len(w.b) == 0 {
		return []byte("[]")
	}
	return append(append([]byte{'['}, w.b[1:]...), ']')
}

// Op starts an operation on view id; its arguments follow.
func (w *Writer) Op(code string, id driver.NodeID) *Writer {
	w.end()
	w.b = append(w.b, ",[\""...)
	w.b = append(w.b, code...)
	w.b = append(w.b, "\","...)
	w.b = strconv.AppendUint(w.b, uint64(id), 10)
	w.open = true
	return w
}

func (w *Writer) end() {
	if w.open {
		w.b = append(w.b, ']')
		w.open = false
	}
}

// Num appends a number, rounded to a hundredth: the render layer's pixels
// need no more.
func (w *Writer) Num(v float64) *Writer {
	w.b = append(w.b, ',')
	w.b = AppendNum(w.b, v)
	return w
}

// AppendNum appends v rounded to a hundredth, and zero for NaN and the
// infinities.
func AppendNum(b []byte, v float64) []byte {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		v = 0
	}
	return strconv.AppendFloat(b, math.Round(v*100)/100, 'f', -1, 64)
}

// Pt appends a point.
func (w *Writer) Pt(v driver.Points) *Writer { return w.Num(float64(v)) }

// Str appends a string.
func (w *Writer) Str(s string) *Writer {
	w.b = append(w.b, ',')
	w.b = AppendString(w.b, s)
	return w
}

// Bool appends a boolean.
func (w *Writer) Bool(v bool) *Writer {
	if v {
		w.b = append(w.b, ",true"...)
	} else {
		w.b = append(w.b, ",false"...)
	}
	return w
}

// Null appends null.
func (w *Writer) Null() *Writer {
	w.b = append(w.b, ",null"...)
	return w
}

// Raw appends JSON written elsewhere.
func (w *Writer) Raw(json []byte) *Writer {
	w.b = append(w.b, ',')
	w.b = append(w.b, json...)
	return w
}

// Object builds a JSON object for an operation's argument.
type Object struct{ b []byte }

// Key starts a member.
func (j *Object) Key(k string) {
	if len(j.b) == 0 {
		j.b = append(j.b, '{')
	} else {
		j.b = append(j.b, ',')
	}
	j.b = AppendString(j.b, k)
	j.b = append(j.b, ':')
}

// Str adds a string member.
func (j *Object) Str(k, v string) *Object { j.Key(k); j.b = AppendString(j.b, v); return j }

// Num adds a number member.
func (j *Object) Num(k string, v float64) *Object {
	j.Key(k)
	if math.IsNaN(v) || math.IsInf(v, 0) {
		v = 0
	}
	j.b = strconv.AppendFloat(j.b, math.Round(v*100)/100, 'f', -1, 64)
	return j
}

// Bool adds a boolean member.
func (j *Object) Bool(k string, v bool) *Object {
	j.Key(k)
	j.b = strconv.AppendBool(j.b, v)
	return j
}

// Raw adds a member written elsewhere.
func (j *Object) Raw(k string, v []byte) *Object { j.Key(k); j.b = append(j.b, v...); return j }

// Bytes returns the object as JSON.
func (j *Object) Bytes() []byte {
	if len(j.b) == 0 {
		return []byte("{}")
	}
	return append(j.b, '}')
}

// AppendString appends s as a JSON string. Invalid UTF-8 becomes the
// replacement character and the line separators U+2028 and U+2029 are
// escaped, as both end a JavaScript line the host reads the batch on.
func AppendString(b []byte, s string) []byte {
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
			b = append(b, '\xef', '\xbf', '\xbd')
		case r == 0x2028 || r == 0x2029:
			b = append(b, '\\', 'u', '2', '0', '2', hex[r&0xf])
		default:
			b = append(b, s[i:i+n]...)
		}
		i += n
	}
	return append(b, '"')
}

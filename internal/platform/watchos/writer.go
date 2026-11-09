package watchos

import (
	"encoding/binary"
	"math"

	"github.com/oneo-me/nsgo/internal/driver"
)

// The ops the host's Store.apply reads, in Host.swift's order.
const (
	opCreate byte = iota + 1
	opInsert
	opRemove
	opDelete
	opFrame
	opLook
	opInk
	opRedraw
	opScroll
	opScrollTo
	opInput
	opFocus
	opBlur
	opAlert
	opRoot
	opLock
	opDismiss
	opCull
)

// writer writes ops as the host's Reader reads them: little-endian
// numbers and length-prefixed UTF-8.
type writer struct{ buf []byte }

// op writes an op and its node IDs.
func (w *writer) op(op byte, ids ...driver.NodeID) {
	w.buf = append(w.buf, op)
	for _, id := range ids {
		w.buf = binary.LittleEndian.AppendUint32(w.buf, uint32(id))
	}
}

func (w *writer) byte(b byte) { w.buf = append(w.buf, b) }

func (w *writer) flag(b bool) {
	if b {
		w.byte(1)
	} else {
		w.byte(0)
	}
}

func (w *writer) f64(vs ...float64) {
	for _, v := range vs {
		w.buf = binary.LittleEndian.AppendUint64(w.buf, math.Float64bits(v))
	}
}

func (w *writer) str(s string) {
	w.buf = binary.LittleEndian.AppendUint32(w.buf, uint32(len(s)))
	w.buf = append(w.buf, s...)
}
